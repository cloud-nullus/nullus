import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Download, Info, Rocket, Save, ShoppingCart, Trash2 } from 'lucide-react'
import Editor from '@monaco-editor/react'
import type { Monaco } from '@monaco-editor/react'
import { configureMonacoYaml } from 'monaco-yaml'
import YAML from 'yaml'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useStackConfigStore } from '../stores/stack-config-store'
import type {
  InstallTab,
  StorageMode,
  StoragePlanMode,
  StorageTargetConfig,
} from '../stores/stack-config-store'
import { getToolAppVersion, getToolChartVersion } from '../stores/stack-config-store'
import { useCreateStack, useDeployStack, useSaveDraft, useResourceDefaults, useStacks, useCompatibilityMatrix, useTemplates } from '../api/stack-api'
import { useClusters, useOrgResourceProfiles, useCreateOrgResourceProfile, useUpdateOrgResourceProfile, useDeleteOrgResourceProfile } from '../../admin/api/admin-api'
import type { CompatibilityMatrix, CreateStackRequest } from '../api/stack-api'
import { useClusterNamespaces } from '../../admin/api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { CodePreview } from '../../../components/shared/code-preview'
import { cn } from '../../../lib/utils'
import { useThemeStore } from '../../../stores/theme-store'
import { useAuthStore } from '../../../stores/auth-store'
import { useAppToast } from '../../../hooks/use-toast'
import { buildInstallOverridesFromTemplate } from '../utils/template-overrides'
import {
  isMatrixCompatibleWithCluster,
  matrixArchMismatches,
} from '../utils/compatibility-arch'
import { extractDeployCompatError } from '../utils/deploy-error'
import { getCompatIssueMessage } from '../utils/compat-issue-i18n'
import { isDeployServerGateLocked } from '../utils/deploy-gate'
import { warnAckKey, readAck, writeAck } from '../utils/warn-ack-storage'
import type { CompatibilityValidationResult } from '../../../types'
import {
  ARTIFACTS_OPTIONS,
  PIPELINE_OPTIONS,
  MONITORING_OPTIONS,
  AUTHENTICATION_OPTIONS,
  LOGGING_OPTIONS,
  STORAGE_PLAN_MODE_OPTIONS,
  STORAGE_SIZE_OPTIONS,
  STORAGE_SIZE_RESOURCE_HINTS,
  STORAGE_PROVIDER_OPTIONS,
  TOOL_LABEL_MAP,
  TOOL_ID_TO_MATRIX_NAME,
  MATRIX_CATEGORY_BY_SLOT,
  GATEWAY_MANIFEST_ID,
  getManifestBundleId,
  SLOT_TOOL_BINDING,
} from '../utils/install-constants'
import {
  PLANNING_PROFILE_LABEL,
  PLANNING_PROFILES,
  PLANNING_OPTION_DEFS,
  round2,
  profileAdjustedBaseline,
  calculateMultipliers,
  applyMultipliers,
  buildFormulaTooltip,
  convertGiToUnit,
  convertUnitToGi,
} from '../utils/install-planning-utils'
import type {
  PlanningSlot,
  PlanningProfile,
  ResourceVector,
  ResourceUnit,
  PlanningRowUnit,
} from '../utils/install-planning-utils'
import {
  normalizeAccessDomain,
  toolLabel,
  getHelmMeta,
  getInstallType,
  buildDefaultStackName,
  buildToolManifest,
  buildGatewayManifest,
  buildHelmStepResourceOverride,
  createDeployScript,
} from '../utils/install-manifest-builders'
import type { ManifestToolEntry } from '../utils/install-manifest-builders'
import { ToolSelector, MultiToolSelector } from '../components/install-tool-selector'

function toDeployErrorMessage(error: unknown): string {
  // Compat-gate errors get a specialized, issue-aware formatter so the user
  // sees every blocking reason without having to open dev tools.
  const gate = extractDeployCompatError(error)
  if (gate) {
    const prefix = gate.code === 'DEPLOY_COMPAT_FAIL' ? '배포 차단(서버 호환성 fail)' : '배포 차단(서버 호환성 warn, 승인 필요)'
    const detail = gate.issueLines.length > 0 ? ' — ' + gate.issueLines.join('; ') : ''
    return `${prefix}${detail}`
  }

  let code = ''
  let backendMessage = ''
  let status: number | undefined
  let genericMessage = ''

  if (typeof error === 'object' && error !== null) {
    const record = error as Record<string, unknown>
    if (typeof record.message === 'string') {
      genericMessage = record.message
    }
    if (typeof record.status === 'number') {
      status = record.status
    }

    const details = record.details
    if (typeof details === 'object' && details !== null) {
      const detailRecord = details as Record<string, unknown>
      const nestedError = detailRecord.error
      if (typeof nestedError === 'object' && nestedError !== null) {
        const nested = nestedError as Record<string, unknown>
        if (typeof nested.code === 'string') {
          code = nested.code
        }
        if (typeof nested.message === 'string') {
          backendMessage = nested.message
        }
        if (typeof nested.http_status === 'number') {
          status = nested.http_status
        }
      }
    }
  }

  const reason = backendMessage || genericMessage || 'unknown backend error'
  const prefix = code ? `[${code}] ` : ''
  const statusSuffix = status ? ` (HTTP ${status})` : ''
  return `배포 작업 등록 실패: ${prefix}${reason}${statusSuffix}`
}

function formatConnectionStatusLabel(status: string): string {
  if (!status) {
    return 'Unknown'
  }
  return status
    .split('_')
    .filter(Boolean)
    .map((token) => token.charAt(0).toUpperCase() + token.slice(1))
    .join(' ')
}


type PreDeployCompatibilityState = 'pass' | 'warn' | 'fail'

type PreDeployCompatibilityIssue = {
  severity: 'warning' | 'error'
  message: string
  // code mirrors the server validate endpoint's issue.code. Used to branch
  // UI copy (e.g. CLUSTER_ARCH_UNKNOWN → link to Refresh Discovery,
  // TOOL_ARCH_UNSUPPORTED → Arch badge, KUBECONFIG_NOT_REGISTERED → admin link).
  code?: string
}

type PreDeployCompatibilityReport = {
  state: PreDeployCompatibilityState
  reason: string
  score: number
  issues: PreDeployCompatibilityIssue[]
  matchedMatrix?: CompatibilityMatrix
  baseline: {
    k8s: string
    minio: string
    postgres: string
    setupType: 'Helm' | 'Deployment' | 'Mixed'
  }
}

function normalizeText(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]/g, '')
}

function resolveMatrixToolName(toolKey: string): string {
  if (TOOL_ID_TO_MATRIX_NAME[toolKey]) {
    return TOOL_ID_TO_MATRIX_NAME[toolKey]
  }

  const label = TOOL_LABEL_MAP.get(toolKey) ?? toolKey
  return label
    .replace('CI/CD', 'CI')
    .replace('ArgoCD', 'Argo CD')
    .replace('Container Registry', 'Registry')
}

function findMatrixToolVersion(matrix: CompatibilityMatrix | undefined, keyword: string): string {
  if (!matrix) return '-'
  const target = normalizeText(keyword)
  const tool = matrix.tools.find((item) => normalizeText(item.name).includes(target))
  return tool?.appVersion ?? '-'
}

type StorageTargetKey = 'database' | 'objectStorage'
type StorageFieldKey = 'existingRef' | 'endpoint' | 'resourceName' | 'accessSecretRef' | 'authId' | 'authPasswordKey'
type StorageValidationErrorKey = `${StorageTargetKey}.${StorageFieldKey}`
type StorageValidationErrors = Partial<Record<StorageValidationErrorKey, string>>
type DryRunCheckStatus = 'pass' | 'warn' | 'fail'

type DryRunCheck = {
  id: string
  title: string
  status: DryRunCheckStatus
  detail: string
}

function getMutationErrorMessage(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return 'Request failed'
}

const STORAGE_ENDPOINT_REGEX = /^((https?:\/\/)[^\s]+|[a-zA-Z0-9.-]+(?::\d{1,5})?)$/
const K8S_SECRET_REF_REGEX = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const SECRET_KEY_REGEX = /^[-._a-zA-Z0-9]+$/

const stackInstallSchema = z.object({
  stackName: z
    .string()
    .min(2, 'Stack name must be at least 2 characters')
    .max(50, 'Stack name must be 50 characters or less')
    .regex(/^[a-zA-Z0-9-]+$/, 'Stack name can include only letters, numbers, and hyphens'),
})

type StackInstallFormData = z.infer<typeof stackInstallSchema>

export function hasDuplicateStackNameInCluster(
  stacks: Array<{ id?: string; clusterId?: string; name: string; status?: string }>,
  clusterId: string | null | undefined,
  stackName: string,
  pendingStackId: string | null = null,
) {
  const normalizedStackName = stackName.trim().toLowerCase()
  if (!clusterId || !normalizedStackName) {
    return false
  }

  return stacks.some(
    (stack) =>
      stack.id !== pendingStackId &&
      stack.clusterId === clusterId &&
      stack.name.trim().toLowerCase() === normalizedStackName
  )
}

export function findReusablePendingStackId(
  stacks: Array<{ id?: string; clusterId?: string; name: string; status?: string }>,
  clusterId: string | null | undefined,
  stackName: string,
) {
  const normalizedStackName = stackName.trim().toLowerCase()
  if (!clusterId || !normalizedStackName) {
    return null
  }

  return stacks.find(
    (stack) =>
      stack.clusterId === clusterId &&
      stack.name.trim().toLowerCase() === normalizedStackName &&
      stack.status === 'pending'
  )?.id ?? null
}

// --- Tab definitions ---

const TABS: { id: InstallTab; label: string }[] = [
  { id: 'authentication', label: 'Authentication' },
  { id: 'artifacts', label: 'Artifacts' },
  { id: 'pipeline', label: 'CI/CD' },
  { id: 'monitoring', label: 'Observability' },
  { id: 'storage', label: 'Storage' },
  { id: 'resources', label: 'Resources' },
  { id: 'manifests', label: 'YAML View' },
  { id: 'deploy-script', label: 'Preview Deploy Script' },
  { id: 'dry-run', label: 'Dry Run' },
]

// --- Main page ---

export function StackInstallPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { t } = useTranslation()
  const theme = useThemeStore((state) => state.theme)
  const isDarkMode = theme === 'dark'
  const {
    draft,
    setActiveTab,
    setTool,
    setMonitoringVisualizations,
    setStackName,
    setAccessDomain,
    setCluster,
    setNamespace,
    loadFromTemplate,
    updateStorage,
    updateStorageTarget,
    updateAccessDomainTls,
    setAuthenticationProvider,
  } = useStackConfigStore()
  const toast = useAppToast()
  const currentOrgId = useAuthStore((state) => state.user?.orgId)
  const createStack = useCreateStack()
  const deployStack = useDeployStack()
  // Separate from compatWarnAcknowledged (client pre-check). When the server
  // returns warn, we require a dedicated ack so the client can't silently
  // re-use a prior pre-check ack.
  const [serverVerdict, setServerVerdict] = useState<CompatibilityValidationResult | null>(null)
  const [serverWarnAcknowledged, setServerWarnAcknowledged] = useState(false)
  const [pendingStackId, setPendingStackId] = useState<string | null>(null)
  const saveDraft = useSaveDraft()
  const { data: resourceDefaultsData } = useResourceDefaults()
  const { data: templatesData, isFetched: isTemplatesFetched } = useTemplates()
  const { data: clustersData } = useClusters()
  const clusters = clustersData?.items ?? []
  const templates = templatesData ?? []
  const { data: stackListData } = useStacks()
  const { data: compatibilityMatrixData } = useCompatibilityMatrix()
  const { data: namespaces } = useClusterNamespaces(draft.clusterId ?? '')
  const [createNewNs, setCreateNewNs] = useState(false)
  const [selectedClusterId, setSelectedClusterId] = useState(draft.clusterId ?? '')
  const [activeTab, setLocalTab] = useState<InstallTab>(draft.activeTab)
  const [planningProfile, setPlanningProfile] = useState<PlanningProfile>('standard')
  const [planningOptionOverrides, setPlanningOptionOverrides] = useState<Record<string, Record<string, number>>>({})
  const [appliedResourceOverrides, setAppliedResourceOverrides] = useState<Record<string, ResourceVector>>({})
  const [planningRowUnits, setPlanningRowUnits] = useState<Record<string, PlanningRowUnit>>({})
  const [selectedOrgProfileId, setSelectedOrgProfileId] = useState<string | null>(null)
  const [saveProfileDialogOpen, setSaveProfileDialogOpen] = useState(false)
  const [saveProfileName, setSaveProfileName] = useState('')
  const { data: orgProfiles = [] } = useOrgResourceProfiles()
  const createOrgProfile = useCreateOrgResourceProfile()
  const updateOrgProfile = useUpdateOrgResourceProfile()
  const deleteOrgProfile = useDeleteOrgResourceProfile()
  const [activeFormulaPopoverKey, setActiveFormulaPopoverKey] = useState<string | null>(null)
  const [storageValidationErrors, setStorageValidationErrors] = useState<StorageValidationErrors>({})
  const [tabGuardError, setTabGuardError] = useState<string | null>(null)
  const [manifestDraftByTool, setManifestDraftByTool] = useState<Record<string, string>>({})
  const [manifestOverridesByTool, setManifestOverridesByTool] = useState<Record<string, string>>({})
  const [manifestErrorsByTool, setManifestErrorsByTool] = useState<Record<string, string>>({})
  const [activeManifestTool, setActiveManifestTool] = useState<string | null>(null)
  const [dryRunExecutedAt, setDryRunExecutedAt] = useState<string | null>(null)
  const manifestSyncTimerRef = useRef<number | null>(null)
  const monacoConfiguredRef = useRef(false)
  const initializedTemplateRef = useRef<string | null>(null)
  const initializedDefaultStackNameRef = useRef(false)
  const stackNameInputRef = useRef<HTMLInputElement | null>(null)
  const clusterSelectRef = useRef<HTMLSelectElement | null>(null)
  const namespaceSelectRef = useRef<HTMLSelectElement | null>(null)
  const newNamespaceInputRef = useRef<HTMLInputElement | null>(null)
  const {
    control,
    trigger,
    setValue,
    setError,
    clearErrors,
    watch,
    formState: { errors, isValid, isSubmitting },
  } = useForm<StackInstallFormData>({
    resolver: zodResolver(stackInstallSchema),
    defaultValues: {
      stackName: draft.stackName,
    },
    mode: 'onChange',
  })

  const effectiveNamespace = createNewNs ? draft.namespace.trim() : draft.namespace.trim() || 'nullus'
  const watchedStackName = watch('stackName')
  const templateIdFromQuery = searchParams.get('template')?.trim() || null
  const effectiveClusterId = selectedClusterId || draft.clusterId
  const normalizedDraftStackName = (watchedStackName || draft.stackName).trim().toLowerCase()
  const duplicateStackNameMessage = t(
    'stackInstall.errors.duplicateStackName',
    'A stack with this name already exists in the selected cluster'
  )
  const noneLabel = t('stackInstall.common.unselected', 'Not selected')
  const reusablePendingStackId = useMemo(
    () =>
      findReusablePendingStackId(
        stackListData?.items ?? [],
        effectiveClusterId,
        normalizedDraftStackName,
      ),
    [effectiveClusterId, normalizedDraftStackName, stackListData?.items],
  )
  const inFlightStackId = pendingStackId ?? reusablePendingStackId
  const isDuplicateStackNameInCluster = useMemo(() => {
    return hasDuplicateStackNameInCluster(
      stackListData?.items ?? [],
      effectiveClusterId,
      normalizedDraftStackName,
      inFlightStackId,
    )
  }, [effectiveClusterId, inFlightStackId, normalizedDraftStackName, stackListData?.items])

  useEffect(() => {
    setSelectedClusterId(draft.clusterId ?? '')
  }, [draft.clusterId])

  useEffect(() => {
    if (!templateIdFromQuery) {
      return
    }
    if (initializedTemplateRef.current === templateIdFromQuery) {
      return
    }

    const matchedTemplate = templates.find((template) => template.id === templateIdFromQuery)
    if (!matchedTemplate && !isTemplatesFetched) {
      return
    }

    const overrides = matchedTemplate ? buildInstallOverridesFromTemplate(matchedTemplate) : undefined
    loadFromTemplate(templateIdFromQuery, overrides)
    initializedTemplateRef.current = templateIdFromQuery
  }, [isTemplatesFetched, loadFromTemplate, templateIdFromQuery, templates])

  useEffect(() => {
    if (!(watchedStackName || draft.stackName).trim() || !effectiveClusterId) {
      if (errors.stackName?.type === 'duplicate') {
        clearErrors('stackName')
      }
      return
    }

    if (isDuplicateStackNameInCluster) {
      setError('stackName', {
        type: 'duplicate',
        message: duplicateStackNameMessage,
      })
      return
    }

    if (errors.stackName?.type === 'duplicate') {
      clearErrors('stackName')
    }
  }, [
    clearErrors,
    draft.stackName,
    duplicateStackNameMessage,
    effectiveClusterId,
    errors.stackName?.type,
    isDuplicateStackNameInCluster,
    setError,
    watchedStackName,
  ])

  const objectStorageBackendTool = draft.storage.objectStorage.providerOrEngine || draft.artifacts.storageBackend.tool || 'minio'
  const objectStorageBackendVersion = draft.storage.objectStorage.version || draft.artifacts.storageBackend.version || getToolAppVersion(objectStorageBackendTool)
  const selectedVisualizations = (draft.monitoring.visualizations ?? []).filter((item) => item.tool)

  const selectedInstallItems = ([
    {
      slot: 'artifacts.sourceRepository',
      category: 'Artifacts > Source Repository',
      toolKey: draft.artifacts.sourceRepository.tool,
      toolLabel: toolLabel(draft.artifacts.sourceRepository.tool, noneLabel),
      toolVersion: draft.artifacts.sourceRepository.version,
    },
    {
      slot: 'artifacts.containerRegistry',
      category: 'Artifacts > Container Registry',
      toolKey: draft.artifacts.containerRegistry.tool,
      toolLabel: toolLabel(draft.artifacts.containerRegistry.tool, noneLabel),
      toolVersion: draft.artifacts.containerRegistry.version,
    },
    {
      slot: 'artifacts.packageRegistry',
      category: 'Artifacts > Package Registry',
      toolKey: draft.artifacts.packageRegistry.tool,
      toolLabel: toolLabel(draft.artifacts.packageRegistry.tool, noneLabel),
      toolVersion: draft.artifacts.packageRegistry.version,
    },
    {
      slot: 'artifacts.storageBackend',
      category: 'Storage > Object Storage Backend',
      toolKey: objectStorageBackendTool,
      toolLabel: toolLabel(objectStorageBackendTool, noneLabel),
      toolVersion: objectStorageBackendVersion,
    },
    {
      slot: 'pipeline.cicdPlatform',
      category: 'CI/CD > Platform',
      toolKey: draft.pipeline.cicdPlatform.tool,
      toolLabel: toolLabel(draft.pipeline.cicdPlatform.tool, noneLabel),
      toolVersion: draft.pipeline.cicdPlatform.version,
    },
    {
      slot: 'pipeline.cdTool',
      category: 'CI/CD > CD Tool',
      toolKey: draft.pipeline.cdTool.tool,
      toolLabel: toolLabel(draft.pipeline.cdTool.tool, noneLabel),
      toolVersion: draft.pipeline.cdTool.version,
    },
    {
      slot: 'monitoring.collection',
      category: 'Observability > Metrics Collection',
      toolKey: draft.monitoring.collection.tool,
      toolLabel: toolLabel(draft.monitoring.collection.tool, noneLabel),
      toolVersion: draft.monitoring.collection.version,
    },
    ...selectedVisualizations.map((item) => ({
      slot: 'monitoring.visualization' as const,
      category: 'Observability > Visualization',
      toolKey: item.tool,
      toolLabel: toolLabel(item.tool, noneLabel),
      toolVersion: item.version,
    })),
    {
      slot: 'logging.search',
      category: 'Observability > Logging/Search',
      toolKey: draft.logging.search.tool,
      toolLabel: toolLabel(draft.logging.search.tool, noneLabel),
      toolVersion: draft.logging.search.version,
    },
    {
      slot: 'logging.traceLayer',
      category: 'Observability > Trace Layer',
      toolKey: draft.logging.traceLayer.tool,
      toolLabel: toolLabel(draft.logging.traceLayer.tool, noneLabel),
      toolVersion: draft.logging.traceLayer.version,
    },
    {
      slot: 'logging.traceExporter',
      category: 'Observability > Trace Exporter/Agent',
      toolKey: draft.logging.traceExporter.tool,
      toolLabel: toolLabel(draft.logging.traceExporter.tool, noneLabel),
      toolVersion: draft.logging.traceExporter.version,
    },
  ] satisfies { slot: PlanningSlot; category: string; toolKey: string; toolLabel: string; toolVersion: string }[]).filter(
    (item) => item.toolKey.length > 0
  )



  const compatibilityGate = useMemo<PreDeployCompatibilityReport>(() => {
    const requestedTools = selectedInstallItems.reduce<Record<string, string>>((acc, item) => {
      const category = MATRIX_CATEGORY_BY_SLOT[item.slot]
      if (!category || !item.toolKey) {
        return acc
      }
      acc[category] = resolveMatrixToolName(item.toolKey)
      return acc
    }, {})

    const setupKinds = new Set(selectedInstallItems.map((item) => getInstallType(getManifestBundleId(item.toolKey))))
    const setupType: 'Helm' | 'Deployment' | 'Mixed' =
      setupKinds.size > 1
        ? 'Mixed'
        : (setupKinds.has('helm') ? 'Helm' : 'Deployment')

    const matrices = Array.isArray(compatibilityMatrixData) ? compatibilityMatrixData : []
    if (matrices.length === 0) {
      return {
        state: 'fail',
        reason: 'Compatibility matrix not available',
        score: 0,
        issues: [{ severity: 'error', message: '호환성 매트릭스를 가져오지 못했습니다. 배포를 잠시 중단해 주세요.' }],
        baseline: {
          k8s: 'Unknown',
          minio: objectStorageBackendVersion,
          postgres: draft.storage.database.version || draft.storage.database.providerOrEngine || 'N/A',
          setupType,
        },
      }
    }

    const matchedCandidates = matrices.filter((matrix) =>
      Object.entries(requestedTools).every(([category, expectedName]) => {
        const matrixTool = matrix.tools.find((tool) => normalizeText(tool.name) === normalizeText(expectedName))
        if (!matrixTool) {
          return false
        }
        return category !== ''
      })
    )

    const matched = matchedCandidates.find((matrix) => matrix.id === draft.selectedTemplateId)
      ?? matchedCandidates[0]

    if (!matched) {
      return {
        state: 'fail',
        reason: 'No matching matrix',
        score: 0,
        issues: [{ severity: 'error', message: '선택한 OSS 조합과 일치하는 호환성 매트릭스가 없습니다.' }],
        baseline: {
          k8s: 'Unknown',
          minio: objectStorageBackendVersion,
          postgres: draft.storage.database.version || draft.storage.database.providerOrEngine || 'N/A',
          setupType,
        },
      }
    }

    const minioFromMatrix = findMatrixToolVersion(matched, 'minio')
    const postgresFromDraft = draft.storage.database.version || draft.storage.database.providerOrEngine || 'N/A'

    // F8 Task 3 / 5: cross-check the matched matrix against the selected
    // cluster's discovered node architectures. Produces issues the gate UI
    // layers on top of the matrix-level verdict below.
    const archIssues: PreDeployCompatibilityIssue[] = []
    let archForcesFail = false
    let archDowngradesToWarn = false
    if (draft.clusterId) {
      const selectedCluster = (clustersData?.items ?? []).find((c) => c.id === draft.clusterId)
      const clusterArchs = selectedCluster?.nodeArchitectures ?? []
      const verdict = isMatrixCompatibleWithCluster(matched, clusterArchs)
      if (verdict === 'unknown') {
        archDowngradesToWarn = true
        archIssues.push({
          severity: 'warning',
          code: 'CLUSTER_ARCH_UNKNOWN',
          message:
            '선택한 클러스터의 노드 아키텍처가 미상입니다. 관리자 > 스택 버전 관리에서 Refresh Discovery를 실행해 주세요.',
        })
      } else if (verdict === 'incompatible') {
        const mismatches = matrixArchMismatches(matched, clusterArchs)
        const detail = mismatches
          .map((m) => `${m.toolName}: ${m.missingArchs.join(', ')}`)
          .join(' · ')
        if (matched.status === 'verified') {
          archForcesFail = true
          archIssues.push({
            severity: 'error',
            code: 'TOOL_ARCH_UNSUPPORTED',
            message: `이 조합의 일부 도구가 클러스터 노드 아키텍처를 지원하지 않습니다 (${detail}).`,
          })
        } else {
          archDowngradesToWarn = true
          archIssues.push({
            severity: 'warning',
            code: 'TOOL_ARCH_UNSUPPORTED',
            message: `검증되지 않은 조합이며 아키텍처 호환성 리스크가 있습니다 (${detail}).`,
          })
        }
      }
    }

    if (matched.status === 'unsupported') {
      return {
        state: 'fail',
        reason: 'Matched matrix is unsupported',
        score: 0,
        issues: [{ severity: 'error', message: '현재 조합은 unsupported 매트릭스로 분류되어 배포할 수 없습니다.' }],
        matchedMatrix: matched,
        baseline: {
          k8s: matched.k8sRange,
          minio: minioFromMatrix,
          postgres: postgresFromDraft,
          setupType,
        },
      }
    }

    if (matched.status === 'untested') {
      return {
        state: 'warn',
        reason: 'Matched matrix is untested',
        score: archDowngradesToWarn || archIssues.length > 0 ? Math.min(70, 60) : 70,
        issues: [
          { severity: 'warning', message: '현재 조합은 untested 매트릭스입니다. 검증 리스크를 인지하고 진행하세요.' },
          ...archIssues,
        ],
        matchedMatrix: matched,
        baseline: {
          k8s: matched.k8sRange,
          minio: minioFromMatrix,
          postgres: postgresFromDraft,
          setupType,
        },
      }
    }

    // Verified matrix path: arch check may downgrade to warn or fail.
    let passState: PreDeployCompatibilityState = 'pass'
    let passScore = 100
    if (archForcesFail) {
      passState = 'fail'
      passScore = 0
    } else if (archDowngradesToWarn) {
      passState = 'warn'
      passScore = Math.min(passScore, 70)
    }
    return {
      state: passState,
      reason: archForcesFail
        ? 'Tool architecture does not match cluster'
        : archDowngradesToWarn
          ? 'Cluster architecture unknown or partial match'
          : 'Matched verified matrix',
      score: passScore,
      issues: archIssues,
      matchedMatrix: matched,
      baseline: {
        k8s: matched.k8sRange,
        minio: minioFromMatrix,
        postgres: postgresFromDraft,
        setupType,
      },
    }
  }, [
    compatibilityMatrixData,
    clustersData,
    draft.clusterId,
    objectStorageBackendVersion,
    draft.storage.database.providerOrEngine,
    draft.storage.database.version,
    draft.selectedTemplateId,
    selectedInstallItems,
  ])

  const [compatWarnAcknowledged, setCompatWarnAcknowledged] = useState(false)

  useEffect(() => {
    if (compatibilityGate.state !== 'warn') {
      setCompatWarnAcknowledged(false)
    }
  }, [compatibilityGate.state])

  // F8-UIUX-WarnAckPersist — persist the warn-ack toggles across tab
  // refreshes via sessionStorage, keyed by (kind, stackName, clusterId,
  // verdictHash). Rotating any of those invalidates the cached ack, so
  // users are forced to re-ack when the underlying issue list changes.
  const clientAckKey = useMemo(
    () => warnAckKey('client', draft.stackName, draft.clusterId ?? '', compatibilityGate.issues),
    [draft.stackName, draft.clusterId, compatibilityGate.issues],
  )
  const serverAckKey = useMemo(
    () =>
      serverVerdict
        ? warnAckKey('server', draft.stackName, draft.clusterId ?? '', serverVerdict.issues)
        : null,
    [draft.stackName, draft.clusterId, serverVerdict],
  )

  useEffect(() => {
    if (compatibilityGate.state !== 'warn') return
    if (readAck(clientAckKey)) setCompatWarnAcknowledged(true)
  }, [clientAckKey, compatibilityGate.state])

  useEffect(() => {
    if (!serverAckKey) return
    if (serverVerdict?.overall.state !== 'warn') return
    if (readAck(serverAckKey)) setServerWarnAcknowledged(true)
  }, [serverAckKey, serverVerdict])
  const selectedToolKeys = Array.from(new Set(selectedInstallItems.map((item) => item.toolKey)))

  const defaultByTool = useMemo(
    () => new Map((resourceDefaultsData?.items ?? []).map((item) => [item.tool_key, item])),
    [resourceDefaultsData?.items]
  )

  const missingDefaultTools = selectedToolKeys.filter((toolKey) => !defaultByTool.has(toolKey))

  const planningRows = selectedInstallItems.map((item) => {
    const rowKey = `${item.slot}:${item.toolKey}`
    const defs = PLANNING_OPTION_DEFS[item.slot]
    const baseOptions = defs.reduce<Record<string, number>>((acc, def) => {
      acc[def.key] = profileAdjustedBaseline(planningProfile, def)
      return acc
    }, {})

    const optionValues = { ...baseOptions, ...(planningOptionOverrides[rowKey] ?? {}) }
    const baseDefault = defaultByTool.get(item.toolKey)

    if (!baseDefault) {
      return {
        ...item,
        rowKey,
        defs,
        optionValues,
        recommended: null,
        applied: null,
        multipliers: null,
      }
    }

		const multipliers = calculateMultipliers(planningProfile, item.slot, optionValues)
    const recommended = applyMultipliers(baseDefault, multipliers)
    const applied = appliedResourceOverrides[rowKey] ?? recommended
    const units = planningRowUnits[rowKey] ?? { memory: 'Gi', storage: 'Gi' }

    return {
      ...item,
      rowKey,
      defs,
      optionValues,
      recommended,
      applied,
      multipliers,
      units,
    }
  })

  const planningAppliedTotal = planningRows.reduce(
    (acc, row) => {
      if (!row.applied) return acc
      return {
        cpuRequest: acc.cpuRequest + row.applied.cpuRequest,
        cpuLimit: acc.cpuLimit + row.applied.cpuLimit,
        memoryRequestGi: acc.memoryRequestGi + row.applied.memoryRequestGi,
        memoryLimitGi: acc.memoryLimitGi + row.applied.memoryLimitGi,
        storageRequestGi: acc.storageRequestGi + row.applied.storageRequestGi,
        storageLimitGi: acc.storageLimitGi + row.applied.storageLimitGi,
      }
    },
    {
      cpuRequest: 0,
      cpuLimit: 0,
      memoryRequestGi: 0,
      memoryLimitGi: 0,
      storageRequestGi: 0,
      storageLimitGi: 0,
    }
  )

  const manifestTools = (() => {
    const map = new Map<string, ManifestToolEntry>()
    selectedInstallItems.forEach((item) => {
      const bundleId = getManifestBundleId(item.toolKey)
      const existing = map.get(bundleId)
      if (!existing) {
        const appVersion = item.toolVersion || getToolAppVersion(bundleId)
        map.set(bundleId, {
          toolId: bundleId,
          toolLabel: toolLabel(bundleId, noneLabel),
          installType: getInstallType(bundleId),
          toolVersion: appVersion,
          chartVersion: getInstallType(bundleId) === 'helm' ? (getToolChartVersion(bundleId) || appVersion) : undefined,
          hasVersionConflict: false,
          roles: [item.category],
          sourceToolIds: [item.toolKey],
          sourceVersions: [appVersion],
        })
        return
      }

      if (!existing.roles.includes(item.category)) {
        existing.roles.push(item.category)
      }
      if (!existing.sourceToolIds.includes(item.toolKey)) {
        existing.sourceToolIds.push(item.toolKey)
      }
      const appVersion = item.toolVersion || getToolAppVersion(bundleId)
      if (!existing.sourceVersions.includes(appVersion)) {
        existing.sourceVersions.push(appVersion)
      }

      if (existing.toolVersion === getToolAppVersion(bundleId) && item.toolVersion) {
        existing.toolVersion = item.toolVersion
      }
      existing.hasVersionConflict = existing.sourceVersions.filter((v) => v && v.length > 0).length > 1
    })
    return Array.from(map.values())
  })()

  const gatewayManifestTool: ManifestToolEntry = {
    toolId: GATEWAY_MANIFEST_ID,
    toolLabel: 'Gateway',
    installType: 'yaml',
    toolVersion: 'gateway.networking.k8s.io/v1',
    hasVersionConflict: false,
    roles: ['Gateway'],
    sourceToolIds: [GATEWAY_MANIFEST_ID],
    sourceVersions: ['gateway.networking.k8s.io/v1'],
  }

  const allManifestTools = [gatewayManifestTool, ...manifestTools]

  const resourceByTool = (() => {
    const map = new Map<string, ResourceVector>()
    planningRows.forEach((row) => {
      if (!row.applied) return
      const bundleId = getManifestBundleId(row.toolKey)
      const prev = map.get(bundleId)
      if (!prev) {
        map.set(bundleId, { ...row.applied })
        return
      }
      map.set(bundleId, {
        cpuRequest: round2(prev.cpuRequest + row.applied.cpuRequest),
        cpuLimit: round2(prev.cpuLimit + row.applied.cpuLimit),
        memoryRequestGi: round2(prev.memoryRequestGi + row.applied.memoryRequestGi),
        memoryLimitGi: round2(prev.memoryLimitGi + row.applied.memoryLimitGi),
        storageRequestGi: round2(prev.storageRequestGi + row.applied.storageRequestGi),
        storageLimitGi: round2(prev.storageLimitGi + row.applied.storageLimitGi),
      })
    })
    return map
  })()

  const rowKeysByTool = (() => {
    const map = new Map<string, string[]>()
    planningRows.forEach((row) => {
      const bundleId = getManifestBundleId(row.toolKey)
      const list = map.get(bundleId) ?? []
      list.push(row.rowKey)
      map.set(bundleId, list)
    })
    return map
  })()

  const defaultManifestByTool = (() => {
    const map: Record<string, string> = {}
    map[GATEWAY_MANIFEST_ID] = buildGatewayManifest(draft, allManifestTools)
    manifestTools.forEach((tool) => {
      const resources = resourceByTool.get(tool.toolId) ?? {
        cpuRequest: 0,
        cpuLimit: 0,
        memoryRequestGi: 0,
        memoryLimitGi: 0,
        storageRequestGi: 0,
        storageLimitGi: 0,
      }
      map[tool.toolId] = buildToolManifest(tool.toolId, tool.toolLabel, draft, resources, tool.toolVersion, tool.chartVersion)
    })
    return map
  })()

  const resolvedActiveManifestTool =
    activeManifestTool && defaultManifestByTool[activeManifestTool]
      ? activeManifestTool
      : (manifestTools[0]?.toolId ?? GATEWAY_MANIFEST_ID)

  const activeManifestInfo = resolvedActiveManifestTool
    ? allManifestTools.find((tool) => tool.toolId === resolvedActiveManifestTool) ?? null
    : null
  const manifestValidationErrorCount = Object.keys(manifestErrorsByTool).length
  const hasManifestValidationError = manifestValidationErrorCount > 0
  const validManifestToolIds = new Set(allManifestTools.map((tool) => tool.toolId))
  const yamlOverridesPayload = allManifestTools.reduce<Record<string, string>>((acc, tool) => {
    if (tool.installType !== 'yaml') {
      return acc
    }

    const overridden = manifestOverridesByTool[tool.toolId]
    const candidate = overridden && overridden.trim() ? overridden : defaultManifestByTool[tool.toolId]
    if (!candidate || !candidate.trim()) {
      return acc
    }

    acc[tool.toolId] = candidate
    return acc
  }, {})

  manifestTools.forEach((tool) => {
    if (tool.installType !== 'helm') {
      return
    }
    const resources = resourceByTool.get(tool.toolId)
    if (!resources) {
      return
    }
    const override = buildHelmStepResourceOverride(tool.toolId, resources)
    if (!override) {
      return
    }
    yamlOverridesPayload[override.key] = YAML.stringify(override.values, { indent: 2, lineWidth: 0 })
  })

  Object.entries(manifestOverridesByTool).forEach(([toolId, yamlText]) => {
    const trimmed = yamlText.trim()
    if (!trimmed || !validManifestToolIds.has(toolId)) {
      return
    }
    yamlOverridesPayload[toolId] = yamlText
  })

  const deployScript = createDeployScript(draft, allManifestTools, defaultManifestByTool, noneLabel)

  const dryRunChecks: DryRunCheck[] = (() => {
    const checks: DryRunCheck[] = []

    checks.push({
      id: 'stackName',
      title: 'Stack Name 형식',
      status: draft.stackName.trim().length >= 2 ? 'pass' : 'fail',
      detail:
        draft.stackName.trim().length >= 2
          ? `stack name: ${draft.stackName}`
          : 'Stack Name은 2자 이상이어야 합니다.',
    })

    checks.push({
      id: 'cluster',
      title: 'Target Cluster 선택',
      status: draft.clusterId ? 'pass' : 'fail',
      detail: draft.clusterId ? `cluster: ${draft.clusterId}` : 'Target Cluster를 선택하세요.',
    })

    checks.push({
      id: 'namespace',
      title: 'Namespace 유효성',
      status: effectiveNamespace ? 'pass' : 'fail',
      detail: effectiveNamespace ? `namespace: ${effectiveNamespace}` : 'Namespace가 비어 있습니다.',
    })

  const accessDomain = normalizeAccessDomain(draft.accessDomain || `${draft.stackName}.internal`)
    checks.push({
      id: 'accessDomain',
      title: 'Access domain 규칙',
      status: accessDomain.endsWith('.internal') ? 'pass' : 'warn',
      detail: accessDomain.endsWith('.internal')
        ? `access domain: ${accessDomain}`
        : `권장 규칙(.internal) 미준수: ${accessDomain}`,
    })

    const tlsConfig = draft.accessDomainTls
    const tlsSecretName = tlsConfig.secretName.trim()
    const tlsSecretNamespace = tlsConfig.secretNamespace.trim()
    const tlsIssuerName = tlsConfig.issuerName.trim()
    if (!tlsConfig.enabled) {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'warn',
        detail: '현재 자동 생성 Gateway는 HTTP(80) 기본값입니다. 운영 환경에서는 Access Domain TLS 인증서 적용을 권장합니다.',
      })
    } else if (!tlsSecretName || !tlsSecretNamespace || !tlsIssuerName) {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'fail',
        detail: 'TLS 활성화 시 Secret 이름, Secret 네임스페이스, cert-manager Issuer 이름은 필수입니다.',
      })
    } else if (tlsSecretNamespace !== effectiveNamespace) {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'warn',
        detail: `TLS Secret namespace가 Gateway namespace(${effectiveNamespace})와 다릅니다: ${tlsSecretNamespace}/${tlsSecretName}. 교차 네임스페이스 참조에는 ReferenceGrant가 필요합니다.`,
      })
    } else {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'pass',
        detail: `TLS 활성화됨: ${tlsSecretNamespace}/${tlsSecretName} (cert-manager issuer: ${tlsIssuerName})`,
      })
    }

    const manifestCount = allManifestTools.length
    const hasOssManifest = manifestTools.length > 0
    checks.push({
      id: 'manifestCount',
      title: '설치 파일 생성 상태',
      status: hasOssManifest ? 'pass' : 'fail',
      detail:
        hasOssManifest
          ? `생성된 설치 파일: ${manifestCount}개 (Gateway 1 + OSS ${manifestTools.length})`
          : '설치 대상 OSS가 없어 YAML/Deploy Script를 생성할 수 없습니다.',
    })

    const manifestErrors = Object.values(manifestErrorsByTool).filter((e) => e && e.length > 0)
    checks.push({
      id: 'manifestLint',
      title: 'YAML/values 검증',
      status: manifestErrors.length === 0 ? 'pass' : 'fail',
      detail:
        manifestErrors.length === 0
          ? '모든 YAML/values 문법 및 필수 항목 검증 통과'
          : `검증 실패 ${manifestErrors.length}건: ${manifestErrors[0]}`,
    })

    const hasResourceFloorIssue = planningRows.some((row) => {
      if (!row.applied) return false
      return (
        row.applied.cpuRequest <= 0 ||
        row.applied.cpuLimit <= 0 ||
        row.applied.memoryRequestGi <= 0 ||
        row.applied.memoryLimitGi <= 0
      )
    })
    checks.push({
      id: 'resourceBounds',
      title: '리소스 하한 검증',
      status: hasResourceFloorIssue ? 'fail' : 'pass',
      detail: hasResourceFloorIssue
        ? '적용값 중 0 이하 리소스가 있습니다.'
        : `request total CPU ${planningAppliedTotal.cpuRequest.toFixed(2)}, memory ${planningAppliedTotal.memoryRequestGi.toFixed(2)}Gi`,
    })

    const hasVersionConflict = manifestTools.some((tool) => tool.hasVersionConflict)
    checks.push({
      id: 'versionConflict',
      title: '번들 OSS 버전 충돌',
      status: hasVersionConflict ? 'warn' : 'pass',
      detail: hasVersionConflict
        ? '동일 번들 내 OSS 버전이 달라 대표 버전으로 통합됩니다.'
        : '번들 OSS 버전 충돌 없음',
    })

    checks.push({
      id: 'storage',
      title: 'Storage 플랜 검토',
      status: draft.storage.planMode === 'existing-all' ? 'warn' : 'pass',
      detail:
        draft.storage.planMode === 'existing-all'
          ? '기존 스토리지 연결 모드입니다. endpoint/secret 참조를 배포 전 확인하세요.'
          : '통합 생성 모드: 설치 시 DB/Object Storage를 함께 생성',
    })

    return checks
  })()

  const dryRunSummary = (() => {
    const failed = dryRunChecks.filter((c) => c.status === 'fail').length
    const warned = dryRunChecks.filter((c) => c.status === 'warn').length
    const passed = dryRunChecks.filter((c) => c.status === 'pass').length
    return {
      failed,
      warned,
      passed,
      total: dryRunChecks.length,
      readyToDeploy: failed === 0,
    }
  })()

  const runDryRunChecks = () => {
    setDryRunExecutedAt(new Date().toLocaleString())
  }

  const handlePlanningOptionChange = (rowKey: string, optionKey: string, value: number) => {
    setPlanningOptionOverrides((prev) => ({
      ...prev,
      [rowKey]: {
        ...(prev[rowKey] ?? {}),
        [optionKey]: value,
      },
    }))
    setAppliedResourceOverrides((prev) => {
      const next = { ...prev }
      delete next[rowKey]
      return next
    })
  }

  const handlePlanningProfileChange = (profile: PlanningProfile) => {
    setPlanningProfile(profile)
    setPlanningOptionOverrides({})
    setAppliedResourceOverrides({})
    setSelectedOrgProfileId(null)
  }

  const handleSizingSelectChange = (value: string) => {
    if (value.startsWith('org:')) {
      const profileId = value.slice(4)
      const profile = orgProfiles.find((p) => p.id === profileId)
      if (!profile) return
      setPlanningProfile(profile.baseProfile)
      setPlanningOptionOverrides(profile.optionOverrides ?? {})
      setAppliedResourceOverrides(profile.appliedResourceOverrides ?? {})
      setPlanningRowUnits(profile.rowUnits ?? {})
      setSelectedOrgProfileId(profileId)
    } else {
      handlePlanningProfileChange(value as PlanningProfile)
    }
  }

  const buildResourceProfilePayload = (name: string) => {
    const currentAppliedResources = planningRows.reduce<Record<string, ResourceVector>>((acc, row) => {
      if (row.applied) {
        acc[row.rowKey] = row.applied
      }
      return acc
    }, {})
    return {
      name,
      baseProfile: planningProfile,
      optionOverrides: planningOptionOverrides,
      appliedResourceOverrides: currentAppliedResources,
      rowUnits: planningRowUnits,
    }
  }

  const handleSaveProfileButtonClick = () => {
    const currentOrgProfiles = currentOrgId
      ? orgProfiles.filter((profile) => profile.orgId === currentOrgId)
      : orgProfiles
    const selectedProfile = selectedOrgProfileId
      ? currentOrgProfiles.find((profile) => profile.id === selectedOrgProfileId)
      : currentOrgProfiles.find((profile) => profile.name.toLowerCase() === PLANNING_PROFILE_LABEL[planningProfile].toLowerCase())
    const selectedProfileName = selectedOrgProfileId
      ? orgProfiles.find((profile) => profile.id === selectedOrgProfileId)?.name ?? PLANNING_PROFILE_LABEL[planningProfile]
      : PLANNING_PROFILE_LABEL[planningProfile]

    if (selectedProfile) {
      updateOrgProfile.mutate({
        id: selectedProfile.id,
        data: buildResourceProfilePayload(selectedProfile.name),
      }, {
        onSuccess: () => {
          toast.success(`Sizing profile "${selectedProfile.name}" saved.`)
        },
        onError: (error) => {
          toast.error(`Failed to save sizing profile "${selectedProfile.name}": ${getMutationErrorMessage(error)}`)
        },
      })
      setSelectedOrgProfileId(selectedProfile.id)
      return
    }

    createOrgProfile.mutate(
      buildResourceProfilePayload(selectedProfileName),
      {
        onSuccess: (created) => {
          setSelectedOrgProfileId(created.id)
          toast.success(`Sizing profile "${created.name}" saved.`)
        },
        onError: (error) => {
          toast.error(`Failed to save sizing profile "${selectedProfileName}": ${getMutationErrorMessage(error)}`)
        },
      }
    )
  }

  const handleSaveProfileConfirm = () => {
    const name = saveProfileName.trim()
    if (!name) return
    createOrgProfile.mutate(
      buildResourceProfilePayload(name),
      {
        onSuccess: (created) => {
          setSelectedOrgProfileId(created.id)
          setSaveProfileDialogOpen(false)
          setSaveProfileName('')
          toast.success(`Sizing profile "${created.name}" saved.`)
        },
        onError: (error) => {
          toast.error(`Failed to save sizing profile "${name}": ${getMutationErrorMessage(error)}`)
        },
      }
    )
  }

  const handleDeleteOrgProfile = () => {
    if (!selectedOrgProfileId) return
    deleteOrgProfile.mutate(selectedOrgProfileId, {
      onSuccess: () => {
        setSelectedOrgProfileId(null)
        setPlanningProfile('standard')
        setPlanningOptionOverrides({})
        setAppliedResourceOverrides({})
      },
    })
  }

  const handleAppliedResourceChange = (rowKey: string, current: ResourceVector, field: keyof ResourceVector, value: number) => {
    setAppliedResourceOverrides((prev) => ({
      ...prev,
      [rowKey]: {
        ...(prev[rowKey] ?? current),
        [field]: value,
      },
    }))
  }

  const handlePlanningUnitChange = (rowKey: string, field: keyof PlanningRowUnit, value: ResourceUnit) => {
    setPlanningRowUnits((prev) => ({
      ...prev,
      [rowKey]: {
        ...(prev[rowKey] ?? { memory: 'Gi', storage: 'Gi' }),
        [field]: value,
      },
    }))
  }

  const handleManifestChange = (toolId: string, value?: string) => {
    const nextYaml = value ?? ''
    setManifestDraftByTool((prev) => ({
      ...prev,
      [toolId]: nextYaml,
    }))

    if (manifestSyncTimerRef.current !== null) {
      window.clearTimeout(manifestSyncTimerRef.current)
    }

    manifestSyncTimerRef.current = window.setTimeout(() => {
      const error = validateManifestAndApply(toolId, nextYaml)
      setManifestErrorsByTool((prev) => {
        if (!error) {
          const next = { ...prev }
          delete next[toolId]
          return next
        }
        return {
          ...prev,
          [toolId]: error,
        }
      })
      if (!error) {
        setManifestOverridesByTool((prev) => ({
          ...prev,
          [toolId]: nextYaml,
        }))
        setManifestDraftByTool((prev) => {
          const next = { ...prev }
          delete next[toolId]
          return next
        })
      }
    }, 350)
  }

  const handleMonacoBeforeMount = useCallback((monaco: Monaco) => {
    if (monacoConfiguredRef.current) return
    configureMonacoYaml(monaco, {
      validate: true,
      completion: false,
      hover: true,
      format: true,
      enableSchemaRequest: false,
      schemas: [],
    })
    monacoConfiguredRef.current = true
  }, [])

  useEffect(() => {
    setValue('stackName', draft.stackName)
  }, [draft.stackName, setValue])

  useEffect(() => {
    if (initializedDefaultStackNameRef.current) return
    initializedDefaultStackNameRef.current = true

    if (useStackConfigStore.getState().draft.stackName.trim().length > 0) return

    const generated = buildDefaultStackName()
    useStackConfigStore.setState((state) => ({
      draft: {
        ...state.draft,
        stackName: generated,
        accessDomain: `${generated}.internal`,
        accessDomainTls: {
          ...state.draft.accessDomainTls,
          secretName: `${generated}-wildcard-tls`,
        },
      },
      isDirty: state.isDirty,
    }))
    setValue('stackName', generated)
  }, [setValue])

  useEffect(() => {
    return () => {
      if (manifestSyncTimerRef.current !== null) {
        window.clearTimeout(manifestSyncTimerRef.current)
      }
    }
  }, [])

  const switchTab = (tab: InstallTab) => {
    if (tab === 'manifests' || tab === 'deploy-script' || tab === 'dry-run') {
      const ok = ensureCoreSelectionsForConfigTabs()
      if (!ok) return
    }
    setTabGuardError(null)
    setLocalTab(tab)
    setActiveTab(tab)
  }

  const handleStoragePlanModeChange = (planMode: StoragePlanMode) => {
    setStorageValidationErrors({})
    if (planMode === 'none') {
      updateStorage({
        planMode,
        database: { ...draft.storage.database, mode: 'create' },
        objectStorage: { ...draft.storage.objectStorage, mode: 'create' },
      })
      return
    }

    if (planMode === 'existing-all') {
      updateStorage({
        planMode,
        database: { ...draft.storage.database, mode: 'existing' },
        objectStorage: { ...draft.storage.objectStorage, mode: 'existing' },
      })
      return
    }

    if (planMode === 'integrated-create') {
      updateStorage({
        planMode,
        database: { ...draft.storage.database, mode: 'create' },
        objectStorage: { ...draft.storage.objectStorage, mode: 'create' },
      })
      return
    }

    updateStorage({
      planMode,
      database: { ...draft.storage.database, mode: 'create' },
      objectStorage: { ...draft.storage.objectStorage, mode: 'create' },
    })
  }

  const getStorageEffectiveMode = (): StorageMode | null => {
    if (draft.storage.planMode === 'none') {
      return null
    }
    return draft.storage.planMode === 'existing-all' ? 'existing' : 'create'
  }

  const getStorageFieldError = (target: StorageTargetKey, field: StorageFieldKey): string | undefined => {
    return storageValidationErrors[`${target}.${field}`]
  }

  const clearStorageFieldError = (target: StorageTargetKey, field: StorageFieldKey) => {
    setStorageValidationErrors((prev) => {
      const next = { ...prev }
      delete next[`${target}.${field}`]
      return next
    })
  }

  const validateStorageConfig = (): boolean => {
    const errors: StorageValidationErrors = {}

    const validateTarget = (target: StorageTargetKey) => {
      if (getStorageEffectiveMode() !== 'existing') return

      const config = draft.storage[target]
      const key = (field: StorageFieldKey): StorageValidationErrorKey => `${target}.${field}`

      if (!config.existingRef.trim()) {
        errors[key('existingRef')] = '기존 리소스 참조 ID는 필수입니다.'
      }

      if (!config.endpoint.trim()) {
        errors[key('endpoint')] = '엔드포인트는 필수입니다.'
      } else if (!STORAGE_ENDPOINT_REGEX.test(config.endpoint.trim())) {
        errors[key('endpoint')] = '엔드포인트 형식이 올바르지 않습니다. (예: postgres.shared.svc:5432 또는 http://minio.shared.svc:9000)'
      }

      if (!config.resourceName.trim()) {
        errors[key('resourceName')] = target === 'database' ? 'DB 이름은 필수입니다.' : 'Bucket 이름은 필수입니다.'
      }

      if (!config.accessSecretRef.trim()) {
        errors[key('accessSecretRef')] = '접근 Secret Ref는 필수입니다.'
      } else if (!K8S_SECRET_REF_REGEX.test(config.accessSecretRef.trim())) {
        errors[key('accessSecretRef')] = 'Secret Ref 형식이 올바르지 않습니다. (소문자/숫자/-, DNS-1123)'
      }

      if (!config.authId.trim()) {
        errors[key('authId')] = target === 'database' ? 'DB 사용자 ID는 필수입니다.' : 'Access Key ID는 필수입니다.'
      }

      if (!config.authPasswordKey.trim()) {
        errors[key('authPasswordKey')] = target === 'database' ? 'DB 비밀번호 Key는 필수입니다.' : 'Secret Key Key는 필수입니다.'
      } else if (!SECRET_KEY_REGEX.test(config.authPasswordKey.trim())) {
        errors[key('authPasswordKey')] = '비밀번호 Key 형식이 올바르지 않습니다. (영문/숫자/-, _, .)'
      }
    }

    validateTarget('database')
    validateTarget('objectStorage')

    setStorageValidationErrors(errors)
    return Object.keys(errors).length === 0
  }

  const ensureCoreSelectionsForConfigTabs = (): boolean => {
    if (!draft.stackName.trim()) {
      setTabGuardError('YAML View 탭으로 이동하려면 Stack Name이 필요합니다.')
      setLocalTab('artifacts')
      setActiveTab('artifacts')
      stackNameInputRef.current?.focus()
      return false
    }

    if (isDuplicateStackNameInCluster) {
      setError('stackName', {
        type: 'duplicate',
        message: duplicateStackNameMessage,
      })
      setTabGuardError(duplicateStackNameMessage)
      stackNameInputRef.current?.focus()
      return false
    }

    if (!draft.clusterId) {
      setTabGuardError('YAML View 탭으로 이동하려면 Target Cluster 선택이 필요합니다.')
      setLocalTab('artifacts')
      setActiveTab('artifacts')
      clusterSelectRef.current?.focus()
      return false
    }

    if (createNewNs && !draft.namespace.trim()) {
      setTabGuardError('YAML View 탭으로 이동하려면 Namespace 선택 또는 입력이 필요합니다.')
      setLocalTab('artifacts')
      setActiveTab('artifacts')
      newNamespaceInputRef.current?.focus()
      return false
    }

    setTabGuardError(null)
    return true
  }

  function validateManifestAndApply(toolId: string, text: string): string | null {
    if (toolId === GATEWAY_MANIFEST_ID) {
      let docs: ReturnType<typeof YAML.parseAllDocuments>
      try {
        docs = YAML.parseAllDocuments(text)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'YAML 파싱 오류'
        return `YAML 문법 오류: ${message}`
      }

      if (docs.some((docItem) => docItem.errors.length > 0)) {
        return 'Gateway YAML 문서에 파싱 오류가 있습니다.'
      }

      const docObjects = docs.map((docItem) => docItem.toJS() as Record<string, unknown>)
      const gateway = docObjects.find((docItem) => docItem.apiVersion === 'gateway.networking.k8s.io/v1' && docItem.kind === 'Gateway')
      const routes = docObjects.filter((docItem) => docItem.apiVersion === 'gateway.networking.k8s.io/v1' && docItem.kind === 'HTTPRoute')
      const certificates = docObjects.filter((docItem) => docItem.apiVersion === 'cert-manager.io/v1' && docItem.kind === 'Certificate')
      const referenceGrants = docObjects.filter((docItem) => docItem.kind === 'ReferenceGrant')
      if (!gateway || routes.length === 0) {
        return 'Gateway YAML은 gateway.networking.k8s.io/v1 Gateway + HTTPRoute 형식이어야 합니다.'
      }

      const metadata = (gateway.metadata ?? {}) as Record<string, unknown>
      const spec = (gateway.spec ?? {}) as Record<string, unknown>
      const namespace = typeof metadata.namespace === 'string' ? metadata.namespace.trim() : ''
      const listeners = Array.isArray(spec.listeners) ? spec.listeners : []
      if (!namespace || listeners.length === 0) {
        return 'Gateway YAML은 metadata.namespace와 spec.listeners를 포함해야 합니다.'
      }

      const httpsListener = listeners.find((listener) => {
        if (!listener || typeof listener !== 'object') return false
        const listenerObj = listener as Record<string, unknown>
        return listenerObj.protocol === 'HTTPS' && typeof listenerObj.tls === 'object'
      })

      let parsedTlsSecretName = ''
      let parsedTlsSecretNamespace = ''
      let parsedTlsIssuerName = ''
      if (httpsListener && typeof httpsListener === 'object') {
        const listenerObj = httpsListener as Record<string, unknown>
        const tls = (listenerObj.tls ?? {}) as Record<string, unknown>
        const certificateRefs = Array.isArray(tls.certificateRefs) ? tls.certificateRefs : []
        if (certificateRefs.length === 0) {
          return 'HTTPS listener를 사용할 때 tls.certificateRefs는 필수입니다.'
        }
        const certRef = certificateRefs[0]
        if (!certRef || typeof certRef !== 'object') {
          return 'tls.certificateRefs[0] 형식이 올바르지 않습니다.'
        }
        const certRefObj = certRef as Record<string, unknown>
        parsedTlsSecretName = typeof certRefObj.name === 'string' ? certRefObj.name.trim() : ''
        parsedTlsSecretNamespace = typeof certRefObj.namespace === 'string' ? certRefObj.namespace.trim() : namespace

        if (!parsedTlsSecretName || !K8S_SECRET_REF_REGEX.test(parsedTlsSecretName)) {
          return 'TLS Secret 이름은 DNS-1123 형식이어야 합니다.'
        }
        if (!parsedTlsSecretNamespace || !K8S_SECRET_REF_REGEX.test(parsedTlsSecretNamespace)) {
          return 'TLS Secret namespace는 DNS-1123 형식이어야 합니다.'
        }

        const matchingCertificate = certificates.find((certificate) => {
          const certificateMetadata = (certificate.metadata ?? {}) as Record<string, unknown>
          const certificateSpec = (certificate.spec ?? {}) as Record<string, unknown>
          const certNamespace = typeof certificateMetadata.namespace === 'string' ? certificateMetadata.namespace.trim() : namespace
          const certSecretName = typeof certificateSpec.secretName === 'string' ? certificateSpec.secretName.trim() : ''
          return certNamespace === parsedTlsSecretNamespace && certSecretName === parsedTlsSecretName
        })

        if (!matchingCertificate) {
          return 'HTTPS listener를 사용할 때 cert-manager Certificate 문서(secretName 매칭)가 필요합니다.'
        }

        const certificateSpec = (matchingCertificate.spec ?? {}) as Record<string, unknown>
        const issuerRef = (certificateSpec.issuerRef ?? {}) as Record<string, unknown>
        parsedTlsIssuerName = typeof issuerRef.name === 'string' ? issuerRef.name.trim() : ''
        if (!parsedTlsIssuerName || !K8S_SECRET_REF_REGEX.test(parsedTlsIssuerName)) {
          return 'cert-manager issuerRef.name은 DNS-1123 형식이어야 합니다.'
        }

        if (parsedTlsSecretNamespace !== namespace) {
          const hasReferenceGrant = referenceGrants.some((grant) => {
            const grantMetadata = (grant.metadata ?? {}) as Record<string, unknown>
            const grantSpec = (grant.spec ?? {}) as Record<string, unknown>
            const grantNamespace = typeof grantMetadata.namespace === 'string' ? grantMetadata.namespace.trim() : ''
            if (grantNamespace !== parsedTlsSecretNamespace) return false

            const from = Array.isArray(grantSpec.from) ? grantSpec.from : []
            const to = Array.isArray(grantSpec.to) ? grantSpec.to : []

            const hasGatewayFrom = from.some((fromItem) => {
              if (!fromItem || typeof fromItem !== 'object') return false
              const fromObj = fromItem as Record<string, unknown>
              return (
                fromObj.group === 'gateway.networking.k8s.io' &&
                fromObj.kind === 'Gateway' &&
                fromObj.namespace === namespace
              )
            })

            const hasSecretTo = to.some((toItem) => {
              if (!toItem || typeof toItem !== 'object') return false
              const toObj = toItem as Record<string, unknown>
              return toObj.kind === 'Secret' && toObj.name === parsedTlsSecretName
            })

            return hasGatewayFrom && hasSecretTo
          })

          if (!hasReferenceGrant) {
            return 'TLS Secret가 Gateway namespace와 다를 때는 ReferenceGrant가 필요합니다.'
          }
        }
      }

      const firstHost = (() => {
        for (const route of routes) {
          const routeSpec = (route.spec ?? {}) as Record<string, unknown>
          const hostnames = Array.isArray(routeSpec.hostnames) ? routeSpec.hostnames : []
          for (const hostname of hostnames) {
            if (typeof hostname === 'string' && hostname.trim()) {
              return hostname.trim()
            }
          }
        }
        return ''
      })()

      if (!firstHost.includes('.')) {
        return 'Gateway host는 {oss}.{access-domain} 형식이어야 합니다.'
      }

      const derivedAccessDomain = normalizeAccessDomain(firstHost.split('.').slice(1).join('.').replace(/^\*\./, ''))
      if (!derivedAccessDomain.endsWith('.internal')) {
        return 'Gateway host의 access domain은 .internal로 끝나야 합니다.'
      }

      const gatewayName = typeof metadata.name === 'string' ? metadata.name.trim() : ''
      for (const route of routes) {
        const routeSpec = (route.spec ?? {}) as Record<string, unknown>
        const parentRefs = Array.isArray(routeSpec.parentRefs) ? routeSpec.parentRefs : []
        const rules = Array.isArray(routeSpec.rules) ? routeSpec.rules : []
        const parentGatewayMatched = parentRefs.some((parent) => {
          if (!parent || typeof parent !== 'object') return false
          return (parent as Record<string, unknown>).name === gatewayName
        })
        if (!parentGatewayMatched) {
          return 'HTTPRoute의 parentRefs.name은 Gateway metadata.name과 일치해야 합니다.'
        }
        if (rules.length === 0) {
          return 'HTTPRoute는 최소 1개 이상의 rules를 포함해야 합니다.'
        }
      }

      setAccessDomain(derivedAccessDomain)
      const hasHttpsListener = Boolean(httpsListener)
      updateAccessDomainTls({
        enabled: hasHttpsListener,
        secretName: hasHttpsListener ? parsedTlsSecretName : draft.accessDomainTls.secretName,
        secretNamespace: hasHttpsListener ? parsedTlsSecretNamespace : draft.accessDomainTls.secretNamespace,
        issuerName: hasHttpsListener ? parsedTlsIssuerName : draft.accessDomainTls.issuerName,
      })
      if (namespace === 'nullus') {
        setCreateNewNs(false)
        setNamespace('')
      } else {
        setCreateNewNs(false)
        setNamespace(namespace)
      }

      return null
    }

    const installType = getInstallType(toolId)
    const parseGi = (value: string) => {
      const match = value.trim().match(/^([0-9]+(?:\.[0-9]+)?)Gi$/i)
      if (!match) return null
      const parsed = Number(match[1])
      return Number.isFinite(parsed) ? parsed : null
    }

    const toNumber = (value: unknown) => {
      const n = typeof value === 'number' ? value : Number(value)
      return Number.isFinite(n) ? n : null
    }

    let stackName = ''
    let accessDomain = ''
    let clusterId = ''
    let namespace = ''
    let version = getToolAppVersion(toolId)
    let planMode: StoragePlanMode | null = null

    let cpuReq: number | null = null
    let cpuLimit: number | null = null
    let memoryReqGi: number | null = null
    let memoryLimitGi: number | null = null
    let storageReqGi: number | null = null
    let storageLimitGi: number | null = null

    let database: Record<string, unknown> = {}
    let objectStorage: Record<string, unknown> = {}

    if (installType === 'helm') {
      let parsed: unknown
      try {
        parsed = YAML.parse(text)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'YAML 파싱 오류'
        return `YAML 문법 오류: ${message}`
      }

      if (!parsed || typeof parsed !== 'object') {
        return 'Helm values.yaml은 객체 형태여야 합니다.'
      }

      const values = parsed as Record<string, unknown>
      const global = (values.global ?? {}) as Record<string, unknown>
      const chart = (values.chart ?? {}) as Record<string, unknown>
      const image = (values.image ?? {}) as Record<string, unknown>
      const resources = (values.resources ?? {}) as Record<string, unknown>
      const requests = (resources.requests ?? {}) as Record<string, unknown>
      const limits = (resources.limits ?? {}) as Record<string, unknown>
      const storage = (values.storage ?? {}) as Record<string, unknown>
      database = (storage.database ?? {}) as Record<string, unknown>
      objectStorage = (storage.objectStorage ?? {}) as Record<string, unknown>

      stackName = typeof global.stackName === 'string' ? global.stackName.trim() : ''
      accessDomain = typeof global.accessDomain === 'string' ? normalizeAccessDomain(global.accessDomain) : ''
      clusterId = typeof global.clusterId === 'string' ? global.clusterId.trim() : ''
      namespace = typeof global.namespace === 'string' ? global.namespace.trim() : ''
      version = typeof image.tag === 'string' && image.tag.trim() ? image.tag.trim() : getToolAppVersion(toolId)

      const chartRepoUrl = typeof chart.repoUrl === 'string' ? chart.repoUrl.trim() : ''
      const chartName = typeof chart.name === 'string' ? chart.name.trim() : ''
      const chartVersion = typeof chart.version === 'string' ? chart.version.trim() : ''
      const expectedChart = getHelmMeta(toolId)
      const expectedChartVersion = getToolChartVersion(toolId)
      if (!chartRepoUrl || !chartName || !chartVersion) {
        return 'Helm values는 chart.repoUrl, chart.name, chart.version이 필요합니다.'
      }
      if (chartRepoUrl !== expectedChart.repoUrl || chartName !== expectedChart.chartName) {
        return `선택된 OSS(${toolId})의 Helm Chart와 일치하지 않습니다. 기대값: ${expectedChart.chartName} @ ${expectedChart.repoUrl}`
      }
      if (expectedChartVersion && chartVersion !== expectedChartVersion) {
        return `선택된 OSS(${toolId})의 Helm Chart 버전과 일치하지 않습니다. 기대값: ${expectedChartVersion}`
      }

      cpuReq = toNumber(requests.cpu)
      cpuLimit = toNumber(limits.cpu)
      memoryReqGi = typeof requests.memory === 'string' ? parseGi(requests.memory) : null
      memoryLimitGi = typeof limits.memory === 'string' ? parseGi(limits.memory) : null
      storageReqGi = typeof requests.storage === 'string' ? parseGi(requests.storage) : null
      storageLimitGi = typeof limits.storage === 'string' ? parseGi(limits.storage) : null

      planMode = storage.planMode === 'existing-all' || storage.planMode === 'integrated-create'
        ? storage.planMode
        : null
    } else {
      let docs: ReturnType<typeof YAML.parseAllDocuments>
      try {
        docs = YAML.parseAllDocuments(text)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'YAML 파싱 오류'
        return `YAML 문법 오류: ${message}`
      }

      if (docs.some((doc) => doc.errors.length > 0)) {
        return 'YAML 문서에 파싱 오류가 있습니다. Deployment/Service 문서를 확인해 주세요.'
      }

      const docObjects = docs.map((doc) => doc.toJS() as Record<string, unknown>)
      const deployment = docObjects.find((doc) => doc.kind === 'Deployment' && doc.apiVersion === 'apps/v1')
      const service = docObjects.find((doc) => doc.kind === 'Service' && doc.apiVersion === 'v1')
      if (docObjects.length !== 2 || !deployment || !service) {
        return 'YAML 타입은 apps/v1 Deployment + v1 Service 두 문서가 모두 필요합니다.'
      }

      const metadata = (deployment.metadata ?? {}) as Record<string, unknown>
      const labels = (metadata.labels ?? {}) as Record<string, unknown>
      const spec = (deployment.spec ?? {}) as Record<string, unknown>
      const selector = (spec.selector ?? {}) as Record<string, unknown>
      const matchLabels = (selector.matchLabels ?? {}) as Record<string, unknown>
      const template = (spec.template ?? {}) as Record<string, unknown>
      const templateSpec = (template.spec ?? {}) as Record<string, unknown>
      const templateMeta = (template.metadata ?? {}) as Record<string, unknown>
      const templateLabels = (templateMeta.labels ?? {}) as Record<string, unknown>
      const containers = Array.isArray(templateSpec.containers) ? templateSpec.containers : []
      const firstContainer = (containers[0] ?? {}) as Record<string, unknown>
      const containerResources = (firstContainer.resources ?? {}) as Record<string, unknown>
      const requests = (containerResources.requests ?? {}) as Record<string, unknown>
      const limits = (containerResources.limits ?? {}) as Record<string, unknown>
      const image = typeof firstContainer.image === 'string' ? firstContainer.image : ''

      if (matchLabels.app !== toolId || templateLabels.app !== toolId) {
        return 'Deployment selector/template labels가 toolId와 일치해야 합니다.'
      }
      if (!image.includes(':')) {
      return 'Deployment 컨테이너 image에는 버전 태그가 필요합니다. (예: docker.io/grafana/grafana:11.1.0)'
    }

      const serviceMeta = (service.metadata ?? {}) as Record<string, unknown>
      const serviceSpec = (service.spec ?? {}) as Record<string, unknown>
      const serviceSelector = (serviceSpec.selector ?? {}) as Record<string, unknown>
      const serviceNamespace = typeof serviceMeta.namespace === 'string' ? serviceMeta.namespace.trim() : ''
      if (serviceSelector.app !== toolId || serviceNamespace !== namespace) {
        return 'Service selector(app)와 namespace가 Deployment와 동일해야 합니다.'
      }

      stackName = typeof labels['nullus.io/stack-name'] === 'string' ? String(labels['nullus.io/stack-name']).trim() : ''
      accessDomain = normalizeAccessDomain(draft.accessDomain || `${stackName}.internal`)
      clusterId = typeof labels['nullus.io/cluster-id'] === 'string' ? String(labels['nullus.io/cluster-id']).trim() : ''
      namespace = typeof metadata.namespace === 'string' ? metadata.namespace.trim() : ''
      if (image.includes(':')) {
        version = image.split(':').pop()?.trim() || getToolAppVersion(toolId)
      }

      cpuReq = toNumber(requests.cpu)
      cpuLimit = toNumber(limits.cpu)
      memoryReqGi = typeof requests.memory === 'string' ? parseGi(requests.memory) : null
      memoryLimitGi = typeof limits.memory === 'string' ? parseGi(limits.memory) : null

      const defaultResource = resourceByTool.get(toolId)
      storageReqGi = defaultResource?.storageRequestGi ?? 0
      storageLimitGi = defaultResource?.storageLimitGi ?? 0
      planMode = draft.storage.planMode
    }

    if (!stackName || !clusterId || !namespace) {
      return installType === 'helm'
        ? 'values.global.stackName, values.global.clusterId, values.global.namespace는 필수입니다.'
        : 'Deployment metadata.namespace 및 labels(nullus.io/stack-name, nullus.io/cluster-id)가 필요합니다.'
    }

    if (installType === 'helm') {
      if (!accessDomain) {
        return 'values.global.accessDomain은 필수입니다.'
      }
      if (!accessDomain.endsWith('.internal')) {
        return 'values.global.accessDomain은 .internal 도메인이어야 합니다.'
      }
    }

    if (
      cpuReq === null ||
      cpuLimit === null ||
      memoryReqGi === null ||
      memoryLimitGi === null ||
      storageReqGi === null ||
      storageLimitGi === null
    ) {
      return installType === 'helm'
        ? 'values.resources.requests/limits(cpu/memory/storage)는 모두 필요하며 memory/storage는 Gi 형식이어야 합니다.'
        : 'Deployment 컨테이너 resources.requests/limits(cpu/memory)가 필요하며 memory는 Gi 형식이어야 합니다.'
    }

    if (cpuReq <= 0 || cpuLimit <= 0 || memoryReqGi <= 0 || memoryLimitGi <= 0 || storageReqGi <= 0 || storageLimitGi <= 0) {
      return '리소스 값은 모두 0보다 커야 합니다.'
    }

    if (cpuReq > cpuLimit || memoryReqGi > memoryLimitGi || storageReqGi > storageLimitGi) {
      return '요청값(request)은 제한값(limit)보다 클 수 없습니다.'
    }

    if (installType === 'helm' && planMode === 'existing-all') {
      const databaseEndpoint = typeof database.endpoint === 'string' ? database.endpoint.trim() : ''
      const databaseSecretRef = typeof database.accessSecretRef === 'string' ? database.accessSecretRef.trim() : ''
      const databaseSecretKey = typeof database.authPasswordKey === 'string' ? database.authPasswordKey.trim() : ''
      const objectEndpoint = typeof objectStorage.endpoint === 'string' ? objectStorage.endpoint.trim() : ''
      const objectSecretRef = typeof objectStorage.accessSecretRef === 'string' ? objectStorage.accessSecretRef.trim() : ''
      const objectSecretKey = typeof objectStorage.authPasswordKey === 'string' ? objectStorage.authPasswordKey.trim() : ''

      const requiredExisting = [
        database.existingRef,
        databaseEndpoint,
        database.resourceName,
        databaseSecretRef,
        database.authId,
        databaseSecretKey,
        objectStorage.existingRef,
        objectEndpoint,
        objectStorage.resourceName,
        objectSecretRef,
        objectStorage.authId,
        objectSecretKey,
      ].every((value) => typeof value === 'string' && value.trim().length > 0)

      if (!requiredExisting) {
        return 'storage.planMode가 existing-all이면 DB/Object Storage 연결 및 계정 정보가 모두 필요합니다.'
      }

      if (
        !STORAGE_ENDPOINT_REGEX.test(databaseEndpoint) ||
        !STORAGE_ENDPOINT_REGEX.test(objectEndpoint) ||
        !K8S_SECRET_REF_REGEX.test(databaseSecretRef) ||
        !K8S_SECRET_REF_REGEX.test(objectSecretRef) ||
        !SECRET_KEY_REGEX.test(databaseSecretKey) ||
        !SECRET_KEY_REGEX.test(objectSecretKey)
      ) {
        return 'existing-all 설정의 endpoint/secret 형식이 올바르지 않습니다.'
      }
    }

    const rowKeys = rowKeysByTool.get(toolId) ?? []
    if (rowKeys.length === 0) {
      return '현재 선택된 OSS에서 해당 tool을 찾을 수 없습니다.'
    }

    const installVersion = version || getToolAppVersion(toolId)
    const rowAppliedMap = new Map(
      planningRows
        .filter((row) => row.applied)
        .map((row) => [row.rowKey, row.applied as ResourceVector])
    )

    const currentTotals = rowKeys.reduce(
      (acc, rowKey) => {
        const current = rowAppliedMap.get(rowKey)
        if (!current) return acc
        return {
          cpuRequest: acc.cpuRequest + current.cpuRequest,
          cpuLimit: acc.cpuLimit + current.cpuLimit,
          memoryRequestGi: acc.memoryRequestGi + current.memoryRequestGi,
          memoryLimitGi: acc.memoryLimitGi + current.memoryLimitGi,
          storageRequestGi: acc.storageRequestGi + current.storageRequestGi,
          storageLimitGi: acc.storageLimitGi + current.storageLimitGi,
        }
      },
      {
        cpuRequest: 0,
        cpuLimit: 0,
        memoryRequestGi: 0,
        memoryLimitGi: 0,
        storageRequestGi: 0,
        storageLimitGi: 0,
      }
    )

    const targetTotal: ResourceVector = {
      cpuRequest: cpuReq,
      cpuLimit,
      memoryRequestGi: memoryReqGi,
      memoryLimitGi,
      storageRequestGi: storageReqGi,
      storageLimitGi,
    }

    const fieldRatio = (rowKey: string, field: keyof ResourceVector) => {
      const base = rowAppliedMap.get(rowKey)
      const total = currentTotals[field]
      if (base && total > 0) return base[field] / total
      return 1 / Math.max(rowKeys.length, 1)
    }

    const distributedOverrides = rowKeys.reduce<Record<string, ResourceVector>>((acc, rowKey) => {
      acc[rowKey] = {
        cpuRequest: round2(targetTotal.cpuRequest * fieldRatio(rowKey, 'cpuRequest')),
        cpuLimit: round2(targetTotal.cpuLimit * fieldRatio(rowKey, 'cpuLimit')),
        memoryRequestGi: round2(targetTotal.memoryRequestGi * fieldRatio(rowKey, 'memoryRequestGi')),
        memoryLimitGi: round2(targetTotal.memoryLimitGi * fieldRatio(rowKey, 'memoryLimitGi')),
        storageRequestGi: round2(targetTotal.storageRequestGi * fieldRatio(rowKey, 'storageRequestGi')),
        storageLimitGi: round2(targetTotal.storageLimitGi * fieldRatio(rowKey, 'storageLimitGi')),
      }
      return acc
    }, {})

    rowKeys.forEach((rowKey) => {
      const slot = rowKey.split(':')[0] as PlanningSlot
      const rowToolId = rowKey.split(':')[1]
      const binding = SLOT_TOOL_BINDING[slot]
      if (!binding) return
      setTool(binding.section, binding.field, { tool: rowToolId, version: installVersion })
    })

    setAppliedResourceOverrides((prev) => ({
      ...prev,
      ...distributedOverrides,
    }))

    setStackName(stackName)
    if (installType === 'helm') {
      setAccessDomain(accessDomain)
    }
    setCluster(clusterId)
    if (namespace === 'nullus') {
      setCreateNewNs(false)
      setNamespace('')
    } else {
      setCreateNewNs(false)
      setNamespace(namespace)
    }

    if (installType === 'helm' && planMode) {
      updateStorage({ planMode })
      if (planMode === 'existing-all') {
        updateStorageTarget('database', {
          mode: 'existing',
          existingRef: String(database.existingRef ?? ''),
          endpoint: String(database.endpoint ?? ''),
          resourceName: String(database.resourceName ?? ''),
          accessSecretRef: String(database.accessSecretRef ?? ''),
          authId: String(database.authId ?? ''),
          authPasswordKey: String(database.authPasswordKey ?? ''),
        })
        updateStorageTarget('objectStorage', {
          mode: 'existing',
          existingRef: String(objectStorage.existingRef ?? ''),
          endpoint: String(objectStorage.endpoint ?? ''),
          resourceName: String(objectStorage.resourceName ?? ''),
          accessSecretRef: String(objectStorage.accessSecretRef ?? ''),
          authId: String(objectStorage.authId ?? ''),
          authPasswordKey: String(objectStorage.authPasswordKey ?? ''),
        })
      }
    }

    return null
  }

  const validateCoreFields = async () => {
    const isStackValid = await trigger(['stackName'])
    if (!isStackValid) {
      stackNameInputRef.current?.focus()
      return false
    }

    if (!draft.clusterId) {
      setTabGuardError('Deploy/Save 전 Target Cluster를 선택해 주세요.')
      clusterSelectRef.current?.focus()
      return false
    }

    if (createNewNs && !draft.namespace.trim()) {
      setTabGuardError('Deploy/Save 전 Namespace를 선택하거나 입력해 주세요.')
      newNamespaceInputRef.current?.focus()
      return false
    }

    if (draft.accessDomainTls.enabled) {
      const tlsSecretName = draft.accessDomainTls.secretName.trim()
      const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim()
      const tlsIssuerName = draft.accessDomainTls.issuerName.trim()
      if (!tlsSecretName || !K8S_SECRET_REF_REGEX.test(tlsSecretName)) {
        setTabGuardError('TLS Secret Name은 DNS-1123 형식으로 입력해 주세요. (예: nullus-wildcard-tls)')
        return false
      }
      if (!tlsSecretNamespace || !K8S_SECRET_REF_REGEX.test(tlsSecretNamespace)) {
        setTabGuardError('TLS Secret Namespace는 DNS-1123 형식으로 입력해 주세요. (예: nullus)')
        return false
      }
      if (!tlsIssuerName || !K8S_SECRET_REF_REGEX.test(tlsIssuerName)) {
        setTabGuardError('cert-manager Issuer Name은 DNS-1123 형식으로 입력해 주세요. (예: nullus-ca-issuer)')
        return false
      }
    }

    setTabGuardError(null)
    return true
  }

  const buildStackRequest = (): CreateStackRequest => {
    const storageConfig = draft.storage.planMode === 'none' ? undefined : draft.storage
    return {
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      namespace: effectiveNamespace,
      stackName: draft.stackName,
      accessDomain: draft.accessDomain,
      accessDomainTls: draft.accessDomainTls,
      authentication: draft.authentication,
      yamlOverrides: yamlOverridesPayload,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
      storage: storageConfig,
    }
  }

  const handleDeploy = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return
    const isStorageValid = validateStorageConfig()
    if (!isStorageValid) {
      switchTab('storage')
      return
    }

    if (compatibilityGate.state === 'fail') {
      setTabGuardError('호환성 검사 결과 fail 상태입니다. 조합을 수정한 후 다시 시도해 주세요.')
      return
    }

    if (compatibilityGate.state === 'warn' && !compatWarnAcknowledged) {
      setTabGuardError('호환성 경고를 확인하고 승인 체크를 완료해 주세요.')
      return
    }

    const request = buildStackRequest()
    let activeStackId = inFlightStackId

    try {
      // Reuse an in-flight stackId if the user already created the stack in
      // this page session or if a previous attempt left a pending row with
      // the same cluster/name. Otherwise create fresh.
      let stackId = activeStackId
      if (!stackId) {
        const createRes = await createStack.mutateAsync(request)
        stackId = createRes?.id ?? null
        if (!stackId) {
          setTabGuardError('스택 생성은 되었지만 stack ID를 확인하지 못했습니다. 다시 시도해 주세요.')
          return
        }
        activeStackId = stackId
        setPendingStackId(stackId)
      }

      await deployStack.mutateAsync({
        stackId,
        acknowledgeWarnings: serverWarnAcknowledged || compatWarnAcknowledged,
      })
      setPendingStackId(null)
      setServerVerdict(null)
      setServerWarnAcknowledged(false)
      navigate(`/stack/deploy/${stackId}`)
    } catch (error) {
      const message = toDeployErrorMessage(error)
      if (
        !activeStackId &&
        (message.includes('COMPATIBILITY_REQUEST_INVALID') || message.includes('tools map or stack_id is required'))
      ) {
        setPendingStackId(null)
      }
      setTabGuardError(message)
    }
  }

  const handleSaveDraft = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return
    const isStorageValid = validateStorageConfig()
    if (!isStorageValid) {
      switchTab('storage')
      return
    }

    saveDraft.mutate(buildStackRequest())
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: t('stackInstall.breadcrumb.stackList', 'Stack List'), path: '/stack/list' },
        { label: t('stackInstall.breadcrumb.newStack', 'New Stack'), path: '/stack/templates' },
        { label: t('stackInstall.breadcrumb.current', 'Stack Install') },
      ]} />

      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <Download size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('stackInstall.page.title', 'Stack Install')}
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              {t('stackInstall.page.description', 'Configure your DevSecOps stack with a 5-step workflow.')}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="md"
            loading={saveDraft.isPending}
            onClick={handleSaveDraft}
            disabled={!isValid || isSubmitting}
            type="button"
          >
            <Save size={14} />
            {t('stackInstall.actions.saveDraft', 'Save Draft')}
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={createStack.isPending || deployStack.isPending}
            onClick={handleDeploy}
            disabled={
              isSubmitting ||
              createStack.isPending ||
              deployStack.isPending ||
              !draft.stackName ||
              draft.stackName.length < 2 ||
              isDuplicateStackNameInCluster ||
              !draft.clusterId ||
              (createNewNs && !draft.namespace.trim()) ||
              hasManifestValidationError ||
              compatibilityGate.state === 'fail' ||
              (compatibilityGate.state === 'warn' && !compatWarnAcknowledged) ||
              isDeployServerGateLocked(serverVerdict, serverWarnAcknowledged)
            }
            type="button"
          >
            <Rocket size={14} />
            {t('stackInstall.actions.deploy', 'Deploy')}
          </Button>
        </div>
      </div>
      {hasManifestValidationError && (
        <div className="mb-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]">
          Strict 버전/YAML 검증 실패 {manifestValidationErrorCount}건으로 Deploy가 잠겼습니다. YAML View에서 오류를 해소해 주세요.
        </div>
      )}
      {serverVerdict?.overall.state === 'fail' && (
        <div
          className="mb-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]"
          data-testid="server-fail-hint"
        >
          {t(
            'stackInstall.compatibility.gate.serverFailHint',
            '서버 호환성 검증에서 차단되었습니다. 위의 상세 이슈를 확인한 뒤 조합을 수정해 주세요.',
          )}
        </div>
      )}

      <div className="mb-5 flex flex-col gap-4">
        <div className="w-full rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div>
              <Controller
                control={control}
                name="stackName"
                render={({ field }) => (
                  <>
                    <Input
                      ref={stackNameInputRef}
                      label={t('stackInstall.form.stackName', 'Stack Name')}
                      placeholder="예: nullus-devsecops-stack-20260324-193000"
                      value={field.value}
                      onChange={(e) => {
                        field.onChange(e.target.value)
                        setStackName(e.target.value)
                      }}
                      onBlur={field.onBlur}
                    />
                    {isDuplicateStackNameInCluster ? (
                      <span className="text-xs text-[#ef4444]">{duplicateStackNameMessage}</span>
                    ) : (
                      errors.stackName && <span className="text-xs text-[#ef4444]">{errors.stackName.message}</span>
                    )}
                  </>
                )}
              />
            </div>
            <div>
              <Input
                label={t('stackInstall.form.accessDomain', 'Access domain')}
                placeholder="{stack-name}.internal"
                value={draft.accessDomain || `${draft.stackName || 'nullus-stack'}.internal`}
                onChange={(e) => setAccessDomain(e.target.value)}
              />
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                {t('stackInstall.form.finalAccessGuidePrefix', 'Final access guide: each OSS is available at')} <code>{`{OSS}.${draft.stackName || 'stack-name'}.internal`}</code> {t('stackInstall.form.finalAccessGuideSuffix', '.')}
              </p>
            </div>
          </div>

          <div className="mt-3">
            <label className="inline-flex items-center gap-2 text-xs text-[var(--color-text-secondary)]">
              <input
                type="checkbox"
                checked={draft.accessDomainTls.enabled}
                onChange={(e) => updateAccessDomainTls({ enabled: e.target.checked })}
              />
              {t('stackInstall.form.accessDomainTls', 'Enable Access Domain TLS (cert-manager)')}
            </label>
          </div>

          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-3">
            {draft.accessDomainTls.enabled && (
              <>
                <Input
                  label={t('stackInstall.form.tlsSecretName', 'TLS Secret Name')}
                  placeholder="nullus-wildcard-tls"
                  value={draft.accessDomainTls.secretName}
                  onChange={(e) => updateAccessDomainTls({ secretName: e.target.value })}
                />
                <Input
                  label={t('stackInstall.form.tlsSecretNamespace', 'TLS Secret Namespace')}
                  placeholder="nullus"
                  value={draft.accessDomainTls.secretNamespace}
                  onChange={(e) => updateAccessDomainTls({ secretNamespace: e.target.value })}
                />
                <Input
                  label={t('stackInstall.form.certManagerIssuerName', 'cert-manager Issuer Name')}
                  placeholder="nullus-ca-issuer"
                  value={draft.accessDomainTls.issuerName}
                  onChange={(e) => updateAccessDomainTls({ issuerName: e.target.value })}
                />
              </>
            )}
          </div>

          {draft.accessDomainTls.enabled && (
            <p className="mt-2 text-[11px] text-[var(--color-text-secondary)]">
              Preview Deploy Script와 Gateway YAML에 cert-manager <code>Certificate</code> 리소스가 포함되며, Secret은 cert-manager가 관리합니다.
            </p>
          )}

          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="flex flex-col gap-1">
              <NativeSelect
                ref={clusterSelectRef}
                label={t('stackInstall.form.targetCluster', 'Target Cluster')}
                value={draft.clusterId ?? ''}
                onChange={(e) => {
                  setSelectedClusterId(e.target.value)
                  setCluster(e.target.value)
                }}
                className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
              >
                <option value="">{t('stackInstall.form.selectClusterPlaceholder', 'Select a cluster')}</option>
                {(clusters ?? []).map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({formatConnectionStatusLabel(c.status)})
                  </option>
                ))}
              </NativeSelect>
              {!draft.clusterId && <span className="text-xs text-[#f59e0b]">{t('stackInstall.form.clusterRequired', 'Required for deployment')}</span>}
            </div>

            {draft.clusterId && (
              <div className="flex flex-col gap-1">
                <NativeSelect
                  ref={namespaceSelectRef}
                  label={t('stackInstall.form.namespace', 'Namespace')}
                  value={createNewNs ? '__new__' : draft.namespace}
                  onChange={(e) => {
                    if (e.target.value === '__new__') {
                      setCreateNewNs(true)
                      setNamespace('')
                    } else {
                      setCreateNewNs(false)
                      setNamespace(e.target.value)
                    }
                  }}
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                >
                  <option value="">기본 (nullus)</option>
                  {(namespaces ?? []).map((ns) => (
                    <option key={ns.name} value={ns.name}>{ns.name}</option>
                  ))}
                  <option value="__new__">새 네임스페이스 생성...</option>
                </NativeSelect>
                {createNewNs && (
                  <input
                    ref={newNamespaceInputRef}
                    type="text"
                    placeholder="my-namespace"
                    value={draft.namespace}
                    onChange={(e) => setNamespace(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                    className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                  />
                )}
                <span className="text-[11px] text-[var(--color-text-secondary)]">배포 대상 네임스페이스</span>
              </div>
            )}
          </div>
        </div>

        <div className="w-full rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <div className="mb-3 flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-[rgba(99,102,241,0.18)] text-[#a5b4fc]">
              <ShoppingCart size={16} />
            </div>
            <div>
            <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Resource Total</h3>
            <p className="m-0 text-xs text-[var(--color-text-secondary)]">
                {t('stackInstall.resourceTotal.description', 'Combined request/limit totals for {{count}} selected OSS', { count: selectedToolKeys.length })}
              </p>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="rounded-lg border border-[rgba(99,102,241,0.25)] bg-[rgba(99,102,241,0.08)] p-3">
              <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">Request Total</div>
              <div className="grid grid-cols-3 gap-2 text-sm">
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">CPU</div>
                  <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.cpuRequest.toFixed(2)}</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Memory</div>
                  <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.memoryRequestGi.toFixed(2)}Gi</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Storage</div>
                  <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.storageRequestGi.toFixed(2)}Gi</div>
                </div>
              </div>
            </div>

            <div className="rounded-lg border border-[rgba(34,197,94,0.25)] bg-[rgba(34,197,94,0.08)] p-3">
              <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">Limit Total</div>
              <div className="grid grid-cols-3 gap-2 text-sm">
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">CPU</div>
                  <div className="font-semibold text-[#86efac]">{planningAppliedTotal.cpuLimit.toFixed(2)}</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Memory</div>
                  <div className="font-semibold text-[#86efac]">{planningAppliedTotal.memoryLimitGi.toFixed(2)}Gi</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Storage</div>
                  <div className="font-semibold text-[#86efac]">{planningAppliedTotal.storageLimitGi.toFixed(2)}Gi</div>
                </div>
              </div>
            </div>
          </div>

          {missingDefaultTools.length > 0 && (
            <div className="mt-3 text-xs text-[#fbbf24]">
              기본값 미정의 OSS: {missingDefaultTools.join(', ')}
            </div>
          )}
        </div>

      <div
        className={cn(
          'mb-3 rounded border px-3 py-3 text-xs',
          compatibilityGate.state === 'pass'
            ? 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)] text-[#86efac]'
            : compatibilityGate.state === 'warn'
              ? 'border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)] text-[#fcd34d]'
              : 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] text-[#fca5a5]'
        )}
      >
        <div className="mb-2 flex items-center justify-between gap-2">
          <span className="font-semibold">Pre-Deploy Compatibility Gate ({compatibilityGate.state.toUpperCase()})</span>
          <span>Score: {compatibilityGate.score}</span>
        </div>
        <div className="grid grid-cols-1 gap-1 sm:grid-cols-2 lg:grid-cols-4">
          <span><strong>K8s:</strong> {compatibilityGate.baseline.k8s}</span>
          <span><strong>MinIO:</strong> {compatibilityGate.baseline.minio}</span>
          <span><strong>Postgres:</strong> {compatibilityGate.baseline.postgres}</span>
          <span><strong>Setup:</strong> {compatibilityGate.baseline.setupType}</span>
        </div>
        {compatibilityGate.matchedMatrix && (
          <p className="mt-2 mb-0 text-[11px] text-[var(--color-text-secondary)]">
            Matched matrix: {compatibilityGate.matchedMatrix.name} ({compatibilityGate.matchedMatrix.status})
          </p>
        )}
        {compatibilityGate.issues.length > 0 && (
          <ul className="mb-0 mt-2 pl-4 text-[11px]">
            {compatibilityGate.issues.map((issue, index) => (
              <li key={`${issue.severity}-${index}`}>{issue.message}</li>
            ))}
          </ul>
        )}
        {compatibilityGate.state === 'warn' && (
          <label className="mt-2 inline-flex items-center gap-2 text-[11px] text-[var(--color-text-secondary)]">
            <input
              type="checkbox"
              checked={compatWarnAcknowledged}
              onChange={(e) => {
                setCompatWarnAcknowledged(e.target.checked)
                writeAck(clientAckKey, e.target.checked)
              }}
              aria-label={t(
                'stackInstall.compatibility.gate.ackAria',
                'Acknowledge client-side compatibility warning',
              )}
            />
            경고를 확인했고, untested 조합 리스크를 인지한 상태로 배포를 진행합니다.
          </label>
        )}
      </div>

      {/* F8-F3: server-side verdict panel. Renders only after the wizard has
          called /stacks/:id/validate post-createStack. Fail shows issue list
          as a hard block; warn shows an ack checkbox the user must tick before
          pressing Deploy again. */}
      {serverVerdict && serverVerdict.overall.state === 'pass' ? (
        <div
          className="mb-3 rounded border border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)] px-3 py-2 text-xs text-[#86efac]"
          data-testid="server-verdict-panel"
          data-state="pass"
        >
          ✓ {t('stackInstall.compatibility.serverVerdict.passShort', '서버 호환성 검증을 통과했습니다')}
        </div>
      ) : serverVerdict && (
        <div
          className={cn(
            'mb-3 rounded border px-3 py-3 text-xs',
            serverVerdict.overall.state === 'warn'
              ? 'border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)] text-[#fcd34d]'
              : 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] text-[#fca5a5]',
          )}
          data-testid="server-verdict-panel"
        >
          <div className="mb-2 font-semibold">
            {t(
              'stackInstall.compatibility.serverVerdict.title',
              'Server Pre-Deploy Gate',
            )}{' '}
            ({serverVerdict.overall.state.toUpperCase()})
          </div>
          {serverVerdict.issues.length > 0 && (
            <ul className="mb-0 mt-1 pl-4 text-[11px]">
              {serverVerdict.issues.map((issue, index) => (
                <li
                  key={`${issue.code ?? issue.tool}-${index}`}
                  data-code={issue.code ?? undefined}
                >
                  {getCompatIssueMessage(t, issue)}
                  {issue.tool ? (
                    <span className="ml-1 text-[10px] text-[var(--color-text-tertiary)]">
                      ({issue.tool})
                    </span>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
          {serverVerdict.overall.state === 'warn' && (
            <label className="mt-2 inline-flex items-center gap-2 text-[11px] text-[var(--color-text-secondary)]">
              <input
                type="checkbox"
                checked={serverWarnAcknowledged}
                onChange={(e) => {
                  setServerWarnAcknowledged(e.target.checked)
                  if (serverAckKey) writeAck(serverAckKey, e.target.checked)
                }}
                aria-label={t(
                  'stackInstall.compatibility.serverVerdict.ackAria',
                  'Acknowledge server-side compatibility warning',
                )}
                data-testid="server-warn-ack"
              />
              {t(
                'stackInstall.compatibility.serverVerdict.ackLabel',
                '서버 호환성 경고를 확인했고 리스크를 감수하고 배포를 진행합니다.',
              )}
            </label>
          )}
        </div>
      )}

      

      </div>

      <div className="flex items-start gap-5">
        {/* Left: tabs + content */}
        <div className="min-w-0 flex-1">
          {/* Tabs */}
          <div className="mb-5 flex gap-0 border-b border-[var(--color-border-default)]">
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id
              return (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => switchTab(tab.id)}
                  className={cn(
                    '-mb-px cursor-pointer border-b-2 border-b-transparent bg-none px-[18px] py-2.5 text-sm transition-all duration-150',
                    isActive
                      ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]'
                      : 'font-normal text-[var(--color-text-secondary)]'
                  )}
                >
                  {tab.label}
                </button>
              )
            })}
          </div>
          {tabGuardError && (
            <div className="mb-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]">
              {tabGuardError}
            </div>
          )}

          {/* Tab content */}
          <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            {activeTab === 'artifacts' && (
              <>
                <ToolSelector
                  label={t('stackInstall.labels.sourceRepository', 'Source Repository')}
                  options={ARTIFACTS_OPTIONS.sourceRepository}
                  value={draft.artifacts.sourceRepository}
                  onChange={(v) => setTool('artifacts', 'sourceRepository', v)}
                />
                <ToolSelector
                  label={t('stackInstall.labels.containerRegistry', 'Container Registry')}
                  options={ARTIFACTS_OPTIONS.containerRegistry}
                  value={draft.artifacts.containerRegistry}
                  onChange={(v) => setTool('artifacts', 'containerRegistry', v)}
                />
                <ToolSelector
                  label={t('stackInstall.labels.packageRegistry', 'Package Registry')}
                  options={ARTIFACTS_OPTIONS.packageRegistry}
                  value={draft.artifacts.packageRegistry}
                  onChange={(v) => setTool('artifacts', 'packageRegistry', v)}
                />
              </>
            )}

            {activeTab === 'pipeline' && (
              <>
                <ToolSelector
                  label={t('stackInstall.labels.cicdPlatform', 'CI/CD Platform')}
                  options={PIPELINE_OPTIONS.cicdPlatform}
                  value={draft.pipeline.cicdPlatform}
                  onChange={(v) => setTool('pipeline', 'cicdPlatform', v)}
                />
                <ToolSelector
                  label={t('stackInstall.labels.cdTool', 'CD Tool')}
                  options={PIPELINE_OPTIONS.cdTool}
                  value={draft.pipeline.cdTool}
                  onChange={(v) => setTool('pipeline', 'cdTool', v)}
                />
              </>
            )}

            {activeTab === 'monitoring' && (
              <>
                <MultiToolSelector
                  label={t('stackInstall.labels.visualization', 'Visualization')}
                  options={MONITORING_OPTIONS.visualization}
                  values={draft.monitoring.visualizations}
                  onChange={setMonitoringVisualizations}
                />
                <ToolSelector
                  label={t('stackInstall.labels.metrics', 'Metrics')}
                  options={MONITORING_OPTIONS.collection}
                  value={draft.monitoring.collection}
                  onChange={(v) => setTool('monitoring', 'collection', v)}
                />
                <ToolSelector
                  label={t('stackInstall.labels.logs', 'Logs')}
                  options={LOGGING_OPTIONS.search}
                  value={draft.logging.search}
                  onChange={(v) => setTool('logging', 'search', v)}
                />
                <ToolSelector
                  label={t('stackInstall.labels.traces', 'Traces')}
                  options={MONITORING_OPTIONS.traceLayer}
                  value={draft.logging.traceLayer}
                  onChange={(v) => setTool('logging', 'traceLayer', v)}
                />
                <ToolSelector
                  label={t('stackInstall.labels.traceExporter', 'Exporter / Agent')}
                  options={MONITORING_OPTIONS.traceExporter}
                  value={draft.logging.traceExporter}
                  onChange={(v) => setTool('logging', 'traceExporter', v)}
                />
              </>
            )}

            {activeTab === 'authentication' && (
              <>
                <ToolSelector
                  label={t('stackInstall.authentication.title', 'Authentication')}
                  options={AUTHENTICATION_OPTIONS}
                  value={{ tool: draft.authentication.provider, version: draft.authentication.provider ? 'latest' : '' }}
                  onChange={(v) => setAuthenticationProvider((v.tool as '' | 'openbao') || '')}
                />
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  {t(
                    'stackInstall.authentication.sharedNotice',
                    'OpenBao를 선택하면 현재 스택에 포함된 모든 OSS가 공통 인증 공급자(OpenBao)를 공유합니다. 미선택 시 기존 방식으로 배포됩니다.',
                  )}
                </p>
              </>
            )}

            {activeTab === 'manifests' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  선택한 OSS별 설치 파일입니다. Helm은 실제 <code>values.yaml</code>, YAML 타입은 배포 가능한 Kubernetes manifest 형식으로 생성됩니다.
                  문법/필수 항목 검증을 통과하면 이전 탭 설정을 오버라이드합니다.
                </p>

                <div className="mb-3 grid gap-3 md:grid-cols-2">
                  <div>
                    <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">Gateway</div>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => setActiveManifestTool(GATEWAY_MANIFEST_ID)}
                        className={cn(
                          'inline-flex items-center gap-2 rounded-lg border px-3 py-[7px] text-xs',
                          resolvedActiveManifestTool === GATEWAY_MANIFEST_ID
                            ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)] text-[#a5b4fc]'
                            : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-primary)]'
                        )}
                      >
                        <span className="font-semibold">Gateway</span>
                        <span className="rounded border border-[var(--color-border-default)] px-1.5 py-0.5 text-[10px] uppercase text-[var(--color-text-secondary)]">yaml</span>
                      </button>
                    </div>
                  </div>

                  <div>
                    <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">OSS</div>
                    {manifestTools.length === 0 ? (
                      <div className="rounded border border-[rgba(251,191,36,0.35)] bg-[rgba(251,191,36,0.08)] px-3 py-2 text-xs text-[#fcd34d]">
                        설치 대상 OSS가 없습니다. Gateway YAML은 자동 생성되며, OSS 설치파일은 툴 선택 후 생성됩니다.
                      </div>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {manifestTools.map((tool) => {
                          const isActive = resolvedActiveManifestTool === tool.toolId
                          return (
                            <button
                              key={tool.toolId}
                              type="button"
                              onClick={() => setActiveManifestTool(tool.toolId)}
                              className={cn(
                                'inline-flex items-center gap-2 rounded-lg border px-3 py-[7px] text-xs',
                                isActive
                                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)] text-[#a5b4fc]'
                                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-primary)]'
                              )}
                            >
                              <span className="font-semibold">{tool.toolLabel}</span>
                              <span className="rounded border border-[var(--color-border-default)] px-1.5 py-0.5 text-[10px] uppercase text-[var(--color-text-secondary)]">
                                {tool.installType}
                              </span>
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                </div>

                {resolvedActiveManifestTool && (
                  <>
                        {activeManifestInfo && (
                          <div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(99,102,241,0.08)] p-3 text-xs">
                            <div className="mb-1 flex flex-wrap items-center gap-2">
                              <span className="font-semibold text-[#a5b4fc]">{activeManifestInfo.toolLabel}</span>
                              <span className="rounded border border-[var(--color-border-default)] px-1.5 py-0.5 uppercase text-[10px] text-[var(--color-text-secondary)]">
                                {activeManifestInfo.installType}
                              </span>
                              {manifestErrorsByTool[activeManifestInfo.toolId] && (
                                <span className="rounded border border-[rgba(239,68,68,0.4)] bg-[rgba(239,68,68,0.15)] px-1.5 py-0.5 text-[10px] font-semibold text-[#fca5a5]">
                                  STRICT 검증 실패
                                </span>
                              )}
                              <span className="text-[var(--color-text-secondary)]">
                                app version: {activeManifestInfo.toolVersion || getToolAppVersion(activeManifestInfo.toolId)}
                                {activeManifestInfo.installType === 'helm' && activeManifestInfo.chartVersion
                                  ? ` / chart version: ${activeManifestInfo.chartVersion}`
                                  : ''}
                              </span>
                            </div>
                            <div className="text-[var(--color-text-secondary)]">역할: {activeManifestInfo.roles.join(', ')}</div>
                            {activeManifestInfo.toolId !== GATEWAY_MANIFEST_ID && (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                포함 OSS: {activeManifestInfo.sourceToolIds.map((id) => toolLabel(id, noneLabel)).join(', ')}
                              </div>
                            )}
                            {activeManifestInfo.hasVersionConflict && activeManifestInfo.toolId !== GATEWAY_MANIFEST_ID && (
                              <div className="mt-1 text-[#fcd34d]">
                                주의: 포함된 OSS들의 선택 버전이 달라 단일 값으로 통합되었습니다({activeManifestInfo.toolVersion}).
                              </div>
                            )}
                            {activeManifestInfo.toolId === GATEWAY_MANIFEST_ID ? (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                Gateway API YAML은 선택된 OSS 기준으로 Gateway/HTTPRoute를 자동 구성합니다. Access Domain TLS 인증서 적용을 켜면 HTTPS(443) + cert-manager Certificate + tls.certificateRefs(secret)가 함께 생성됩니다.
                              </div>
                            ) : (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                동일 OSS가 여러 역할에 선택돼도 설치 파일은 하나로 통합되어 생성됩니다.
                              </div>
                            )}
                            {activeManifestInfo.toolId !== GATEWAY_MANIFEST_ID && (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                버전 정책: <span className="font-semibold">Strict 고정</span> (카탈로그 app/chart 버전과 불일치하면 검증에서 차단됩니다)
                              </div>
                            )}
                          </div>
                        )}

                        <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] p-2">
                          <Editor
                            beforeMount={handleMonacoBeforeMount}
                            height="520px"
                            language="yaml"
                            theme={isDarkMode ? 'vs-dark' : 'vs-light'}
                            value={manifestDraftByTool[resolvedActiveManifestTool] ?? manifestOverridesByTool[resolvedActiveManifestTool] ?? defaultManifestByTool[resolvedActiveManifestTool] ?? ''}
                            onChange={(value) => handleManifestChange(resolvedActiveManifestTool, value)}
                            options={{
                              minimap: { enabled: false },
                              fontSize: 13,
                              lineNumbers: 'on',
                              scrollBeyondLastLine: false,
                              wordWrap: 'on',
                              tabSize: 2,
                            }}
                          />
                        </div>
                        {manifestErrorsByTool[resolvedActiveManifestTool] && (
                          <div className="mt-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]">
                            {manifestErrorsByTool[resolvedActiveManifestTool]}
                          </div>
                        )}
                  </>
                )}
              </div>
            )}

            {activeTab === 'deploy-script' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  현재 선택된 YAML View(OSS별 설치 파일), 버전, 네임스페이스, 스토리지 설정을 기반으로 생성된 배포 스크립트입니다.
                  Helm 항목은 <code>values.yaml</code> 파일을 EOF로 생성한 뒤 <code>helm upgrade --install -f</code>로 적용합니다.
                </p>
                <CodePreview
                  code={deployScript}
                  language="bash"
                  title={`${draft.stackName || 'nullus-stack'}-deploy.sh`}
                  maxHeight="560px"
                />
              </div>
            )}

            {activeTab === 'dry-run' && (
              <div className="space-y-4">
                <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] p-3">
                  <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Dry Run — 배포 전 최종 검토</h3>
                      <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                        필수 항목, YAML 검증, 리소스/스토리지 상태를 점검하고 배포 준비 여부를 확인합니다.
                      </p>
                    </div>
                    <Button variant="outline" size="sm" type="button" onClick={runDryRunChecks}>
                      Run Dry Run
                    </Button>
                  </div>

                  <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                    <div className="rounded border border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)] px-3 py-2 text-xs">
                      <div className="text-[var(--color-text-secondary)]">PASS</div>
                      <div className="font-semibold text-[#86efac]">{dryRunSummary.passed}</div>
                    </div>
                    <div className="rounded border border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)] px-3 py-2 text-xs">
                      <div className="text-[var(--color-text-secondary)]">WARN</div>
                      <div className="font-semibold text-[#fcd34d]">{dryRunSummary.warned}</div>
                    </div>
                    <div className="rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs">
                      <div className="text-[var(--color-text-secondary)]">FAIL</div>
                      <div className="font-semibold text-[#fca5a5]">{dryRunSummary.failed}</div>
                    </div>
                    <div
                      className={cn(
                        'rounded border px-3 py-2 text-xs',
                        dryRunSummary.readyToDeploy
                          ? 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)]'
                          : 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)]'
                      )}
                    >
                      <div className="text-[var(--color-text-secondary)]">READY</div>
                      <div className={cn('font-semibold', dryRunSummary.readyToDeploy ? 'text-[#86efac]' : 'text-[#fca5a5]')}>
                        {dryRunSummary.readyToDeploy ? 'YES' : 'NO'}
                      </div>
                    </div>
                  </div>

                  {dryRunExecutedAt && (
                    <div className="mt-2 text-[11px] text-[var(--color-text-secondary)]">last run: {dryRunExecutedAt}</div>
                  )}
                </div>

                <div className="space-y-2">
                  {dryRunChecks.map((check) => (
                    <div
                      key={check.id}
                      className={cn(
                        'rounded-lg border px-3 py-2',
                        check.status === 'pass' && 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)]',
                        check.status === 'warn' && 'border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)]',
                        check.status === 'fail' && 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)]'
                      )}
                    >
                      <div className="mb-1 flex items-center justify-between gap-2">
                        <span className="text-sm font-semibold text-[var(--color-text-primary)]">{check.title}</span>
                        <span
                          className={cn(
                            'rounded px-2 py-0.5 text-[10px] font-bold uppercase',
                            check.status === 'pass' && 'bg-[rgba(34,197,94,0.2)] text-[#86efac]',
                            check.status === 'warn' && 'bg-[rgba(245,158,11,0.2)] text-[#fcd34d]',
                            check.status === 'fail' && 'bg-[rgba(239,68,68,0.2)] text-[#fca5a5]'
                          )}
                        >
                          {check.status}
                        </span>
                      </div>
                      <div className="text-xs text-[var(--color-text-secondary)]">{check.detail}</div>
                    </div>
                  ))}
                </div>

              </div>
            )}

            {activeTab === 'resources' && (
              <div>
                <div className="space-y-4">
                  <div className="flex flex-wrap items-end justify-between gap-3">
                    <div>
                      <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">OSS별 Resource Planning</h3>
                      <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                        각 OSS별 세부 옵션을 변경하면 추천값이 재계산되고 적용값은 추천값으로 재설정됩니다.
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-[11px] text-[var(--color-text-secondary)]">Sizing Profile</span>
                      <NativeSelect
                        value={selectedOrgProfileId ? `org:${selectedOrgProfileId}` : planningProfile}
                        onChange={(e) => handleSizingSelectChange(e.target.value)}
                        className="min-w-[160px] rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs"
                      >
                        {PLANNING_PROFILES.map((profile) => (
                          <option key={profile} value={profile}>
                            {PLANNING_PROFILE_LABEL[profile]}
                          </option>
                        ))}
                        {orgProfiles.length > 0 && (
                          <optgroup label="Organization Profiles">
                            {orgProfiles.map((p) => (
                              <option key={p.id} value={`org:${p.id}`}>
                                {p.name}
                              </option>
                            ))}
                          </optgroup>
                        )}
                      </NativeSelect>
                      <button
                        type="button"
                        title="Save current values to selected profile"
                        onClick={handleSaveProfileButtonClick}
                        disabled={createOrgProfile.isPending || updateOrgProfile.isPending}
                        className="inline-flex h-6 w-6 items-center justify-center rounded border border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[rgba(99,102,241,0.5)] hover:text-[#a5b4fc]"
                      >
                        <Save size={12} />
                      </button>
                      {selectedOrgProfileId && (
                        <button
                          type="button"
                          title="Delete this organization profile"
                          onClick={handleDeleteOrgProfile}
                          className="inline-flex h-6 w-6 items-center justify-center rounded border border-[rgba(239,68,68,0.3)] text-[#f87171] hover:bg-[rgba(239,68,68,0.1)]"
                        >
                          <Trash2 size={12} />
                        </button>
                      )}
                    </div>
                  </div>

                  {saveProfileDialogOpen && (
                    <div className="mb-4 flex items-center gap-2 rounded-lg border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.06)] px-3 py-2">
                      <span className="text-[11px] text-[var(--color-text-secondary)] shrink-0">Profile name</span>
                      <input
                        type="text"
                        value={saveProfileName}
                        onChange={(e) => setSaveProfileName(e.target.value)}
                        onKeyDown={(e) => { if (e.key === 'Enter') handleSaveProfileConfirm(); if (e.key === 'Escape') setSaveProfileDialogOpen(false) }}
                        placeholder="e.g. Production-M"
                        autoFocus
                        className="flex-1 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:border-[rgba(99,102,241,0.5)]"
                      />
                      <button
                        type="button"
                        onClick={handleSaveProfileConfirm}
                        disabled={!saveProfileName.trim() || createOrgProfile.isPending}
                        className="rounded border border-[rgba(99,102,241,0.4)] bg-[rgba(99,102,241,0.15)] px-2.5 py-1 text-[11px] font-semibold text-[#a5b4fc] disabled:opacity-50"
                      >
                        Save
                      </button>
                      <button
                        type="button"
                        onClick={() => setSaveProfileDialogOpen(false)}
                        className="rounded border border-[var(--color-border-default)] px-2.5 py-1 text-[11px] text-[var(--color-text-secondary)]"
                      >
                        Cancel
                      </button>
                    </div>
                  )}

                  <div className="mb-4 grid grid-cols-3 gap-3 rounded-lg border border-[rgba(99,102,241,0.2)] bg-[rgba(99,102,241,0.06)] p-3">
                    <div>
                      <div className="text-[11px] text-[var(--color-text-secondary)]">적용값 총 CPU (Req | Limit)</div>
                      <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.cpuRequest.toFixed(2)} | {planningAppliedTotal.cpuLimit.toFixed(2)}</div>
                    </div>
                    <div>
                      <div className="text-[11px] text-[var(--color-text-secondary)]">적용값 총 Memory (Gi)</div>
                      <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.memoryRequestGi.toFixed(2)} | {planningAppliedTotal.memoryLimitGi.toFixed(2)}</div>
                    </div>
                    <div>
                      <div className="text-[11px] text-[var(--color-text-secondary)]">적용값 총 Storage (Gi)</div>
                      <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.storageRequestGi.toFixed(2)} | {planningAppliedTotal.storageLimitGi.toFixed(2)}</div>
                    </div>
                  </div>

                  <div className="flex flex-col gap-3">
                    {planningRows.map((row) => (
                      <div key={row.rowKey} className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-3">
                        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                          <div>
                            <div className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">{row.category}</div>
                            <div className="flex items-center gap-1 text-sm font-bold text-[var(--color-text-primary)]">
                              <span>{row.toolLabel} 리소스 플래닝</span>
                              <button
                                type="button"
                                aria-label={`${row.toolLabel} 리소스 산정식 보기`}
                                onClick={() => setActiveFormulaPopoverKey((prev) => (prev === row.rowKey ? null : row.rowKey))}
                                className="inline-flex h-5 w-5 items-center justify-center rounded text-[var(--color-text-secondary)] hover:bg-[rgba(255,255,255,0.06)] hover:text-[var(--color-text-primary)]"
                              >
                                <Info size={13} />
                              </button>
                            </div>
                          </div>
                        </div>

                        {activeFormulaPopoverKey === row.rowKey && (
                          <div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
                            <div className="mb-2 flex items-center justify-between">
                              <div className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">산정식</div>
                              <button
                                type="button"
                                onClick={() => setActiveFormulaPopoverKey(null)}
                                className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                              >
                                닫기
                              </button>
                            </div>
                            <pre className="m-0 whitespace-pre-wrap break-words text-[11px] leading-5 text-[var(--color-text-secondary)]">
                              {buildFormulaTooltip(row.toolLabel, row.defs)}
                            </pre>
                          </div>
                        )}

                        {!row.recommended || !row.applied ? (
                          <div className="rounded border border-[rgba(251,191,36,0.35)] bg-[rgba(251,191,36,0.08)] px-3 py-2 text-xs text-[#fcd34d]">
                            해당 OSS의 default 리소스가 정의되지 않았습니다.
                          </div>
                        ) : (
                          <>
                            {row.multipliers && (row.multipliers.clamped.cpu || row.multipliers.clamped.memory || row.multipliers.clamped.storage) && (
                              <div className="mb-3 rounded border border-[rgba(251,191,36,0.35)] bg-[rgba(251,191,36,0.08)] px-3 py-2 text-xs text-[#fcd34d]">
                                계산 배수가 제한에 도달했습니다:
                                {row.multipliers.clamped.cpu ? ' CPU' : ''}
                                {row.multipliers.clamped.memory ? ' Memory' : ''}
                                {row.multipliers.clamped.storage ? ' Storage' : ''}
                                {' '} 
                                (최소 0.5x, 프로파일/슬롯별 상한 적용). 현재 입력에서는 추가 증가/감소가 추천값에 제한적으로 반영될 수 있습니다.
                              </div>
                            )}

                            <div className="mb-3 grid grid-cols-2 gap-3">
                              <div className="flex items-center gap-2 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
                                <span className="text-[11px] text-[var(--color-text-secondary)]">Memory 단위</span>
                                <NativeSelect
                                  value={row.units.memory}
                                  onChange={(e) => handlePlanningUnitChange(row.rowKey, 'memory', e.target.value as ResourceUnit)}
                                  className="max-w-[90px] rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs"
                                >
                                  <option value="Gi">Gi</option>
                                  <option value="Mi">Mi</option>
                                </NativeSelect>
                              </div>
                              <div className="flex items-center gap-2 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
                                <span className="text-[11px] text-[var(--color-text-secondary)]">Storage 단위</span>
                                <NativeSelect
                                  value={row.units.storage}
                                  onChange={(e) => handlePlanningUnitChange(row.rowKey, 'storage', e.target.value as ResourceUnit)}
                                  className="max-w-[90px] rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs"
                                >
                                  <option value="Gi">Gi</option>
                                  <option value="Mi">Mi</option>
                                </NativeSelect>
                              </div>
                            </div>

                            <div className="mb-3 grid grid-cols-2 gap-3">
                              {row.defs.map((def) => (
                                <div key={def.key} className="flex flex-col gap-1">
                                  <label className="text-[11px] text-[var(--color-text-secondary)]">{def.label}</label>
                                  <Input
                                    type="number"
                                    min={def.min}
                                    max={def.max}
                                    step={def.step}
                                    value={row.optionValues[def.key] ?? def.baseline}
                                    onChange={(e) => handlePlanningOptionChange(row.rowKey, def.key, Number(e.target.value))}
                                  />
                                </div>
                              ))}
                            </div>

                            <div className="grid grid-cols-2 gap-3 border-t border-[rgba(255,255,255,0.06)] pt-3">
                              <div className="p-1">
                                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">추천값 (읽기 전용)</div>
                                <div className="grid grid-cols-3 gap-2 text-sm">
                                  <div><div className="text-[11px] text-[var(--color-text-secondary)]">CPU</div><div className="font-semibold text-[#a5b4fc]">{row.recommended.cpuRequest.toFixed(1)} | {row.recommended.cpuLimit.toFixed(1)}</div></div>
                                  <div><div className="text-[11px] text-[var(--color-text-secondary)]">Memory</div><div className="font-semibold text-[#a5b4fc]">{convertGiToUnit(row.recommended.memoryRequestGi, row.units.memory).toFixed(1)} | {convertGiToUnit(row.recommended.memoryLimitGi, row.units.memory).toFixed(1)} {row.units.memory}</div></div>
                                  <div><div className="text-[11px] text-[var(--color-text-secondary)]">Storage</div><div className="font-semibold text-[#a5b4fc]">{convertGiToUnit(row.recommended.storageRequestGi, row.units.storage).toFixed(1)} | {convertGiToUnit(row.recommended.storageLimitGi, row.units.storage).toFixed(1)} {row.units.storage}</div></div>
                                </div>
                              </div>

                              <div className="p-1">
                                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">적용값 (수정 가능)</div>
                                <div className="grid grid-cols-3 gap-2 text-sm">
                                  <div className="flex flex-col gap-1">
                                    <span className="text-[11px] text-[var(--color-text-secondary)]">CPU (Req|Limit)</span>
                                    <div className="flex gap-1">
                                      <Input type="number" step="0.01" value={row.applied.cpuRequest} onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'cpuRequest', Number(e.target.value))} />
                                      <Input type="number" step="0.01" value={row.applied.cpuLimit} onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'cpuLimit', Number(e.target.value))} />
                                    </div>
                                  </div>
                                  <div className="flex flex-col gap-1">
                                    <span className="text-[11px] text-[var(--color-text-secondary)]">Memory (Req|Limit)</span>
                                    <div className="flex gap-1">
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.memoryRequestGi, row.units.memory)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'memoryRequestGi', convertUnitToGi(Number(e.target.value), row.units.memory))}
                                      />
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.memoryLimitGi, row.units.memory)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'memoryLimitGi', convertUnitToGi(Number(e.target.value), row.units.memory))}
                                      />
                                    </div>
                                  </div>
                                  <div className="flex flex-col gap-1">
                                    <span className="text-[11px] text-[var(--color-text-secondary)]">Storage (Req|Limit)</span>
                                    <div className="flex gap-1">
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.storageRequestGi, row.units.storage)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'storageRequestGi', convertUnitToGi(Number(e.target.value), row.units.storage))}
                                      />
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.storageLimitGi, row.units.storage)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'storageLimitGi', convertUnitToGi(Number(e.target.value), row.units.storage))}
                                      />
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </div>
                          </>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'storage' && (
              <div className="space-y-4">
                <div>
                  <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Storage Plan</h3>
                  <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                    DB(Postgres)와 Object Storage를 기존 연결 또는 통합 생성으로 선택할 수 있습니다.
                  </p>
                </div>

                <div className="grid gap-2">
                  {STORAGE_PLAN_MODE_OPTIONS.map((option) => {
                    const selected = draft.storage.planMode === option.id
                    return (
                      <button
                        key={option.id}
                        type="button"
                        onClick={() => handleStoragePlanModeChange(option.id)}
                        className={cn(
                          'w-full rounded-lg border px-3 py-2 text-left transition-all',
                          selected
                            ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                            : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
                        )}
                      >
                        <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                          {t(option.labelKey, option.labelDefault)}
                        </div>
                        <div className="mt-0.5 text-xs text-[var(--color-text-secondary)]">
                          {t(option.descriptionKey, option.descriptionDefault)}
                        </div>
                      </button>
                    )
                  })}
                </div>

                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  {([
                    { key: 'database', title: 'Database', target: draft.storage.database },
                    { key: 'objectStorage', title: 'Object Storage', target: draft.storage.objectStorage },
                  ] as const).map((item) => {
                    const targetKey = item.key
                    const effectiveMode = getStorageEffectiveMode()

                    const providerOptions = STORAGE_PROVIDER_OPTIONS[targetKey]
                    const existingRefError = getStorageFieldError(targetKey, 'existingRef')
                    const endpointError = getStorageFieldError(targetKey, 'endpoint')
                    const resourceNameError = getStorageFieldError(targetKey, 'resourceName')
                    const accessSecretRefError = getStorageFieldError(targetKey, 'accessSecretRef')
                    const authIdError = getStorageFieldError(targetKey, 'authId')
                    const authPasswordKeyError = getStorageFieldError(targetKey, 'authPasswordKey')

                    return (
                      <div
                        key={targetKey}
                        className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3"
                      >
                        <div className="mb-2 flex items-center justify-between">
                          <h4 className="m-0 text-sm font-semibold text-[var(--color-text-primary)]">{item.title}</h4>
                          <span className="rounded border border-[var(--color-border-default)] px-2 py-0.5 text-[10px] text-[var(--color-text-secondary)]">
                            {effectiveMode === 'existing'
                              ? t('stackInstall.storagePlan.badge.existing', 'Existing')
                              : effectiveMode === 'create'
                                ? t('stackInstall.storagePlan.badge.create', 'New')
                                : noneLabel}
                          </span>
                        </div>

                        {effectiveMode === null ? (
                          <div className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                            Storage Plan에서 연결 방식을 먼저 선택해 주세요.
                          </div>
                        ) : effectiveMode === 'existing' && (
                          <div className="mb-3 grid gap-2">
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">기존 리소스 참조 ID</label>
                            <Input
                              value={item.target.existingRef}
                              placeholder={targetKey === 'database' ? 'org-shared-postgres' : 'org-shared-object-storage'}
                              onChange={(e) => {
                                clearStorageFieldError(targetKey, 'existingRef')
                                updateStorageTarget(targetKey, { existingRef: e.target.value })
                              }}
                            />
                            {existingRefError && <span className="mt-1 block text-xs text-[#ef4444]">{existingRefError}</span>}
                          </div>
                          <div>
                            <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">엔드포인트</label>
                            <Input
                              value={item.target.endpoint}
                              placeholder={targetKey === 'database' ? 'postgres.shared.svc:5432' : 'http://minio.shared.svc:9000'}
                              onChange={(e) => {
                                clearStorageFieldError(targetKey, 'endpoint')
                                updateStorageTarget(targetKey, { endpoint: e.target.value })
                              }}
                            />
                            {endpointError && <span className="mt-1 block text-xs text-[#ef4444]">{endpointError}</span>}
                          </div>
                          <div>
                            <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 이름' : 'Bucket 이름'}</label>
                            <Input
                              value={item.target.resourceName}
                              placeholder={targetKey === 'database' ? 'nullus' : 'nullus-artifacts'}
                              onChange={(e) => {
                                clearStorageFieldError(targetKey, 'resourceName')
                                updateStorageTarget(targetKey, { resourceName: e.target.value })
                              }}
                            />
                            {resourceNameError && <span className="mt-1 block text-xs text-[#ef4444]">{resourceNameError}</span>}
                          </div>
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">접근 Secret Ref</label>
                              <Input
                                value={item.target.accessSecretRef}
                                placeholder={
                                  targetKey === 'database'
                                    ? 'shared-postgres-credentials'
                                    : 'shared-object-storage-credentials'
                                }
                                onChange={(e) => {
                                  clearStorageFieldError(targetKey, 'accessSecretRef')
                                  updateStorageTarget(targetKey, { accessSecretRef: e.target.value })
                                }}
                              />
                              {accessSecretRefError && <span className="mt-1 block text-xs text-[#ef4444]">{accessSecretRefError}</span>}
                            </div>
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 사용자 ID' : 'Access Key ID'}</label>
                              <Input
                                value={item.target.authId}
                                placeholder={targetKey === 'database' ? 'nullus_app' : 'nullus_access_key'}
                                onChange={(e) => {
                                  clearStorageFieldError(targetKey, 'authId')
                                  updateStorageTarget(targetKey, { authId: e.target.value })
                                }}
                              />
                              {authIdError && <span className="mt-1 block text-xs text-[#ef4444]">{authIdError}</span>}
                            </div>
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 비밀번호 Key' : 'Secret Key Key'}</label>
                              <Input
                                value={item.target.authPasswordKey}
                                placeholder={targetKey === 'database' ? 'password' : 'secretKey'}
                                onChange={(e) => {
                                  clearStorageFieldError(targetKey, 'authPasswordKey')
                                  updateStorageTarget(targetKey, { authPasswordKey: e.target.value })
                                }}
                              />
                              {authPasswordKeyError && <span className="mt-1 block text-xs text-[#ef4444]">{authPasswordKeyError}</span>}
                            </div>
                          </div>
                        )}

                        {effectiveMode !== null && (
                        <div className="mb-3">
                          <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 엔진' : 'Storage Provider'}</label>
                          <NativeSelect
                            value={item.target.providerOrEngine}
                            onChange={(e) => updateStorageTarget(targetKey, { providerOrEngine: e.target.value })}
                            className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[7px] text-xs"
                          >
                            {providerOptions.map((provider) => (
                              <option key={provider.id} value={provider.id}>
                                {provider.label}
                              </option>
                            ))}
                          </NativeSelect>
                        </div>
                        )}

                        {effectiveMode !== null && (
                        <div className={cn('grid gap-2', effectiveMode === 'create' ? 'grid-cols-2' : 'grid-cols-1')}>
                          <div>
                            <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">버전</label>
                            <Input
                              value={item.target.version}
                              placeholder={targetKey === 'database' ? '16' : 'latest'}
                              onChange={(e) => updateStorageTarget(targetKey, { version: e.target.value })}
                            />
                          </div>
                          {effectiveMode === 'create' && (
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">사이즈</label>
                              <NativeSelect
                                value={item.target.size}
                                onChange={(e) =>
                                  updateStorageTarget(targetKey, {
                                    size: e.target.value as StorageTargetConfig['size'],
                                  })
                                }
                                className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[7px] text-xs"
                              >
                                {STORAGE_SIZE_OPTIONS.map((size) => (
                                  <option key={size} value={size}>
                                    {`${size} ${STORAGE_SIZE_RESOURCE_HINTS[targetKey][size]}`}
                                  </option>
                                ))}
                              </NativeSelect>
                            </div>
                          )}
                        </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Right: Configuration Summary */}
        <div className="sticky top-6 w-[260px] shrink-0 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <h3 className="mb-[14px] mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-primary)]">
            Configuration Summary
          </h3>
          {[
            ['Template', draft.selectedTemplateId ?? '—'],
            ['Stack Name', draft.stackName || '—'],
            ['Access Domain', draft.accessDomain || `${draft.stackName || 'nullus-stack'}.internal`],
            [
              'Access TLS',
              draft.accessDomainTls.enabled
                ? `enabled (${draft.accessDomainTls.secretNamespace || 'nullus'}/${draft.accessDomainTls.secretName || 'nullus-wildcard-tls'}, issuer=${draft.accessDomainTls.issuerName || 'nullus-ca-issuer'})`
                : 'disabled',
            ],
            ['Source Repo', draft.artifacts.sourceRepository.tool],
            ['Container Registry', draft.artifacts.containerRegistry.tool],
            ['Package Registry', draft.artifacts.packageRegistry.tool],
            ['Storage', objectStorageBackendTool],
            ['CI/CD', draft.pipeline.cicdPlatform.tool],
            ['CD Tool', draft.pipeline.cdTool.tool],
            ['Visualization', selectedVisualizations.map((item) => item.tool).join(', ') || noneLabel],
            ['Metrics', draft.monitoring.collection.tool],
            ['Logs', draft.logging.search.tool],
            ['Traces', draft.logging.traceLayer.tool],
            ['Exporter/Agent', draft.logging.traceExporter.tool],
            ['Storage Plan', draft.storage.planMode === 'none' ? noneLabel : draft.storage.planMode],
            [
              'Database',
              `${draft.storage.database.mode}:${draft.storage.database.providerOrEngine || noneLabel}${draft.storage.database.mode === 'create' ? `/${draft.storage.database.size}` : ''}`,
            ],
            ['DB Ref', `${draft.storage.database.existingRef || '-'} @ ${draft.storage.database.endpoint || '-'}`],
            ['DB Auth', `${draft.storage.database.authId || '-'} (${draft.storage.database.authPasswordKey || '-'})`],
            [
              'Object Storage',
              `${draft.storage.objectStorage.mode}:${draft.storage.objectStorage.providerOrEngine}${draft.storage.objectStorage.mode === 'create' ? `/${draft.storage.objectStorage.size}` : ''}`,
            ],
            ['Object Ref', `${draft.storage.objectStorage.existingRef || '-'} @ ${draft.storage.objectStorage.endpoint || '-'}`],
            ['Object Auth', `${draft.storage.objectStorage.authId || '-'} (${draft.storage.objectStorage.authPasswordKey || '-'})`],
          ].map(([label, val]) => (
            <div
              key={label}
              className="flex items-baseline justify-between gap-2 border-b border-[rgba(255,255,255,0.04)] py-1.5"
            >
              <span className="shrink-0 text-[11px] text-[var(--color-text-secondary)]">{label}</span>
              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-right text-xs font-semibold text-[var(--color-text-primary)]">
                {typeof val === 'string' && val.length === 0 ? noneLabel : val}
              </span>
            </div>
          ))}
        </div>
      </div>

    </div>
  )
}

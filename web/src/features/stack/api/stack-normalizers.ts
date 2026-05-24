import type {
  CompatibilityMatrix,
  CompatibilityValidationResult,
  Stack,
  StackHistoryEntry,
  StackTemplate,
  TemplateToolDetail,
  CreateStackRequest,
} from '../../../types'
import type { MatrixInput } from './stack-api-types'

export interface RawTemplate {
  id: string
  ID?: string
  name: string
  Name?: string
  description: string
  Description?: string
  tools?: unknown[]
  Tools?: unknown[]
  estimatedMinutes?: number
  estimated_install_time?: number
  EstimatedInstallTime?: number
  category?: string
  Category?: string
  createdBy?: string
  created_by?: string
  CreatedBy?: string
  recommendedUseCase?: string
  recommended_use_case?: string
  RecommendedUseCase?: string
  minResources?: string
  min_resources?: string
  MinResources?: string
}

interface RawCompatibilityTool {
  name?: string
  Name?: string
  helmVersion?: string
  helm_version?: string
  HelmVersion?: string
  appVersion?: string
  app_version?: string
  AppVersion?: string
  archSupport?: string[]
  arch_support?: string[]
  ArchSupport?: string[]
  minK8sVersion?: string
  min_k8s_version?: string
  MinK8sVersion?: string
  tier?: string
  Tier?: string
}

interface RawKubernetesRange {
  min?: string
  Min?: string
  max?: string
  Max?: string
  recommended?: string
  Recommended?: string
}

export interface RawCompatibilityMatrix {
  id?: string
  ID?: string
  name?: string
  Name?: string
  status?: string
  Status?: string
  k8sRange?: string
  Kubernetes?: RawKubernetesRange
  kubernetes?: RawKubernetesRange
  tools?: RawCompatibilityTool[]
  Tools?: RawCompatibilityTool[] | Record<string, RawCompatibilityTool>
}

export interface RawStackItem {
  id?: string
  name?: string
  template_id?: string
  templateId?: string
  template_name?: string
  templateName?: string
  cluster_id?: string
  clusterId?: string
  cluster_name?: string
  clusterName?: string
  namespace?: string
  state?: string
  status?: string
  created_at?: string
  createdAt?: string
  updated_at?: string
  updatedAt?: string
  deleted_at?: string
  deletedAt?: string
}

export interface RawStackHistoryEntry {
  id?: string
  ID?: string
  stackId?: string
  StackID?: string
  version?: number
  Version?: number
  changedBy?: string
  ChangedBy?: string
  changedAt?: string
  CreatedAt?: string
  reason?: string
  changeReason?: string
  ChangeReason?: string
  snapshot?: Record<string, unknown>
  config?: Record<string, unknown>
  Config?: Record<string, unknown>
}

interface RawCompatibilityIssue {
  tool?: string
  message?: string
  severity?: string
  code?: string
}

interface RawCompatibilityOverall {
  state?: string
  score?: number
}

export interface RawCompatibilityValidationResult {
  compatible?: boolean
  overall?: RawCompatibilityOverall
  issues?: RawCompatibilityIssue[]
  node_architectures?: string[]
  nodeArchitectures?: string[]
  matrix?: RawCompatibilityMatrix | null
  message?: string
  checkedAt?: string
}

const toToolName = (tool: unknown): string => {
  if (typeof tool === 'string') {
    return tool
  }

  if (tool && typeof tool === 'object' && 'name' in tool) {
    const maybeName = (tool as { name?: unknown }).name
    return typeof maybeName === 'string' ? maybeName : ''
  }

  return ''
}

const toToolDetail = (tool: unknown): TemplateToolDetail | null => {
  if (!tool || typeof tool !== 'object') {
    return null
  }

  const record = tool as Record<string, unknown>
  const category = typeof record.category === 'string' ? record.category : ''
  const name =
    typeof record.name === 'string'
      ? record.name
      : (typeof record.Name === 'string' ? record.Name : '')
  const helmVersion =
    typeof record.helm_version === 'string'
      ? record.helm_version
      : (typeof record.HelmVersion === 'string' ? record.HelmVersion : '')
  const appVersion =
    typeof record.app_version === 'string'
      ? record.app_version
      : (typeof record.AppVersion === 'string' ? record.AppVersion : '')

  if (!name) {
    return null
  }

  return {
    category,
    name,
    helm_version: helmVersion,
    app_version: appVersion,
  }
}

export const normalizeTemplate = (raw: RawTemplate): StackTemplate => ({
  id: raw.id ?? raw.ID ?? '',
  name: raw.name ?? raw.Name ?? '',
  description: raw.description ?? raw.Description ?? '',
  tools: Array.isArray(raw.tools ?? raw.Tools) ? (raw.tools ?? raw.Tools ?? []).map(toToolName).filter((tool) => tool.length > 0) : [],
  toolDetails: Array.isArray(raw.tools ?? raw.Tools)
    ? (raw.tools ?? raw.Tools ?? []).map(toToolDetail).filter((detail): detail is TemplateToolDetail => detail !== null)
    : [],
  estimatedMinutes: typeof raw.estimatedMinutes === 'number'
    ? raw.estimatedMinutes
    : (typeof (raw.estimated_install_time ?? raw.EstimatedInstallTime) === 'number'
      ? Math.round((raw.estimated_install_time ?? raw.EstimatedInstallTime ?? 1800000000000) / 60000000000)
      : 30),
  category: raw.category ?? raw.Category ?? 'default',
  createdBy: raw.createdBy ?? raw.created_by ?? raw.CreatedBy,
  recommendedUseCase: raw.recommendedUseCase ?? raw.recommended_use_case ?? raw.RecommendedUseCase,
  minResources: raw.minResources ?? raw.min_resources ?? raw.MinResources,
})

const normalizeArchs = (archs: string[] | undefined): string[] => {
  if (!Array.isArray(archs) || archs.length === 0) {
    return []
  }
  const seen = new Set<string>()
  const out: string[] = []
  for (const arch of archs) {
    if (typeof arch === 'string' && arch.length > 0 && !seen.has(arch)) {
      seen.add(arch)
      out.push(arch)
    }
  }
  out.sort()
  return out
}

const normalizeTier = (raw: string | undefined): import('../../../types').CompatibilityTier => {
  if (raw === 'beta' || raw === 'deprecated') {
    return raw
  }
  return 'stable'
}

const normalizeCompatibilityTool = (tool: RawCompatibilityTool) => ({
  name: tool.name ?? tool.Name ?? 'Unknown',
  helmVersion: tool.helmVersion ?? tool.helm_version ?? tool.HelmVersion ?? '-',
  appVersion: tool.appVersion ?? tool.app_version ?? tool.AppVersion ?? '-',
  archSupport: normalizeArchs(tool.archSupport ?? tool.arch_support ?? tool.ArchSupport),
  minK8sVersion: tool.minK8sVersion ?? tool.min_k8s_version ?? tool.MinK8sVersion ?? '',
  tier: normalizeTier(tool.tier ?? tool.Tier),
})

const normalizeK8sRange = (raw: RawCompatibilityMatrix): string => {
  if (raw.k8sRange) {
    return raw.k8sRange
  }

  const kubernetes = raw.kubernetes ?? raw.Kubernetes
  if (!kubernetes) {
    return 'N/A'
  }

  const min = kubernetes.min ?? kubernetes.Min
  const max = kubernetes.max ?? kubernetes.Max
  if (!min && !max) {
    return 'N/A'
  }
  if (min && max && min !== max) {
    return `${min}-${max}`
  }
  return min ?? max ?? 'N/A'
}

export const normalizeCompatibilityMatrix = (raw: RawCompatibilityMatrix): CompatibilityMatrix => {
  const rawTools = raw.tools ?? raw.Tools
  const tools = Array.isArray(rawTools)
    ? rawTools.map(normalizeCompatibilityTool)
    : Object.values(rawTools ?? {}).map(normalizeCompatibilityTool)

  const status = raw.status ?? raw.Status
  const normalizedStatus: CompatibilityMatrix['status'] = status === 'verified' || status === 'unsupported' ? status : 'untested'

  return {
    id: raw.id ?? raw.ID ?? '',
    name: raw.name ?? raw.Name ?? 'Unnamed Matrix',
    status: normalizedStatus,
    k8sRange: normalizeK8sRange(raw),
    tools,
  }
}

export const normalizeStackItem = (raw: RawStackItem): Stack => {
  const deletedAt = raw.deleted_at ?? raw.deletedAt ?? ''
  const status = deletedAt ? 'deleted' : (raw.state ?? raw.status ?? 'pending')
  return {
    id: raw.id ?? '',
    name: raw.name ?? '',
    templateId: raw.template_id ?? raw.templateId ?? '',
    templateName: raw.template_name ?? raw.templateName ?? raw.template_id ?? '',
    clusterId: raw.cluster_id ?? raw.clusterId ?? '',
    clusterName: raw.cluster_name ?? raw.clusterName ?? raw.cluster_id ?? '',
    namespace: raw.namespace ?? 'nullus',
    status: status as Stack['status'],
    createdAt: raw.created_at ?? raw.createdAt ?? '',
    updatedAt: raw.updated_at ?? raw.updatedAt ?? '',
    deletedAt: deletedAt || undefined,
  }
}

export const normalizeStackHistoryEntry = (raw: RawStackHistoryEntry): StackHistoryEntry => ({
  id: raw.id ?? raw.ID ?? '',
  stackId: raw.stackId ?? raw.StackID ?? '',
  version: raw.version ?? raw.Version ?? 0,
  changedBy: raw.changedBy ?? raw.ChangedBy ?? 'system',
  changedAt: raw.changedAt ?? raw.CreatedAt ?? '',
  reason: raw.reason ?? raw.changeReason ?? raw.ChangeReason ?? '',
  snapshot: raw.snapshot ?? raw.config ?? raw.Config ?? {},
})


export const normalizeCompatibilityValidationResult = (raw: RawCompatibilityValidationResult): CompatibilityValidationResult => {
  const state = raw.overall?.state
  const normalizedState: CompatibilityValidationResult['overall']['state'] =
    state === 'pass' || state === 'warn' || state === 'fail'
      ? state
      : (raw.compatible ? 'pass' : 'fail')

  const matrix = raw.matrix ? normalizeCompatibilityMatrix(raw.matrix) : undefined

  return {
    compatible: raw.compatible ?? (normalizedState !== 'fail'),
    overall: {
      state: normalizedState,
      score: typeof raw.overall?.score === 'number'
        ? raw.overall.score
        : (normalizedState === 'pass' ? 100 : (normalizedState === 'warn' ? 70 : 0)),
    },
    issues: Array.isArray(raw.issues)
      ? raw.issues.map((issue) => ({
          tool: issue.tool ?? 'matrix',
          message: issue.message ?? 'Compatibility issue detected',
          severity: issue.severity === 'error' ? 'error' : 'warning',
          code: issue.code,
        }))
      : [],
    nodeArchitectures: normalizeArchs(raw.node_architectures ?? raw.nodeArchitectures),
    matrix,
    message: raw.message,
    checkedAt: raw.checkedAt ?? new Date().toISOString(),
  }
}

export function matrixInputToPayload(input: MatrixInput): unknown {
  const tools: Record<string, unknown> = {}
  for (const [cat, t] of Object.entries(input.tools)) {
    tools[cat] = {
      name: t.name,
      helm_version: t.helmVersion,
      app_version: t.appVersion,
      min_k8s_version: t.minK8sVersion ?? '',
      arch_support: t.archSupport ?? ['amd64'],
      tier: t.tier ?? 'stable',
    }
  }
  return {
    id: input.id,
    name: input.name,
    status: input.status,
    kubernetes: input.kubernetes,
    tools,
  }
}

function toBackendTool(sel: { tool: string; version: string }) {
  const name = (sel.tool ?? '').trim()
  return { name, version: sel.version, enabled: name.length > 0 }
}

function toBackendStoragePlanMode(planMode: string): 'integrated-create' | 'existing-connect' | null {
  if (planMode === 'integrated-create') return 'integrated-create'
  if (planMode === 'existing-all') return 'existing-connect'
  return null
}

function toBackendStorageTargetMode(mode: string): 'create' | 'existing-connect' {
  return mode === 'existing' ? 'existing-connect' : 'create'
}

function toStorageSizeGi(target: 'database' | 'objectStorage', size: 'small' | 'medium' | 'large'): number {
  if (target === 'database') {
    if (size === 'small') return 20
    if (size === 'medium') return 50
    return 100
  }

  if (size === 'small') return 50
  if (size === 'medium') return 100
  return 300
}

export function toCreateStackBody(req: CreateStackRequest) {
  const a = req.artifacts as Record<string, { tool: string; version: string }>
  const p = req.pipeline as Record<string, { tool: string; version: string }>
  const m = req.monitoring as Record<string, { tool: string; version: string }>
  const l = req.logging as Record<string, { tool: string; version: string }>
  const monitoringWithMulti = req.monitoring as Record<string, unknown>
  const visualizationMulti = Array.isArray(monitoringWithMulti.visualizations)
    ? (monitoringWithMulti.visualizations as Array<{ tool?: string; version?: string }>)
        .filter((item) => typeof item?.tool === 'string' && item.tool.trim() !== '')
        .map((item) => ({ tool: item.tool ?? '', version: item.version ?? '' }))
    : []
  const primaryVisualization = visualizationMulti[0] ?? (m.visualization ?? { tool: '', version: '' })
  const backendStoragePlanMode = req.storage ? toBackendStoragePlanMode(req.storage.planMode) : null
  const storageBackendFromStorageTab = req.storage?.objectStorage?.providerOrEngine
    ? {
        tool: req.storage.objectStorage.providerOrEngine,
        version: req.storage.objectStorage.version || 'latest',
      }
    : (a.storageBackend ?? a.storage_backend ?? { tool: '', version: '' })
  return {
    name: req.stackName,
    cluster_id: req.clusterId ?? '',
    namespace: req.namespace || 'nullus',
    golden_path_id: req.templateId ?? '',
    config: {
      access_domain: req.accessDomain || `${req.stackName}.internal`,
      access_domain_tls: req.accessDomainTls
        ? {
            enabled: req.accessDomainTls.enabled,
            secret_name: req.accessDomainTls.secretName,
            secret_namespace: req.accessDomainTls.secretNamespace,
            issuer_name: req.accessDomainTls.issuerName,
          }
        : undefined,
      authentication: req.authentication?.provider
        ? { provider: req.authentication.provider }
        : undefined,
      yaml_overrides: req.yamlOverrides,
      artifacts: {
        package_registry: toBackendTool(a.packageRegistry ?? a.package_registry ?? { tool: '', version: '' }),
        source_repository: toBackendTool(a.sourceRepository ?? a.source_repository ?? { tool: '', version: '' }),
        container_registry: toBackendTool(a.containerRegistry ?? a.container_registry ?? { tool: '', version: '' }),
        storage_backend: toBackendTool(storageBackendFromStorageTab),
      },
      pipeline: {
        ci_platform: toBackendTool(p.cicdPlatform ?? p.ci_platform ?? { tool: '', version: '' }),
        cd_tool: toBackendTool(p.cdTool ?? p.cd_tool ?? { tool: '', version: '' }),
      },
      monitoring: {
        collection: toBackendTool(m.collection ?? { tool: '', version: '' }),
        visualization: toBackendTool(primaryVisualization),
        visualizations: visualizationMulti.map(toBackendTool),
      },
      logging: {
        collection: toBackendTool(l.collection ?? { tool: '', version: '' }),
        search: toBackendTool(l.search ?? { tool: '', version: '' }),
        trace_layer: toBackendTool(l.traceLayer ?? l.trace_layer ?? { tool: '', version: '' }),
        trace_exporter: toBackendTool(l.traceExporter ?? { tool: '', version: '' }),
      },
      resources: {
        developers: req.resources?.developerCount ?? 0,
        concurrent_runners: req.resources?.concurrentRunners ?? 0,
        weekly_commits: req.resources?.commitsPerDay ?? 0,
        build_frequency: req.resources?.buildFrequency ?? 'medium',
      },
      storage: req.storage && backendStoragePlanMode
        ? {
            plan_mode: backendStoragePlanMode,
            database: {
              mode: toBackendStorageTargetMode(req.storage.database.mode),
              existing_ref: req.storage.database.existingRef,
              endpoint: req.storage.database.endpoint,
              resource_name: req.storage.database.resourceName,
              access_secret_ref: req.storage.database.accessSecretRef,
              auth_id: req.storage.database.authId,
              auth_password_key: req.storage.database.authPasswordKey,
              provider_or_engine: req.storage.database.providerOrEngine,
              version: req.storage.database.version,
              size:
                req.storage.database.mode === 'create'
                  ? toStorageSizeGi('database', req.storage.database.size)
                  : undefined,
            },
            object_storage: {
              mode: toBackendStorageTargetMode(req.storage.objectStorage.mode),
              existing_ref: req.storage.objectStorage.existingRef,
              endpoint: req.storage.objectStorage.endpoint,
              resource_name: req.storage.objectStorage.resourceName,
              access_secret_ref: req.storage.objectStorage.accessSecretRef,
              auth_id: req.storage.objectStorage.authId,
              auth_password_key: req.storage.objectStorage.authPasswordKey,
              provider_or_engine: req.storage.objectStorage.providerOrEngine,
              version: req.storage.objectStorage.version,
              size:
                req.storage.objectStorage.mode === 'create'
                  ? toStorageSizeGi('objectStorage', req.storage.objectStorage.size)
                  : undefined,
            },
          }
        : undefined,
    },
  }
}

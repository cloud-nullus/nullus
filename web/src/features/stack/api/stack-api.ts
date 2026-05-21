import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import type {
  ClusterStatus,
  CompatibilityMatrix,
  CompatibilityValidationResult,
  CreateStackRequest,
  ResourceEstimate,
  RetryHistoryEntry,
  StackResourceDefault,
  Stack,
  StackHistoryEntry,
  StackTemplate,
  StackWorkloads,
  TemplateToolDetail,
  StackVersionDiff,
} from '../../../types'

export interface TemplateMutationRequest {
  id: string
  name: string
  description: string
  tools: unknown[]
  estimated_install_time: number
  recommended_use_case: string
  min_resources: string
}

export type {
  CompatibilityMatrix,
  CompatibilityValidationResult,
  CreateStackRequest,
  ResourceEstimate,
  RetryHistoryEntry,
  Stack,
  StackHistoryEntry,
  StackTemplate,
  StackVersionDiff,
} from '../../../types'

export interface ClusterSummary {
  id: string
  name: string
  connection_status: ClusterStatus
}

interface RawClusterSummary {
  id: string
  name: string
  connection_status?: ClusterStatus
  status?: ClusterStatus
}

interface RawClusterVerifyResult {
  status?: string
  version?: string
}

const queryKeys = {
  templates: () => ['stacks', 'templates'] as const,
  template: (id: string) => ['stacks', 'templates', id] as const,
  list: (filters?: Record<string, unknown>) => ['stacks', 'list', filters] as const,
  history: (stackId: string) => ['stacks', 'history', stackId] as const,
  monitoring: (stackId: string) => ['stacks', 'monitoring', stackId] as const,
  versionDiff: (stackId: string, from: number, to: number) => ['stacks', 'diff', stackId, from, to] as const,
  compatibilityMatrix: () => ['stacks', 'compatibility'] as const,
  clusters: () => ['clusters'] as const,
  workloads: (stackId: string) => ['stacks', 'workloads', stackId] as const,
  resourceDefaults: () => ['stacks', 'resource-defaults'] as const,
}

export interface PodMonitoringStatus {
  name: string
  phase: string
  ready: boolean
  restart_count: number
  node_name: string
  cpu_request_millicores: number
  cpu_limit_millicores: number
  cpu_usage_millicores: number
  memory_request_mib: number
  memory_limit_mib: number
  memory_usage_mib: number
  storage_request_gib?: number
  storage_limit_gib?: number
  storage_usage_gib?: number
  status: 'running' | 'warning' | 'error'
}

export interface OSSMonitoringStatus {
  key: string
  name: string
  version: string
  enabled: boolean
  status: 'running' | 'warning' | 'error'
  pod_count: number
  ready_pods: number
  pods: PodMonitoringStatus[]
}

export interface StackMonitoringSummary {
  total_pods: number
  ready_pods: number
  cpu_request_millicores: number
  cpu_limit_millicores: number
  cpu_usage_millicores: number
  memory_request_mib: number
  memory_limit_mib: number
  memory_usage_mib: number
  storage_request_gib: number
  storage_limit_gib: number
  storage_usage_gib: number
  storage_usage_available?: boolean
  usage_available: boolean
}

export interface InstalledResourceStatus {
  kind: string
  name: string
  desired_replicas: number
  ready_replicas: number
  available_replicas: number
  status: 'running' | 'warning' | 'error'
}

export interface StackMonitoringSnapshot {
  stack_id: string
  namespace: string
  timestamp: string
  summary: StackMonitoringSummary
  pod_status_counts: Array<{ name: string; count: number }>
  installed_resources: InstalledResourceStatus[]
  oss_statuses: OSSMonitoringStatus[]
}

interface RawTemplate {
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

interface RawCompatibilityMatrix {
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

interface RawStackItem {
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

interface RawStackHistoryEntry {
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

interface RawCompatibilityValidationResult {
  compatible?: boolean
  overall?: RawCompatibilityOverall
  issues?: RawCompatibilityIssue[]
  node_architectures?: string[]
  nodeArchitectures?: string[]
  matrix?: RawCompatibilityMatrix | null
  message?: string
  checkedAt?: string
}

export interface ValidateCompatibilityInput {
  stackId: string
  // clusterId tells the backend to resolve node architectures from the
  // admin module's cluster record (F8 Task 3). Takes precedence over
  // nodeArchitectures when both are set server-side.
  clusterId?: string
  // nodeArchitectures is the explicit override — useful in the wizard
  // before a stack row exists or when the caller already has the fleet
  // layout in hand.
  nodeArchitectures?: string[]
  // tools map is forwarded to the server's tool-based matrix matcher. If
  // omitted, the server falls back to its default Validate flow.
  tools?: Record<string, string>
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

const normalizeTemplate = (raw: RawTemplate): StackTemplate => ({
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

const normalizeCompatibilityMatrix = (raw: RawCompatibilityMatrix): CompatibilityMatrix => {
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

const normalizeStackItem = (raw: RawStackItem): Stack => {
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

const normalizeStackHistoryEntry = (raw: RawStackHistoryEntry): StackHistoryEntry => ({
  id: raw.id ?? raw.ID ?? '',
  stackId: raw.stackId ?? raw.StackID ?? '',
  version: raw.version ?? raw.Version ?? 0,
  changedBy: raw.changedBy ?? raw.ChangedBy ?? 'system',
  changedAt: raw.changedAt ?? raw.CreatedAt ?? '',
  reason: raw.reason ?? raw.changeReason ?? raw.ChangeReason ?? '',
  snapshot: raw.snapshot ?? raw.config ?? raw.Config ?? {},
})


const normalizeCompatibilityValidationResult = (raw: RawCompatibilityValidationResult): CompatibilityValidationResult => {
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

// --- API functions ---

// F8-Phase5 matrix CRUD input type. Mirrors the backend matrixPayload but
// uses camelCase on the TS side; `matrixInputToPayload` flips to snake_case
// for the wire.
export interface MatrixInput {
  id: string
  name: string
  status: 'verified' | 'untested' | 'unsupported'
  kubernetes: { min: string; max: string; recommended: string }
  tools: Record<string, {
    name: string
    helmVersion: string
    appVersion: string
    minK8sVersion?: string
    archSupport?: string[]
    tier?: 'stable' | 'beta' | 'deprecated'
  }>
}

function matrixInputToPayload(input: MatrixInput): unknown {
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

const stackApiCalls = {
  getTemplates: () =>
    api.get<RawTemplate[]>('/stacks/templates').then((r) => (r.data ?? []).map(normalizeTemplate)),

  getTemplate: (id: string) =>
    api.get<StackTemplate>(`/stacks/templates/${id}`).then((r) => r.data),

  getList: (filters?: { status?: string; search?: string; include_deleted?: boolean }) =>
    api.get<{ items: Stack[]; total: number }>('/stacks', { params: filters }).then((r) => ({
      ...r.data,
      items: ((r.data.items ?? []) as unknown as RawStackItem[]).map(normalizeStackItem),
    })),

  create: (request: CreateStackRequest) =>
    api.post<{ id: string }>('/stacks', toCreateStackBody(request)).then((r) => r.data),

  delete: (stackId: string) =>
    api.delete('/stacks/' + stackId).then((r) => r.data),

  saveDraft: (request: CreateStackRequest) =>
    api.post<{ draftId: string }>('/stacks/draft', request).then((r) => r.data),

  estimateResources: (input: CreateStackRequest['resources']) =>
    api.post<ResourceEstimate>('/stacks/estimate', input).then((r) => r.data),

  getResourceDefaults: () =>
    api
      .get<{ items: StackResourceDefault[]; total: number }>('/stacks/resource-defaults')
      .then((r) => r.data),

  upsertResourceDefault: (payload: Omit<StackResourceDefault, 'updated_at'>) =>
    api.post<StackResourceDefault>('/stacks/resource-defaults', payload).then((r) => r.data),

  getHistory: (stackId: string) =>
    api
      .get<RawStackHistoryEntry[]>(`/stacks/${stackId}/history`)
      .then((r) => (r.data ?? []).map(normalizeStackHistoryEntry)),

  getMonitoring: (stackId: string) =>
    api.get<StackMonitoringSnapshot>(`/stacks/${stackId}/monitoring`).then((r) => r.data),

  getVersionDiff: (stackId: string, from: number, to: number) =>
    api.get<StackVersionDiff>(`/stacks/${stackId}/history/diff`, { params: { versionA: from, versionB: to } }).then((r) => r.data),

   rollbackStack: (stackId: string, version: number, preservePVC: boolean) =>
     api.post<{ id: string }>(`/stacks/${stackId}/rollback`, { version, preservePVC }).then((r) => r.data),

  getCompatibilityMatrix: () =>
    api.get<RawCompatibilityMatrix[]>('/stacks/compatibility').then((r) => (r.data ?? []).map(normalizeCompatibilityMatrix)),

  // F8-Phase5 admin CRUD — create/update/delete compatibility matrices.
  // Wire body in snake_case to match the backend matrixPayload struct.
  createMatrix: (input: MatrixInput) =>
    api.post<RawCompatibilityMatrix>('/admin/compatibility/matrices', matrixInputToPayload(input))
      .then((r) => normalizeCompatibilityMatrix(r.data)),

  updateMatrix: (input: MatrixInput) =>
    api.put<RawCompatibilityMatrix>(`/admin/compatibility/matrices/${input.id}`, matrixInputToPayload(input))
      .then((r) => normalizeCompatibilityMatrix(r.data)),

  deleteMatrix: (id: string) =>
    api.delete<void>(`/admin/compatibility/matrices/${id}`).then(() => undefined),

  validateCompatibility: (input: ValidateCompatibilityInput) => {
    const { stackId, tools, clusterId, nodeArchitectures } = input
    const body: Record<string, unknown> = {}
    if (tools && Object.keys(tools).length > 0) {
      body.tools = tools
    }
    if (clusterId) {
      body.cluster_id = clusterId
    }
    if (nodeArchitectures && nodeArchitectures.length > 0) {
      body.node_architectures = nodeArchitectures
    }
    return api
      .post<RawCompatibilityValidationResult>(`/stacks/${stackId}/validate`, body)
      .then((r) => normalizeCompatibilityValidationResult(r.data ?? {}))
  },

  createTemplate: (request: TemplateMutationRequest) =>
    api.post<StackTemplate>('/stacks/templates', request).then((r) => r.data),

  updateTemplate: (request: TemplateMutationRequest) =>
    api.put<StackTemplate>(`/stacks/templates/${request.id}`, request).then((r) => r.data),

  deleteTemplate: (id: string) =>
    api.delete<void>(`/stacks/templates/${id}`).then((r) => r.data),

  getClusters: () =>
    api.get<{ items: RawClusterSummary[] }>('/admin/clusters').then((r) =>
      (r.data?.items ?? []).map((cluster) => ({
        id: cluster.id,
        name: cluster.name,
        connection_status: cluster.connection_status ?? cluster.status ?? 'pending',
      }))
    ),

  getClusterK8sVersion: (clusterId: string) =>
    api.post<RawClusterVerifyResult>(`/admin/clusters/${clusterId}/verify`).then((r) => (r.data?.version ?? '').trim()),

  deployStack: (input: DeployStackInput | string) => {
    const { stackId, acknowledgeWarnings } =
      typeof input === 'string' ? { stackId: input, acknowledgeWarnings: false } : input
    const body = acknowledgeWarnings ? { acknowledge_warnings: true } : undefined
    return api
      .post<{ stack_id: string; status: string }>(`/stacks/${stackId}/deploy`, body)
      .then((r) => r.data)
  },

  // retryStack — F8 follow-up Phase 3. Invokes POST /stacks/:id/retry to
  // rewind a failed/rolled_back stack to pending and re-run the install
  // pipeline. Same acknowledge_warnings contract as deployStack.
  retryStack: (input: DeployStackInput) => {
    const body = input.acknowledgeWarnings ? { acknowledge_warnings: true } : undefined
    return api
      .post<{ stack_id: string; status: string }>(`/stacks/${input.stackId}/retry`, body)
      .then((r) => r.data)
  },

  continueStack: (input: DeployStackInput) => {
    const body = input.acknowledgeWarnings ? { acknowledge_warnings: true } : undefined
    return api
      .post<{ stack_id: string; status: string }>(`/stacks/${input.stackId}/continue`, body)
      .then((r) => r.data)
  },

  getWorkloads: (stackId: string) =>
    api.get<StackWorkloads>(`/stacks/${stackId}/workloads`).then((r) => r.data),
}

export interface DeployStackInput {
  stackId: string
  // acknowledgeWarnings opts in to proceeding when the server-side
  // Pre-Deploy Gate (F8-F3) returns overall.state == "warn". Defaults to
  // false so legacy clients that pass a bare stackId are blocked on warn
  // instead of silently installing.
  acknowledgeWarnings?: boolean
}

// --- Hooks ---

export function useTemplates() {
  return useQuery({
    queryKey: queryKeys.templates(),
    queryFn: stackApiCalls.getTemplates,
  })
}

export function useClusters() {
  return useQuery({
    queryKey: queryKeys.clusters(),
    queryFn: stackApiCalls.getClusters,
  })
}

export function useClusterK8sVersion() {
  return useMutation({
    mutationFn: (clusterId: string) => stackApiCalls.getClusterK8sVersion(clusterId),
  })
}

export function useStacks(
  filters?: { status?: string; search?: string; include_deleted?: boolean },
  options?: { refetchIntervalMs?: number },
) {
  return useQuery({
    queryKey: queryKeys.list(filters),
    queryFn: () => stackApiCalls.getList(filters),
    refetchInterval: options?.refetchIntervalMs && options.refetchIntervalMs > 0
      ? options.refetchIntervalMs
      : false,
  })
}

export function useCreateStack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: stackApiCalls.create,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

export function useAddTools() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      stackId,
      tools,
    }: {
      stackId: string
      tools: Array<{ category: string; tool: string; version: string }>
    }) => api.patch(`/stacks/${stackId}/tools`, { tools }).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

export function useDeleteStack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (stackId: string) => stackApiCalls.delete(stackId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

export function useSaveDraft() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: stackApiCalls.saveDraft,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

export function useEstimateResources() {
  return useMutation({
    mutationFn: stackApiCalls.estimateResources,
  })
}

export function useResourceDefaults() {
  return useQuery({
    queryKey: queryKeys.resourceDefaults(),
    queryFn: stackApiCalls.getResourceDefaults,
  })
}

export function useUpsertResourceDefault() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: stackApiCalls.upsertResourceDefault,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.resourceDefaults() })
    },
  })
}

export function useStackHistory(stackId: string) {
  return useQuery({
    queryKey: queryKeys.history(stackId),
    queryFn: () => stackApiCalls.getHistory(stackId),
    enabled: !!stackId,
  })
}

// F8-UIUX-RetryAuditSurface-Frontend — load retry audit entries for the
// deployment logs page. staleTime 30s keeps the panel quiet while still
// surfacing brand-new retries without requiring a hard refresh.
export function useStackRetryHistory(stackId: string | undefined) {
  return useQuery<{ items: RetryHistoryEntry[] }>({
    queryKey: ['stack-retry-history', stackId],
    queryFn: async () => {
      const res = await api.get<{ items: RetryHistoryEntry[] }>(`/stacks/${stackId}/retry-history`)
      return res.data
    },
    enabled: Boolean(stackId),
    staleTime: 30_000,
  })
}

export function useStackMonitoring(stackId: string, refetchIntervalMs = 5000) {
  return useQuery({
    queryKey: queryKeys.monitoring(stackId),
    queryFn: () => stackApiCalls.getMonitoring(stackId),
    enabled: !!stackId,
    refetchInterval: refetchIntervalMs,
    staleTime: 0,
  })
}

export function useStackVersionDiff(stackId: string, from: number, to: number) {
  return useQuery({
    queryKey: queryKeys.versionDiff(stackId, from, to),
    queryFn: () => stackApiCalls.getVersionDiff(stackId, from, to),
    enabled: !!stackId && from > 0 && to > 0,
  })
}

export function useRollbackStack() {
   const qc = useQueryClient()
   return useMutation({
     mutationFn: ({ stackId, version, preservePVC }: { stackId: string; version: number; preservePVC: boolean }) =>
       stackApiCalls.rollbackStack(stackId, version, preservePVC),
     onSuccess: (_, variables) => {
       void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
       void qc.invalidateQueries({ queryKey: queryKeys.history(variables.stackId) })
    },
  })
}

export function useCompatibilityMatrix() {
  return useQuery({
    queryKey: queryKeys.compatibilityMatrix(),
    queryFn: stackApiCalls.getCompatibilityMatrix,
  })
}

export function useValidateCompatibility(defaultStackId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input?: ValidateCompatibilityInput | string) => {
      const normalized: ValidateCompatibilityInput =
        typeof input === 'string'
          ? { stackId: input }
          : (input ?? { stackId: defaultStackId ?? '' })
      return stackApiCalls.validateCompatibility(normalized)
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() })
    },
  })
}

export function useCreateTemplate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: stackApiCalls.createTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.templates() })
    },
  })
}

export function useUpdateTemplate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: stackApiCalls.updateTemplate,
    onSuccess: (_, variables) => {
      void qc.invalidateQueries({ queryKey: queryKeys.templates() })
      void qc.invalidateQueries({ queryKey: queryKeys.template(variables.id) })
    },
  })
}

export function useDeleteTemplate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: stackApiCalls.deleteTemplate,
    onSuccess: (_, id) => {
      void qc.invalidateQueries({ queryKey: queryKeys.templates() })
      void qc.invalidateQueries({ queryKey: queryKeys.template(id) })
    },
  })
}

export function useDeployStack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: DeployStackInput | string) => stackApiCalls.deployStack(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

// useRetryStack — F8 follow-up Phase 3. Drives POST /stacks/:id/retry from
// UI. Invalidates the stack list cache so Retry buttons update.
export function useRetryStack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: DeployStackInput) => stackApiCalls.retryStack(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

export function useContinueStack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: DeployStackInput) => stackApiCalls.continueStack(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

// F8-Phase5 (재개) matrix CRUD mutations. Each onSuccess invalidates the
// compatibility cache so the Stack Version Management page refetches.
export function useCreateMatrix() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: MatrixInput) => stackApiCalls.createMatrix(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() })
    },
  })
}

export function useUpdateMatrix() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: MatrixInput) => stackApiCalls.updateMatrix(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() })
    },
  })
}

export function useDeleteMatrix() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => stackApiCalls.deleteMatrix(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() })
    },
  })
}

export function useStackWorkloads(stackId: string) {
  return useQuery({
    queryKey: queryKeys.workloads(stackId),
    queryFn: () => stackApiCalls.getWorkloads(stackId),
    enabled: !!stackId,
    refetchInterval: 30_000,
  })
}

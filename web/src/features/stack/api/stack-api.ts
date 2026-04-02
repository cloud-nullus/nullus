import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import { useOrganization } from '../../admin/api/admin-api'
import type {
  ClusterStatus,
  CompatibilityMatrix,
  CompatibilityValidationResult,
  CreateStackRequest,
  ResourceEstimate,
  StackResourceDefault,
  Stack,
  StackHistoryEntry,
  StackTemplate,
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
  HelmVersion?: string
  appVersion?: string
  AppVersion?: string
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

const normalizeCompatibilityTool = (tool: RawCompatibilityTool) => ({
  name: tool.name ?? tool.Name ?? 'Unknown',
  helmVersion: tool.helmVersion ?? tool.HelmVersion ?? '-',
  appVersion: tool.appVersion ?? tool.AppVersion ?? '-',
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
  const status = raw.state ?? raw.status ?? 'pending'
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
    checkedAt: raw.checkedAt ?? new Date().toISOString(),
  }
}

// --- API functions ---

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
        visualization: toBackendTool(m.visualization ?? { tool: '', version: '' }),
      },
      logging: {
        collection: toBackendTool(l.collection ?? { tool: '', version: '' }),
        search: toBackendTool(l.search ?? { tool: '', version: '' }),
        trace_layer: toBackendTool(l.traceLayer ?? l.trace_layer ?? { tool: '', version: '' }),
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

  getList: (filters?: { status?: string; search?: string }) =>
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

  validateCompatibility: (stackId: string) =>
    api.post<RawCompatibilityValidationResult>(`/stacks/${stackId}/validate`).then((r) => normalizeCompatibilityValidationResult(r.data ?? {})),

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

  deployStack: (stackId: string) =>
    api.post<{ stack_id: string; status: string }>(`/stacks/${stackId}/deploy`).then((r) => r.data),
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

export function useScopedClusters() {
  const { data: clusters, ...rest } = useClusters()
  const { data: org } = useOrganization()
  const scope = org?.clusterAccessScope ?? []

  const filtered = scope.length > 0
    ? (clusters ?? []).filter((c) => scope.includes(c.name))
    : (clusters ?? [])

  return { ...rest, data: filtered }
}

export function useClusterK8sVersion() {
  return useMutation({
    mutationFn: (clusterId: string) => stackApiCalls.getClusterK8sVersion(clusterId),
  })
}

export function useStacks(filters?: { status?: string; search?: string }) {
  return useQuery({
    queryKey: queryKeys.list(filters),
    queryFn: () => stackApiCalls.getList(filters),
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

export function useValidateCompatibility() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (stackId: string) => stackApiCalls.validateCompatibility(stackId),
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
    mutationFn: stackApiCalls.deployStack,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks', 'list'] })
    },
  })
}

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import type {
  CompatibilityMatrix,
  CompatibilityValidationResult,
  CreateStackRequest,
  ResourceEstimate,
  Stack,
  StackHistoryEntry,
  StackTemplate,
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
  connection_status: string
}

const queryKeys = {
  templates: () => ['stacks', 'templates'] as const,
  template: (id: string) => ['stacks', 'templates', id] as const,
  list: (filters?: Record<string, unknown>) => ['stacks', 'list', filters] as const,
  history: (stackId: string) => ['stacks', 'history', stackId] as const,
  versionDiff: (stackId: string, from: number, to: number) => ['stacks', 'diff', stackId, from, to] as const,
  compatibilityMatrix: () => ['stacks', 'compatibility'] as const,
  clusters: () => ['clusters'] as const,
}

// --- API functions ---

function toBackendTool(sel: { tool: string; version: string }) {
  return { name: sel.tool, version: sel.version, enabled: true }
}

function toCreateStackBody(req: CreateStackRequest) {
  const a = req.artifacts as Record<string, { tool: string; version: string }>
  const p = req.pipeline as Record<string, { tool: string; version: string }>
  const m = req.monitoring as Record<string, { tool: string; version: string }>
  const l = req.logging as Record<string, { tool: string; version: string }>
  return {
    name: req.stackName,
    cluster_id: req.clusterId ?? '',
    namespace: req.namespace || 'nullus',
    golden_path_id: req.templateId ?? '',
    config: {
      artifacts: {
        package_registry: toBackendTool(a.packageRegistry ?? a.package_registry ?? { tool: '', version: '' }),
        source_repository: toBackendTool(a.sourceRepository ?? a.source_repository ?? { tool: '', version: '' }),
        container_registry: toBackendTool(a.containerRegistry ?? a.container_registry ?? { tool: '', version: '' }),
        storage_backend: toBackendTool(a.storageBackend ?? a.storage_backend ?? { tool: '', version: '' }),
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
      },
      resources: {
        developers: req.resources?.developerCount ?? 0,
        concurrent_runners: req.resources?.concurrentRunners ?? 0,
        weekly_commits: req.resources?.commitsPerDay ?? 0,
        build_frequency: req.resources?.buildFrequency ?? 'medium',
      },
    },
  }
}

const stackApiCalls = {
  getTemplates: () =>
    api.get<StackTemplate[]>('/stacks/templates').then((r) =>
      (r.data ?? []).map((t: Record<string, unknown>) => ({
        ...t,
        tools: Array.isArray(t.tools)
          ? t.tools.map((tool: unknown) => (typeof tool === 'string' ? tool : (tool as Record<string, string>).name ?? ''))
          : [],
        estimatedMinutes: t.estimatedMinutes ?? (typeof t.estimated_install_time === 'number' ? Math.round(Number(t.estimated_install_time) / 60000000000) : 30),
      }))
    ),

  getTemplate: (id: string) =>
    api.get<StackTemplate>(`/stacks/templates/${id}`).then((r) => r.data),

  getList: (filters?: { status?: string; search?: string }) =>
    api.get<{ items: Stack[]; total: number }>('/stacks', { params: filters }).then((r) => r.data),

  create: (request: CreateStackRequest) =>
    api.post<{ id: string }>('/stacks', toCreateStackBody(request)).then((r) => r.data),

  saveDraft: (request: CreateStackRequest) =>
    api.post<{ draftId: string }>('/stacks/draft', request).then((r) => r.data),

  estimateResources: (input: CreateStackRequest['resources']) =>
    api.post<ResourceEstimate>('/stacks/estimate', input).then((r) => r.data),

  getHistory: (stackId: string) =>
    api.get<StackHistoryEntry[]>(`/stacks/${stackId}/history`).then((r) => r.data),

  getVersionDiff: (stackId: string, from: number, to: number) =>
    api.get<StackVersionDiff>(`/stacks/${stackId}/history/diff`, { params: { versionA: from, versionB: to } }).then((r) => r.data),

  rollbackStack: (stackId: string, version: number) =>
    api.post<{ id: string }>(`/stacks/${stackId}/rollback`, { version }).then((r) => r.data),

  getCompatibilityMatrix: () =>
    api.get<CompatibilityMatrix[]>('/stacks/compatibility').then((r) => r.data),

  validateCompatibility: (stackId: string) =>
    api.post<CompatibilityValidationResult>(`/stacks/${stackId}/validate`).then((r) => r.data),

  createTemplate: (request: TemplateMutationRequest) =>
    api.post<StackTemplate>('/stacks/templates', request).then((r) => r.data),

  updateTemplate: (request: TemplateMutationRequest) =>
    api.put<StackTemplate>(`/stacks/templates/${request.id}`, request).then((r) => r.data),

  deleteTemplate: (id: string) =>
    api.delete<void>(`/stacks/templates/${id}`).then((r) => r.data),

  getClusters: () =>
    api.get<{ items: ClusterSummary[] }>('/admin/clusters').then((r) => r.data?.items ?? []),

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

export function useStackHistory(stackId: string) {
  return useQuery({
    queryKey: queryKeys.history(stackId),
    queryFn: () => stackApiCalls.getHistory(stackId),
    enabled: !!stackId,
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
    mutationFn: ({ stackId, version }: { stackId: string; version: number }) =>
      stackApiCalls.rollbackStack(stackId, version),
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

export function useValidateCompatibility(stackId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => stackApiCalls.validateCompatibility(stackId),
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

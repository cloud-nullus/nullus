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

// --- Query keys ---

const queryKeys = {
  templates: () => ['stacks', 'templates'] as const,
  template: (id: string) => ['stacks', 'templates', id] as const,
  list: (filters?: Record<string, unknown>) => ['stacks', 'list', filters] as const,
  history: (stackId: string) => ['stacks', 'history', stackId] as const,
  versionDiff: (stackId: string, from: number, to: number) => ['stacks', 'diff', stackId, from, to] as const,
  compatibilityMatrix: () => ['stacks', 'compatibility'] as const,
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

const normalizeTemplate = (raw: RawTemplate): StackTemplate => ({
  id: raw.id ?? raw.ID ?? '',
  name: raw.name ?? raw.Name ?? '',
  description: raw.description ?? raw.Description ?? '',
  tools: Array.isArray(raw.tools ?? raw.Tools) ? (raw.tools ?? raw.Tools ?? []).map(toToolName).filter((tool) => tool.length > 0) : [],
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

  return {
    id: raw.id ?? raw.ID ?? '',
    name: raw.name ?? raw.Name ?? 'Unnamed Matrix',
    status: raw.status ?? raw.Status ?? 'untested',
    k8sRange: normalizeK8sRange(raw),
    tools,
  }
}

// --- API functions ---

const stackApiCalls = {
  getTemplates: () =>
    api.get<RawTemplate[]>('/stacks/templates').then((r) => (r.data ?? []).map(normalizeTemplate)),

  getTemplate: (id: string) =>
    api.get<StackTemplate>(`/stacks/templates/${id}`).then((r) => r.data),

  getList: (filters?: { status?: string; search?: string }) =>
    api.get<{ items: Stack[]; total: number }>('/stacks', { params: filters }).then((r) => r.data),

  create: (request: CreateStackRequest) =>
    api.post<{ id: string }>('/stacks', request).then((r) => r.data),

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
    api.get<RawCompatibilityMatrix[]>('/stacks/compatibility').then((r) => (r.data ?? []).map(normalizeCompatibilityMatrix)),

  validateCompatibility: (stackId: string) =>
    api.post<CompatibilityValidationResult>(`/stacks/${stackId}/validate`).then((r) => r.data),

  createTemplate: (request: TemplateMutationRequest) =>
    api.post<StackTemplate>('/stacks/templates', request).then((r) => r.data),

  updateTemplate: (request: TemplateMutationRequest) =>
    api.put<StackTemplate>(`/stacks/templates/${request.id}`, request).then((r) => r.data),

  deleteTemplate: (id: string) =>
    api.delete<void>(`/stacks/templates/${id}`).then((r) => r.data),
}

// --- Hooks ---

export function useTemplates() {
  return useQuery({
    queryKey: queryKeys.templates(),
    queryFn: stackApiCalls.getTemplates,
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

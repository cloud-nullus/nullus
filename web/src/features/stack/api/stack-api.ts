import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'

// --- Types ---

export interface StackHistoryEntry {
  id: string
  stackId: string
  version: number
  changedBy: string
  changedAt: string
  reason: string
  snapshot: Record<string, unknown>
}

export interface StackVersionDiff {
  fromVersion: number
  toVersion: number
  added: { key: string; value: string }[]
  removed: { key: string; value: string }[]
  changed: { key: string; from: string; to: string }[]
}

export interface CompatibilityMatrix {
  id: string
  name: string
  status: 'verified' | 'untested'
  k8sRange: string
  tools: { name: string; helmVersion: string; appVersion: string }[]
}

export interface CompatibilityValidationResult {
  compatible: boolean
  issues: { tool: string; message: string; severity: 'error' | 'warning' }[]
  checkedAt: string
}

export interface StackTemplate {
  id: string
  name: string
  description: string
  tools: string[]
  estimatedMinutes: number
  category: string
}

export interface Stack {
  id: string
  name: string
  templateId: string
  templateName: string
  clusterId: string
  clusterName: string
  status: 'running' | 'success' | 'failed' | 'pending' | 'cancelled'
  createdAt: string
  updatedAt: string
}

export interface CreateStackRequest {
  templateId: string | null
  clusterId: string | null
  stackName: string
  artifacts: Record<string, { tool: string; version: string }>
  pipeline: Record<string, { tool: string; version: string }>
  monitoring: Record<string, { tool: string; version: string }>
  logging: Record<string, { tool: string; version: string }>
  resources: {
    developerCount: number
    concurrentRunners: number
    commitsPerDay: number
    buildFrequency: string
    currency: string
  }
}

export interface ResourceEstimate {
  cpu: string
  memory: string
  storage: string
  estimatedCostMonthly: number
  currency: string
}

// --- Query keys ---

const queryKeys = {
  templates: () => ['stacks', 'templates'] as const,
  template: (id: string) => ['stacks', 'templates', id] as const,
  list: (filters?: Record<string, unknown>) => ['stacks', 'list', filters] as const,
  estimate: (input: Record<string, unknown>) => ['stacks', 'estimate', input] as const,
  history: (stackId: string) => ['stacks', 'history', stackId] as const,
  versionDiff: (stackId: string, from: number, to: number) => ['stacks', 'diff', stackId, from, to] as const,
  compatibilityMatrix: () => ['stacks', 'compatibility'] as const,
  validateCompatibility: (stackId: string) => ['stacks', 'validate', stackId] as const,
}

// --- API functions ---

const stackApiCalls = {
  getTemplates: () =>
    api.get<StackTemplate[]>('/stacks/templates').then((r) => r.data),

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
    api.get<StackVersionDiff>(`/stacks/${stackId}/diff`, { params: { from, to } }).then((r) => r.data),

  rollbackStack: (stackId: string, version: number) =>
    api.post<{ id: string }>(`/stacks/${stackId}/rollback`, { version }).then((r) => r.data),

  getCompatibilityMatrix: () =>
    api.get<CompatibilityMatrix[]>('/stacks/compatibility').then((r) => r.data),

  validateCompatibility: (stackId: string) =>
    api.post<CompatibilityValidationResult>(`/stacks/${stackId}/validate`).then((r) => r.data),
}

// --- Hooks ---

export function useTemplates() {
  return useQuery({
    queryKey: queryKeys.templates(),
    queryFn: stackApiCalls.getTemplates,
  })
}

export function useTemplate(id: string) {
  return useQuery({
    queryKey: queryKeys.template(id),
    queryFn: () => stackApiCalls.getTemplate(id),
    enabled: !!id,
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
  return useMutation({
    mutationFn: stackApiCalls.saveDraft,
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
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['stacks'] })
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
  return useMutation({
    mutationFn: (stackId: string) => stackApiCalls.validateCompatibility(stackId),
  })
}

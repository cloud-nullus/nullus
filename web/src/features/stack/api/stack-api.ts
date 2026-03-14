import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'

// --- Types ---

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

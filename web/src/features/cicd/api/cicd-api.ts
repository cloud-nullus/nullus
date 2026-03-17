import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import type {
  AppTemplateInfo,
  CicdTemplate,
  CreateCicdTemplateRequest,
  CreatePipelineRequest,
  DeployAppRequest,
  DeployAppResult,
  Deployment,
  Pipeline,
} from '../../../types'

export type {
  AppTemplate,
  AppTemplateInfo,
  AppType,
  CicdTemplate,
  CreateCicdTemplateRequest,
  CreatePipelineRequest,
  DeployAppRequest,
  DeployAppResult,
  Deployment,
  Pipeline,
  PipelineStatus,
} from '../../../types'

// --- Query keys ---

const queryKeys = {
  templates: () => ['cicd', 'templates'] as const,
  pipelines: (filters?: Record<string, unknown>) => ['cicd', 'pipelines', filters] as const,
  deployments: (filters?: Record<string, unknown>) => ['cicd', 'deployments', filters] as const,
  appTemplates: () => ['cicd', 'appTemplates'] as const,
}

// --- API functions ---

const cicdApiCalls = {
  getTemplates: () =>
    api.get<CicdTemplate[]>('/cicd/templates').then((r) => r.data),

  createTemplate: (data: CreateCicdTemplateRequest) =>
    api.post<CicdTemplate>('/cicd/templates', data).then((r) => r.data),

  updateTemplate: (data: CreateCicdTemplateRequest) =>
    api.put<CicdTemplate>(`/cicd/templates/${data.id}`, data).then((r) => r.data),

  deleteTemplate: (id: string) =>
    api.delete<void>(`/cicd/templates/${id}`).then((r) => r.data),

  getPipelines: (filters?: { status?: string; search?: string }) =>
    api.get<{ items: Pipeline[]; total: number }>('/cicd/pipelines', { params: filters }).then((r) => r.data),

  createPipeline: (data: CreatePipelineRequest) =>
    api.post<Pipeline>('/cicd/pipelines', data).then((r) => r.data),

  deployPipeline: (pipelineId: string) =>
    api.post<{ deploymentId: string }>(`/cicd/pipelines/${pipelineId}/deploy`).then((r) => r.data),

  getDeployments: (filters?: { pipelineId?: string; status?: string }) =>
    api.get<{ items: Deployment[]; total: number }>('/cicd/deployments', { params: filters }).then((r) => r.data),

  getAppTemplates: () =>
    api.get<AppTemplateInfo[]>('/cicd/app-templates').then((r) => r.data),

  deployApp: (request: DeployAppRequest) =>
    api.post<DeployAppResult>('/cicd/deploy-app', request).then((r) => r.data),
}

// --- Hooks ---

export function useCicdTemplates() {
  return useQuery({
    queryKey: queryKeys.templates(),
    queryFn: cicdApiCalls.getTemplates,
  })
}

export function useCreateCicdTemplate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cicdApiCalls.createTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cicd', 'templates'] })
    },
  })
}

export function useUpdateCicdTemplate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cicdApiCalls.updateTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cicd', 'templates'] })
    },
  })
}

export function useDeleteCicdTemplate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cicdApiCalls.deleteTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cicd', 'templates'] })
    },
  })
}

export function usePipelines(filters?: { status?: string; search?: string }) {
  return useQuery({
    queryKey: queryKeys.pipelines(filters),
    queryFn: () => cicdApiCalls.getPipelines(filters),
  })
}

export function useCreatePipeline() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cicdApiCalls.createPipeline,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cicd', 'pipelines'] })
    },
  })
}

export function useDeployPipeline() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cicdApiCalls.deployPipeline,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cicd', 'pipelines'] })
      void qc.invalidateQueries({ queryKey: ['cicd', 'deployments'] })
    },
  })
}

export function useDeployments(filters?: { pipelineId?: string; status?: string }) {
  return useQuery({
    queryKey: queryKeys.deployments(filters),
    queryFn: () => cicdApiCalls.getDeployments(filters),
  })
}

export function useAppTemplates() {
  return useQuery({
    queryKey: queryKeys.appTemplates(),
    queryFn: cicdApiCalls.getAppTemplates,
  })
}

export function useDeployApp() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cicdApiCalls.deployApp,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cicd', 'pipelines'] })
      void qc.invalidateQueries({ queryKey: ['cicd', 'deployments'] })
    },
  })
}

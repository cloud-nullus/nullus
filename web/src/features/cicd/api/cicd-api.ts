import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'

// --- Types ---

export type PipelineStatus = 'running' | 'success' | 'failed' | 'pending' | 'cancelled'
export type AppType = 'web-backend' | 'web-frontend' | 'batch-job'
export type AppTemplate = 'react-spa' | 'next-app' | 'express-api' | 'spring-boot' | 'python-fastapi'

export interface CicdTemplate {
  id: string
  name: string
  description: string
  appType: AppType
  stages: string[]
}

export interface Pipeline {
  id: string
  name: string
  appType: AppType
  clusterId: string
  clusterName: string
  status: PipelineStatus
  lastDeployedAt: string | null
  createdAt: string
}

export interface Deployment {
  id: string
  pipelineId: string
  pipelineName: string
  version: string
  status: PipelineStatus
  triggeredBy: string
  startedAt: string
  completedAt: string | null
}

export interface CreatePipelineRequest {
  name: string
  appType: AppType
  clusterId: string
  templateId?: string
}

export interface AppTemplateInfo {
  id: AppTemplate
  name: string
  description: string
  language: string
}

export interface DeployAppRequest {
  appName: string
  gitUrl: string
  clusterId: string
  namespace: string
  template: AppTemplate
  resources: {
    cpuRequest: string
    cpuLimit: string
    memoryRequest: string
    memoryLimit: string
  }
  envVars: { key: string; value: string }[]
}

export interface DeployAppResult {
  deploymentId: string
  appName: string
  status: PipelineStatus
}

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
  return useMutation({
    mutationFn: cicdApiCalls.deployApp,
  })
}

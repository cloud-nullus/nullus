import { create } from 'zustand'

export type BuildFrequency = 'low' | 'medium' | 'high'
export type Currency = 'USD' | 'KRW' | 'CNY'
export type ResourceMode = 'auto' | 'manual'
export type InstallTab = 'artifacts' | 'pipeline' | 'monitoring' | 'resources' | 'yaml'

export interface ToolSelection {
  tool: string
  version: string
}

export interface ArtifactsConfig {
  packageRegistry: ToolSelection
  sourceRepository: ToolSelection
  containerRegistry: ToolSelection
  storageBackend: ToolSelection
}

export interface PipelineConfig {
  cicdPlatform: ToolSelection
  cdTool: ToolSelection
}

export interface MonitoringConfig {
  collection: ToolSelection
  visualization: ToolSelection
}

export interface LoggingConfig {
  search: ToolSelection
  traceLayer: ToolSelection
}

export interface ResourceConfig {
  developerCount: number
  concurrentRunners: number
  commitsPerDay: number
  buildFrequency: BuildFrequency
  currency: Currency
  mode: ResourceMode
  cpuRequest?: string
  memoryRequest?: string
  storageRequest?: string
}

export interface StackConfigDraft {
  selectedTemplateId: string | null
  clusterId: string | null
  namespace: string
  stackName: string
  artifacts: ArtifactsConfig
  pipeline: PipelineConfig
  monitoring: MonitoringConfig
  logging: LoggingConfig
  resources: ResourceConfig
  activeTab: InstallTab
}

interface StackConfigState {
  draft: StackConfigDraft
  isDirty: boolean
  setTemplate: (templateId: string) => void
  setCluster: (clusterId: string) => void
  setNamespace: (namespace: string) => void
  setStackName: (name: string) => void
  setTool: (
    section: 'artifacts' | 'pipeline' | 'monitoring' | 'logging',
    field: string,
    value: ToolSelection
  ) => void
  updateResources: (config: Partial<ResourceConfig>) => void
  setActiveTab: (tab: InstallTab) => void
  loadFromTemplate: (templateId: string, overrides?: Partial<StackConfigDraft>) => void
  resetConfig: () => void
}

const DEFAULT_DRAFT: StackConfigDraft = {
  selectedTemplateId: null,
  clusterId: null,
  namespace: '',
  stackName: '',
  artifacts: {
    packageRegistry: { tool: 'gitlab', version: 'latest' },
    sourceRepository: { tool: 'gitlab', version: 'latest' },
    containerRegistry: { tool: 'gitlab-registry', version: 'latest' },
    storageBackend: { tool: 'minio', version: 'latest' },
  },
  pipeline: {
    cicdPlatform: { tool: 'gitlab-ci', version: 'latest' },
    cdTool: { tool: 'argocd', version: 'latest' },
  },
  monitoring: {
    collection: { tool: 'prometheus', version: 'latest' },
    visualization: { tool: 'grafana', version: 'latest' },
  },
  logging: {
    search: { tool: 'opensearch', version: 'latest' },
    traceLayer: { tool: 'tempo', version: 'latest' },
  },
  resources: {
    developerCount: 10,
    concurrentRunners: 5,
    commitsPerDay: 50,
    buildFrequency: 'medium',
    currency: 'KRW',
    mode: 'auto',
    cpuRequest: '4',
    memoryRequest: '8Gi',
    storageRequest: '100Gi',
  },
  activeTab: 'artifacts',
}

export const useStackConfigStore = create<StackConfigState>()((set) => ({
  draft: DEFAULT_DRAFT,
  isDirty: false,

  setTemplate: (templateId) =>
    set((s) => ({ draft: { ...s.draft, selectedTemplateId: templateId }, isDirty: true })),

  setCluster: (clusterId) =>
    set((s) => ({ draft: { ...s.draft, clusterId, namespace: '' }, isDirty: true })),

  setNamespace: (namespace) =>
    set((s) => ({ draft: { ...s.draft, namespace }, isDirty: true })),

  setStackName: (name) =>
    set((s) => ({ draft: { ...s.draft, stackName: name }, isDirty: true })),

  setTool: (section, field, value) =>
    set((s) => ({
      draft: {
        ...s.draft,
        [section]: { ...(s.draft[section] as unknown as Record<string, ToolSelection>), [field]: value },
      },
      isDirty: true,
    })),

  updateResources: (config) =>
    set((s) => ({
      draft: { ...s.draft, resources: { ...s.draft.resources, ...config } },
      isDirty: true,
    })),

  setActiveTab: (tab) =>
    set((s) => ({ draft: { ...s.draft, activeTab: tab } })),

  loadFromTemplate: (templateId, overrides) =>
    set(() => ({
      draft: { ...DEFAULT_DRAFT, selectedTemplateId: templateId, ...overrides },
      isDirty: false,
    })),

  resetConfig: () => set({ draft: DEFAULT_DRAFT, isDirty: false }),
}))

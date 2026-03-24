import { create } from 'zustand'

export type BuildFrequency = 'low' | 'medium' | 'high'
export type Currency = 'USD' | 'KRW' | 'CNY'
export type ResourceMode = 'auto' | 'manual'
export type InstallTab = 'artifacts' | 'pipeline' | 'monitoring' | 'resources' | 'storage' | 'manifests' | 'deploy-script' | 'dry-run'

export type StorageMode = 'existing' | 'create'
export type StoragePlanMode = 'existing-all' | 'integrated-create'

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

export interface StorageTargetConfig {
  mode: StorageMode
  existingRef: string
  endpoint: string
  resourceName: string
  accessSecretRef: string
  authId: string
  authPasswordKey: string
  providerOrEngine: string
  version: string
  size: 'small' | 'medium' | 'large'
}

export interface StorageConfig {
  planMode: StoragePlanMode
  database: StorageTargetConfig
  objectStorage: StorageTargetConfig
}

export interface StackConfigDraft {
  selectedTemplateId: string | null
  clusterId: string | null
  namespace: string
  stackName: string
  accessDomain: string
  artifacts: ArtifactsConfig
  pipeline: PipelineConfig
  monitoring: MonitoringConfig
  logging: LoggingConfig
  resources: ResourceConfig
  storage: StorageConfig
  activeTab: InstallTab
}

interface StackConfigState {
  draft: StackConfigDraft
  isDirty: boolean
  setTemplate: (templateId: string) => void
  setCluster: (clusterId: string) => void
  setNamespace: (namespace: string) => void
  setStackName: (name: string) => void
  setAccessDomain: (domain: string) => void
  setTool: (
    section: 'artifacts' | 'pipeline' | 'monitoring' | 'logging',
    field: string,
    value: ToolSelection
  ) => void
  updateResources: (config: Partial<ResourceConfig>) => void
  updateStorage: (config: Partial<StorageConfig>) => void
  updateStorageTarget: (target: 'database' | 'objectStorage', config: Partial<StorageTargetConfig>) => void
  setActiveTab: (tab: InstallTab) => void
  loadFromTemplate: (templateId: string, overrides?: Partial<StackConfigDraft>) => void
  resetConfig: () => void
}

const DEFAULT_DRAFT: StackConfigDraft = {
  selectedTemplateId: null,
  clusterId: null,
  namespace: '',
  stackName: '',
  accessDomain: '',
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
  storage: {
    planMode: 'integrated-create',
    database: {
      mode: 'create',
      existingRef: 'org-shared-postgres',
      endpoint: 'postgres.shared.svc:5432',
      resourceName: 'nullus',
      accessSecretRef: 'shared-postgres-credentials',
      authId: 'nullus_app',
      authPasswordKey: 'password',
      providerOrEngine: 'postgres',
      version: '16',
      size: 'medium',
    },
    objectStorage: {
      mode: 'create',
      existingRef: 'org-shared-object-storage',
      endpoint: 'http://minio.shared.svc:9000',
      resourceName: 'nullus-artifacts',
      accessSecretRef: 'shared-object-storage-credentials',
      authId: 'nullus_access_key',
      authPasswordKey: 'secretKey',
      providerOrEngine: 'minio',
      version: 'latest',
      size: 'medium',
    },
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
    set((s) => {
      const prevDefaultAccessDomain = s.draft.stackName ? `${s.draft.stackName}.internal` : ''
      const shouldUpdateAccessDomain =
        s.draft.accessDomain.trim().length === 0 ||
        s.draft.accessDomain === prevDefaultAccessDomain

      return {
        draft: {
          ...s.draft,
          stackName: name,
          accessDomain: shouldUpdateAccessDomain ? `${name}.internal` : s.draft.accessDomain,
        },
        isDirty: true,
      }
    }),

  setAccessDomain: (domain) =>
    set((s) => ({ draft: { ...s.draft, accessDomain: domain }, isDirty: true })),

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

  updateStorage: (config) =>
    set((s) => ({
      draft: { ...s.draft, storage: { ...s.draft.storage, ...config } },
      isDirty: true,
    })),

  updateStorageTarget: (target, config) =>
    set((s) => ({
      draft: {
        ...s.draft,
        storage: {
          ...s.draft.storage,
          [target]: {
            ...s.draft.storage[target],
            ...config,
          },
        },
      },
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

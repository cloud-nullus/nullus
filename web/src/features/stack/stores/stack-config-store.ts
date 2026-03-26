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

export interface ToolVersionCatalogEntry {
  appVersion: string
  chartVersion?: string
}

export const TOOL_VERSION_CATALOG: Record<string, ToolVersionCatalogEntry> = {
  gitlab: { appVersion: '18.5.1', chartVersion: '9.5.1' },
  'gitlab-registry': { appVersion: '18.5.1', chartVersion: '9.5.1' },
  'gitlab-ci': { appVersion: '18.5.1', chartVersion: '9.5.1' },
  argocd: { appVersion: 'v2.8.3', chartVersion: '6.8.0' },
  minio: { appVersion: 'RELEASE.2024-08-03T04-33-23Z', chartVersion: '5.2.0' },
  prometheus: { appVersion: 'v2.54.1' },
  grafana: { appVersion: '11.1.0' },
  opensearch: { appVersion: '2.14.0', chartVersion: '2.22.0' },
  tempo: { appVersion: '2.5.0' },
  nexus: { appVersion: '3.68.1', chartVersion: '62.1.0' },
  jfrog: { appVersion: '7.77.3', chartVersion: '107.95.10' },
  github: { appVersion: '2.45.0', chartVersion: '0.23.7' },
  gitea: { appVersion: '1.22.2', chartVersion: '10.4.0' },
  harbor: { appVersion: '2.11.0', chartVersion: '1.15.0' },
  'docker-hub': { appVersion: '2.0.0', chartVersion: '0.1.0' },
  s3: { appVersion: '1.0.0', chartVersion: '1.0.0' },
  gcs: { appVersion: '1.0.0', chartVersion: '1.0.0' },
  'github-actions': { appVersion: 'v0.9.0', chartVersion: '0.9.0' },
  jenkins: { appVersion: '2.452.3', chartVersion: '5.5.0' },
  flux: { appVersion: 'v2.3.0', chartVersion: '2.13.0' },
  spinnaker: { appVersion: '1.33.0', chartVersion: '2.32.1' },
  thanos: { appVersion: '0.36.1', chartVersion: '15.7.1' },
  victoriametrics: { appVersion: 'v1.102.1', chartVersion: '0.30.0' },
  kibana: { appVersion: '8.14.1', chartVersion: '8.5.1' },
  'opensearch-dashboards': { appVersion: '2.14.0', chartVersion: '2.18.0' },
  jaeger: { appVersion: '1.57.0', chartVersion: '3.3.0' },
  'opentelemetry-collector': { appVersion: '0.104.0', chartVersion: '0.75.0' },
  elasticsearch: { appVersion: '8.14.1', chartVersion: '8.5.1' },
  loki: { appVersion: '2.9.8', chartVersion: '2.10.2' },
}

export function getToolAppVersion(toolId: string): string {
  return TOOL_VERSION_CATALOG[toolId]?.appVersion ?? '1.0.0'
}

export function getToolChartVersion(toolId: string): string | undefined {
  return TOOL_VERSION_CATALOG[toolId]?.chartVersion
}

function normalizeToolSelectionVersion(selection: ToolSelection): ToolSelection {
  if (!selection.version || selection.version === 'latest') {
    return {
      ...selection,
      version: getToolAppVersion(selection.tool),
    }
  }
  return selection
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

export interface AccessDomainTlsConfig {
  enabled: boolean
  secretName: string
  secretNamespace: string
  issuerName: string
}

export interface StackConfigDraft {
  selectedTemplateId: string | null
  clusterId: string | null
  namespace: string
  stackName: string
  accessDomain: string
  accessDomainTls: AccessDomainTlsConfig
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
  updateAccessDomainTls: (config: Partial<AccessDomainTlsConfig>) => void
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
  accessDomainTls: {
    enabled: false,
    secretName: 'nullus-wildcard-tls',
    secretNamespace: 'nullus',
    issuerName: 'nullus-ca-issuer',
  },
  artifacts: {
    packageRegistry: { tool: 'gitlab', version: getToolAppVersion('gitlab') },
    sourceRepository: { tool: 'gitlab', version: getToolAppVersion('gitlab') },
    containerRegistry: { tool: 'gitlab-registry', version: getToolAppVersion('gitlab-registry') },
    storageBackend: { tool: 'minio', version: getToolAppVersion('minio') },
  },
  pipeline: {
    cicdPlatform: { tool: 'gitlab-ci', version: getToolAppVersion('gitlab-ci') },
    cdTool: { tool: 'argocd', version: getToolAppVersion('argocd') },
  },
  monitoring: {
    collection: { tool: 'prometheus', version: getToolAppVersion('prometheus') },
    visualization: { tool: 'grafana', version: getToolAppVersion('grafana') },
  },
  logging: {
    search: { tool: 'opensearch', version: getToolAppVersion('opensearch') },
    traceLayer: { tool: 'tempo', version: getToolAppVersion('tempo') },
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

function migrateDraftToolVersions(draft: StackConfigDraft): StackConfigDraft {
  return {
    ...draft,
    artifacts: {
      packageRegistry: normalizeToolSelectionVersion(draft.artifacts.packageRegistry),
      sourceRepository: normalizeToolSelectionVersion(draft.artifacts.sourceRepository),
      containerRegistry: normalizeToolSelectionVersion(draft.artifacts.containerRegistry),
      storageBackend: normalizeToolSelectionVersion(draft.artifacts.storageBackend),
    },
    pipeline: {
      cicdPlatform: normalizeToolSelectionVersion(draft.pipeline.cicdPlatform),
      cdTool: normalizeToolSelectionVersion(draft.pipeline.cdTool),
    },
    monitoring: {
      collection: normalizeToolSelectionVersion(draft.monitoring.collection),
      visualization: normalizeToolSelectionVersion(draft.monitoring.visualization),
    },
    logging: {
      search: normalizeToolSelectionVersion(draft.logging.search),
      traceLayer: normalizeToolSelectionVersion(draft.logging.traceLayer),
    },
  }
}

export const useStackConfigStore = create<StackConfigState>()((set) => ({
  draft: migrateDraftToolVersions(DEFAULT_DRAFT),
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
          accessDomainTls: {
            ...s.draft.accessDomainTls,
            secretName:
              !s.draft.accessDomainTls.secretName.trim() ||
              s.draft.accessDomainTls.secretName === 'nullus-wildcard-tls' ||
              s.draft.accessDomainTls.secretName === `${s.draft.stackName || 'nullus'}-wildcard-tls`
                ? `${name || 'nullus'}-wildcard-tls`
                : s.draft.accessDomainTls.secretName,
          },
        },
        isDirty: true,
      }
    }),

  setAccessDomain: (domain) =>
    set((s) => ({ draft: { ...s.draft, accessDomain: domain }, isDirty: true })),

  updateAccessDomainTls: (config) =>
    set((s) => ({
      draft: { ...s.draft, accessDomainTls: { ...s.draft.accessDomainTls, ...config } },
      isDirty: true,
    })),

  setTool: (section, field, value) =>
    set((s) => ({
      draft: {
        ...s.draft,
        [section]: {
          ...(s.draft[section] as unknown as Record<string, ToolSelection>),
          [field]: normalizeToolSelectionVersion(value),
        },
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
      draft: migrateDraftToolVersions({ ...DEFAULT_DRAFT, selectedTemplateId: templateId, ...overrides }),
      isDirty: false,
    })),

  resetConfig: () => set({ draft: migrateDraftToolVersions(DEFAULT_DRAFT), isDirty: false }),
}))

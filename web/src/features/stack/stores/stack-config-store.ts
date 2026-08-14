import { create } from 'zustand'

export type BuildFrequency = 'low' | 'medium' | 'high'
export type Currency = 'USD' | 'KRW' | 'CNY'
export type ResourceMode = 'auto' | 'manual'
export type InstallTab = 'artifacts' | 'pipeline' | 'monitoring' | 'authentication' | 'resources' | 'storage' | 'manifests' | 'deploy-script' | 'dry-run'

export type StorageMode = 'existing' | 'create'
export type StoragePlanMode = 'none' | 'existing-all' | 'integrated-create'

export interface ToolSelection {
  tool: string
  version: string
}

export interface ToolVersionCatalogEntry {
  appVersion: string
  chartVersion?: string
}

/**
 * 화면이 "이 도구는 몇 버전인가" 를 물을 때 보는 표.
 *
 * 클러스터에 실제로 설치되는 도구의 값은 **백엔드가 소유한다**
 * (`internal/stack/domain/connection.go`). 그 상수 하나가 설치 경로와 호환성
 * 매트릭스를 함께 먹이고, 백엔드는 `TestChartVersionsMatchCompatibilityMatrix`
 * 로 둘이 갈라지지 않게 붙들고 있다. 여기는 그 값을 베껴 오는 자리다.
 *
 * 베껴 오는 자리에 지킴이가 없어서 실제로 갈라졌다 — 설치는 Argo CD 7.7.16 을
 * 올리는데 화면은 6.8.0 을, GitLab 은 8.7.2 인데 9.5.1 을 말하고 있었고 대조한
 * 11개 중 9개가 어긋나 있었다. 템플릿 편집기가 기본값을 여기서 가져가므로,
 * 관리자가 손대지 않으면 **존재하지 않는 버전이 그대로 템플릿에 pin 됐다.**
 * `tool-version-catalog.test.ts` 가 이제 그 상수와 대조한다.
 *
 * 백엔드에 상수가 없는 도구(설치 경로가 없거나 외부 SaaS)는 여기서만 관리한다.
 */
export const TOOL_VERSION_CATALOG: Record<string, ToolVersionCatalogEntry> = {
  gitlab: { appVersion: 'v17.7.0', chartVersion: '8.7.2' },
  'gitlab-registry': { appVersion: 'v17.7.0', chartVersion: '8.7.2' },
  'gitlab-ci': { appVersion: 'v17.7.0', chartVersion: '8.7.2' },
  argocd: { appVersion: 'v2.13.3', chartVersion: '7.7.16' },
  minio: { appVersion: 'RELEASE.2024-12-18T13-15-44Z', chartVersion: '5.4.0' },
  prometheus: { appVersion: 'v3.1.0', chartVersion: '69.3.0' },
  grafana: { appVersion: '11.5.1', chartVersion: '8.9.0' },
  opensearch: { appVersion: '2.14.0', chartVersion: '2.22.0' },
  tempo: { appVersion: '2.7.0', chartVersion: '1.18.1' },
  nexus: { appVersion: '3.64.0', chartVersion: '64.2.0' },
  jfrog: { appVersion: '7.77.3', chartVersion: '107.95.10' },
  github: { appVersion: 'external' },
  gitea: { appVersion: '1.27.0', chartVersion: '12.7.0' },
  harbor: { appVersion: '2.11.0', chartVersion: '1.15.0' },
  'docker-hub': { appVersion: '2.0.0', chartVersion: '0.1.0' },
  s3: { appVersion: '1.0.0', chartVersion: '1.0.0' },
  gcs: { appVersion: '1.0.0', chartVersion: '1.0.0' },
  // GitHub·GitHub Actions·GHCR 은 외부 SaaS 라 설치할 차트가 없다.
  // 버전은 표시용 값일 뿐이므로 external 로 둔다.
  'github-actions': { appVersion: 'external' },
  ghcr: { appVersion: 'external' },
  // Jenkins 차트는 임의로 내릴 수 없다 — Gitea multibranch 스캔에 쓰는 gitea
  // 플러그인이 Jenkins 2.528.3 이상을 요구한다.
  // (백엔드 단일 출처: internal/stack/domain/connection.go)
  jenkins: { appVersion: '2.568.2', chartVersion: '5.9.54' },
  flux: { appVersion: 'v2.3.0', chartVersion: '2.13.0' },
  spinnaker: { appVersion: '1.33.0', chartVersion: '2.32.1' },
  thanos: { appVersion: '0.36.1', chartVersion: '15.7.1' },
  victoriametrics: { appVersion: 'v1.102.1', chartVersion: '0.30.0' },
  kibana: { appVersion: '8.14.1', chartVersion: '8.5.1' },
  'opensearch-dashboards': { appVersion: '2.14.0', chartVersion: '2.18.0' },
  jaeger: { appVersion: '1.57.0', chartVersion: '3.3.0' },
  'opentelemetry-collector': { appVersion: '0.90.0', chartVersion: '0.75.0' },
  elasticsearch: { appVersion: '8.14.1', chartVersion: '8.5.1' },
  // Loki·OpenSearch·Elasticsearch·Jaeger 의 차트 버전은 domain 상수가 아니라
  // 설치 경로의 분기(helm-values.go 의 resolveChartSpecForStep)에만 있다.
  // 아래 테스트가 못 잡는 값이므로 그쪽을 고칠 때 여기도 함께 본다.
  loki: { appVersion: '2.9.8', chartVersion: '2.10.3' },
}

export function getToolAppVersion(toolId: string): string {
  return TOOL_VERSION_CATALOG[toolId]?.appVersion ?? '1.0.0'
}

export function getToolChartVersion(toolId: string): string | undefined {
  return TOOL_VERSION_CATALOG[toolId]?.chartVersion
}

function normalizeToolSelectionVersion(selection: ToolSelection): ToolSelection {
  if (!selection.tool) {
    return {
      ...selection,
      version: '',
    }
  }
  if (!selection.version || selection.version === 'latest') {
    return {
      ...selection,
      version: getToolAppVersion(selection.tool),
    }
  }
  return selection
}

function normalizeAccessDomain(domain: string): string {
  return domain.trim().replace(/\.intenral$/i, '.internal')
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
  visualizations: ToolSelection[]
}

export interface LoggingConfig {
  search: ToolSelection
  traceLayer: ToolSelection
  traceExporter: ToolSelection
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
  /**
   * 스택이 만드는 모든 PVC 가 사용할 StorageClass.
   * 빈 값은 "클러스터 기본 StorageClass 사용"을 뜻한다. 기본 SC 가 없는
   * 클러스터에서는 반드시 선택해야 하며, 미선택 시 PVC 가 Pending 에 머물러
   * 설치가 멈춘다.
   */
  storageClass: string
}

/** 대상 클러스터에서 조회한 StorageClass 정보 */
export interface StorageClassOption {
  name: string
  provisioner: string
  is_default: boolean
  reclaim_policy: string
  volume_binding_mode: string
}

export interface AccessDomainTlsConfig {
  enabled: boolean
  secretName: string
  secretNamespace: string
  issuerName: string
}

/**
 * 클러스터 밖 소스 저장소(GitHub)에 붙기 위한 입력.
 *
 * personalAccessToken 은 이 초안에만 머문다 — 스택 구성으로 저장되지 않고
 * 배포 요청 본문으로만 나간다. 구성은 평문으로 저장되고 조회 API 로 다시
 * 내려오므로 토큰을 넣으면 스택을 볼 수 있는 누구에게나 노출된다.
 */
export interface SourceControlDraft {
  owner: string
  apiBaseUrl: string
  personalAccessToken: string
}

export interface StackConfigDraft {
  selectedTemplateId: string | null
  clusterId: string | null
  namespace: string
  stackName: string
  accessDomain: string
  accessDomainTls: AccessDomainTlsConfig
  authentication: {
    provider: '' | 'openbao'
  }
  sourceControl: SourceControlDraft
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
  setAuthenticationProvider: (provider: '' | 'openbao') => void
  updateSourceControl: (config: Partial<SourceControlDraft>) => void
  setTool: (
    section: 'artifacts' | 'pipeline' | 'monitoring' | 'logging',
    field: string,
    value: ToolSelection
  ) => void
  setMonitoringVisualizations: (values: ToolSelection[]) => void
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
  authentication: {
    provider: '',
  },
  sourceControl: {
    owner: '',
    apiBaseUrl: '',
    personalAccessToken: '',
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
    visualizations: [{ tool: 'grafana', version: getToolAppVersion('grafana') }],
  },
  logging: {
    search: { tool: 'opensearch', version: getToolAppVersion('opensearch') },
    traceLayer: { tool: 'tempo', version: getToolAppVersion('tempo') },
    traceExporter: emptyToolSelection(),
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
    storageClass: '',
    database: {
      mode: 'create',
      existingRef: 'local-docker-postgres',
      endpoint: 'host.docker.internal:5433',
      resourceName: 'nullus',
      accessSecretRef: '',
      authId: 'nullus',
      authPasswordKey: 'nullus_dev',
      providerOrEngine: 'postgres',
      version: '17',
      size: 'medium',
    },
    objectStorage: {
      mode: 'create',
      existingRef: 'local-docker-minio',
      endpoint: 'http://host.docker.internal:9000',
      resourceName: 'nullus-artifacts',
      accessSecretRef: '',
      authId: 'nullus',
      authPasswordKey: 'nullus_dev',
      providerOrEngine: 'minio',
      version: 'latest',
      size: 'medium',
    },
  },
  activeTab: 'artifacts',
}

function emptyToolSelection(): ToolSelection {
  return { tool: '', version: '' }
}

function buildTemplateDraft(templateId: string, overrides?: Partial<StackConfigDraft>): StackConfigDraft {
  const baseDraft: StackConfigDraft = templateId === 'empty-template-v1'
    ? {
        ...DEFAULT_DRAFT,
        selectedTemplateId: templateId,
        artifacts: {
          packageRegistry: emptyToolSelection(),
          sourceRepository: emptyToolSelection(),
          containerRegistry: emptyToolSelection(),
          storageBackend: emptyToolSelection(),
        },
        pipeline: {
          cicdPlatform: emptyToolSelection(),
          cdTool: emptyToolSelection(),
        },
        monitoring: {
          collection: emptyToolSelection(),
          visualization: emptyToolSelection(),
          visualizations: [],
        },
        logging: {
          search: emptyToolSelection(),
          traceLayer: emptyToolSelection(),
          traceExporter: emptyToolSelection(),
        },
        storage: {
          ...DEFAULT_DRAFT.storage,
          planMode: 'none',
        },
      }
    : {
        ...DEFAULT_DRAFT,
        selectedTemplateId: templateId,
      }

  return migrateDraftToolVersions({ ...baseDraft, ...overrides })
}

function migrateDraftToolVersions(draft: StackConfigDraft): StackConfigDraft {
  const normalizedPrimaryVisualization = normalizeToolSelectionVersion(draft.monitoring.visualization)
  const rawVisualizations = Array.isArray(draft.monitoring.visualizations) ? draft.monitoring.visualizations : []
  const normalizedVisualizations = rawVisualizations
    .map(normalizeToolSelectionVersion)
    .filter((selection) => selection.tool)
  const effectiveVisualizations =
    normalizedVisualizations.length > 0
      ? normalizedVisualizations
      : (normalizedPrimaryVisualization.tool ? [normalizedPrimaryVisualization] : [])

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
      visualization: effectiveVisualizations[0] ?? emptyToolSelection(),
      visualizations: effectiveVisualizations,
    },
    logging: {
      search: normalizeToolSelectionVersion(draft.logging.search),
      traceLayer: normalizeToolSelectionVersion(draft.logging.traceLayer),
      traceExporter: normalizeToolSelectionVersion(draft.logging.traceExporter),
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
          accessDomain: shouldUpdateAccessDomain ? `${name}.internal` : normalizeAccessDomain(s.draft.accessDomain),
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
    set((s) => ({ draft: { ...s.draft, accessDomain: normalizeAccessDomain(domain) }, isDirty: true })),

  updateAccessDomainTls: (config) =>
    set((s) => ({
      draft: { ...s.draft, accessDomainTls: { ...s.draft.accessDomainTls, ...config } },
      isDirty: true,
    })),

  setAuthenticationProvider: (provider) =>
    set((s) => ({
      draft: {
        ...s.draft,
        authentication: {
          ...s.draft.authentication,
          provider,
        },
      },
      isDirty: true,
    })),

  updateSourceControl: (config) =>
    set((s) => ({
      draft: {
        ...s.draft,
        sourceControl: {
          ...s.draft.sourceControl,
          ...config,
        },
      },
      isDirty: true,
    })),

  setTool: (section, field, value) =>
    set((s) => {
      const normalized = normalizeToolSelectionVersion(value)
      const nextSection = {
        ...(s.draft[section] as unknown as Record<string, ToolSelection>),
        [field]: normalized,
      } as Record<string, unknown>

      if (section === 'monitoring' && field === 'visualization') {
        nextSection.visualizations = normalized.tool ? [normalized] : []
      }

      return {
        draft: {
          ...s.draft,
          [section]: nextSection,
        },
        isDirty: true,
      }
    }),

  setMonitoringVisualizations: (values) =>
    set((s) => {
      const normalized = values
        .map(normalizeToolSelectionVersion)
        .filter((selection) => selection.tool)
      return {
        draft: {
          ...s.draft,
          monitoring: {
            ...s.draft.monitoring,
            visualizations: normalized,
            visualization: normalized[0] ?? emptyToolSelection(),
          },
        },
        isDirty: true,
      }
    }),

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
      draft: buildTemplateDraft(templateId, overrides),
      isDirty: false,
    })),

  resetConfig: () => set({ draft: migrateDraftToolVersions(DEFAULT_DRAFT), isDirty: false }),
}))

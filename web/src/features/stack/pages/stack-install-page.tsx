import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { Download, Info, Rocket, Save, ShoppingCart } from 'lucide-react'
import Editor from '@monaco-editor/react'
import type { Monaco } from '@monaco-editor/react'
import { configureMonacoYaml } from 'monaco-yaml'
import YAML from 'yaml'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useStackConfigStore } from '../stores/stack-config-store'
import type {
  InstallTab,
  ToolSelection,
  StackConfigDraft,
  StorageMode,
  StoragePlanMode,
  StorageTargetConfig,
} from '../stores/stack-config-store'
import { getToolAppVersion, getToolChartVersion } from '../stores/stack-config-store'
import { useCreateStack, useDeployStack, useSaveDraft, useClusters, useResourceDefaults } from '../api/stack-api'
import { useClusterNamespaces } from '../../admin/api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { CodePreview } from '../../../components/shared/code-preview'
import { cn } from '../../../lib/utils'
import { useThemeStore } from '../../../stores/theme-store'

// --- Tool option types ---

interface ToolOption {
  id: string
  label: string
  description: string
}

function toDeployErrorMessage(error: unknown): string {
  let code = ''
  let backendMessage = ''
  let status: number | undefined
  let genericMessage = ''

  if (typeof error === 'object' && error !== null) {
    const record = error as Record<string, unknown>
    if (typeof record.message === 'string') {
      genericMessage = record.message
    }
    if (typeof record.status === 'number') {
      status = record.status
    }

    const details = record.details
    if (typeof details === 'object' && details !== null) {
      const detailRecord = details as Record<string, unknown>
      const nestedError = detailRecord.error
      if (typeof nestedError === 'object' && nestedError !== null) {
        const nested = nestedError as Record<string, unknown>
        if (typeof nested.code === 'string') {
          code = nested.code
        }
        if (typeof nested.message === 'string') {
          backendMessage = nested.message
        }
        if (typeof nested.http_status === 'number') {
          status = nested.http_status
        }
      }
    }
  }

  const reason = backendMessage || genericMessage || 'unknown backend error'
  const prefix = code ? `[${code}] ` : ''
  const statusSuffix = status ? ` (HTTP ${status})` : ''
  return `배포 작업 등록 실패: ${prefix}${reason}${statusSuffix}`
}

const ARTIFACTS_OPTIONS: Record<string, ToolOption[]> = {
  packageRegistry: [
    { id: 'gitlab', label: 'GitLab Package Registry', description: 'GitLab 내장 패키지 레지스트리' },
    { id: 'nexus', label: 'Nexus Repository', description: '범용 아티팩트 저장소' },
    { id: 'jfrog', label: 'JFrog Artifactory', description: '엔터프라이즈급 아티팩트 관리' },
  ],
  sourceRepository: [
    { id: 'gitlab', label: 'GitLab', description: 'GitLab 소스 코드 관리' },
    { id: 'github', label: 'GitHub', description: 'GitHub 소스 코드 관리' },
    { id: 'gitea', label: 'Gitea', description: '경량 셀프호스팅 Git 서비스' },
  ],
  containerRegistry: [
    { id: 'gitlab-registry', label: 'GitLab Container Registry', description: 'GitLab 내장 컨테이너 레지스트리' },
    { id: 'harbor', label: 'Harbor', description: '엔터프라이즈 컨테이너 레지스트리' },
    { id: 'docker-hub', label: 'Docker Hub', description: 'Docker 공식 레지스트리' },
  ],
  storageBackend: [
    { id: 'minio', label: 'MinIO', description: 'S3 호환 오브젝트 스토리지' },
    { id: 's3', label: 'AWS S3', description: 'Amazon S3 오브젝트 스토리지' },
    { id: 'gcs', label: 'Google Cloud Storage', description: 'GCP 오브젝트 스토리지' },
  ],
}

const PIPELINE_OPTIONS: Record<string, ToolOption[]> = {
  cicdPlatform: [
    { id: 'gitlab-ci', label: 'GitLab CI/CD', description: 'GitLab 내장 CI/CD 파이프라인' },
    { id: 'github-actions', label: 'GitHub Actions', description: 'GitHub 워크플로우 기반 CI/CD' },
    { id: 'jenkins', label: 'Jenkins', description: '전통적인 오픈소스 CI 서버' },
  ],
  cdTool: [
    { id: 'argocd', label: 'ArgoCD', description: 'GitOps 기반 쿠버네티스 CD' },
    { id: 'flux', label: 'Flux CD', description: 'GitOps 툴킷' },
    { id: 'spinnaker', label: 'Spinnaker', description: '멀티 클라우드 CD 플랫폼' },
  ],
}

const MONITORING_OPTIONS: Record<string, ToolOption[]> = {
  collection: [
    { id: 'prometheus', label: 'Prometheus', description: '시계열 메트릭 수집' },
    { id: 'thanos', label: 'Thanos', description: '장기 보관 및 글로벌 메트릭 집계' },
    { id: 'victoriametrics', label: 'VictoriaMetrics', description: '고성능 시계열 데이터베이스' },
  ],
  visualization: [
    { id: 'grafana', label: 'Grafana', description: '오픈소스 메트릭 시각화' },
    { id: 'kibana', label: 'Kibana', description: 'Elastic Stack 시각화' },
    { id: 'opensearch-dashboards', label: 'OpenSearch Dashboards', description: 'OpenSearch 시각화 대시보드' },
  ],
  traceLayer: [
    { id: 'tempo', label: 'Tempo', description: '분산 추적 백엔드' },
    { id: 'jaeger', label: 'Jaeger', description: '분산 추적 및 트레이스 분석' },
    { id: 'opentelemetry-collector', label: 'OpenTelemetry Collector', description: 'OTLP 수집/처리 파이프라인' },
  ],
}

const LOGGING_OPTIONS: Record<string, ToolOption[]> = {
  search: [
    { id: 'opensearch', label: 'OpenSearch', description: 'Elasticsearch 호환 검색/분석' },
    { id: 'elasticsearch', label: 'Elasticsearch', description: '분산 검색/분석 엔진' },
    { id: 'loki', label: 'Grafana Loki', description: 'Prometheus 스타일 로그 집계' },
  ],
}

const STORAGE_PLAN_MODE_OPTIONS: Array<{ id: StoragePlanMode; label: string; description: string }> = [
  {
    id: 'existing-all',
    label: '기존 DB/Storage 연결',
    description: '조직에서 이미 운영 중인 DB와 Object Storage를 참조하여 연결합니다.',
  },
  {
    id: 'integrated-create',
    label: '통합 DB/Storage 생성 연결',
    description: '설치 시 DB와 Object Storage를 함께 신규 생성하고 자동 연동합니다.',
  },
]

const STORAGE_SIZE_OPTIONS: Array<StorageTargetConfig['size']> = ['small', 'medium', 'large']

const STORAGE_SIZE_RESOURCE_HINTS: Record<
  'database' | 'objectStorage',
  Record<StorageTargetConfig['size'], string>
> = {
  database: {
    small: '(CPU 0.5 / Memory 1Gi / Storage 20Gi)',
    medium: '(CPU 1 / Memory 2Gi / Storage 50Gi)',
    large: '(CPU 2 / Memory 4Gi / Storage 100Gi)',
  },
  objectStorage: {
    small: '(CPU 0.5 / Memory 1Gi / Storage 50Gi)',
    medium: '(CPU 1 / Memory 2Gi / Storage 100Gi)',
    large: '(CPU 2 / Memory 4Gi / Storage 300Gi)',
  },
}

const STORAGE_PROVIDER_OPTIONS: Record<'database' | 'objectStorage', Array<{ id: string; label: string }>> = {
  database: [
    { id: 'postgres', label: 'PostgreSQL' },
    { id: 'mysql', label: 'MySQL' },
    { id: 'mariadb', label: 'MariaDB' },
  ],
  objectStorage: [
    { id: 'minio', label: 'MinIO' },
    { id: 's3', label: 'Amazon S3' },
    { id: 'gcs', label: 'Google Cloud Storage' },
    { id: 'azure-blob', label: 'Azure Blob Storage' },
  ],
}

const TOOL_OPTIONS_ALL = [
  ...Object.values(ARTIFACTS_OPTIONS).flat(),
  ...Object.values(PIPELINE_OPTIONS).flat(),
  ...Object.values(MONITORING_OPTIONS).flat(),
  ...Object.values(LOGGING_OPTIONS).flat(),
]

const TOOL_LABEL_MAP = new Map(TOOL_OPTIONS_ALL.map((opt) => [opt.id, opt.label]))

const TOOL_HELM_META: Record<string, { repoUrl: string; chartName: string }> = {
  gitlab: { repoUrl: 'https://charts.gitlab.io', chartName: 'gitlab/gitlab' },
  nexus: { repoUrl: 'https://sonatype.github.io/helm3-charts', chartName: 'nexus-repository-manager/nexus-repository-manager' },
  jfrog: { repoUrl: 'https://charts.jfrog.io', chartName: 'jfrog/artifactory-oss' },
  github: { repoUrl: 'https://actions-runner-controller.github.io/actions-runner-controller', chartName: 'actions-runner-controller/actions-runner-controller' },
  gitea: { repoUrl: 'https://dl.gitea.io/charts', chartName: 'gitea-charts/gitea' },
  'gitlab-registry': { repoUrl: 'https://charts.gitlab.io', chartName: 'gitlab/container-registry' },
  harbor: { repoUrl: 'https://helm.goharbor.io', chartName: 'harbor/harbor' },
  'docker-hub': { repoUrl: 'https://registry-1.docker.io', chartName: 'dockerhub/proxy-cache' },
  minio: { repoUrl: 'https://charts.min.io', chartName: 'minio/minio' },
  s3: { repoUrl: 'https://aws.github.io/eks-charts', chartName: 'aws/ack-s3-controller' },
  gcs: { repoUrl: 'https://example.storage.google/charts', chartName: 'gcs/storage-gateway' },
  'gitlab-ci': { repoUrl: 'https://charts.gitlab.io', chartName: 'gitlab/gitlab-runner' },
  'github-actions': { repoUrl: 'https://actions-runner-controller.github.io/actions-runner-controller', chartName: 'actions-runner-controller/actions-runner-controller' },
  jenkins: { repoUrl: 'https://charts.jenkins.io', chartName: 'jenkins/jenkins' },
  argocd: { repoUrl: 'https://argoproj.github.io/argo-helm', chartName: 'argo/argo-cd' },
  flux: { repoUrl: 'https://fluxcd-community.github.io/helm-charts', chartName: 'fluxcd/flux2' },
  spinnaker: { repoUrl: 'https://opsmx.github.io/charts', chartName: 'spinnaker/spin' },
  prometheus: { repoUrl: 'https://prometheus-community.github.io/helm-charts', chartName: 'prometheus-community/kube-prometheus-stack' },
  thanos: { repoUrl: 'https://prometheus-community.github.io/helm-charts', chartName: 'prometheus-community/thanos' },
  victoriametrics: { repoUrl: 'https://victoriametrics.github.io/helm-charts', chartName: 'victoria-metrics/victoria-metrics-k8s-stack' },
  grafana: { repoUrl: 'https://grafana.github.io/helm-charts', chartName: 'grafana/grafana' },
  kibana: { repoUrl: 'https://helm.elastic.co', chartName: 'elastic/kibana' },
  'opensearch-dashboards': { repoUrl: 'https://opensearch-project.github.io/helm-charts', chartName: 'opensearch/opensearch-dashboards' },
  tempo: { repoUrl: 'https://grafana.github.io/helm-charts', chartName: 'grafana/tempo' },
  jaeger: { repoUrl: 'https://jaegertracing.github.io/helm-charts', chartName: 'jaegertracing/jaeger' },
  'opentelemetry-collector': {
    repoUrl: 'https://open-telemetry.github.io/opentelemetry-helm-charts',
    chartName: 'open-telemetry/opentelemetry-collector',
  },
  opensearch: { repoUrl: 'https://opensearch-project.github.io/helm-charts', chartName: 'opensearch/opensearch' },
  elasticsearch: { repoUrl: 'https://helm.elastic.co', chartName: 'elastic/elasticsearch' },
  loki: { repoUrl: 'https://grafana.github.io/helm-charts', chartName: 'grafana/loki-stack' },
}

type K8sPreviewTab = 'namespace' | 'deployment' | 'service' | 'gateway'

type PlanningSlot =
  | 'artifacts.packageRegistry'
  | 'artifacts.sourceRepository'
  | 'artifacts.containerRegistry'
  | 'artifacts.storageBackend'
  | 'pipeline.cicdPlatform'
  | 'pipeline.cdTool'
  | 'monitoring.collection'
  | 'monitoring.visualization'
  | 'logging.search'
  | 'logging.traceLayer'

type PlanningProfile = 'startup' | 'standard' | 'enterprise'

type ResourceVector = {
  cpuRequest: number
  cpuLimit: number
  memoryRequestGi: number
  memoryLimitGi: number
  storageRequestGi: number
  storageLimitGi: number
}

type ResourceMultipliers = {
  cpu: number
  memory: number
  storage: number
  raw: {
    cpu: number
    memory: number
    storage: number
  }
  clamped: {
    cpu: boolean
    memory: boolean
    storage: boolean
  }
}

type ResourceUnit = 'Gi' | 'Mi'

type PlanningRowUnit = {
  memory: ResourceUnit
  storage: ResourceUnit
}

type ManifestInstallType = 'helm' | 'yaml'

type ManifestToolEntry = {
  toolId: string
  toolLabel: string
  installType: ManifestInstallType
  toolVersion: string
  chartVersion?: string
  hasVersionConflict: boolean
  roles: string[]
  sourceToolIds: string[]
  sourceVersions: string[]
}

type ToolManifestResourceSpec = {
  requests: { cpu: number; memory: string; storage: string }
  limits: { cpu: number; memory: string; storage: string }
}

type PlanningOptionDefinition = {
  key: string
  label: string
  baseline: number
  min: number
  max: number
  step: number
  weight: number
  impact: {
    cpu: number
    memory: number
    storage: number
  }
}

type StorageTargetKey = 'database' | 'objectStorage'
type StorageFieldKey = 'existingRef' | 'endpoint' | 'resourceName' | 'accessSecretRef' | 'authId' | 'authPasswordKey'
type StorageValidationErrorKey = `${StorageTargetKey}.${StorageFieldKey}`
type StorageValidationErrors = Partial<Record<StorageValidationErrorKey, string>>
type DryRunCheckStatus = 'pass' | 'warn' | 'fail'

type DryRunCheck = {
  id: string
  title: string
  status: DryRunCheckStatus
  detail: string
}

const PLANNING_PROFILE_LABEL: Record<PlanningProfile, string> = {
  startup: 'Startup',
  standard: 'Standard',
  enterprise: 'Enterprise',
}

const STORAGE_ENDPOINT_REGEX = /^((https?:\/\/)[^\s]+|[a-zA-Z0-9.-]+(?::\d{1,5})?)$/
const K8S_SECRET_REF_REGEX = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const SECRET_KEY_REGEX = /^[-._a-zA-Z0-9]+$/

const TOOL_INSTALL_METHOD: Record<string, ManifestInstallType> = {
  grafana: 'yaml',
  prometheus: 'yaml',
  tempo: 'yaml',
  jaeger: 'yaml',
  loki: 'yaml',
}

const TOOL_BUNDLE_CANONICAL: Record<string, string> = {
  'gitlab-registry': 'gitlab',
  'gitlab-ci': 'gitlab',
}

const TOOL_DEFAULT_IMAGE_REPOSITORY: Record<string, string> = {
  prometheus: 'quay.io/prometheus/prometheus',
  grafana: 'docker.io/grafana/grafana',
  loki: 'docker.io/grafana/loki',
  tempo: 'docker.io/grafana/tempo',
  jaeger: 'docker.io/jaegertracing/all-in-one',
  'opentelemetry-collector': 'docker.io/otel/opentelemetry-collector-k8s',
}

function getManifestBundleId(toolId: string): string {
  return TOOL_BUNDLE_CANONICAL[toolId] ?? toolId
}

const SLOT_TOOL_BINDING: Record<PlanningSlot, { section: 'artifacts' | 'pipeline' | 'monitoring' | 'logging'; field: string }> = {
  'artifacts.packageRegistry': { section: 'artifacts', field: 'packageRegistry' },
  'artifacts.sourceRepository': { section: 'artifacts', field: 'sourceRepository' },
  'artifacts.containerRegistry': { section: 'artifacts', field: 'containerRegistry' },
  'artifacts.storageBackend': { section: 'artifacts', field: 'storageBackend' },
  'pipeline.cicdPlatform': { section: 'pipeline', field: 'cicdPlatform' },
  'pipeline.cdTool': { section: 'pipeline', field: 'cdTool' },
  'monitoring.collection': { section: 'monitoring', field: 'collection' },
  'monitoring.visualization': { section: 'monitoring', field: 'visualization' },
  'logging.search': { section: 'logging', field: 'search' },
  'logging.traceLayer': { section: 'logging', field: 'traceLayer' },
}

const GATEWAY_MANIFEST_ID = 'gateway'

const PLANNING_OPTION_DEFS: Record<PlanningSlot, PlanningOptionDefinition[]> = {
  'artifacts.packageRegistry': [
    { key: 'registryCallsPerDay', label: 'Registry 호출 수/일', baseline: 3000, min: 500, max: 50000, step: 500, weight: 0.45, impact: { cpu: 1, memory: 0.8, storage: 0.3 } },
    { key: 'avgArtifactSizeMb', label: '평균 패키지 크기(MB)', baseline: 120, min: 10, max: 2000, step: 10, weight: 0.25, impact: { cpu: 0.1, memory: 0.2, storage: 1 } },
    { key: 'retentionDays', label: '보관 기간(일)', baseline: 30, min: 1, max: 365, step: 1, weight: 0.30, impact: { cpu: 0, memory: 0.1, storage: 1 } },
  ],
  'artifacts.sourceRepository': [
    { key: 'activeRepoUsers', label: '활성 Repo 사용자 수', baseline: 20, min: 5, max: 500, step: 1, weight: 0.4, impact: { cpu: 0.8, memory: 0.6, storage: 0.3 } },
    { key: 'repoCount', label: '관리 저장소 수', baseline: 60, min: 5, max: 3000, step: 5, weight: 0.35, impact: { cpu: 0.2, memory: 0.4, storage: 1 } },
    { key: 'dailyPushEvents', label: '일일 Push 이벤트 수', baseline: 250, min: 20, max: 20000, step: 10, weight: 0.25, impact: { cpu: 1, memory: 0.5, storage: 0.4 } },
  ],
  'artifacts.containerRegistry': [
    { key: 'imagePullsPerDay', label: '이미지 Pull 수/일', baseline: 2000, min: 200, max: 100000, step: 100, weight: 0.4, impact: { cpu: 0.8, memory: 0.7, storage: 0.4 } },
    { key: 'newImagePushesPerDay', label: '신규 이미지 Push 수/일', baseline: 180, min: 10, max: 8000, step: 10, weight: 0.35, impact: { cpu: 0.9, memory: 0.6, storage: 0.8 } },
    { key: 'avgImageSizeGb', label: '평균 이미지 크기(GB)', baseline: 1.2, min: 0.1, max: 20, step: 0.1, weight: 0.25, impact: { cpu: 0.1, memory: 0.2, storage: 1 } },
  ],
  'artifacts.storageBackend': [
    { key: 'objectOpsPerDay', label: 'Object 요청 수/일', baseline: 10000, min: 1000, max: 200000, step: 500, weight: 0.45, impact: { cpu: 0.9, memory: 0.8, storage: 0.4 } },
    { key: 'storedDataTb', label: '저장 데이터(TB)', baseline: 1.5, min: 0.1, max: 100, step: 0.1, weight: 0.35, impact: { cpu: 0.1, memory: 0.2, storage: 1 } },
    { key: 'backupFrequencyPerWeek', label: '주간 백업 횟수', baseline: 7, min: 1, max: 30, step: 1, weight: 0.20, impact: { cpu: 0.4, memory: 0.3, storage: 0.7 } },
  ],
  'pipeline.cicdPlatform': [
    { key: 'developers', label: '개발자 수', baseline: 20, min: 1, max: 1000, step: 1, weight: 0.2, impact: { cpu: 0.4, memory: 0.4, storage: 0.2 } },
    { key: 'concurrentRunners', label: '동시 러너 수', baseline: 4, min: 1, max: 400, step: 1, weight: 0.55, impact: { cpu: 1.8, memory: 1.6, storage: 0.7 } },
    { key: 'dailyCommits', label: '일일 커밋 수', baseline: 120, min: 10, max: 10000, step: 10, weight: 0.25, impact: { cpu: 0.8, memory: 0.6, storage: 0.3 } },
  ],
  'pipeline.cdTool': [
    { key: 'deploymentsPerDay', label: '배포 횟수/일', baseline: 40, min: 1, max: 2000, step: 1, weight: 0.5, impact: { cpu: 0.8, memory: 0.6, storage: 0.2 } },
    { key: 'environmentsCount', label: '운영 환경 수', baseline: 4, min: 1, max: 30, step: 1, weight: 0.25, impact: { cpu: 0.4, memory: 0.5, storage: 0.3 } },
    { key: 'rollbackRatePercent', label: '롤백 비율(%)', baseline: 8, min: 0, max: 80, step: 1, weight: 0.25, impact: { cpu: 0.5, memory: 0.6, storage: 0.2 } },
  ],
  'monitoring.collection': [
    { key: 'metricsTargets', label: '모니터링 타겟 수', baseline: 150, min: 20, max: 5000, step: 5, weight: 0.45, impact: { cpu: 0.7, memory: 0.9, storage: 0.4 } },
    { key: 'scrapeIntervalSec', label: '스크랩 주기(초)', baseline: 30, min: 5, max: 120, step: 1, weight: 0.30, impact: { cpu: -0.6, memory: -0.7, storage: -0.2 } },
    { key: 'retentionDays', label: '메트릭 보관 기간(일)', baseline: 15, min: 1, max: 365, step: 1, weight: 0.25, impact: { cpu: 0, memory: 0.2, storage: 1 } },
  ],
  'monitoring.visualization': [
    { key: 'dashboardUsers', label: '대시보드 사용자 수', baseline: 30, min: 5, max: 2000, step: 1, weight: 0.45, impact: { cpu: 0.5, memory: 0.5, storage: 0.1 } },
    { key: 'dashboardCount', label: '대시보드 수', baseline: 40, min: 5, max: 1500, step: 5, weight: 0.30, impact: { cpu: 0.4, memory: 0.6, storage: 0.2 } },
    { key: 'refreshIntervalSec', label: '대시보드 갱신 주기(초)', baseline: 30, min: 5, max: 300, step: 1, weight: 0.25, impact: { cpu: -0.5, memory: -0.4, storage: -0.1 } },
  ],
  'logging.search': [
    { key: 'logGbPerDay', label: '로그 수집량(GB/일)', baseline: 100, min: 5, max: 10000, step: 5, weight: 0.5, impact: { cpu: 0.6, memory: 0.7, storage: 1 } },
    { key: 'retentionDays', label: '로그 보관 기간(일)', baseline: 30, min: 1, max: 365, step: 1, weight: 0.3, impact: { cpu: 0, memory: 0.2, storage: 1 } },
    { key: 'queryUsers', label: '로그 조회 사용자 수', baseline: 20, min: 1, max: 1000, step: 1, weight: 0.2, impact: { cpu: 0.7, memory: 0.6, storage: 0.2 } },
  ],
  'logging.traceLayer': [
    { key: 'traceSpansPerMin', label: 'Trace Span 수/분', baseline: 50000, min: 1000, max: 3000000, step: 1000, weight: 0.5, impact: { cpu: 0.8, memory: 0.7, storage: 0.5 } },
    { key: 'serviceCount', label: '추적 대상 서비스 수', baseline: 40, min: 5, max: 2000, step: 1, weight: 0.3, impact: { cpu: 0.4, memory: 0.5, storage: 0.3 } },
    { key: 'traceRetentionDays', label: '트레이스 보관 기간(일)', baseline: 7, min: 1, max: 90, step: 1, weight: 0.2, impact: { cpu: 0, memory: 0.2, storage: 1 } },
  ],
}

function round2(value: number): number {
  return Number(value.toFixed(2))
}

function ceil2(value: number): number {
  return Math.ceil(value * 100) / 100
}

function convertGiToUnit(valueGi: number, unit: ResourceUnit): number {
  if (unit === 'Gi') {
    return ceil2(valueGi)
  }
  return ceil2(valueGi * 1024)
}

function convertUnitToGi(value: number, unit: ResourceUnit): number {
  if (unit === 'Gi') {
    return ceil2(value)
  }
  return ceil2(value / 1024)
}

function profileFactorByOption(profile: PlanningProfile, optionKey: string): number {
  if (profile === 'standard') {
    return 1
  }

  const isRetention = optionKey.toLowerCase().includes('retention')
  const isInterval = optionKey.toLowerCase().includes('interval')
  const isConcurrency = optionKey === 'concurrentRunners'
  const isThroughput = /(calls|events|pulls|pushes|ops|deployments|commits|targets|spans|query|users|count)/i.test(optionKey)

  if (profile === 'startup') {
    if (isRetention) return 0.6
    if (isInterval) return 1.35
    if (isConcurrency) return 0.55
    if (isThroughput) return 0.6
    return 0.7
  }

  if (isRetention) return 1.8
  if (isInterval) return 0.7
  if (isConcurrency) return 1.8
  if (isThroughput) return 1.7
  return 1.45
}

function profileAdjustedBaseline(profile: PlanningProfile, def: PlanningOptionDefinition): number {
  const factor = profileFactorByOption(profile, def.key)
  const value = def.baseline * factor
  return Math.min(def.max, Math.max(def.min, ceil2(value)))
}

function calculateMultipliers(slot: PlanningSlot, optionValues: Record<string, number>): ResourceMultipliers {
  const defs = PLANNING_OPTION_DEFS[slot]
  const weighted = defs.reduce(
    (sum, def) => {
    const value = optionValues[def.key] ?? def.baseline
      const delta = (value - def.baseline) / def.baseline
      return {
        cpu: sum.cpu + delta * def.weight * def.impact.cpu,
        memory: sum.memory + delta * def.weight * def.impact.memory,
        storage: sum.storage + delta * def.weight * def.impact.storage,
      }
    },
    { cpu: 0, memory: 0, storage: 0 }
  )

  const clampMax =
    slot === 'pipeline.cicdPlatform'
      ? { cpu: 6, memory: 6, storage: 4 }
      : { cpu: 3, memory: 3, storage: 3 }

  const clamp = (value: number, max: number) => Math.min(max, Math.max(0.5, value))
  let rawCpu = 1 + weighted.cpu
  let rawMemory = 1 + weighted.memory
  const rawStorage = 1 + weighted.storage

  if (slot === 'pipeline.cicdPlatform') {
    const runnerDef = defs.find((def) => def.key === 'concurrentRunners')
    if (runnerDef) {
      const runners = optionValues.concurrentRunners ?? runnerDef.baseline
      const ratio = Math.max(0.25, runners / runnerDef.baseline)
      const runnerCpuBoost = Math.pow(ratio, 0.6)
      const runnerMemoryBoost = Math.pow(ratio, 0.55)

      rawCpu *= runnerCpuBoost
      rawMemory *= runnerMemoryBoost
    }
  }

  return {
    cpu: clamp(rawCpu, clampMax.cpu),
    memory: clamp(rawMemory, clampMax.memory),
    storage: clamp(rawStorage, clampMax.storage),
    raw: {
      cpu: rawCpu,
      memory: rawMemory,
      storage: rawStorage,
    },
    clamped: {
      cpu: rawCpu !== clamp(rawCpu, clampMax.cpu),
      memory: rawMemory !== clamp(rawMemory, clampMax.memory),
      storage: rawStorage !== clamp(rawStorage, clampMax.storage),
    },
  }
}

function applyMultipliers(base: {
  cpu_request: number
  cpu_limit: number
  memory_request_gi: number
  memory_limit_gi: number
  storage_request_gi: number
  storage_limit_gi: number
}, multipliers: Pick<ResourceMultipliers, 'cpu' | 'memory' | 'storage'>): ResourceVector {
  return {
    cpuRequest: round2(base.cpu_request * multipliers.cpu),
    cpuLimit: round2(base.cpu_limit * multipliers.cpu),
    memoryRequestGi: round2(base.memory_request_gi * multipliers.memory),
    memoryLimitGi: round2(base.memory_limit_gi * multipliers.memory),
    storageRequestGi: round2(base.storage_request_gi * multipliers.storage),
    storageLimitGi: round2(base.storage_limit_gi * multipliers.storage),
  }
}

function buildFormulaTooltip(toolLabelValue: string, defs: PlanningOptionDefinition[]): string {
  const clampText = '최종 배수는 최소 0.5배이며, 상한은 슬롯별로 적용됩니다(CI/CD CPU/MEM 최대 6배).' 
  const lines = [
    `${toolLabelValue} 리소스 산정 가이드`,
    '',
    '1) 기본값에서 얼마나 바뀌었는지 계산합니다.',
    '   변화율(Δ) = (입력값 - 기본값) / 기본값',
    '',
    '2) 각 옵션의 영향도를 CPU/Memory/Storage에 따로 반영합니다.',
    '   - w: 옵션 중요도(가중치)',
    '   - a: CPU 영향도',
    '   - m: Memory 영향도',
    '   - s: Storage 영향도',
    '   - 값이 클수록 해당 자원에 더 크게 반영됩니다.',
    '   - 음수면(예: interval) 값이 커질수록 부하가 줄어듭니다.',
    '',
    '3) 추천값 계산식',
    '   CPU 추천 = 기본 CPU × (1 + Σ(w × a × Δ))',
    '   MEM 추천 = 기본 MEM × (1 + Σ(w × m × Δ))',
    '   STO 추천 = 기본 STO × (1 + Σ(w × s × Δ))',
    `   ${clampText}`,
    '',
    '4) 적용값',
    '   - 처음에는 추천값으로 자동 세팅됩니다.',
    '   - 이후 직접 수정할 수 있습니다.',
    '   - 플래닝 옵션을 다시 바꾸면 추천값 기준으로 재설정됩니다.',
    '',
    '옵션별 계수:',
  ]

  defs.forEach((def) => {
    lines.push(`${def.label}: w=${def.weight}, a=${def.impact.cpu}, m=${def.impact.memory}, s=${def.impact.storage}`)
  })

  if (defs.some((def) => def.key === 'concurrentRunners')) {
    lines.push('')
    lines.push('추가 규칙(CI/CD): 동시 러너 수는 CPU/MEM에 배수 계수로 추가 반영됩니다.')
    lines.push('CPU 추가 배수 = (동시러너 / 기준러너)^0.6')
    lines.push('MEM 추가 배수 = (동시러너 / 기준러너)^0.55')
  }

  return lines.join('\n')
}

const stackInstallSchema = z.object({
  stackName: z
    .string()
    .min(2, 'Stack name must be at least 2 characters')
    .max(50, 'Stack name must be 50 characters or less')
    .regex(/^[a-zA-Z0-9-]+$/, 'Stack name can include only letters, numbers, and hyphens'),
})

type StackInstallFormData = z.infer<typeof stackInstallSchema>

function toolLabel(toolId: string): string {
  return TOOL_LABEL_MAP.get(toolId) ?? toolId
}

function getHelmMeta(toolId: string) {
  return TOOL_HELM_META[toolId] ?? { repoUrl: 'https://charts.example.com', chartName: `nullus/${toolId}` }
}

function buildDefaultStackName(now = new Date()): string {
  const pad = (value: number) => value.toString().padStart(2, '0')
  const year = now.getFullYear()
  const month = pad(now.getMonth() + 1)
  const day = pad(now.getDate())
  const hour = pad(now.getHours())
  const minute = pad(now.getMinutes())
  const second = pad(now.getSeconds())
  return `nullus-devsecops-stack-${year}${month}${day}-${hour}${minute}${second}`
}

function getInstallType(toolId: string): ManifestInstallType {
  if (TOOL_INSTALL_METHOD[toolId]) return TOOL_INSTALL_METHOD[toolId]
  return TOOL_HELM_META[toolId] ? 'helm' : 'yaml'
}

function resolveToolImage(toolId: string, toolVersion: string): string {
  const repository = TOOL_DEFAULT_IMAGE_REPOSITORY[toolId] ?? `ghcr.io/cloud-nullus/${toolId}`
  const version = toolVersion || getToolAppVersion(toolId)
  return `${repository}:${version}`
}

function buildToolManifest(
  toolId: string,
  toolLabelValue: string,
  draft: StackConfigDraft,
  resources: ResourceVector,
  toolVersion: string,
  chartVersion?: string
): string {
  const installType = getInstallType(toolId)
  const helmMeta = getHelmMeta(toolId)
  const namespace = draft.namespace.trim() || 'nullus'
  const resourcesSpec: ToolManifestResourceSpec = {
    requests: {
      cpu: resources.cpuRequest,
      memory: `${resources.memoryRequestGi.toFixed(2)}Gi`,
      storage: `${resources.storageRequestGi.toFixed(2)}Gi`,
    },
    limits: {
      cpu: resources.cpuLimit,
      memory: `${resources.memoryLimitGi.toFixed(2)}Gi`,
      storage: `${resources.storageLimitGi.toFixed(2)}Gi`,
    },
  }

  if (installType === 'helm') {
    const valuesYaml = {
      global: {
        stackName: draft.stackName,
        accessDomain: draft.accessDomain || `${draft.stackName}.internal`,
        clusterId: draft.clusterId ?? '',
        namespace,
        toolId,
        toolLabel: toolLabelValue,
      },
      chart: {
        repoUrl: helmMeta.repoUrl,
        name: helmMeta.chartName,
        version: chartVersion || getToolChartVersion(toolId) || toolVersion,
      },
      image: {
        tag: toolVersion || getToolAppVersion(toolId),
      },
      resources: resourcesSpec,
      storage: {
        planMode: draft.storage.planMode,
        database: {
          mode: draft.storage.database.mode,
          existingRef: draft.storage.database.existingRef,
          endpoint: draft.storage.database.endpoint,
          resourceName: draft.storage.database.resourceName,
          accessSecretRef: draft.storage.database.accessSecretRef,
          authId: draft.storage.database.authId,
          authPasswordKey: draft.storage.database.authPasswordKey,
          providerOrEngine: draft.storage.database.providerOrEngine,
          version: draft.storage.database.version,
          size: draft.storage.database.size,
        },
        objectStorage: {
          mode: draft.storage.objectStorage.mode,
          existingRef: draft.storage.objectStorage.existingRef,
          endpoint: draft.storage.objectStorage.endpoint,
          resourceName: draft.storage.objectStorage.resourceName,
          accessSecretRef: draft.storage.objectStorage.accessSecretRef,
          authId: draft.storage.objectStorage.authId,
          authPasswordKey: draft.storage.objectStorage.authPasswordKey,
          providerOrEngine: draft.storage.objectStorage.providerOrEngine,
          version: draft.storage.objectStorage.version,
          size: draft.storage.objectStorage.size,
        },
      },
    }

    return YAML.stringify(valuesYaml, { indent: 2, lineWidth: 0 })
  }

  const deployment = {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: toolId,
      namespace,
      labels: {
        app: toolId,
        'nullus.io/stack-name': draft.stackName,
        'nullus.io/cluster-id': draft.clusterId ?? '',
        'nullus.io/tool-id': toolId,
      },
    },
    spec: {
      replicas: 1,
      selector: { matchLabels: { app: toolId } },
      template: {
        metadata: { labels: { app: toolId } },
        spec: {
          containers: [
            {
              name: toolId,
              image: resolveToolImage(toolId, toolVersion),
              resources: {
                requests: {
                  cpu: String(resources.cpuRequest),
                  memory: resourcesSpec.requests.memory,
                },
                limits: {
                  cpu: String(resources.cpuLimit),
                  memory: resourcesSpec.limits.memory,
                },
              },
            },
          ],
        },
      },
    },
  }

  const service = {
    apiVersion: 'v1',
    kind: 'Service',
    metadata: {
      name: `${toolId}-svc`,
      namespace,
      labels: { app: toolId },
    },
    spec: {
      selector: { app: toolId },
      ports: [{ name: 'http', port: 80, targetPort: 8080 }],
    },
  }

  return [YAML.stringify(deployment, { indent: 2, lineWidth: 0 }), YAML.stringify(service, { indent: 2, lineWidth: 0 })]
    .join('\n---\n')
}

function buildGatewayManifest(draft: StackConfigDraft, manifestTools: ManifestToolEntry[]): string {
  const namespace = draft.namespace.trim() || 'nullus'
  const stackName = draft.stackName || 'nullus-stack'
  const accessDomain = draft.accessDomain || `${stackName}.internal`
  const gatewayName = `${stackName}-gateway`
  const tlsEnabled = draft.accessDomainTls.enabled
  const tlsSecretName = draft.accessDomainTls.secretName.trim()
  const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim()
  const tlsIssuerName = draft.accessDomainTls.issuerName.trim() || 'nullus-ca-issuer'
  const requiresReferenceGrant = tlsEnabled && tlsSecretName.length > 0 && tlsSecretNamespace.length > 0 && tlsSecretNamespace !== namespace

  const rules = manifestTools
    .filter((tool) => tool.toolId !== GATEWAY_MANIFEST_ID)
    .map((tool) => ({
      host: `${tool.toolId}.${accessDomain}`,
      http: {
        paths: [
          {
            path: '/',
            pathType: 'Prefix',
            backend: {
              service: {
                name: `${tool.toolId}-svc`,
                port: { number: 80 },
              },
            },
          },
        ],
      },
    }))

  const gateway = {
    apiVersion: 'gateway.networking.k8s.io/v1',
    kind: 'Gateway',
    metadata: {
      name: gatewayName,
      namespace,
      labels: {
        'nullus.io/stack-name': stackName,
        'nullus.io/type': 'gateway',
      },
    },
    spec: {
      gatewayClassName: 'nginx',
      listeners: [
        {
          name: 'http',
          protocol: 'HTTP',
          port: 80,
          hostname: `*.${accessDomain}`,
          allowedRoutes: {
            namespaces: {
              from: 'Same',
            },
          },
        },
        ...(tlsEnabled && tlsSecretName
          ? [
              {
                name: 'https',
                protocol: 'HTTPS',
                port: 443,
                hostname: `*.${accessDomain}`,
                tls: {
                  mode: 'Terminate',
                  certificateRefs: [
                    {
                      kind: 'Secret',
                      name: tlsSecretName,
                      ...(tlsSecretNamespace ? { namespace: tlsSecretNamespace } : {}),
                    },
                  ],
                },
                allowedRoutes: {
                  namespaces: {
                    from: 'Same',
                  },
                },
              },
            ]
          : []),
      ],
    },
  }

  const httpRoutes = rules.map((rule) => {
    const host = rule.host
    const backendServiceName = rule.http.paths[0]?.backend?.service?.name

    return {
      apiVersion: 'gateway.networking.k8s.io/v1',
      kind: 'HTTPRoute',
      metadata: {
        name: `${String(backendServiceName).replace(/-svc$/, '')}-route`,
        namespace,
        labels: {
          'nullus.io/stack-name': stackName,
          'nullus.io/type': 'gateway-route',
        },
      },
      spec: {
        parentRefs: [
          {
            name: gatewayName,
          },
        ],
        hostnames: [host],
        rules: [
          {
            matches: [
              {
                path: {
                  type: 'PathPrefix',
                  value: '/',
                },
              },
            ],
            backendRefs: [
              {
                name: backendServiceName,
                port: 80,
              },
            ],
          },
        ],
      },
    }
  })

  const referenceGrant = requiresReferenceGrant
    ? {
        apiVersion: 'gateway.networking.k8s.io/v1beta1',
        kind: 'ReferenceGrant',
        metadata: {
          name: `${stackName}-tls-secret-grant`,
          namespace: tlsSecretNamespace,
          labels: {
            'nullus.io/stack-name': stackName,
            'nullus.io/type': 'gateway-reference-grant',
          },
        },
        spec: {
          from: [
            {
              group: 'gateway.networking.k8s.io',
              kind: 'Gateway',
              namespace,
            },
          ],
          to: [
            {
              group: '',
              kind: 'Secret',
              name: tlsSecretName,
            },
          ],
        },
      }
    : null

  const certificate = tlsEnabled && tlsSecretName
    ? {
        apiVersion: 'cert-manager.io/v1',
        kind: 'Certificate',
        metadata: {
          name: `${stackName}-wildcard-cert`,
          namespace: tlsSecretNamespace || namespace,
          labels: {
            'nullus.io/stack-name': stackName,
            'nullus.io/type': 'gateway-certificate',
          },
        },
        spec: {
          secretName: tlsSecretName,
          commonName: accessDomain,
          dnsNames: [accessDomain, `*.${accessDomain}`],
          issuerRef: {
            name: tlsIssuerName,
            kind: 'ClusterIssuer',
          },
        },
      }
    : null

  const gatewayDocuments = [
    YAML.stringify(gateway, { indent: 2, lineWidth: 0 }),
    ...httpRoutes.map((route) => YAML.stringify(route, { indent: 2, lineWidth: 0 })),
    ...(certificate ? [YAML.stringify(certificate, { indent: 2, lineWidth: 0 })] : []),
    ...(referenceGrant ? [YAML.stringify(referenceGrant, { indent: 2, lineWidth: 0 })] : []),
  ]

  return gatewayDocuments.join('\n---\n')
}

function createDeployScript(
  draft: StackConfigDraft,
  manifestTools: ManifestToolEntry[],
  manifestByTool: Record<string, string>
): string {
  const stackName = draft.stackName || 'nullus-stack'
  const namespace = draft.namespace.trim() || 'nullus'
  const accessDomain = draft.accessDomain || `${stackName}.internal`
  const clusterContext = draft.clusterId ?? ''
  const tlsEnabled = draft.accessDomainTls.enabled
  const tlsSecretName = draft.accessDomainTls.secretName.trim() || `${stackName}-wildcard-tls`
  const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim() || namespace
  const tlsIssuerName = draft.accessDomainTls.issuerName.trim() || 'nullus-ca-issuer'

  const deployBlocks = manifestTools.flatMap((tool, index) => {
    const blockHeader = [`# ${index + 1}. ${toolLabel(tool.toolId)} (${tool.installType.toUpperCase()})`, `# roles: ${tool.roles.join(', ')}`]
    const manifestText = (manifestByTool[tool.toolId] ?? '').trimEnd()

    if (tool.installType === 'helm') {
      const meta = getHelmMeta(tool.toolId)
      const chartVersion = tool.chartVersion || getToolChartVersion(tool.toolId) || tool.toolVersion
      const valuesPath = `.nullus/generated-values/${tool.toolId}.values.yaml`
      const delimiter = `NULLUS_VALUES_EOF_${index + 1}`
      return [
        ...blockHeader,
        `cat <<'${delimiter}' > "${valuesPath}"`,
        ...manifestText.split('\n'),
        delimiter,
        `helm repo add ${tool.toolId} ${meta.repoUrl}`,
        `helm upgrade --install ${tool.toolId} ${meta.chartName} --namespace ${namespace} --create-namespace -f "${valuesPath}" --version ${chartVersion}`,
        '',
      ]
    }

    const manifestPath = `.nullus/generated-manifests/${tool.toolId}.yaml`
    const delimiter = `NULLUS_MANIFEST_EOF_${index + 1}`

    return [
      ...blockHeader,
      `cat <<'${delimiter}' > "${manifestPath}"`,
      ...manifestText.split('\n'),
      delimiter,
      `kubectl apply -n ${namespace} -f "${manifestPath}"`,
      '',
    ]
  })

  return [
    '#!/usr/bin/env bash',
    '# Nullus Stack Deploy Script (generated from current Stack Install options)',
    `# stack: ${stackName}`,
    `# namespace: ${namespace}`,
    `# access-domain: ${accessDomain}`,
    `# access-domain-tls: ${tlsEnabled ? `enabled (${tlsSecretNamespace}/${tlsSecretName})` : 'disabled'}`,
    `# cluster-context: ${clusterContext || '(current context)'}`,
    '',
    'set -euo pipefail',
    '',
    clusterContext ? `kubectl config use-context "${clusterContext}"` : '# using current kubectl context',
    `kubectl create namespace ${namespace} --dry-run=client -o yaml | kubectl apply -f -`,
    'mkdir -p .nullus/generated-values .nullus/generated-manifests',
    '',
    '# storage plan',
    `# plan_mode=${draft.storage.planMode}`,
    `# database=${draft.storage.database.mode}:${draft.storage.database.providerOrEngine}:${draft.storage.database.version}`,
    `# object_storage=${draft.storage.objectStorage.mode}:${draft.storage.objectStorage.providerOrEngine}:${draft.storage.objectStorage.version}`,
    '',
    '# resource planning inputs',
    `# developers=${draft.resources.developerCount}, runners=${draft.resources.concurrentRunners}, commitsPerDay=${draft.resources.commitsPerDay}`,
    `# buildFrequency=${draft.resources.buildFrequency}, currency=${draft.resources.currency}`,
    '',
    ...(tlsEnabled
      ? [
          '# TLS via cert-manager (manual openssl/secret creation is not required)',
          `cat <<'NULLUS_TLS_CERT_EOF' > ".nullus/generated-manifests/${stackName}-tls-certificate.yaml"`,
          'apiVersion: cert-manager.io/v1',
          'kind: Certificate',
          'metadata:',
          `  name: ${stackName}-wildcard-cert`,
          `  namespace: ${tlsSecretNamespace}`,
          'spec:',
          `  secretName: ${tlsSecretName}`,
          `  commonName: ${accessDomain}`,
          '  dnsNames:',
          `    - ${accessDomain}`,
          `    - *.${accessDomain}`,
          '  issuerRef:',
          `    name: ${tlsIssuerName}`,
          '    kind: ClusterIssuer',
          'NULLUS_TLS_CERT_EOF',
          `kubectl apply -f ".nullus/generated-manifests/${stackName}-tls-certificate.yaml"`,
          ...(tlsSecretNamespace !== namespace
            ? [
              '# If TLS Secret namespace differs from Gateway namespace, apply ReferenceGrant too.',
                `cat <<'NULLUS_TLS_REFGRANT_EOF' > ".nullus/generated-manifests/${stackName}-tls-reference-grant.yaml"`,
                'apiVersion: gateway.networking.k8s.io/v1beta1',
                'kind: ReferenceGrant',
                'metadata:',
                `  name: ${stackName}-tls-secret-grant`,
                `  namespace: ${tlsSecretNamespace}`,
                'spec:',
                '  from:',
                '    - group: gateway.networking.k8s.io',
                '      kind: Gateway',
                `      namespace: ${namespace}`,
                '  to:',
                '    - group: ""',
                '      kind: Secret',
                `      name: ${tlsSecretName}`,
                'NULLUS_TLS_REFGRANT_EOF',
                `kubectl apply -f ".nullus/generated-manifests/${stackName}-tls-reference-grant.yaml"`,
              ]
            : []),
          '',
        ]
      : []),
    ...deployBlocks,
    'echo "✅ Nullus stack deploy script completed"',
  ].join('\n')
}

function createK8sObjects(draft: StackConfigDraft): Record<K8sPreviewTab, string> {
  const appName = draft.stackName || 'nullus-stack'
  const serviceName = `${appName}-svc`
  const gatewayNamespace = 'nullus-stack'
  const accessDomain = draft.accessDomain || `${appName}.internal`
  const gatewayName = `${appName}-gateway`
  const tlsEnabled = draft.accessDomainTls.enabled
  const tlsSecretName = draft.accessDomainTls.secretName.trim() || `${appName}-wildcard-tls`
  const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim() || gatewayNamespace
  const tlsIssuerName = draft.accessDomainTls.issuerName.trim() || 'nullus-ca-issuer'
  const requiresReferenceGrant = tlsEnabled && tlsSecretNamespace !== gatewayNamespace

  return {
    namespace: [
      'apiVersion: v1',
      'kind: Namespace',
      'metadata:',
      '  name: nullus-stack',
      '  labels:',
      '    app.kubernetes.io/managed-by: nullus',
      `    nullus.io/stack: ${appName}`,
    ].join('\n'),
    deployment: [
      'apiVersion: apps/v1',
      'kind: Deployment',
      'metadata:',
      `  name: ${appName}`,
      '  namespace: nullus-stack',
      '  labels:',
      `    app: ${appName}`,
      'spec:',
      '  replicas: 2',
      '  selector:',
      '    matchLabels:',
      `      app: ${appName}`,
      '  template:',
      '    metadata:',
      '      labels:',
      `        app: ${appName}`,
      '    spec:',
      '      containers:',
      `        - name: ${draft.pipeline.cicdPlatform.tool}`,
      `          image: ghcr.io/nullus/${draft.pipeline.cicdPlatform.tool}:${draft.pipeline.cicdPlatform.version || getToolAppVersion(draft.pipeline.cicdPlatform.tool)}`,
      '          ports:',
      '            - containerPort: 8080',
      '        - name: metrics-sidecar',
      `          image: ghcr.io/nullus/${draft.monitoring.collection.tool}:${draft.monitoring.collection.version || getToolAppVersion(draft.monitoring.collection.tool)}`,
      '          ports:',
      '            - containerPort: 9090',
    ].join('\n'),
    service: [
      'apiVersion: v1',
      'kind: Service',
      'metadata:',
      `  name: ${serviceName}`,
      '  namespace: nullus-stack',
      'spec:',
      '  selector:',
      `    app: ${appName}`,
      '  ports:',
      '    - name: http',
      '      protocol: TCP',
      '      port: 80',
      '      targetPort: 8080',
      '  type: ClusterIP',
    ].join('\n'),
    gateway: [
      'apiVersion: gateway.networking.k8s.io/v1',
      'kind: Gateway',
      'metadata:',
      `  name: ${gatewayName}`,
      `  namespace: ${gatewayNamespace}`,
      'spec:',
      '  gatewayClassName: nginx',
      '  listeners:',
      '    - name: http',
      '      protocol: HTTP',
      '      port: 80',
      `      hostname: *.${accessDomain}`,
      '      allowedRoutes:',
      '        namespaces:',
      '          from: Same',
      ...(tlsEnabled
        ? [
            '    - name: https',
            '      protocol: HTTPS',
            '      port: 443',
            `      hostname: *.${accessDomain}`,
            '      tls:',
            '        mode: Terminate',
            '        certificateRefs:',
            `          - kind: Secret`,
            `            name: ${tlsSecretName}`,
            `            namespace: ${tlsSecretNamespace}`,
            '      allowedRoutes:',
            '        namespaces:',
            '          from: Same',

            '---',
            'apiVersion: cert-manager.io/v1',
            'kind: Certificate',
            'metadata:',
            `  name: ${appName}-wildcard-cert`,
            `  namespace: ${tlsSecretNamespace}`,
            'spec:',
            `  secretName: ${tlsSecretName}`,
            `  commonName: ${accessDomain}`,
            '  dnsNames:',
            `    - ${accessDomain}`,
            `    - *.${accessDomain}`,
            '  issuerRef:',
            `    name: ${tlsIssuerName}`,
            '    kind: ClusterIssuer',
          ]
        : []),
      '---',
      'apiVersion: gateway.networking.k8s.io/v1',
      'kind: HTTPRoute',
      'metadata:',
      `  name: ${appName}-route`,
      `  namespace: ${gatewayNamespace}`,
      'spec:',
      '  parentRefs:',
      `    - name: ${gatewayName}`,
      '  hostnames:',
      `    - ${appName}.${accessDomain}`,
      '  rules:',
      '    - matches:',
      '        - path:',
      '            type: PathPrefix',
      '            value: /',
      '      backendRefs:',
      `        - name: ${serviceName}`,
      '          port: 80',
      ...(requiresReferenceGrant
        ? [
            '---',
            'apiVersion: gateway.networking.k8s.io/v1beta1',
            'kind: ReferenceGrant',
            'metadata:',
            `  name: ${appName}-tls-secret-grant`,
            `  namespace: ${tlsSecretNamespace}`,
            'spec:',
            '  from:',
            '    - group: gateway.networking.k8s.io',
            '      kind: Gateway',
            `      namespace: ${gatewayNamespace}`,
            '  to:',
            '    - group: ""',
            '      kind: Secret',
            `      name: ${tlsSecretName}`,
          ]
        : []),
    ].join('\n'),
  }
}

// --- ToolSelector component ---

interface ToolSelectorProps {
  label: string
  options: ToolOption[]
  value: ToolSelection
  onChange: (v: ToolSelection) => void
}

function ToolSelector({ label, options, value, onChange }: ToolSelectorProps) {
  return (
    <div className="mb-5">
      <div className="mb-2.5 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
        {label}
      </div>
      <div className="flex flex-col gap-2">
        {options.map((opt) => {
          const selected = value.tool === opt.id
          return (
            <button
              key={opt.id}
              type="button"
              onClick={() => onChange({ tool: opt.id, version: 'latest' })}
              className={cn(
                'flex w-full cursor-pointer items-center gap-3 rounded-lg border px-[14px] py-3 text-left transition-all duration-150',
                selected
                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
              )}
            >
              <div
                className={cn(
                  'flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2',
                  selected
                    ? 'border-[#6366f1] bg-[#6366f1]'
                    : 'border-[var(--color-border-hover)] bg-transparent'
                )}
              >
                {selected && <div className="h-1.5 w-1.5 rounded-full bg-white" />}
              </div>
              <div>
                <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                  {opt.label}
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">{opt.description}</div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}

// --- Tab definitions ---

const TABS: { id: InstallTab; label: string }[] = [
  { id: 'artifacts', label: 'Artifacts' },
  { id: 'pipeline', label: 'CI/CD' },
  { id: 'monitoring', label: 'Observability' },
  { id: 'resources', label: 'Resources' },
  { id: 'storage', label: 'Storage' },
  { id: 'manifests', label: 'YAML View' },
  { id: 'deploy-script', label: 'Preview Deploy Script' },
  { id: 'dry-run', label: 'Dry Run' },
]

// --- Main page ---

export function StackInstallPage() {
  const navigate = useNavigate()
  const theme = useThemeStore((state) => state.theme)
  const isDarkMode = theme === 'dark'
  const {
    draft,
    setActiveTab,
    setTool,
    setStackName,
    setAccessDomain,
    setCluster,
    setNamespace,
    updateStorage,
    updateStorageTarget,
    updateAccessDomainTls,
  } = useStackConfigStore()
  const createStack = useCreateStack()
  const deployStack = useDeployStack()
  const saveDraft = useSaveDraft()
  const { data: resourceDefaultsData } = useResourceDefaults()
  const { data: clusters } = useClusters()
  const { data: namespaces } = useClusterNamespaces(draft.clusterId ?? '')
  const [createNewNs, setCreateNewNs] = useState(false)
  const [activeTab, setLocalTab] = useState<InstallTab>(draft.activeTab)
  const [k8sPreviewModalOpen, setK8sPreviewModalOpen] = useState(false)
  const [activeK8sPreviewTab, setActiveK8sPreviewTab] = useState<K8sPreviewTab>('namespace')
  const [planningProfile, setPlanningProfile] = useState<PlanningProfile>('standard')
  const [planningOptionOverrides, setPlanningOptionOverrides] = useState<Record<string, Record<string, number>>>({})
  const [appliedResourceOverrides, setAppliedResourceOverrides] = useState<Record<string, ResourceVector>>({})
  const [planningRowUnits, setPlanningRowUnits] = useState<Record<string, PlanningRowUnit>>({})
  const [activeFormulaPopoverKey, setActiveFormulaPopoverKey] = useState<string | null>(null)
  const [storageValidationErrors, setStorageValidationErrors] = useState<StorageValidationErrors>({})
  const [tabGuardError, setTabGuardError] = useState<string | null>(null)
  const [manifestDraftByTool, setManifestDraftByTool] = useState<Record<string, string>>({})
  const [manifestOverridesByTool, setManifestOverridesByTool] = useState<Record<string, string>>({})
  const [manifestErrorsByTool, setManifestErrorsByTool] = useState<Record<string, string>>({})
  const [activeManifestTool, setActiveManifestTool] = useState<string | null>(null)
  const [dryRunExecutedAt, setDryRunExecutedAt] = useState<string | null>(null)
  const manifestSyncTimerRef = useRef<number | null>(null)
  const monacoConfiguredRef = useRef(false)
  const initializedDefaultStackNameRef = useRef(false)
  const stackNameInputRef = useRef<HTMLInputElement | null>(null)
  const clusterSelectRef = useRef<HTMLSelectElement | null>(null)
  const namespaceSelectRef = useRef<HTMLSelectElement | null>(null)
  const newNamespaceInputRef = useRef<HTMLInputElement | null>(null)
  const {
    control,
    trigger,
    setValue,
    formState: { errors, isValid, isSubmitting },
  } = useForm<StackInstallFormData>({
    resolver: zodResolver(stackInstallSchema),
    defaultValues: {
      stackName: draft.stackName,
    },
    mode: 'onChange',
  })

  const effectiveNamespace = createNewNs ? draft.namespace.trim() : draft.namespace.trim() || 'nullus'

  const k8sObjects = createK8sObjects(draft)

  const selectedInstallItems = ([
    {
      slot: 'artifacts.packageRegistry',
      category: 'Artifacts > Package Registry',
      toolKey: draft.artifacts.packageRegistry.tool,
      toolLabel: toolLabel(draft.artifacts.packageRegistry.tool),
      toolVersion: draft.artifacts.packageRegistry.version,
    },
    {
      slot: 'artifacts.sourceRepository',
      category: 'Artifacts > Source Repository',
      toolKey: draft.artifacts.sourceRepository.tool,
      toolLabel: toolLabel(draft.artifacts.sourceRepository.tool),
      toolVersion: draft.artifacts.sourceRepository.version,
    },
    {
      slot: 'artifacts.containerRegistry',
      category: 'Artifacts > Container Registry',
      toolKey: draft.artifacts.containerRegistry.tool,
      toolLabel: toolLabel(draft.artifacts.containerRegistry.tool),
      toolVersion: draft.artifacts.containerRegistry.version,
    },
    {
      slot: 'artifacts.storageBackend',
      category: 'Artifacts > Storage Backend',
      toolKey: draft.artifacts.storageBackend.tool,
      toolLabel: toolLabel(draft.artifacts.storageBackend.tool),
      toolVersion: draft.artifacts.storageBackend.version,
    },
    {
      slot: 'pipeline.cicdPlatform',
      category: 'CI/CD > Platform',
      toolKey: draft.pipeline.cicdPlatform.tool,
      toolLabel: toolLabel(draft.pipeline.cicdPlatform.tool),
      toolVersion: draft.pipeline.cicdPlatform.version,
    },
    {
      slot: 'pipeline.cdTool',
      category: 'CI/CD > CD Tool',
      toolKey: draft.pipeline.cdTool.tool,
      toolLabel: toolLabel(draft.pipeline.cdTool.tool),
      toolVersion: draft.pipeline.cdTool.version,
    },
    {
      slot: 'monitoring.collection',
      category: 'Observability > Metrics Collection',
      toolKey: draft.monitoring.collection.tool,
      toolLabel: toolLabel(draft.monitoring.collection.tool),
      toolVersion: draft.monitoring.collection.version,
    },
    {
      slot: 'monitoring.visualization',
      category: 'Observability > Visualization',
      toolKey: draft.monitoring.visualization.tool,
      toolLabel: toolLabel(draft.monitoring.visualization.tool),
      toolVersion: draft.monitoring.visualization.version,
    },
    {
      slot: 'logging.search',
      category: 'Observability > Logging/Search',
      toolKey: draft.logging.search.tool,
      toolLabel: toolLabel(draft.logging.search.tool),
      toolVersion: draft.logging.search.version,
    },
    {
      slot: 'logging.traceLayer',
      category: 'Observability > Trace Layer',
      toolKey: draft.logging.traceLayer.tool,
      toolLabel: toolLabel(draft.logging.traceLayer.tool),
      toolVersion: draft.logging.traceLayer.version,
    },
  ] satisfies { slot: PlanningSlot; category: string; toolKey: string; toolLabel: string; toolVersion: string }[]).filter(
    (item) => item.toolKey.length > 0
  )

  const selectedToolKeys = Array.from(new Set(selectedInstallItems.map((item) => item.toolKey)))

  const defaultByTool = useMemo(
    () => new Map((resourceDefaultsData?.items ?? []).map((item) => [item.tool_key, item])),
    [resourceDefaultsData?.items]
  )

  const missingDefaultTools = selectedToolKeys.filter((toolKey) => !defaultByTool.has(toolKey))

  const planningRows = selectedInstallItems.map((item) => {
    const rowKey = `${item.slot}:${item.toolKey}`
    const defs = PLANNING_OPTION_DEFS[item.slot]
    const baseOptions = defs.reduce<Record<string, number>>((acc, def) => {
      acc[def.key] = profileAdjustedBaseline(planningProfile, def)
      return acc
    }, {})

    const optionValues = { ...baseOptions, ...(planningOptionOverrides[rowKey] ?? {}) }
    const baseDefault = defaultByTool.get(item.toolKey)

    if (!baseDefault) {
      return {
        ...item,
        rowKey,
        defs,
        optionValues,
        recommended: null,
        applied: null,
        multipliers: null,
      }
    }

    const multipliers = calculateMultipliers(item.slot, optionValues)
    const recommended = applyMultipliers(baseDefault, multipliers)
    const applied = appliedResourceOverrides[rowKey] ?? recommended
    const units = planningRowUnits[rowKey] ?? { memory: 'Gi', storage: 'Gi' }

    return {
      ...item,
      rowKey,
      defs,
      optionValues,
      recommended,
      applied,
      multipliers,
      units,
    }
  })

  const planningAppliedTotal = planningRows.reduce(
    (acc, row) => {
      if (!row.applied) return acc
      return {
        cpuRequest: acc.cpuRequest + row.applied.cpuRequest,
        cpuLimit: acc.cpuLimit + row.applied.cpuLimit,
        memoryRequestGi: acc.memoryRequestGi + row.applied.memoryRequestGi,
        memoryLimitGi: acc.memoryLimitGi + row.applied.memoryLimitGi,
        storageRequestGi: acc.storageRequestGi + row.applied.storageRequestGi,
        storageLimitGi: acc.storageLimitGi + row.applied.storageLimitGi,
      }
    },
    {
      cpuRequest: 0,
      cpuLimit: 0,
      memoryRequestGi: 0,
      memoryLimitGi: 0,
      storageRequestGi: 0,
      storageLimitGi: 0,
    }
  )

  const manifestTools = (() => {
    const map = new Map<string, ManifestToolEntry>()
    selectedInstallItems.forEach((item) => {
      const bundleId = getManifestBundleId(item.toolKey)
      const existing = map.get(bundleId)
      if (!existing) {
        const appVersion = item.toolVersion || getToolAppVersion(bundleId)
        map.set(bundleId, {
          toolId: bundleId,
          toolLabel: toolLabel(bundleId),
          installType: getInstallType(bundleId),
          toolVersion: appVersion,
          chartVersion: getInstallType(bundleId) === 'helm' ? (getToolChartVersion(bundleId) || appVersion) : undefined,
          hasVersionConflict: false,
          roles: [item.category],
          sourceToolIds: [item.toolKey],
          sourceVersions: [appVersion],
        })
        return
      }

      if (!existing.roles.includes(item.category)) {
        existing.roles.push(item.category)
      }
      if (!existing.sourceToolIds.includes(item.toolKey)) {
        existing.sourceToolIds.push(item.toolKey)
      }
      const appVersion = item.toolVersion || getToolAppVersion(bundleId)
      if (!existing.sourceVersions.includes(appVersion)) {
        existing.sourceVersions.push(appVersion)
      }

      if (existing.toolVersion === getToolAppVersion(bundleId) && item.toolVersion) {
        existing.toolVersion = item.toolVersion
      }
      existing.hasVersionConflict = existing.sourceVersions.filter((v) => v && v.length > 0).length > 1
    })
    return Array.from(map.values())
  })()

  const gatewayManifestTool: ManifestToolEntry = {
    toolId: GATEWAY_MANIFEST_ID,
    toolLabel: 'Gateway',
    installType: 'yaml',
    toolVersion: 'gateway.networking.k8s.io/v1',
    hasVersionConflict: false,
    roles: ['Gateway'],
    sourceToolIds: [GATEWAY_MANIFEST_ID],
    sourceVersions: ['gateway.networking.k8s.io/v1'],
  }

  const allManifestTools = [gatewayManifestTool, ...manifestTools]

  const resourceByTool = (() => {
    const map = new Map<string, ResourceVector>()
    planningRows.forEach((row) => {
      if (!row.applied) return
      const bundleId = getManifestBundleId(row.toolKey)
      const prev = map.get(bundleId)
      if (!prev) {
        map.set(bundleId, { ...row.applied })
        return
      }
      map.set(bundleId, {
        cpuRequest: round2(prev.cpuRequest + row.applied.cpuRequest),
        cpuLimit: round2(prev.cpuLimit + row.applied.cpuLimit),
        memoryRequestGi: round2(prev.memoryRequestGi + row.applied.memoryRequestGi),
        memoryLimitGi: round2(prev.memoryLimitGi + row.applied.memoryLimitGi),
        storageRequestGi: round2(prev.storageRequestGi + row.applied.storageRequestGi),
        storageLimitGi: round2(prev.storageLimitGi + row.applied.storageLimitGi),
      })
    })
    return map
  })()

  const rowKeysByTool = (() => {
    const map = new Map<string, string[]>()
    planningRows.forEach((row) => {
      const bundleId = getManifestBundleId(row.toolKey)
      const list = map.get(bundleId) ?? []
      list.push(row.rowKey)
      map.set(bundleId, list)
    })
    return map
  })()

  const defaultManifestByTool = (() => {
    const map: Record<string, string> = {}
    map[GATEWAY_MANIFEST_ID] = buildGatewayManifest(draft, allManifestTools)
    manifestTools.forEach((tool) => {
      const resources = resourceByTool.get(tool.toolId) ?? {
        cpuRequest: 0,
        cpuLimit: 0,
        memoryRequestGi: 0,
        memoryLimitGi: 0,
        storageRequestGi: 0,
        storageLimitGi: 0,
      }
      map[tool.toolId] = buildToolManifest(tool.toolId, tool.toolLabel, draft, resources, tool.toolVersion, tool.chartVersion)
    })
    return map
  })()

  const resolvedActiveManifestTool =
    activeManifestTool && defaultManifestByTool[activeManifestTool]
      ? activeManifestTool
      : (manifestTools[0]?.toolId ?? GATEWAY_MANIFEST_ID)

  const activeManifestInfo = resolvedActiveManifestTool
    ? allManifestTools.find((tool) => tool.toolId === resolvedActiveManifestTool) ?? null
    : null
  const manifestValidationErrorCount = Object.keys(manifestErrorsByTool).length
  const hasManifestValidationError = manifestValidationErrorCount > 0
  const validManifestToolIds = new Set(allManifestTools.map((tool) => tool.toolId))
  const yamlOverridesPayload = allManifestTools.reduce<Record<string, string>>((acc, tool) => {
    if (tool.toolId === GATEWAY_MANIFEST_ID || tool.installType !== 'yaml') {
      return acc
    }

    const overridden = manifestOverridesByTool[tool.toolId]
    const candidate = overridden && overridden.trim() ? overridden : defaultManifestByTool[tool.toolId]
    if (!candidate || !candidate.trim()) {
      return acc
    }

    acc[tool.toolId] = candidate
    return acc
  }, {})

  Object.entries(manifestOverridesByTool).forEach(([toolId, yamlText]) => {
    const trimmed = yamlText.trim()
    if (!trimmed || !validManifestToolIds.has(toolId)) {
      return
    }
    yamlOverridesPayload[toolId] = yamlText
  })

  const deployScript = createDeployScript(draft, allManifestTools, defaultManifestByTool)

  const dryRunChecks: DryRunCheck[] = (() => {
    const checks: DryRunCheck[] = []

    checks.push({
      id: 'stackName',
      title: 'Stack Name 형식',
      status: draft.stackName.trim().length >= 2 ? 'pass' : 'fail',
      detail:
        draft.stackName.trim().length >= 2
          ? `stack name: ${draft.stackName}`
          : 'Stack Name은 2자 이상이어야 합니다.',
    })

    checks.push({
      id: 'cluster',
      title: 'Target Cluster 선택',
      status: draft.clusterId ? 'pass' : 'fail',
      detail: draft.clusterId ? `cluster: ${draft.clusterId}` : 'Target Cluster를 선택하세요.',
    })

    checks.push({
      id: 'namespace',
      title: 'Namespace 유효성',
      status: effectiveNamespace ? 'pass' : 'fail',
      detail: effectiveNamespace ? `namespace: ${effectiveNamespace}` : 'Namespace가 비어 있습니다.',
    })

    const accessDomain = draft.accessDomain || `${draft.stackName}.internal`
    checks.push({
      id: 'accessDomain',
      title: 'Access domain 규칙',
      status: accessDomain.endsWith('.internal') ? 'pass' : 'warn',
      detail: accessDomain.endsWith('.internal')
        ? `access domain: ${accessDomain}`
        : `권장 규칙(.internal) 미준수: ${accessDomain}`,
    })

    const tlsConfig = draft.accessDomainTls
    const tlsSecretName = tlsConfig.secretName.trim()
    const tlsSecretNamespace = tlsConfig.secretNamespace.trim()
    const tlsIssuerName = tlsConfig.issuerName.trim()
    if (!tlsConfig.enabled) {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'warn',
        detail: '현재 자동 생성 Gateway는 HTTP(80) 기본값입니다. 운영 환경에서는 Access Domain TLS 인증서 적용을 권장합니다.',
      })
    } else if (!tlsSecretName || !tlsSecretNamespace || !tlsIssuerName) {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'fail',
        detail: 'TLS 활성화 시 Secret 이름, Secret 네임스페이스, cert-manager Issuer 이름은 필수입니다.',
      })
    } else if (tlsSecretNamespace !== effectiveNamespace) {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'warn',
        detail: `TLS Secret namespace가 Gateway namespace(${effectiveNamespace})와 다릅니다: ${tlsSecretNamespace}/${tlsSecretName}. 교차 네임스페이스 참조에는 ReferenceGrant가 필요합니다.`,
      })
    } else {
      checks.push({
        id: 'gatewayTls',
        title: 'Gateway HTTPS/TLS 적용 여부',
        status: 'pass',
        detail: `TLS 활성화됨: ${tlsSecretNamespace}/${tlsSecretName} (cert-manager issuer: ${tlsIssuerName})`,
      })
    }

    const manifestCount = allManifestTools.length
    const hasOssManifest = manifestTools.length > 0
    checks.push({
      id: 'manifestCount',
      title: '설치 파일 생성 상태',
      status: hasOssManifest ? 'pass' : 'fail',
      detail:
        hasOssManifest
          ? `생성된 설치 파일: ${manifestCount}개 (Gateway 1 + OSS ${manifestTools.length})`
          : '설치 대상 OSS가 없어 YAML/Deploy Script를 생성할 수 없습니다.',
    })

    const manifestErrors = Object.values(manifestErrorsByTool).filter((e) => e && e.length > 0)
    checks.push({
      id: 'manifestLint',
      title: 'YAML/values 검증',
      status: manifestErrors.length === 0 ? 'pass' : 'fail',
      detail:
        manifestErrors.length === 0
          ? '모든 YAML/values 문법 및 필수 항목 검증 통과'
          : `검증 실패 ${manifestErrors.length}건: ${manifestErrors[0]}`,
    })

    const hasResourceFloorIssue = planningRows.some((row) => {
      if (!row.applied) return false
      return (
        row.applied.cpuRequest <= 0 ||
        row.applied.cpuLimit <= 0 ||
        row.applied.memoryRequestGi <= 0 ||
        row.applied.memoryLimitGi <= 0
      )
    })
    checks.push({
      id: 'resourceBounds',
      title: '리소스 하한 검증',
      status: hasResourceFloorIssue ? 'fail' : 'pass',
      detail: hasResourceFloorIssue
        ? '적용값 중 0 이하 리소스가 있습니다.'
        : `request total CPU ${planningAppliedTotal.cpuRequest.toFixed(2)}, memory ${planningAppliedTotal.memoryRequestGi.toFixed(2)}Gi`,
    })

    const hasVersionConflict = manifestTools.some((tool) => tool.hasVersionConflict)
    checks.push({
      id: 'versionConflict',
      title: '번들 OSS 버전 충돌',
      status: hasVersionConflict ? 'warn' : 'pass',
      detail: hasVersionConflict
        ? '동일 번들 내 OSS 버전이 달라 대표 버전으로 통합됩니다.'
        : '번들 OSS 버전 충돌 없음',
    })

    checks.push({
      id: 'storage',
      title: 'Storage 플랜 검토',
      status: draft.storage.planMode === 'existing-all' ? 'warn' : 'pass',
      detail:
        draft.storage.planMode === 'existing-all'
          ? '기존 스토리지 연결 모드입니다. endpoint/secret 참조를 배포 전 확인하세요.'
          : '통합 생성 모드: 설치 시 DB/Object Storage를 함께 생성',
    })

    return checks
  })()

  const dryRunSummary = (() => {
    const failed = dryRunChecks.filter((c) => c.status === 'fail').length
    const warned = dryRunChecks.filter((c) => c.status === 'warn').length
    const passed = dryRunChecks.filter((c) => c.status === 'pass').length
    return {
      failed,
      warned,
      passed,
      total: dryRunChecks.length,
      readyToDeploy: failed === 0,
    }
  })()

  const runDryRunChecks = () => {
    setDryRunExecutedAt(new Date().toLocaleString())
  }

  const handlePlanningOptionChange = (rowKey: string, optionKey: string, value: number) => {
    setPlanningOptionOverrides((prev) => ({
      ...prev,
      [rowKey]: {
        ...(prev[rowKey] ?? {}),
        [optionKey]: value,
      },
    }))
    setAppliedResourceOverrides((prev) => {
      const next = { ...prev }
      delete next[rowKey]
      return next
    })
  }

  const handlePlanningProfileChange = (profile: PlanningProfile) => {
    setPlanningProfile(profile)
    setPlanningOptionOverrides({})
    setAppliedResourceOverrides({})
  }

  const handleAppliedResourceChange = (rowKey: string, current: ResourceVector, field: keyof ResourceVector, value: number) => {
    setAppliedResourceOverrides((prev) => ({
      ...prev,
      [rowKey]: {
        ...(prev[rowKey] ?? current),
        [field]: value,
      },
    }))
  }

  const handlePlanningUnitChange = (rowKey: string, field: keyof PlanningRowUnit, value: ResourceUnit) => {
    setPlanningRowUnits((prev) => ({
      ...prev,
      [rowKey]: {
        ...(prev[rowKey] ?? { memory: 'Gi', storage: 'Gi' }),
        [field]: value,
      },
    }))
  }

  const handleManifestChange = (toolId: string, value?: string) => {
    const nextYaml = value ?? ''
    setManifestDraftByTool((prev) => ({
      ...prev,
      [toolId]: nextYaml,
    }))

    if (manifestSyncTimerRef.current !== null) {
      window.clearTimeout(manifestSyncTimerRef.current)
    }

    manifestSyncTimerRef.current = window.setTimeout(() => {
      const error = validateManifestAndApply(toolId, nextYaml)
      setManifestErrorsByTool((prev) => {
        if (!error) {
          const next = { ...prev }
          delete next[toolId]
          return next
        }
        return {
          ...prev,
          [toolId]: error,
        }
      })
      if (!error) {
        setManifestOverridesByTool((prev) => ({
          ...prev,
          [toolId]: nextYaml,
        }))
        setManifestDraftByTool((prev) => {
          const next = { ...prev }
          delete next[toolId]
          return next
        })
      }
    }, 350)
  }

  const handleMonacoBeforeMount = useCallback((monaco: Monaco) => {
    if (monacoConfiguredRef.current) return
    configureMonacoYaml(monaco, {
      validate: true,
      completion: false,
      hover: true,
      format: true,
      enableSchemaRequest: false,
      schemas: [],
    })
    monacoConfiguredRef.current = true
  }, [])

  useEffect(() => {
    setValue('stackName', draft.stackName)
  }, [draft.stackName, setValue])

  useEffect(() => {
    if (initializedDefaultStackNameRef.current) return
    initializedDefaultStackNameRef.current = true

    if (useStackConfigStore.getState().draft.stackName.trim().length > 0) return

    const generated = buildDefaultStackName()
    useStackConfigStore.setState((state) => ({
      draft: {
        ...state.draft,
        stackName: generated,
        accessDomain: `${generated}.internal`,
        accessDomainTls: {
          ...state.draft.accessDomainTls,
          secretName: `${generated}-wildcard-tls`,
        },
      },
      isDirty: state.isDirty,
    }))
    setValue('stackName', generated)
  }, [setValue])

  useEffect(() => {
    return () => {
      if (manifestSyncTimerRef.current !== null) {
        window.clearTimeout(manifestSyncTimerRef.current)
      }
    }
  }, [])

  const switchTab = (tab: InstallTab) => {
    if (tab === 'manifests' || tab === 'deploy-script' || tab === 'dry-run') {
      const ok = ensureCoreSelectionsForConfigTabs()
      if (!ok) return
    }
    setTabGuardError(null)
    setLocalTab(tab)
    setActiveTab(tab)
  }

  const handleStoragePlanModeChange = (planMode: StoragePlanMode) => {
    setStorageValidationErrors({})
    if (planMode === 'existing-all') {
      updateStorage({
        planMode,
        database: { ...draft.storage.database, mode: 'existing' },
        objectStorage: { ...draft.storage.objectStorage, mode: 'existing' },
      })
      return
    }

    if (planMode === 'integrated-create') {
      updateStorage({
        planMode,
        database: { ...draft.storage.database, mode: 'create' },
        objectStorage: { ...draft.storage.objectStorage, mode: 'create' },
      })
      return
    }

    updateStorage({
      planMode,
      database: { ...draft.storage.database, mode: 'create' },
      objectStorage: { ...draft.storage.objectStorage, mode: 'create' },
    })
  }

  const getStorageEffectiveMode = (): StorageMode => {
    return draft.storage.planMode === 'existing-all' ? 'existing' : 'create'
  }

  const getStorageFieldError = (target: StorageTargetKey, field: StorageFieldKey): string | undefined => {
    return storageValidationErrors[`${target}.${field}`]
  }

  const clearStorageFieldError = (target: StorageTargetKey, field: StorageFieldKey) => {
    setStorageValidationErrors((prev) => {
      const next = { ...prev }
      delete next[`${target}.${field}`]
      return next
    })
  }

  const validateStorageConfig = (): boolean => {
    const errors: StorageValidationErrors = {}

    const validateTarget = (target: StorageTargetKey) => {
      if (getStorageEffectiveMode() !== 'existing') return

      const config = draft.storage[target]
      const key = (field: StorageFieldKey): StorageValidationErrorKey => `${target}.${field}`

      if (!config.existingRef.trim()) {
        errors[key('existingRef')] = '기존 리소스 참조 ID는 필수입니다.'
      }

      if (!config.endpoint.trim()) {
        errors[key('endpoint')] = '엔드포인트는 필수입니다.'
      } else if (!STORAGE_ENDPOINT_REGEX.test(config.endpoint.trim())) {
        errors[key('endpoint')] = '엔드포인트 형식이 올바르지 않습니다. (예: postgres.shared.svc:5432 또는 http://minio.shared.svc:9000)'
      }

      if (!config.resourceName.trim()) {
        errors[key('resourceName')] = target === 'database' ? 'DB 이름은 필수입니다.' : 'Bucket 이름은 필수입니다.'
      }

      if (!config.accessSecretRef.trim()) {
        errors[key('accessSecretRef')] = '접근 Secret Ref는 필수입니다.'
      } else if (!K8S_SECRET_REF_REGEX.test(config.accessSecretRef.trim())) {
        errors[key('accessSecretRef')] = 'Secret Ref 형식이 올바르지 않습니다. (소문자/숫자/-, DNS-1123)'
      }

      if (!config.authId.trim()) {
        errors[key('authId')] = target === 'database' ? 'DB 사용자 ID는 필수입니다.' : 'Access Key ID는 필수입니다.'
      }

      if (!config.authPasswordKey.trim()) {
        errors[key('authPasswordKey')] = target === 'database' ? 'DB 비밀번호 Key는 필수입니다.' : 'Secret Key Key는 필수입니다.'
      } else if (!SECRET_KEY_REGEX.test(config.authPasswordKey.trim())) {
        errors[key('authPasswordKey')] = '비밀번호 Key 형식이 올바르지 않습니다. (영문/숫자/-, _, .)'
      }
    }

    validateTarget('database')
    validateTarget('objectStorage')

    setStorageValidationErrors(errors)
    return Object.keys(errors).length === 0
  }

  const ensureCoreSelectionsForConfigTabs = (): boolean => {
    if (!draft.stackName.trim()) {
      setTabGuardError('YAML View 탭으로 이동하려면 Stack Name이 필요합니다.')
      setLocalTab('artifacts')
      setActiveTab('artifacts')
      stackNameInputRef.current?.focus()
      return false
    }

    if (!draft.clusterId) {
      setTabGuardError('YAML View 탭으로 이동하려면 Target Cluster 선택이 필요합니다.')
      setLocalTab('artifacts')
      setActiveTab('artifacts')
      clusterSelectRef.current?.focus()
      return false
    }

    if (createNewNs && !draft.namespace.trim()) {
      setTabGuardError('YAML View 탭으로 이동하려면 Namespace 선택 또는 입력이 필요합니다.')
      setLocalTab('artifacts')
      setActiveTab('artifacts')
      newNamespaceInputRef.current?.focus()
      return false
    }

    setTabGuardError(null)
    return true
  }

  function validateManifestAndApply(toolId: string, text: string): string | null {
    if (toolId === GATEWAY_MANIFEST_ID) {
      let docs: ReturnType<typeof YAML.parseAllDocuments>
      try {
        docs = YAML.parseAllDocuments(text)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'YAML 파싱 오류'
        return `YAML 문법 오류: ${message}`
      }

      if (docs.some((docItem) => docItem.errors.length > 0)) {
        return 'Gateway YAML 문서에 파싱 오류가 있습니다.'
      }

      const docObjects = docs.map((docItem) => docItem.toJS() as Record<string, unknown>)
      const gateway = docObjects.find((docItem) => docItem.apiVersion === 'gateway.networking.k8s.io/v1' && docItem.kind === 'Gateway')
      const routes = docObjects.filter((docItem) => docItem.apiVersion === 'gateway.networking.k8s.io/v1' && docItem.kind === 'HTTPRoute')
      const certificates = docObjects.filter((docItem) => docItem.apiVersion === 'cert-manager.io/v1' && docItem.kind === 'Certificate')
      const referenceGrants = docObjects.filter((docItem) => docItem.kind === 'ReferenceGrant')
      if (!gateway || routes.length === 0) {
        return 'Gateway YAML은 gateway.networking.k8s.io/v1 Gateway + HTTPRoute 형식이어야 합니다.'
      }

      const metadata = (gateway.metadata ?? {}) as Record<string, unknown>
      const spec = (gateway.spec ?? {}) as Record<string, unknown>
      const namespace = typeof metadata.namespace === 'string' ? metadata.namespace.trim() : ''
      const listeners = Array.isArray(spec.listeners) ? spec.listeners : []
      if (!namespace || listeners.length === 0) {
        return 'Gateway YAML은 metadata.namespace와 spec.listeners를 포함해야 합니다.'
      }

      const httpsListener = listeners.find((listener) => {
        if (!listener || typeof listener !== 'object') return false
        const listenerObj = listener as Record<string, unknown>
        return listenerObj.protocol === 'HTTPS' && typeof listenerObj.tls === 'object'
      })

      let parsedTlsSecretName = ''
      let parsedTlsSecretNamespace = ''
      let parsedTlsIssuerName = ''
      if (httpsListener && typeof httpsListener === 'object') {
        const listenerObj = httpsListener as Record<string, unknown>
        const tls = (listenerObj.tls ?? {}) as Record<string, unknown>
        const certificateRefs = Array.isArray(tls.certificateRefs) ? tls.certificateRefs : []
        if (certificateRefs.length === 0) {
          return 'HTTPS listener를 사용할 때 tls.certificateRefs는 필수입니다.'
        }
        const certRef = certificateRefs[0]
        if (!certRef || typeof certRef !== 'object') {
          return 'tls.certificateRefs[0] 형식이 올바르지 않습니다.'
        }
        const certRefObj = certRef as Record<string, unknown>
        parsedTlsSecretName = typeof certRefObj.name === 'string' ? certRefObj.name.trim() : ''
        parsedTlsSecretNamespace = typeof certRefObj.namespace === 'string' ? certRefObj.namespace.trim() : namespace

        if (!parsedTlsSecretName || !K8S_SECRET_REF_REGEX.test(parsedTlsSecretName)) {
          return 'TLS Secret 이름은 DNS-1123 형식이어야 합니다.'
        }
        if (!parsedTlsSecretNamespace || !K8S_SECRET_REF_REGEX.test(parsedTlsSecretNamespace)) {
          return 'TLS Secret namespace는 DNS-1123 형식이어야 합니다.'
        }

        const matchingCertificate = certificates.find((certificate) => {
          const certificateMetadata = (certificate.metadata ?? {}) as Record<string, unknown>
          const certificateSpec = (certificate.spec ?? {}) as Record<string, unknown>
          const certNamespace = typeof certificateMetadata.namespace === 'string' ? certificateMetadata.namespace.trim() : namespace
          const certSecretName = typeof certificateSpec.secretName === 'string' ? certificateSpec.secretName.trim() : ''
          return certNamespace === parsedTlsSecretNamespace && certSecretName === parsedTlsSecretName
        })

        if (!matchingCertificate) {
          return 'HTTPS listener를 사용할 때 cert-manager Certificate 문서(secretName 매칭)가 필요합니다.'
        }

        const certificateSpec = (matchingCertificate.spec ?? {}) as Record<string, unknown>
        const issuerRef = (certificateSpec.issuerRef ?? {}) as Record<string, unknown>
        parsedTlsIssuerName = typeof issuerRef.name === 'string' ? issuerRef.name.trim() : ''
        if (!parsedTlsIssuerName || !K8S_SECRET_REF_REGEX.test(parsedTlsIssuerName)) {
          return 'cert-manager issuerRef.name은 DNS-1123 형식이어야 합니다.'
        }

        if (parsedTlsSecretNamespace !== namespace) {
          const hasReferenceGrant = referenceGrants.some((grant) => {
            const grantMetadata = (grant.metadata ?? {}) as Record<string, unknown>
            const grantSpec = (grant.spec ?? {}) as Record<string, unknown>
            const grantNamespace = typeof grantMetadata.namespace === 'string' ? grantMetadata.namespace.trim() : ''
            if (grantNamespace !== parsedTlsSecretNamespace) return false

            const from = Array.isArray(grantSpec.from) ? grantSpec.from : []
            const to = Array.isArray(grantSpec.to) ? grantSpec.to : []

            const hasGatewayFrom = from.some((fromItem) => {
              if (!fromItem || typeof fromItem !== 'object') return false
              const fromObj = fromItem as Record<string, unknown>
              return (
                fromObj.group === 'gateway.networking.k8s.io' &&
                fromObj.kind === 'Gateway' &&
                fromObj.namespace === namespace
              )
            })

            const hasSecretTo = to.some((toItem) => {
              if (!toItem || typeof toItem !== 'object') return false
              const toObj = toItem as Record<string, unknown>
              return toObj.kind === 'Secret' && toObj.name === parsedTlsSecretName
            })

            return hasGatewayFrom && hasSecretTo
          })

          if (!hasReferenceGrant) {
            return 'TLS Secret가 Gateway namespace와 다를 때는 ReferenceGrant가 필요합니다.'
          }
        }
      }

      const firstHost = (() => {
        for (const route of routes) {
          const routeSpec = (route.spec ?? {}) as Record<string, unknown>
          const hostnames = Array.isArray(routeSpec.hostnames) ? routeSpec.hostnames : []
          for (const hostname of hostnames) {
            if (typeof hostname === 'string' && hostname.trim()) {
              return hostname.trim()
            }
          }
        }
        return ''
      })()

      if (!firstHost.includes('.')) {
        return 'Gateway host는 {oss}.{access-domain} 형식이어야 합니다.'
      }

      const derivedAccessDomain = firstHost.split('.').slice(1).join('.').replace(/^\*\./, '')
      if (!derivedAccessDomain.endsWith('.internal')) {
        return 'Gateway host의 access domain은 .internal로 끝나야 합니다.'
      }

      const gatewayName = typeof metadata.name === 'string' ? metadata.name.trim() : ''
      for (const route of routes) {
        const routeSpec = (route.spec ?? {}) as Record<string, unknown>
        const parentRefs = Array.isArray(routeSpec.parentRefs) ? routeSpec.parentRefs : []
        const rules = Array.isArray(routeSpec.rules) ? routeSpec.rules : []
        const parentGatewayMatched = parentRefs.some((parent) => {
          if (!parent || typeof parent !== 'object') return false
          return (parent as Record<string, unknown>).name === gatewayName
        })
        if (!parentGatewayMatched) {
          return 'HTTPRoute의 parentRefs.name은 Gateway metadata.name과 일치해야 합니다.'
        }
        if (rules.length === 0) {
          return 'HTTPRoute는 최소 1개 이상의 rules를 포함해야 합니다.'
        }
      }

      setAccessDomain(derivedAccessDomain)
      const hasHttpsListener = Boolean(httpsListener)
      updateAccessDomainTls({
        enabled: hasHttpsListener,
        secretName: hasHttpsListener ? parsedTlsSecretName : draft.accessDomainTls.secretName,
        secretNamespace: hasHttpsListener ? parsedTlsSecretNamespace : draft.accessDomainTls.secretNamespace,
        issuerName: hasHttpsListener ? parsedTlsIssuerName : draft.accessDomainTls.issuerName,
      })
      if (namespace === 'nullus') {
        setCreateNewNs(false)
        setNamespace('')
      } else {
        setCreateNewNs(false)
        setNamespace(namespace)
      }

      return null
    }

    const installType = getInstallType(toolId)
    const parseGi = (value: string) => {
      const match = value.trim().match(/^([0-9]+(?:\.[0-9]+)?)Gi$/i)
      if (!match) return null
      const parsed = Number(match[1])
      return Number.isFinite(parsed) ? parsed : null
    }

    const toNumber = (value: unknown) => {
      const n = typeof value === 'number' ? value : Number(value)
      return Number.isFinite(n) ? n : null
    }

    let stackName = ''
    let accessDomain = ''
    let clusterId = ''
    let namespace = ''
    let version = getToolAppVersion(toolId)
    let planMode: StoragePlanMode | null = null

    let cpuReq: number | null = null
    let cpuLimit: number | null = null
    let memoryReqGi: number | null = null
    let memoryLimitGi: number | null = null
    let storageReqGi: number | null = null
    let storageLimitGi: number | null = null

    let database: Record<string, unknown> = {}
    let objectStorage: Record<string, unknown> = {}

    if (installType === 'helm') {
      let parsed: unknown
      try {
        parsed = YAML.parse(text)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'YAML 파싱 오류'
        return `YAML 문법 오류: ${message}`
      }

      if (!parsed || typeof parsed !== 'object') {
        return 'Helm values.yaml은 객체 형태여야 합니다.'
      }

      const values = parsed as Record<string, unknown>
      const global = (values.global ?? {}) as Record<string, unknown>
      const chart = (values.chart ?? {}) as Record<string, unknown>
      const image = (values.image ?? {}) as Record<string, unknown>
      const resources = (values.resources ?? {}) as Record<string, unknown>
      const requests = (resources.requests ?? {}) as Record<string, unknown>
      const limits = (resources.limits ?? {}) as Record<string, unknown>
      const storage = (values.storage ?? {}) as Record<string, unknown>
      database = (storage.database ?? {}) as Record<string, unknown>
      objectStorage = (storage.objectStorage ?? {}) as Record<string, unknown>

      stackName = typeof global.stackName === 'string' ? global.stackName.trim() : ''
      accessDomain = typeof global.accessDomain === 'string' ? global.accessDomain.trim() : ''
      clusterId = typeof global.clusterId === 'string' ? global.clusterId.trim() : ''
      namespace = typeof global.namespace === 'string' ? global.namespace.trim() : ''
      version = typeof image.tag === 'string' && image.tag.trim() ? image.tag.trim() : getToolAppVersion(toolId)

      const chartRepoUrl = typeof chart.repoUrl === 'string' ? chart.repoUrl.trim() : ''
      const chartName = typeof chart.name === 'string' ? chart.name.trim() : ''
      const chartVersion = typeof chart.version === 'string' ? chart.version.trim() : ''
      const expectedChart = getHelmMeta(toolId)
      const expectedChartVersion = getToolChartVersion(toolId)
      if (!chartRepoUrl || !chartName || !chartVersion) {
        return 'Helm values는 chart.repoUrl, chart.name, chart.version이 필요합니다.'
      }
      if (chartRepoUrl !== expectedChart.repoUrl || chartName !== expectedChart.chartName) {
        return `선택된 OSS(${toolId})의 Helm Chart와 일치하지 않습니다. 기대값: ${expectedChart.chartName} @ ${expectedChart.repoUrl}`
      }
      if (expectedChartVersion && chartVersion !== expectedChartVersion) {
        return `선택된 OSS(${toolId})의 Helm Chart 버전과 일치하지 않습니다. 기대값: ${expectedChartVersion}`
      }

      cpuReq = toNumber(requests.cpu)
      cpuLimit = toNumber(limits.cpu)
      memoryReqGi = typeof requests.memory === 'string' ? parseGi(requests.memory) : null
      memoryLimitGi = typeof limits.memory === 'string' ? parseGi(limits.memory) : null
      storageReqGi = typeof requests.storage === 'string' ? parseGi(requests.storage) : null
      storageLimitGi = typeof limits.storage === 'string' ? parseGi(limits.storage) : null

      planMode = storage.planMode === 'existing-all' || storage.planMode === 'integrated-create'
        ? storage.planMode
        : null
    } else {
      let docs: ReturnType<typeof YAML.parseAllDocuments>
      try {
        docs = YAML.parseAllDocuments(text)
      } catch (error) {
        const message = error instanceof Error ? error.message : 'YAML 파싱 오류'
        return `YAML 문법 오류: ${message}`
      }

      if (docs.some((doc) => doc.errors.length > 0)) {
        return 'YAML 문서에 파싱 오류가 있습니다. Deployment/Service 문서를 확인해 주세요.'
      }

      const docObjects = docs.map((doc) => doc.toJS() as Record<string, unknown>)
      const deployment = docObjects.find((doc) => doc.kind === 'Deployment' && doc.apiVersion === 'apps/v1')
      const service = docObjects.find((doc) => doc.kind === 'Service' && doc.apiVersion === 'v1')
      if (docObjects.length !== 2 || !deployment || !service) {
        return 'YAML 타입은 apps/v1 Deployment + v1 Service 두 문서가 모두 필요합니다.'
      }

      const metadata = (deployment.metadata ?? {}) as Record<string, unknown>
      const labels = (metadata.labels ?? {}) as Record<string, unknown>
      const spec = (deployment.spec ?? {}) as Record<string, unknown>
      const selector = (spec.selector ?? {}) as Record<string, unknown>
      const matchLabels = (selector.matchLabels ?? {}) as Record<string, unknown>
      const template = (spec.template ?? {}) as Record<string, unknown>
      const templateSpec = (template.spec ?? {}) as Record<string, unknown>
      const templateMeta = (template.metadata ?? {}) as Record<string, unknown>
      const templateLabels = (templateMeta.labels ?? {}) as Record<string, unknown>
      const containers = Array.isArray(templateSpec.containers) ? templateSpec.containers : []
      const firstContainer = (containers[0] ?? {}) as Record<string, unknown>
      const containerResources = (firstContainer.resources ?? {}) as Record<string, unknown>
      const requests = (containerResources.requests ?? {}) as Record<string, unknown>
      const limits = (containerResources.limits ?? {}) as Record<string, unknown>
      const image = typeof firstContainer.image === 'string' ? firstContainer.image : ''

      if (matchLabels.app !== toolId || templateLabels.app !== toolId) {
        return 'Deployment selector/template labels가 toolId와 일치해야 합니다.'
      }
      if (!image.includes(':')) {
      return 'Deployment 컨테이너 image에는 버전 태그가 필요합니다. (예: docker.io/grafana/grafana:11.1.0)'
    }

      const serviceMeta = (service.metadata ?? {}) as Record<string, unknown>
      const serviceSpec = (service.spec ?? {}) as Record<string, unknown>
      const serviceSelector = (serviceSpec.selector ?? {}) as Record<string, unknown>
      const serviceNamespace = typeof serviceMeta.namespace === 'string' ? serviceMeta.namespace.trim() : ''
      if (serviceSelector.app !== toolId || serviceNamespace !== namespace) {
        return 'Service selector(app)와 namespace가 Deployment와 동일해야 합니다.'
      }

      stackName = typeof labels['nullus.io/stack-name'] === 'string' ? String(labels['nullus.io/stack-name']).trim() : ''
      accessDomain = draft.accessDomain || `${stackName}.internal`
      clusterId = typeof labels['nullus.io/cluster-id'] === 'string' ? String(labels['nullus.io/cluster-id']).trim() : ''
      namespace = typeof metadata.namespace === 'string' ? metadata.namespace.trim() : ''
      if (image.includes(':')) {
        version = image.split(':').pop()?.trim() || getToolAppVersion(toolId)
      }

      cpuReq = toNumber(requests.cpu)
      cpuLimit = toNumber(limits.cpu)
      memoryReqGi = typeof requests.memory === 'string' ? parseGi(requests.memory) : null
      memoryLimitGi = typeof limits.memory === 'string' ? parseGi(limits.memory) : null

      const defaultResource = resourceByTool.get(toolId)
      storageReqGi = defaultResource?.storageRequestGi ?? 0
      storageLimitGi = defaultResource?.storageLimitGi ?? 0
      planMode = draft.storage.planMode
    }

    if (!stackName || !clusterId || !namespace) {
      return installType === 'helm'
        ? 'values.global.stackName, values.global.clusterId, values.global.namespace는 필수입니다.'
        : 'Deployment metadata.namespace 및 labels(nullus.io/stack-name, nullus.io/cluster-id)가 필요합니다.'
    }

    if (installType === 'helm') {
      if (!accessDomain) {
        return 'values.global.accessDomain은 필수입니다.'
      }
      if (!accessDomain.endsWith('.internal')) {
        return 'values.global.accessDomain은 .internal 도메인이어야 합니다.'
      }
    }

    if (
      cpuReq === null ||
      cpuLimit === null ||
      memoryReqGi === null ||
      memoryLimitGi === null ||
      storageReqGi === null ||
      storageLimitGi === null
    ) {
      return installType === 'helm'
        ? 'values.resources.requests/limits(cpu/memory/storage)는 모두 필요하며 memory/storage는 Gi 형식이어야 합니다.'
        : 'Deployment 컨테이너 resources.requests/limits(cpu/memory)가 필요하며 memory는 Gi 형식이어야 합니다.'
    }

    if (cpuReq <= 0 || cpuLimit <= 0 || memoryReqGi <= 0 || memoryLimitGi <= 0 || storageReqGi <= 0 || storageLimitGi <= 0) {
      return '리소스 값은 모두 0보다 커야 합니다.'
    }

    if (cpuReq > cpuLimit || memoryReqGi > memoryLimitGi || storageReqGi > storageLimitGi) {
      return '요청값(request)은 제한값(limit)보다 클 수 없습니다.'
    }

    if (installType === 'helm' && planMode === 'existing-all') {
      const databaseEndpoint = typeof database.endpoint === 'string' ? database.endpoint.trim() : ''
      const databaseSecretRef = typeof database.accessSecretRef === 'string' ? database.accessSecretRef.trim() : ''
      const databaseSecretKey = typeof database.authPasswordKey === 'string' ? database.authPasswordKey.trim() : ''
      const objectEndpoint = typeof objectStorage.endpoint === 'string' ? objectStorage.endpoint.trim() : ''
      const objectSecretRef = typeof objectStorage.accessSecretRef === 'string' ? objectStorage.accessSecretRef.trim() : ''
      const objectSecretKey = typeof objectStorage.authPasswordKey === 'string' ? objectStorage.authPasswordKey.trim() : ''

      const requiredExisting = [
        database.existingRef,
        databaseEndpoint,
        database.resourceName,
        databaseSecretRef,
        database.authId,
        databaseSecretKey,
        objectStorage.existingRef,
        objectEndpoint,
        objectStorage.resourceName,
        objectSecretRef,
        objectStorage.authId,
        objectSecretKey,
      ].every((value) => typeof value === 'string' && value.trim().length > 0)

      if (!requiredExisting) {
        return 'storage.planMode가 existing-all이면 DB/Object Storage 연결 및 계정 정보가 모두 필요합니다.'
      }

      if (
        !STORAGE_ENDPOINT_REGEX.test(databaseEndpoint) ||
        !STORAGE_ENDPOINT_REGEX.test(objectEndpoint) ||
        !K8S_SECRET_REF_REGEX.test(databaseSecretRef) ||
        !K8S_SECRET_REF_REGEX.test(objectSecretRef) ||
        !SECRET_KEY_REGEX.test(databaseSecretKey) ||
        !SECRET_KEY_REGEX.test(objectSecretKey)
      ) {
        return 'existing-all 설정의 endpoint/secret 형식이 올바르지 않습니다.'
      }
    }

    const rowKeys = rowKeysByTool.get(toolId) ?? []
    if (rowKeys.length === 0) {
      return '현재 선택된 OSS에서 해당 tool을 찾을 수 없습니다.'
    }

    const installVersion = version || getToolAppVersion(toolId)
    const rowAppliedMap = new Map(
      planningRows
        .filter((row) => row.applied)
        .map((row) => [row.rowKey, row.applied as ResourceVector])
    )

    const currentTotals = rowKeys.reduce(
      (acc, rowKey) => {
        const current = rowAppliedMap.get(rowKey)
        if (!current) return acc
        return {
          cpuRequest: acc.cpuRequest + current.cpuRequest,
          cpuLimit: acc.cpuLimit + current.cpuLimit,
          memoryRequestGi: acc.memoryRequestGi + current.memoryRequestGi,
          memoryLimitGi: acc.memoryLimitGi + current.memoryLimitGi,
          storageRequestGi: acc.storageRequestGi + current.storageRequestGi,
          storageLimitGi: acc.storageLimitGi + current.storageLimitGi,
        }
      },
      {
        cpuRequest: 0,
        cpuLimit: 0,
        memoryRequestGi: 0,
        memoryLimitGi: 0,
        storageRequestGi: 0,
        storageLimitGi: 0,
      }
    )

    const targetTotal: ResourceVector = {
      cpuRequest: cpuReq,
      cpuLimit,
      memoryRequestGi: memoryReqGi,
      memoryLimitGi,
      storageRequestGi: storageReqGi,
      storageLimitGi,
    }

    const fieldRatio = (rowKey: string, field: keyof ResourceVector) => {
      const base = rowAppliedMap.get(rowKey)
      const total = currentTotals[field]
      if (base && total > 0) return base[field] / total
      return 1 / Math.max(rowKeys.length, 1)
    }

    const distributedOverrides = rowKeys.reduce<Record<string, ResourceVector>>((acc, rowKey) => {
      acc[rowKey] = {
        cpuRequest: round2(targetTotal.cpuRequest * fieldRatio(rowKey, 'cpuRequest')),
        cpuLimit: round2(targetTotal.cpuLimit * fieldRatio(rowKey, 'cpuLimit')),
        memoryRequestGi: round2(targetTotal.memoryRequestGi * fieldRatio(rowKey, 'memoryRequestGi')),
        memoryLimitGi: round2(targetTotal.memoryLimitGi * fieldRatio(rowKey, 'memoryLimitGi')),
        storageRequestGi: round2(targetTotal.storageRequestGi * fieldRatio(rowKey, 'storageRequestGi')),
        storageLimitGi: round2(targetTotal.storageLimitGi * fieldRatio(rowKey, 'storageLimitGi')),
      }
      return acc
    }, {})

    rowKeys.forEach((rowKey) => {
      const slot = rowKey.split(':')[0] as PlanningSlot
      const rowToolId = rowKey.split(':')[1]
      const binding = SLOT_TOOL_BINDING[slot]
      if (!binding) return
      setTool(binding.section, binding.field, { tool: rowToolId, version: installVersion })
    })

    setAppliedResourceOverrides((prev) => ({
      ...prev,
      ...distributedOverrides,
    }))

    setStackName(stackName)
    if (installType === 'helm') {
      setAccessDomain(accessDomain)
    }
    setCluster(clusterId)
    if (namespace === 'nullus') {
      setCreateNewNs(false)
      setNamespace('')
    } else {
      setCreateNewNs(false)
      setNamespace(namespace)
    }

    if (installType === 'helm' && planMode) {
      updateStorage({ planMode })
      if (planMode === 'existing-all') {
        updateStorageTarget('database', {
          mode: 'existing',
          existingRef: String(database.existingRef ?? ''),
          endpoint: String(database.endpoint ?? ''),
          resourceName: String(database.resourceName ?? ''),
          accessSecretRef: String(database.accessSecretRef ?? ''),
          authId: String(database.authId ?? ''),
          authPasswordKey: String(database.authPasswordKey ?? ''),
        })
        updateStorageTarget('objectStorage', {
          mode: 'existing',
          existingRef: String(objectStorage.existingRef ?? ''),
          endpoint: String(objectStorage.endpoint ?? ''),
          resourceName: String(objectStorage.resourceName ?? ''),
          accessSecretRef: String(objectStorage.accessSecretRef ?? ''),
          authId: String(objectStorage.authId ?? ''),
          authPasswordKey: String(objectStorage.authPasswordKey ?? ''),
        })
      }
    }

    return null
  }

  const validateCoreFields = async () => {
    const isStackValid = await trigger(['stackName'])
    if (!isStackValid) {
      stackNameInputRef.current?.focus()
      return false
    }

    if (!draft.clusterId) {
      setTabGuardError('Deploy/Save 전 Target Cluster를 선택해 주세요.')
      clusterSelectRef.current?.focus()
      return false
    }

    if (createNewNs && !draft.namespace.trim()) {
      setTabGuardError('Deploy/Save 전 Namespace를 선택하거나 입력해 주세요.')
      newNamespaceInputRef.current?.focus()
      return false
    }

    if (draft.accessDomainTls.enabled) {
      const tlsSecretName = draft.accessDomainTls.secretName.trim()
      const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim()
      const tlsIssuerName = draft.accessDomainTls.issuerName.trim()
      if (!tlsSecretName || !K8S_SECRET_REF_REGEX.test(tlsSecretName)) {
        setTabGuardError('TLS Secret Name은 DNS-1123 형식으로 입력해 주세요. (예: nullus-wildcard-tls)')
        return false
      }
      if (!tlsSecretNamespace || !K8S_SECRET_REF_REGEX.test(tlsSecretNamespace)) {
        setTabGuardError('TLS Secret Namespace는 DNS-1123 형식으로 입력해 주세요. (예: nullus)')
        return false
      }
      if (!tlsIssuerName || !K8S_SECRET_REF_REGEX.test(tlsIssuerName)) {
        setTabGuardError('cert-manager Issuer Name은 DNS-1123 형식으로 입력해 주세요. (예: nullus-ca-issuer)')
        return false
      }
    }

    setTabGuardError(null)
    return true
  }

  const handleDeploy = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return
    const isStorageValid = validateStorageConfig()
    if (!isStorageValid) {
      switchTab('storage')
      return
    }

    const request = {
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      namespace: effectiveNamespace,
      stackName: draft.stackName,
      accessDomain: draft.accessDomain,
      accessDomainTls: draft.accessDomainTls,
      yamlOverrides: yamlOverridesPayload,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
      storage: draft.storage,
    }

    try {
      const createRes = await createStack.mutateAsync(request)
      const stackId = createRes?.id
      if (!stackId) {
        setTabGuardError('스택 생성은 되었지만 stack ID를 확인하지 못했습니다. 다시 시도해 주세요.')
        return
      }
      await deployStack.mutateAsync(stackId)
      navigate(`/stack/deploy/${stackId}`)
    } catch (error) {
      setTabGuardError(toDeployErrorMessage(error))
    }
  }

  const handleSaveDraft = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return
    const isStorageValid = validateStorageConfig()
    if (!isStorageValid) {
      switchTab('storage')
      return
    }

    saveDraft.mutate({
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      namespace: effectiveNamespace,
      stackName: draft.stackName,
      accessDomain: draft.accessDomain,
      accessDomainTls: draft.accessDomainTls,
      yamlOverrides: yamlOverridesPayload,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
      storage: draft.storage,
    })
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Stack List', path: '/stack/list' },
        { label: 'New Stack', path: '/stack/templates' },
        { label: 'Stack Template', path: '/stack/templates' },
        { label: 'Stack Install' },
      ]} />

      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <Download size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Stack Install
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              5단계 워크플로우로 DevSecOps 스택을 구성하세요.
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="md"
            loading={saveDraft.isPending}
            onClick={handleSaveDraft}
            disabled={!isValid || isSubmitting}
            type="button"
          >
            <Save size={14} />
            Save Draft
          </Button>
          <Button variant="ghost" size="md" onClick={() => setK8sPreviewModalOpen(true)} type="button">
            Preview K8s Objects
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={createStack.isPending || deployStack.isPending}
            onClick={handleDeploy}
            disabled={
              isSubmitting ||
              createStack.isPending ||
              deployStack.isPending ||
              !draft.stackName ||
              draft.stackName.length < 2 ||
              !draft.clusterId ||
              (createNewNs && !draft.namespace.trim()) ||
              hasManifestValidationError
            }
            type="button"
          >
            <Rocket size={14} />
            Deploy
          </Button>
        </div>
      </div>
      {hasManifestValidationError && (
        <div className="mb-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]">
          Strict 버전/YAML 검증 실패 {manifestValidationErrorCount}건으로 Deploy가 잠겼습니다. YAML View에서 오류를 해소해 주세요.
        </div>
      )}

      <div className="mb-5 flex flex-wrap items-start gap-4">
        <div className="flex min-w-0 flex-1 flex-wrap items-start gap-4">
          <div className="max-w-[400px] flex-1">
            <Controller
              control={control}
              name="stackName"
              render={({ field }) => (
                <>
                  <Input
                    ref={stackNameInputRef}
                    label="Stack Name"
                    placeholder="예: nullus-devsecops-stack-20260324-193000"
                    value={field.value}
                    onChange={(e) => {
                      field.onChange(e.target.value)
                      setStackName(e.target.value)
                    }}
                    onBlur={field.onBlur}
                  />
                  {errors.stackName && <span className="text-xs text-[#ef4444]">{errors.stackName.message}</span>}
                </>
              )}
            />
            <div className="mt-3">
              <Input
                label="Access domain"
                placeholder="{stack-name}.internal"
                value={draft.accessDomain || `${draft.stackName || 'nullus-stack'}.internal`}
                onChange={(e) => setAccessDomain(e.target.value)}
              />
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                최종 접근 가이드: 각 OSS는 <code>{`{OSS}.${draft.stackName || 'stack-name'}.internal`}</code> 형태로 접근합니다.
              </p>
              <label className="mt-2 inline-flex items-center gap-2 text-xs text-[var(--color-text-secondary)]">
                <input
                  type="checkbox"
                  checked={draft.accessDomainTls.enabled}
                  onChange={(e) => updateAccessDomainTls({ enabled: e.target.checked })}
                />
                Access Domain TLS 인증서 적용 (cert-manager)
              </label>
              {draft.accessDomainTls.enabled && (
                <div className="mt-2 grid gap-2">
                  <Input
                    label="TLS Secret Name"
                    placeholder="nullus-wildcard-tls"
                    value={draft.accessDomainTls.secretName}
                    onChange={(e) => updateAccessDomainTls({ secretName: e.target.value })}
                  />
                  <Input
                    label="TLS Secret Namespace"
                    placeholder="nullus"
                    value={draft.accessDomainTls.secretNamespace}
                    onChange={(e) => updateAccessDomainTls({ secretNamespace: e.target.value })}
                  />
                  <Input
                    label="cert-manager Issuer Name"
                    placeholder="nullus-ca-issuer"
                    value={draft.accessDomainTls.issuerName}
                    onChange={(e) => updateAccessDomainTls({ issuerName: e.target.value })}
                  />
                  <p className="text-[11px] text-[var(--color-text-secondary)]">
                    Preview Deploy Script와 Gateway YAML에 cert-manager <code>Certificate</code> 리소스가 포함되며, Secret은 cert-manager가 관리합니다.
                  </p>
                </div>
              )}
            </div>
          </div>
          <div className="flex max-w-[300px] flex-1 flex-col gap-1">
            <NativeSelect
              ref={clusterSelectRef}
              label="Target Cluster"
              value={draft.clusterId ?? ''}
              onChange={(e) => setCluster(e.target.value)}
              className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
            >
              <option value="">클러스터를 선택하세요</option>
              {(clusters ?? []).map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.connection_status})
                </option>
              ))}
            </NativeSelect>
            {!draft.clusterId && <span className="text-xs text-[#f59e0b]">배포에 필요합니다</span>}
          </div>
          {draft.clusterId && (
            <div className="flex max-w-[300px] flex-1 flex-col gap-1">
              <NativeSelect
                ref={namespaceSelectRef}
                label="Namespace"
                value={createNewNs ? '__new__' : draft.namespace}
                onChange={(e) => {
                  if (e.target.value === '__new__') {
                    setCreateNewNs(true)
                    setNamespace('')
                  } else {
                    setCreateNewNs(false)
                    setNamespace(e.target.value)
                  }
                }}
                className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
              >
                <option value="">기본 (nullus)</option>
                {(namespaces ?? []).map((ns) => (
                  <option key={ns.name} value={ns.name}>{ns.name}</option>
                ))}
                <option value="__new__">새 네임스페이스 생성...</option>
              </NativeSelect>
              {createNewNs && (
                <input
                  ref={newNamespaceInputRef}
                  type="text"
                  placeholder="my-namespace"
                  value={draft.namespace}
                  onChange={(e) => setNamespace(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                />
              )}
              <span className="text-[11px] text-[var(--color-text-secondary)]">배포 대상 네임스페이스</span>
            </div>
          )}
        </div>

        <div className="w-full max-w-[860px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <div className="mb-3 flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-[rgba(99,102,241,0.18)] text-[#a5b4fc]">
              <ShoppingCart size={16} />
            </div>
            <div>
              <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Resource Total</h3>
              <p className="m-0 text-xs text-[var(--color-text-secondary)]">
                선택한 OSS {selectedToolKeys.length}개 적용값(request/limit) 총합
              </p>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="rounded-lg border border-[rgba(99,102,241,0.25)] bg-[rgba(99,102,241,0.08)] p-3">
              <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">Request Total</div>
              <div className="grid grid-cols-3 gap-2 text-sm">
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">CPU</div>
                  <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.cpuRequest.toFixed(2)}</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Memory</div>
                  <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.memoryRequestGi.toFixed(2)}Gi</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Storage</div>
                  <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.storageRequestGi.toFixed(2)}Gi</div>
                </div>
              </div>
            </div>

            <div className="rounded-lg border border-[rgba(34,197,94,0.25)] bg-[rgba(34,197,94,0.08)] p-3">
              <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">Limit Total</div>
              <div className="grid grid-cols-3 gap-2 text-sm">
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">CPU</div>
                  <div className="font-semibold text-[#86efac]">{planningAppliedTotal.cpuLimit.toFixed(2)}</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Memory</div>
                  <div className="font-semibold text-[#86efac]">{planningAppliedTotal.memoryLimitGi.toFixed(2)}Gi</div>
                </div>
                <div>
                  <div className="text-[11px] text-[var(--color-text-secondary)]">Storage</div>
                  <div className="font-semibold text-[#86efac]">{planningAppliedTotal.storageLimitGi.toFixed(2)}Gi</div>
                </div>
              </div>
            </div>
          </div>

          {missingDefaultTools.length > 0 && (
            <div className="mt-3 text-xs text-[#fbbf24]">
              기본값 미정의 OSS: {missingDefaultTools.join(', ')}
            </div>
          )}
        </div>
      </div>

      <div className="flex items-start gap-5">
        {/* Left: tabs + content */}
        <div className="min-w-0 flex-1">
          {/* Tabs */}
          <div className="mb-5 flex gap-0 border-b border-[var(--color-border-default)]">
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id
              return (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => switchTab(tab.id)}
                  className={cn(
                    '-mb-px cursor-pointer border-b-2 border-b-transparent bg-none px-[18px] py-2.5 text-sm transition-all duration-150',
                    isActive
                      ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]'
                      : 'font-normal text-[var(--color-text-secondary)]'
                  )}
                >
                  {tab.label}
                </button>
              )
            })}
          </div>
          {tabGuardError && (
            <div className="mb-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]">
              {tabGuardError}
            </div>
          )}

          {/* Tab content */}
          <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            {activeTab === 'artifacts' && (
              <>
                <ToolSelector
                  label="Package Registry"
                  options={ARTIFACTS_OPTIONS.packageRegistry}
                  value={draft.artifacts.packageRegistry}
                  onChange={(v) => setTool('artifacts', 'packageRegistry', v)}
                />
                <ToolSelector
                  label="Source Repository"
                  options={ARTIFACTS_OPTIONS.sourceRepository}
                  value={draft.artifacts.sourceRepository}
                  onChange={(v) => setTool('artifacts', 'sourceRepository', v)}
                />
                <ToolSelector
                  label="Container Registry"
                  options={ARTIFACTS_OPTIONS.containerRegistry}
                  value={draft.artifacts.containerRegistry}
                  onChange={(v) => setTool('artifacts', 'containerRegistry', v)}
                />
                <ToolSelector
                  label="Storage Backend"
                  options={ARTIFACTS_OPTIONS.storageBackend}
                  value={draft.artifacts.storageBackend}
                  onChange={(v) => setTool('artifacts', 'storageBackend', v)}
                />
              </>
            )}

            {activeTab === 'pipeline' && (
              <>
                <ToolSelector
                  label="CI/CD Platform"
                  options={PIPELINE_OPTIONS.cicdPlatform}
                  value={draft.pipeline.cicdPlatform}
                  onChange={(v) => setTool('pipeline', 'cicdPlatform', v)}
                />
                <ToolSelector
                  label="CD Tool"
                  options={PIPELINE_OPTIONS.cdTool}
                  value={draft.pipeline.cdTool}
                  onChange={(v) => setTool('pipeline', 'cdTool', v)}
                />
              </>
            )}

            {activeTab === 'monitoring' && (
              <>
                <ToolSelector
                  label="Visualization"
                  options={MONITORING_OPTIONS.visualization}
                  value={draft.monitoring.visualization}
                  onChange={(v) => setTool('monitoring', 'visualization', v)}
                />
                <ToolSelector
                  label="Metrics"
                  options={MONITORING_OPTIONS.collection}
                  value={draft.monitoring.collection}
                  onChange={(v) => setTool('monitoring', 'collection', v)}
                />
                <ToolSelector
                  label="Logs"
                  options={LOGGING_OPTIONS.search}
                  value={draft.logging.search}
                  onChange={(v) => setTool('logging', 'search', v)}
                />
                <ToolSelector
                  label="Traces"
                  options={MONITORING_OPTIONS.traceLayer}
                  value={draft.logging.traceLayer}
                  onChange={(v) => setTool('logging', 'traceLayer', v)}
                />
              </>
            )}

            {activeTab === 'manifests' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  선택한 OSS별 설치 파일입니다. Helm은 실제 <code>values.yaml</code>, YAML 타입은 배포 가능한 Kubernetes manifest 형식으로 생성됩니다.
                  문법/필수 항목 검증을 통과하면 이전 탭 설정을 오버라이드합니다.
                </p>

                <div className="mb-3 grid gap-3 md:grid-cols-2">
                  <div>
                    <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">Gateway</div>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => setActiveManifestTool(GATEWAY_MANIFEST_ID)}
                        className={cn(
                          'inline-flex items-center gap-2 rounded-lg border px-3 py-[7px] text-xs',
                          resolvedActiveManifestTool === GATEWAY_MANIFEST_ID
                            ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)] text-[#a5b4fc]'
                            : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-primary)]'
                        )}
                      >
                        <span className="font-semibold">Gateway</span>
                        <span className="rounded border border-[var(--color-border-default)] px-1.5 py-0.5 text-[10px] uppercase text-[var(--color-text-secondary)]">yaml</span>
                      </button>
                    </div>
                  </div>

                  <div>
                    <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">OSS</div>
                    {manifestTools.length === 0 ? (
                      <div className="rounded border border-[rgba(251,191,36,0.35)] bg-[rgba(251,191,36,0.08)] px-3 py-2 text-xs text-[#fcd34d]">
                        설치 대상 OSS가 없습니다. Gateway YAML은 자동 생성되며, OSS 설치파일은 툴 선택 후 생성됩니다.
                      </div>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {manifestTools.map((tool) => {
                          const isActive = resolvedActiveManifestTool === tool.toolId
                          return (
                            <button
                              key={tool.toolId}
                              type="button"
                              onClick={() => setActiveManifestTool(tool.toolId)}
                              className={cn(
                                'inline-flex items-center gap-2 rounded-lg border px-3 py-[7px] text-xs',
                                isActive
                                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)] text-[#a5b4fc]'
                                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-primary)]'
                              )}
                            >
                              <span className="font-semibold">{tool.toolLabel}</span>
                              <span className="rounded border border-[var(--color-border-default)] px-1.5 py-0.5 text-[10px] uppercase text-[var(--color-text-secondary)]">
                                {tool.installType}
                              </span>
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                </div>

                {resolvedActiveManifestTool && (
                  <>
                        {activeManifestInfo && (
                          <div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(99,102,241,0.08)] p-3 text-xs">
                            <div className="mb-1 flex flex-wrap items-center gap-2">
                              <span className="font-semibold text-[#a5b4fc]">{activeManifestInfo.toolLabel}</span>
                              <span className="rounded border border-[var(--color-border-default)] px-1.5 py-0.5 uppercase text-[10px] text-[var(--color-text-secondary)]">
                                {activeManifestInfo.installType}
                              </span>
                              {manifestErrorsByTool[activeManifestInfo.toolId] && (
                                <span className="rounded border border-[rgba(239,68,68,0.4)] bg-[rgba(239,68,68,0.15)] px-1.5 py-0.5 text-[10px] font-semibold text-[#fca5a5]">
                                  STRICT 검증 실패
                                </span>
                              )}
                              <span className="text-[var(--color-text-secondary)]">
                                app version: {activeManifestInfo.toolVersion || getToolAppVersion(activeManifestInfo.toolId)}
                                {activeManifestInfo.installType === 'helm' && activeManifestInfo.chartVersion
                                  ? ` / chart version: ${activeManifestInfo.chartVersion}`
                                  : ''}
                              </span>
                            </div>
                            <div className="text-[var(--color-text-secondary)]">역할: {activeManifestInfo.roles.join(', ')}</div>
                            {activeManifestInfo.toolId !== GATEWAY_MANIFEST_ID && (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                포함 OSS: {activeManifestInfo.sourceToolIds.map((id) => toolLabel(id)).join(', ')}
                              </div>
                            )}
                            {activeManifestInfo.hasVersionConflict && activeManifestInfo.toolId !== GATEWAY_MANIFEST_ID && (
                              <div className="mt-1 text-[#fcd34d]">
                                주의: 포함된 OSS들의 선택 버전이 달라 단일 값으로 통합되었습니다({activeManifestInfo.toolVersion}).
                              </div>
                            )}
                            {activeManifestInfo.toolId === GATEWAY_MANIFEST_ID ? (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                Gateway API YAML은 선택된 OSS 기준으로 Gateway/HTTPRoute를 자동 구성합니다. Access Domain TLS 인증서 적용을 켜면 HTTPS(443) + cert-manager Certificate + tls.certificateRefs(secret)가 함께 생성됩니다.
                              </div>
                            ) : (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                동일 OSS가 여러 역할에 선택돼도 설치 파일은 하나로 통합되어 생성됩니다.
                              </div>
                            )}
                            {activeManifestInfo.toolId !== GATEWAY_MANIFEST_ID && (
                              <div className="mt-1 text-[var(--color-text-secondary)]">
                                버전 정책: <span className="font-semibold">Strict 고정</span> (카탈로그 app/chart 버전과 불일치하면 검증에서 차단됩니다)
                              </div>
                            )}
                          </div>
                        )}

                        <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] p-2">
                          <Editor
                            beforeMount={handleMonacoBeforeMount}
                            height="520px"
                            language="yaml"
                            theme={isDarkMode ? 'vs-dark' : 'vs-light'}
                            value={manifestDraftByTool[resolvedActiveManifestTool] ?? manifestOverridesByTool[resolvedActiveManifestTool] ?? defaultManifestByTool[resolvedActiveManifestTool] ?? ''}
                            onChange={(value) => handleManifestChange(resolvedActiveManifestTool, value)}
                            options={{
                              minimap: { enabled: false },
                              fontSize: 13,
                              lineNumbers: 'on',
                              scrollBeyondLastLine: false,
                              wordWrap: 'on',
                              tabSize: 2,
                            }}
                          />
                        </div>
                        {manifestErrorsByTool[resolvedActiveManifestTool] && (
                          <div className="mt-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs text-[#fca5a5]">
                            {manifestErrorsByTool[resolvedActiveManifestTool]}
                          </div>
                        )}
                  </>
                )}
              </div>
            )}

            {activeTab === 'deploy-script' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  현재 선택된 YAML View(OSS별 설치 파일), 버전, 네임스페이스, 스토리지 설정을 기반으로 생성된 배포 스크립트입니다.
                  Helm 항목은 <code>values.yaml</code> 파일을 EOF로 생성한 뒤 <code>helm upgrade --install -f</code>로 적용합니다.
                </p>
                <CodePreview
                  code={deployScript}
                  language="bash"
                  title={`${draft.stackName || 'nullus-stack'}-deploy.sh`}
                  maxHeight="560px"
                />
              </div>
            )}

            {activeTab === 'dry-run' && (
              <div className="space-y-4">
                <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] p-3">
                  <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Dry Run — 배포 전 최종 검토</h3>
                      <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                        필수 항목, YAML 검증, 리소스/스토리지 상태를 점검하고 배포 준비 여부를 확인합니다.
                      </p>
                    </div>
                    <Button variant="outline" size="sm" type="button" onClick={runDryRunChecks}>
                      Run Dry Run
                    </Button>
                  </div>

                  <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                    <div className="rounded border border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)] px-3 py-2 text-xs">
                      <div className="text-[var(--color-text-secondary)]">PASS</div>
                      <div className="font-semibold text-[#86efac]">{dryRunSummary.passed}</div>
                    </div>
                    <div className="rounded border border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)] px-3 py-2 text-xs">
                      <div className="text-[var(--color-text-secondary)]">WARN</div>
                      <div className="font-semibold text-[#fcd34d]">{dryRunSummary.warned}</div>
                    </div>
                    <div className="rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-xs">
                      <div className="text-[var(--color-text-secondary)]">FAIL</div>
                      <div className="font-semibold text-[#fca5a5]">{dryRunSummary.failed}</div>
                    </div>
                    <div
                      className={cn(
                        'rounded border px-3 py-2 text-xs',
                        dryRunSummary.readyToDeploy
                          ? 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)]'
                          : 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)]'
                      )}
                    >
                      <div className="text-[var(--color-text-secondary)]">READY</div>
                      <div className={cn('font-semibold', dryRunSummary.readyToDeploy ? 'text-[#86efac]' : 'text-[#fca5a5]')}>
                        {dryRunSummary.readyToDeploy ? 'YES' : 'NO'}
                      </div>
                    </div>
                  </div>

                  {dryRunExecutedAt && (
                    <div className="mt-2 text-[11px] text-[var(--color-text-secondary)]">last run: {dryRunExecutedAt}</div>
                  )}
                </div>

                <div className="space-y-2">
                  {dryRunChecks.map((check) => (
                    <div
                      key={check.id}
                      className={cn(
                        'rounded-lg border px-3 py-2',
                        check.status === 'pass' && 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)]',
                        check.status === 'warn' && 'border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)]',
                        check.status === 'fail' && 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)]'
                      )}
                    >
                      <div className="mb-1 flex items-center justify-between gap-2">
                        <span className="text-sm font-semibold text-[var(--color-text-primary)]">{check.title}</span>
                        <span
                          className={cn(
                            'rounded px-2 py-0.5 text-[10px] font-bold uppercase',
                            check.status === 'pass' && 'bg-[rgba(34,197,94,0.2)] text-[#86efac]',
                            check.status === 'warn' && 'bg-[rgba(245,158,11,0.2)] text-[#fcd34d]',
                            check.status === 'fail' && 'bg-[rgba(239,68,68,0.2)] text-[#fca5a5]'
                          )}
                        >
                          {check.status}
                        </span>
                      </div>
                      <div className="text-xs text-[var(--color-text-secondary)]">{check.detail}</div>
                    </div>
                  ))}
                </div>

                <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] p-3">
                  <div className="mb-2">
                    <h4 className="m-0 text-sm font-semibold text-[var(--color-text-primary)]">Final Kubernetes Objects</h4>
                    <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                      현재 옵션으로 최종 생성되는 Kubernetes 오브젝트를 배포 전에 확인합니다.
                    </p>
                  </div>

                  <div className="mb-3 flex flex-wrap gap-2">
                    {([
                      { id: 'namespace', label: 'Namespace' },
                      { id: 'deployment', label: 'Deployment' },
                      { id: 'service', label: 'Service' },
                      { id: 'gateway', label: 'Gateway API' },
                    ] as const).map((tab) => {
                      const isActive = activeK8sPreviewTab === tab.id
                      return (
                        <button
                          key={tab.id}
                          type="button"
                          onClick={() => setActiveK8sPreviewTab(tab.id)}
                          className={cn(
                            'cursor-pointer rounded-lg border px-3 py-[7px] text-[13px]',
                            isActive
                              ? 'border-[#ca8a04] bg-[rgba(202,138,4,0.18)] font-bold text-[#fcd34d]'
                              : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] font-medium text-[var(--color-text-secondary)]'
                          )}
                        >
                          {tab.label}
                        </button>
                      )
                    })}
                  </div>

                  <CodePreview
                    code={k8sObjects[activeK8sPreviewTab]}
                    language="yaml"
                    title={`${activeK8sPreviewTab}.yaml`}
                    maxHeight="420px"
                  />
                </div>
              </div>
            )}

            {activeTab === 'resources' && (
              <div>
                <div className="space-y-4">
                  <div className="flex flex-wrap items-end justify-between gap-3">
                    <div>
                      <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">OSS별 Resource Planning</h3>
                      <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                        각 OSS별 세부 옵션을 변경하면 추천값이 재계산되고 적용값은 추천값으로 재설정됩니다.
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-[11px] text-[var(--color-text-secondary)]">Sizing Profile</span>
                      <NativeSelect
                        value={planningProfile}
                        onChange={(e) => handlePlanningProfileChange(e.target.value as PlanningProfile)}
                        className="min-w-[140px] rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs"
                      >
                        {(['startup', 'standard', 'enterprise'] as PlanningProfile[]).map((profile) => (
                          <option key={profile} value={profile}>
                            {PLANNING_PROFILE_LABEL[profile]}
                          </option>
                        ))}
                      </NativeSelect>
                    </div>
                  </div>

                  <div className="mb-4 grid grid-cols-3 gap-3 rounded-lg border border-[rgba(99,102,241,0.2)] bg-[rgba(99,102,241,0.06)] p-3">
                    <div>
                      <div className="text-[11px] text-[var(--color-text-secondary)]">적용값 총 CPU (Req | Limit)</div>
                      <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.cpuRequest.toFixed(2)} | {planningAppliedTotal.cpuLimit.toFixed(2)}</div>
                    </div>
                    <div>
                      <div className="text-[11px] text-[var(--color-text-secondary)]">적용값 총 Memory (Gi)</div>
                      <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.memoryRequestGi.toFixed(2)} | {planningAppliedTotal.memoryLimitGi.toFixed(2)}</div>
                    </div>
                    <div>
                      <div className="text-[11px] text-[var(--color-text-secondary)]">적용값 총 Storage (Gi)</div>
                      <div className="font-semibold text-[#a5b4fc]">{planningAppliedTotal.storageRequestGi.toFixed(2)} | {planningAppliedTotal.storageLimitGi.toFixed(2)}</div>
                    </div>
                  </div>

                  <div className="flex flex-col gap-3">
                    {planningRows.map((row) => (
                      <div key={row.rowKey} className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-3">
                        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                          <div>
                            <div className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">{row.category}</div>
                            <div className="flex items-center gap-1 text-sm font-bold text-[var(--color-text-primary)]">
                              <span>{row.toolLabel} 리소스 플래닝</span>
                              <button
                                type="button"
                                aria-label={`${row.toolLabel} 리소스 산정식 보기`}
                                onClick={() => setActiveFormulaPopoverKey((prev) => (prev === row.rowKey ? null : row.rowKey))}
                                className="inline-flex h-5 w-5 items-center justify-center rounded text-[var(--color-text-secondary)] hover:bg-[rgba(255,255,255,0.06)] hover:text-[var(--color-text-primary)]"
                              >
                                <Info size={13} />
                              </button>
                            </div>
                          </div>
                        </div>

                        {activeFormulaPopoverKey === row.rowKey && (
                          <div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
                            <div className="mb-2 flex items-center justify-between">
                              <div className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">산정식</div>
                              <button
                                type="button"
                                onClick={() => setActiveFormulaPopoverKey(null)}
                                className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                              >
                                닫기
                              </button>
                            </div>
                            <pre className="m-0 whitespace-pre-wrap break-words text-[11px] leading-5 text-[var(--color-text-secondary)]">
                              {buildFormulaTooltip(row.toolLabel, row.defs)}
                            </pre>
                          </div>
                        )}

                        {!row.recommended || !row.applied ? (
                          <div className="rounded border border-[rgba(251,191,36,0.35)] bg-[rgba(251,191,36,0.08)] px-3 py-2 text-xs text-[#fcd34d]">
                            해당 OSS의 default 리소스가 정의되지 않았습니다.
                          </div>
                        ) : (
                          <>
                            {row.multipliers && (row.multipliers.clamped.cpu || row.multipliers.clamped.memory || row.multipliers.clamped.storage) && (
                              <div className="mb-3 rounded border border-[rgba(251,191,36,0.35)] bg-[rgba(251,191,36,0.08)] px-3 py-2 text-xs text-[#fcd34d]">
                                계산 배수가 제한에 도달했습니다:
                                {row.multipliers.clamped.cpu ? ' CPU' : ''}
                                {row.multipliers.clamped.memory ? ' Memory' : ''}
                                {row.multipliers.clamped.storage ? ' Storage' : ''}
                                {' '} 
                                (0.5x~3.0x 범위). 현재 입력에서는 추가 증가/감소가 추천값에 제한적으로 반영될 수 있습니다.
                              </div>
                            )}

                            <div className="mb-3 grid grid-cols-2 gap-3">
                              <div className="flex items-center gap-2 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
                                <span className="text-[11px] text-[var(--color-text-secondary)]">Memory 단위</span>
                                <NativeSelect
                                  value={row.units.memory}
                                  onChange={(e) => handlePlanningUnitChange(row.rowKey, 'memory', e.target.value as ResourceUnit)}
                                  className="max-w-[90px] rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs"
                                >
                                  <option value="Gi">Gi</option>
                                  <option value="Mi">Mi</option>
                                </NativeSelect>
                              </div>
                              <div className="flex items-center gap-2 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
                                <span className="text-[11px] text-[var(--color-text-secondary)]">Storage 단위</span>
                                <NativeSelect
                                  value={row.units.storage}
                                  onChange={(e) => handlePlanningUnitChange(row.rowKey, 'storage', e.target.value as ResourceUnit)}
                                  className="max-w-[90px] rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-xs"
                                >
                                  <option value="Gi">Gi</option>
                                  <option value="Mi">Mi</option>
                                </NativeSelect>
                              </div>
                            </div>

                            <div className="mb-3 grid grid-cols-2 gap-3">
                              {row.defs.map((def) => (
                                <div key={def.key} className="flex flex-col gap-1">
                                  <label className="text-[11px] text-[var(--color-text-secondary)]">{def.label}</label>
                                  <Input
                                    type="number"
                                    min={def.min}
                                    max={def.max}
                                    step={def.step}
                                    value={row.optionValues[def.key] ?? def.baseline}
                                    onChange={(e) => handlePlanningOptionChange(row.rowKey, def.key, Number(e.target.value))}
                                  />
                                </div>
                              ))}
                            </div>

                            <div className="grid grid-cols-2 gap-3 border-t border-[rgba(255,255,255,0.06)] pt-3">
                              <div className="p-1">
                                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">추천값 (읽기 전용)</div>
                                <div className="grid grid-cols-3 gap-2 text-sm">
                                  <div><div className="text-[11px] text-[var(--color-text-secondary)]">CPU</div><div className="font-semibold text-[#a5b4fc]">{row.recommended.cpuRequest.toFixed(2)} | {row.recommended.cpuLimit.toFixed(2)}</div></div>
                                  <div><div className="text-[11px] text-[var(--color-text-secondary)]">Memory</div><div className="font-semibold text-[#a5b4fc]">{convertGiToUnit(row.recommended.memoryRequestGi, row.units.memory).toFixed(2)} | {convertGiToUnit(row.recommended.memoryLimitGi, row.units.memory).toFixed(2)} {row.units.memory}</div></div>
                                  <div><div className="text-[11px] text-[var(--color-text-secondary)]">Storage</div><div className="font-semibold text-[#a5b4fc]">{convertGiToUnit(row.recommended.storageRequestGi, row.units.storage).toFixed(2)} | {convertGiToUnit(row.recommended.storageLimitGi, row.units.storage).toFixed(2)} {row.units.storage}</div></div>
                                </div>
                              </div>

                              <div className="p-1">
                                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">적용값 (수정 가능)</div>
                                <div className="grid grid-cols-3 gap-2 text-sm">
                                  <div className="flex flex-col gap-1">
                                    <span className="text-[11px] text-[var(--color-text-secondary)]">CPU (Req|Limit)</span>
                                    <div className="flex gap-1">
                                      <Input type="number" step="0.01" value={row.applied.cpuRequest} onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'cpuRequest', Number(e.target.value))} />
                                      <Input type="number" step="0.01" value={row.applied.cpuLimit} onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'cpuLimit', Number(e.target.value))} />
                                    </div>
                                  </div>
                                  <div className="flex flex-col gap-1">
                                    <span className="text-[11px] text-[var(--color-text-secondary)]">Memory (Req|Limit)</span>
                                    <div className="flex gap-1">
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.memoryRequestGi, row.units.memory)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'memoryRequestGi', convertUnitToGi(Number(e.target.value), row.units.memory))}
                                      />
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.memoryLimitGi, row.units.memory)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'memoryLimitGi', convertUnitToGi(Number(e.target.value), row.units.memory))}
                                      />
                                    </div>
                                  </div>
                                  <div className="flex flex-col gap-1">
                                    <span className="text-[11px] text-[var(--color-text-secondary)]">Storage (Req|Limit)</span>
                                    <div className="flex gap-1">
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.storageRequestGi, row.units.storage)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'storageRequestGi', convertUnitToGi(Number(e.target.value), row.units.storage))}
                                      />
                                      <Input
                                        type="number"
                                        step="0.01"
                                        value={convertGiToUnit(row.applied.storageLimitGi, row.units.storage)}
                                        onChange={(e) => handleAppliedResourceChange(row.rowKey, row.applied, 'storageLimitGi', convertUnitToGi(Number(e.target.value), row.units.storage))}
                                      />
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </div>
                          </>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'storage' && (
              <div className="space-y-4">
                <div>
                  <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Storage Plan</h3>
                  <p className="mb-0 mt-1 text-xs text-[var(--color-text-secondary)]">
                    DB(Postgres)와 Object Storage를 기존 연결 또는 통합 생성으로 선택할 수 있습니다.
                  </p>
                </div>

                <div className="grid gap-2">
                  {STORAGE_PLAN_MODE_OPTIONS.map((option) => {
                    const selected = draft.storage.planMode === option.id
                    return (
                      <button
                        key={option.id}
                        type="button"
                        onClick={() => handleStoragePlanModeChange(option.id)}
                        className={cn(
                          'w-full rounded-lg border px-3 py-2 text-left transition-all',
                          selected
                            ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                            : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
                        )}
                      >
                        <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                          {option.label}
                        </div>
                        <div className="mt-0.5 text-xs text-[var(--color-text-secondary)]">{option.description}</div>
                      </button>
                    )
                  })}
                </div>

                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  {([
                    { key: 'database', title: 'Database', target: draft.storage.database },
                    { key: 'objectStorage', title: 'Object Storage', target: draft.storage.objectStorage },
                  ] as const).map((item) => {
                    const targetKey = item.key
                    const effectiveMode: StorageMode = getStorageEffectiveMode()

                    const providerOptions = STORAGE_PROVIDER_OPTIONS[targetKey]
                    const existingRefError = getStorageFieldError(targetKey, 'existingRef')
                    const endpointError = getStorageFieldError(targetKey, 'endpoint')
                    const resourceNameError = getStorageFieldError(targetKey, 'resourceName')
                    const accessSecretRefError = getStorageFieldError(targetKey, 'accessSecretRef')
                    const authIdError = getStorageFieldError(targetKey, 'authId')
                    const authPasswordKeyError = getStorageFieldError(targetKey, 'authPasswordKey')

                    return (
                      <div
                        key={targetKey}
                        className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3"
                      >
                        <div className="mb-2 flex items-center justify-between">
                          <h4 className="m-0 text-sm font-semibold text-[var(--color-text-primary)]">{item.title}</h4>
                          <span className="rounded border border-[var(--color-border-default)] px-2 py-0.5 text-[10px] text-[var(--color-text-secondary)]">
                            {effectiveMode === 'existing' ? '기존 연결' : '신규 생성'}
                          </span>
                        </div>

                        {effectiveMode === 'existing' && (
                          <div className="mb-3 grid gap-2">
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">기존 리소스 참조 ID</label>
                            <Input
                              value={item.target.existingRef}
                              placeholder={targetKey === 'database' ? 'org-shared-postgres' : 'org-shared-object-storage'}
                              onChange={(e) => {
                                clearStorageFieldError(targetKey, 'existingRef')
                                updateStorageTarget(targetKey, { existingRef: e.target.value })
                              }}
                            />
                            {existingRefError && <span className="mt-1 block text-xs text-[#ef4444]">{existingRefError}</span>}
                          </div>
                          <div>
                            <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">엔드포인트</label>
                            <Input
                              value={item.target.endpoint}
                              placeholder={targetKey === 'database' ? 'postgres.shared.svc:5432' : 'http://minio.shared.svc:9000'}
                              onChange={(e) => {
                                clearStorageFieldError(targetKey, 'endpoint')
                                updateStorageTarget(targetKey, { endpoint: e.target.value })
                              }}
                            />
                            {endpointError && <span className="mt-1 block text-xs text-[#ef4444]">{endpointError}</span>}
                          </div>
                          <div>
                            <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 이름' : 'Bucket 이름'}</label>
                            <Input
                              value={item.target.resourceName}
                              placeholder={targetKey === 'database' ? 'nullus' : 'nullus-artifacts'}
                              onChange={(e) => {
                                clearStorageFieldError(targetKey, 'resourceName')
                                updateStorageTarget(targetKey, { resourceName: e.target.value })
                              }}
                            />
                            {resourceNameError && <span className="mt-1 block text-xs text-[#ef4444]">{resourceNameError}</span>}
                          </div>
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">접근 Secret Ref</label>
                              <Input
                                value={item.target.accessSecretRef}
                                placeholder={
                                  targetKey === 'database'
                                    ? 'shared-postgres-credentials'
                                    : 'shared-object-storage-credentials'
                                }
                                onChange={(e) => {
                                  clearStorageFieldError(targetKey, 'accessSecretRef')
                                  updateStorageTarget(targetKey, { accessSecretRef: e.target.value })
                                }}
                              />
                              {accessSecretRefError && <span className="mt-1 block text-xs text-[#ef4444]">{accessSecretRefError}</span>}
                            </div>
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 사용자 ID' : 'Access Key ID'}</label>
                              <Input
                                value={item.target.authId}
                                placeholder={targetKey === 'database' ? 'nullus_app' : 'nullus_access_key'}
                                onChange={(e) => {
                                  clearStorageFieldError(targetKey, 'authId')
                                  updateStorageTarget(targetKey, { authId: e.target.value })
                                }}
                              />
                              {authIdError && <span className="mt-1 block text-xs text-[#ef4444]">{authIdError}</span>}
                            </div>
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 비밀번호 Key' : 'Secret Key Key'}</label>
                              <Input
                                value={item.target.authPasswordKey}
                                placeholder={targetKey === 'database' ? 'password' : 'secretKey'}
                                onChange={(e) => {
                                  clearStorageFieldError(targetKey, 'authPasswordKey')
                                  updateStorageTarget(targetKey, { authPasswordKey: e.target.value })
                                }}
                              />
                              {authPasswordKeyError && <span className="mt-1 block text-xs text-[#ef4444]">{authPasswordKeyError}</span>}
                            </div>
                          </div>
                        )}

                        <div className="mb-3">
                          <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{targetKey === 'database' ? 'DB 엔진' : 'Storage Provider'}</label>
                          <NativeSelect
                            value={item.target.providerOrEngine}
                            onChange={(e) => updateStorageTarget(targetKey, { providerOrEngine: e.target.value })}
                            className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[7px] text-xs"
                          >
                            {providerOptions.map((provider) => (
                              <option key={provider.id} value={provider.id}>
                                {provider.label}
                              </option>
                            ))}
                          </NativeSelect>
                        </div>

                        <div className={cn('grid gap-2', effectiveMode === 'create' ? 'grid-cols-2' : 'grid-cols-1')}>
                          <div>
                            <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">버전</label>
                            <Input
                              value={item.target.version}
                              placeholder={targetKey === 'database' ? '16' : 'latest'}
                              onChange={(e) => updateStorageTarget(targetKey, { version: e.target.value })}
                            />
                          </div>
                          {effectiveMode === 'create' && (
                            <div>
                              <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">사이즈</label>
                              <NativeSelect
                                value={item.target.size}
                                onChange={(e) =>
                                  updateStorageTarget(targetKey, {
                                    size: e.target.value as StorageTargetConfig['size'],
                                  })
                                }
                                className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[7px] text-xs"
                              >
                                {STORAGE_SIZE_OPTIONS.map((size) => (
                                  <option key={size} value={size}>
                                    {`${size} ${STORAGE_SIZE_RESOURCE_HINTS[targetKey][size]}`}
                                  </option>
                                ))}
                              </NativeSelect>
                            </div>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Right: Configuration Summary */}
        <div className="sticky top-6 w-[260px] shrink-0 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <h3 className="mb-[14px] mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-primary)]">
            Configuration Summary
          </h3>
          {[
            ['Template', draft.selectedTemplateId ?? '—'],
            ['Stack Name', draft.stackName || '—'],
            ['Access Domain', draft.accessDomain || `${draft.stackName || 'nullus-stack'}.internal`],
            [
              'Access TLS',
              draft.accessDomainTls.enabled
                ? `enabled (${draft.accessDomainTls.secretNamespace || 'nullus'}/${draft.accessDomainTls.secretName || 'nullus-wildcard-tls'}, issuer=${draft.accessDomainTls.issuerName || 'nullus-ca-issuer'})`
                : 'disabled',
            ],
            ['Package Registry', draft.artifacts.packageRegistry.tool],
            ['Source Repo', draft.artifacts.sourceRepository.tool],
            ['Container Registry', draft.artifacts.containerRegistry.tool],
            ['Storage', draft.artifacts.storageBackend.tool],
            ['CI/CD', draft.pipeline.cicdPlatform.tool],
            ['CD Tool', draft.pipeline.cdTool.tool],
            ['Visualization', draft.monitoring.visualization.tool],
            ['Metrics', draft.monitoring.collection.tool],
            ['Logs', draft.logging.search.tool],
            ['Traces', draft.logging.traceLayer.tool],
            ['Storage Plan', draft.storage.planMode],
            [
              'Database',
              `${draft.storage.database.mode}:${draft.storage.database.providerOrEngine}${draft.storage.database.mode === 'create' ? `/${draft.storage.database.size}` : ''}`,
            ],
            ['DB Ref', `${draft.storage.database.existingRef || '-'} @ ${draft.storage.database.endpoint || '-'}`],
            ['DB Auth', `${draft.storage.database.authId || '-'} (${draft.storage.database.authPasswordKey || '-'})`],
            [
              'Object Storage',
              `${draft.storage.objectStorage.mode}:${draft.storage.objectStorage.providerOrEngine}${draft.storage.objectStorage.mode === 'create' ? `/${draft.storage.objectStorage.size}` : ''}`,
            ],
            ['Object Ref', `${draft.storage.objectStorage.existingRef || '-'} @ ${draft.storage.objectStorage.endpoint || '-'}`],
            ['Object Auth', `${draft.storage.objectStorage.authId || '-'} (${draft.storage.objectStorage.authPasswordKey || '-'})`],
          ].map(([label, val]) => (
            <div
              key={label}
              className="flex items-baseline justify-between gap-2 border-b border-[rgba(255,255,255,0.04)] py-1.5"
            >
              <span className="shrink-0 text-[11px] text-[var(--color-text-secondary)]">{label}</span>
              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-right text-xs font-semibold text-[var(--color-text-primary)]">
                {val}
              </span>
            </div>
          ))}
        </div>
      </div>

      <Modal
        open={k8sPreviewModalOpen}
        onClose={() => setK8sPreviewModalOpen(false)}
        title="K8s Object Preview"
        wide
        footer={
          <Button variant="outline" size="sm" onClick={() => setK8sPreviewModalOpen(false)} type="button">
            Close
          </Button>
        }
      >
        <div className="mb-[14px] flex flex-wrap gap-2">
          {[
            { id: 'namespace', label: 'Namespace' },
            { id: 'deployment', label: 'Deployment' },
            { id: 'service', label: 'Service' },
            { id: 'gateway', label: 'Gateway API' },
          ].map((tab) => {
            const isActive = activeK8sPreviewTab === tab.id
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveK8sPreviewTab(tab.id as K8sPreviewTab)}
                className={cn(
                  'cursor-pointer rounded-lg border px-3 py-[7px] text-[13px]',
                  isActive
                    ? 'border-[#ca8a04] bg-[rgba(202,138,4,0.18)] font-bold text-[#fcd34d]'
                    : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] font-medium text-[var(--color-text-secondary)]'
                )}
              >
                {tab.label}
              </button>
            )
          })}
        </div>
        <CodePreview
          code={k8sObjects[activeK8sPreviewTab]}
          language="yaml"
          title={`${activeK8sPreviewTab}.yaml`}
          maxHeight="500px"
        />
      </Modal>
    </div>
  )
}

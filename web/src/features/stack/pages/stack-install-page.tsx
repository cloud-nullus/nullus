import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { AlignLeft, Check, Copy, Download, Info, Rocket, Save, ShoppingCart } from 'lucide-react'
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
import { useCreateStack, useSaveDraft, useClusters, useResourceDefaults, toCreateStackBody } from '../api/stack-api'
import { api } from '../../../lib/api'
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
  opensearch: { repoUrl: 'https://opensearch-project.github.io/helm-charts', chartName: 'opensearch/opensearch' },
  elasticsearch: { repoUrl: 'https://helm.elastic.co', chartName: 'elastic/elasticsearch' },
  loki: { repoUrl: 'https://grafana.github.io/helm-charts', chartName: 'grafana/loki-stack' },
}

type K8sPreviewTab = 'namespace' | 'deployment' | 'service' | 'ingress'

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

const PLANNING_PROFILE_LABEL: Record<PlanningProfile, string> = {
  startup: 'Startup',
  standard: 'Standard',
  enterprise: 'Enterprise',
}

const STORAGE_ENDPOINT_REGEX = /^((https?:\/\/)[^\s]+|[a-zA-Z0-9.-]+(?::\d{1,5})?)$/
const K8S_SECRET_REF_REGEX = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const SECRET_KEY_REGEX = /^[-._a-zA-Z0-9]+$/

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

function createDeployScript(draft: StackConfigDraft): string {
  const stackName = draft.stackName || 'nullus-stack'

  const installBlock = (title: string, selection: ToolSelection) => {
    const selectedLabel = toolLabel(selection.tool)
    const meta = getHelmMeta(selection.tool)
    return [
      `# ${title} (${selectedLabel})`,
      `helm repo add ${selection.tool} ${meta.repoUrl}`,
      `helm install ${selection.tool} ${meta.chartName} -n nullus-stack --version ${selection.version}`,
      '',
    ]
  }

  return [
    '#!/bin/bash',
    '# Nullus Stack Deploy Script',
    `# Stack: ${stackName}`,
    '',
    'set -euo pipefail',
    '',
    '# 1. Create namespace',
    'kubectl create namespace nullus-stack --dry-run=client -o yaml | kubectl apply -f -',
    '',
    '# 2. Install Artifacts',
    ...installBlock('Package Registry', draft.artifacts.packageRegistry),
    '# 3. Install CI/CD',
    ...installBlock('CI/CD Platform', draft.pipeline.cicdPlatform),
    ...installBlock('CD Tool', draft.pipeline.cdTool),
    '# 4. Install Observability',
    ...installBlock('Visualization', draft.monitoring.visualization),
    ...installBlock('Metrics', draft.monitoring.collection),
    ...installBlock('Logs', draft.logging.search),
    ...installBlock('Traces', draft.logging.traceLayer),
    'echo "Nullus stack deploy script completed."',
  ].join('\n')
}

function createK8sObjects(draft: StackConfigDraft): Record<K8sPreviewTab, string> {
  const appName = draft.stackName || 'nullus-stack'
  const serviceName = `${appName}-svc`
  const host = `${appName}.nullus.local`

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
      `          image: ghcr.io/nullus/${draft.pipeline.cicdPlatform.tool}:latest`,
      '          ports:',
      '            - containerPort: 8080',
      '        - name: metrics-sidecar',
      `          image: ghcr.io/nullus/${draft.monitoring.collection.tool}:latest`,
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
    ingress: [
      'apiVersion: networking.k8s.io/v1',
      'kind: Ingress',
      'metadata:',
      `  name: ${appName}-ingress`,
      '  namespace: nullus-stack',
      '  annotations:',
      '    nginx.ingress.kubernetes.io/rewrite-target: /',
      'spec:',
      '  rules:',
      `    - host: ${host}`,
      '      http:',
      '        paths:',
      '          - path: /',
      '            pathType: Prefix',
      '            backend:',
      '              service:',
      `                name: ${serviceName}`,
      '                port:',
      '                  number: 80',
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

// --- YAML conversion ---

function draftToYaml(draft: StackConfigDraft): string {
  return YAML.stringify(
    {
      stackName: draft.stackName,
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      namespace: draft.namespace,
      artifacts: {
        packageRegistry: draft.artifacts.packageRegistry.tool,
        sourceRepository: draft.artifacts.sourceRepository.tool,
        containerRegistry: draft.artifacts.containerRegistry.tool,
        storageBackend: draft.artifacts.storageBackend.tool,
      },
      pipeline: {
        cicdPlatform: draft.pipeline.cicdPlatform.tool,
        cdTool: draft.pipeline.cdTool.tool,
      },
      monitoring: {
        collection: draft.monitoring.collection.tool,
        visualization: draft.monitoring.visualization.tool,
      },
      logging: {
        logs: draft.logging.search.tool,
        traces: draft.logging.traceLayer.tool,
      },
      resources: {
        developerCount: draft.resources.developerCount,
        concurrentRunners: draft.resources.concurrentRunners,
        commitsPerDay: draft.resources.commitsPerDay,
        buildFrequency: draft.resources.buildFrequency,
        currency: draft.resources.currency,
        mode: draft.resources.mode,
        cpuRequest: draft.resources.cpuRequest,
        memoryRequest: draft.resources.memoryRequest,
        storageRequest: draft.resources.storageRequest,
      },
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
    },
    { indent: 2, lineWidth: 0 }
  )
}

function parseDraftFromYaml(text: string, currentDraft: StackConfigDraft): StackConfigDraft | null {
  const parsed = YAML.parse(text)
  if (!parsed || typeof parsed !== 'object') return null

  const root = parsed as Record<string, unknown>
  const artifacts = (root.artifacts ?? {}) as Record<string, unknown>
  const pipeline = (root.pipeline ?? {}) as Record<string, unknown>
  const monitoring = (root.monitoring ?? {}) as Record<string, unknown>
  const logging = (root.logging ?? {}) as Record<string, unknown>
  const resources = (root.resources ?? {}) as Record<string, unknown>
  const storage = (root.storage ?? {}) as Record<string, unknown>
  const storageDatabase = (storage.database ?? {}) as Record<string, unknown>
  const storageObjectStorage = (storage.objectStorage ?? {}) as Record<string, unknown>

  const toStringOrFallback = (value: unknown, fallback: string) =>
    typeof value === 'string' ? value : fallback

  const toNullableStringOrFallback = (value: unknown, fallback: string | null) => {
    if (value === null) return null
    return typeof value === 'string' ? value : fallback
  }

  const toNumberOrFallback = (value: unknown, fallback: number) => {
    if (typeof value === 'number' && Number.isFinite(value)) return value
    return fallback
  }

  const toStorageModeOrFallback = (value: unknown, fallback: StorageMode): StorageMode =>
    value === 'existing' || value === 'create' ? value : fallback

  const toStoragePlanModeOrFallback = (value: unknown, fallback: StoragePlanMode): StoragePlanMode =>
    value === 'existing-all' || value === 'integrated-create' ? value : fallback

  const toStorageSizeOrFallback = (
    value: unknown,
    fallback: StorageTargetConfig['size']
  ): StorageTargetConfig['size'] =>
    value === 'small' || value === 'medium' || value === 'large' ? value : fallback

  return {
    ...currentDraft,
    stackName: toStringOrFallback(root.stackName, currentDraft.stackName),
    selectedTemplateId: toNullableStringOrFallback(root.templateId, currentDraft.selectedTemplateId),
    clusterId: toNullableStringOrFallback(root.clusterId, currentDraft.clusterId),
    namespace: toStringOrFallback(root.namespace, currentDraft.namespace),
    artifacts: {
      packageRegistry: {
        tool: toStringOrFallback(artifacts.packageRegistry, currentDraft.artifacts.packageRegistry.tool),
        version: currentDraft.artifacts.packageRegistry.version,
      },
      sourceRepository: {
        tool: toStringOrFallback(artifacts.sourceRepository, currentDraft.artifacts.sourceRepository.tool),
        version: currentDraft.artifacts.sourceRepository.version,
      },
      containerRegistry: {
        tool: toStringOrFallback(artifacts.containerRegistry, currentDraft.artifacts.containerRegistry.tool),
        version: currentDraft.artifacts.containerRegistry.version,
      },
      storageBackend: {
        tool: toStringOrFallback(artifacts.storageBackend, currentDraft.artifacts.storageBackend.tool),
        version: currentDraft.artifacts.storageBackend.version,
      },
    },
    pipeline: {
      cicdPlatform: {
        tool: toStringOrFallback(pipeline.cicdPlatform, currentDraft.pipeline.cicdPlatform.tool),
        version: currentDraft.pipeline.cicdPlatform.version,
      },
      cdTool: {
        tool: toStringOrFallback(pipeline.cdTool, currentDraft.pipeline.cdTool.tool),
        version: currentDraft.pipeline.cdTool.version,
      },
    },
    monitoring: {
      collection: {
        tool: toStringOrFallback(monitoring.collection, currentDraft.monitoring.collection.tool),
        version: currentDraft.monitoring.collection.version,
      },
      visualization: {
        tool: toStringOrFallback(monitoring.visualization, currentDraft.monitoring.visualization.tool),
        version: currentDraft.monitoring.visualization.version,
      },
    },
    logging: {
      search: {
        tool: toStringOrFallback(logging.logs, currentDraft.logging.search.tool),
        version: currentDraft.logging.search.version,
      },
      traceLayer: {
        tool: toStringOrFallback(logging.traces, currentDraft.logging.traceLayer.tool),
        version: currentDraft.logging.traceLayer.version,
      },
    },
    resources: {
      ...currentDraft.resources,
      developerCount: toNumberOrFallback(resources.developerCount, currentDraft.resources.developerCount),
      concurrentRunners: toNumberOrFallback(resources.concurrentRunners, currentDraft.resources.concurrentRunners),
      commitsPerDay: toNumberOrFallback(resources.commitsPerDay, currentDraft.resources.commitsPerDay),
      buildFrequency: toStringOrFallback(resources.buildFrequency, currentDraft.resources.buildFrequency) as StackConfigDraft['resources']['buildFrequency'],
      currency: toStringOrFallback(resources.currency, currentDraft.resources.currency) as StackConfigDraft['resources']['currency'],
      mode: toStringOrFallback(resources.mode, currentDraft.resources.mode) as StackConfigDraft['resources']['mode'],
      cpuRequest: toStringOrFallback(resources.cpuRequest, currentDraft.resources.cpuRequest ?? ''),
      memoryRequest: toStringOrFallback(resources.memoryRequest, currentDraft.resources.memoryRequest ?? ''),
      storageRequest: toStringOrFallback(resources.storageRequest, currentDraft.resources.storageRequest ?? ''),
    },
    storage: {
      ...currentDraft.storage,
      planMode: toStoragePlanModeOrFallback(storage.planMode, currentDraft.storage.planMode),
      database: {
        ...currentDraft.storage.database,
        mode: toStorageModeOrFallback(storageDatabase.mode, currentDraft.storage.database.mode),
        existingRef: toStringOrFallback(storageDatabase.existingRef, currentDraft.storage.database.existingRef),
        endpoint: toStringOrFallback(storageDatabase.endpoint, currentDraft.storage.database.endpoint),
        resourceName: toStringOrFallback(storageDatabase.resourceName, currentDraft.storage.database.resourceName),
        accessSecretRef: toStringOrFallback(storageDatabase.accessSecretRef, currentDraft.storage.database.accessSecretRef),
        authId: toStringOrFallback(storageDatabase.authId, currentDraft.storage.database.authId),
        authPasswordKey: toStringOrFallback(storageDatabase.authPasswordKey, currentDraft.storage.database.authPasswordKey),
        providerOrEngine: toStringOrFallback(storageDatabase.providerOrEngine, currentDraft.storage.database.providerOrEngine),
        version: toStringOrFallback(storageDatabase.version, currentDraft.storage.database.version),
        size: toStorageSizeOrFallback(storageDatabase.size, currentDraft.storage.database.size),
      },
      objectStorage: {
        ...currentDraft.storage.objectStorage,
        mode: toStorageModeOrFallback(storageObjectStorage.mode, currentDraft.storage.objectStorage.mode),
        existingRef: toStringOrFallback(storageObjectStorage.existingRef, currentDraft.storage.objectStorage.existingRef),
        endpoint: toStringOrFallback(storageObjectStorage.endpoint, currentDraft.storage.objectStorage.endpoint),
        resourceName: toStringOrFallback(storageObjectStorage.resourceName, currentDraft.storage.objectStorage.resourceName),
        accessSecretRef: toStringOrFallback(storageObjectStorage.accessSecretRef, currentDraft.storage.objectStorage.accessSecretRef),
        authId: toStringOrFallback(storageObjectStorage.authId, currentDraft.storage.objectStorage.authId),
        authPasswordKey: toStringOrFallback(storageObjectStorage.authPasswordKey, currentDraft.storage.objectStorage.authPasswordKey),
        providerOrEngine: toStringOrFallback(storageObjectStorage.providerOrEngine, currentDraft.storage.objectStorage.providerOrEngine),
        version: toStringOrFallback(storageObjectStorage.version, currentDraft.storage.objectStorage.version),
        size: toStorageSizeOrFallback(storageObjectStorage.size, currentDraft.storage.objectStorage.size),
      },
    },
  }
}

// --- Tab definitions ---

const TABS: { id: InstallTab; label: string }[] = [
  { id: 'artifacts', label: 'Artifacts' },
  { id: 'pipeline', label: 'CI/CD' },
  { id: 'monitoring', label: 'Observability' },
  { id: 'resources', label: 'Resources' },
  { id: 'storage', label: 'Storage' },
  { id: 'yaml', label: 'YAML View' },
]

// --- Main page ---

export function StackInstallPage() {
  const navigate = useNavigate()
  const theme = useThemeStore((state) => state.theme)
  const isDarkMode = theme === 'dark'
  const { draft, setActiveTab, setTool, setStackName, setCluster, setNamespace, updateStorage, updateStorageTarget } = useStackConfigStore()
  const createStack = useCreateStack()
  const saveDraft = useSaveDraft()
  const { data: resourceDefaultsData } = useResourceDefaults()
  const { data: clusters } = useClusters()
  const { data: namespaces } = useClusterNamespaces(draft.clusterId ?? '')
  const [createNewNs, setCreateNewNs] = useState(false)
  const [activeTab, setLocalTab] = useState<InstallTab>(draft.activeTab)
  const [deployScriptModalOpen, setDeployScriptModalOpen] = useState(false)
  const [k8sPreviewModalOpen, setK8sPreviewModalOpen] = useState(false)
  const [activeK8sPreviewTab, setActiveK8sPreviewTab] = useState<K8sPreviewTab>('namespace')
  const [planningProfile, setPlanningProfile] = useState<PlanningProfile>('standard')
  const [planningOptionOverrides, setPlanningOptionOverrides] = useState<Record<string, Record<string, number>>>({})
  const [appliedResourceOverrides, setAppliedResourceOverrides] = useState<Record<string, ResourceVector>>({})
  const [planningRowUnits, setPlanningRowUnits] = useState<Record<string, PlanningRowUnit>>({})
  const [activeFormulaPopoverKey, setActiveFormulaPopoverKey] = useState<string | null>(null)
  const [storageValidationErrors, setStorageValidationErrors] = useState<StorageValidationErrors>({})
  const [yamlContent, setYamlContent] = useState(() => draftToYaml(draft))
  const [yamlCopied, setYamlCopied] = useState(false)
  const yamlContentRef = useRef(yamlContent)
  const syncFromYamlTimerRef = useRef<number | null>(null)
  const syncFromDraftTimerRef = useRef<number | null>(null)
  const applyingYamlToStoreRef = useRef(false)
  const monacoConfiguredRef = useRef(false)
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

  const deployScript = createDeployScript(draft)
  const k8sObjects = createK8sObjects(draft)

  const selectedInstallItems = ([
    {
      slot: 'artifacts.packageRegistry',
      category: 'Artifacts > Package Registry',
      toolKey: draft.artifacts.packageRegistry.tool,
      toolLabel: toolLabel(draft.artifacts.packageRegistry.tool),
    },
    {
      slot: 'artifacts.sourceRepository',
      category: 'Artifacts > Source Repository',
      toolKey: draft.artifacts.sourceRepository.tool,
      toolLabel: toolLabel(draft.artifacts.sourceRepository.tool),
    },
    {
      slot: 'artifacts.containerRegistry',
      category: 'Artifacts > Container Registry',
      toolKey: draft.artifacts.containerRegistry.tool,
      toolLabel: toolLabel(draft.artifacts.containerRegistry.tool),
    },
    {
      slot: 'artifacts.storageBackend',
      category: 'Artifacts > Storage Backend',
      toolKey: draft.artifacts.storageBackend.tool,
      toolLabel: toolLabel(draft.artifacts.storageBackend.tool),
    },
    {
      slot: 'pipeline.cicdPlatform',
      category: 'CI/CD > Platform',
      toolKey: draft.pipeline.cicdPlatform.tool,
      toolLabel: toolLabel(draft.pipeline.cicdPlatform.tool),
    },
    {
      slot: 'pipeline.cdTool',
      category: 'CI/CD > CD Tool',
      toolKey: draft.pipeline.cdTool.tool,
      toolLabel: toolLabel(draft.pipeline.cdTool.tool),
    },
    {
      slot: 'monitoring.collection',
      category: 'Observability > Metrics Collection',
      toolKey: draft.monitoring.collection.tool,
      toolLabel: toolLabel(draft.monitoring.collection.tool),
    },
    {
      slot: 'monitoring.visualization',
      category: 'Observability > Visualization',
      toolKey: draft.monitoring.visualization.tool,
      toolLabel: toolLabel(draft.monitoring.visualization.tool),
    },
    {
      slot: 'logging.search',
      category: 'Observability > Logging/Search',
      toolKey: draft.logging.search.tool,
      toolLabel: toolLabel(draft.logging.search.tool),
    },
    {
      slot: 'logging.traceLayer',
      category: 'Observability > Trace Layer',
      toolKey: draft.logging.traceLayer.tool,
      toolLabel: toolLabel(draft.logging.traceLayer.tool),
    },
  ] satisfies { slot: PlanningSlot; category: string; toolKey: string; toolLabel: string }[]).filter(
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

  const handleYamlChange = useCallback((value?: string) => {
    const nextYaml = value ?? ''
    yamlContentRef.current = nextYaml
    setYamlContent(nextYaml)

    if (syncFromDraftTimerRef.current !== null) {
      window.clearTimeout(syncFromDraftTimerRef.current)
      syncFromDraftTimerRef.current = null
    }

    if (syncFromYamlTimerRef.current !== null) {
      window.clearTimeout(syncFromYamlTimerRef.current)
    }

    syncFromYamlTimerRef.current = window.setTimeout(() => {
      try {
        const currentDraft = useStackConfigStore.getState().draft
        const parsedDraft = parseDraftFromYaml(nextYaml, currentDraft)
        if (!parsedDraft) return

        applyingYamlToStoreRef.current = true
        useStackConfigStore.setState((state) => ({
          draft: {
            ...parsedDraft,
            activeTab: state.draft.activeTab,
          },
          isDirty: true,
        }))
      } catch (error) {
        void error
      }
    }, 300)
  }, [])

  const handleCopyYaml = useCallback(() => {
    void navigator.clipboard.writeText(yamlContentRef.current).then(() => {
      setYamlCopied(true)
      window.setTimeout(() => setYamlCopied(false), 1500)
    })
  }, [])

  const handleFormatYaml = useCallback(() => {
    try {
      const parsed = YAML.parse(yamlContentRef.current)
      const formatted = YAML.stringify(parsed, { indent: 2, lineWidth: 0 })
      handleYamlChange(formatted)
    } catch (error) {
      void error
    }
  }, [handleYamlChange])

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
    yamlContentRef.current = yamlContent
  }, [yamlContent])

  useEffect(() => {
    if (applyingYamlToStoreRef.current) {
      applyingYamlToStoreRef.current = false
      return
    }

    const nextYaml = draftToYaml(draft)
    if (nextYaml === yamlContentRef.current) return

    if (syncFromDraftTimerRef.current !== null) {
      window.clearTimeout(syncFromDraftTimerRef.current)
    }

    syncFromDraftTimerRef.current = window.setTimeout(() => {
      yamlContentRef.current = nextYaml
      setYamlContent(nextYaml)
    }, 300)
  }, [draft])

  useEffect(() => {
    setValue('stackName', draft.stackName)
  }, [draft.stackName, setValue])

  useEffect(() => {
    return () => {
      if (syncFromYamlTimerRef.current !== null) {
        window.clearTimeout(syncFromYamlTimerRef.current)
      }
      if (syncFromDraftTimerRef.current !== null) {
        window.clearTimeout(syncFromDraftTimerRef.current)
      }
    }
  }, [])

  const switchTab = (tab: InstallTab) => {
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

  const validateCoreFields = async () => {
    return trigger(['stackName'])
  }

  const handleDeploy = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return
    const isStorageValid = validateStorageConfig()
    if (!isStorageValid) {
      switchTab('storage')
      return
    }

    const body = toCreateStackBody({
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      namespace: draft.namespace,
      stackName: draft.stackName,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
      storage: draft.storage,
    })

    try {
      const createRes = await api.post<{ id: string }>('/stacks', body)
      const stackId = createRes.data?.id
      if (!stackId) { navigate('/stack/list'); return }
      await api.post(`/stacks/${stackId}/deploy`).catch(() => { })
      navigate(`/stack/deploy/${stackId}`)
    } catch {
      navigate('/stack/list')
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
      stackName: draft.stackName,
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
          <Button variant="ghost" size="md" onClick={() => setDeployScriptModalOpen(true)} type="button">
            Preview Deploy Script
          </Button>
          <Button variant="ghost" size="md" onClick={() => setK8sPreviewModalOpen(true)} type="button">
            Preview K8s Objects
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={createStack.isPending}
            onClick={handleDeploy}
            disabled={isSubmitting || !draft.stackName || draft.stackName.length < 2 || !draft.clusterId}
            type="button"
          >
            <Rocket size={14} />
            Deploy
          </Button>
        </div>
      </div>

      <div className="mb-5 flex flex-wrap items-start gap-4">
        <div className="flex min-w-0 flex-1 flex-wrap items-start gap-4">
          <div className="max-w-[400px] flex-1">
            <Controller
              control={control}
              name="stackName"
              render={({ field }) => (
                <>
                  <Input
                    label="Stack Name"
                    placeholder="예: prod-gitlab-stack"
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
          </div>
          <div className="flex max-w-[300px] flex-1 flex-col gap-1">
            <NativeSelect
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

            {activeTab === 'yaml' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  폼과 YAML이 300ms 단위로 동기화됩니다. YAML 문법이 유효할 때만 설정에 반영됩니다.
                </p>
                <div className="mb-2 flex items-center justify-end gap-2">
                  <Button variant="outline" size="sm" type="button" onClick={handleFormatYaml}>
                    <AlignLeft size={12} />
                    Format
                  </Button>
                  <Button variant="outline" size="sm" type="button" onClick={handleCopyYaml}>
                    {yamlCopied ? <Check size={12} /> : <Copy size={12} />}
                    {yamlCopied ? 'Copied!' : 'Copy'}
                  </Button>
                </div>
                <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)]">
                  <Editor
                    beforeMount={handleMonacoBeforeMount}
                    height="500px"
                    language="yaml"
                    theme={isDarkMode ? 'vs-dark' : 'vs-light'}
                    value={yamlContent}
                    onChange={handleYamlChange}
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
                    DB(Postgres)와 Object Storage를 기존 연결/통합 생성/개별 구성 중 하나로 선택할 수 있습니다.
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
        open={deployScriptModalOpen}
        onClose={() => setDeployScriptModalOpen(false)}
        title="Deploy Script Preview"
        wide
        footer={
          <Button variant="outline" size="sm" onClick={() => setDeployScriptModalOpen(false)} type="button">
            Close
          </Button>
        }
      >
        <CodePreview
          code={deployScript}
          language="bash"
          title={`${draft.stackName || 'nullus-stack'}-deploy.sh`}
          maxHeight="520px"
        />
      </Modal>

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
            { id: 'ingress', label: 'Ingress' },
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

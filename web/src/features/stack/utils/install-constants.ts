import type { StoragePlanMode, StorageTargetConfig } from '../stores/stack-config-store'
import type { ManifestInstallType, PlanningSlot } from './install-planning-utils'

export interface ToolOption {
  id: string
  label: string
  description: string
}

export const ARTIFACTS_OPTIONS: Record<string, ToolOption[]> = {
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
}

export const PIPELINE_OPTIONS: Record<string, ToolOption[]> = {
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

export const MONITORING_OPTIONS: Record<string, ToolOption[]> = {
  collection: [
    { id: 'prometheus', label: 'Prometheus', description: '시계열 메트릭 수집' },
    { id: 'thanos', label: 'Thanos', description: '장기 보관 및 글로벌 메트릭 집계' },
    { id: 'victoriametrics', label: 'VictoriaMetrics', description: '고성능 시계열 데이터베이스' },
  ],
  visualization: [
    { id: 'grafana', label: 'Grafana', description: '오픈소스 메트릭 시각화' },
    { id: 'opensearch-dashboards', label: 'OpenSearch Dashboards', description: 'OpenSearch 시각화 대시보드' },
  ],
  traceLayer: [
    { id: 'tempo', label: 'Tempo', description: '분산 추적 백엔드' },
    { id: 'jaeger', label: 'Jaeger', description: '분산 추적 및 트레이스 분석' },
  ],
  traceExporter: [
    { id: 'opentelemetry-collector', label: 'OpenTelemetry Collector', description: 'OTLP 수집/처리 파이프라인' },
  ],
}

export const AUTHENTICATION_OPTIONS: ToolOption[] = [
  {
    id: 'openbao',
    label: 'OpenBao',
    description: 'Use OpenBao as shared token provider for all selected OSS integrations.',
  },
]

export const LOGGING_OPTIONS: Record<string, ToolOption[]> = {
  search: [
    { id: 'opensearch', label: 'OpenSearch', description: 'Elasticsearch 호환 검색/분석' },
    { id: 'loki', label: 'Grafana Loki', description: 'Prometheus 스타일 로그 집계' },
  ],
}

export const STORAGE_PLAN_MODE_OPTIONS: Array<{
  id: StoragePlanMode
  labelKey: string
  labelDefault: string
  descriptionKey: string
  descriptionDefault: string
}> = [
  {
    id: 'none',
    labelKey: 'stackInstall.storagePlan.mode.none.label',
    labelDefault: 'Not selected',
    descriptionKey: 'stackInstall.storagePlan.mode.none.description',
    descriptionDefault: 'Connection mode for DB and Object Storage is not decided yet.',
  },
  {
    id: 'existing-all',
    labelKey: 'stackInstall.storagePlan.mode.existingAll.label',
    labelDefault: 'Connect existing DB/Storage',
    descriptionKey: 'stackInstall.storagePlan.mode.existingAll.description',
    descriptionDefault: 'Reference DB and Object Storage that are already managed by your organization.',
  },
  {
    id: 'integrated-create',
    labelKey: 'stackInstall.storagePlan.mode.integratedCreate.label',
    labelDefault: 'Create and connect integrated DB/Storage',
    descriptionKey: 'stackInstall.storagePlan.mode.integratedCreate.description',
    descriptionDefault: 'Create DB and Object Storage during installation and wire them automatically.',
  },
]

export const STORAGE_SIZE_OPTIONS: Array<StorageTargetConfig['size']> = ['small', 'medium', 'large']

export const STORAGE_SIZE_RESOURCE_HINTS: Record<
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

export const STORAGE_PROVIDER_OPTIONS: Record<'database' | 'objectStorage', Array<{ id: string; label: string }>> = {
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

export const TOOL_OPTIONS_ALL = [
  ...Object.values(ARTIFACTS_OPTIONS).flat(),
  ...Object.values(PIPELINE_OPTIONS).flat(),
  ...Object.values(MONITORING_OPTIONS).flat(),
  ...Object.values(LOGGING_OPTIONS).flat(),
]

export const TOOL_LABEL_MAP = new Map(TOOL_OPTIONS_ALL.map((opt) => [opt.id, opt.label]))

export const MATRIX_CATEGORY_BY_SLOT: Record<PlanningSlot, string | null> = {
  'artifacts.packageRegistry': null,
  'artifacts.sourceRepository': 'source_repository',
  'artifacts.containerRegistry': 'container_registry',
  'artifacts.storageBackend': 'storage_backend',
  'pipeline.cicdPlatform': 'ci_platform',
  'pipeline.cdTool': 'cd_tool',
  'monitoring.collection': 'monitoring_collection',
  'monitoring.visualization': 'monitoring_visualization',
  'logging.search': null,
  'logging.traceLayer': null,
  'logging.traceExporter': null,
}

export const TOOL_ID_TO_MATRIX_NAME: Record<string, string> = {
  gitlab: 'GitLab CE',
  'gitlab-ci': 'GitLab CI',
  argocd: 'Argo CD',
  prometheus: 'Prometheus',
  grafana: 'Grafana',
  minio: 'MinIO',
  'gitlab-registry': 'GitLab Registry',
  github: 'GitHub',
  'github-actions': 'GitHub Actions',
  harbor: 'Harbor',
  'opentelemetry-collector': 'OpenTelemetry Collector',
}

export const TOOL_HELM_META: Record<string, { repoUrl: string; chartName: string }> = {
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

export const TOOL_INSTALL_METHOD: Record<string, ManifestInstallType> = {
  grafana: 'helm',
  prometheus: 'helm',
  tempo: 'helm',
  jaeger: 'helm',
  loki: 'helm',
}

export const TOOL_BUNDLE_CANONICAL: Record<string, string> = {
  'gitlab-registry': 'gitlab',
  'gitlab-ci': 'gitlab',
  'argo-cd': 'argocd',
  'opensearch-dashboards': 'opensearch',
}

export const TOOL_DEFAULT_IMAGE_REPOSITORY: Record<string, string> = {
  prometheus: 'quay.io/prometheus/prometheus',
  grafana: 'docker.io/grafana/grafana',
  loki: 'docker.io/grafana/loki',
  tempo: 'docker.io/grafana/tempo',
  jaeger: 'docker.io/jaegertracing/all-in-one',
  'opentelemetry-collector': 'docker.io/otel/opentelemetry-collector-k8s',
}

export function getManifestBundleId(toolId: string): string {
  return TOOL_BUNDLE_CANONICAL[toolId] ?? toolId
}

export const SLOT_TOOL_BINDING: Record<PlanningSlot, { section: 'artifacts' | 'pipeline' | 'monitoring' | 'logging'; field: string }> = {
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
  'logging.traceExporter': { section: 'logging', field: 'traceExporter' },
}

export const GATEWAY_MANIFEST_ID = 'gateway'

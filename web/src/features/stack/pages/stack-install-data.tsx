import { z } from 'zod'
import YAML from 'yaml'
import { cn } from '../../../lib/utils'
import type { InstallTab, ToolSelection, StackConfigDraft } from '../stores/stack-config-store'

interface ToolOption {
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
  storageBackend: [
    { id: 'minio', label: 'MinIO', description: 'S3 호환 오브젝트 스토리지' },
    { id: 's3', label: 'AWS S3', description: 'Amazon S3 오브젝트 스토리지' },
    { id: 'gcs', label: 'Google Cloud Storage', description: 'GCP 오브젝트 스토리지' },
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
    { id: 'kibana', label: 'Kibana', description: 'Elastic Stack 시각화' },
    { id: 'opensearch-dashboards', label: 'OpenSearch Dashboards', description: 'OpenSearch 시각화 대시보드' },
  ],
  traceLayer: [
    { id: 'tempo', label: 'Tempo', description: '분산 추적 백엔드' },
    { id: 'jaeger', label: 'Jaeger', description: '분산 추적 및 트레이스 분석' },
  ],
}

export const LOGGING_OPTIONS: Record<string, ToolOption[]> = {
  search: [
    { id: 'opensearch', label: 'OpenSearch', description: 'Elasticsearch 호환 검색/분석' },
    { id: 'elasticsearch', label: 'Elasticsearch', description: '분산 검색/분석 엔진' },
    { id: 'loki', label: 'Grafana Loki', description: 'Prometheus 스타일 로그 집계' },
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

export type K8sPreviewTab = 'namespace' | 'deployment' | 'service' | 'ingress'

export const stackInstallSchema = z.object({
  stackName: z
    .string()
    .min(2, 'Stack name must be at least 2 characters')
    .max(50, 'Stack name must be 50 characters or less')
    .regex(/^[a-zA-Z0-9-]+$/, 'Stack name can include only letters, numbers, and hyphens'),
  developerCount: z.number().min(1, 'Developer count must be greater than 0'),
  concurrentRunners: z.number().min(1, 'Concurrent runners must be greater than 0'),
})

export type StackInstallFormData = z.infer<typeof stackInstallSchema>

function toolLabel(toolId: string): string {
  return TOOL_LABEL_MAP.get(toolId) ?? toolId
}

function getHelmMeta(toolId: string) {
  return TOOL_HELM_META[toolId] ?? { repoUrl: 'https://charts.example.com', chartName: `nullus/${toolId}` }
}

export function createDeployScript(draft: StackConfigDraft): string {
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

export function createK8sObjects(draft: StackConfigDraft): Record<K8sPreviewTab, string> {
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

interface ToolSelectorProps {
  label: string
  options: ToolOption[]
  value: ToolSelection
  onChange: (v: ToolSelection) => void
}

export function ToolSelector({ label, options, value, onChange }: ToolSelectorProps) {
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

export function draftToYaml(draft: StackConfigDraft): string {
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
    },
    { indent: 2, lineWidth: 0 }
  )
}

export function parseDraftFromYaml(text: string, currentDraft: StackConfigDraft): StackConfigDraft | null {
  const parsed = YAML.parse(text)
  if (!parsed || typeof parsed !== 'object') return null

  const root = parsed as Record<string, unknown>
  const artifacts = (root.artifacts ?? {}) as Record<string, unknown>
  const pipeline = (root.pipeline ?? {}) as Record<string, unknown>
  const monitoring = (root.monitoring ?? {}) as Record<string, unknown>
  const logging = (root.logging ?? {}) as Record<string, unknown>
  const resources = (root.resources ?? {}) as Record<string, unknown>

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
  }
}

export const TABS: { id: InstallTab; label: string }[] = [
  { id: 'artifacts', label: 'Artifacts' },
  { id: 'pipeline', label: 'CI/CD' },
  { id: 'monitoring', label: 'Observability' },
  { id: 'resources', label: 'Resources' },
  { id: 'yaml', label: 'YAML View' },
]

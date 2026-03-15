import { useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { Download, Save, Rocket } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useStackConfigStore } from '../stores/stack-config-store'
import type { InstallTab, ToolSelection, StackConfigDraft } from '../stores/stack-config-store'
import { useCreateStack, useSaveDraft, useEstimateResources } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { YamlEditor } from '../../../components/shared/yaml-editor'
import { Modal } from '../../../components/ui/modal'
import { CodePreview } from '../../../components/shared/code-preview'
import { cn } from '../../../lib/utils'

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
    { id: 'datadog', label: 'Datadog', description: '클라우드 모니터링 플랫폼' },
    { id: 'newrelic', label: 'New Relic', description: 'APM 및 모니터링 플랫폼' },
  ],
  visualization: [
    { id: 'grafana', label: 'Grafana', description: '오픈소스 메트릭 시각화' },
    { id: 'kibana', label: 'Kibana', description: 'Elastic Stack 시각화' },
    { id: 'datadog-dashboards', label: 'Datadog Dashboards', description: 'Datadog 내장 대시보드' },
  ],
}

const LOGGING_OPTIONS: Record<string, ToolOption[]> = {
  collection: [
    { id: 'opentelemetry', label: 'OpenTelemetry', description: '벤더 중립 텔레메트리 수집' },
    { id: 'fluentbit', label: 'Fluent Bit', description: '경량 로그 수집기' },
    { id: 'logstash', label: 'Logstash', description: 'ELK Stack 로그 파이프라인' },
  ],
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
  datadog: { repoUrl: 'https://helm.datadoghq.com', chartName: 'datadog/datadog' },
  newrelic: { repoUrl: 'https://helm-charts.newrelic.com', chartName: 'newrelic/nri-bundle' },
  grafana: { repoUrl: 'https://grafana.github.io/helm-charts', chartName: 'grafana/grafana' },
  kibana: { repoUrl: 'https://helm.elastic.co', chartName: 'elastic/kibana' },
  'datadog-dashboards': { repoUrl: 'https://helm.datadoghq.com', chartName: 'datadog/datadog' },
  opentelemetry: { repoUrl: 'https://open-telemetry.github.io/opentelemetry-helm-charts', chartName: 'open-telemetry/opentelemetry-collector' },
  fluentbit: { repoUrl: 'https://fluent.github.io/helm-charts', chartName: 'fluent/fluent-bit' },
  logstash: { repoUrl: 'https://helm.elastic.co', chartName: 'elastic/logstash' },
  opensearch: { repoUrl: 'https://opensearch-project.github.io/helm-charts', chartName: 'opensearch/opensearch' },
  elasticsearch: { repoUrl: 'https://helm.elastic.co', chartName: 'elastic/elasticsearch' },
  loki: { repoUrl: 'https://grafana.github.io/helm-charts', chartName: 'grafana/loki-stack' },
}

type K8sPreviewTab = 'namespace' | 'deployment' | 'service' | 'ingress'

const stackInstallSchema = z.object({
  stackName: z
    .string()
    .min(2, 'Stack name must be at least 2 characters')
    .max(50, 'Stack name must be 50 characters or less')
    .regex(/^[a-zA-Z0-9-]+$/, 'Stack name can include only letters, numbers, and hyphens'),
  developerCount: z.number().min(1, 'Developer count must be greater than 0'),
  concurrentRunners: z.number().min(1, 'Concurrent runners must be greater than 0'),
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
    '# 3. Install Pipeline',
    ...installBlock('CI/CD Platform', draft.pipeline.cicdPlatform),
    ...installBlock('CD Tool', draft.pipeline.cdTool),
    '# 4. Install Monitoring',
    ...installBlock('Metrics Collection', draft.monitoring.collection),
    ...installBlock('Visualization', draft.monitoring.visualization),
    '# 5. Install Logging',
    ...installBlock('Log Collection', draft.logging.collection),
    ...installBlock('Log Search', draft.logging.search),
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
  const lines: string[] = [
    `stackName: ${draft.stackName || '""'}`,
    `templateId: ${draft.selectedTemplateId ?? 'null'}`,
    `clusterId: ${draft.clusterId ?? 'null'}`,
    '',
    'artifacts:',
    `  packageRegistry: ${draft.artifacts.packageRegistry.tool}`,
    `  sourceRepository: ${draft.artifacts.sourceRepository.tool}`,
    `  containerRegistry: ${draft.artifacts.containerRegistry.tool}`,
    `  storageBackend: ${draft.artifacts.storageBackend.tool}`,
    '',
    'pipeline:',
    `  cicdPlatform: ${draft.pipeline.cicdPlatform.tool}`,
    `  cdTool: ${draft.pipeline.cdTool.tool}`,
    '',
    'monitoring:',
    `  collection: ${draft.monitoring.collection.tool}`,
    `  visualization: ${draft.monitoring.visualization.tool}`,
    '',
    'logging:',
    `  collection: ${draft.logging.collection.tool}`,
    `  search: ${draft.logging.search.tool}`,
    '',
    'resources:',
    `  developerCount: ${draft.resources.developerCount}`,
    `  concurrentRunners: ${draft.resources.concurrentRunners}`,
    `  commitsPerDay: ${draft.resources.commitsPerDay}`,
    `  buildFrequency: ${draft.resources.buildFrequency}`,
    `  currency: ${draft.resources.currency}`,
  ]
  return lines.join('\n')
}

// --- Tab definitions ---

const TABS: { id: InstallTab; label: string }[] = [
  { id: 'artifacts', label: 'Artifacts' },
  { id: 'pipeline', label: 'Pipeline' },
  { id: 'monitoring', label: 'Monitoring' },
  { id: 'logging', label: 'Logging' },
  { id: 'resources', label: 'Resources' },
  { id: 'yaml', label: 'YAML View' },
]

// --- Main page ---

export function StackInstallPage() {
  const navigate = useNavigate()
  const { draft, setActiveTab, setTool, setStackName, updateResources } = useStackConfigStore()
  const createStack = useCreateStack()
  const saveDraft = useSaveDraft()
  const estimateResources = useEstimateResources()
  const [activeTab, setLocalTab] = useState<InstallTab>(draft.activeTab)
  const [deployScriptModalOpen, setDeployScriptModalOpen] = useState(false)
  const [k8sPreviewModalOpen, setK8sPreviewModalOpen] = useState(false)
  const [activeK8sPreviewTab, setActiveK8sPreviewTab] = useState<K8sPreviewTab>('namespace')
  const {
    control,
    trigger,
    formState: { errors, isValid, isSubmitting },
  } = useForm<StackInstallFormData>({
    resolver: zodResolver(stackInstallSchema),
    defaultValues: {
      stackName: draft.stackName,
      developerCount: draft.resources.developerCount,
      concurrentRunners: draft.resources.concurrentRunners,
    },
    mode: 'onChange',
  })

  const deployScript = createDeployScript(draft)
  const k8sObjects = createK8sObjects(draft)

  const switchTab = (tab: InstallTab) => {
    setLocalTab(tab)
    setActiveTab(tab)
  }

  const validateCoreFields = async () => {
    return trigger(['stackName', 'developerCount', 'concurrentRunners'])
  }

  const handleDeploy = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return

    createStack.mutate(
      {
        templateId: draft.selectedTemplateId,
        clusterId: draft.clusterId,
        stackName: draft.stackName,
        artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
        pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
        monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
        logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
        resources: draft.resources,
      },
      {
        onSuccess: () => navigate('/stack/list'),
      }
    )
  }

  const handleSaveDraft = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return

    saveDraft.mutate({
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      stackName: draft.stackName,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
    })
  }

  const handleEstimate = () => {
    estimateResources.mutate(draft.resources)
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
            disabled={!isValid || isSubmitting}
            type="button"
          >
            <Rocket size={14} />
            Deploy
          </Button>
        </div>
      </div>

      {/* Stack name */}
      <div className="mb-5 max-w-[400px]">
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
                  label="Metrics Collection"
                  options={MONITORING_OPTIONS.collection}
                  value={draft.monitoring.collection}
                  onChange={(v) => setTool('monitoring', 'collection', v)}
                />
                <ToolSelector
                  label="Visualization"
                  options={MONITORING_OPTIONS.visualization}
                  value={draft.monitoring.visualization}
                  onChange={(v) => setTool('monitoring', 'visualization', v)}
                />
              </>
            )}

            {activeTab === 'logging' && (
              <>
                <ToolSelector
                  label="Log Collection"
                  options={LOGGING_OPTIONS.collection}
                  value={draft.logging.collection}
                  onChange={(v) => setTool('logging', 'collection', v)}
                />
                <ToolSelector
                  label="Log Search"
                  options={LOGGING_OPTIONS.search}
                  value={draft.logging.search}
                  onChange={(v) => setTool('logging', 'search', v)}
                />
              </>
            )}

            {activeTab === 'yaml' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  현재 스택 설정의 YAML 표현입니다. (읽기 전용)
                </p>
                <YamlEditor
                  value={draftToYaml(draft)}
                  readOnly
                  height="360px"
                />
              </div>
            )}

            {activeTab === 'resources' && (
              <div>
                <p className="mb-4 mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  팀 규모와 사용 패턴을 입력하면 필요한 리소스를 계산합니다.
                </p>
                <div className="mb-4 grid grid-cols-2 gap-[14px]">
                  <Controller
                    control={control}
                    name="developerCount"
                    render={({ field }) => (
                      <>
                        <Input
                          label="개발자 수"
                          type="number"
                          min={1}
                          value={field.value}
                          onChange={(e) => {
                            const value = Number(e.target.value)
                            field.onChange(value)
                            updateResources({ developerCount: value })
                          }}
                          onBlur={field.onBlur}
                        />
                        {errors.developerCount && <span className="text-xs text-[#ef4444]">{errors.developerCount.message}</span>}
                      </>
                    )}
                  />
                  <Controller
                    control={control}
                    name="concurrentRunners"
                    render={({ field }) => (
                      <>
                        <Input
                          label="동시 러너 수"
                          type="number"
                          min={1}
                          value={field.value}
                          onChange={(e) => {
                            const value = Number(e.target.value)
                            field.onChange(value)
                            updateResources({ concurrentRunners: value })
                          }}
                          onBlur={field.onBlur}
                        />
                        {errors.concurrentRunners && <span className="text-xs text-[#ef4444]">{errors.concurrentRunners.message}</span>}
                      </>
                    )}
                  />
                  <Input
                    label="일일 커밋 수"
                    type="number"
                    min={1}
                    value={draft.resources.commitsPerDay}
                    onChange={(e) => updateResources({ commitsPerDay: Number(e.target.value) })}
                  />
                  <div className="flex flex-col gap-1">
                    <label htmlFor="build-frequency" className="text-xs font-medium text-[var(--color-text-secondary)]">
                      빌드 빈도
                    </label>
                    <select
                      id="build-frequency"
                      value={draft.resources.buildFrequency}
                      onChange={(e) =>
                        updateResources({ buildFrequency: e.target.value as 'low' | 'medium' | 'high' })
                      }
                      className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                    >
                      <option value="low">낮음 (Low)</option>
                      <option value="medium">보통 (Medium)</option>
                      <option value="high">높음 (High)</option>
                    </select>
                  </div>
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  loading={estimateResources.isPending}
                  onClick={handleEstimate}
                  type="button"
                  className="mb-4"
                >
                  리소스 계산
                </Button>
                {estimateResources.data && (
                  <div className="grid grid-cols-4 gap-3 rounded-lg border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.08)] p-[14px]">
                    {[
                      ['CPU', estimateResources.data.cpu],
                      ['Memory', estimateResources.data.memory],
                      ['Storage', estimateResources.data.storage],
                      ['월 비용', `${estimateResources.data.estimatedCostMonthly.toLocaleString()} ${estimateResources.data.currency}`],
                    ].map(([label, val]) => (
                      <div key={label}>
                        <div className="mb-1 text-[11px] text-[var(--color-text-secondary)]">{label}</div>
                        <div className="text-[15px] font-bold text-[#a5b4fc]">{val}</div>
                      </div>
                    ))}
                  </div>
                )}
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
            ['CI/CD Platform', draft.pipeline.cicdPlatform.tool],
            ['CD Tool', draft.pipeline.cdTool.tool],
            ['Metrics', draft.monitoring.collection.tool],
            ['Visualization', draft.monitoring.visualization.tool],
            ['Log Collection', draft.logging.collection.tool],
            ['Log Search', draft.logging.search.tool],
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

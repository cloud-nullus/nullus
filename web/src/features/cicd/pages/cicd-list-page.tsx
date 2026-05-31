import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Activity,
  BarChart2,
  Boxes,
  Box,
  CheckCircle2,
  CircleDashed,
  ExternalLink,
  Eye,
  EyeOff,
  FileCode2,
  GitBranch,
  Globe,
  History,
  Info,
  List,
  Package,
  Plus,
  RefreshCw,
  Rocket,
  Server,
  XCircle,
  Search,
  Terminal,
  Trash2,
  Loader2,
  X,
} from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { useDeletePipeline, useDeploymentStatus, useDeployPipeline, usePipelineDeployments, usePipelineResources, usePipelines, useTemplateById } from '../api/cicd-api'
import type { Pipeline } from '../api/cicd-api'
import { useScopedClusters as useClusters } from '../../admin/api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { YamlEditor } from '../../../components/shared/yaml-editor'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { formatDate, formatDateTime, resolveLocale } from '../../../lib/locale'
import { getPipelineStatusLabel, getPipelineStatusStyle } from '../utils/pipeline-status'
import { cn } from '../../../lib/utils'


// ── Execute Modal ─────────────────────────────────────────────────────────────

type ExecuteSetupTab = 'cluster' | 'build' | 'deploy'
type ExecuteDeployMode = 'template' | 'custom'

const EXECUTE_DOCKERFILE_PRESETS = [
  {
    id: 'dockerfile.root',
    label: 'Dockerfile (root)',
    path: './Dockerfile',
    content: [
      'FROM node:20-alpine AS builder',
      'WORKDIR /app',
      'COPY package*.json ./',
      'RUN npm ci',
      'COPY . .',
      'RUN npm run build',
      '',
      'FROM nginx:1.27-alpine',
      'COPY --from=builder /app/dist /usr/share/nginx/html',
      'EXPOSE 80',
      'CMD ["nginx", "-g", "daemon off;"]',
    ].join('\n'),
  },
  {
    id: 'dockerfile.app',
    label: 'Dockerfile (app/)',
    path: './app/Dockerfile',
    content: [
      'FROM golang:1.24-alpine AS builder',
      'WORKDIR /src',
      'COPY go.mod go.sum ./',
      'RUN go mod download',
      'COPY . .',
      'RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/server',
      '',
      'FROM gcr.io/distroless/static:nonroot',
      'COPY --from=builder /src/app /app',
      'USER nonroot:nonroot',
      'ENTRYPOINT ["/app"]',
    ].join('\n'),
  },
  {
    id: 'dockerfile.service',
    label: 'Dockerfile (services/api/)',
    path: './services/api/Dockerfile',
    content: [
      'FROM python:3.12-slim',
      'WORKDIR /service',
      'COPY requirements.txt .',
      'RUN pip install --no-cache-dir -r requirements.txt',
      'COPY . .',
      'EXPOSE 8080',
      'CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8080"]',
    ].join('\n'),
  },
]

const EXECUTE_DEPLOY_YAML_PRESETS = [
  {
    id: 'k8s-deployment',
    label: 'Kubernetes Deployment',
    description: 'Deployment + Service 기본 매니페스트',
    content: [
      'apiVersion: apps/v1',
      'kind: Deployment',
      'metadata:',
      '  name: app-placeholder',
      'spec:',
      '  replicas: 2',
      '  selector:',
      '    matchLabels:',
      '      app: app-placeholder',
      '  template:',
      '    metadata:',
      '      labels:',
      '        app: app-placeholder',
      '    spec:',
      '      containers:',
      '        - name: app',
      '          image: harbor.local/app-placeholder:latest',
      '          ports:',
      '            - containerPort: 8080',
      '---',
      'apiVersion: v1',
      'kind: Service',
      'metadata:',
      '  name: app-placeholder-svc',
      'spec:',
      '  selector:',
      '    app: app-placeholder',
      '  ports:',
      '    - port: 80',
      '      targetPort: 8080',
      '  type: ClusterIP',
    ].join('\n'),
  },
  {
    id: 'k8s-cronjob',
    label: 'Kubernetes CronJob',
    description: '배치/스케줄 작업용 CronJob 매니페스트',
    content: [
      'apiVersion: batch/v1',
      'kind: CronJob',
      'metadata:',
      '  name: batch-placeholder',
      'spec:',
      '  schedule: "*/10 * * * *"',
      '  jobTemplate:',
      '    spec:',
      '      template:',
      '        spec:',
      '          restartPolicy: OnFailure',
      '          containers:',
      '            - name: batch',
      '              image: harbor.local/batch-placeholder:latest',
    ].join('\n'),
  },
  {
    id: 'kustomize',
    label: 'Kustomize Base',
    description: 'Kustomization 기반 배포 구성',
    content: [
      'apiVersion: kustomize.config.k8s.io/v1beta1',
      'kind: Kustomization',
      'namespace: default',
      'resources:',
      '  - deployment.yaml',
      '  - service.yaml',
    ].join('\n'),
  },
]

const EXECUTE_SETUP_TABS: { id: ExecuteSetupTab; label: string; icon: React.ReactNode }[] = [
  { id: 'cluster', label: 'Cluster', icon: <Server size={13} /> },
  { id: 'build', label: 'Build', icon: <FileCode2 size={13} /> },
  { id: 'deploy', label: 'Deploy', icon: <Boxes size={13} /> },
]

function ExecuteModal({
  pipeline,
  onClose,
  onExecute,
  isExecuting,
}: {
  pipeline: Pipeline
  onClose: () => void
  onExecute: () => void
  isExecuting: boolean
}) {
  const { data: clustersData } = useClusters()
  const clusterList = clustersData?.items ?? []
  const targetClusters = clusterList.filter((c) => {
    const types = Array.isArray((c as any).types) ? (c as any).types : [(c as any).type ?? '']
    return types.flatMap((t: string) => t.split(',')).map((t: string) => t.trim().toLowerCase()).includes('target')
  })
  const clusterOptions = targetClusters.length > 0
    ? targetClusters.sort((a, b) => a.name.localeCompare(b.name)).map((c) => ({ id: c.id, name: c.name }))
    : [{ id: 'c1', name: 'prod-k8s' }, { id: 'c2', name: 'dev-k8s' }]

  const [activeTab, setActiveTab] = useState<ExecuteSetupTab>('cluster')
  const [clusterId, setClusterId] = useState(pipeline.clusterId || clusterOptions[0]?.id || '')
  const [dockerfileId, setDockerfileId] = useState(EXECUTE_DOCKERFILE_PRESETS[0].id)
  const [deployMode, setDeployMode] = useState<ExecuteDeployMode>('template')
  const [deployYamlId, setDeployYamlId] = useState(EXECUTE_DEPLOY_YAML_PRESETS[0].id)
  const [customDeployYaml, setCustomDeployYaml] = useState(EXECUTE_DEPLOY_YAML_PRESETS[0].content)

  const selectedDockerfile = EXECUTE_DOCKERFILE_PRESETS.find((p) => p.id === dockerfileId) ?? EXECUTE_DOCKERFILE_PRESETS[0]
  const selectedDeployYaml = EXECUTE_DEPLOY_YAML_PRESETS.find((p) => p.id === deployYamlId) ?? EXECUTE_DEPLOY_YAML_PRESETS[0]

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div
        className="relative flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
              <Rocket size={16} />
            </div>
            <div>
              <div className="text-[15px] font-bold text-[var(--color-text-primary)]">Execute Pipeline</div>
              <div className="text-[12px] text-[var(--color-text-secondary)]">{pipeline.name}</div>
            </div>
          </div>
          <button type="button" onClick={onClose} className="cursor-pointer rounded-lg p-1.5 text-[var(--color-text-secondary)] hover:bg-[rgba(255,255,255,0.06)] hover:text-[var(--color-text-primary)]">
            <X size={16} />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-[var(--color-border-default)]">
          {EXECUTE_SETUP_TABS.map((tab) => {
            const active = activeTab === tab.id
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={cn(
                  '-mb-px flex cursor-pointer items-center gap-1.5 border-b-2 px-5 py-2.5 text-[13px] transition-all duration-150',
                  active
                    ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]'
                    : 'border-b-transparent font-normal text-[var(--color-text-secondary)]'
                )}
              >
                {tab.icon}
                {tab.label}
              </button>
            )
          })}
        </div>

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto p-5">
          {activeTab === 'cluster' && (
            <div className="max-w-sm">
              <NativeSelect
                label="Deploy Cluster"
                value={clusterId}
                onChange={(e) => setClusterId(e.target.value)}
                className="w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
              >
                {clusterOptions.map((c) => (
                  <option key={c.id} value={c.id} className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">
                    {c.name}
                  </option>
                ))}
              </NativeSelect>
            </div>
          )}

          {activeTab === 'build' && (
            <div className="flex flex-col gap-4">
              <p className="m-0 text-[13px] text-[var(--color-text-secondary)]">Select the Dockerfile to use in the build stage.</p>
              <div className="grid grid-cols-3 gap-2.5">
                {EXECUTE_DOCKERFILE_PRESETS.map((preset) => {
                  const selected = dockerfileId === preset.id
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      onClick={() => setDockerfileId(preset.id)}
                      className={cn(
                        'cursor-pointer rounded-lg border p-3 text-left transition-all duration-150',
                        selected
                          ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                          : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
                      )}
                    >
                      <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>{preset.label}</div>
                      <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{preset.path}</div>
                    </button>
                  )
                })}
              </div>
              <YamlEditor value={selectedDockerfile.content} readOnly height="240px" />
            </div>
          )}

          {activeTab === 'deploy' && (
            <div className="flex flex-col gap-4">
              <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2">
                {(['template', 'custom'] as ExecuteDeployMode[]).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    onClick={() => setDeployMode(mode)}
                    className={cn(
                      'cursor-pointer rounded-md px-3 py-1.5 text-xs font-semibold',
                      deployMode === mode ? 'bg-[rgba(99,102,241,0.2)] text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]'
                    )}
                  >
                    {mode === 'template' ? 'Select Template YAML' : 'Write Custom YAML'}
                  </button>
                ))}
              </div>

              {deployMode === 'template' ? (
                <>
                  <div className="grid grid-cols-3 gap-2.5">
                    {EXECUTE_DEPLOY_YAML_PRESETS.map((preset) => {
                      const selected = deployYamlId === preset.id
                      return (
                        <button
                          key={preset.id}
                          type="button"
                          onClick={() => setDeployYamlId(preset.id)}
                          className={cn(
                            'cursor-pointer rounded-lg border p-3 text-left transition-all duration-150',
                            selected
                              ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                              : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
                          )}
                        >
                          <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>{preset.label}</div>
                          <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{preset.description}</div>
                        </button>
                      )
                    })}
                  </div>
                  <YamlEditor value={selectedDeployYaml.content} readOnly height="240px" />
                </>
              ) : (
                <YamlEditor value={customDeployYaml} onChange={setCustomDeployYaml} height="240px" />
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 border-t border-[var(--color-border-default)] px-5 py-4">
          <Button variant="secondary" size="sm" type="button" onClick={onClose} disabled={isExecuting}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" type="button" onClick={onExecute} loading={isExecuting} disabled={isExecuting}>
            <Rocket size={12} />
            Execute
          </Button>
        </div>
      </div>
    </div>
  )
}

// ── Pipeline inner tabs ────────────────────────────────────────────────────────

type PipelineInnerTab = 'info' | 'monitoring' | 'history'

const INNER_TABS: Array<{ key: PipelineInnerTab; label: string; icon: React.ReactNode }> = [
  { key: 'info', label: 'Info', icon: <Info size={13} /> },
  { key: 'monitoring', label: 'Monitoring', icon: <BarChart2 size={13} /> },
  { key: 'history', label: 'History', icon: <History size={13} /> },
]

function DetailCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4">
      <div className="mb-3 text-[12px] font-semibold uppercase tracking-[0.02em] text-[var(--color-text-secondary)]">
        {title}
      </div>
      {children}
    </div>
  )
}

function ConfigRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 text-[13px]">
      <span className="text-[var(--color-text-secondary)]">{label}</span>
      <span className="font-semibold text-[var(--color-text-primary)]">{value}</span>
    </div>
  )
}

type PipelineResourceNode = {
  kind: string
  name: string
  status: string
  labelSelector?: string
  serviceUrls?: string[]
}

type StageState = 'queued' | 'in_progress' | 'completed' | 'failed'

function pickResourcesByKind(resources: PipelineResourceNode[], kinds: string[]): PipelineResourceNode[] {
  const lowered = kinds.map((kind) => kind.toLowerCase())
  return resources.filter((resource) => lowered.includes(resource.kind.toLowerCase()))
}

function buildStageStates(stageCount: number, deploymentStatus?: string): StageState[] {
  if (stageCount <= 0) return []
  const normalized = (deploymentStatus ?? '').toLowerCase()
  if (!normalized) return Array.from({ length: stageCount }, () => 'queued')

  if (normalized === 'success') {
    return Array.from({ length: stageCount }, () => 'completed')
  }
  if (normalized === 'running') {
    return Array.from({ length: stageCount }, (_, i) => (i < stageCount - 1 ? 'completed' : 'in_progress'))
  }
  if (normalized === 'failed') {
    return Array.from({ length: stageCount }, (_, i) => {
      if (i < stageCount - 1) return 'completed'
      return 'failed'
    })
  }
  return Array.from({ length: stageCount }, () => 'queued')
}

function stageMeta(state: StageState): { icon: React.ReactNode; label: string; cls: string } {
  if (state === 'completed') {
    return {
      icon: <CheckCircle2 size={15} />,
      label: 'Completed',
      cls: 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)] text-[#86efac]',
    }
  }
  if (state === 'failed') {
    return {
      icon: <XCircle size={15} />,
      label: 'Failed',
      cls: 'border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] text-[#fca5a5]',
    }
  }
  if (state === 'in_progress') {
    return {
      icon: <Loader2 size={15} className="animate-spin" />,
      label: 'In progress',
      cls: 'border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)] text-[#fcd34d]',
    }
  }
  return {
    icon: <CircleDashed size={15} />,
    label: 'Queued',
    cls: 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-muted)]',
  }
}

function statusClass(status: string): string {
  const normalized = status.toLowerCase()
  if (normalized === 'running' || normalized === 'completed') return 'bg-[rgba(34,197,94,0.18)] text-[#86efac]'
  if (normalized.includes('crash') || normalized === 'failed' || normalized === 'degraded') return 'bg-[rgba(239,68,68,0.2)] text-[#fca5a5]'
  if (normalized === 'updating' || normalized === 'progressing' || normalized === 'scheduled') return 'bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
  return 'bg-[rgba(148,163,184,0.18)] text-[#cbd5e1]'
}

function logLineClass(line: string): string {
  const normalized = line.toLowerCase()
  if (normalized.startsWith('$')) return 'text-[#58a6ff]'
  if (normalized.includes('error') || normalized.includes('failed') || normalized.includes('panic')) return 'text-[#fca5a5]'
  if (normalized.includes('created') || normalized.includes('applied') || normalized.includes('ready') || normalized.includes('running')) return 'text-[#86efac]'
  if (normalized.includes('warning') || normalized.includes('progress') || normalized.includes('waiting')) return 'text-[#fcd34d]'
  return 'text-[#cbd5e1]'
}

function modeLabel(mode: Pipeline['mode']): string {
  if (mode === 'ci') return 'CI'
  if (mode === 'cd') return 'CD'
  return 'CI/CD'
}

function ModeIndicator({ mode }: { mode: Pipeline['mode'] }) {
  const ciOn = mode === 'ci' || mode === 'ci_cd'
  const cdOn = mode === 'cd' || mode === 'ci_cd'

  return (
    <div className="flex items-center gap-1.5">
      <span
        className={`inline-flex items-center gap-1 rounded-md border px-2 py-[2px] text-[11px] font-semibold ${
          ciOn
            ? 'border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.15)] text-[#86efac]'
            : 'border-[var(--color-border-default)] bg-[rgba(148,163,184,0.1)] text-[var(--color-text-muted)]'
        }`}
      >
        <span className={`h-1.5 w-1.5 rounded-full ${ciOn ? 'bg-[#22c55e]' : 'bg-[#64748b]'}`} />
        CI
      </span>
      <span
        className={`inline-flex items-center gap-1 rounded-md border px-2 py-[2px] text-[11px] font-semibold ${
          cdOn
            ? 'border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.15)] text-[#93c5fd]'
            : 'border-[var(--color-border-default)] bg-[rgba(148,163,184,0.1)] text-[var(--color-text-muted)]'
        }`}
      >
        <span className={`h-1.5 w-1.5 rounded-full ${cdOn ? 'bg-[#60a5fa]' : 'bg-[#64748b]'}`} />
        CD
      </span>
    </div>
  )
}

function ResourceNode({
  title,
  resources,
  accentClass,
  emptyLabel = '-',
}: {
  title: string
  resources: PipelineResourceNode[]
  accentClass: string
  emptyLabel?: string
}) {
  return (
    <div className="min-h-[84px] min-w-0 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2.5">
      <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
        {title}
      </div>
      {resources.length > 0 ? (
        <div className="flex flex-col gap-1.5">
          {resources.map((resource) => (
            <div key={`${resource.kind}-${resource.name}`} className="min-w-0 overflow-hidden rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-2 py-1.5">
              <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                <span className={`max-w-full truncate rounded px-1.5 py-0.5 font-mono text-[11px] ${accentClass}`}>
                  {resource.name}
                </span>
                <span className={`rounded px-1.5 py-0.5 text-[10px] font-bold uppercase ${statusClass(resource.status || 'unknown')}`}>
                  {resource.status || 'unknown'}
                </span>
              </div>
              {resource.labelSelector && (
                <div className="mt-1 break-all font-mono text-[10px] text-[var(--color-text-muted)]">
                  selector: {resource.labelSelector}
                </div>
              )}
              {resource.serviceUrls && resource.serviceUrls.length > 0 && (
                <div className="mt-1 flex min-w-0 flex-wrap gap-1">
                  {resource.serviceUrls.slice(0, 2).map((url) => (
                    <code key={url} className="max-w-full break-all rounded bg-[rgba(255,255,255,0.07)] px-1.5 py-[1px] text-[10px] text-[var(--color-text-secondary)]">
                      {url}
                    </code>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="text-[12px] text-[var(--color-text-muted)]">{emptyLabel}</div>
      )}
    </div>
  )
}

const MANIFEST_ENV_KEYS = new Set(['NULLUS_MANIFEST_DEPLOYMENT', 'NULLUS_MANIFEST_SERVICE', 'NULLUS_MANIFEST_INGRESS'])

function PipelineInfoTab({ pipeline }: { pipeline: Pipeline }) {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const { data: template } = useTemplateById(pipeline.templateId)
  const { data: deploymentsData, isLoading: isDeploymentsLoading } = usePipelineDeployments(pipeline.id)
  const { data: resourcesData, isLoading: isResourcesLoading } = usePipelineResources(pipeline.id)
  const [revealedVars, setRevealedVars] = useState<Set<string>>(new Set())

  const toggleReveal = (key: string) => {
    setRevealedVars((prev) => {
      const next = new Set(prev)
      next.has(key) ? next.delete(key) : next.add(key)
      return next
    })
  }

  const deployments = deploymentsData?.items ?? []
  const latestDeployment = [...deployments].sort(
    (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
  )[0]
  const resources = resourcesData?.items ?? []
  const ingressResources = pickResourcesByKind(resources, ['Ingress'])
  const serviceResources = pickResourcesByKind(resources, ['Service'])
  const workloadResources = pickResourcesByKind(resources, ['Deployment', 'StatefulSet'])
  const podResources = pickResourcesByKind(resources, ['Pod'])
  const jobResources = pickResourcesByKind(resources, ['Job', 'CronJob'])

  const allServiceUrls = Array.from(new Set([
    ...ingressResources.flatMap((r) => r.serviceUrls ?? []),
    ...serviceResources.flatMap((r) => r.serviceUrls ?? []),
  ])).filter(Boolean)

  const envEntries = Object.entries(pipeline.envVars ?? {}).filter(([k]) => !MANIFEST_ENV_KEYS.has(k))
  const manifestEntries = Object.entries(pipeline.envVars ?? {}).filter(([k]) => MANIFEST_ENV_KEYS.has(k))

  const modeLabel = pipeline.executionMode === 'ci' ? 'CI Only' : pipeline.executionMode === 'cd' ? 'CD Only' : 'CI/CD'

  return (
    <div className="flex flex-col gap-4">

      {/* service url open buttons */}
      {allServiceUrls.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {allServiceUrls.map((url) => (
            <a
              key={url}
              href={url.startsWith('http') ? url : `http://${url}`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-lg border border-[rgba(99,102,241,0.4)] bg-[rgba(99,102,241,0.1)] px-3 py-1.5 text-[12px] font-semibold text-[#a5b4fc] transition-colors hover:bg-[rgba(99,102,241,0.2)]"
            >
              <ExternalLink size={12} />
              {url}
            </a>
          ))}
        </div>
      )}

      {/* basic info */}
      <DetailCard title="Pipeline Info">
        <div className="flex flex-col gap-2.5">
          <ConfigRow label="Name" value={<span className="font-semibold">{pipeline.name}</span>} />
          <ConfigRow label="App Type" value={pipeline.appType} />
          <ConfigRow label="Execution Mode" value={
            <span className="rounded-md border border-[var(--color-border-default)] bg-[rgba(99,102,241,0.08)] px-2 py-[2px] text-[11px] font-semibold text-[#c7d2fe]">
              {modeLabel}
            </span>
          } />
          <ConfigRow label="Template" value={template?.name ?? (pipeline.templateId || '-')} />
          <ConfigRow label="Status" value={
            <span
              className="rounded-md px-[9px] py-[3px] text-xs font-semibold"
              style={{ backgroundColor: getPipelineStatusStyle(pipeline.status).bg, color: getPipelineStatusStyle(pipeline.status).color }}
            >
              {getPipelineStatusLabel(t, pipeline.status)}
            </span>
          } />
          <ConfigRow label="Created" value={formatDateTime(pipeline.createdAt, locale)} />
          <ConfigRow label="Last Deployed" value={formatDateTime(pipeline.lastDeployedAt, locale)} />
        </div>
      </DetailCard>

      {/* code checkout */}
      <DetailCard title="Code Checkout">
        <div className="flex flex-col gap-2.5">
          <ConfigRow
            label="Git Repository"
            value={
              pipeline.gitRepoUrl ? (
                <code className="max-w-[260px] truncate rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]" title={pipeline.gitRepoUrl}>
                  {pipeline.gitRepoUrl}
                </code>
              ) : '-'
            }
          />
          {pipeline.stackId && (
            <ConfigRow label="Stack" value={
              <code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.stackId}</code>
            } />
          )}
        </div>
      </DetailCard>

      {/* build */}
      {pipeline.dockerfilePath && (
        <DetailCard title="Build">
          <div className="flex flex-col gap-2.5">
            <ConfigRow
              label="Dockerfile"
              value={<code className="max-w-[260px] truncate rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]" title={pipeline.dockerfilePath}>{pipeline.dockerfilePath}</code>}
            />
            <ConfigRow
              label="Build Context"
              value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.dockerContext || '.'}</code>}
            />
            {(pipeline.envVars ?? {})['IMAGE_REGISTRY_URL'] && (
              <ConfigRow
                label="Image Registry"
                value={<code className="max-w-[260px] truncate rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{(pipeline.envVars ?? {})['IMAGE_REGISTRY_URL']}</code>}
              />
            )}
          </div>
        </DetailCard>
      )}

      {/* deployment target */}
      <DetailCard title="Deployment Target">
        <div className="flex flex-col gap-2.5">
          <ConfigRow label="Cluster" value={pipeline.clusterName || pipeline.clusterId} />
          <ConfigRow
            label="Namespace"
            value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.namespace}</code>}
          />
        </div>

        <div className="mt-4 rounded-lg border border-[var(--color-border-default)] bg-[rgba(15,23,42,0.45)] p-3">
          <div className="mb-2 text-[12px] font-semibold text-[var(--color-text-primary)]">Deployed Resources</div>

          {(isDeploymentsLoading || isResourcesLoading) && (
            <div className="text-[12px] text-[var(--color-text-secondary)]">Loading resources...</div>
          )}
          {!isDeploymentsLoading && !isResourcesLoading && deployments.length === 0 && (
            <div className="text-[12px] text-[var(--color-text-secondary)]">No deployment history yet.</div>
          )}
          {!isDeploymentsLoading && !isResourcesLoading && deployments.length > 0 && (
            <div className="space-y-2">
              <div className="text-[11px] text-[var(--color-text-secondary)]">
                Latest: <strong className="text-[var(--color-text-primary)]">{latestDeployment?.version ?? '-'}</strong>
              </div>
              <div className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)] md:items-stretch">
                <ResourceNode title="Ingress" resources={ingressResources} accentClass="bg-[rgba(34,197,94,0.12)] text-[#86efac]" />
                <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">→</div>
                <ResourceNode title="Service" resources={serviceResources} accentClass="bg-[rgba(59,130,246,0.12)] text-[#93c5fd]" />
                <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">→</div>
                <ResourceNode title="Deployment / StatefulSet" resources={workloadResources} accentClass="bg-[rgba(129,140,248,0.12)] text-[#c7d2fe]" />
                <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">→</div>
                <ResourceNode title="Pod" resources={podResources} accentClass="bg-[rgba(251,191,36,0.14)] text-[#fde68a]" emptyLabel={workloadResources.length > 0 ? '(managed by workload)' : '-'} />
              </div>
              {jobResources.length > 0 && (
                <ResourceNode title="Job / CronJob" resources={jobResources} accentClass="bg-[rgba(14,165,233,0.16)] text-[#7dd3fc]" />
              )}
            </div>
          )}
        </div>
      </DetailCard>

      {/* env vars */}
      {envEntries.length > 0 && (
        <DetailCard title="Environment Variables">
          <div className="flex flex-col gap-2">
            {envEntries.map(([key, value]) => {
              const isRevealed = revealedVars.has(key)
              return (
                <div key={key} className="grid grid-cols-[1fr_1fr_88px] items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-[12px]">
                  <span className="font-mono text-[var(--color-text-primary)]">{key}</span>
                  <span className="truncate font-mono text-[var(--color-text-secondary)]">{isRevealed ? value : '••••••••'}</span>
                  <button
                    type="button"
                    onClick={() => toggleReveal(key)}
                    className="inline-flex items-center justify-center gap-1 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[2px] text-[11px] text-[var(--color-text-secondary)]"
                  >
                    {isRevealed ? <EyeOff size={11} /> : <Eye size={11} />}
                    {isRevealed ? 'Hide' : 'Show'}
                  </button>
                </div>
              )
            })}
          </div>
        </DetailCard>
      )}

      {/* manifest overrides */}
      {manifestEntries.length > 0 && (
        <DetailCard title="Manifest Overrides">
          <div className="flex flex-col gap-2">
            {manifestEntries.map(([key, value]) => {
              const isRevealed = revealedVars.has(key)
              const shortKey = key.replace('NULLUS_MANIFEST_', '')
              return (
                <div key={key} className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2.5">
                  <div className="mb-1.5 flex items-center justify-between">
                    <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]">{shortKey}</span>
                    <button
                      type="button"
                      onClick={() => toggleReveal(key)}
                      className="inline-flex items-center gap-1 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[2px] text-[11px] text-[var(--color-text-secondary)]"
                    >
                      {isRevealed ? <EyeOff size={11} /> : <Eye size={11} />}
                      {isRevealed ? 'Hide' : 'Show'}
                    </button>
                  </div>
                  {isRevealed && (
                    <pre className="max-h-40 overflow-y-auto whitespace-pre-wrap break-all rounded bg-[#0d1117] p-2 font-mono text-[11px] text-[#94a3b8]">
                      {value}
                    </pre>
                  )}
                </div>
              )
            })}
          </div>
        </DetailCard>
      )}
    </div>
  )
}

function resourceStatusColor(status: string) {
  const s = status.toLowerCase()
  if (s === 'running' || s === 'active' || s === 'healthy') return { dot: '#22c55e', bg: 'rgba(34,197,94,0.1)', text: '#22c55e' }
  if (s === 'pending' || s === 'progressing' || s === 'updating') return { dot: '#f59e0b', bg: 'rgba(245,158,11,0.1)', text: '#f59e0b' }
  if (s === 'failed' || s === 'error' || s === 'degraded') return { dot: '#ef4444', bg: 'rgba(239,68,68,0.1)', text: '#ef4444' }
  return { dot: '#94a3b8', bg: 'rgba(148,163,184,0.08)', text: '#94a3b8' }
}

function kindIcon(kind: string) {
  const k = kind.toLowerCase()
  if (k === 'deployment' || k === 'statefulset') return <Box size={13} />
  if (k === 'service') return <Activity size={13} />
  if (k === 'ingress') return <Globe size={13} />
  if (k === 'pod') return <Server size={13} />
  return <Package size={13} />
}

function PipelineMonitoringTab({ pipeline }: { pipeline: Pipeline }) {
  const { i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const { data: deploymentsData, isLoading: isDeploymentsLoading } = usePipelineDeployments(pipeline.id)
  const { data: resourcesData, isLoading: isResourcesLoading, refetch: refetchResources } = usePipelineResources(pipeline.id)
  const deployments = deploymentsData?.items ?? []
  const resources = resourcesData?.items ?? []

  const total = deployments.length
  const successCount = deployments.filter((d) => d.status === 'success').length
  const failedCount = deployments.filter((d) => d.status === 'failed').length
  const successRate = total > 0 ? Math.round((successCount / total) * 100) : 0
  const latestDeployment = [...deployments].sort(
    (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime()
  )[0]

  const deploymentsWithDuration = deployments.filter((d) => d.startedAt && d.completedAt)
  const avgDurationMs =
    deploymentsWithDuration.reduce((acc, d) => {
      return acc + (new Date(d.completedAt as string).getTime() - new Date(d.startedAt).getTime())
    }, 0) / Math.max(deploymentsWithDuration.length, 1)
  const avgDuration =
    avgDurationMs > 60000
      ? `${Math.round(avgDurationMs / 60000)}m ${Math.round((avgDurationMs % 60000) / 1000)}s`
      : `${Math.round(avgDurationMs / 1000)}s`

  const trendMap = new Map<string, { success: number; failed: number }>()
  for (const d of deployments) {
    const date = formatDate(d.startedAt, locale, { month: 'numeric', day: 'numeric' })
    const entry = trendMap.get(date) ?? { success: 0, failed: 0 }
    if (d.status === 'success') entry.success += 1
    else if (d.status === 'failed') entry.failed += 1
    trendMap.set(date, entry)
  }
  const buildTrend = [...trendMap.entries()].slice(-7).map(([date, counts]) => ({ date, ...counts }))

  const workloadResources = resources.filter((r) =>
    ['deployment', 'statefulset', 'daemonset'].includes(r.kind.toLowerCase())
  )
  const serviceResources = resources.filter((r) => r.kind.toLowerCase() === 'service')
  const ingressResources = resources.filter((r) => r.kind.toLowerCase() === 'ingress')
  const podResources = resources.filter((r) => r.kind.toLowerCase() === 'pod')

  const runningPods = podResources.filter((r) => r.status.toLowerCase() === 'running').length
  const totalPods = podResources.length

  const isLoading = isDeploymentsLoading || isResourcesLoading

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <Loader2 size={18} className="animate-spin text-[#818cf8]" />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      {/* stat cards */}
      <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        {[
          { label: 'Success Rate', value: total > 0 ? `${successRate}%` : '-', sub: `${successCount}/${total} runs`, color: '#10b981' },
          { label: 'Total Runs', value: String(total), sub: latestDeployment ? `Last: ${formatDateTime(latestDeployment.startedAt, locale)}` : 'No runs yet', color: '#818cf8' },
          { label: 'Avg Duration', value: total > 0 ? avgDuration : '-', sub: `${deploymentsWithDuration.length} measured`, color: '#f59e0b' },
          { label: 'Failed', value: String(failedCount), sub: total > 0 ? `${Math.round((failedCount / total) * 100)}% failure rate` : '-', color: failedCount > 0 ? '#ef4444' : '#94a3b8' },
        ].map((item) => (
          <div key={item.label} className="rounded-xl border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4">
            <div className="text-[26px] font-extrabold leading-none" style={{ color: item.color }}>{item.value}</div>
            <div className="mt-1 text-[12px] font-semibold text-[var(--color-text-secondary)]">{item.label}</div>
            <div className="mt-1 text-[11px] text-[var(--color-text-secondary)] opacity-70">{item.sub}</div>
          </div>
        ))}
      </div>

      {/* live k8s resources */}
      <div className="rounded-xl border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4">
        <div className="mb-3 flex items-center justify-between">
          <span className="text-[13px] font-bold text-[var(--color-text-primary)]">Live Resources</span>
          <div className="flex items-center gap-3">
            {totalPods > 0 && (
              <span className="text-[12px] text-[var(--color-text-secondary)]">
                <span className="font-semibold text-[#22c55e]">{runningPods}</span>/{totalPods} pods running
              </span>
            )}
            <button
              type="button"
              onClick={() => void refetchResources()}
              className="rounded p-1 text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              <RefreshCw size={12} />
            </button>
          </div>
        </div>

        {resources.length === 0 ? (
          <div className="py-6 text-center text-[13px] text-[var(--color-text-secondary)]">
            No resources found. Run a deployment first.
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {[
              { label: 'Workloads', items: workloadResources },
              { label: 'Services', items: serviceResources },
              { label: 'Ingress', items: ingressResources },
              { label: 'Pods', items: podResources },
            ]
              .filter((group) => group.items.length > 0)
              .map((group) => (
                <div key={group.label}>
                  <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
                    {group.label}
                  </div>
                  <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
                    {group.items.map((r) => {
                      const sc = resourceStatusColor(r.status)
                      return (
                        <div
                          key={`${r.kind}-${r.name}`}
                          className="flex items-center gap-2.5 rounded-lg border border-[var(--color-border-default)] px-3 py-2"
                          style={{ background: sc.bg }}
                        >
                          <span className="shrink-0" style={{ color: sc.text }}>{kindIcon(r.kind)}</span>
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-[12px] font-semibold text-[var(--color-text-primary)]">{r.name}</div>
                            {r.serviceUrls && r.serviceUrls.length > 0 && (
                              <div className="truncate text-[11px] text-[var(--color-text-secondary)]">{r.serviceUrls[0]}</div>
                            )}
                          </div>
                          <span
                            className="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold capitalize"
                            style={{ background: sc.bg, color: sc.text, border: `1px solid ${sc.dot}40` }}
                          >
                            {r.status}
                          </span>
                        </div>
                      )
                    })}
                  </div>
                </div>
              ))}
          </div>
        )}
      </div>

      {/* deployment trend chart */}
      {buildTrend.length > 0 && (
        <div className="rounded-xl border border-[var(--color-border-default)] bg-[#0b1220] p-4">
          <h4 className="m-0 mb-3 text-[13px] font-bold text-[#f8fafc]">Deployment Trend (last 7 days)</h4>
          <ResponsiveContainer width="100%" height={160}>
            <BarChart data={buildTrend} barSize={14}>
              <CartesianGrid stroke="rgba(148,163,184,0.12)" strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="date" stroke="#475569" tick={{ fill: '#94a3b8', fontSize: 11 }} axisLine={false} tickLine={false} />
              <YAxis stroke="#475569" tick={{ fill: '#94a3b8', fontSize: 11 }} axisLine={false} tickLine={false} allowDecimals={false} />
              <Tooltip
                contentStyle={{ background: '#111827', border: '1px solid #1e293b', borderRadius: 8, color: '#e2e8f0', fontSize: 12 }}
                cursor={{ fill: 'rgba(99,102,241,0.06)' }}
              />
              <Bar dataKey="success" name="Success" fill="#6366f1" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" name="Failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

function PipelineHistoryTab({ pipeline }: { pipeline: Pipeline }) {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const { data: template } = useTemplateById(pipeline.templateId)
  const { data: deploymentsData, isLoading } = usePipelineDeployments(pipeline.id)
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null)
  const deployments = deploymentsData?.items ?? []
  const stages = (template?.stages ?? []) as string[]

  useEffect(() => {
    if (deployments.length === 0) {
      setSelectedDeploymentId(null)
      return
    }
    if (!selectedDeploymentId || !deployments.some((d) => d.id === selectedDeploymentId)) {
      setSelectedDeploymentId(deployments[0].id)
    }
  }, [deployments, selectedDeploymentId])

  const selectedDeployment = deployments.find((d) => d.id === selectedDeploymentId) ?? null
  const { data: deploymentStatus, isLoading: isDeploymentStatusLoading } = useDeploymentStatus(selectedDeploymentId)
  const selectedStageStates = buildStageStates(stages.length, deploymentStatus?.status ?? selectedDeployment?.status)
  const stepDetails = deploymentStatus?.steps ?? []
  const logLineCount = stepDetails.reduce((total, step) => total + (step.logs?.length ?? 0), 0)

  if (isLoading) {
    return <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">Loading deployment history...</div>
  }

  if (deployments.length === 0) {
    return <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">{t('cicdListPage.emptyDeployments', 'No deployment history.')}</div>
  }

  return (
    <div className="flex flex-col gap-3">
      {deployments.map((d) => {
        const st = getPipelineStatusStyle(d.status)
        const durationMs =
          d.completedAt && d.startedAt
            ? new Date(d.completedAt).getTime() - new Date(d.startedAt).getTime()
            : 0
        const duration =
          durationMs > 0
            ? durationMs >= 60000
              ? `${Math.floor(durationMs / 60000)}m ${Math.round((durationMs % 60000) / 1000)}s`
              : `${Math.round(durationMs / 1000)}s`
            : d.status === 'running'
              ? 'running'
              : '-'
        const isSelected = d.id === selectedDeploymentId

        return (
          <div
            key={d.id}
            className={`flex flex-wrap items-center gap-2.5 rounded-lg border px-3.5 py-3 ${
              isSelected
                ? 'border-[rgba(99,102,241,0.45)] bg-[rgba(99,102,241,0.12)]'
                : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
            }`}
          >
            <span className="rounded-full px-2.5 py-0.5 text-[11px] font-bold" style={{ background: st.bg, color: st.color }}>
              {getPipelineStatusLabel(t, d.status)}
            </span>
            <button
              type="button"
              onClick={() => setSelectedDeploymentId(d.id)}
              className="rounded px-1 py-0.5 text-[13px] font-semibold text-[#a5b4fc] underline decoration-dotted underline-offset-2 hover:text-[#c7d2fe]"
            >
              {d.version}
            </button>
            <span className="flex-1 text-[12px] text-[var(--color-text-secondary)]">{d.triggeredBy || '-'}</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">{duration}</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">{formatDateTime(d.startedAt, locale)}</span>
          </div>
        )
      })}

      {selectedDeployment && (
        <div className="rounded-lg border border-[rgba(99,102,241,0.35)] bg-[rgba(15,23,42,0.5)] p-3">
          <div className="flex flex-wrap items-center gap-2 text-[12px] text-[var(--color-text-secondary)]">
            <span className="rounded bg-[rgba(99,102,241,0.2)] px-1.5 py-[2px] font-mono text-[#c7d2fe]">
              {selectedDeployment.version}
            </span>
            <span>Deployment ID:</span>
            <code className="rounded bg-[rgba(255,255,255,0.08)] px-1.5 py-[2px]">{selectedDeployment.id}</code>
            <span>Triggered by:</span>
            <span className="text-[var(--color-text-primary)]">{selectedDeployment.triggeredBy || '-'}</span>
          </div>

          {stages.length > 0 && (
            <div className="mt-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2">
              <div className="mb-2 text-[12px] font-semibold text-[var(--color-text-primary)]">Pipeline Stages</div>
              {stages.map((stage: string, i: number) => {
                const state = selectedStageStates[i] ?? 'queued'
                const meta = stageMeta(state)
                return (
                  <div key={`${selectedDeployment.id}-${stage}`} className="relative">
                    {i < stages.length - 1 && (
                      <div className="absolute left-[17px] top-8 h-[calc(100%-8px)] w-px bg-[rgba(148,163,184,0.3)]" />
                    )}
                    <div className={`mb-2 grid grid-cols-[26px_1fr_auto] items-center gap-2 rounded-md border px-2.5 py-2 ${meta.cls}`}>
                      <span className="flex items-center justify-center">{meta.icon}</span>
                      <div className="min-w-0">
                        <div className="truncate text-[12px] font-semibold">{stage}</div>
                        <div className="text-[10px] opacity-80">{meta.label}</div>
                      </div>
                      <span className="rounded bg-[rgba(255,255,255,0.08)] px-1.5 py-[1px] text-[10px] font-mono">
                        step {i + 1}
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>
          )}

          <div className="mt-3 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[#0d1117]">
            <div className="flex flex-wrap items-center gap-2 border-b border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.03)] px-3 py-2 text-[11px] text-[rgba(255,255,255,0.65)]">
              <span>Detailed Logs</span>
              <span>·</span>
              <span>{stepDetails.length} steps</span>
              <span>·</span>
              <span>{logLineCount} lines</span>
              {isDeploymentStatusLoading && <span className="text-[#fcd34d]">Loading...</span>}
            </div>

            <div className="max-h-[460px] space-y-3 overflow-y-auto p-3 font-mono text-[12px]">
              {!isDeploymentStatusLoading && stepDetails.length === 0 && (
                <div className="text-[12px] text-[#94a3b8]">No detailed logs available for this deployment.</div>
              )}

              {stepDetails.map((step, stepIndex) => (
                <div key={`${selectedDeployment.id}-${step.name}-${stepIndex}`} className="rounded border border-[rgba(148,163,184,0.25)] bg-[rgba(2,6,23,0.65)]">
                  <div className="flex flex-wrap items-center gap-2 border-b border-[rgba(148,163,184,0.25)] px-2.5 py-2 text-[11px] text-[#94a3b8]">
                    <span className="font-semibold text-[#cbd5e1]">{step.name}</span>
                    {step.kind && <span className="rounded bg-[rgba(148,163,184,0.2)] px-1.5 py-[1px]">{step.kind}</span>}
                    {step.status && (
                      <span className={`rounded px-1.5 py-[1px] uppercase ${statusClass(step.status)}`}>
                        {step.status}
                      </span>
                    )}
                    {step.applied_at && <span>{formatDateTime(step.applied_at, locale)}</span>}
                  </div>
                  <div className="space-y-1 px-2.5 py-2">
                    {(step.logs ?? []).map((line, lineIndex) => (
                      <div key={`${selectedDeployment.id}-${step.name}-${lineIndex}`} className="grid grid-cols-[30px_minmax(0,1fr)] gap-2">
                        <span className="text-right text-[10px] text-[#64748b]">{lineIndex + 1}</span>
                        <span className={`break-all ${logLineClass(line)}`}>{line}</span>
                      </div>
                    ))}
                    {(step.logs ?? []).length === 0 && (
                      <div className="text-[11px] text-[#94a3b8]">{step.message || 'No log lines for this step.'}</div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function PipelineDetailPanel({
  pipeline,
  onExecuteClick,
  onOpenLogs,
  onDelete,
  isDeleting,
}: {
  pipeline: Pipeline
  onExecuteClick: () => void
  onOpenLogs: () => void
  onDelete: () => void
  isDeleting: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [innerTab, setInnerTab] = useState<PipelineInnerTab>('info')
  const statusStyle = getPipelineStatusStyle(pipeline.status)

  return (
    <div className="mt-2.5 overflow-hidden rounded-[var(--card-radius)] border border-[rgba(99,102,241,0.3)] bg-[var(--color-surface-card)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-5 py-3.5">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
            <GitBranch size={16} />
          </div>
          <h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">{pipeline.name}</h3>
          <span className="rounded-[10px] px-[9px] py-[3px] text-[11px] font-bold" style={{ background: statusStyle.bg, color: statusStyle.color }}>
            {getPipelineStatusLabel(t, pipeline.status)}
          </span>
          <span className="text-[12px] text-[var(--color-text-secondary)]">
            · {pipeline.appType} · {pipeline.clusterName} · {formatDateTime(pipeline.lastDeployedAt, locale)}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          <Button
            variant="secondary"
            size="sm"
            type="button"
            className="border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.15)] text-[#fecaca] hover:bg-[rgba(239,68,68,0.25)]"
            onClick={onDelete}
            disabled={isDeleting}
          >
            <Trash2 size={12} />
            {isDeleting ? 'Deleting...' : 'Delete'}
          </Button>
          <Button variant="secondary" size="sm" type="button" onClick={onOpenLogs}>
            <Terminal size={12} />
            Logs
          </Button>
          <Button variant="primary" size="sm" type="button" onClick={onExecuteClick}>
            <Rocket size={12} />
            Execute
          </Button>
        </div>
      </div>

      <div className="flex border-b border-[var(--color-border-default)]">
        {INNER_TABS.map((tab) => {
          const active = innerTab === tab.key
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => setInnerTab(tab.key)}
              className={`flex items-center gap-1.5 border-b-2 px-4 py-2.5 text-[13px] font-medium transition-all duration-150 ${
                active
                  ? 'border-b-[#6366f1] bg-[rgba(30,41,59,0.6)] text-[var(--color-text-primary)]'
                  : 'border-b-transparent text-[var(--color-text-secondary)] hover:bg-[rgba(99,102,241,0.08)] hover:text-[var(--color-text-primary)]'
              }`}
            >
              {tab.icon}
              {tab.label}
            </button>
          )
        })}
      </div>

      <div className="p-5">
        {innerTab === 'info' && <PipelineInfoTab pipeline={pipeline} />}
        {innerTab === 'monitoring' && <PipelineMonitoringTab pipeline={pipeline} />}
        {innerTab === 'history' && <PipelineHistoryTab pipeline={pipeline} />}
      </div>
    </div>
  )
}

export function CicdListPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState('')
  const [clusterFilter, setClusterFilter] = useState('')
  const [search, setSearch] = useState('')
  const [expandedPipelineId, setExpandedPipelineId] = useState<string | null>(null)
  const [deletingPipelineId, setDeletingPipelineId] = useState<string | null>(null)
  const [deployingPipelineId, setDeployingPipelineId] = useState<string | null>(null)
  const [executeModalPipeline, setExecuteModalPipeline] = useState<Pipeline | null>(null)
  const [viewportWidth, setViewportWidth] = useState(() =>
    typeof window !== 'undefined' ? window.innerWidth : 1440,
  )
  const isDesktopLayout = viewportWidth >= 1280

  useEffect(() => {
    if (typeof window === 'undefined') return
    const onResize = () => setViewportWidth(window.innerWidth)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const { data: clustersData } = useClusters()
  const { data: apiData } = usePipelines({ status: statusFilter || undefined, search: search || undefined })
  const deletePipelineMutation = useDeletePipeline()
  const deployPipelineMutation = useDeployPipeline()
  const pipelines = apiData?.items ?? []

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase()
    const matchesSearch = !search || p.name.toLowerCase().includes(q) || p.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || p.status === statusFilter
    const matchesCluster = !clusterFilter || p.clusterId === clusterFilter
    return matchesSearch && matchesStatus && matchesCluster
  })

  const selectedPipelineId = expandedPipelineId && filtered.some((pipeline) => pipeline.id === expandedPipelineId)
    ? expandedPipelineId
    : (filtered[0]?.id ?? null)
  const expandedPipeline = selectedPipelineId
    ? filtered.find((pipeline) => pipeline.id === selectedPipelineId) ?? null
    : null

  const columns: ColumnDef<Pipeline, unknown>[] = [
    {
      accessorKey: 'name',
      header: t('cicdListPage.table.name', 'Name'),
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          {selectedPipelineId === row.original.id && (
            <div className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#6366f1]" />
          )}
          <span className="font-semibold">{row.original.name}</span>
        </div>
      ),
    },
    {
      accessorKey: 'mode',
      header: 'Mode',
      cell: ({ row }) => (
        <span className="rounded-md border border-[var(--color-border-default)] bg-[rgba(99,102,241,0.08)] px-[8px] py-[2px] text-[11px] font-semibold text-[#c7d2fe]">
          {modeLabel(row.original.mode)}
        </span>
      ),
    },
    {
      accessorKey: 'appType',
      header: t('cicdListPage.table.appType', 'App Type'),
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.appType}</span>,
    },
    {
      accessorKey: 'clusterName',
      header: t('cicdListPage.table.cluster', 'Cluster'),
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.clusterName}</span>,
    },
    {
      accessorKey: 'status',
      header: t('cicdListPage.table.status', 'Status'),
      cell: ({ row }) => {
        const st = getPipelineStatusStyle(row.original.status)
        return (
          <span className="rounded-md px-[9px] py-[3px] text-xs font-semibold" style={{ backgroundColor: st.bg, color: st.color }}>
            {getPipelineStatusLabel(t, row.original.status)}
          </span>
        )
      },
    },
    {
      accessorKey: 'lastDeployedAt',
      header: t('cicdListPage.table.lastDeployed', 'Last Deployed'),
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDateTime(row.original.lastDeployedAt, locale)}</span>,
    },
  ]

  const handleDeletePipeline = async (pipeline: Pipeline) => {
    const confirmed = window.confirm(`Delete pipeline "${pipeline.name}"?\nThis also removes deployment history.`)
    if (!confirmed) return

    try {
      setDeletingPipelineId(pipeline.id)
      await deletePipelineMutation.mutateAsync(pipeline.id)
      if (selectedPipelineId === pipeline.id) {
        setExpandedPipelineId(null)
      }
    } finally {
      setDeletingPipelineId(null)
    }
  }

  const handleDeployPipeline = async (pipeline: Pipeline) => {
    try {
      setDeployingPipelineId(pipeline.id)
      const result = await deployPipelineMutation.mutateAsync({ pipelineId: pipeline.id })
      setExecuteModalPipeline(null)
      navigate(`/cicd/pipelines/${pipeline.id}/logs?deploymentId=${result.deploymentId}`)
    } finally {
      setDeployingPipelineId(null)
    }
  }

  return (
    <div>
      <Breadcrumb items={[{ label: t('sidebar.cicdList', 'CI/CD List') }]} />

      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <List size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('cicdListPage.title', 'CI/CD List')}
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              {t('cicdListPage.description', 'CI/CD Pipeline List')}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="md"
            onClick={() => navigate('/cicd/developer-deploy')}
            type="button"
          >
            <Plus size={15} />
            {t('cicd.addPhase', 'Add Phase')}
          </Button>
          <Button
            variant="primary"
            size="md"
            onClick={() => navigate('/cicd/templates')}
            type="button"
          >
            <Plus size={15} />
            {t('cicd.newPipeline', 'New Pipeline')}
          </Button>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(300px,38%)_minmax(0,62%)]">
        <div className="min-w-0">
          <DataTable
            columns={columns}
            data={filtered}
            getRowKey={(row) => row.id}
            onRowClick={(row) => setExpandedPipelineId(row.id)}
            emptyMessage={t('cicdListPage.emptyPipelines', 'No pipelines found.')}
            toolbar={
              <>
                <NativeSelect value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
                  <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicdListPage.filters.allStatus', 'All Status')}</option>
                  <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.success', 'Success')}</option>
                  <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.running', 'Running')}</option>
                  <option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.pending', 'Pending')}</option>
                  <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.failed', 'Failed')}</option>
                  <option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t('cicd.status.cancelled', 'Cancelled')}</option>
                </NativeSelect>
                <NativeSelect value={clusterFilter} onChange={(e) => setClusterFilter(e.target.value)} className="w-auto">
                  <option value="">{t('cicdListPage.filters.allClusters', 'All Clusters')}</option>
                  {(clustersData?.items ?? []).map((c) => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </NativeSelect>
                <div className="relative ml-auto">
                  <Search
                    size={13}
                    className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
                  />
                  <input
                    placeholder={t('cicdListPage.searchPlaceholder', 'Search pipelines...')}
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                  />
                </div>
              </>
            }
          />
          <div className="mt-2 hidden text-[12px] text-[var(--color-text-secondary)] xl:block">
            {t('cicdListPage.listHint', 'Selecting a pipeline from the list updates the detail panel immediately.')}
          </div>
        </div>

        {isDesktopLayout && (
          <div>
            {expandedPipeline ? (
              <div className="h-full pr-1">
                <PipelineDetailPanel
                  key={expandedPipeline.id}
                  pipeline={expandedPipeline}
                  onDelete={() => void handleDeletePipeline(expandedPipeline)}
                  isDeleting={deletingPipelineId === expandedPipeline.id}
                  onExecuteClick={() => setExecuteModalPipeline(expandedPipeline)}
                  onOpenLogs={() => navigate(`/cicd/pipelines/${expandedPipeline.id}/logs`)}
                />
              </div>
            ) : (
              <div className="rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-8 text-center text-[13px] text-[var(--color-text-secondary)]">
                {t('cicdListPage.emptyDetail', 'Select a pipeline from the list to view details here.')}
              </div>
            )}
          </div>
        )}
      </div>

      {!isDesktopLayout && expandedPipeline && (
        <PipelineDetailPanel
          key={`${expandedPipeline.id}-mobile`}
          pipeline={expandedPipeline}
          onDelete={() => void handleDeletePipeline(expandedPipeline)}
          isDeleting={deletingPipelineId === expandedPipeline.id}
          onExecuteClick={() => setExecuteModalPipeline(expandedPipeline)}
          onOpenLogs={() => navigate(`/cicd/pipelines/${expandedPipeline.id}/logs`)}
          activeDeploymentId={deployingPipelineId === expandedPipeline.id || activeDeploymentId ? activeDeploymentId : null}
        />
      )}

      {executeModalPipeline && (
        <ExecuteModal
          pipeline={executeModalPipeline}
          onClose={() => setExecuteModalPipeline(null)}
          onExecute={() => void handleDeployPipeline(executeModalPipeline)}
          isExecuting={deployingPipelineId === executeModalPipeline.id}
        />
      )}
    </div>
  )
}

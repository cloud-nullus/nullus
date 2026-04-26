import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  BarChart2,
  CheckCircle2,
  CircleDashed,
  Eye,
  EyeOff,
  GitBranch,
  History,
  Info,
  List,
  Plus,
  Rocket,
  XCircle,
  Search,
  Terminal,
  Trash2,
  Loader2,
} from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { useDeletePipeline, useDeploymentStatus, usePipelineDeployments, usePipelineResources, usePipelines, useTemplateById } from '../api/cicd-api'
import type { Pipeline } from '../api/cicd-api'
import { useScopedClusters as useClusters } from '../../admin/api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { formatDate, formatDateTime, resolveLocale } from '../../../lib/locale'
import { getPipelineStatusLabel, getPipelineStatusStyle } from '../utils/pipeline-status'


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
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
  }

  const hasBuildConfig = !!pipeline.dockerfilePath
  const envEntries = Object.entries(pipeline.envVars ?? {})
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
  const hasDeploymentResources = resources.length > 0

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-4">
        <DetailCard title="Pipeline Info">
          <div className="flex flex-col gap-2.5">
            <ConfigRow label="Mode" value={<ModeIndicator mode={pipeline.mode} />} />
            <ConfigRow label="App Type" value={pipeline.appType} />
            <ConfigRow label="Template" value={template?.name ?? pipeline.templateId} />
            <ConfigRow
              label="Git Repository"
              value={
                pipeline.gitRepoUrl ? (
                  <code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.gitRepoUrl}</code>
                ) : (
                  '-'
                )
              }
            />
            <ConfigRow
              label="Status"
              value={
                <span
                  className="rounded-md px-[9px] py-[3px] text-xs font-semibold"
                  style={{
                    backgroundColor: getPipelineStatusStyle(pipeline.status).bg,
                    color: getPipelineStatusStyle(pipeline.status).color,
                  }}
                >
                  {getPipelineStatusLabel(t, pipeline.status)}
                </span>
              }
            />
          </div>
        </DetailCard>

        {hasBuildConfig && (
          <DetailCard title="Build Configuration">
            <div className="flex flex-col gap-2.5">
              <ConfigRow
                label="Dockerfile"
                value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.dockerfilePath}</code>}
              />
              <ConfigRow
                label="Build Context"
                value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.dockerContext || '.'}</code>}
              />
            </div>
          </DetailCard>
        )}

        <DetailCard title="Deployment Target">
          <div className="flex flex-col gap-2.5">
            <ConfigRow label="Cluster" value={pipeline.clusterName} />
            <ConfigRow
              label="Namespace"
              value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.namespace}</code>}
            />
            <ConfigRow label="Created" value={formatDateTime(pipeline.createdAt, locale)} />
            <ConfigRow label="Last Deployed" value={formatDateTime(pipeline.lastDeployedAt, locale)} />
          </div>

          <div className="mt-4 rounded-lg border border-[var(--color-border-default)] bg-[rgba(15,23,42,0.45)] p-3">
            <div className="mb-2 text-[12px] font-semibold text-[var(--color-text-primary)]">
              Deployed Resources (Latest)
            </div>

            {(isDeploymentsLoading || isResourcesLoading) && (
              <div className="text-[12px] text-[var(--color-text-secondary)]">Loading resource topology...</div>
            )}

            {!isDeploymentsLoading && !isResourcesLoading && deployments.length === 0 && (
              <div className="text-[12px] text-[var(--color-text-secondary)]">
                No deployment history yet.
              </div>
            )}

            {!isDeploymentsLoading && !isResourcesLoading && deployments.length > 0 && (
              <div className="space-y-2">
                <div className="text-[11px] text-[var(--color-text-secondary)]">
                  Deployment {latestDeployment?.version ? <strong className="text-[var(--color-text-primary)]">{latestDeployment.version}</strong> : '-'}
                </div>

                <div className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)] md:items-stretch">
                  <ResourceNode
                    title="Ingress"
                    resources={ingressResources}
                    accentClass="bg-[rgba(34,197,94,0.12)] text-[#86efac]"
                  />
                  <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">→</div>
                  <ResourceNode
                    title="Service"
                    resources={serviceResources}
                    accentClass="bg-[rgba(59,130,246,0.12)] text-[#93c5fd]"
                  />
                  <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">→</div>
                  <ResourceNode
                    title="Deployment / StatefulSet"
                    resources={workloadResources}
                    accentClass="bg-[rgba(129,140,248,0.12)] text-[#c7d2fe]"
                  />
                  <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">→</div>
                  <ResourceNode
                    title="Pod"
                    resources={podResources}
                    accentClass="bg-[rgba(251,191,36,0.14)] text-[#fde68a]"
                    emptyLabel={workloadResources.length > 0 ? '(managed by workload)' : '-'}
                  />
                </div>

                <div className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                  <ResourceNode
                    title="Job / CronJob"
                    resources={jobResources}
                    accentClass="bg-[rgba(14,165,233,0.16)] text-[#7dd3fc]"
                    emptyLabel="No batch resources"
                  />
                  <div className="min-w-0 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2.5">
                    <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
                      Namespace / Access
                    </div>
                    <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-[var(--color-text-secondary)]">
                      <span>Namespace:</span>
                      <code className="rounded bg-[rgba(255,255,255,0.08)] px-1.5 py-[2px] text-[11px]">
                        {pipeline.namespace}
                      </code>
                    </div>
                    {serviceResources.flatMap((item) => item.serviceUrls ?? []).length > 0 && (
                      <div className="mt-1.5 flex min-w-0 flex-wrap gap-1">
                        {Array.from(new Set(serviceResources.flatMap((item) => item.serviceUrls ?? []))).slice(0, 4).map((url) => (
                          <code key={url} className="max-w-full break-all rounded bg-[rgba(99,102,241,0.14)] px-1.5 py-[1px] text-[10px] text-[#c7d2fe]">
                            {url}
                          </code>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {!hasDeploymentResources && (
                  <div className="text-[11px] text-[var(--color-text-muted)]">
                    Resource logs are not available for this deployment yet.
                  </div>
                )}
              </div>
            )}
          </div>
        </DetailCard>
      </div>

      {envEntries.length > 0 && (
        <DetailCard title="Environment Variables">
          <div className="flex flex-col gap-2">
            {envEntries.map(([key, value]) => {
              const isRevealed = revealedVars.has(key)
              return (
                <div
                  key={key}
                  className="grid grid-cols-[1fr_1fr_88px] items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-[12px]"
                >
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
    </div>
  )
}

function PipelineMonitoringTab({ pipeline }: { pipeline: Pipeline }) {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const { data: deploymentsData, isLoading } = usePipelineDeployments(pipeline.id)
  const deployments = deploymentsData?.items ?? []

  if (isLoading) {
    return <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">Loading deployment metrics...</div>
  }

  const total = deployments.length
  const successCount = deployments.filter((d) => d.status === 'success').length
  const failedCount = deployments.filter((d) => d.status === 'failed').length
  const successRate = total > 0 ? Math.round((successCount / total) * 100) : 0

  const deploymentsWithDuration = deployments.filter((d) => d.startedAt && d.completedAt)
  const avgDurationMs =
    deploymentsWithDuration.reduce((acc, d) => {
      const start = new Date(d.startedAt).getTime()
      const end = new Date(d.completedAt as string).getTime()
      return acc + (end - start)
    }, 0) / Math.max(deploymentsWithDuration.length, 1)

  const avgDuration =
    avgDurationMs > 60000
      ? `${Math.round(avgDurationMs / 60000)}m ${Math.round((avgDurationMs % 60000) / 1000)}s`
      : `${Math.round(avgDurationMs / 1000)}s`

  const trendMap = new Map<string, { success: number; failed: number }>()
  for (const d of deployments) {
    const date = formatDate(d.startedAt, locale, { month: 'numeric', day: 'numeric' })
    const entry = trendMap.get(date) ?? { success: 0, failed: 0 }
    if (d.status === 'success') {
      entry.success += 1
    } else if (d.status === 'failed') {
      entry.failed += 1
    }
    trendMap.set(date, entry)
  }

  const buildTrend = [...trendMap.entries()].slice(-7).map(([date, counts]) => ({ date, ...counts }))

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {[
          { label: 'Success Rate', value: total > 0 ? `${successRate}%` : '-', color: '#10b981' },
          { label: 'Total Deployments', value: String(total), color: '#818cf8' },
          { label: 'Avg Duration', value: total > 0 ? avgDuration : '-', color: '#f59e0b' },
          { label: 'Failed', value: String(failedCount), color: '#ef4444' },
        ].map((item) => (
          <div key={item.label} className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4 text-center">
            <div className="text-[28px] font-extrabold leading-none" style={{ color: item.color }}>
              {item.value}
            </div>
            <div className="mt-1 text-[12px] text-[var(--color-text-secondary)]">{item.label}</div>
          </div>
        ))}
      </div>

      {buildTrend.length > 0 && (
        <div className="rounded-lg border border-[var(--color-border-default)] bg-[#0b1220] p-4">
          <h4 className="m-0 mb-3 text-[14px] font-bold text-[#f8fafc]">Deployment Trend</h4>
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={buildTrend}>
              <CartesianGrid stroke="rgba(148,163,184,0.2)" strokeDasharray="3 3" />
              <XAxis dataKey="date" stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
              <YAxis stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
              <Tooltip contentStyle={{ background: '#111827', border: '1px solid #374151', color: '#e5e7eb' }} />
              <Bar dataKey="success" fill="#6366f1" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {buildTrend.length === 0 && (
        <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
          {t('cicdListPage.emptyDeployments', 'No deployment history.')}
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
  onRun,
  onOpenLogs,
  onDelete,
  isDeleting,
}: {
  pipeline: Pipeline
  onRun: () => void
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
          <Button variant="primary" size="sm" type="button" onClick={onRun}>
            <Rocket size={12} />
            Run
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
                  onRun={() => navigate(`/cicd/developer-deploy?pipelineId=${expandedPipeline.id}&clusterId=${expandedPipeline.clusterId}&namespace=${expandedPipeline.namespace}&appName=${expandedPipeline.name}`)}
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
          onRun={() => navigate(`/cicd/developer-deploy?pipelineId=${expandedPipeline.id}&clusterId=${expandedPipeline.clusterId}&namespace=${expandedPipeline.namespace}&appName=${expandedPipeline.name}`)}
          onOpenLogs={() => navigate(`/cicd/pipelines/${expandedPipeline.id}/logs`)}
        />
      )}
    </div>
  )
}

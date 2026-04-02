import { Fragment, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  BarChart2,
  ChevronDown,
  ChevronUp,
  Eye,
  EyeOff,
  GitBranch,
  History,
  Info,
  List,
  Play,
  Plus,
  Rocket,
  Search,
  Terminal,
} from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { usePipelineDeployments, usePipelines, useTemplateById } from '../api/cicd-api'
import type { Pipeline } from '../api/cicd-api'
import { useScopedClusters as useClusters } from '../../admin/api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { formatDate, formatDateTime, resolveLocale } from '../../../lib/locale'
import { getPipelineStatusLabel, getPipelineStatusStyle } from '../utils/pipeline-status'


type PipelineInnerTab = 'info' | 'monitoring' | 'history' | 'actions'

const INNER_TABS: Array<{ key: PipelineInnerTab; label: string; icon: React.ReactNode }> = [
  { key: 'info', label: 'Info', icon: <Info size={13} /> },
  { key: 'monitoring', label: 'Monitoring', icon: <BarChart2 size={13} /> },
  { key: 'history', label: 'History', icon: <History size={13} /> },
  { key: 'actions', label: 'Actions', icon: <Play size={13} /> },
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

function PipelineInfoTab({ pipeline }: { pipeline: Pipeline }) {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const { data: template } = useTemplateById(pipeline.templateId)
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

  const stages = (template?.stages ?? []) as string[]
  const hasBuildConfig = !!pipeline.dockerfilePath
  const envEntries = Object.entries(pipeline.envVars ?? {})

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <DetailCard title="Pipeline Info">
          <div className="flex flex-col gap-2.5">
            <ConfigRow label="Name" value={pipeline.name} />
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
            <ConfigRow label="Status" value={getPipelineStatusLabel(t, pipeline.status)} />
          </div>
        </DetailCard>

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
        </DetailCard>
      </div>

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

      {stages.length > 0 && (
        <DetailCard title="Pipeline Stages">
          <div className="flex flex-wrap items-center gap-2">
            {stages.map((stage: string, i: number) => (
              <Fragment key={stage}>
                <div className="flex items-center gap-1.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(99,102,241,0.1)] px-3 py-1.5">
                  <span className="flex h-5 w-5 items-center justify-center rounded-full bg-[#6366f1] text-[10px] font-bold text-white">{i + 1}</span>
                  <span className="text-[12px] font-semibold text-[var(--color-text-primary)]">{stage}</span>
                </div>
                {i < stages.length - 1 && <span className="text-[var(--color-text-muted)]">→</span>}
              </Fragment>
            ))}
          </div>
        </DetailCard>
      )}

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
  const { data: deploymentsData, isLoading } = usePipelineDeployments(pipeline.id)
  const deployments = deploymentsData?.items ?? []

  if (isLoading) {
    return <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">Loading deployment history...</div>
  }

  if (deployments.length === 0) {
    return <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">{t('cicdListPage.emptyDeployments', 'No deployment history.')}</div>
  }

  return (
    <div className="flex flex-col gap-2.5">
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

        return (
          <div key={d.id} className="flex flex-wrap items-center gap-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3.5 py-3">
            <span className="rounded-full px-2.5 py-0.5 text-[11px] font-bold" style={{ background: st.bg, color: st.color }}>
              {getPipelineStatusLabel(t, d.status)}
            </span>
            <span className="text-[13px] font-semibold text-[#a5b4fc]">{d.version}</span>
            <span className="flex-1 text-[12px] text-[var(--color-text-secondary)]">{d.triggeredBy || '-'}</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">{duration}</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">{formatDateTime(d.startedAt, locale)}</span>
          </div>
        )
      })}
    </div>
  )
}

function PipelineActionsTab({
  pipeline,
  onRun,
  onOpenLogs,
}: {
  pipeline: Pipeline
  onRun: () => void
  onOpenLogs: () => void
}) {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  return (
    <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
      <DetailCard title="Actions">
        <div className="flex flex-col gap-2.5">
          <button
            type="button"
            onClick={onRun}
            className="flex items-center justify-between rounded-lg border border-[rgba(99,102,241,0.4)] bg-[rgba(99,102,241,0.12)] px-3 py-2.5 text-left"
          >
            <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">Run Pipeline</span>
            <span className="text-[12px] text-[#a5b4fc]">Deploy latest commit</span>
          </button>
          <button
            type="button"
            onClick={onOpenLogs}
            className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-left"
          >
            <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">View Deployment History</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">Open CI/CD history for this pipeline</span>
          </button>
        </div>
      </DetailCard>

      <DetailCard title="Pipeline Scope">
        <div className="space-y-2 text-[13px] text-[var(--color-text-secondary)]">
          {[
            { label: 'Pipeline', value: pipeline.name },
            { label: 'Cluster', value: pipeline.clusterName },
            { label: 'Namespace', value: pipeline.namespace },
            { label: 'Status', value: getPipelineStatusLabel(t, pipeline.status) },
            { label: 'Last Deployed', value: formatDateTime(pipeline.lastDeployedAt, locale) },
          ].map((item) => (
            <div key={item.label} className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.03em] text-[var(--color-text-muted)]">{item.label}</div>
              <div className="font-semibold text-[var(--color-text-primary)]">{item.value}</div>
            </div>
          ))}
        </div>
      </DetailCard>
    </div>
  )
}

function PipelineDetailPanel({
  pipeline,
  onRun,
  onOpenLogs,
}: {
  pipeline: Pipeline
  onRun: () => void
  onOpenLogs: () => void
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
        {innerTab === 'actions' && <PipelineActionsTab pipeline={pipeline} onRun={onRun} onOpenLogs={onOpenLogs} />}
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

  const { data: clustersData } = useClusters()
  const { data: apiData } = usePipelines({ status: statusFilter || undefined, search: search || undefined })
  const pipelines = apiData?.items ?? []

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase()
    const matchesSearch = !search || p.name.toLowerCase().includes(q) || p.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || p.status === statusFilter
    const matchesCluster = !clusterFilter || p.clusterId === clusterFilter
    return matchesSearch && matchesStatus && matchesCluster
  })

  const columns: ColumnDef<Pipeline, unknown>[] = [
    {
      id: 'expand',
      header: '',
      enableSorting: false,
      cell: ({ row }) => {
        const isExpanded = expandedPipelineId === row.original.id
        return (
          <Button
            variant={isExpanded ? 'secondary' : 'ghost'}
            size="sm"
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              setExpandedPipelineId((prev) => (prev === row.original.id ? null : row.original.id))
            }}
          >
            {isExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </Button>
        )
      },
    },
    {
      accessorKey: 'name',
      header: t('cicdListPage.table.name', 'Name'),
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
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

      <DataTable
        columns={columns}
        data={filtered}
        getRowKey={(row) => row.id}
        expandedRowId={expandedPipelineId}
        renderExpanded={(pipeline) => (
            <PipelineDetailPanel
              key={pipeline.id}
              pipeline={pipeline}
            onRun={() => navigate(`/cicd/developer-deploy?pipelineId=${pipeline.id}&clusterId=${pipeline.clusterId}&namespace=${pipeline.namespace}&appName=${pipeline.name}`)}
            onOpenLogs={() => navigate(`/cicd/pipelines/${pipeline.id}/logs`)}
          />
        )}
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
    </div>
  )
}

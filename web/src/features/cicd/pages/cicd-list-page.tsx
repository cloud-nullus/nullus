import { Fragment, useState } from 'react'
import { useNavigate } from 'react-router-dom'
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
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'

const STATUS_STYLES: Record<string, { bg: string; color: string; label: string }> = {
  active: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Active' },
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa', label: 'Running' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Success' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Failed' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Cancelled' },
}

function formatDate(iso: string | null) {
  if (!iso) return '-'
  return new Date(iso).toLocaleDateString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}


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
  const pipelineVariables = [
    { key: 'TEMPLATE_ID', value: pipeline.templateId || '-', masked: false },
    { key: 'NAMESPACE', value: pipeline.namespace || 'default', masked: false },
    { key: 'GIT_REPOSITORY', value: pipeline.gitRepoUrl || '-', masked: true },
  ]

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
            <ConfigRow label="Status" value={(STATUS_STYLES[pipeline.status] ?? STATUS_STYLES.pending).label} />
          </div>
        </DetailCard>

        <DetailCard title="Deployment Target">
          <div className="flex flex-col gap-2.5">
            <ConfigRow label="Cluster" value={pipeline.clusterName} />
            <ConfigRow
              label="Namespace"
              value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">{pipeline.namespace}</code>}
            />
            <ConfigRow label="Created" value={formatDate(pipeline.createdAt)} />
            <ConfigRow label="Last Deployed" value={formatDate(pipeline.lastDeployedAt)} />
          </div>
        </DetailCard>
      </div>

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

      <DetailCard title="Pipeline Variables">
        <div className="flex flex-col gap-2">
          {pipelineVariables.map((variable) => {
            const isRevealed = revealedVars.has(variable.key)
            const displayValue = variable.masked && !isRevealed ? '********' : variable.value

            return (
              <div
                key={variable.key}
                className="grid grid-cols-[1fr_1fr_88px] items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-[12px]"
              >
                <span className="font-mono text-[var(--color-text-primary)]">{variable.key}</span>
                <span className="font-mono text-[var(--color-text-secondary)]">{displayValue}</span>
                {variable.masked ? (
                  <button
                    type="button"
                    onClick={() => toggleReveal(variable.key)}
                    className="inline-flex items-center justify-center gap-1 rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-[2px] text-[11px] text-[var(--color-text-secondary)]"
                  >
                    {isRevealed ? <EyeOff size={11} /> : <Eye size={11} />}
                    {isRevealed ? 'Hide' : 'Show'}
                  </button>
                ) : (
                  <span className="rounded px-2 py-[2px] text-center text-[11px] bg-[rgba(148,163,184,0.2)] text-[var(--color-text-secondary)]">plain</span>
                )}
              </div>
            )
          })}
        </div>
      </DetailCard>
    </div>
  )
}

function PipelineMonitoringTab({ pipeline }: { pipeline: Pipeline }) {
  const { data: deploymentsData } = usePipelineDeployments(pipeline.id)
  const deployments = deploymentsData?.items ?? []

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
    const date = new Date(d.startedAt).toLocaleDateString('ko-KR', { month: 'numeric', day: 'numeric' })
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
          배포 이력이 없습니다
        </div>
      )}
    </div>
  )
}

function PipelineHistoryTab({ pipeline }: { pipeline: Pipeline }) {
  const { data: deploymentsData } = usePipelineDeployments(pipeline.id)
  const deployments = deploymentsData?.items ?? []

  if (deployments.length === 0) {
    return <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">배포 이력이 없습니다</div>
  }

  return (
    <div className="flex flex-col gap-2.5">
      {deployments.map((d) => {
        const st = STATUS_STYLES[d.status] ?? STATUS_STYLES.pending
        const duration =
          d.completedAt && d.startedAt
            ? `${Math.round((new Date(d.completedAt).getTime() - new Date(d.startedAt).getTime()) / 1000)}s`
            : d.status === 'running'
              ? 'running'
              : '-'

        return (
          <div key={d.id} className="flex flex-wrap items-center gap-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3.5 py-3">
            <span className="rounded-full px-2.5 py-0.5 text-[11px] font-bold" style={{ background: st.bg, color: st.color }}>
              {st.label}
            </span>
            <span className="text-[13px] font-semibold text-[#a5b4fc]">{d.version}</span>
            <span className="flex-1 text-[12px] text-[var(--color-text-secondary)]">{d.triggeredBy || '-'}</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">{duration}</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">{formatDate(d.startedAt)}</span>
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
            { label: 'Status', value: (STATUS_STYLES[pipeline.status] ?? STATUS_STYLES.pending).label },
            { label: 'Last Deployed', value: formatDate(pipeline.lastDeployedAt) },
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
  const [innerTab, setInnerTab] = useState<PipelineInnerTab>('info')
  const statusStyle = STATUS_STYLES[pipeline.status] ?? STATUS_STYLES.pending

  return (
    <div className="mt-2.5 overflow-hidden rounded-[var(--card-radius)] border border-[rgba(99,102,241,0.3)] bg-[var(--color-surface-card)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-5 py-3.5">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
            <GitBranch size={16} />
          </div>
          <h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">{pipeline.name}</h3>
          <span className="rounded-[10px] px-[9px] py-[3px] text-[11px] font-bold" style={{ background: statusStyle.bg, color: statusStyle.color }}>
            {statusStyle.label}
          </span>
          <span className="text-[12px] text-[var(--color-text-secondary)]">
            · {pipeline.appType} · {pipeline.clusterName} · {formatDate(pipeline.lastDeployedAt)}
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
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [expandedPipelineId, setExpandedPipelineId] = useState<string | null>(null)

  const { data: apiData } = usePipelines({ status: statusFilter || undefined, search: search || undefined })
  const pipelines = apiData?.items ?? []

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase()
    const matchesSearch = !search || p.name.toLowerCase().includes(q) || p.clusterName.toLowerCase().includes(q)
    const matchesStatus = !statusFilter || p.status === statusFilter
    return matchesSearch && matchesStatus
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
      header: '이름',
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
    },
    {
      accessorKey: 'appType',
      header: '앱 타입',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.appType}</span>,
    },
    {
      accessorKey: 'clusterName',
      header: '클러스터',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.clusterName}</span>,
    },
    {
      accessorKey: 'status',
      header: '상태',
      cell: ({ row }) => {
        const st = STATUS_STYLES[row.original.status] ?? STATUS_STYLES.pending
        return (
          <span className="rounded-md px-[9px] py-[3px] text-xs font-semibold" style={{ backgroundColor: st.bg, color: st.color }}>
            {st.label}
          </span>
        )
      },
    },
    {
      accessorKey: 'lastDeployedAt',
      header: '최근 배포',
      cell: ({ row }) => <span className="text-[13px] text-[var(--color-text-secondary)]">{formatDate(row.original.lastDeployedAt)}</span>,
    },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'CI/CD List' }]} />

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
              CI/CD List
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              CI/CD 파이프라인 목록
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
          New Pipeline
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
            onRun={() => navigate(`/cicd/developer-deploy?pipeline=${pipeline.id}`)}
            onOpenLogs={() => navigate(`/cicd/history?pipeline=${pipeline.id}`)}
          />
        )}
        emptyMessage="파이프라인이 없습니다."
        toolbar={
          <>
            <NativeSelect value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]">
              <option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Status</option>
              <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Success</option>
              <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Running</option>
              <option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Pending</option>
              <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Failed</option>
              <option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Cancelled</option>
            </NativeSelect>
            <div className="relative ml-auto">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder="파이프라인 검색..."
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

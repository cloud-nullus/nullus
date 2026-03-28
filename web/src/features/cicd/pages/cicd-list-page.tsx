import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  BarChart2,
  ChevronDown,
  ChevronUp,
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
import { usePipelines } from '../api/cicd-api'
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

type PipelineHistoryItem = {
  id: string
  stage: 'ci' | 'cd'
  status: 'success' | 'failed' | 'running'
  title: string
  detail: string
  duration: string
  timestamp: string
}

const INNER_TABS: Array<{ key: PipelineInnerTab; label: string; icon: React.ReactNode }> = [
  { key: 'info', label: 'Info', icon: <Info size={13} /> },
  { key: 'monitoring', label: 'Monitoring', icon: <BarChart2 size={13} /> },
  { key: 'history', label: 'History', icon: <History size={13} /> },
  { key: 'actions', label: 'Actions', icon: <Play size={13} /> },
]

const STAGE_FLOW = [
  { key: 'build', label: 'Build', detail: '1m 12s', done: true },
  { key: 'test', label: 'Test', detail: '45s', done: true },
  { key: 'security', label: 'Security', detail: '2m 10s', done: true },
  { key: 'package', label: 'Package', detail: '37s', done: true },
  { key: 'deploy', label: 'Deploy', detail: '45s', done: true },
]

const PIPELINE_VARIABLES = [
  { key: 'DOCKER_DRIVER', value: 'overlay2', type: 'plaintext' },
  { key: 'NODE_VERSION', value: '18', type: 'plaintext' },
  { key: 'REGISTRY_TOKEN', value: '********', type: 'masked' },
]

const BUILD_TREND = [
  { date: '2/25', success: 12, failed: 1 },
  { date: '2/26', success: 15, failed: 2 },
  { date: '2/27', success: 11, failed: 3 },
  { date: '2/28', success: 16, failed: 1 },
  { date: '3/01', success: 13, failed: 2 },
  { date: '3/02', success: 17, failed: 1 },
  { date: '3/03', success: 19, failed: 0 },
]

const PIPELINE_HISTORY: PipelineHistoryItem[] = [
  {
    id: 'ci-145',
    stage: 'ci',
    status: 'success',
    title: '#145',
    detail: 'feat: add dark mode toggle',
    duration: '2m 34s',
    timestamp: '2026-03-03 14:22',
  },
  {
    id: 'cd-311',
    stage: 'cd',
    status: 'success',
    title: 'prod-k8s / production',
    detail: 'registry/frontend-web:v1.2.3',
    duration: '45s',
    timestamp: '2026-03-03 14:28',
  },
  {
    id: 'ci-143',
    stage: 'ci',
    status: 'failed',
    title: '#143',
    detail: 'refactor: update API client',
    duration: '1m 05s',
    timestamp: '2026-03-01 16:30',
  },
  {
    id: 'cd-309',
    stage: 'cd',
    status: 'running',
    title: 'prod-k8s / production',
    detail: 'registry/frontend-web:v1.2.4-rc1',
    duration: 'running',
    timestamp: '2026-03-03 15:10',
  },
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
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <DetailCard title="CI Configuration">
          <div className="flex flex-col gap-2.5">
            <ConfigRow label="Platform" value="GitLab CI/CD" />
            <ConfigRow label="Branch" value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">main</code>} />
            <ConfigRow label="Config File" value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">.gitlab-ci.yml</code>} />
            <ConfigRow label="Runner" value="k8s-runner-01" />
            <ConfigRow label="Trigger" value="Push / MR" />
          </div>
        </DetailCard>

        <DetailCard title="CD Configuration">
          <div className="flex flex-col gap-2.5">
            <ConfigRow label="Platform" value="Argo CD" />
            <ConfigRow label="Cluster" value={pipeline.clusterName} />
            <ConfigRow label="Namespace" value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[12px]">production</code>} />
            <ConfigRow label="Sync Policy" value="Auto Sync" />
            <ConfigRow label="Image" value={<code className="rounded bg-[rgba(255,255,255,0.08)] px-2 py-[2px] text-[11px]">registry/{pipeline.name}:v1.2.3</code>} />
          </div>
        </DetailCard>
      </div>

      <DetailCard title="Pipeline Stages">
        <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
          {STAGE_FLOW.map((stage) => (
            <div key={stage.key} className="flex flex-col items-center gap-1.5">
              <div className={`flex h-9 w-9 items-center justify-center rounded-full text-[13px] font-semibold ${stage.done ? 'bg-[#6366f1] text-white' : 'bg-[rgba(100,116,139,0.2)] text-[var(--color-text-secondary)]'}`}>
                {stage.key.slice(0, 1).toUpperCase()}
              </div>
              <div className="text-[11px] font-semibold text-[var(--color-text-primary)]">{stage.label}</div>
              <div className="text-[10px] text-[#6ee7b7]">{stage.done ? `✓ ${stage.detail}` : 'Pending'}</div>
            </div>
          ))}
        </div>
      </DetailCard>

      <DetailCard title="Pipeline Variables">
        <div className="flex flex-col gap-2">
          {PIPELINE_VARIABLES.map((variable) => (
            <div key={variable.key} className="grid grid-cols-[1fr_1fr_88px] items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-[12px]">
              <span className="font-mono text-[var(--color-text-primary)]">{variable.key}</span>
              <span className="font-mono text-[var(--color-text-secondary)]">{variable.value}</span>
              <span className={`rounded px-2 py-[2px] text-center text-[11px] ${variable.type === 'masked' ? 'bg-[rgba(245,158,11,0.2)] text-[#f59e0b]' : 'bg-[rgba(148,163,184,0.2)] text-[var(--color-text-secondary)]'}`}>
                {variable.type}
              </span>
            </div>
          ))}
        </div>
      </DetailCard>
    </div>
  )
}

function PipelineMonitoringTab() {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {[
          { label: 'Success Rate', value: '97.3%', color: '#10b981' },
          { label: 'Total Builds', value: '145', color: '#818cf8' },
          { label: 'Avg Duration', value: '2m 34s', color: '#f59e0b' },
          { label: 'Pods Running', value: '3/3', color: '#22c55e' },
        ].map((item) => (
          <div key={item.label} className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4 text-center">
            <div className="text-[28px] font-extrabold leading-none" style={{ color: item.color }}>
              {item.value}
            </div>
            <div className="mt-1 text-[12px] text-[var(--color-text-secondary)]">{item.label}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[2fr_1fr]">
        <div className="rounded-lg border border-[var(--color-border-default)] bg-[#0b1220] p-4">
          <h4 className="m-0 mb-3 text-[14px] font-bold text-[#f8fafc]">Build Trend (Last 7 days)</h4>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={BUILD_TREND}>
              <CartesianGrid stroke="rgba(148,163,184,0.2)" strokeDasharray="3 3" />
              <XAxis dataKey="date" stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
              <YAxis stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
              <Tooltip contentStyle={{ background: '#111827', border: '1px solid #374151', color: '#e5e7eb' }} />
              <Bar dataKey="success" fill="#6366f1" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        <div className="rounded-lg border border-[var(--color-border-default)] bg-[#0b1220] p-4">
          <h4 className="m-0 mb-3 text-[14px] font-bold text-[#f8fafc]">Application Health</h4>
          <div className="flex flex-col gap-3">
            {[
              { label: 'CPU', value: '0.3 cores', rate: 15, color: '#6366f1' },
              { label: 'Memory', value: '256 Mi', rate: 25, color: '#10b981' },
            ].map((item) => (
              <div key={item.label}>
                <div className="mb-1 flex items-center justify-between text-[12px]">
                  <span className="text-[#94a3b8]">{item.label}</span>
                  <span style={{ color: item.color }} className="font-semibold">{item.value}</span>
                </div>
                <div className="h-1.5 rounded-full bg-[#1e293b]">
                  <div className="h-1.5 rounded-full" style={{ width: `${item.rate}%`, backgroundColor: item.color }} />
                </div>
              </div>
            ))}
            <div className="rounded-md bg-[#1e293b] p-2.5">
              <div className="text-[11px] text-[#64748b]">ArgoCD Sync Status</div>
              <div className="text-[13px] font-semibold text-[#6ee7b7]">Synced</div>
            </div>
            <div className="rounded-md bg-[#1e293b] p-2.5">
              <div className="text-[11px] text-[#64748b]">Last Deployment</div>
              <div className="text-[13px] text-[#e2e8f0]">2026-03-03 14:28</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function PipelineHistoryTab() {
  const [typeFilter, setTypeFilter] = useState<'all' | 'ci' | 'cd'>('all')
  const [statusFilter, setStatusFilter] = useState<'all' | 'success' | 'failed' | 'running'>('all')

  const historyItems = useMemo(
    () =>
      PIPELINE_HISTORY.filter((item) => {
        const matchesType = typeFilter === 'all' || item.stage === typeFilter
        const matchesStatus = statusFilter === 'all' || item.status === statusFilter
        return matchesType && matchesStatus
      }),
    [statusFilter, typeFilter],
  )

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <NativeSelect
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value as 'all' | 'ci' | 'cd')}
          className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
        >
          <option value="all" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Types</option>
          <option value="ci" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">CI (Build)</option>
          <option value="cd" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">CD (Deploy)</option>
        </NativeSelect>
        <NativeSelect
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as 'all' | 'success' | 'failed' | 'running')}
          className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
        >
          <option value="all" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Status</option>
          <option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Success</option>
          <option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Failed</option>
          <option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Running</option>
        </NativeSelect>
      </div>

      <div className="flex flex-col gap-2.5">
        {historyItems.map((item) => {
          const statusStyle =
            item.status === 'success'
              ? 'bg-[rgba(16,185,129,0.15)] text-[#6ee7b7]'
              : item.status === 'failed'
                ? 'bg-[rgba(239,68,68,0.15)] text-[#fca5a5]'
                : 'bg-[rgba(59,130,246,0.15)] text-[#93c5fd]'
          const typeStyle =
            item.stage === 'ci'
              ? 'bg-[rgba(245,158,11,0.15)] text-[#fcd34d]'
              : 'bg-[rgba(16,185,129,0.15)] text-[#6ee7b7]'

          return (
            <div key={item.id} className="flex flex-wrap items-center gap-2.5 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3.5 py-3">
              <span className={`rounded-full px-2.5 py-0.5 text-[11px] font-bold uppercase ${statusStyle}`}>{item.status}</span>
              <span className={`rounded-md px-2 py-0.5 text-[11px] font-semibold uppercase ${typeStyle}`}>{item.stage}</span>
              <span className="text-[13px] font-semibold text-[#a5b4fc]">{item.title}</span>
              <code className="flex-1 rounded bg-[rgba(255,255,255,0.06)] px-2 py-[2px] text-[12px] text-[var(--color-text-primary)]">{item.detail}</code>
              <span className="text-[12px] text-[var(--color-text-secondary)]">{item.duration}</span>
              <span className="text-[12px] text-[var(--color-text-secondary)]">{item.timestamp}</span>
              <button type="button" className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2.5 py-1 text-[12px] text-[var(--color-text-primary)]">
                Logs
              </button>
            </div>
          )
        })}
      </div>
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
      <DetailCard title="Runbook Actions">
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
            <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">View Live Logs</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">Open CI/CD history logs</span>
          </button>
          <button
            type="button"
            className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-left"
          >
            <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">Rollback</span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">Rollback to previous image</span>
          </button>
          <button
            type="button"
            className="flex items-center justify-between rounded-lg border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2.5 text-left"
          >
            <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">Stop Pipeline</span>
            <span className="text-[12px] text-[#fca5a5]">Cancel running pipeline</span>
          </button>
        </div>
      </DetailCard>

      <DetailCard title="Action Scope">
        <div className="space-y-2 text-[13px] text-[var(--color-text-secondary)]">
          <div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
            <div className="text-[11px] uppercase tracking-[0.03em] text-[var(--color-text-muted)]">Pipeline</div>
            <div className="font-semibold text-[var(--color-text-primary)]">{pipeline.name}</div>
          </div>
          <div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
            <div className="text-[11px] uppercase tracking-[0.03em] text-[var(--color-text-muted)]">Cluster</div>
            <div className="font-semibold text-[var(--color-text-primary)]">{pipeline.clusterName}</div>
          </div>
          <div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
            <div className="text-[11px] uppercase tracking-[0.03em] text-[var(--color-text-muted)]">Current Status</div>
            <div className="font-semibold text-[var(--color-text-primary)]">{(STATUS_STYLES[pipeline.status] ?? STATUS_STYLES.pending).label}</div>
          </div>
          <div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
            <div className="text-[11px] uppercase tracking-[0.03em] text-[var(--color-text-muted)]">Last Deployed</div>
            <div className="font-semibold text-[var(--color-text-primary)]">{formatDate(pipeline.lastDeployedAt)}</div>
          </div>
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
        {innerTab === 'monitoring' && <PipelineMonitoringTab />}
        {innerTab === 'history' && <PipelineHistoryTab />}
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

  const expandedPipeline = filtered.find((pipeline) => pipeline.id === expandedPipelineId) ?? null

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

      {expandedPipeline && (
        <PipelineDetailPanel
          key={expandedPipeline.id}
          pipeline={expandedPipeline}
          onRun={() => navigate(`/cicd/developer-deploy?pipeline=${expandedPipeline.id}`)}
          onOpenLogs={() => navigate(`/cicd/history?pipeline=${expandedPipeline.id}`)}
        />
      )}

    </div>
  )
}

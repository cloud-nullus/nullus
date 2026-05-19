import { useMemo, useState } from 'react'
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import {
  Cpu, HardDrive, MemoryStick, Box, CheckCircle, AlertCircle, XCircle,
  Server, GitBranch, BarChart3, RefreshCw, Clock, Package, Layers,
} from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useDashboard, useDeployedApps } from '../api/observability-api'
import { useAuthStore } from '../../../stores/auth-store'
import { cn } from '../../../lib/utils'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { DashboardTabLayout, StackConnectPanel, TOOL_STATUS } from '../components/dashboard-tab-layout'
import type { EmbedTab } from '../components/dashboard-tab-layout'

// ─── Types ────────────────────────────────────────────────────────────────────
type ViewType = 'cluster' | 'stack' | 'cicd'
type TimeRange = '1h' | '6h' | '24h' | '7d'

// ─── Shared chart style helpers ───────────────────────────────────────────────
const CHART_STYLE = {
  bg: '#0b1220',
  grid: 'rgba(148,163,184,0.15)',
  tick: { fill: '#94a3b8', fontSize: 11 },
  tooltip: { background: '#111827', border: '1px solid #374151', color: '#e5e7eb' },
}

// ─── Time series generator ────────────────────────────────────────────────────
function makeSeries(range: TimeRange) {
  const cfg: Record<TimeRange, [number, number]> = {
    '1h': [12, 5], '6h': [12, 30], '24h': [24, 60], '7d': [14, 1440],
  }
  const [pts, stepMin] = cfg[range]
  const now = Date.now()
  return Array.from({ length: pts }, (_, i) => {
    const t = new Date(now - (pts - 1 - i) * stepMin * 60_000)
    const label = range === '7d'
      ? t.toLocaleDateString('en', { weekday: 'short' })
      : t.toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hour12: false })
    return {
      time: label,
      cpu: Math.max(12, Math.min(96, Math.round(56 + Math.sin(i / 2.5) * 16 + (i % 3) * 2))),
      memory: Math.max(24, Math.min(97, Math.round(63 + Math.cos(i / 3.2) * 10 + (i % 4) * 2))),
      success: Math.round(89 + Math.random() * 10),
    }
  })
}

const WEEK_BARS = [
  { day: 'Mon', success: 16, failed: 2 }, { day: 'Tue', success: 19, failed: 3 },
  { day: 'Wed', success: 15, failed: 4 }, { day: 'Thu', success: 21, failed: 2 },
  { day: 'Fri', success: 24, failed: 3 }, { day: 'Sat', success: 11, failed: 2 },
  { day: 'Sun', success: 9, failed: 1 },
]

// ─── Shared chart panel wrapper ───────────────────────────────────────────────
function ChartPanel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[10px] border border-[var(--color-border-default)] p-3" style={{ background: CHART_STYLE.bg }}>
      <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">{title}</div>
      {children}
    </div>
  )
}

// ─── KPI card ────────────────────────────────────────────────────────────────
function KpiCard({ label, value, icon, color, iconCls, bar }: { label: string; value: string; icon: React.ReactNode; color: string; iconCls: string; bar: number }) {
  return (
    <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
      <div className="mb-2.5 flex items-center gap-2.5">
        <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', iconCls)}>{icon}</div>
        <span className="text-xs font-medium text-[var(--color-text-secondary)]">{label}</span>
      </div>
      <div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">{value}</div>
      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[rgba(255,255,255,0.08)]">
        <svg className="h-full w-full" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
          <rect width={Math.max(0, Math.min(100, bar))} height="6" rx="3" fill={color} />
        </svg>
      </div>
    </div>
  )
}

// ─── Default content: Cluster view ───────────────────────────────────────────
function ClusterDefault({ clusterId }: { clusterId: string }) {
  const [range, setRange] = useState<TimeRange>('24h')
  const series = useMemo(() => makeSeries(range), [range])
  const pods = [
    { name: 'Running', value: 22, color: '#22c55e' },
    { name: 'Pending', value: 1, color: '#f59e0b' },
    { name: 'Failed', value: 1, color: '#ef4444' },
  ]
  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <div className={cn('flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium',
          'border-emerald-500/20 bg-emerald-500/5 text-emerald-400')}>
          <span className="h-2 w-2 rounded-full bg-emerald-400" />{clusterId}
        </div>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-[var(--color-text-secondary)]">Range:</span>
          {(['1h', '6h', '24h', '7d'] as TimeRange[]).map((r) => (
            <button key={r} type="button" onClick={() => setRange(r)}
              className={cn('rounded-[7px] border px-2.5 py-[5px] text-xs font-bold',
                range === r ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]')}>
              {r}
            </button>
          ))}
        </div>
      </div>
      <div className="mb-5 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
        <KpiCard label="Nodes" value="3/4" icon={<Server size={18} />} color="#60a5fa" iconCls="bg-[rgba(59,130,246,0.15)] text-[#60a5fa]" bar={75} />
        <KpiCard label="Pods" value="22/24" icon={<Box size={18} />} color="#22c55e" iconCls="bg-[rgba(34,197,94,0.15)] text-[#22c55e]" bar={92} />
        <KpiCard label="CPU" value="62%" icon={<Cpu size={18} />} color="#f59e0b" iconCls="bg-[rgba(245,158,11,0.15)] text-[#f59e0b]" bar={62} />
        <KpiCard label="Memory" value="71%" icon={<MemoryStick size={18} />} color="#a78bfa" iconCls="bg-[rgba(139,92,246,0.15)] text-[#a78bfa]" bar={71} />
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPanel title="CPU Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="ccpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.5} /><stop offset="95%" stopColor="#f59e0b" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis domain={[0, 100]} stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Area type="monotone" dataKey="cpu" name="CPU %" stroke="#f59e0b" strokeWidth={2} fill="url(#ccpu)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Memory Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="cmem" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={.5} /><stop offset="95%" stopColor="#3b82f6" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis domain={[0, 100]} stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Area type="monotone" dataKey="memory" name="Memory %" stroke="#3b82f6" strokeWidth={2} fill="url(#cmem)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pod Status">
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              <Pie data={pods} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                {pods.map((e) => <Cell key={e.name} fill={e.color} />)}
              </Pie>
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb' }} />
            </PieChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pipeline Success (this week)">
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={WEEK_BARS}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="day" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb' }} />
              <Bar dataKey="success" fill="#22c55e" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>
    </div>
  )
}

// ─── Default content: Stack view ─────────────────────────────────────────────
function StackDefault({ stackName }: { stackName: string }) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: apiData, isLoading, refetch } = useDashboard(5000)
  const series = useMemo(() => makeSeries(range), [range])

  const fallback = { kpi: { cpuUsage: 68, memoryUsage: 42, storageUsage: 31, podCount: 27, podRunning: 24 }, pipeline: { successRate: 97.3, totalRuns: 145, avgBuildSeconds: 154 }, tools: [{ name: 'GitLab', version: '16.7', status: 'running' as const }, { name: 'ArgoCD', version: '2.9.3', status: 'running' as const }, { name: 'Prometheus', version: '2.48.1', status: 'running' as const }, { name: 'Grafana', version: '10.3', status: 'warning' as const }, { name: 'Harbor', version: '2.8.2', status: 'running' as const }] }
  const dash = (!isLoading && apiData && 'kpi' in apiData) ? apiData : fallback
  const kpi = dash.kpi

  const kpis = [
    { label: 'CPU Usage', value: `${kpi.cpuUsage}%`, icon: <Cpu size={18} />, color: '#60a5fa', iconCls: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]', bar: kpi.cpuUsage },
    { label: 'Memory', value: `${kpi.memoryUsage}%`, icon: <MemoryStick size={18} />, color: '#a78bfa', iconCls: 'bg-[rgba(139,92,246,0.15)] text-[#a78bfa]', bar: kpi.memoryUsage },
    { label: 'Storage', value: `${kpi.storageUsage}%`, icon: <HardDrive size={18} />, color: '#34d399', iconCls: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]', bar: kpi.storageUsage },
    { label: 'Running Pods', value: `${kpi.podRunning}/${kpi.podCount}`, icon: <Box size={18} />, color: '#fbbf24', iconCls: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]', bar: kpi.podCount ? Math.round(kpi.podRunning / kpi.podCount * 100) : 0 },
  ]

  const podData = [
    { name: 'Running', value: kpi.podRunning, color: '#22c55e' },
    { name: 'Pending', value: Math.max(1, kpi.podCount - kpi.podRunning - 1), color: '#f59e0b' },
    { name: 'Failed', value: 1, color: '#ef4444' },
  ]

  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-1.5 text-xs text-[var(--color-text-secondary)]">
          Stack: <span className="font-semibold text-[var(--color-text-primary)]">{stackName}</span>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <button type="button" onClick={() => void refetch()}
            className="flex items-center gap-1 rounded-lg border border-[var(--color-border-default)] px-2.5 py-[5px] text-xs text-[var(--color-text-secondary)] hover:bg-[rgba(255,255,255,0.06)]">
            <RefreshCw size={11} className={cn(isLoading && 'animate-spin')} />Refresh
          </button>
          {(['1h', '6h', '24h', '7d'] as TimeRange[]).map((r) => (
            <button key={r} type="button" onClick={() => setRange(r)}
              className={cn('rounded-[7px] border px-2.5 py-[5px] text-xs font-bold',
                range === r ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]')}>
              {r}
            </button>
          ))}
        </div>
      </div>

      <div className="mb-5 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
        {kpis.map((c) => <KpiCard key={c.label} {...c} />)}
      </div>

      <div className="mb-5 grid grid-cols-2 gap-3.5">
        <ChartPanel title="CPU Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="scpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.5} /><stop offset="95%" stopColor="#f59e0b" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} /><YAxis domain={[0, 100]} stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Area type="monotone" dataKey="cpu" name="CPU %" stroke="#f59e0b" strokeWidth={2} fill="url(#scpu)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Memory Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="smem" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={.5} /><stop offset="95%" stopColor="#3b82f6" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} /><YAxis domain={[0, 100]} stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Area type="monotone" dataKey="memory" name="Memory %" stroke="#3b82f6" strokeWidth={2} fill="url(#smem)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pipeline Success Rate">
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={WEEK_BARS}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="day" stroke="#94a3b8" tick={CHART_STYLE.tick} /><YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} /><Legend wrapperStyle={{ color: '#e5e7eb' }} />
              <Bar dataKey="success" fill="#22c55e" radius={[4, 4, 0, 0]} /><Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Pod Status">
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              <Pie data={podData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                {podData.map((e) => <Cell key={e.name} fill={e.color} />)}
              </Pie>
              <Tooltip contentStyle={CHART_STYLE.tooltip} /><Legend wrapperStyle={{ color: '#e5e7eb' }} />
            </PieChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>

      <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
        <h2 className="mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">Tool Health</h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
          {dash.tools.map((t) => {
            const cfg = TOOL_STATUS[t.status]
            return (
              <div key={t.name} className="rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3.5">
                <div className="mb-1.5 flex items-center justify-between">
                  <span className="text-sm font-bold text-[var(--color-text-primary)]">{t.name}</span>
                  <span className={cn('inline-flex items-center gap-1 rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', cfg.cls)}>
                    {cfg.icon}{cfg.label}
                  </span>
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">v{t.version}</div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// ─── Deployed Stacks panel ───────────────────────────────────────────────────
function DeployedStacksPanel() {
  const { data, isLoading } = useDeployedApps()
  const apps = data?.items ?? []

  return (
    <div className="mb-5 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
      <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-4 py-3">
        <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
          <Package size={15} className="text-[#a5b4fc]" />
          Deployed Stacks
        </h2>
        <span className="text-xs text-[var(--color-text-secondary)]">
          {isLoading ? 'loading…' : `${apps.length} stacks`}
        </span>
      </div>
      <div className="overflow-x-auto">
        {apps.length === 0 ? (
          <div className="py-10 text-center text-sm text-[var(--color-text-secondary)]">
            배포된 스택이 없습니다. Stack Template에서 스택을 만들어 배포해보세요.
          </div>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[var(--color-border-default)] text-[11px] text-[var(--color-text-secondary)]">
                {['Stack', 'Template', 'Namespace', 'Cluster', 'Pods', 'Status', 'Deployed At'].map((h) => (
                  <th key={h} className="px-4 py-2.5 text-left font-semibold tracking-[0.03em]">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {apps.map((app, i) => {
                const isLast = i === apps.length - 1
                const statusColor = app.status === 'success'
                  ? 'bg-emerald-500/15 text-emerald-400'
                  : app.status === 'failed'
                    ? 'bg-red-500/15 text-red-400'
                    : 'bg-amber-500/15 text-amber-400'
                const dotColor = app.status === 'success'
                  ? 'bg-emerald-400'
                  : app.status === 'failed'
                    ? 'bg-red-400'
                    : 'bg-amber-400'
                return (
                  <tr key={app.id}
                    className={cn('transition-colors hover:bg-[rgba(255,255,255,0.02)]', !isLast && 'border-b border-[var(--color-border-default)]')}>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={cn('h-2 w-2 shrink-0 rounded-full', dotColor)} />
                        <span className="font-semibold text-[var(--color-text-primary)]">{app.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <GitBranch size={11} />{app.template_id || '-'}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="rounded-[4px] bg-[rgba(99,102,241,0.12)] px-1.5 py-0.5 text-[10px] font-bold text-[#a5b4fc]">
                        {app.namespace}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <Server size={11} className="text-[var(--color-text-muted)]" />
                        <span className="font-medium text-[var(--color-text-primary)]">
                          {app.cluster_name || app.cluster_id?.slice(0, 8)}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {(app.pods ?? []).length === 0 ? (
                          <span className="text-[10px] italic text-[var(--color-text-muted)]">no pods</span>
                        ) : (
                          (app.pods ?? []).map((pod) => (
                            <span
                              key={pod.name}
                              title={pod.node ? `node: ${pod.node} · ${pod.status}` : pod.status}
                              className="inline-flex items-center gap-0.5 rounded-[4px] bg-[rgba(34,197,94,0.1)] px-1.5 py-0.5 text-[10px] font-mono text-[#4ade80]"
                            >
                              <Box size={9} />{pod.name}
                            </span>
                          ))
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', statusColor)}
                        title={app.state ? `state: ${app.state}` : undefined}
                      >
                        {app.state || app.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <Clock size={11} />
                        {new Date(app.deployed_at).toLocaleString('ko-KR')}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

// ─── Default content: CI/CD view ─────────────────────────────────────────────
const REQ_SERIES = [
  { time: '00:00', frontend: 980, backend: 3200, auth: 620 },
  { time: '04:00', frontend: 420, backend: 1800, auth: 310 },
  { time: '08:00', frontend: 1100, backend: 3900, auth: 740 },
  { time: '12:00', frontend: 1560, backend: 4800, auth: 890 },
  { time: '16:00', frontend: 1380, backend: 4200, auth: 710 },
  { time: '20:00', frontend: 1240, backend: 3870, auth: 580 },
  { time: 'now', frontend: 1240, backend: 3870, auth: 580 },
]

const ERR_SERIES = [
  { time: '00:00', frontend: 0.0, backend: 0.0, auth: 0.8 },
  { time: '04:00', frontend: 0.1, backend: 0.0, auth: 1.2 },
  { time: '08:00', frontend: 0.0, backend: 0.1, auth: 2.0 },
  { time: '12:00', frontend: 0.2, backend: 0.0, auth: 2.8 },
  { time: '16:00', frontend: 0.1, backend: 0.1, auth: 3.0 },
  { time: '20:00', frontend: 0.1, backend: 0.0, auth: 3.2 },
  { time: 'now', frontend: 0.1, backend: 0.0, auth: 3.2 },
]

/** Sample Grafana tab pre-seeded into CI/CD localStorage */
export const CICD_DEFAULT_TABS: EmbedTab[] = [
  {
    id: 'cicd-seed-grafana',
    label: 'Grafana',
    url: 'https://play.grafana.org/d/000000012/grafana-play-home?orgId=1&theme=dark&kiosk',
    order: 0,
  },
]

function CicdDefault() {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: deployedAppsData } = useDeployedApps()
  const deployedApps = deployedAppsData?.items ?? []

  const healthy = deployedApps.filter((a) => a.status === 'success').length
  const failed = deployedApps.filter((a) => a.status === 'failed').length
  const running = deployedApps.filter((a) => a.status === 'running').length

  const appKpis = [
    { label: 'Total Apps', value: String(deployedApps.length), icon: <Layers size={18} />, color: '#6366f1', iconCls: 'bg-[rgba(99,102,241,0.15)] text-[#6366f1]', bar: 100 },
    { label: 'Success', value: String(healthy), icon: <CheckCircle size={18} />, color: '#22c55e', iconCls: 'bg-emerald-500/15 text-emerald-400', bar: deployedApps.length ? Math.round(healthy / deployedApps.length * 100) : 0 },
    { label: 'Failed / Running', value: `${failed} / ${running}`, icon: <AlertCircle size={18} />, color: '#f59e0b', iconCls: 'bg-amber-500/15 text-amber-400', bar: deployedApps.length ? Math.round((failed + running) / deployedApps.length * 100) : 0 },
    { label: 'Namespaces', value: String(new Set(deployedApps.map((a) => a.namespace)).size), icon: <Box size={18} />, color: '#10b981', iconCls: 'bg-[rgba(16,185,129,0.15)] text-[#10b981]', bar: 80 },
  ]

  return (
    <div>
      {/* Toolbar */}
      <div className="mb-5 flex flex-wrap items-center gap-3">
        <div className="ml-auto flex items-center gap-2">
          {(['1h', '6h', '24h', '7d'] as TimeRange[]).map((r) => (
            <button key={r} type="button" onClick={() => setRange(r)}
              className={cn('rounded-[7px] border px-2.5 py-[5px] text-xs font-bold',
                range === r
                  ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]')}>
              {r}
            </button>
          ))}
        </div>
      </div>

      {/* KPI cards */}
      <div className="mb-5 grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-4">
        {appKpis.map((c) => <KpiCard key={c.label} {...c} />)}
      </div>

      {/* Charts */}
      <div className="mb-5 grid grid-cols-2 gap-3.5">
        <ChartPanel title="Request Rate (req/min)">
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={REQ_SERIES}>
              <defs>
                <linearGradient id="rr-fe" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#6366f1" stopOpacity={.4} /><stop offset="95%" stopColor="#6366f1" stopOpacity={.02} /></linearGradient>
                <linearGradient id="rr-be" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#22c55e" stopOpacity={.4} /><stop offset="95%" stopColor="#22c55e" stopOpacity={.02} /></linearGradient>
                <linearGradient id="rr-au" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.4} /><stop offset="95%" stopColor="#f59e0b" stopOpacity={.02} /></linearGradient>
              </defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb', fontSize: 11 }} />
              <Area type="monotone" dataKey="frontend" name="app-frontend" stroke="#6366f1" strokeWidth={2} fill="url(#rr-fe)" />
              <Area type="monotone" dataKey="backend" name="app-backend" stroke="#22c55e" strokeWidth={2} fill="url(#rr-be)" />
              <Area type="monotone" dataKey="auth" name="auth-service" stroke="#f59e0b" strokeWidth={2} fill="url(#rr-au)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
        <ChartPanel title="Error Rate (%)">
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={ERR_SERIES}>
              <defs>
                <linearGradient id="er-fe" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#6366f1" stopOpacity={.3} /><stop offset="95%" stopColor="#6366f1" stopOpacity={.02} /></linearGradient>
                <linearGradient id="er-au" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#ef4444" stopOpacity={.4} /><stop offset="95%" stopColor="#ef4444" stopOpacity={.02} /></linearGradient>
              </defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb', fontSize: 11 }} />
              <Area type="monotone" dataKey="frontend" name="app-frontend" stroke="#6366f1" strokeWidth={2} fill="url(#er-fe)" />
              <Area type="monotone" dataKey="auth" name="auth-service" stroke="#ef4444" strokeWidth={2} fill="url(#er-au)" />
            </AreaChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>

      {/* Application table */}
      <div className="mb-5 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-4 py-3">
          <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
            <Package size={15} className="text-[#a5b4fc]" />
            Deployed Applications
          </h2>
          <span className="text-xs text-[var(--color-text-secondary)]">{deployedApps.length} apps</span>
        </div>
        <div className="overflow-x-auto">
          {deployedApps.length === 0 ? (
            <div className="py-10 text-center text-sm text-[var(--color-text-secondary)]">
              배포된 앱이 없습니다. CI/CD Pipeline Setup에서 앱을 배포해보세요.
            </div>
          ) : (
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-[var(--color-border-default)] text-[11px] text-[var(--color-text-secondary)]">
                  {['Application', 'Version', 'Template', 'Namespace', 'Cluster', 'Pods', 'Status', 'Deployed At'].map((h) => (
                    <th key={h} className="px-4 py-2.5 text-left font-semibold tracking-[0.03em]">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {deployedApps.map((app, i) => {
                  const isLast = i === deployedApps.length - 1
                  const statusColor = app.status === 'success'
                    ? 'bg-emerald-500/15 text-emerald-400'
                    : app.status === 'failed'
                      ? 'bg-red-500/15 text-red-400'
                      : 'bg-amber-500/15 text-amber-400'
                  const dotColor = app.status === 'success'
                    ? 'bg-emerald-400'
                    : app.status === 'failed'
                      ? 'bg-red-400'
                      : 'bg-amber-400'
                  return (
                    <tr key={app.id}
                      className={cn('transition-colors hover:bg-[rgba(255,255,255,0.02)]', !isLast && 'border-b border-[var(--color-border-default)]')}>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <span className={cn('h-2 w-2 shrink-0 rounded-full', dotColor)} />
                          <span className="font-semibold text-[var(--color-text-primary)]">{app.name}</span>
                        </div>
                      </td>
                      <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">{app.version}</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                          <GitBranch size={11} />{app.template_id}
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <span className="rounded-[4px] bg-[rgba(99,102,241,0.12)] px-1.5 py-0.5 text-[10px] font-bold text-[#a5b4fc]">
                          {app.namespace}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                          <Server size={11} className="text-[var(--color-text-muted)]" />
                          <span className="font-medium text-[var(--color-text-primary)]">{app.cluster_name || app.cluster_id?.slice(0, 8)}</span>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex flex-wrap gap-1">
                          {(app.pods ?? []).length === 0 ? (
                            <span className="text-[10px] italic text-[var(--color-text-muted)]">no pods</span>
                          ) : (
                            (app.pods ?? []).map((pod) => (
                              <span
                                key={pod.name}
                                title={pod.node ? `node: ${pod.node} · ${pod.status}` : pod.status}
                                className="inline-flex items-center gap-0.5 rounded-[4px] bg-[rgba(34,197,94,0.1)] px-1.5 py-0.5 text-[10px] font-mono text-[#4ade80]"
                              >
                                <Box size={9} />{pod.name}
                              </span>
                            ))
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', statusColor)}
                          title={app.state ? `state: ${app.state}` : undefined}
                        >
                          {app.state || app.status}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                          <Clock size={11} />
                          {new Date(app.deployed_at).toLocaleString('ko-KR')}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Recent deployments */}
      <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="border-b border-[var(--color-border-default)] px-4 py-3">
          <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
            <GitBranch size={15} className="text-[#a5b4fc]" />
            Recent Deployments
          </h2>
        </div>
        <div className="divide-y divide-[var(--color-border-default)]">
          {deployedApps.length === 0 ? (
            <div className="py-6 text-center text-sm text-[var(--color-text-secondary)]">
              최근 배포 이력이 없습니다.
            </div>
          ) : (
            deployedApps.slice(0, 5).map((app) => (
              <div key={app.id} className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-3">
                <div className="flex items-center gap-2">
                  {app.status === 'success'
                    ? <CheckCircle size={13} className="text-emerald-400" />
                    : <XCircle size={13} className="text-red-400" />}
                  <span className="font-semibold text-[var(--color-text-primary)]">{app.name}</span>
                </div>
                <span className="font-mono text-[11px] text-[var(--color-text-secondary)]">{app.version}</span>
                <span className="rounded-[4px] bg-[rgba(99,102,241,0.12)] px-1.5 py-0.5 text-[10px] font-bold text-[#a5b4fc]">
                  {app.namespace}
                </span>
                <div className="flex items-center gap-1 text-[11px] text-[var(--color-text-secondary)]">
                  <GitBranch size={10} />{app.template_id}
                </div>
                <span className="ml-auto text-[11px] text-[var(--color-text-secondary)]">
                  {new Date(app.deployed_at).toLocaleString('ko-KR')}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────
export function MonitoringPage() {
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === 'admin'

  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [activeView, setActiveView] = useState<ViewType | null>('cicd')

  const { clusters, filteredStacks, selectedCluster, selectedStack } =
    useClusterStackFilterState(selectedClusterId, selectedStackId)

  function handleClusterChange(id: string) {
    const clusterChanged = id !== selectedClusterId
    setSelectedClusterId(id)
    if (clusterChanged) {
      setSelectedStackId('')
      if (activeView === 'stack') setActiveView('cluster')
    }
    if (id && !activeView) setActiveView('cluster')
  }
  function handleStackChange(id: string) {
    setSelectedStackId(id)
    if (id && !activeView) setActiveView('stack')
  }

  const views: { id: ViewType; label: string; icon: React.ReactNode; disabled?: boolean }[] = [
    { id: 'cluster', label: 'Cluster', icon: <Server size={15} />, disabled: !selectedClusterId },
    { id: 'stack', label: 'Stack', icon: <BarChart3 size={15} />, disabled: !selectedStackId },
    { id: 'cicd', label: 'CI/CD', icon: <GitBranch size={15} /> },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'Monitoring Dashboard' }]} />

      {/* Page header */}
      <div className="mb-6 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
          <BarChart3 size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">Monitoring Dashboard</h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">Select a Cluster or Stack to start monitoring</p>
        </div>
      </div>

      <DeployedStacksPanel />

      <ClusterStackFilter
        selectedClusterId={selectedClusterId}
        selectedStackId={selectedStackId}
        onClusterChange={handleClusterChange}
        onStackChange={handleStackChange}
        onClear={() => { setSelectedClusterId(''); setSelectedStackId(''); setActiveView(null) }}
        clusters={clusters}
        filteredStacks={filteredStacks}
        selectedCluster={selectedCluster}
        selectedStack={selectedStack}
      />

      {/* ── View switcher ── */}
      <div className="mb-0 flex items-end border-b border-[var(--color-border-default)]">
        {views.map((v) => (
          <button key={v.id} type="button"
            onClick={() => !v.disabled && setActiveView(v.id)}
            disabled={v.disabled}
            className={cn(
              'flex items-center gap-2 border-b-2 px-5 py-3 text-sm font-semibold transition-colors',
              activeView === v.id
                ? 'border-b-[var(--color-primary)] text-[var(--color-text-primary)]'
                : v.disabled
                  ? 'cursor-not-allowed border-b-transparent text-[var(--color-text-secondary)] opacity-35'
                  : 'border-b-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
            )}>
            {v.icon}{v.label}
          </button>
        ))}
      </div>

      {/* View content */}
      <div className="pt-5">
        {activeView === 'cluster' && selectedClusterId && (
          <DashboardTabLayout
            viewId="cluster"
            isAdmin={isAdmin}
            defaultContent={<ClusterDefault clusterId={selectedCluster?.name ?? selectedClusterId} />}
          />
        )}
        {activeView === 'cluster' && !selectedClusterId && (
          <div className="flex h-44 flex-col items-center justify-center gap-2 rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] text-[var(--color-text-secondary)]">
            <Server size={28} className="opacity-20" />
            <p className="text-sm font-medium text-[var(--color-text-primary)]">Select a Cluster above</p>
          </div>
        )}
        {activeView === 'stack' && selectedStackId && (
          <DashboardTabLayout
            viewId="stack"
            isAdmin={isAdmin}
            defaultContent={<StackDefault stackName={selectedStack?.name ?? selectedStackId} />}
            firstTimePanel={(onConnect, onSkip) => (
              <StackConnectPanel
                stackName={selectedStack?.name ?? selectedStackId}
                onConnect={onConnect}
                onSkip={onSkip}
              />
            )}
          />
        )}
        {activeView === 'stack' && !selectedStackId && (
          <div className="flex h-44 flex-col items-center justify-center gap-2 rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] text-[var(--color-text-secondary)]">
            <BarChart3 size={28} className="opacity-20" />
            <p className="text-sm font-medium text-[var(--color-text-primary)]">Select a Stack above</p>
          </div>
        )}
        {activeView === 'cicd' && (
          <DashboardTabLayout
            viewId="cicd"
            isAdmin={isAdmin}
            defaultContent={<CicdDefault />}
            seedTabs={CICD_DEFAULT_TABS}
          />
        )}
      </div>
    </div>
  )
}

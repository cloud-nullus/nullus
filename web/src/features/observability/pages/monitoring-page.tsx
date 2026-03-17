import { useMemo, useState } from 'react'
import { Cpu, HardDrive, MemoryStick, Box, CheckCircle, AlertCircle, XCircle } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useDashboard } from '../api/observability-api'
import type { ToolHealthStatus } from '../api/observability-api'
import { cn } from '../../../lib/utils'
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'

const TOOL_STATUS_CONFIG: Record<ToolHealthStatus, { icon: React.ReactNode; badgeClassName: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, badgeClassName: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, badgeClassName: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Warning' },
  error: { icon: <XCircle size={13} />, badgeClassName: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Error' },
}

function UsageBar({ value, color }: { value: number; color: string }) {
  const normalized = Math.max(0, Math.min(100, value))

  return (
    <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[rgba(255,255,255,0.08)]">
      <svg className="h-full w-full" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
        <rect width={normalized} height="6" rx="3" fill={color} />
      </svg>
    </div>
  )
}

type TimeRange = '1h' | '6h' | '24h' | '7d'

function generateTimeSeries(range: TimeRange) {
  const pointsByRange: Record<TimeRange, number> = {
    '1h': 6,
    '6h': 12,
    '24h': 24,
    '7d': 28,
  }

  const hoursByRange: Record<TimeRange, number> = {
    '1h': 1,
    '6h': 6,
    '24h': 24,
    '7d': 24 * 7,
  }

  const now = Date.now()
  const points = pointsByRange[range]
  const totalHours = hoursByRange[range]
  const hourStep = totalHours / points

  return Array.from({ length: points }, (_, index) => {
    const ageHours = totalHours - hourStep * (index + 1)
    const ts = new Date(now - ageHours * 60 * 60 * 1000)
    const label = range === '7d'
      ? ts.toLocaleDateString('en-US', { weekday: 'short' })
      : ts.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })

    const cpuWave = 56 + Math.sin(index / 2.5) * 16 + (index % 3) * 2.1
    const memoryWave = 63 + Math.cos(index / 3.2) * 10 + (index % 4) * 1.8

    return {
      time: label,
      cpu: Math.max(12, Math.min(96, Math.round(cpuWave))),
      memory: Math.max(24, Math.min(97, Math.round(memoryWave))),
    }
  })
}

type ObsTab = 'stack' | 'cicd'

const CICD_KPI_CARDS = [
  { label: 'Build Success Rate', value: '97.3%', icon: <CheckCircle size={18} />, color: '#22c55e', iconWrapClassName: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', bar: 97 },
  { label: 'Total Builds', value: '145', icon: <Box size={18} />, color: '#6366f1', iconWrapClassName: 'bg-[rgba(99,102,241,0.15)] text-[#6366f1]', bar: 72 },
  { label: 'Avg Build Time', value: '2m 34s', icon: <Cpu size={18} />, color: '#f59e0b', iconWrapClassName: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', bar: 43 },
  { label: 'Pods Running', value: '3/3', icon: <Box size={18} />, color: '#10b981', iconWrapClassName: 'bg-[rgba(16,185,129,0.15)] text-[#10b981]', bar: 100 },
]

const CICD_PIPELINE_BARS = [
  { day: 'Mon', success: 12, failed: 1 },
  { day: 'Tue', success: 16, failed: 2 },
  { day: 'Wed', success: 14, failed: 3 },
  { day: 'Thu', success: 19, failed: 1 },
  { day: 'Fri', success: 22, failed: 2 },
  { day: 'Sat', success: 8, failed: 0 },
  { day: 'Sun', success: 6, failed: 1 },
]

const CICD_TOOLS: { name: string; version: string; status: ToolHealthStatus }[] = [
  { name: 'GitLab CI', status: 'running', version: '16.7' },
  { name: 'ArgoCD', status: 'running', version: '2.9.3' },
  { name: 'Harbor', status: 'running', version: '2.8.2' },
  { name: 'Trivy', status: 'warning', version: '0.48.1' },
]

export function MonitoringPage() {
  const [range, setRange] = useState<TimeRange>('24h')
  const [activeTab, setActiveTab] = useState<ObsTab>('stack')
  const { data: apiData, isLoading } = useDashboard(5000)
  const fallbackDashboard = {
    kpi: {
      cpuUsage: 68,
      memoryUsage: 42,
      storageUsage: 31,
      podCount: 27,
      podRunning: 24,
    },
    pipeline: {
      successRate: 97.3,
      totalRuns: 145,
      avgBuildSeconds: 154,
    },
    tools: [
      { name: 'GitLab', version: '16.7', status: 'running' as const },
      { name: 'Argo CD', version: '2.9.3', status: 'running' as const },
      { name: 'Prometheus', version: '2.48.1', status: 'running' as const },
      { name: 'Grafana', version: '10.3', status: 'warning' as const },
      { name: 'Harbor', version: '2.8.2', status: 'running' as const },
    ],
  }
  const isDashboardReady = !isLoading && !!apiData && typeof apiData === 'object' && 'kpi' in apiData
  const dashboard = isDashboardReady ? apiData : fallbackDashboard
  const kpi = dashboard.kpi
  const pipeline = dashboard.pipeline
  const tools = dashboard.tools

  const usageData = useMemo(() => generateTimeSeries(range), [range])

  const pipelineBars = useMemo(
    () => [
      { day: 'Mon', success: 16, failed: 2 },
      { day: 'Tue', success: 19, failed: 3 },
      { day: 'Wed', success: 15, failed: 4 },
      { day: 'Thu', success: 21, failed: 2 },
      { day: 'Fri', success: 24, failed: 3 },
      { day: 'Sat', success: 11, failed: 2 },
      { day: 'Sun', success: 9, failed: 1 },
    ],
    []
  )

  const podStatusData = useMemo(
    () => [
      { name: 'Running', value: kpi?.podRunning ?? 0, color: '#22c55e' },
      { name: 'Pending', value: Math.max(1, (kpi?.podCount ?? 0) - (kpi?.podRunning ?? 0) - 1), color: '#f59e0b' },
      { name: 'Failed', value: 1, color: '#ef4444' },
    ],
    [kpi?.podCount, kpi?.podRunning]
  )

  const kpiCards = [
    { label: 'CPU 사용률', value: `${kpi?.cpuUsage ?? 0}%`, icon: <Cpu size={18} />, color: '#60a5fa', iconWrapClassName: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]', bar: kpi?.cpuUsage ?? 0 },
    { label: '메모리 사용률', value: `${kpi?.memoryUsage ?? 0}%`, icon: <MemoryStick size={18} />, color: '#a78bfa', iconWrapClassName: 'bg-[rgba(139,92,246,0.15)] text-[#a78bfa]', bar: kpi?.memoryUsage ?? 0 },
    { label: '스토리지', value: `${kpi?.storageUsage ?? 0}%`, icon: <HardDrive size={18} />, color: '#34d399', iconWrapClassName: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]', bar: kpi?.storageUsage ?? 0 },
    { label: 'Pod 수', value: `${kpi?.podRunning ?? 0} / ${kpi?.podCount ?? 0}`, icon: <Box size={18} />, color: '#fbbf24', iconWrapClassName: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]', bar: kpi?.podCount ? Math.round(((kpi?.podRunning ?? 0) / kpi.podCount) * 100) : 0 },
  ]

  const cardClassName = 'rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]'
  const sectionTitleClassName = 'm-0 text-[15px] font-bold text-[var(--color-text-primary)]'

  return (
    <div>
      <Breadcrumb items={[{ label: 'Monitoring Dashboard' }]} />

      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
          <Cpu size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Monitoring Dashboard
          </h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            클러스터 및 도구 상태 실시간 모니터링 (5초 자동 갱신)
          </p>
        </div>
      </div>

      {/* Stack / CI/CD Tab Toggle */}
      <div className="mb-6 flex gap-1.5">
        {(['stack', 'cicd'] as const).map((tab) => {
          const active = activeTab === tab
          return (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={cn(
                'cursor-pointer rounded-[7px] border px-3 py-[5px] text-xs font-bold',
                active
                  ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]'
              )}
            >
              {tab === 'stack' ? 'Stack' : 'CI/CD'}
            </button>
          )
        })}
      </div>

      {/* KPI Cards */}
      <div className="mb-6 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
        {(activeTab === 'cicd' ? CICD_KPI_CARDS : kpiCards).map((card) => (
          <div key={card.label} className={cardClassName}>
            <div className="mb-2.5 flex items-center gap-2.5">
              <div
                className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', card.iconWrapClassName)}
              >
                {card.icon}
              </div>
              <span className="text-xs font-medium text-[var(--color-text-secondary)]">{card.label}</span>
            </div>
            <div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">
              {card.value}
            </div>
            <UsageBar value={card.bar} color={card.color} />
          </div>
        ))}
      </div>

      <div className={cn(cardClassName, 'mb-6')}>
        <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
          <h2 className={sectionTitleClassName}>Monitoring Charts</h2>
          <div className="flex gap-1.5">
            {(['1h', '6h', '24h', '7d'] as const).map((item) => {
              const active = range === item
              return (
                <button
                  key={item}
                  type="button"
                  onClick={() => setRange(item)}
                  className={cn(
                    'cursor-pointer rounded-[7px] border px-2.5 py-[5px] text-xs font-bold',
                    active
                      ? 'border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]'
                      : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]'
                  )}
                >
                  {item}
                </button>
              )
            })}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3.5">
          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">CPU Usage</div>
            <ResponsiveContainer width="100%" height={250}>
              <AreaChart data={usageData}>
                <defs>
                  <linearGradient id="cpuGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.58} />
                    <stop offset="95%" stopColor="#f59e0b" stopOpacity={0.06} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="rgba(148,163,184,0.2)" strokeDasharray="3 3" />
                <XAxis dataKey="time" stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
                <YAxis domain={[0, 100]} stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
                <Tooltip contentStyle={{ background: '#111827', border: '1px solid #374151', color: '#e5e7eb' }} />
                <Legend wrapperStyle={{ color: '#e5e7eb' }} />
                <Area type="monotone" dataKey="cpu" stroke="#f59e0b" strokeWidth={2} fill="url(#cpuGradient)" name="CPU %" />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">Memory Usage</div>
            <ResponsiveContainer width="100%" height={250}>
              <AreaChart data={usageData}>
                <defs>
                  <linearGradient id="memoryGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.54} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0.08} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="rgba(148,163,184,0.2)" strokeDasharray="3 3" />
                <XAxis dataKey="time" stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
                <YAxis domain={[0, 100]} stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
                <Tooltip contentStyle={{ background: '#111827', border: '1px solid #374151', color: '#e5e7eb' }} />
                <Legend wrapperStyle={{ color: '#e5e7eb' }} />
                <Area type="monotone" dataKey="memory" stroke="#3b82f6" strokeWidth={2} fill="url(#memoryGradient)" name="Memory %" />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">Pipeline Success Rate</div>
            <ResponsiveContainer width="100%" height={250}>
              <BarChart data={activeTab === 'cicd' ? CICD_PIPELINE_BARS : pipelineBars}>
                <CartesianGrid stroke="rgba(148,163,184,0.2)" strokeDasharray="3 3" />
                <XAxis dataKey="day" stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
                <YAxis stroke="#cbd5e1" tick={{ fill: '#cbd5e1', fontSize: 11 }} />
                <Tooltip contentStyle={{ background: '#111827', border: '1px solid #374151', color: '#e5e7eb' }} />
                <Legend wrapperStyle={{ color: '#e5e7eb' }} />
                <Bar dataKey="success" fill="#22c55e" radius={[5, 5, 0, 0]} />
                <Bar dataKey="failed" fill="#ef4444" radius={[5, 5, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">Pod Status</div>
            <ResponsiveContainer width="100%" height={250}>
              <PieChart>
                <Pie data={podStatusData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={86} label>
                  {podStatusData.map((entry) => (
                    <Cell key={entry.name} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip contentStyle={{ background: '#111827', border: '1px solid #374151', color: '#e5e7eb' }} />
                <Legend wrapperStyle={{ color: '#e5e7eb' }} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="mt-3 text-xs text-[var(--color-text-secondary)]">
          {activeTab === 'cicd'
            ? 'Pipeline summary: 97.3% success, 145 total runs, average build 2m 34s.'
            : `Pipeline summary: ${pipeline.successRate}% success, ${pipeline.totalRuns} total runs, average build ${Math.floor(pipeline.avgBuildSeconds / 60)}m ${pipeline.avgBuildSeconds % 60}s.`}
        </div>
      </div>

      {/* Tool Health */}
      <div className={cardClassName}>
        <h2 className={cn(sectionTitleClassName, 'mb-4')}>Tool Health</h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
          {(activeTab === 'cicd' ? CICD_TOOLS : tools).map((tool) => {
            const cfg = TOOL_STATUS_CONFIG[tool.status]
            return (
              <div key={tool.name} className="rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3.5">
                <div className="mb-1.5 flex items-center justify-between">
                  <span className="text-sm font-bold text-[var(--color-text-primary)]">{tool.name}</span>
                  <span className={cn('inline-flex items-center gap-1 rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', cfg.badgeClassName)}>
                    {cfg.icon}
                    {cfg.label}
                  </span>
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">v{tool.version}</div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

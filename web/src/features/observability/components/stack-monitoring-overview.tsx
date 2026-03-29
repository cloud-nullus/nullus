import {
  AlertCircle,
  Box,
  CheckCircle,
  Cpu,
  HardDrive,
  MemoryStick,
  XCircle,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend as ChartLegend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip as ChartTooltip,
  type ChartOptions,
} from 'chart.js'
import { Bar, Doughnut, Line } from 'react-chartjs-2'
import { cn } from '../../../lib/utils'
import { useStackMonitoring } from '../../stack/api/stack-api'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  ChartTooltip,
  ChartLegend,
  Filler,
)

type MonitoringRange = 'realtime' | '1h' | '6h' | '24h' | '7d'
type ToolHealthStatus = 'running' | 'warning' | 'error'

const TOOL_STATUS_CONFIG: Record<
  ToolHealthStatus,
  { icon: React.ReactNode; badgeClassName: string; label: string }
> = {
  running: {
    icon: <CheckCircle size={13} />,
    badgeClassName: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
    label: 'Running',
  },
  warning: {
    icon: <AlertCircle size={13} />,
    badgeClassName: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
    label: 'Warning',
  },
  error: {
    icon: <XCircle size={13} />,
    badgeClassName: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]',
    label: 'Error',
  },
}

type MonitoringSample = {
  ts: number
  overall: ScopeMetrics
  byTool: Record<string, ScopeMetrics>
}

type ScopeMetrics = {
  cpuRequest: number
  cpuLimit: number
  cpuUsage: number | null
  memoryRequest: number
  memoryLimit: number
  memoryUsage: number | null
  storageRequest: number
  storageLimit: number
  storageUsage: number | null
  readyPods: number
  totalPods: number
  statusCounts: Record<string, number>
}

function toolLogoURL(toolName: string): string {
  const key = toolName.toLowerCase()
  const map: Record<string, string> = {
    gitlab: 'gitlab',
    'gitlab-ci': 'gitlab',
    'gitlab-registry': 'gitlab',
    argocd: 'argo',
    'argo-cd': 'argo',
    grafana: 'grafana',
    prometheus: 'prometheus',
    loki: 'grafana',
    opensearch: 'opensearch',
    elasticsearch: 'elasticsearch',
    'opentelemetry-collector': 'opentelemetry',
    tempo: 'grafana',
    jaeger: 'jaeger',
    harbor: 'goharbor',
    minio: 'minio',
  }
  const slug = map[key] ?? 'kubernetes'
  return `https://cdn.simpleicons.org/${slug}`
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

function normalizeToPercent(value: number, maxValue: number): number {
  if (maxValue <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((value / maxValue) * 100)))
}

function selectSeries(samples: MonitoringSample[], range: MonitoringRange): MonitoringSample[] {
  if (samples.length === 0) return []
  if (range === 'realtime') {
    return samples.slice(-60)
  }

  const now = Date.now()
  const windowMs: Record<Exclude<MonitoringRange, 'realtime'>, number> = {
    '1h': 60 * 60 * 1000,
    '6h': 6 * 60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
    '7d': 7 * 24 * 60 * 60 * 1000,
  }
  const cutoff = now - windowMs[range]
  const ranged = samples.filter((s) => s.ts >= cutoff)
  if (ranged.length <= 120) return ranged

  const stride = Math.ceil(ranged.length / 120)
  return ranged.filter((_, idx) => idx % stride === 0)
}

function toStatusCountMap(items: Array<{ name: string; count: number }>): Record<string, number> {
  return items.reduce<Record<string, number>>((acc, item) => {
    acc[item.name] = item.count
    return acc
  }, {})
}

function toScopeMetricsFromPods(
  pods: Array<{
    phase: string
    ready: boolean
    cpu_request_millicores: number
    cpu_limit_millicores: number
    cpu_usage_millicores: number
    memory_request_mib: number
    memory_limit_mib: number
    memory_usage_mib: number
    storage_request_gib?: number
    storage_limit_gib?: number
    storage_usage_gib?: number
  }>,
): ScopeMetrics {
  const statusCounts: Record<string, number> = {}
  let cpuRequest = 0
  let cpuLimit = 0
  let cpuUsage = 0
  let memoryRequest = 0
  let memoryLimit = 0
  let memoryUsage = 0
  let storageRequest = 0
  let storageLimit = 0
  let storageUsage = 0
  let storageUsageHit = false
  let readyPods = 0

  for (const pod of pods) {
    const phase = pod.phase?.trim() || 'Unknown'
    statusCounts[phase] = (statusCounts[phase] ?? 0) + 1
    cpuRequest += pod.cpu_request_millicores
    cpuLimit += pod.cpu_limit_millicores
    cpuUsage += pod.cpu_usage_millicores
    memoryRequest += pod.memory_request_mib
    memoryLimit += pod.memory_limit_mib
    memoryUsage += pod.memory_usage_mib
    storageRequest += pod.storage_request_gib ?? 0
    storageLimit += pod.storage_limit_gib ?? 0
    if (typeof pod.storage_usage_gib === 'number') {
      storageUsage += pod.storage_usage_gib
      storageUsageHit = true
    }
    if (pod.ready) readyPods += 1
  }

  return {
    cpuRequest,
    cpuLimit,
    cpuUsage,
    memoryRequest,
    memoryLimit,
    memoryUsage,
    storageRequest,
    storageLimit,
    storageUsage: storageUsageHit ? storageUsage : null,
    readyPods,
    totalPods: pods.length,
    statusCounts,
  }
}

function isResourceLinkedToPods(resourceName: string, podNames: string[]): boolean {
  for (const podName of podNames) {
    if (podName === resourceName || podName.startsWith(`${resourceName}-`)) {
      return true
    }
  }
  return false
}

export function StackMonitoringOverview({ stackId }: { stackId: string }) {
  const [range, setRange] = useState<MonitoringRange>('realtime')
  const [scope, setScope] = useState<string>('all')
  const [samples, setSamples] = useState<MonitoringSample[]>([])
  const [ossIconPositions, setOssIconPositions] = useState<number[]>([])
  const ossBarChartRef = useRef<ChartJS<'bar'> | null>(null)
  const { data: monitoring, isLoading } = useStackMonitoring(stackId, 5000)

  const scopeOptions = useMemo(
    () => [
      { key: 'all', label: 'All' },
      ...(monitoring?.oss_statuses ?? []).map((tool) => ({ key: tool.key, label: tool.name })),
    ],
    [monitoring],
  )

  useEffect(() => {
    if (!scopeOptions.some((item) => item.key === scope)) {
      setScope('all')
    }
  }, [scopeOptions, scope])

  useEffect(() => {
    if (!monitoring?.summary) return

    const overall: ScopeMetrics = {
      cpuRequest: monitoring.summary.cpu_request_millicores,
      cpuLimit: monitoring.summary.cpu_limit_millicores,
      cpuUsage: monitoring.summary.usage_available ? monitoring.summary.cpu_usage_millicores : null,
      memoryRequest: monitoring.summary.memory_request_mib,
      memoryLimit: monitoring.summary.memory_limit_mib,
      memoryUsage: monitoring.summary.usage_available ? monitoring.summary.memory_usage_mib : null,
      storageRequest: monitoring.summary.storage_request_gib ?? 0,
      storageLimit: monitoring.summary.storage_limit_gib ?? 0,
      storageUsage: monitoring.summary.storage_usage_available ? (monitoring.summary.storage_usage_gib ?? 0) : null,
      readyPods: monitoring.summary.ready_pods,
      totalPods: monitoring.summary.total_pods,
      statusCounts: toStatusCountMap(monitoring.pod_status_counts ?? []),
    }

    const byTool = (monitoring.oss_statuses ?? []).reduce<Record<string, ScopeMetrics>>((acc, tool) => {
      const metrics = toScopeMetricsFromPods(tool.pods ?? [])
      if (!monitoring.summary.usage_available) {
        metrics.cpuUsage = null
        metrics.memoryUsage = null
      }
      acc[tool.key] = metrics
      return acc
    }, {})

    const next: MonitoringSample = {
      ts: Date.now(),
      overall,
      byTool,
    }

    setSamples((prev) => [...prev, next].slice(-4000))
  }, [monitoring])

  const activeTool = useMemo(
    () => (monitoring?.oss_statuses ?? []).find((tool) => tool.key === scope) ?? null,
    [monitoring, scope],
  )

  const currentMetrics = useMemo<ScopeMetrics>(() => {
    if (!monitoring?.summary) {
      return {
        cpuRequest: 0,
        cpuLimit: 0,
        cpuUsage: null,
        memoryRequest: 0,
        memoryLimit: 0,
        memoryUsage: null,
        storageRequest: 0,
        storageLimit: 0,
        storageUsage: null,
        readyPods: 0,
        totalPods: 0,
        statusCounts: {},
      }
    }
    if (scope === 'all') {
      return {
        cpuRequest: monitoring.summary.cpu_request_millicores,
        cpuLimit: monitoring.summary.cpu_limit_millicores,
        cpuUsage: monitoring.summary.usage_available ? monitoring.summary.cpu_usage_millicores : null,
        memoryRequest: monitoring.summary.memory_request_mib,
        memoryLimit: monitoring.summary.memory_limit_mib,
        memoryUsage: monitoring.summary.usage_available ? monitoring.summary.memory_usage_mib : null,
        storageRequest: monitoring.summary.storage_request_gib ?? 0,
        storageLimit: monitoring.summary.storage_limit_gib ?? 0,
        storageUsage: monitoring.summary.storage_usage_available ? (monitoring.summary.storage_usage_gib ?? 0) : null,
        readyPods: monitoring.summary.ready_pods,
        totalPods: monitoring.summary.total_pods,
        statusCounts: toStatusCountMap(monitoring.pod_status_counts ?? []),
      }
    }
    if (!activeTool) {
      return {
        cpuRequest: 0,
        cpuLimit: 0,
        cpuUsage: null,
        memoryRequest: 0,
        memoryLimit: 0,
        memoryUsage: null,
        storageRequest: 0,
        storageLimit: 0,
        storageUsage: null,
        readyPods: 0,
        totalPods: 0,
        statusCounts: {},
      }
    }
    const scoped = toScopeMetricsFromPods(activeTool.pods ?? [])
    if (!monitoring.summary.usage_available) {
      scoped.cpuUsage = null
      scoped.memoryUsage = null
    }
    return scoped
  }, [monitoring, scope, activeTool])

  const usageData = useMemo(() => {
    const selected = selectSeries(samples, range)
    return selected.map((item) => {
      const ts = new Date(item.ts)
      const scoped =
        scope === 'all'
          ? item.overall
          : (item.byTool[scope] ?? {
              cpuRequest: 0,
              cpuLimit: 0,
              cpuUsage: null,
              memoryRequest: 0,
              memoryLimit: 0,
              memoryUsage: null,
              storageRequest: 0,
              storageLimit: 0,
              storageUsage: null,
              readyPods: 0,
              totalPods: 0,
              statusCounts: {},
            })
      return {
        time:
          range === '7d'
            ? ts.toLocaleDateString('en-US', { month: '2-digit', day: '2-digit' })
            : ts.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }),
        cpuRequest: scoped.cpuRequest,
        cpuLimit: scoped.cpuLimit,
        cpuUsage: scoped.cpuUsage,
        memoryRequest: scoped.memoryRequest,
        memoryLimit: scoped.memoryLimit,
        memoryUsage: scoped.memoryUsage,
      }
    })
  }, [samples, range, scope])

  const cpuMaxInWindow = useMemo(() => {
    const values = usageData.flatMap((item) => [item.cpuRequest, item.cpuLimit, item.cpuUsage ?? 0])
    return Math.max(0, ...values)
  }, [usageData])

  const memoryMaxInWindow = useMemo(() => {
    const values = usageData.flatMap((item) => [item.memoryRequest, item.memoryLimit, item.memoryUsage ?? 0])
    return Math.max(0, ...values)
  }, [usageData])

  const podStatusData = useMemo(() => {
    const palette = ['#22c55e', '#f59e0b', '#ef4444', '#60a5fa', '#a78bfa', '#94a3b8']
    const counts = Object.entries(currentMetrics.statusCounts).map(([name, count]) => ({ name, count }))
    return counts.map((item, idx) => ({
      name: item.name,
      value: item.count,
      color: palette[idx % palette.length],
    }))
  }, [currentMetrics])

  const ossBars = useMemo(() => {
    if (scope === 'all') {
      return [...(monitoring?.oss_statuses ?? [])]
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((tool) => ({
          key: tool.key,
          fullName: tool.name,
          iconUrl: toolLogoURL(tool.name),
          pods: tool.pod_count,
          ready: tool.ready_pods,
        }))
    }
    if (!activeTool) return []
    return [{
      key: activeTool.key,
      fullName: activeTool.name,
      iconUrl: toolLogoURL(activeTool.name),
      pods: activeTool.pod_count,
      ready: activeTool.ready_pods,
    }]
  }, [monitoring, scope, activeTool])

  const visibleResources = useMemo(() => {
    const all = monitoring?.installed_resources ?? []
    if (scope === 'all' || !activeTool) return all
    const podNames = (activeTool.pods ?? []).map((pod) => pod.name)
    return all.filter((res) => isResourceLinkedToPods(res.name, podNames))
  }, [monitoring, scope, activeTool])

  const kpiCards = useMemo(() => {
    if (!monitoring?.summary) {
      return [
        { label: 'Current CPU', value: '-', icon: <Cpu size={18} />, color: '#60a5fa', iconWrapClassName: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]', bar: 0, metricScale: { current: null, request: 0, limit: 0, unit: 'Core' } },
        { label: 'Current Memory', value: '-', icon: <MemoryStick size={18} />, color: '#a78bfa', iconWrapClassName: 'bg-[rgba(139,92,246,0.15)] text-[#a78bfa]', bar: 0, metricScale: { current: null, request: 0, limit: 0, unit: 'GiB' } },
        { label: 'Current Storage', value: '-', icon: <HardDrive size={18} />, color: '#34d399', iconWrapClassName: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]', bar: 0 },
        { label: 'Ready Pods', value: '-', icon: <Box size={18} />, color: '#fbbf24', iconWrapClassName: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]', bar: 0 },
      ]
    }

    const readyRatio = currentMetrics.totalPods > 0 ? Math.round((currentMetrics.readyPods / currentMetrics.totalPods) * 100) : 0
    const cpuCurrentBar = currentMetrics.cpuUsage !== null
      ? normalizeToPercent(currentMetrics.cpuUsage, currentMetrics.cpuRequest || currentMetrics.cpuLimit || cpuMaxInWindow || 1)
      : 0
    const memoryCurrentBar = currentMetrics.memoryUsage !== null
      ? normalizeToPercent(currentMetrics.memoryUsage, currentMetrics.memoryRequest || currentMetrics.memoryLimit || memoryMaxInWindow || 1)
      : 0
    const storageCurrentBar = currentMetrics.storageUsage !== null
      ? normalizeToPercent(currentMetrics.storageUsage, currentMetrics.storageRequest || currentMetrics.storageLimit || 1)
      : 0
    const cpuUsageC = currentMetrics.cpuUsage !== null ? currentMetrics.cpuUsage / 1000 : null
    const cpuRequestC = currentMetrics.cpuRequest / 1000
    const cpuLimitC = currentMetrics.cpuLimit / 1000
    const memoryUsageGiB = currentMetrics.memoryUsage !== null ? currentMetrics.memoryUsage / 1024 : null
    const memoryRequestGiB = currentMetrics.memoryRequest / 1024
    const memoryLimitGiB = currentMetrics.memoryLimit / 1024
    const storageUsageGiB = currentMetrics.storageUsage
    const storageRequestGiB = currentMetrics.storageRequest
    const storageLimitGiB = currentMetrics.storageLimit

    return [
      {
        label: 'Current CPU',
        value: cpuUsageC !== null ? `${cpuUsageC.toFixed(2)} Core` : 'N/A',
        icon: <Cpu size={18} />,
        color: '#60a5fa',
        iconWrapClassName: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
        bar: cpuCurrentBar,
        metricScale: {
          current: cpuUsageC,
          request: cpuRequestC,
          limit: cpuLimitC,
          unit: 'Core',
        },
      },
      {
        label: 'Current Memory',
        value: memoryUsageGiB !== null ? `${memoryUsageGiB.toFixed(2)} GiB` : 'N/A',
        icon: <MemoryStick size={18} />,
        color: '#a78bfa',
        iconWrapClassName: 'bg-[rgba(139,92,246,0.15)] text-[#a78bfa]',
        bar: memoryCurrentBar,
        metricScale: {
          current: memoryUsageGiB,
          request: memoryRequestGiB,
          limit: memoryLimitGiB,
          unit: 'GiB',
        },
      },
      {
        label: 'Current Storage',
        value: storageUsageGiB !== null
          ? `${storageUsageGiB.toFixed(2)} GiB`
          : (storageLimitGiB > 0
              ? `${storageLimitGiB.toFixed(2)} GiB`
              : (storageRequestGiB > 0 ? `${storageRequestGiB.toFixed(2)} GiB` : '0.00 GiB')),
        icon: <HardDrive size={18} />,
        color: '#34d399',
        iconWrapClassName: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]',
        bar: storageCurrentBar,
        metricScale: {
          current: storageUsageGiB,
          request: storageRequestGiB,
          limit: storageRequestGiB,
          unit: 'GiB',
          showLimit: false,
        },
      },
      { label: 'Ready Pods', value: `${currentMetrics.readyPods} / ${currentMetrics.totalPods}`, icon: <Box size={18} />, color: '#fbbf24', iconWrapClassName: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]', bar: readyRatio },
    ]
  }, [monitoring, currentMetrics, cpuMaxInWindow, memoryMaxInWindow])

  const tools: { name: string; version: string; status: ToolHealthStatus }[] = useMemo(() => {
    const all = monitoring?.oss_statuses ?? []
    const filtered = scope === 'all' ? all : all.filter((tool) => tool.key === scope)
    return filtered.map((tool) => ({ name: tool.name, version: tool.version, status: tool.status }))
  }, [monitoring, scope])

  const cpuChartOptions: ChartOptions<'line'> = useMemo(() => ({
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { labels: { color: '#e5e7eb', boxWidth: 10, boxHeight: 10 } },
      tooltip: {
        backgroundColor: '#111827',
        borderColor: '#374151',
        borderWidth: 1,
        titleColor: '#f9fafb',
        bodyColor: '#e5e7eb',
      },
    },
    scales: {
      x: {
        ticks: {
          color: '#cbd5e1',
          maxRotation: 0,
          autoSkip: true,
          maxTicksLimit: 8,
        },
        grid: { color: 'rgba(148,163,184,0.12)' },
      },
      y: {
        ticks: {
          color: '#cbd5e1',
          callback: (value) => {
            const n = Number(value)
            if (!Number.isFinite(n)) return '0'
            if (n !== 0 && Math.abs(n) < 1) return n.toFixed(2)
            return `${Math.round(n)}`
          },
        },
        title: { display: true, text: 'Core', color: '#cbd5e1' },
        grid: { color: 'rgba(148,163,184,0.12)' },
        beginAtZero: true,
      },
    },
    elements: { line: { tension: 0.35 }, point: { radius: 0, hoverRadius: 3 } },
  }), [])

  const memoryChartOptions: ChartOptions<'line'> = useMemo(() => ({
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { labels: { color: '#e5e7eb', boxWidth: 10, boxHeight: 10 } },
      tooltip: {
        backgroundColor: '#111827',
        borderColor: '#374151',
        borderWidth: 1,
        titleColor: '#f9fafb',
        bodyColor: '#e5e7eb',
      },
    },
    scales: {
      x: {
        ticks: {
          color: '#cbd5e1',
          maxRotation: 0,
          autoSkip: true,
          maxTicksLimit: 8,
        },
        grid: { color: 'rgba(148,163,184,0.12)' },
      },
      y: {
        ticks: {
          color: '#cbd5e1',
          callback: (value) => {
            const n = Number(value)
            if (!Number.isFinite(n)) return '0'
            if (n !== 0 && Math.abs(n) < 1) return n.toFixed(2)
            return `${Math.round(n)}`
          },
        },
        title: { display: true, text: 'GiB', color: '#cbd5e1' },
        grid: { color: 'rgba(148,163,184,0.12)' },
        beginAtZero: true,
      },
    },
    elements: { line: { tension: 0.35 }, point: { radius: 0, hoverRadius: 3 } },
  }), [])

  const cpuChartData = useMemo(() => ({
    labels: usageData.map((item) => item.time),
    datasets: [
      { label: 'CPU Request', data: usageData.map((item) => item.cpuRequest / 1000), borderColor: '#60a5fa', backgroundColor: 'rgba(96,165,250,0.18)', fill: true },
      { label: 'CPU Limit', data: usageData.map((item) => item.cpuLimit / 1000), borderColor: '#f59e0b', backgroundColor: 'rgba(245,158,11,0.08)', fill: false },
      { label: 'CPU Current', data: usageData.map((item) => (item.cpuUsage === null ? null : item.cpuUsage / 1000)), borderColor: '#22c55e', backgroundColor: 'rgba(34,197,94,0.08)', fill: false },
    ],
  }), [usageData])

  const memoryChartData = useMemo(() => ({
    labels: usageData.map((item) => item.time),
    datasets: [
      { label: 'Memory Request', data: usageData.map((item) => item.memoryRequest / 1024), borderColor: '#3b82f6', backgroundColor: 'rgba(59,130,246,0.18)', fill: true },
      { label: 'Memory Limit', data: usageData.map((item) => item.memoryLimit / 1024), borderColor: '#f59e0b', backgroundColor: 'rgba(245,158,11,0.08)', fill: false },
      { label: 'Memory Current', data: usageData.map((item) => (item.memoryUsage === null ? null : item.memoryUsage / 1024)), borderColor: '#22c55e', backgroundColor: 'rgba(34,197,94,0.08)', fill: false },
    ],
  }), [usageData])

  const ossBarData = useMemo(() => ({
    labels: ossBars.map(() => ''),
    datasets: [
      { label: 'Total Pods', data: ossBars.map((item) => item.pods), backgroundColor: 'rgba(99,102,241,0.72)', borderRadius: 6 },
      { label: 'Ready Pods', data: ossBars.map((item) => item.ready), backgroundColor: 'rgba(34,197,94,0.72)', borderRadius: 6 },
    ],
  }), [ossBars])

  const ossBarOptions: ChartOptions<'bar'> = useMemo(() => ({
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { labels: { color: '#e5e7eb', boxWidth: 10, boxHeight: 10 } },
      tooltip: {
        backgroundColor: '#111827',
        borderColor: '#374151',
        borderWidth: 1,
        titleColor: '#f9fafb',
        bodyColor: '#e5e7eb',
        callbacks: {
          title: (items) => {
            const idx = items[0]?.dataIndex ?? 0
            return ossBars[idx]?.fullName ?? 'OSS'
          },
        },
      },
    },
    scales: {
      x: { ticks: { display: false }, grid: { color: 'rgba(148,163,184,0.12)' } },
      y: { beginAtZero: true, ticks: { color: '#cbd5e1', precision: 0 }, grid: { color: 'rgba(148,163,184,0.12)' } },
    },
  }), [ossBars])

  useEffect(() => {
    const updateIconPositions = () => {
      const chart = ossBarChartRef.current
      const xScale = chart?.scales?.x
      if (!xScale || ossBars.length === 0) {
        setOssIconPositions([])
        return
      }
      setOssIconPositions(ossBars.map((_, idx) => xScale.getPixelForValue(idx)))
    }

    const rafId = window.requestAnimationFrame(updateIconPositions)
    window.addEventListener('resize', updateIconPositions)
    return () => {
      window.cancelAnimationFrame(rafId)
      window.removeEventListener('resize', updateIconPositions)
    }
  }, [ossBars, usageData])

  const podStatusChartData = useMemo(() => ({
    labels: podStatusData.map((item) => item.name),
    datasets: [
      { data: podStatusData.map((item) => item.value), backgroundColor: podStatusData.map((item) => item.color), borderColor: 'rgba(15,23,42,0.8)', borderWidth: 2 },
    ],
  }), [podStatusData])

  const podStatusOptions: ChartOptions<'doughnut'> = useMemo(() => ({
    responsive: true,
    maintainAspectRatio: false,
    cutout: '62%',
    plugins: {
      legend: { position: 'bottom', labels: { color: '#e5e7eb', boxWidth: 10, boxHeight: 10 } },
      tooltip: {
        backgroundColor: '#111827',
        borderColor: '#374151',
        borderWidth: 1,
        titleColor: '#f9fafb',
        bodyColor: '#e5e7eb',
      },
    },
  }), [])

  const cardClassName =
    'rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]'

  return (
    <div>
      <div className="mb-6 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
        {kpiCards.map((card) => (
          <div key={card.label} className={cardClassName}>
            <div className="mb-2.5 flex items-center gap-2.5">
              <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', card.iconWrapClassName)}>
                {card.icon}
              </div>
              <span className="text-xs font-medium text-[var(--color-text-secondary)]">
                {card.label}
              </span>
            </div>
            <div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">
              {card.value}
            </div>
            {card.metricScale ? (
              <div className="mt-2">
                {(() => {
                  const showLimit = card.metricScale.showLimit !== false
                  const scaleLimit = showLimit ? card.metricScale.limit : card.metricScale.request
                  const scaleMax = Math.max(scaleLimit, 0.000001)
                  const reqPos = Math.max(0, Math.min(100, (card.metricScale.request / scaleMax) * 100))
                  const limPos = showLimit ? 100 : reqPos
                  const curPos = card.metricScale.current === null
                    ? null
                    : Math.max(0, Math.min(100, (card.metricScale.current / scaleMax) * 100))

                  const reqLabelShift = reqPos < 8 ? 'translate-x-0' : reqPos > 92 ? '-translate-x-full' : '-translate-x-1/2'
                  const limLabelShift = limPos < 8 ? 'translate-x-0' : limPos > 92 ? '-translate-x-full' : '-translate-x-1/2'

                  return (
                    <>
                      <div className="relative h-4">
                        <div className="absolute left-0 right-0 top-1 h-2 rounded-full bg-[rgba(148,163,184,0.22)]" />
                        {curPos !== null && (
                          <div className="absolute left-0 top-1 h-2 rounded-full" style={{ width: `${curPos}%`, backgroundColor: card.color }} />
                        )}
                        <div className="absolute top-0.5 h-[10px] w-px bg-[#60a5fa]" style={{ left: `${reqPos}%` }} />
                        {showLimit && (
                          <div className="absolute top-0.5 h-[10px] w-px bg-[#f59e0b]" style={{ left: `${limPos}%` }} />
                        )}
                      </div>
                      <div className="relative mt-1 h-8 text-[10px] font-semibold text-[var(--color-text-secondary)]">
                        <span className="absolute left-0 top-0 whitespace-nowrap">0</span>
                        <span className={`absolute top-0 whitespace-nowrap ${reqLabelShift}`} style={{ left: `${reqPos}%` }}>{card.metricScale.request.toFixed(2)}</span>
                        <span className={`absolute top-4 whitespace-nowrap ${reqLabelShift}`} style={{ left: `${reqPos}%` }}>(Req)</span>
                        {showLimit && (
                          <>
                            <span className={`absolute top-0 whitespace-nowrap ${limLabelShift}`} style={{ left: `${limPos}%` }}>{card.metricScale.limit.toFixed(2)}</span>
                            <span className={`absolute top-4 whitespace-nowrap ${limLabelShift}`} style={{ left: `${limPos}%` }}>(Lim)</span>
                          </>
                        )}
                      </div>
                    </>
                  )
                })()}
              </div>
            ) : (
              <UsageBar value={card.bar} color={card.color} />
            )}
          </div>
        ))}
      </div>

      <div className={cn(cardClassName, 'mb-6')}>
        <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <h2 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
              Resource Trend
            </h2>
            {isLoading && (
              <span className="rounded-full bg-[rgba(99,102,241,0.15)] px-2 py-0.5 text-[11px] font-semibold text-[#a5b4fc]">
                Loading...
              </span>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex gap-1.5">
              {scopeOptions.map((item) => {
                const active = scope === item.key
                return (
                  <button
                    key={item.key}
                    type="button"
                    onClick={() => setScope(item.key)}
                    className={cn(
                      'cursor-pointer rounded-[7px] border px-2.5 py-[5px] text-xs font-semibold',
                      active
                        ? 'border-[rgba(16,185,129,0.65)] bg-[rgba(16,185,129,0.2)] text-[#6ee7b7]'
                        : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]',
                    )}
                  >
                    {item.label}
                  </button>
                )
              })}
            </div>
            <div className="h-4 w-px bg-[var(--color-border-default)]" />
            <div className="flex gap-1.5">
              {(['realtime', '1h', '6h', '24h', '7d'] as const).map((item) => {
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
                        : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]',
                    )}
                  >
                    {item === 'realtime' ? 'Live 5s' : item}
                  </button>
                )
              })}
            </div>
          </div>
        </div>
        <div className="grid grid-cols-1 gap-3.5 xl:grid-cols-2">
          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
              CPU (Request / Limit / Current)
            </div>
            <div className="h-[250px]">
              <Line data={cpuChartData} options={cpuChartOptions} />
            </div>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
              Memory (Request / Limit / Current)
            </div>
            <div className="h-[250px]">
              <Line data={memoryChartData} options={memoryChartOptions} />
            </div>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
              OSS Pod Coverage
            </div>
            <div className="relative h-[272px]">
              <div className="h-[250px]">
                <Bar ref={ossBarChartRef} data={ossBarData} options={ossBarOptions} />
              </div>
              <div className="pointer-events-none absolute bottom-0 left-0 right-0 h-[20px]">
                {ossBars.map((item, idx) => {
                  const fallbackLeft = ((idx + 0.5) / Math.max(ossBars.length, 1)) * 100
                  const left = ossIconPositions[idx] ?? fallbackLeft
                  const leftStyle = typeof left === 'number'
                    ? (ossIconPositions[idx] !== undefined ? `${left}px` : `${left}%`)
                    : '0px'
                  return (
                    <div key={`oss-icon-${item.key}`} className="absolute bottom-0 -translate-x-1/2" style={{ left: leftStyle }}>
                      <div className="relative h-4 w-4">
                        <img
                          src={item.iconUrl}
                          alt={`${item.fullName} icon`}
                          className="h-4 w-4 rounded-[3px]"
                          loading="lazy"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            const fallback = e.currentTarget.nextElementSibling as HTMLElement | null
                            if (fallback) fallback.style.display = 'flex'
                          }}
                        />
                        <span className="hidden h-4 w-4 items-center justify-center rounded-[3px] bg-[rgba(148,163,184,0.25)] text-[9px] font-bold text-[#e2e8f0]" aria-hidden="true">
                          {item.fullName.slice(0, 1).toUpperCase()}
                        </span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
              Pod Status
            </div>
            <div className="h-[250px]">
              <Doughnut data={podStatusChartData} options={podStatusOptions} />
            </div>
          </div>
        </div>

        <div className="mt-3 text-xs text-[var(--color-text-secondary)]">
          Metrics refresh every 5 seconds. Scoped charts and tables reflect the currently selected OSS range.
        </div>
      </div>
      <div className={cardClassName}>
        <h2 className="m-0 mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">
          Tool Health
        </h2>
        <div className="mb-4 overflow-x-auto rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]">
          <table className="min-w-full text-left text-[12px] text-[var(--color-text-secondary)]">
            <thead>
              <tr className="border-b border-[var(--color-border-default)] text-[11px] uppercase tracking-[0.05em]">
                <th className="px-3 py-2">Kind</th>
                <th className="px-3 py-2">Resource</th>
                <th className="px-3 py-2">Ready/Desired</th>
                <th className="px-3 py-2">Status</th>
              </tr>
            </thead>
            <tbody>
              {visibleResources.map((item) => (
                <tr key={`${item.kind}-${item.name}`} className="border-b border-[rgba(255,255,255,0.04)]">
                  <td className="px-3 py-2">{item.kind}</td>
                  <td className="px-3 py-2 font-medium text-[var(--color-text-primary)]">{item.name}</td>
                  <td className="px-3 py-2">{item.ready_replicas}/{item.desired_replicas}</td>
                  <td className="px-3 py-2">{item.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
          {tools.map((tool) => {
            const cfg = TOOL_STATUS_CONFIG[tool.status]
            return (
              <div key={tool.name} className="rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3.5">
                <div className="mb-1.5 flex items-center justify-between">
                  <span className="text-sm font-bold text-[var(--color-text-primary)]">
                    {tool.name}
                  </span>
                  <span className={cn('inline-flex items-center gap-1 rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', cfg.badgeClassName)}>
                    {cfg.icon}
                    {cfg.label}
                  </span>
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">
                  v{tool.version}
                </div>
              </div>
            )
          })}
        </div>
        <div className="mt-4 space-y-2">
          {(scope === 'all' ? (monitoring?.oss_statuses ?? []) : (activeTool ? [activeTool] : [])).map((tool) => (
            <div key={`pods-${tool.key}`} className="rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
              <div className="mb-2 flex items-center justify-between">
                <div className="text-sm font-semibold text-[var(--color-text-primary)]">{tool.name} Pod Details</div>
                <div className="text-xs text-[var(--color-text-secondary)]">ready {tool.ready_pods}/{tool.pod_count}</div>
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-[12px] text-[var(--color-text-secondary)]">
                  <thead>
                    <tr className="border-b border-[var(--color-border-default)] text-[11px] uppercase tracking-[0.05em]">
                      <th className="py-1 pr-3">Pod</th>
                      <th className="py-1 pr-3">Status</th>
                      <th className="py-1 pr-3">Ready</th>
                      <th className="py-1 pr-3">CPU Req/Limit</th>
                      <th className="py-1 pr-3">Mem Req/Limit</th>
                      <th className="py-1 pr-3">Restarts</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tool.pods.map((pod) => (
                      <tr key={pod.name} className="border-b border-[rgba(255,255,255,0.04)]">
                        <td className="py-1 pr-3 font-medium text-[var(--color-text-primary)]">{pod.name}</td>
                        <td className="py-1 pr-3">{pod.status}</td>
                        <td className="py-1 pr-3">{pod.ready ? 'yes' : 'no'}</td>
                        <td className="py-1 pr-3">{pod.cpu_request_millicores}m / {pod.cpu_limit_millicores}m</td>
                        <td className="py-1 pr-3">{pod.memory_request_mib}Mi / {pod.memory_limit_mib}Mi</td>
                        <td className="py-1 pr-3">{pod.restart_count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

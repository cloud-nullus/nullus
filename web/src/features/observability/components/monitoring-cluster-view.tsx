import { useEffect, useMemo, useState } from "react"
import { useQueries } from "@tanstack/react-query"
import { AreaChart, Area, BarChart, Bar, Cell, Legend, Pie, PieChart, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts"
import { Box, Cpu, MemoryStick, Server } from "lucide-react"
import { api } from "../../../lib/api"
import type { StackMonitoringSnapshot } from "../../stack/api/stack-api"
import { useClusterMonitoringSummary } from "../../admin/api/admin-api"
import { useDeployments, usePipelines } from "../../cicd/api/cicd-api"
import { cn } from "../../../lib/utils"
import { CHART_STYLE, KpiCard, ChartPanel } from "./monitoring-chart-widgets"
import type { TimeRange } from "./monitoring-tab-layout"
import { formatRangeLabel, selectSeries } from "../utils/monitoring-utils"

export function ClusterDefault({
  clusterId,
  clusterName,
  stackIds,
}: {
  clusterId: string
  clusterName: string
  stackIds: string[]
}) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: pipelinesData } = usePipelines()
  const { data: deploymentsData } = useDeployments()
  const { data: clusterSummary } = useClusterMonitoringSummary(clusterId)
  const [samples, setSamples] = useState<Array<{ ts: number; cpu: number; memory: number }>>([])

  const monitoringQueries = useQueries({
    queries: stackIds.map((stackId) => ({
      queryKey: ['stacks', 'monitoring', stackId],
      queryFn: () => api.get<StackMonitoringSnapshot>(`/stacks/${stackId}/monitoring`).then((r) => r.data),
      enabled: !!stackId,
      refetchInterval: 5000,
      staleTime: 0,
    })),
  })

  const snapshots = useMemo(
    () =>
      monitoringQueries
        .map((query) => query.data)
        .filter((snapshot): snapshot is StackMonitoringSnapshot => !!snapshot),
    [monitoringQueries],
  )

  const aggregatedFromStacks = useMemo(() => {
    const totals = snapshots.reduce(
      (acc, snapshot) => {
        acc.totalPods += snapshot.summary.total_pods ?? 0
        acc.readyPods += snapshot.summary.ready_pods ?? 0
        acc.cpuRequest += snapshot.summary.cpu_request_millicores ?? 0
        acc.cpuUsage += snapshot.summary.cpu_usage_millicores ?? 0
        acc.memoryRequest += snapshot.summary.memory_request_mib ?? 0
        acc.memoryUsage += snapshot.summary.memory_usage_mib ?? 0
        return acc
      },
      {
        totalPods: 0,
        readyPods: 0,
        cpuRequest: 0,
        cpuUsage: 0,
        memoryRequest: 0,
        memoryUsage: 0,
      },
    )

    const cpuPercent = totals.cpuRequest > 0 ? Math.max(0, Math.round((totals.cpuUsage / totals.cpuRequest) * 100)) : 0
    const memoryPercent = totals.memoryRequest > 0 ? Math.max(0, Math.round((totals.memoryUsage / totals.memoryRequest) * 100)) : 0

    return {
      podCount: totals.totalPods,
      podRunning: totals.readyPods,
      cpuUsage: cpuPercent,
      memoryUsage: memoryPercent,
    }
  }, [snapshots])

  const aggregated = useMemo(() => {
    if (aggregatedFromStacks.podCount > 0) {
      return aggregatedFromStacks
    }

    return {
      podCount: clusterSummary?.total_pods ?? 0,
      podRunning: clusterSummary?.ready_pods ?? 0,
      cpuUsage: 0,
      memoryUsage: 0,
    }
  }, [aggregatedFromStacks, clusterSummary])

  useEffect(() => {
    if (!clusterId) return
    setSamples((prev) => [
      ...prev,
      {
        ts: Date.now(),
        cpu: aggregated.cpuUsage,
        memory: aggregated.memoryUsage,
      },
    ].slice(-4000))
  }, [clusterId, aggregated.cpuUsage, aggregated.memoryUsage])

  const selected = useMemo(() => selectSeries(samples, range), [samples, range])
  const series = useMemo(
    () => selected.map((s) => ({ time: formatRangeLabel(s.ts, range), cpu: s.cpu, memory: s.memory })),
    [selected, range],
  )

  const weekBars = useMemo(() => {
    const deployments = deploymentsData?.items ?? []
    const pipelineIds = new Set(
      (pipelinesData?.items ?? [])
        .filter((pipeline) => pipeline.clusterId === clusterId)
        .map((pipeline) => pipeline.id),
    )

    const deploymentsForCluster = deployments.filter((deployment) => pipelineIds.has(deployment.pipelineId))
    const now = new Date()
    const dayKeys = Array.from({ length: 7 }, (_, idx) => {
      const d = new Date(now)
      d.setDate(now.getDate() - (6 - idx))
      return d.toLocaleDateString('en-CA')
    })

    const byDay = dayKeys.reduce<Record<string, { day: string; success: number; failed: number }>>((acc, key) => {
      const dayDate = new Date(key)
      acc[key] = { day: dayDate.toLocaleDateString('en', { weekday: 'short' }), success: 0, failed: 0 }
      return acc
    }, {})

    deploymentsForCluster.forEach((deployment) => {
      const key = deployment.startedAt ? new Date(deployment.startedAt).toLocaleDateString('en-CA') : ''
      const bucket = byDay[key]
      if (!bucket) return
      if (deployment.status === 'success') bucket.success += 1
      if (deployment.status === 'failed') bucket.failed += 1
    })

    return dayKeys.map((key) => byDay[key])
  }, [deploymentsData?.items, pipelinesData?.items, clusterId])

  const podCount = aggregated.podCount
  const podRunning = aggregated.podRunning
  const cpuUsage = aggregated.cpuUsage
  const memoryUsage = aggregated.memoryUsage

  const pods = [
    { name: 'Running', value: podRunning, color: '#22c55e' },
    { name: 'Other', value: Math.max(0, podCount - podRunning), color: '#f59e0b' },
  ]

  const runningPodsPercent = podCount > 0 ? Math.round((podRunning / podCount) * 100) : 0

  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <div className={cn('flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium',
          'border-emerald-500/20 bg-emerald-500/5 text-emerald-400')}>
          <span className="h-2 w-2 rounded-full bg-emerald-400" />{clusterName || clusterId}
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
        <KpiCard label="Running Pods" value={`${podRunning}/${podCount}`} icon={<Server size={18} />} color="#60a5fa" iconCls="bg-[rgba(59,130,246,0.15)] text-[#60a5fa]" bar={runningPodsPercent} />
        <KpiCard label="Pods" value={String(podCount)} icon={<Box size={18} />} color="#22c55e" iconCls="bg-[rgba(34,197,94,0.15)] text-[#22c55e]" bar={runningPodsPercent} />
        <KpiCard label="CPU" value={`${Math.round(cpuUsage)}%`} icon={<Cpu size={18} />} color="#f59e0b" iconCls="bg-[rgba(245,158,11,0.15)] text-[#f59e0b]" bar={cpuUsage} />
        <KpiCard label="Memory" value={`${Math.round(memoryUsage)}%`} icon={<MemoryStick size={18} />} color="#a78bfa" iconCls="bg-[rgba(139,92,246,0.15)] text-[#a78bfa]" bar={memoryUsage} />
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPanel title="CPU Usage">
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={series}>
              <defs><linearGradient id="ccpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#f59e0b" stopOpacity={.5} /><stop offset="95%" stopColor="#f59e0b" stopOpacity={.05} /></linearGradient></defs>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
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
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
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
            <BarChart data={weekBars}>
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

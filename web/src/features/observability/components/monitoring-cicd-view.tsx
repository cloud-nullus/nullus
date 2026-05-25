import { useState, useMemo } from "react"
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts"
import { Activity, AlertCircle, CheckCircle, Clock, GitBranch, Layers, Package, XCircle } from "lucide-react"
import { useDeployments, usePipelines } from "../../cicd/api/cicd-api"
import { cn } from "../../../lib/utils"
import { CHART_STYLE, KpiCard, ChartPanel } from "./monitoring-chart-widgets"
import type { TimeRange } from "./monitoring-tab-layout"
import type { EmbedTab } from "../utils/monitoring-utils"
import { formatDuration, timeAgo } from "../utils/monitoring-utils"

// ─── Default content: CI/CD view ─────────────────────────────────────────────
// ─── CI/CD Application monitoring data ───────────────────────────────────────
type AppStatus = 'healthy' | 'degraded' | 'down'

interface DeployedAppRow {
  name: string
  version: string
  pipeline: string
  status: AppStatus
  pods: [number | null, number | null]
  cluster: string
  namespace: string
  duration: string
  lastDeploy: string
}

const APP_STATUS_CFG: Record<AppStatus, { label: string; cls: string; dot: string }> = {
  healthy: { label: 'Healthy', cls: 'bg-emerald-500/15 text-emerald-400', dot: 'bg-emerald-400' },
  degraded: { label: 'Degraded', cls: 'bg-amber-500/15 text-amber-400', dot: 'bg-amber-400' },
  down: { label: 'Down', cls: 'bg-red-500/15 text-red-400', dot: 'bg-red-400' },
}

/** Sample Grafana tab pre-seeded into CI/CD localStorage */
export const CICD_DEFAULT_TABS: EmbedTab[] = [
  {
    id: 'cicd-seed-grafana',
    label: 'Grafana',
    url: 'https://play.grafana.org/d/000000012/grafana-play-home?orgId=1&theme=dark&kiosk',
    order: 0,
  },
]

export function CicdDefault({ selectedClusterId }: { selectedClusterId: string }) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: pipelinesData } = usePipelines()
  const { data: deploymentsData } = useDeployments()

  const pipelines = useMemo(
    () => (pipelinesData?.items ?? []).filter((pipeline) => !selectedClusterId || pipeline.clusterId === selectedClusterId),
    [pipelinesData?.items, selectedClusterId],
  )

  const deployments = useMemo(() => {
    const allDeployments = deploymentsData?.items ?? []
    const pipelineIds = new Set(pipelines.map((pipeline) => pipeline.id))
    return allDeployments.filter((deployment) => pipelineIds.has(deployment.pipelineId))
  }, [deploymentsData?.items, pipelines])

  const latestByPipeline = useMemo(() => {
    const map = new Map<string, (typeof deployments)[number]>()
    deployments.forEach((deployment) => {
      const prev = map.get(deployment.pipelineId)
      if (!prev || new Date(deployment.startedAt).getTime() > new Date(prev.startedAt).getTime()) {
        map.set(deployment.pipelineId, deployment)
      }
    })
    return map
  }, [deployments])

  const rows = useMemo<DeployedAppRow[]>(() => pipelines.map((pipeline) => {
    const latest = latestByPipeline.get(pipeline.id)
    const status: AppStatus = latest?.status === 'failed' ? 'down' : latest?.status === 'running' ? 'degraded' : 'healthy'

    return {
      name: pipeline.name,
      version: latest?.version || '—',
      pipeline: pipeline.appType,
      status,
      pods: [null, null],
      cluster: pipeline.clusterName || '—',
      namespace: pipeline.namespace || 'default',
      duration: formatDuration(latest?.startedAt ?? null, latest?.completedAt ?? null),
      lastDeploy: timeAgo(latest?.startedAt ?? null),
    }
  }), [pipelines, latestByPipeline])

  const latestDeployments = useMemo(
    () => [...deployments].sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime()).slice(0, 8),
    [deployments],
  )

  const timeline = useMemo(() => {
    const now = new Date()
    const isDaily = range === '7d'
    const windowMs: Record<TimeRange, number> = {
      '1h': 60 * 60 * 1000,
      '6h': 6 * 60 * 60 * 1000,
      '24h': 24 * 60 * 60 * 1000,
      '7d': 7 * 24 * 60 * 60 * 1000,
    }
    const cutoff = now.getTime() - windowMs[range]

    const keys: string[] = []
    if (isDaily) {
      for (let i = 6; i >= 0; i -= 1) {
        const day = new Date(now)
        day.setDate(now.getDate() - i)
        keys.push(day.toLocaleDateString('en-CA'))
      }
    } else {
      const start = new Date(cutoff)
      start.setMinutes(0, 0, 0)
      const cur = new Date(start)
      while (cur.getTime() <= now.getTime()) {
        const key = `${cur.toLocaleDateString('en-CA')} ${cur.getHours().toString().padStart(2, '0')}:00`
        keys.push(key)
        cur.setHours(cur.getHours() + 1)
      }
    }

    const byKey = keys.reduce<Record<string, { time: string; success: number; failed: number }>>((acc, key) => {
      const label = isDaily
        ? new Date(key).toLocaleDateString('en', { weekday: 'short' })
        : key.slice(-5)
      acc[key] = { time: label, success: 0, failed: 0 }
      return acc
    }, {})

    deployments.forEach((deployment) => {
      const started = new Date(deployment.startedAt).getTime()
      if (Number.isNaN(started) || started < cutoff) return
      const date = new Date(started)
      const key = isDaily
        ? date.toLocaleDateString('en-CA')
        : `${date.toLocaleDateString('en-CA')} ${date.getHours().toString().padStart(2, '0')}:00`
      const bucket = byKey[key]
      if (!bucket) return
      if (deployment.status === 'success') bucket.success += 1
      if (deployment.status === 'failed') bucket.failed += 1
    })

    return keys.map((k) => byKey[k])
  }, [deployments, range])

  const successPipelines = pipelines.reduce((count, pipeline) => {
    const status = latestByPipeline.get(pipeline.id)?.status
    return status === 'success' ? count + 1 : count
  }, 0)
  const failedPipelines = pipelines.reduce((count, pipeline) => {
    const status = latestByPipeline.get(pipeline.id)?.status
    return status === 'failed' ? count + 1 : count
  }, 0)
  const runningDeployments = deployments.filter((d) => ['running', 'pending', 'validating', 'installing', 'configuring', 'health_check', 'rolling_back'].includes(d.status)).length

  const appKpis = [
    { label: 'Total Pipelines', value: String(pipelines.length), icon: <Layers size={18} />, color: '#6366f1', iconCls: 'bg-[rgba(99,102,241,0.15)] text-[#6366f1]', bar: 100 },
    { label: 'Pipeline Success / Failed', value: `${successPipelines} / ${failedPipelines}`, icon: <CheckCircle size={18} />, color: '#22c55e', iconCls: 'bg-emerald-500/15 text-emerald-400', bar: pipelines.length ? Math.round((successPipelines / pipelines.length) * 100) : 0 },
    { label: 'Total Deployments', value: String(deployments.length), icon: <GitBranch size={18} />, color: '#f59e0b', iconCls: 'bg-amber-500/15 text-amber-400', bar: 100 },
    { label: 'Running Deployments', value: String(runningDeployments), icon: <Activity size={18} />, color: '#10b981', iconCls: 'bg-[rgba(16,185,129,0.15)] text-[#10b981]', bar: deployments.length ? Math.round((runningDeployments / deployments.length) * 100) : 0 },
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
      <div className="mb-5 grid grid-cols-1 gap-3.5">
        <ChartPanel title="Deployment Timeline">
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={timeline}>
              <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <YAxis stroke="#94a3b8" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend wrapperStyle={{ color: '#e5e7eb', fontSize: 11 }} />
              <Bar dataKey="success" fill="#22c55e" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
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
          <span className="text-xs text-[var(--color-text-secondary)]">{rows.length} apps</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[var(--color-border-default)] text-[11px] text-[var(--color-text-secondary)]">
                {['Application', 'Version', 'Pipeline', 'Status', 'Pods', 'Cluster', 'Namespace', 'Duration', 'Last Deploy'].map((h) => (
                  <th key={h} className="px-4 py-2.5 text-left font-semibold tracking-[0.03em]">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((app, i) => {
                const sc = APP_STATUS_CFG[app.status]
                const isLast = i === rows.length - 1
                return (
                  <tr key={app.name}
                    className={cn('transition-colors hover:bg-[rgba(255,255,255,0.02)]', !isLast && 'border-b border-[var(--color-border-default)]')}>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={cn('h-2 w-2 shrink-0 rounded-full', sc.dot)} />
                        <span className="font-semibold text-[var(--color-text-primary)]">{app.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">{app.version}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <GitBranch size={11} />{app.pipeline}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', sc.cls)}>{sc.label}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn(
                        'font-mono',
                        typeof app.pods[0] === 'number' && typeof app.pods[1] === 'number' && app.pods[0] < app.pods[1]
                          ? 'text-amber-400'
                          : 'text-[var(--color-text-primary)]',
                      )}>
                        {typeof app.pods[0] === 'number' && typeof app.pods[1] === 'number' ? `${app.pods[0]}/${app.pods[1]}` : '—'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-[var(--color-text-secondary)]">{app.cluster}</td>
                    <td className="px-4 py-3 text-[var(--color-text-secondary)]">{app.namespace}</td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">{app.duration}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-[var(--color-text-secondary)]">
                        <Clock size={11} />{app.lastDeploy}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
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
          {latestDeployments.map((d) => (
            <div key={`${d.pipelineName}-${d.startedAt}`} className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-3">
              <div className="flex items-center gap-2">
                {d.status === 'success'
                  ? <CheckCircle size={13} className="text-emerald-400" />
                  : d.status === 'failed'
                    ? <XCircle size={13} className="text-red-400" />
                    : <AlertCircle size={13} className="text-amber-400" />}
                <span className="font-semibold text-[var(--color-text-primary)]">{d.pipelineName}</span>
              </div>
              <span className="font-mono text-[11px] text-[var(--color-text-secondary)]">{d.version}</span>
              <div className="flex items-center gap-1 text-[11px] text-[var(--color-text-secondary)]">
                <Clock size={10} />{formatDuration(d.startedAt, d.completedAt)}
              </div>
              <span className="ml-auto text-[11px] text-[var(--color-text-secondary)]">{timeAgo(d.startedAt)}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

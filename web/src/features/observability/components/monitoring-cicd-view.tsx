import { useState, useMemo } from "react"
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts"
import { Activity, AlertCircle, CheckCircle, Clock, GitBranch, Layers, Package, XCircle } from "lucide-react"
import { useDeployments, usePipelines } from "../../cicd/api/cicd-api"
import { useStackWorkloads } from "../../stack/api/stack-api"
import { cn } from "../../../lib/utils"
import { CHART_LEGEND_PROPS, CHART_STYLE, KpiCard, ChartPanel } from "./monitoring-chart-widgets"
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

export function CicdDefault({
  selectedClusterId,
  selectedStackId,
}: {
  selectedClusterId: string
  selectedStackId: string
}) {
  const [range, setRange] = useState<TimeRange>('24h')
  const { data: pipelinesData } = usePipelines()
  const { data: deploymentsData } = useDeployments()
  // 배포된 앱의 파드는 클러스터에서 읽는다. 스택 단위 엔드포인트라 스택을 골라야
  // 나온다 — 그래서 아래에서 스택 미선택 상태를 먼저 처리한다.
  const { data: workloads } = useStackWorkloads(selectedStackId)

  // 파이프라인 id 로 실제 파드 수를 찾는다.
  const podsByPipeline = useMemo(() => {
    const map = new Map<string, [number, number]>()
    for (const pipeline of workloads?.pipelines ?? []) {
      const pods = pipeline.k8sObjects.filter((object) => object.kind === 'Pod')
      const ready = pods.filter((pod) => pod.status === 'Running').length
      map.set(pipeline.id, [ready, pods.length])
    }
    return map
  }, [workloads?.pipelines])

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
      pods: podsByPipeline.get(pipeline.id) ?? [null, null],
      cluster: pipeline.clusterName || '—',
      namespace: pipeline.namespace || 'default',
      duration: formatDuration(latest?.startedAt ?? null, latest?.completedAt ?? null),
      lastDeploy: timeAgo(latest?.startedAt ?? null),
    }
  }), [pipelines, latestByPipeline, podsByPipeline])

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
    { label: 'Total Pipelines', value: String(pipelines.length), icon: <Layers size={18} />, color: 'var(--color-primary)', iconCls: 'bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] text-[var(--color-primary)]', bar: 100 },
    { label: 'Pipeline Success / Failed', value: `${successPipelines} / ${failedPipelines}`, icon: <CheckCircle size={18} />, color: 'var(--color-success)', iconCls: 'bg-emerald-500/15 text-emerald-400', bar: pipelines.length ? Math.round((successPipelines / pipelines.length) * 100) : 0 },
    { label: 'Total Deployments', value: String(deployments.length), icon: <GitBranch size={18} />, color: 'var(--color-warning)', iconCls: 'bg-amber-500/15 text-amber-400', bar: 100 },
    { label: 'Running Deployments', value: String(runningDeployments), icon: <Activity size={18} />, color: 'var(--color-success)', iconCls: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]', bar: deployments.length ? Math.round((runningDeployments / deployments.length) * 100) : 0 },
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
                  ? 'border-[color-mix(in_srgb,_var(--color-warning)_60%,_transparent)] bg-[color-mix(in_srgb,_var(--color-warning)_20%,_transparent)] text-[var(--color-warning)]'
                  : 'border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]')}>
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
              <XAxis dataKey="time" stroke="var(--color-text-secondary)" tick={CHART_STYLE.tick} />
              <YAxis stroke="var(--color-text-secondary)" tick={CHART_STYLE.tick} />
              <Tooltip contentStyle={CHART_STYLE.tooltip} />
              <Legend {...CHART_LEGEND_PROPS} />
              <Bar dataKey="success" fill="var(--color-success)" radius={[4, 4, 0, 0]} />
              <Bar dataKey="failed" fill="var(--color-error)" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </ChartPanel>
      </div>

      {/* Application table */}
      <div className="mb-5 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-4 py-3">
          <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
            <Package size={15} className="text-[var(--color-primary)]" />
            Deployed Applications
          </h2>
          <span className="text-xs text-[var(--color-text-secondary)]">{rows.length} apps</span>
        </div>
        {!selectedStackId && (
          // 파드 수는 스택 단위 엔드포인트에서 온다. 안 고르면 그 열만 비므로,
          // 표가 고장난 것처럼 보이지 않게 이유를 적는다.
          <div className="mb-3 rounded-[var(--radius-sm)] border border-[var(--color-border-default)] px-3 py-2 text-[12px] text-[var(--color-text-secondary)]">
            위에서 스택을 고르면 배포된 애플리케이션의 실제 파드 수를 함께 보여줍니다.
          </div>
        )}
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
                    className={cn('transition-colors hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]', !isLast && 'border-b border-[var(--color-border-default)]')}>
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
                        {typeof app.pods[0] === 'number' && typeof app.pods[1] === 'number'
                          ? `${app.pods[0]}/${app.pods[1]}`
                          : selectedStackId
                            ? '—'
                            : 'select stack'}
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
            <GitBranch size={15} className="text-[var(--color-primary)]" />
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

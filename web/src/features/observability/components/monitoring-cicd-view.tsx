import { useState, useMemo, useEffect, useRef } from "react"
import { BarChart, Bar, Line, LineChart, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts"
import { Activity, AlertCircle, CheckCircle, Clock, GitBranch, Layers, Package, ScrollText, XCircle } from "lucide-react"
import { useDeployments, usePipelines } from "../../cicd/api/cicd-api"
import { useStackWorkloadLogs, useStackWorkloads } from "../../stack/api/stack-api"
import { cn } from "../../../lib/utils"
import { CHART_LEGEND_PROPS, CHART_STYLE, KpiCard, ChartPanel } from "./monitoring-chart-widgets"
import type { TimeRange } from "./monitoring-tab-layout"
import type { EmbedTab } from "../utils/monitoring-utils"
import { formatDuration, timeAgo } from "../utils/monitoring-utils"
import { appsWithUsage, pushSample, sampleAppUsage, usageSeries } from "../utils/cicd-usage-series"
import type { UsageRow, UsageSample } from "../utils/cicd-usage-series"
import type { StackWorkloadLogLine } from "../../../types"

/** 앱마다 선 하나. 색은 이름 순서로 고정한다 — 폴링마다 바뀌면 읽을 수 없다. */
const USAGE_LINE_COLORS = [
  'var(--color-primary)',
  'var(--color-success)',
  'var(--color-warning)',
  'var(--color-info)',
  'var(--color-error)',
]

/** 실시간 그래프라 스택 모니터링과 같은 주기로 읽는다. */
const USAGE_POLL_MS = 5_000

/** 표본이 없을 때 쓰는 고정 참조. 매번 [] 를 새로 만들면 useMemo 가 무의미해진다. */
const EMPTY_SAMPLES: UsageSample[] = []

/**
 * 로그는 파드마다 요청이 하나씩 나가므로 지표보다 느리게 읽는다.
 * 꼬리 줄 수는 패널 높이에 맞춰 넉넉히 — 스크롤로 거슬러 볼 만큼은 있어야 한다.
 */
const LOG_POLL_MS = 10_000
const LOG_TAIL_LINES = 200

const EMPTY_LOG_LINES: StackWorkloadLogLine[] = []

/** 파드 이름의 마지막 구간. "demo-app-5599f6cfc-xgcmc" → "xgcmc" */
function podSuffix(pod: string): string {
  const parts = pod.split('-')
  return parts[parts.length - 1] || pod
}

// ─── Default content: CI/CD view ─────────────────────────────────────────────
// ─── CI/CD Application monitoring data ───────────────────────────────────────
type AppStatus = 'healthy' | 'degraded' | 'down'

interface DeployedAppRow {
  name: string
  version: string
  pipeline: string
  status: AppStatus
  pods: [number | null, number | null]
  /** 앱 파드들의 실사용량 합. metrics-server 가 없으면 null 이다. */
  cpuMillicores: number | null
  memoryMib: number | null
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

/**
 * UsageChart 는 배포된 앱들의 자원 사용을 시간축에 그린다.
 *
 * 빈 상태를 세 가지로 나눠 말한다. 셋 다 "선이 없다" 로 보이지만 사용자가 할 일이
 * 다르다 — 스택을 고르거나, 기다리거나, metrics-server 를 깔아야 한다.
 */
function UsageChart({
  title,
  unit,
  data,
  apps,
  hasStack,
}: {
  title: string
  unit: string
  data: UsageRow[]
  apps: string[]
  hasStack: boolean
}) {
  const emptyReason = !hasStack
    ? '위에서 스택을 고르면 배포된 앱의 자원 사용을 실시간으로 그립니다.'
    : data.length === 0
      ? '첫 표본을 기다리는 중입니다.'
      : '사용량을 읽지 못했습니다. 클러스터에 metrics-server 가 있는지 확인하세요.'

  return (
    <ChartPanel title={title}>
      {apps.length === 0 ? (
        <div className="flex h-[200px] items-center justify-center px-4 text-center text-[12px] text-[var(--color-text-secondary)]">
          {emptyReason}
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
            <XAxis dataKey="time" stroke="var(--color-text-secondary)" tick={CHART_STYLE.tick} minTickGap={32} />
            <YAxis
              stroke="var(--color-text-secondary)"
              tick={CHART_STYLE.tick}
              width={52}
              tickFormatter={(value: number) => `${value}${unit}`}
            />
            <Tooltip
              contentStyle={CHART_STYLE.tooltip}
              formatter={(value) => (typeof value === 'number' ? `${value}${unit}` : '—')}
            />
            <Legend {...CHART_LEGEND_PROPS} />
            {apps.map((app, index) => (
              <Line
                key={app}
                type="monotone"
                dataKey={app}
                name={app}
                stroke={USAGE_LINE_COLORS[index % USAGE_LINE_COLORS.length]}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3 }}
                // 못 읽은 시점은 선을 끊는다. 이으면 없는 값을 지어내는 것이다.
                connectNulls={false}
                isAnimationActive={false}
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      )}
    </ChartPanel>
  )
}

/**
 * AppLogPanel 은 배포된 앱 컨테이너가 뱉은 로그를 시간순으로 보여준다.
 *
 * 지표가 "얼마나 쓰는가" 를 말한다면 로그는 "무엇을 하다 죽었는가" 를 말한다.
 * 지표만 보고는 앱이 왜 그런 상태인지 알 수 없어 결국 kubectl 로 넘어가게 된다.
 *
 * 파드별로 나누지 않고 섞는다 — 나눠 두면 요청이 어느 파드로 갔는지를 사람이
 * 맞춰 봐야 한다. 대신 줄마다 파드 꼬리를 붙여 출처를 알 수 있게 한다.
 */
function AppLogPanel({ stackId }: { stackId: string }) {
  const { data, isLoading } = useStackWorkloadLogs(stackId, LOG_TAIL_LINES, LOG_POLL_MS)
  const [follow, setFollow] = useState(true)
  const viewportRef = useRef<HTMLDivElement | null>(null)

  // useMemo 로 감싸야 빈 배열 리터럴이 매 렌더 새 참조가 되어 아래 이펙트를
  // 계속 깨우지 않는다.
  const lines = useMemo(() => data?.lines ?? EMPTY_LOG_LINES, [data?.lines])

  // 컨테이너를 직접 내린다. scrollIntoView 는 바깥 페이지까지 움직여
  // 로그가 갱신될 때마다 화면이 튄다.
  useEffect(() => {
    if (!follow) return
    const viewport = viewportRef.current
    if (!viewport) return
    viewport.scrollTop = viewport.scrollHeight
  }, [lines, follow])

  return (
    // min-h-0 + overflow-hidden 이라야 로그 줄 수가 그리드 행 높이를 밀지 않는다.
    // 없으면 로그가 길어질수록 옆 차트까지 같이 늘어나 빈 공간만 커진다.
    // min-h-[320px] 은 한 칸으로 접히는 좁은 화면에서의 바닥값이다.
    <div className="flex h-full min-h-[320px] flex-col overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
      <div className="flex items-center justify-between gap-2 border-b border-[var(--color-border-default)] px-4 py-3">
        <h2 className="flex items-center gap-2 text-[14px] font-bold text-[var(--color-text-primary)]">
          <ScrollText size={15} className="text-[var(--color-primary)]" />
          Application Logs
        </h2>
        <div className="flex items-center gap-2">
          {data?.pods?.length ? (
            <span className="text-[11px] text-[var(--color-text-secondary)]">
              {data.pods.length} pods{data.truncated ? ' (일부)' : ''}
            </span>
          ) : null}
          <button
            type="button"
            onClick={() => setFollow((prev) => !prev)}
            className={cn(
              'rounded-[7px] border px-2 py-[3px] text-[11px] font-bold',
              follow
                ? 'border-[color-mix(in_srgb,_var(--color-success)_60%,_transparent)] bg-[color-mix(in_srgb,_var(--color-success)_18%,_transparent)] text-[var(--color-success)]'
                : 'border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]',
            )}
          >
            Follow
          </button>
        </div>
      </div>

      {!stackId ? (
        <div className="flex flex-1 items-center justify-center px-4 text-center text-[12px] text-[var(--color-text-secondary)]">
          위에서 스택을 고르면 배포된 앱의 로그를 보여줍니다.
        </div>
      ) : lines.length === 0 ? (
        <div className="flex flex-1 items-center justify-center px-4 text-center text-[12px] text-[var(--color-text-secondary)]">
          {isLoading ? '로그를 읽는 중입니다.' : '아직 출력한 로그가 없습니다.'}
        </div>
      ) : (
        <div
          ref={viewportRef}
          className="min-h-0 flex-1 overflow-auto bg-[var(--color-surface-sunken)] px-3 py-2 font-mono text-[11px] leading-[1.6]"
        >
          {lines.map((line, index) => (
            <div
              key={`${line.pod}-${line.timestamp}-${index}`}
              className="flex gap-2 whitespace-pre-wrap break-all"
            >
              <span className="shrink-0 text-[var(--color-text-muted)]">
                {line.timestamp ? line.timestamp.slice(11, 19) : '--:--:--'}
              </span>
              {/* 파드 이름 마지막 구간이면 같은 앱의 파드끼리 구분된다.
                  전체 이름(demo-app-5599f6cfc-xgcmc)은 로그 폭을 다 먹는다. */}
              <span className="shrink-0 text-[var(--color-primary)]">{podSuffix(line.pod)}</span>
              <span className="text-[var(--color-text-secondary)]">{line.message}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

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
  const { data: workloads, dataUpdatedAt } = useStackWorkloads(selectedStackId, USAGE_POLL_MS)

  // 백엔드는 "지금" 만 준다. 스택 모니터링과 같이 폴링 결과를 화면에서 쌓아
  // 실시간 그래프를 만든다. dataUpdatedAt 로 걸어야 값이 그대로인 폴링에서도
  // 한 점이 찍힌다 — 안 그러면 평평한 구간이 통째로 사라진다.
  //
  // 표본에 스택 id 를 함께 담는다. 초기화를 별도 이펙트로 두면 마운트 때 두
  // 이펙트가 같이 돌아 방금 담은 첫 표본을 지운다 — 순서에 기대지 않는다.
  const [collected, setCollected] = useState<{ stackId: string; samples: UsageSample[] }>({
    stackId: selectedStackId,
    samples: [],
  })

  useEffect(() => {
    if (!workloads?.pipelines) return
    const next = sampleAppUsage(workloads.pipelines, dataUpdatedAt)
    setCollected((prev) =>
      prev.stackId === selectedStackId
        ? { stackId: selectedStackId, samples: pushSample(prev.samples, next) }
        : { stackId: selectedStackId, samples: [next] },
    )
  }, [workloads?.pipelines, dataUpdatedAt, selectedStackId])

  // 스택을 바꾼 직후 새 데이터가 오기 전까지는 앞 스택의 선을 보여주지 않는다.
  // useMemo 로 감싸야 빈 배열 리터럴이 매 렌더 새 참조가 되지 않는다.
  const usageSamples = useMemo(
    () => (collected.stackId === selectedStackId ? collected.samples : EMPTY_SAMPLES),
    [collected, selectedStackId],
  )

  const cpuApps = useMemo(() => appsWithUsage(usageSamples, 'cpu'), [usageSamples])
  const memoryApps = useMemo(() => appsWithUsage(usageSamples, 'memory'), [usageSamples])
  const cpuSeries = useMemo(() => usageSeries(usageSamples, 'cpu', cpuApps), [usageSamples, cpuApps])
  const memorySeries = useMemo(() => usageSeries(usageSamples, 'memory', memoryApps), [usageSamples, memoryApps])

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

  // 앱이 실제로 쓰는 자원은 그 앱 파드들의 합이다.
  //
  // 한 파드라도 값을 읽었으면 합을 보여준다. 하나도 못 읽었으면 null 이다 —
  // 0 으로 두면 metrics-server 가 없는 클러스터에서 "안 쓰는 앱" 으로 보인다.
  const usageByPipeline = useMemo(() => {
    const map = new Map<string, { cpu: number | null; memory: number | null }>()
    for (const pipeline of workloads?.pipelines ?? []) {
      const pods = pipeline.k8sObjects.filter((object) => object.kind === 'Pod')
      const sum = (pick: (pod: (typeof pods)[number]) => number | null | undefined) => {
        const values = pods.map(pick).filter((v): v is number => typeof v === 'number')
        return values.length > 0 ? values.reduce((a, b) => a + b, 0) : null
      }
      map.set(pipeline.id, { cpu: sum((pod) => pod.cpuMillicores), memory: sum((pod) => pod.memoryMib) })
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
      cpuMillicores: usageByPipeline.get(pipeline.id)?.cpu ?? null,
      memoryMib: usageByPipeline.get(pipeline.id)?.memory ?? null,
      cluster: pipeline.clusterName || '—',
      namespace: pipeline.namespace || 'default',
      duration: formatDuration(latest?.startedAt ?? null, latest?.completedAt ?? null),
      lastDeploy: timeAgo(latest?.startedAt ?? null),
    }
  }), [pipelines, latestByPipeline, podsByPipeline, usageByPipeline])

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

      {/* 왼쪽은 배포된 앱의 실시간 자원 사용(앱마다 선 하나), 오른쪽은 그 앱들의
          로그다. 지표만으로는 앱이 왜 그 상태인지 알 수 없어 결국 kubectl 로
          넘어가게 된다 — 같은 화면에서 함께 읽히도록 나란히 둔다. */}
      {/* 행 높이를 못박는다. 안 그러면 로그 줄 수가 행 높이를 밀고, 왼쪽 중첩
          그리드가 그 높이를 둘로 나눠 차트 패널이 빈 공간째로 늘어난다. */}
      <div className="mb-5 grid grid-cols-1 gap-3.5 xl:h-[544px] xl:grid-cols-2">
        <div className="grid min-h-0 grid-cols-1 gap-3.5 xl:grid-rows-2">
          <UsageChart
            title="App CPU (Live)"
            unit="m"
            data={cpuSeries}
            apps={cpuApps}
            hasStack={!!selectedStackId}
          />
          <UsageChart
            title="App Memory (Live)"
            unit="Mi"
            data={memorySeries}
            apps={memoryApps}
            hasStack={!!selectedStackId}
          />
        </div>
        <AppLogPanel stackId={selectedStackId} />
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
                {['Application', 'Version', 'Pipeline', 'Status', 'Pods', 'CPU', 'Memory', 'Cluster', 'Namespace', 'Duration', 'Last Deploy'].map((h) => (
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
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">
                      {typeof app.cpuMillicores === 'number' ? `${app.cpuMillicores}m` : '—'}
                    </td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">
                      {typeof app.memoryMib === 'number' ? `${app.memoryMib}Mi` : '—'}
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

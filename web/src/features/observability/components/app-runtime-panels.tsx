// 배포된 앱의 런타임 상태 패널 — 자원 사용 그래프와 컨테이너 로그.
//
// 모니터링 대시보드(스택 단위)와 CI/CD 목록 상세(파이프라인 단위)가 같은 것을
// 봐야 하므로 여기 한 벌만 둔다. 보는 범위만 apps 로 좁힌다.
//
// 백엔드는 "지금" 만 준다. 폴링 결과를 화면에서 쌓아 실시간 그래프를 만들고,
// 값을 못 읽은 시점은 0 이 아니라 선을 끊어 "모름" 을 그대로 보인다.

import { useEffect, useMemo, useRef, useState } from 'react'
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { ScrollText } from 'lucide-react'
import { useStackWorkloadLogs, useStackWorkloads } from '../../stack/api/stack-api'
import { cn } from '../../../lib/utils'
import { CHART_LEGEND_PROPS, CHART_STYLE, ChartPanel } from './monitoring-chart-widgets'
import { appsWithUsage, pushSample, sampleAppUsage, usageSeries } from '../utils/cicd-usage-series'
import type { UsageRow, UsageSample } from '../utils/cicd-usage-series'
import type { StackWorkloadLogLine } from '../../../types'

/** 앱마다 선 하나. 색은 이름 순서로 고정한다 — 폴링마다 바뀌면 읽을 수 없다. */
const USAGE_LINE_COLORS = [
  'var(--color-primary)',
  'var(--color-success)',
  'var(--color-warning)',
  'var(--color-info)',
  'var(--color-error)',
]

/** 실시간 그래프라 스택 모니터링과 같은 주기로 읽는다. */
export const USAGE_POLL_MS = 5_000

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

/**
 * 스택이 없을 때의 안내는 화면마다 다르다. 대시보드는 위에서 고르면 되지만,
 * 파이프라인 상세에서는 고를 수 있는 것이 없고 파이프라인이 스택에 연결돼야 한다.
 */
const NO_STACK_HINT = {
  usage: {
    pick: '위에서 스택을 고르면 배포된 앱의 자원 사용을 실시간으로 그립니다.',
    linked: '이 파이프라인이 스택에 연결되어야 자원 사용을 그릴 수 있습니다.',
  },
  logs: {
    pick: '위에서 스택을 고르면 배포된 앱의 로그를 보여줍니다.',
    linked: '이 파이프라인이 스택에 연결되어야 로그를 읽을 수 있습니다.',
  },
}

/**
 * UsageChart 는 배포된 앱들의 자원 사용을 시간축에 그린다.
 *
 * 빈 상태를 세 가지로 나눠 말한다. 셋 다 "선이 없다" 로 보이지만 사용자가 할 일이
 * 다르다 — 스택을 잇거나, 기다리거나, metrics-server 를 깔아야 한다.
 */
function UsageChart({
  title,
  unit,
  data,
  apps,
  hasStack,
  noStackHint,
}: {
  title: string
  unit: string
  data: UsageRow[]
  apps: string[]
  hasStack: boolean
  noStackHint: string
}) {
  const emptyReason = !hasStack
    ? noStackHint
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
 * AppUsageCharts 는 CPU/메모리 두 장을 함께 그린다.
 *
 * apps 를 주면 그 앱들만 본다 — 파이프라인 상세에서 옆 앱의 선이 섞이면
 * 어느 것이 이 앱인지 알 수 없다.
 */
export function AppUsageCharts({
  stackId,
  apps,
  linkedHint = false,
}: {
  stackId: string
  apps?: string[]
  /** 스택이 없을 때의 안내를 "파이프라인이 스택에 연결되어야" 로 바꾼다. */
  linkedHint?: boolean
}) {
  const { data: workloads, dataUpdatedAt } = useStackWorkloads(stackId, USAGE_POLL_MS)

  // 표본에 스택 id 를 함께 담는다. 초기화를 별도 이펙트로 두면 마운트 때 두
  // 이펙트가 같이 돌아 방금 담은 첫 표본을 지운다 — 순서에 기대지 않는다.
  const [collected, setCollected] = useState<{ stackId: string; samples: UsageSample[] }>({
    stackId,
    samples: [],
  })

  const pipelines = useMemo(() => {
    const all = workloads?.pipelines ?? []
    return apps ? all.filter((pipeline) => apps.includes(pipeline.name)) : all
  }, [workloads?.pipelines, apps])

  useEffect(() => {
    if (!workloads?.pipelines) return
    const next = sampleAppUsage(pipelines, dataUpdatedAt)
    setCollected((prev) =>
      prev.stackId === stackId
        ? { stackId, samples: pushSample(prev.samples, next) }
        : { stackId, samples: [next] },
    )
  }, [workloads?.pipelines, pipelines, dataUpdatedAt, stackId])

  // 스택을 바꾼 직후 새 데이터가 오기 전까지는 앞 스택의 선을 보여주지 않는다.
  const samples = useMemo(
    () => (collected.stackId === stackId ? collected.samples : EMPTY_SAMPLES),
    [collected, stackId],
  )

  const cpuApps = useMemo(() => appsWithUsage(samples, 'cpu'), [samples])
  const memoryApps = useMemo(() => appsWithUsage(samples, 'memory'), [samples])
  const cpuSeries = useMemo(() => usageSeries(samples, 'cpu', cpuApps), [samples, cpuApps])
  const memorySeries = useMemo(() => usageSeries(samples, 'memory', memoryApps), [samples, memoryApps])

  const hint = linkedHint ? NO_STACK_HINT.usage.linked : NO_STACK_HINT.usage.pick

  return (
    <>
      <UsageChart title="App CPU (Live)" unit="m" data={cpuSeries} apps={cpuApps} hasStack={!!stackId} noStackHint={hint} />
      <UsageChart title="App Memory (Live)" unit="Mi" data={memorySeries} apps={memoryApps} hasStack={!!stackId} noStackHint={hint} />
    </>
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
export function AppLogPanel({
  stackId,
  apps,
  linkedHint = false,
}: {
  stackId: string
  apps?: string[]
  linkedHint?: boolean
}) {
  const { data, isLoading } = useStackWorkloadLogs(stackId, LOG_TAIL_LINES, LOG_POLL_MS)
  const [follow, setFollow] = useState(true)
  const viewportRef = useRef<HTMLDivElement | null>(null)

  // useMemo 로 감싸야 빈 배열 리터럴이 매 렌더 새 참조가 되어 아래 이펙트를
  // 계속 깨우지 않는다.
  const lines = useMemo(() => {
    const all = data?.lines ?? EMPTY_LOG_LINES
    return apps ? all.filter((line) => apps.includes(line.app)) : all
  }, [data?.lines, apps])

  // 컨테이너를 직접 내린다. scrollIntoView 는 바깥 페이지까지 움직여
  // 로그가 갱신될 때마다 화면이 튄다.
  useEffect(() => {
    if (!follow) return
    const viewport = viewportRef.current
    if (!viewport) return
    viewport.scrollTop = viewport.scrollHeight
  }, [lines, follow])

  const podCount = apps
    ? new Set(lines.map((line) => line.pod)).size
    : (data?.pods?.length ?? 0)

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
          {podCount > 0 ? (
            <span className="text-[11px] text-[var(--color-text-secondary)]">
              {podCount} pods{!apps && data?.truncated ? ' (일부)' : ''}
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
          {linkedHint ? NO_STACK_HINT.logs.linked : NO_STACK_HINT.logs.pick}
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

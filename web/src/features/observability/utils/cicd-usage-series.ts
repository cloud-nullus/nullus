// CI/CD 로 배포한 앱의 자원 사용 시계열.
//
// 백엔드는 "지금 이 순간" 만 준다 (/workloads 가 metrics-server 를 그때그때 읽는다).
// 스택 모니터링과 같은 방식으로, 폴링 결과를 화면에서 쌓아 실시간 그래프를 만든다.
// 지난 값을 서버에 저장하지 않으므로 화면을 떠나면 사라진다 — 장기 추세는
// Grafana 탭의 몫이다.
//
// 값을 못 읽은 시점은 0 이 아니라 null 이다. 0 으로 이으면 선이 바닥을 기어
// "안 쓰는 앱" 으로 읽히는데, 실제로는 metrics-server 가 없거나 파드가 아직
// 안 떠서 모르는 것이다. null 이면 선이 끊겨 "모름" 이 그대로 보인다.

import type { StackWorkloadPipeline } from '../../../types'

/** 실시간 창에 유지하는 표본 수. 5초 폴링 기준 약 5분치다. */
export const REALTIME_POINTS = 60

export interface AppUsage {
  cpu: number | null
  memory: number | null
}

/** 한 시점의 앱별 사용량. */
export interface UsageSample {
  ts: number
  byApp: Record<string, AppUsage>
}

export type UsageMetric = 'cpu' | 'memory'

/**
 * sampleAppUsage 는 한 번의 조회 결과를 한 시점의 표본으로 접는다.
 *
 * 앱이 쓰는 자원은 그 앱 파드들의 합이다. 읽힌 파드가 하나도 없으면 null 이고,
 * 일부만 읽혔으면 읽힌 것만 더한다 — 전부 버리면 정보를 잃는다.
 */
export function sampleAppUsage(pipelines: StackWorkloadPipeline[], ts: number): UsageSample {
  const byApp: Record<string, AppUsage> = {}

  for (const pipeline of pipelines) {
    const pods = pipeline.k8sObjects.filter((object) => object.kind === 'Pod')
    if (pods.length === 0) continue

    const sum = (pick: (pod: (typeof pods)[number]) => number | null | undefined) => {
      const values = pods.map(pick).filter((value): value is number => typeof value === 'number')
      return values.length > 0 ? values.reduce((a, b) => a + b, 0) : null
    }

    byApp[pipeline.name] = {
      cpu: sum((pod) => pod.cpuMillicores),
      memory: sum((pod) => pod.memoryMib),
    }
  }

  return { ts, byApp }
}

/** pushSample 은 표본을 붙이고 실시간 창 밖으로 밀려난 것을 버린다. */
export function pushSample(samples: UsageSample[], next: UsageSample): UsageSample[] {
  return [...samples, next].slice(-REALTIME_POINTS)
}

/**
 * appsWithUsage 는 그래프에 선을 그릴 앱을 고른다.
 *
 * 한 번도 값을 못 읽은 앱은 뺀다 — 범례만 차지하고 선이 없다.
 * 순서는 이름 순으로 고정한다. 매 폴링마다 순서가 바뀌면 선 색이 춤춘다.
 */
export function appsWithUsage(samples: UsageSample[], metric: UsageMetric): string[] {
  const seen = new Set<string>()
  for (const sample of samples) {
    for (const [app, usage] of Object.entries(sample.byApp)) {
      if (typeof usage[metric] === 'number') seen.add(app)
    }
  }
  return [...seen].sort()
}

/** 차트 한 행. time 은 축 라벨, 나머지 키는 앱 이름이다. */
export type UsageRow = Record<string, string | number | null>

/** usageSeries 는 표본들을 recharts 가 먹는 행 배열로 편다. */
export function usageSeries(samples: UsageSample[], metric: UsageMetric, apps: string[]): UsageRow[] {
  return samples.map((sample) => {
    const row: UsageRow = { time: formatClock(sample.ts) }
    for (const app of apps) {
      const value = sample.byApp[app]?.[metric]
      row[app] = typeof value === 'number' ? value : null
    }
    return row
  })
}

/** 실시간 창은 5분 남짓이라 시:분:초까지 보여야 변화를 읽을 수 있다. */
function formatClock(ts: number): string {
  const date = new Date(ts)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

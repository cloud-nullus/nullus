import { describe, expect, it } from 'vitest'
import { REALTIME_POINTS, appsWithUsage, pushSample, sampleAppUsage, usageSeries } from './cicd-usage-series'
import type { UsageSample } from './cicd-usage-series'
import type { StackWorkloadPipeline } from '../../../types'

function pipeline(name: string, pods: Array<{ cpu: number | null; memory: number | null }>): StackWorkloadPipeline {
  return {
    id: `pl-${name}`,
    name,
    namespace: 'apps',
    status: 'success',
    lastDeployment: null,
    k8sObjects: [
      { kind: 'Deployment', name, namespace: 'apps', status: 'running', replicas: pods.length },
      ...pods.map((usage, index) => ({
        kind: 'Pod',
        name: `${name}-${index}`,
        namespace: 'apps',
        status: 'Running',
        cpuMillicores: usage.cpu,
        memoryMib: usage.memory,
      })),
    ],
  }
}

describe('sampleAppUsage', () => {
  // 한 앱이 파드 여럿이면 그 앱이 쓰는 자원은 파드들의 합이다.
  it('앱별로 파드 사용량을 합산한다', () => {
    const sample = sampleAppUsage(
      [pipeline('demo-app', [{ cpu: 37, memory: 128 }, { cpu: 13, memory: 96 }])],
      1000,
    )

    expect(sample.ts).toBe(1000)
    expect(sample.byApp['demo-app']).toEqual({ cpu: 50, memory: 224 })
  })

  // metrics-server 가 없거나 아직 안 긁은 파드는 null 로 온다. 0 으로 접으면
  // 그래프가 바닥을 기어 "안 쓰는 앱" 으로 읽힌다 — 선이 끊겨야 맞다.
  it('사용량을 못 읽으면 0 이 아니라 null 이다', () => {
    const sample = sampleAppUsage([pipeline('demo-app', [{ cpu: null, memory: null }])], 1000)

    expect(sample.byApp['demo-app']).toEqual({ cpu: null, memory: null })
  })

  // 일부만 읽힌 경우까지 null 로 버리면 정보를 잃는다. 읽힌 것만 더한다.
  it('일부 파드만 읽혔으면 읽힌 것만 더한다', () => {
    const sample = sampleAppUsage(
      [pipeline('demo-app', [{ cpu: 20, memory: 64 }, { cpu: null, memory: null }])],
      1000,
    )

    expect(sample.byApp['demo-app']).toEqual({ cpu: 20, memory: 64 })
  })

  // 아직 파드가 안 뜬 앱은 그래프에 낼 것이 없다.
  it('파드가 없는 앱은 넣지 않는다', () => {
    const sample = sampleAppUsage([pipeline('demo-app', [])], 1000)

    expect(sample.byApp).toEqual({})
  })
})

describe('pushSample', () => {
  // 실시간 그래프라 무한히 쌓으면 메모리도 축도 망가진다.
  it(`최근 ${REALTIME_POINTS}개만 남긴다`, () => {
    let samples: UsageSample[] = []
    for (let i = 0; i < REALTIME_POINTS + 20; i += 1) {
      samples = pushSample(samples, { ts: i, byApp: { 'demo-app': { cpu: i, memory: i } } })
    }

    expect(samples).toHaveLength(REALTIME_POINTS)
    // 오래된 것부터 버린다.
    expect(samples[0].ts).toBe(20)
    expect(samples[samples.length - 1].ts).toBe(REALTIME_POINTS + 19)
  })
})

describe('appsWithUsage', () => {
  // 한 번도 값을 못 읽은 앱은 범례만 차지하고 선이 없다.
  it('값이 한 번이라도 있었던 앱만 낸다', () => {
    const samples = [
      { ts: 1, byApp: { 'demo-app': { cpu: 10, memory: 20 }, 'demo-api': { cpu: null, memory: null } } },
      { ts: 2, byApp: { 'demo-app': { cpu: 12, memory: 22 }, 'demo-api': { cpu: null, memory: null } } },
    ]

    expect(appsWithUsage(samples, 'cpu')).toEqual(['demo-app'])
  })

  // 이름 순으로 고정한다. 매 폴링마다 순서가 바뀌면 선 색이 춤춘다.
  it('이름 순으로 고정한다', () => {
    const samples = [
      { ts: 1, byApp: { zeta: { cpu: 1, memory: 1 }, alpha: { cpu: 2, memory: 2 } } },
    ]

    expect(appsWithUsage(samples, 'cpu')).toEqual(['alpha', 'zeta'])
  })
})

describe('usageSeries', () => {
  it('시각 라벨과 앱별 값을 한 행으로 묶는다', () => {
    const rows = usageSeries(
      [{ ts: new Date('2026-08-12T10:20:30').getTime(), byApp: { 'demo-app': { cpu: 50, memory: 224 } } }],
      'memory',
      ['demo-app'],
    )

    expect(rows).toHaveLength(1)
    expect(rows[0]['demo-app']).toBe(224)
    expect(rows[0].time).toMatch(/20:30$/)
  })

  // 그 시점에 값을 못 읽었으면 null 이라 선이 끊긴다 — 0 으로 이으면 거짓이다.
  it('못 읽은 시점은 null 로 둔다', () => {
    const rows = usageSeries(
      [
        { ts: 1000, byApp: { 'demo-app': { cpu: 50, memory: 224 } } },
        { ts: 2000, byApp: {} },
      ],
      'cpu',
      ['demo-app'],
    )

    expect(rows[0]['demo-app']).toBe(50)
    expect(rows[1]['demo-app']).toBeNull()
  })
})

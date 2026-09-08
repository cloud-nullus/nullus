import { describe, it, expect } from 'vitest'
import {
  formatElapsed,
  jobPhaseOf,
  mergeJobNotifications,
  toJobNotifications,
  type JobNotification,
} from './job-notifications'
import type { Deployment, Stack } from '../../../types'

const stack = (overrides: Partial<Stack>): Stack => ({
  id: 's1',
  name: 'demo-stack',
  templateId: 'tpl',
  templateName: 'Template',
  clusterId: 'c1',
  clusterName: 'cluster',
  status: 'installing',
  createdAt: '2026-09-08T00:00:00Z',
  updatedAt: '2026-09-08T00:00:00Z',
  ...overrides,
})

const deployment = (overrides: Partial<Deployment>): Deployment => ({
  id: 'd1',
  pipelineId: 'p1',
  pipelineName: 'demo-pipeline',
  version: 'v1',
  status: 'running',
  triggeredBy: 'devops@nullus.dev',
  startedAt: '2026-09-08T00:00:00Z',
  completedAt: null,
  ...overrides,
})

describe('jobPhaseOf', () => {
  it.each(['pending', 'validating', 'installing', 'configuring', 'health_check', 'rolling_back', 'running'])(
    '%s 는 진행 중이다',
    (status) => {
      expect(jobPhaseOf(status)).toBe('running')
    },
  )

  it.each(['completed', 'success'])('%s 는 성공이다', (status) => {
    expect(jobPhaseOf(status)).toBe('succeeded')
  })

  it.each(['failed', 'cancelled', 'rolled_back'])('%s 는 실패다', (status) => {
    expect(jobPhaseOf(status)).toBe('failed')
  })

  // active/inactive 는 파이프라인의 '등록 상태'지 실행이 아니다. deleted 도 마찬가지로
  // 작업이 아니다 — 작업으로 세면 종에 끝나지 않는 항목이 남는다.
  it.each(['active', 'inactive', 'deleted', ''])('%s 는 작업이 아니다', (status) => {
    expect(jobPhaseOf(status)).toBeNull()
  })
})

describe('toJobNotifications', () => {
  it('스택과 배포를 한 목록으로 모은다', () => {
    const jobs = toJobNotifications(
      [stack({ id: 's1', status: 'installing' })],
      [deployment({ id: 'd1', status: 'running' })],
    )

    expect(jobs.map((job) => job.id)).toEqual(['stack:s1', 'deployment:d1'])
    expect(jobs[0]).toMatchObject({
      kind: 'stack',
      name: 'demo-stack',
      stage: 'installing',
      phase: 'running',
      detailPath: '/stack/logs/s1',
    })
    expect(jobs[1]).toMatchObject({
      kind: 'deployment',
      name: 'demo-pipeline',
      stage: 'running',
      phase: 'running',
      detailPath: '/cicd/pipelines/p1/logs?deploymentId=d1',
    })
  })

  it('작업이 아닌 상태는 버린다', () => {
    const jobs = toJobNotifications([stack({ id: 's9', status: 'deleted' })], [])
    expect(jobs).toEqual([])
  })

  it('목록이 없어도 견딘다', () => {
    expect(toJobNotifications(undefined, undefined)).toEqual([])
  })
})

describe('mergeJobNotifications', () => {
  const RETENTION = 60_000

  it('진행 중인 작업을 그대로 싣는다', () => {
    const incoming = toJobNotifications([stack({ id: 's1', status: 'installing' })], [])
    const merged = mergeJobNotifications([], incoming, 1_000, RETENTION)

    expect(merged).toHaveLength(1)
    expect(merged[0].phase).toBe('running')
    expect(merged[0].settledAt).toBeUndefined()
  })

  // 스택 목록에는 완료된 스택이 수십 개 들어 있다. 이번 세션에서 도는 것을 본 적
  // 없는 완료 항목까지 알림으로 세면 종은 늘 켜져 있고 목록은 이력 화면이 된다.
  it('진행 중인 것을 본 적 없는 완료 항목은 싣지 않는다', () => {
    const incoming = toJobNotifications([stack({ id: 's-old', status: 'completed' })], [])
    expect(mergeJobNotifications([], incoming, 1_000, RETENTION)).toEqual([])
  })

  it('진행 → 완료 전환을 잡아 전환 시각을 남긴다', () => {
    const running = mergeJobNotifications(
      [],
      toJobNotifications([stack({ id: 's1', status: 'installing' })], []),
      1_000,
      RETENTION,
    )
    const settled = mergeJobNotifications(
      running,
      toJobNotifications([stack({ id: 's1', status: 'completed' })], []),
      5_000,
      RETENTION,
    )

    expect(settled).toHaveLength(1)
    expect(settled[0].phase).toBe('succeeded')
    expect(settled[0].settledAt).toBe(5_000)
  })

  it('진행 → 실패 전환도 잡는다', () => {
    const running = mergeJobNotifications(
      [],
      toJobNotifications([], [deployment({ id: 'd1', status: 'running' })]),
      1_000,
      RETENTION,
    )
    const settled = mergeJobNotifications(
      running,
      toJobNotifications([], [deployment({ id: 'd1', status: 'failed' })]),
      3_000,
      RETENTION,
    )

    expect(settled[0].phase).toBe('failed')
    expect(settled[0].settledAt).toBe(3_000)
  })

  it('전환 시각을 다시 찍지 않는다', () => {
    const running = mergeJobNotifications(
      [],
      toJobNotifications([stack({ id: 's1', status: 'installing' })], []),
      1_000,
      RETENTION,
    )
    const first = mergeJobNotifications(
      running,
      toJobNotifications([stack({ id: 's1', status: 'completed' })], []),
      5_000,
      RETENTION,
    )
    const second = mergeJobNotifications(
      first,
      toJobNotifications([stack({ id: 's1', status: 'completed' })], []),
      9_000,
      RETENTION,
    )

    expect(second[0].settledAt).toBe(5_000)
  })

  it('보존 시간이 지난 완료 항목은 내린다', () => {
    const settled: JobNotification[] = [
      {
        id: 'stack:s1',
        kind: 'stack',
        refId: 's1',
        name: 'demo-stack',
        stage: 'completed',
        phase: 'succeeded',
        startedAt: '2026-09-08T00:00:00Z',
        settledAt: 1_000,
        detailPath: '/stack/logs/s1',
      },
    ]

    expect(mergeJobNotifications(settled, [], 1_000 + RETENTION + 1, RETENTION)).toEqual([])
    expect(mergeJobNotifications(settled, [], 1_000 + RETENTION - 1, RETENTION)).toHaveLength(1)
  })

  // 목록에서 사라진 진행 중 작업(삭제된 스택 등)은 영원히 돌게 둘 수 없다.
  it('응답에서 사라진 진행 중 작업은 내린다', () => {
    const running = mergeJobNotifications(
      [],
      toJobNotifications([stack({ id: 's1', status: 'installing' })], []),
      1_000,
      RETENTION,
    )

    expect(mergeJobNotifications(running, [], 2_000, RETENTION)).toEqual([])
  })

  it('진행 중이 먼저, 끝난 것은 최근 순으로 정렬한다', () => {
    const previous: JobNotification[] = [
      {
        id: 'stack:s-old',
        kind: 'stack',
        refId: 's-old',
        name: 'old',
        stage: 'installing',
        phase: 'running',
        startedAt: '2026-09-08T00:00:00Z',
        detailPath: '/stack/logs/s-old',
      },
      {
        id: 'stack:s-new',
        kind: 'stack',
        refId: 's-new',
        name: 'new',
        stage: 'installing',
        phase: 'running',
        startedAt: '2026-09-08T00:00:00Z',
        detailPath: '/stack/logs/s-new',
      },
    ]

    const merged = mergeJobNotifications(
      previous,
      toJobNotifications(
        [
          stack({ id: 's-old', name: 'old', status: 'completed' }),
          stack({ id: 's-new', name: 'new', status: 'installing' }),
        ],
        [],
      ),
      7_000,
      RETENTION,
    )

    expect(merged.map((job) => job.id)).toEqual(['stack:s-new', 'stack:s-old'])
  })

  it('바뀐 것이 없으면 같은 배열을 돌려준다', () => {
    const running = mergeJobNotifications(
      [],
      toJobNotifications([stack({ id: 's1', status: 'installing' })], []),
      1_000,
      RETENTION,
    )
    const again = mergeJobNotifications(
      running,
      toJobNotifications([stack({ id: 's1', status: 'installing' })], []),
      2_000,
      RETENTION,
    )

    expect(again).toBe(running)
  })
})

describe('formatElapsed', () => {
  it('분 미만은 초로 센다', () => {
    expect(formatElapsed(0)).toBe('0s')
    expect(formatElapsed(12_400)).toBe('12s')
  })

  it('시간 미만은 분과 초로 센다', () => {
    expect(formatElapsed(65_000)).toBe('1m 5s')
  })

  it('한 시간이 넘으면 시간과 분으로 센다', () => {
    expect(formatElapsed(3_725_000)).toBe('1h 2m')
  })

  it('음수는 0 으로 본다 — 서버와 브라우저 시계가 어긋날 수 있다', () => {
    expect(formatElapsed(-5_000)).toBe('0s')
  })
})

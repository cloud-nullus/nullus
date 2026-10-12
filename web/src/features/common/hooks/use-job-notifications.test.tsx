import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import type { Deployment, Stack } from '../../../types'

const stackQuery = vi.fn()
const deploymentQuery = vi.fn()

vi.mock('../../stack/api/stack-api', () => ({
  useStacks: () => stackQuery(),
}))

vi.mock('../../cicd/api/cicd-api', () => ({
  useDeployments: () => deploymentQuery(),
}))

const { useJobNotifications } = await import('./use-job-notifications')

const stack = (id: string, status: Stack['status']): Stack => ({
  id,
  name: `stack-${id}`,
  templateId: 'tpl',
  templateName: 'Template',
  clusterId: 'c1',
  clusterName: 'cluster',
  status,
  createdAt: '2026-09-08T00:00:00Z',
  updatedAt: '2026-09-08T00:00:00Z',
})

const deployment = (id: string, status: Deployment['status']): Deployment => ({
  id,
  pipelineId: 'p1',
  pipelineName: 'demo-pipeline',
  version: 'v1',
  status,
  triggeredBy: 'devops@nullus.dev',
  startedAt: '2026-09-08T00:00:00Z',
  completedAt: null,
})

function setData(stacks: Stack[], deployments: Deployment[]) {
  stackQuery.mockReturnValue({ data: { items: stacks, total: stacks.length } })
  deploymentQuery.mockReturnValue({ data: { items: deployments, total: deployments.length } })
}

beforeEach(() => {
  vi.useFakeTimers()
  setData([], [])
})

afterEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('useJobNotifications', () => {
  it('도는 작업이 없으면 빈 목록이다', () => {
    const { result } = renderHook(() => useJobNotifications())

    expect(result.current.jobs).toEqual([])
    expect(result.current.runningCount).toBe(0)
  })

  it('스택 설치와 배포를 함께 센다', () => {
    setData([stack('s1', 'installing')], [deployment('d1', 'running')])
    const { result } = renderHook(() => useJobNotifications())

    expect(result.current.runningCount).toBe(2)
    expect(result.current.jobs.map((job) => job.id)).toContain('stack:s1')
    expect(result.current.jobs.map((job) => job.id)).toContain('deployment:d1')
  })

  it('진행 중이던 작업이 완료되면 완료 상태로 남는다', () => {
    setData([stack('s1', 'installing')], [])
    const { result, rerender } = renderHook(() => useJobNotifications())
    expect(result.current.runningCount).toBe(1)

    setData([stack('s1', 'completed')], [])
    act(() => rerender())

    expect(result.current.runningCount).toBe(0)
    expect(result.current.jobs).toHaveLength(1)
    expect(result.current.jobs[0].phase).toBe('succeeded')
  })

  it('진행 중이던 작업이 실패하면 실패 상태로 남는다', () => {
    setData([], [deployment('d1', 'running')])
    const { result, rerender } = renderHook(() => useJobNotifications())

    setData([], [deployment('d1', 'failed')])
    act(() => rerender())

    expect(result.current.jobs[0].phase).toBe('failed')
  })

  // 폴링이 멈춘 뒤에도 목록은 스스로 비워져야 한다 — 응답이 더 오지 않기 때문이다.
  it('보존 시간이 지나면 완료 항목이 사라진다', () => {
    setData([stack('s1', 'installing')], [])
    const { result, rerender } = renderHook(() => useJobNotifications(5_000))

    setData([stack('s1', 'completed')], [])
    act(() => rerender())
    expect(result.current.jobs).toHaveLength(1)

    act(() => {
      vi.advanceTimersByTime(6_000)
    })

    expect(result.current.jobs).toEqual([])
  })

  it('경과 시간 기준 시각이 초마다 갱신된다', () => {
    setData([stack('s1', 'installing')], [])
    const { result } = renderHook(() => useJobNotifications())
    const first = result.current.now

    act(() => {
      vi.advanceTimersByTime(2_000)
    })

    expect(result.current.now).toBeGreaterThan(first)
  })
})

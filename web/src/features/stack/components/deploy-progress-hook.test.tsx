import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'

import { DeployProgressBar, useDeployProgressDisplay } from './deploy-progress-bar'

const MILESTONES = [5, 15, 90, 96, 100]

function Harness({ target, status }: { target: number; status: 'running' | 'success' | 'failed' }) {
  const value = useDeployProgressDisplay(target, status, MILESTONES)
  return <DeployProgressBar value={value} status={status} />
}

function shownPercent(): number {
  return Number(screen.getByRole('progressbar').getAttribute('aria-valuenow'))
}

describe('배포 진행률 표시(시간 경과)', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  // 서버 값은 단계가 바뀔 때만 뛴다. 그 사이 몇 분 동안 막대가 멈춰 있으면
  // 배포가 죽은 것처럼 보인다 — 이 화면을 고친 이유다.
  it('서버 값이 멈춰 있어도 막대는 계속 차오른다', () => {
    render(<Harness target={15} status="running" />)
    const before = shownPercent()

    act(() => {
      vi.advanceTimersByTime(4000)
    })

    expect(shownPercent()).toBeGreaterThan(before)
  })

  it('다음 단계까지 앞질러 가지는 않는다', () => {
    render(<Harness target={15} status="running" />)

    act(() => {
      vi.advanceTimersByTime(120_000)
    })

    // 다음 이정표는 90 이다. 거기 닿으면 시작도 안 한 단계를 끝난 것처럼 보인다.
    expect(shownPercent()).toBeLessThan(90)
    expect(shownPercent()).toBeGreaterThan(15)
  })

  it('성공하면 곧바로 100 이 된다', () => {
    render(<Harness target={62} status="success" />)

    expect(shownPercent()).toBe(100)
    expect(screen.getByTestId('deploy-rocket').className).toContain('nullus-rocket--launched')
  })

  it('실패하면 더 차오르지 않는다', () => {
    render(<Harness target={38} status="failed" />)
    const before = shownPercent()

    act(() => {
      vi.advanceTimersByTime(10_000)
    })

    expect(shownPercent()).toBe(before)
  })
})

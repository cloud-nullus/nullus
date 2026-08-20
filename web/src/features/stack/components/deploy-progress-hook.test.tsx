import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'

import { DeployProgressBar, useDeployProgressDisplay } from './deploy-progress-bar'

// 서버가 "이 스텝이 끝났을 때 닿을 값" 을 함께 보낸다. 화면은 그 안에서만 움직인다.
function Harness({
  target,
  status,
  ceiling,
}: {
  target: number
  status: 'running' | 'success' | 'failed'
  ceiling?: number
}) {
  const value = useDeployProgressDisplay(target, status, ceiling)
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
    render(<Harness target={15} status="running" ceiling={18} />)
    const before = shownPercent()

    // 스텝 하나가 대개 1~2분이라 그 눈금에서 확인한다. 몇 초 만에 눈에 띄게
    // 움직이면 실제로 한 일보다 앞서 가는 것이다 — 그게 이 화면의 원래 문제였다.
    act(() => {
      vi.advanceTimersByTime(60_000)
    })

    expect(shownPercent()).toBeGreaterThan(before)
  })

  it('이 스텝의 몫을 앞질러 가지는 않는다', () => {
    render(<Harness target={15} status="running" ceiling={18} />)

    act(() => {
      vi.advanceTimersByTime(600_000)
    })

    // 상한에 닿으면 아직 하지 않은 다음 스텝을 끝낸 것처럼 보인다.
    expect(shownPercent()).toBeLessThan(18)
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

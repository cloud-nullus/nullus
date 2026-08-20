import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'

import { DeployProgressBar } from './deploy-progress-bar'

describe('배포 진행 막대', () => {
  it('진행률만큼 채우고 값을 읽어 준다', () => {
    render(<DeployProgressBar value={42.7} status="running" />)

    const bar = screen.getByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '43')
    expect(screen.getByTestId('deploy-progress-value')).toHaveTextContent('43%')
    expect(screen.getByTestId('deploy-progress-fill')).toHaveStyle({ width: '42.7%' })
  })

  // 예전에는 1% 칸 100 개를 색만 바꿔 칠했다. 서버 값이 크게 뛰는 탓에 칸이
  // 우르르 켜졌다가 몇 분씩 멈춰 있어 차오르는 느낌이 없었다.
  it('진행 중에는 표면이 살아 있다', () => {
    render(<DeployProgressBar value={20} status="running" />)

    expect(screen.getByTestId('deploy-progress-fill').className).toContain('nullus-progress-fill--live')
    expect(screen.getByTestId('deploy-rocket').className).toContain('nullus-rocket--flying')
    expect(screen.getByTestId('deploy-rocket-flame')).toBeInTheDocument()
  })

  it('끝나면 로켓이 날아간다', () => {
    render(<DeployProgressBar value={100} status="success" />)

    const rocket = screen.getByTestId('deploy-rocket')
    expect(rocket.className).toContain('nullus-rocket--launched')
    expect(rocket.className).not.toContain('nullus-rocket--flying')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
  })

  // 실패한 배포에서 불꽃이 계속 타면 아직 가고 있는 것처럼 보인다.
  it('실패하면 불꽃을 끄고 막대를 오류색으로 둔다', () => {
    render(<DeployProgressBar value={38} status="failed" />)

    expect(screen.queryByTestId('deploy-rocket-flame')).not.toBeInTheDocument()
    expect(screen.getByTestId('deploy-progress-fill').className).toContain('color-error')
    expect(screen.getByTestId('deploy-rocket').className).not.toContain('nullus-rocket--flying')
  })

  it('범위를 벗어난 값은 잘라 낸다', () => {
    render(<DeployProgressBar value={140} status="success" />)
    expect(screen.getByTestId('deploy-progress-fill')).toHaveStyle({ width: '100%' })
  })
})

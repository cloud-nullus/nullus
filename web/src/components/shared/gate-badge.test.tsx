import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DependencyScanGateBadge, GateBadge, SecretScanGateBadge } from './gate-badge'

// 판정 배지 한 벌 — 시크릿 · 의존성 · SAST · 이미지가 같은 색 규칙을 쓴다.
describe('GateBadge', () => {
  it.each([
    ['pass', 'color-success'],
    ['warn', 'color-warning'],
    ['block', 'color-error'],
    ['error', 'color-text-secondary'],
  ] as const)('should color %s by the shared rule', (result, token) => {
    render(<GateBadge result={result} label={result} />)
    expect(screen.getByText(result).className).toContain(token)
  })

  // 모르는 판정은 초록이 아니다.
  it('should render an unknown result as neutral', () => {
    render(<GateBadge result={'maybe' as never} label="?" />)
    expect(screen.getByText('?').className).toContain('color-text-secondary')
  })
})

describe('DependencyScanGateBadge', () => {
  it('should show a zero-target pass as No targets, not green', () => {
    render(<DependencyScanGateBadge scan={{ gateResult: 'pass', dbStale: false, targets: 0 }} />)
    const badge = screen.getByText('No targets')
    expect(badge.className).toContain('color-text-secondary')
    expect(badge.getAttribute('title')).toContain('nothing was scanned')
  })

  it('should not treat a pass with unknown targets as No targets', () => {
    render(<DependencyScanGateBadge scan={{ gateResult: 'pass', dbStale: false }} />)
    expect(screen.getByText('Passed').className).toContain('color-success')
  })

  it('should warn on a pass with a stale DB', () => {
    render(<DependencyScanGateBadge scan={{ gateResult: 'pass', dbStale: true, targets: 3 }} />)
    expect(screen.getByText('Passed').className).toContain('color-warning')
    expect(screen.getByText('Stale DB')).toBeTruthy()
  })
})

describe('SecretScanGateBadge', () => {
  it.each([
    ['pass', 'No secrets found'],
    ['warn', 'Secrets found (warn)'],
    ['block', 'Blocked: secrets found'],
    ['error', 'Scan error'],
  ] as const)('should label %s', (gateResult, label) => {
    render(<SecretScanGateBadge scan={{ gateResult }} />)
    expect(screen.getByText(label)).toBeTruthy()
  })
})

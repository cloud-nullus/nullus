import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { PipelineSASTResult } from '../../../types'
import { SASTGateBadge, SASTResultDetail, SASTRowSummary } from './sast-result-summary'

function result(overrides: Partial<PipelineSASTResult> = {}): PipelineSASTResult {
  return {
    id: 'sast_dep_7',
    pipelineId: 'pip_x',
    deploymentId: 'dep_7',
    projectKey: 'app',
    qualityGateStatus: 'ERROR',
    gateResult: 'block',
    conditions: [
      { metric: 'new_violations', comparator: 'GT', threshold: '0', actual: '1', status: 'ERROR' },
      { metric: 'new_coverage', comparator: 'LT', threshold: '80', actual: '92.1', status: 'OK' },
      { metric: 'new_security_rating', comparator: 'GT', threshold: '1', actual: '4', status: 'ERROR' },
    ],
    metrics: { bugs: 0, vulnerabilities: 3, codeSmells: 2, securityHotspots: 0, coverage: 12.5, duplicatedLinesDensity: 0, ncloc: 85 },
    dashboardUrl: 'https://sonarqube.example.com/dashboard?id=app',
    analyzedAt: '2026-10-11T02:00:00Z',
    ...overrides,
  }
}

describe('SASTGateBadge', () => {
  it.each([
    ['pass', 'Quality Gate passed', 'color-success'],
    ['warn', 'Quality Gate failed (warn)', 'color-warning'],
    ['block', 'Blocked by Quality Gate', 'color-error'],
  ] as const)('%s 는 %s 로 보인다', (gateResult, label, token) => {
    render(<SASTGateBadge result={result({ gateResult })} />)

    expect(screen.getByText(label).className).toContain(token)
  })

  // 분석 실패는 코드 판정이 아니다. 초록(통과)도 빨강(문제 발견)도 아니다.
  it('분석 실패는 중립색이다', () => {
    render(<SASTGateBadge result={result({ gateResult: 'error' })} />)

    const badge = screen.getByText('Analysis failed')
    expect(badge.className).not.toContain('color-success')
    expect(badge.className).not.toContain('color-error')
  })
})

describe('SASTRowSummary', () => {
  // 실행 줄에는 이미지 스캔 배지도 붙는다. 어느 검사의 판정인지 이름을 단다.
  it('SAST 라는 이름과 판정을 함께 보여준다', () => {
    render(<SASTRowSummary result={result()} />)

    expect(screen.getByText('SAST')).toBeTruthy()
    expect(screen.getByText('Blocked by Quality Gate')).toBeTruthy()
  })
})

describe('SASTResultDetail', () => {
  it('통과하지 못한 조건만 실제 값과 기준으로 보여준다', () => {
    render(<SASTResultDetail result={result()} locale="en-US" />)

    expect(screen.getByText('Static analysis (SonarQube)')).toBeTruthy()
    const failed = screen.getByRole('list', { name: 'Failed conditions' })
    expect(failed.textContent).toContain('New issues')
    expect(failed.textContent).toContain('1')
    expect(failed.textContent).toContain('> 0')
    // 등급 지표는 SonarQube 화면처럼 문자로 보인다(4 → D).
    expect(failed.textContent).toContain('Security rating on new code')
    expect(failed.textContent).toContain('D')
    expect(failed.textContent).not.toContain('Coverage on new code')
  })

  it('백분율 조건은 % 로 보인다', () => {
    render(
      <SASTResultDetail
        result={result({
          conditions: [
            { metric: 'new_security_hotspots_reviewed', comparator: 'LT', threshold: '100', actual: '0.0', status: 'ERROR' },
          ],
        })}
        locale="en-US"
      />,
    )

    const failed = screen.getByRole('list', { name: 'Failed conditions' })
    expect(failed.textContent).toContain('0.0%')
    expect(failed.textContent).toContain('< 100%')
  })

  it('지표를 보여주고, 0 은 0 으로 보인다', () => {
    render(<SASTResultDetail result={result()} locale="en-US" />)

    expect(screen.getByLabelText('Vulnerabilities 3')).toBeTruthy()
    expect(screen.getByLabelText('Bugs 0')).toBeTruthy()
    expect(screen.getByLabelText('Coverage 12.5%')).toBeTruthy()
  })

  // 분석하지 못한 실행에 0 을 그리면 "문제 0건" 으로 읽힌다.
  it('지표를 모르면 0 이 아니라 모름으로 보인다', () => {
    render(<SASTResultDetail result={result({ gateResult: 'error', metrics: undefined, conditions: [], qualityGateStatus: undefined })} locale="en-US" />)

    expect(screen.getByText('Metrics unavailable')).toBeTruthy()
    expect(screen.queryByLabelText(/Vulnerabilities/)).toBeNull()
    expect(screen.getByText(/could not run/)).toBeTruthy()
  })

  it('경고 정책으로 통과한 실행은 그 사실을 알린다', () => {
    render(<SASTResultDetail result={result({ gateResult: 'warn' })} locale="en-US" />)

    expect(screen.getByText(/policy is set to warn/)).toBeTruthy()
  })

  it('SonarQube 링크는 http(s) 일 때만 건다', () => {
    const { unmount } = render(<SASTResultDetail result={result()} locale="en-US" />)
    expect(screen.getByRole('link', { name: /Open in SonarQube/ }).getAttribute('href')).toBe(
      'https://sonarqube.example.com/dashboard?id=app',
    )
    unmount()

    render(<SASTResultDetail result={result({ dashboardUrl: 'javascript:alert(1)' })} locale="en-US" />)
    expect(screen.queryByRole('link', { name: /Open in SonarQube/ })).toBeNull()
  })
})

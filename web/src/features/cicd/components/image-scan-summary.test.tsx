import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { PipelineImageScan } from '../../../types'
import { ImageScanDetail, ImageScanGateBadge, ImageScanRowSummary } from './image-scan-summary'

function scan(overrides: Partial<PipelineImageScan> = {}): PipelineImageScan {
  return {
    id: 'scan_dep_2',
    pipelineId: 'pip_x',
    deploymentId: 'dep_2',
    imageRepository: 'harbor.example/nullus/app',
    imageTag: '09b48b6e',
    imageDigest: 'sha256:0c92a1b2c3d4e5f60718293a4b5c6d7e8f9',
    scanSource: 'central',
    scanner: 'trivy',
    scannerVersion: '0.74.0',
    dbUpdatedAt: '2026-09-13T07:13:14Z',
    counts: { critical: 0, high: 6, medium: 23, low: 19, unknown: 0 },
    gateResult: 'pass',
    reportUri: 'https://gitlab.example/nullus/app/-/jobs/11/artifacts/file/trivy-report.json',
    scannedAt: '2026-09-13T13:29:51Z',
    dbStale: false,
    ...overrides,
  }
}

describe('ImageScanGateBadge', () => {
  it.each([
    ['pass', 'Passed', 'color-success'],
    ['warn', 'Passed with warnings', 'color-warning'],
    ['block', 'Blocked by policy', 'color-error'],
  ] as const)('%s 는 %s 로 보인다', (gateResult, label, token) => {
    render(<ImageScanGateBadge scan={scan({ gateResult })} />)

    const badge = screen.getByText(label)
    expect(badge.className).toContain(token)
  })

  // error 는 스캔을 못 했다는 뜻이다. 초록(통과)도 빨강(취약점)도 아니다.
  it('error 는 스캔 오류로, 초록·빨강이 아닌 중립색으로 보인다', () => {
    render(<ImageScanGateBadge scan={scan({ gateResult: 'error' })} />)

    const badge = screen.getByText('Scan error')
    expect(badge.className).not.toContain('color-success')
    expect(badge.className).not.toContain('color-error')
  })

  // DB 가 오래됐으면 통과라도 깨끗한 초록 통과로 보여선 안 된다.
  it('DB 가 오래된 pass 는 초록이 아니고 오래됨 경고가 붙는다', () => {
    render(<ImageScanGateBadge scan={scan({ gateResult: 'pass', dbStale: true })} />)

    expect(screen.getByText('Passed').className).not.toContain('color-success')
    expect(screen.getByText('Stale DB')).toBeTruthy()
  })
})

describe('ImageScanRowSummary', () => {
  it('게이트와 심각도 건수를 함께 보여준다', () => {
    render(<ImageScanRowSummary scan={scan()} />)

    expect(screen.getByText('Passed')).toBeTruthy()
    expect(screen.getByLabelText('High 6')).toBeTruthy()
  })

  it('건수가 없으면 0 이 아니라 모름으로 보인다', () => {
    render(<ImageScanRowSummary scan={scan({ counts: undefined, gateResult: 'error' })} />)

    expect(screen.getByText('Counts unknown')).toBeTruthy()
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
  })
})

describe('ImageScanDetail', () => {
  it('이미지·다이제스트·스캐너·DB·스캔 시각을 보여준다', () => {
    render(<ImageScanDetail scan={scan()} locale="en-US" />)

    expect(screen.getByText('Image scan')).toBeTruthy()
    expect(screen.getByText('harbor.example/nullus/app:09b48b6e')).toBeTruthy()
    // 다이제스트는 길어서 앞부분만 보이고 전체는 title 로 남긴다.
    const digest = screen.getByText('sha256:0c92a1b2c3d4')
    expect(digest.getAttribute('title')).toBe('sha256:0c92a1b2c3d4e5f60718293a4b5c6d7e8f9')
    expect(screen.getByText('trivy 0.74.0')).toBeTruthy()
    expect(screen.getByLabelText('Unknown severity 0')).toBeTruthy()
  })

  it('리포트 링크는 새 창으로, opener 없이 연다', () => {
    render(<ImageScanDetail scan={scan()} locale="en-US" />)

    const link = screen.getByRole('link', { name: /View report/ })
    expect(link.getAttribute('href')).toBe(
      'https://gitlab.example/nullus/app/-/jobs/11/artifacts/file/trivy-report.json',
    )
    expect(link.getAttribute('target')).toBe('_blank')
    expect(link.getAttribute('rel')).toBe('noopener noreferrer')
  })

  // report_uri 는 CI 가 준 값이다. http(s) 가 아니면(javascript: 등) 링크로 만들지 않는다.
  it('http(s) 가 아닌 report_uri 는 링크로 만들지 않는다', () => {
    render(<ImageScanDetail scan={scan({ reportUri: 'javascript:alert(1)' })} locale="en-US" />)

    expect(screen.queryByRole('link', { name: /View report/ })).toBeNull()
  })

  it('report_uri 가 비면 링크를 만들지 않는다', () => {
    render(<ImageScanDetail scan={scan({ reportUri: undefined })} locale="en-US" />)

    expect(screen.queryByRole('link', { name: /View report/ })).toBeNull()
  })

  it('DB 가 오래됐으면 경고 문구를 보여준다', () => {
    render(<ImageScanDetail scan={scan({ dbStale: true, dbUpdatedAt: undefined })} locale="en-US" />)

    expect(screen.getAllByText('Stale DB').length).toBeGreaterThan(0)
    expect(screen.getByText(/older than 30 days/)).toBeTruthy()
  })

  it('error 게이트는 취약점이 아니라 스캔을 못 했다고 설명한다', () => {
    render(<ImageScanDetail scan={scan({ gateResult: 'error', counts: undefined })} locale="en-US" />)

    expect(screen.getByText(/could not be performed/)).toBeTruthy()
    expect(screen.getByText('Counts unknown')).toBeTruthy()
  })
})

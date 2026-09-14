import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { StackImageScanItem, StackImageScanReport } from '../api/stack-api-types'
import { StackImageScansTab } from './stack-image-scans-tab'

const mockUseStackImageScans = vi.fn()

vi.mock('../api/stack-api', () => ({
  useStackImageScans: (...args: unknown[]) => mockUseStackImageScans(...args),
}))

function report(overrides: Partial<StackImageScanReport> = {}): StackImageScanReport {
  return {
    stackId: 'stk_1',
    status: 'scanned',
    reason: '',
    lastScannedAt: '2026-09-14T01:02:03Z',
    summary: { critical: 1, high: 10, medium: 40, low: 30, unknown: 2 },
    items: [],
    total: 0,
    ...overrides,
  }
}

function item(image: string, overrides: Partial<StackImageScanItem> = {}): StackImageScanItem {
  return {
    image,
    imageDigest: 'sha256:abcdef0123456789abcdef',
    release: 'nullus-postgresql',
    workloads: ['nullus-postgresql-0'],
    status: 'scanned',
    counts: { critical: 0, high: 2, medium: 5, low: 3, unknown: 0 },
    fixableCounts: { critical: 0, high: 1, medium: 2, low: 0, unknown: 0 },
    scannerVersion: '0.74.0',
    dbUpdatedAt: '2026-09-13T07:13:14Z',
    dbStale: false,
    scannedAt: '2026-09-14T01:02:03Z',
    ...overrides,
  }
}

function mockReport(data: StackImageScanReport) {
  mockUseStackImageScans.mockReturnValue({ data, isLoading: false, isError: false })
}

describe('StackImageScansTab', () => {
  beforeEach(() => {
    mockUseStackImageScans.mockReset()
  })

  it('스택 id 로 조회하고 보고용 결과임을 밝힌다', () => {
    mockReport(report())

    render(<StackImageScansTab stackId="stk_1" />)

    expect(mockUseStackImageScans).toHaveBeenCalledWith('stk_1')
    expect(screen.getByText('Installed image vulnerabilities')).toBeTruthy()
    expect(screen.getByText(/never blocked the installation/)).toBeTruthy()
  })

  // Trivy 를 고르지 않은 스택은 스캔 자체를 안 한다. 그게 "취약점 0" 으로 보이면 안 된다.
  it('스캐너가 없어 스캔하지 않은 스택은 이유를 설명하고 0 을 그리지 않는다', () => {
    mockReport(report({ status: 'not_scanned', reason: 'scanner_not_installed', summary: undefined, lastScannedAt: undefined }))

    const { container } = render(<StackImageScansTab stackId="stk_1" />)

    expect(screen.getByText(/Trivy is not selected for this stack/)).toBeTruthy()
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
    expect(container.textContent).not.toMatch(/\b0\b/)
  })

  it('폐쇄망 설치는 설치 이미지를 스캔하지 않는다고 설명한다', () => {
    mockReport(report({ status: 'not_scanned', reason: 'airgap', summary: undefined, lastScannedAt: undefined }))

    render(<StackImageScansTab stackId="stk_1" />)

    expect(screen.getByText(/Air-gapped installs do not scan installed images/)).toBeTruthy()
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
  })

  it('스캔 대기 중이면 곧 스캔한다고 알리고 건수를 그리지 않는다', () => {
    mockReport(report({ status: 'pending', summary: undefined, lastScannedAt: undefined }))

    render(<StackImageScansTab stackId="stk_1" />)

    expect(screen.getByText(/Waiting for the first scan/)).toBeTruthy()
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
  })

  it('요약 건수와 마지막 스캔 시각을 보여준다', () => {
    mockReport(report({ items: [item('docker.io/bitnami/postgresql:16.4.0')], total: 1 }))

    render(<StackImageScansTab stackId="stk_1" />)

    const summary = screen.getByTestId('stack-image-scan-summary')
    expect(within(summary).getByLabelText('Critical 1')).toBeTruthy()
    expect(within(summary).getByLabelText('Unknown severity 2')).toBeTruthy()
    expect(within(summary).getByText(/include vulnerabilities without an available fix/)).toBeTruthy()
    expect(within(summary).getByText(/Last scanned/)).toBeTruthy()
    // 스캐너와 DB 날짜는 이미지마다 같다. 행마다 되풀이하지 않고 요약에 한 번 둔다.
    expect(within(summary).getByText(/trivy 0\.74\.0/)).toBeTruthy()
    expect(within(summary).getByText(/Vulnerability DB/)).toBeTruthy()
  })

  // 이미지가 수십 개다(GitLab 스택 33개). 카드 목록은 건수를 나란히 비교할 수 없어 그리드로 보인다.
  it('이미지를 그리드로 보이고 심각도별 열을 둔다', () => {
    mockReport(report({ items: [item('docker.io/bitnami/postgresql:16.4.0')], total: 1 }))

    render(<StackImageScansTab stackId="stk_1" />)

    const grid = screen.getByRole('table')
    for (const name of ['Image', 'Release', 'Critical', 'High', 'Medium', 'Low', 'Fixable', 'Status']) {
      expect(within(grid).getByRole('columnheader', { name }), name).toBeTruthy()
    }
    const row = screen.getByTestId('stack-image-scan-item')
    expect(row.tagName).toBe('TR')
    expect(within(row).getByText('sha256:abcdef012345')).toBeTruthy()
    expect(within(row).getByText('nullus-postgresql-0')).toBeTruthy()
  })

  it('항목을 심각도 순(Critical, High 많은 순)으로 보여준다', () => {
    mockReport(
      report({
        items: [
          item('img-low-high', { counts: { critical: 0, high: 1, medium: 0, low: 0, unknown: 0 } }),
          item('img-critical', { counts: { critical: 3, high: 0, medium: 0, low: 0, unknown: 0 } }),
          item('img-many-high', { counts: { critical: 0, high: 7, medium: 0, low: 0, unknown: 0 } }),
        ],
        total: 3,
      }),
    )

    render(<StackImageScansTab stackId="stk_1" />)

    const images = screen
      .getAllByTestId('stack-image-scan-item')
      .map((el) => within(el).getByTestId('stack-image-scan-image').textContent)
    expect(images).toEqual(['img-critical', 'img-many-high', 'img-low-high'])
  })

  it('실패한 항목은 오류를 보여주고 건수를 그리지 않는다', () => {
    mockReport(
      report({
        items: [item('quay.io/broken:1', { status: 'failed', error: 'manifest unknown', counts: undefined, fixableCounts: undefined })],
        total: 1,
      }),
    )

    render(<StackImageScansTab stackId="stk_1" />)

    const card = screen.getByTestId('stack-image-scan-item')
    expect(within(card).getByText('Scan failed')).toBeTruthy()
    expect(within(card).getByText('manifest unknown')).toBeTruthy()
    expect(within(card).queryByLabelText(/Critical/)).toBeNull()
    expect(within(card).queryByText('Counts unknown')).toBeNull()
  })

  it('건수가 없는 항목은 모름으로, 고칠 수 있는 건수와 함께 보여준다', () => {
    mockReport(
      report({
        items: [
          item('img-unknown', { counts: undefined, fixableCounts: undefined }),
          item('img-known'),
        ],
        total: 2,
      }),
    )

    render(<StackImageScansTab stackId="stk_1" />)

    const [known, unknown] = screen.getAllByTestId('stack-image-scan-item')
    expect(within(unknown).getByText('Counts unknown')).toBeTruthy()
    expect(within(unknown).queryByLabelText(/Critical/)).toBeNull()
    expect(within(known).getByLabelText('High 2')).toBeTruthy()
    // 고칠 수 있는 건수는 합계를 칸에, 심각도별 내역은 툴팁에 둔다.
    expect(within(known).getByTitle('Fixable: Critical 0 · High 1 · Medium 2 · Low 0')).toBeTruthy()
  })

  it('취약점 DB 가 오래됐으면 경고를 붙인다', () => {
    mockReport(report({ items: [item('img-stale', { dbStale: true })], total: 1 }))

    render(<StackImageScansTab stackId="stk_1" />)

    const card = screen.getByTestId('stack-image-scan-item')
    expect(within(card).getByText('Stale DB')).toBeTruthy()
  })

  // 404(모르는 스택)·503(배선 전)은 화면을 깨지 않고 중립 안내로 끝난다.
  it('조회가 실패하면 데이터 없음으로 보여준다', () => {
    mockUseStackImageScans.mockReturnValue({ data: undefined, isLoading: false, isError: true })

    render(<StackImageScansTab stackId="stk_1" />)

    expect(screen.getByText('No image scan data is available for this stack.')).toBeTruthy()
  })

  it('불러오는 중이면 로딩 문구를 보여준다', () => {
    mockUseStackImageScans.mockReturnValue({ data: undefined, isLoading: true, isError: false })

    render(<StackImageScansTab stackId="stk_1" />)

    expect(screen.getByText('Loading image scan results...')).toBeTruthy()
  })
})

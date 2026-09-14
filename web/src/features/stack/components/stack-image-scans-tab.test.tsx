import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import type { StackImageScanItem, StackImageScanReport } from '../api/stack-api-types'
import type { VulnerabilityListResult } from '../../../types'
import { StackImageScansTab } from './stack-image-scans-tab'

const mockUseStackImageScans = vi.fn()
const mockUseStackImageVulnerabilities = vi.fn()

vi.mock('../api/stack-api', () => ({
  useStackImageScans: (...args: unknown[]) => mockUseStackImageScans(...args),
  useStackImageVulnerabilities: (...args: unknown[]) => mockUseStackImageVulnerabilities(...args),
}))

const vulnerabilityList: VulnerabilityListResult = {
  status: 'available',
  reason: '',
  targets: [{ target: 'debian 12.7', class: 'os', total: 1 }],
  items: [
    {
      id: 'CVE-2024-6119',
      pkg: 'libssl3',
      installed: '3.0.14-1~deb12u1',
      fixed: '3.0.14-1~deb12u2',
      severity: 'high',
      class: 'os',
      target: 'debian 12.7',
      url: 'https://avd.aquasec.com/nvd/cve-2024-6119',
    },
  ],
  total: 1,
  limit: 50,
  offset: 0,
}

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
    mockUseStackImageVulnerabilities.mockReset()
    mockUseStackImageVulnerabilities.mockReturnValue({
      data: vulnerabilityList,
      isLoading: false,
      isError: false,
      isFetching: false,
    })
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

  // 건수만으로는 무엇을 고쳐야 하는지 알 수 없다. 행을 펼쳐 그 이미지의 취약점 목록을 본다.
  it('스캔된 이미지 행을 펼치면 모든 열을 가로지르는 상세 행에 목록을 보이고, 펼치기 전에는 조회하지 않는다', () => {
    mockReport(report({ items: [item('docker.io/bitnami/postgresql:16.4.0')], total: 1 }))

    render(<StackImageScansTab stackId="stk_1" />)

    const row = screen.getByTestId('stack-image-scan-item')
    const toggle = within(row).getByRole('button', { name: /Vulnerability list/ })
    // 네이티브 버튼이라 키보드(Tab·Enter·Space)로 펼칠 수 있다.
    expect(toggle.tagName).toBe('BUTTON')
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(mockUseStackImageVulnerabilities).not.toHaveBeenCalled()
    expect(screen.queryByTestId('stack-image-scan-detail')).toBeNull()

    fireEvent.click(toggle)

    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    const detail = screen.getByTestId('stack-image-scan-detail')
    expect(detail.tagName).toBe('TR')
    expect(detail.querySelector('td')?.getAttribute('colspan')).toBe('9')
    expect(toggle.getAttribute('aria-controls')).toBe(detail.querySelector('td > div')?.id)
    expect(mockUseStackImageVulnerabilities).toHaveBeenLastCalledWith(
      'stk_1',
      'sha256:abcdef0123456789abcdef',
      expect.objectContaining({ offset: 0, severities: [] }),
      true,
    )
    expect(within(detail).getByText('libssl3')).toBeTruthy()
  })

  it('펼친 뒤에도 그리드의 열 너비는 그대로다', () => {
    mockReport(report({ items: [item('docker.io/bitnami/postgresql:16.4.0')], total: 1 }))

    render(<StackImageScansTab stackId="stk_1" />)
    fireEvent.click(screen.getByRole('button', { name: /Vulnerability list/ }))

    const [grid] = screen.getAllByRole('table')
    const widths = Array.from(grid.querySelectorAll(':scope > colgroup > col')).map(
      (col) => (col as HTMLElement).style.width,
    )
    expect(widths).toEqual(['', '200px', '64px', '72px', '64px', '64px', '96px', '88px', '75px'])
  })

  it('다시 누르면 상세 행을 접는다', () => {
    mockReport(report({ items: [item('docker.io/bitnami/postgresql:16.4.0')], total: 1 }))

    render(<StackImageScansTab stackId="stk_1" />)
    const toggle = screen.getByRole('button', { name: /Vulnerability list/ })
    fireEvent.click(toggle)
    fireEvent.click(toggle)

    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByTestId('stack-image-scan-detail')).toBeNull()
  })

  // 실패한 스캔에는 목록이 없다. 다이제스트가 없으면 어느 이미지를 물을지 모른다.
  it('실패했거나 다이제스트가 없는 행은 펼칠 수 없다', () => {
    mockReport(
      report({
        items: [
          item('quay.io/broken:1', { status: 'failed', error: 'manifest unknown', counts: undefined, fixableCounts: undefined }),
          item('quay.io/nodigest:1', { imageDigest: undefined }),
        ],
        total: 2,
      }),
    )

    render(<StackImageScansTab stackId="stk_1" />)

    expect(screen.queryByRole('button', { name: /Vulnerability list/ })).toBeNull()
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

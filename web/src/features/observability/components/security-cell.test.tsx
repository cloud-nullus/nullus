import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { SecurityCell, SecurityStage } from '../types/security-posture'
import { SecurityCellView } from './security-cell'

const cell = (overrides: Partial<SecurityCell>): SecurityCell => ({
  state: 'pass',
  staleRun: false,
  dbStale: false,
  ...overrides,
})

const renderCell = (stage: SecurityStage, value: SecurityCell, latestVersion?: string) =>
  render(<SecurityCellView stage={stage} cell={value} latestVersion={latestVersion} />)

// 여섯 상태 중 뒤의 셋(판정 불가 · 기록 없음 · 미적용)은 초록이 아니다(설계 §2.2).
describe('SecurityCellView', () => {
  it.each([
    ['pass', 'Pass', 'color-success'],
    ['warn', 'Warn', 'color-warning'],
    ['block', 'Blocked', 'color-error'],
    ['error', 'Undetermined', 'color-text-secondary'],
  ] as const)('should render %s as %s', (state, label, token) => {
    renderCell('sast', cell({ state }))
    const badge = screen.getByText(label)
    expect(badge.className).toContain(token)
  })

  it('should render unrecorded with a dashed border, not green', () => {
    renderCell('sast', cell({ state: 'unrecorded' }))
    const badge = screen.getByText('No record')
    expect(badge.className).toContain('border-dashed')
    expect(badge.className).not.toContain('color-success')
  })

  it('should render not applied as a dash', () => {
    renderCell('secret_scan', cell({ state: 'not_applied' }))
    expect(screen.getByRole('img', { name: 'Not applied' }).textContent).toBe('—')
  })

  // 락파일이 없어 아무것도 검사하지 않은 통과는 초록이 아니다.
  it('should render a dependency pass with zero targets as gray No targets', () => {
    renderCell('dependency_scan', cell({ state: 'pass', targets: 0, counts: { critical: 0, high: 0, medium: 0, low: 0, unknown: 0 } }))
    const badge = screen.getByText('No targets')
    expect(badge.className).toContain('color-text-secondary')
    expect(screen.queryByText('Pass')).toBeNull()
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
  })

  it('should show critical and high counts for image and dependency cells', () => {
    renderCell('image_scan', cell({ state: 'block', counts: { critical: 2, high: 11, medium: 3, low: 0, unknown: 0 } }))
    expect(screen.getByLabelText('Critical 2').textContent).toBe('C2')
    expect(screen.getByLabelText('High 11').textContent).toBe('H11')
  })

  // 숫자가 빠졌으면 모르는 것이다. 0 을 그리면 "문제 0건" 으로 읽힌다.
  it('should not render unknown numbers as zero', () => {
    const { container } = renderCell('image_scan', cell({ state: 'error', counts: undefined }))
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
    expect(container.textContent).not.toMatch(/C0|H0/)

    renderCell('secret_scan', cell({ state: 'block', findings: undefined }))
    expect(screen.queryByText(/found/)).toBeNull()

    renderCell('sast', cell({ state: 'warn', vulnerabilities: undefined, securityHotspots: 2 }))
    expect(screen.queryByText(/Vuln/)).toBeNull()
    expect(screen.getByText('Hotspots 2')).toBeTruthy()
  })

  it('should show the secret findings count', () => {
    renderCell('secret_scan', cell({ state: 'block', findings: 2 }))
    expect(screen.getByText('2 found')).toBeTruthy()
  })

  it('should show SAST vulnerabilities and hotspots', () => {
    renderCell('sast', cell({ state: 'warn', vulnerabilities: 4, securityHotspots: 2 }))
    expect(screen.getByText('Vuln 4 · Hotspots 2')).toBeTruthy()
  })

  it('should mark a result from an older run with a muted run number and tooltip', () => {
    renderCell('image_scan', cell({ state: 'block', version: '#38', staleRun: true }), '#42')
    const marker = screen.getByText('#38')
    expect(marker.className).toContain('color-text-muted')
    expect(marker.getAttribute('title')).toBe(
      'The latest run #42 has no result for this stage — this is from #38.',
    )
  })

  it('should not show a run number for a result from the latest run', () => {
    renderCell('image_scan', cell({ state: 'block', version: '#42', staleRun: false }), '#42')
    expect(screen.queryByText('#42')).toBeNull()
  })

  it('should show the DB age when the vulnerability DB is stale', () => {
    const tenDaysAgo = new Date(Date.now() - 34 * 24 * 60 * 60 * 1000 - 1000).toISOString()
    renderCell('image_scan', cell({ state: 'pass', dbStale: true, dbUpdatedAt: tenDaysAgo }))
    expect(screen.getByText('DB 34d')).toBeTruthy()
    // 낡은 DB 로 얻은 통과는 초록이 아니다.
    expect(screen.getByText('Pass').className).toContain('color-warning')
  })

  it('should fall back to Stale DB when the DB date is unknown', () => {
    renderCell('dependency_scan', cell({ state: 'warn', dbStale: true, targets: 2 }))
    expect(screen.getByText('Stale DB')).toBeTruthy()
  })
})

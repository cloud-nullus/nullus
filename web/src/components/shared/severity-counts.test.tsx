import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SeverityCounts } from './severity-counts'

describe('SeverityCounts', () => {
  it('심각도별 건수를 이름과 함께 보여준다', () => {
    render(<SeverityCounts counts={{ critical: 1, high: 6, medium: 23, low: 19, unknown: 0 }} />)

    expect(screen.getByLabelText('Critical 1')).toBeTruthy()
    expect(screen.getByLabelText('High 6')).toBeTruthy()
    expect(screen.getByLabelText('Medium 23')).toBeTruthy()
    expect(screen.getByLabelText('Low 19')).toBeTruthy()
    expect(screen.getByLabelText('Unknown severity 0')).toBeTruthy()
  })

  it('compact 는 Unknown 심각도가 0 이면 생략한다', () => {
    render(<SeverityCounts compact counts={{ critical: 0, high: 1, medium: 0, low: 0, unknown: 0 }} />)

    expect(screen.getByLabelText('Critical 0')).toBeTruthy()
    expect(screen.queryByLabelText(/Unknown severity/)).toBeNull()
  })

  // 모르는 건수를 0 으로 그리면 "취약점 없음" 으로 읽힌다.
  it('건수가 없으면 0 대신 모름을 보여준다', () => {
    const { container } = render(<SeverityCounts counts={undefined} />)

    expect(screen.getByText('Counts unknown')).toBeTruthy()
    expect(screen.queryByLabelText(/Critical/)).toBeNull()
    expect(container.textContent).not.toMatch(/\b0\b/)
  })
})

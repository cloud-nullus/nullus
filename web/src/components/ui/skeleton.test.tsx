import { describe, it, expect } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { Skeleton } from './skeleton'

describe('Skeleton', () => {
  it('renders a pulsing placeholder and merges the caller className', () => {
    render(<Skeleton className="h-4 w-24" />)
    const el = screen.getByTestId('skeleton')
    expect(el).toBeInTheDocument()
    // 펄스 애니메이션 확인. 내부가 MUI Skeleton 으로 바뀌면서 Tailwind 의
    // animate-pulse 대신 MuiSkeleton-pulse 가 붙는다 — 구현 디테일이라 단정만 옮겼다.
    expect(el.className).toContain('MuiSkeleton-pulse')
    // 계약: 호출자가 준 클래스가 병합돼 크기를 호출자가 정한다.
    expect(el.className).toContain('h-4')
  })
})

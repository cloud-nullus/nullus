import { describe, it, expect } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { Skeleton } from './skeleton'

describe('Skeleton', () => {
  it('renders with the pulsing placeholder class', () => {
    render(<Skeleton className="h-4 w-24" />)
    const el = screen.getByTestId('skeleton')
    expect(el).toBeInTheDocument()
    expect(el.className).toContain('animate-pulse')
    // Caller-provided modifier is merged in.
    expect(el.className).toContain('h-4')
  })
})

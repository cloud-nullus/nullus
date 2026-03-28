import type { ReactElement } from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ErrorBoundary } from './error-boundary'

function HealthyChild() {
  return <div>Healthy content</div>
}

const CrashChild: () => ReactElement = () => {
  throw new Error('Boom from child')
}

describe('ErrorBoundary', () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleErrorSpy.mockRestore()
  })

  it('renders children without crashing', () => {
    render(
      <ErrorBoundary>
        <HealthyChild />
      </ErrorBoundary>
    )

    expect(screen.getByText('Healthy content')).not.toBeNull()
  })

  it('catches render errors and shows fallback UI', () => {
    render(
      <ErrorBoundary>
        <CrashChild />
      </ErrorBoundary>
    )

    expect(screen.getByText('Something went wrong')).not.toBeNull()
    expect(screen.getByText('A rendering error interrupted this page.')).not.toBeNull()
    expect(screen.getByText('Boom from child')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Try Again' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Go Home' })).not.toBeNull()
  })
})

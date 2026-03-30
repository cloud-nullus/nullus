import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { HomePage } from './home-page'
import { useAuthStore } from '../../../stores/auth-store'

const START_STACK_LABEL = /Start Stack|Stack 시작하기/
const CICD_PIPELINE_LABEL = /CI\/CD Pipeline|CI\/CD 파이프라인/
const CORE_FEATURES_LABEL = /Core Features|핵심 기능/

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
})

describe('HomePage', () => {
  it('renders the welcome heading', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText('Nullus Platform')).toBeInTheDocument()
  })

  it('renders Stack Start button for all roles', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('renders CI/CD Pipeline button for all roles', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByRole('button', { name: CICD_PIPELINE_LABEL })).toBeInTheDocument()
  })

  it('shows core features section for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText(CORE_FEATURES_LABEL)).toBeInTheDocument()
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('shows core features section for devops role', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText(CORE_FEATURES_LABEL)).toBeInTheDocument()
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('shows core features section for admin role', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText(CORE_FEATURES_LABEL)).toBeInTheDocument()
    expect(screen.getByText(START_STACK_LABEL)).toBeInTheDocument()
  })

  it('does not show View Stacks button for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.queryByText('View Stacks')).not.toBeInTheDocument()
  })
})

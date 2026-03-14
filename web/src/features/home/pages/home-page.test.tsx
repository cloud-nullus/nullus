import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { HomePage } from './home-page'
import { useAuthStore } from '../../../stores/auth-store'

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
})

describe('HomePage', () => {
  it('renders the welcome heading', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText('Welcome to Nullus Platform')).toBeInTheDocument()
  })

  it('renders Get Started button for all roles', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText('Get Started')).toBeInTheDocument()
  })

  it('renders View Pipelines button for all roles', () => {
    renderWithProviders(<HomePage />)
    expect(screen.getByText('View Pipelines')).toBeInTheDocument()
  })

  it('shows developer greeting for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText('You are deploying applications as Developer.')).toBeInTheDocument()
  })

  it('shows devops greeting and View Stacks CTA for devops role', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText('You are managing DevSecOps stacks as DevOps Engineer.')).toBeInTheDocument()
    expect(screen.getByText('View Stacks')).toBeInTheDocument()
  })

  it('shows admin greeting and View Stacks CTA for admin role', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.getByText('You are managing the platform as Administrator.')).toBeInTheDocument()
    expect(screen.getByText('View Stacks')).toBeInTheDocument()
  })

  it('does not show View Stacks button for developer role', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
    renderWithProviders(<HomePage />)
    expect(screen.queryByText('View Stacks')).not.toBeInTheDocument()
  })
})

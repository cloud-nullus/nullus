/**
 * UAT-2: Developer "지은" scenario
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from './test-utils'
import { LoginPage } from '../features/auth/pages/login-page'
import { Sidebar } from '../components/layout/sidebar'
import { useAuthStore } from '../stores/auth-store'
import { useSidebarStore } from '../stores/sidebar-store'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
  useSidebarStore.setState({ collapsed: false })
  mockNavigate.mockClear()
})

describe('UAT-2: Developer scenario', () => {
  it('step 1: developer can log in with developer@nullus.dev', async () => {
    renderWithProviders(<LoginPage />)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'developer@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'developer123' } })
    fireEvent.submit(screen.getByText('Sign in').closest('form')!)

    await vi.waitFor(() => {
      expect(useAuthStore.getState().isAuthenticated).toBe(true)
    })
    expect(useAuthStore.getState().role).toBe('developer')
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('step 2: developer sidebar does not show DevSecOps Stack menu', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.queryByText('DevSecOps Stack')).not.toBeInTheDocument()
  })

  it('step 3: developer sidebar does not show Stack Template link', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.queryByText('Stack Template')).not.toBeInTheDocument()
    expect(screen.queryByText('Stack Install')).not.toBeInTheDocument()
  })

  it('step 3: developer can access CI/CD section - CI/CD List visible', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('CI/CD')).toBeInTheDocument()
    expect(screen.getByText('CI/CD List')).toBeInTheDocument()
    expect(screen.getByText('CI/CD History')).toBeInTheDocument()
  })

  it('step 3: developer cannot access CI/CD Template (devops only)', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.queryByText('CI/CD Template')).not.toBeInTheDocument()
  })

  it('step 4: developer can access Observability - Monitoring Dashboard visible', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByText('Monitoring Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Alert History')).toBeInTheDocument()
  })

  it('step 4: developer cannot see Alert Rules (devops/admin only)', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.queryByText('Alert Rules')).not.toBeInTheDocument()
  })

  it('step 5: admin menu is hidden for developer', () => {
    useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.queryByText('Admin')).not.toBeInTheDocument()
    expect(screen.queryByText('Organization')).not.toBeInTheDocument()
    expect(screen.queryByText('User Management')).not.toBeInTheDocument()
    expect(screen.queryByText('Cluster Management')).not.toBeInTheDocument()
  })

  it('invalid credentials show error message', async () => {
    renderWithProviders(<LoginPage />)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'wrong@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrongpassword' } })
    fireEvent.submit(screen.getByText('Sign in').closest('form')!)

    await vi.waitFor(() => {
      expect(screen.getByText('Invalid email or password.')).toBeInTheDocument()
    })
  })
})

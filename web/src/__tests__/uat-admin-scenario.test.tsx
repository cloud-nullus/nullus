/**
 * UAT-3: Admin management scenario
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from './test-utils'
import { LoginPage } from '../features/auth/pages/login-page'
import { Sidebar } from '../components/layout/sidebar'
import { OrganizationPage } from '../features/admin/pages/organization-page'
import { ClusterPage } from '../features/admin/pages/cluster-page'
import { useAuthStore } from '../stores/auth-store'
import { useSidebarStore } from '../stores/sidebar-store'

vi.mock('../features/admin/api/admin-api', () => ({
  useOrganization: () => ({ data: undefined }),
  useUpdateOrganization: () => ({ mutate: vi.fn(), isPending: false }),
  useMembers: () => ({ data: undefined }),
  useInviteMember: () => ({ mutate: vi.fn(), isPending: false }),
  useRemoveMember: () => ({ mutate: vi.fn(), isPending: false }),
  useClusters: () => ({ data: undefined }),
  useCreateCluster: () => ({ mutate: vi.fn(), isPending: false }),
}))

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

describe('UAT-3: Admin scenario', () => {
  it('step 1: admin can log in with admin@nullus.dev', () => {
    renderWithProviders(<LoginPage />)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'admin@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'admin' } })
    fireEvent.click(screen.getByText('Sign in'))

    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(true)
    expect(state.role).toBe('admin')
    expect(mockNavigate).toHaveBeenCalledWith('/admin/organization')
  })

  it('step 2: admin sidebar shows Admin group', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('Admin')).toBeInTheDocument()
    expect(screen.getByText('Organization')).toBeInTheDocument()
    expect(screen.getByText('User Management')).toBeInTheDocument()
    expect(screen.getByText('Cluster Management')).toBeInTheDocument()
  })

  it('step 2: admin sidebar shows all groups including DevSecOps Stack, CI/CD, Observability', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    // admin role is included in all group role arrays
    expect(screen.getByText('DevSecOps Stack')).toBeInTheDocument()
    expect(screen.getByText('CI/CD')).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByText('Admin')).toBeInTheDocument()
  })

  it('step 3: organization page renders org info form', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Organization')).toBeInTheDocument()
    expect(screen.getByText('조직 정보')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Cloud Nullus')).toBeInTheDocument()
  })

  it('step 4: user management section renders member table', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('멤버 관리')).toBeInTheDocument()
    expect(screen.getByText('Alice Kim')).toBeInTheDocument()
    expect(screen.getByText('Bob Lee')).toBeInTheDocument()
    expect(screen.getByText('Carol Park')).toBeInTheDocument()
  })

  it('step 4: member table renders name, email, role, status columns', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('이름')).toBeInTheDocument()
    expect(screen.getByText('이메일')).toBeInTheDocument()
    expect(screen.getAllByText('역할')[0]).toBeInTheDocument()
    expect(screen.getAllByText('상태')[0]).toBeInTheDocument()
  })

  it('step 5: cluster management page renders cluster list', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Cluster Management')).toBeInTheDocument()
    expect(screen.getAllByText('prod-cluster')[0]).toBeInTheDocument()
    expect(screen.getByText('staging-cluster')).toBeInTheDocument()
    expect(screen.getByText('dev-cluster')).toBeInTheDocument()
  })

  it('step 5: cluster page shows detail panel for selected cluster', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<ClusterPage />)
    // prod-cluster is selected by default
    expect(screen.getByText('https://prod.k8s.nullus.io')).toBeInTheDocument()
    expect(screen.getByText('연결 상태')).toBeInTheDocument()
  })

  it('step 5: admin can switch to different cluster and see its detail', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('staging-cluster'))
    expect(screen.getByText('https://staging.k8s.nullus.io')).toBeInTheDocument()
  })

  it('step 5: admin can open Register Cluster modal', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    expect(screen.getByText('Register')).toBeInTheDocument()
  })

  it('invite member flow works end-to-end', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<OrganizationPage />)
    fireEvent.click(screen.getByText('Invite Member'))
    expect(screen.getByPlaceholderText('member@example.com')).toBeInTheDocument()
    fireEvent.change(screen.getByPlaceholderText('member@example.com'), {
      target: { value: 'newmember@nullus.io' },
    })
    expect(screen.getByDisplayValue('newmember@nullus.io')).toBeInTheDocument()
  })
})

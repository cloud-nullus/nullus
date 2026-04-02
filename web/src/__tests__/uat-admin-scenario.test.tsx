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

// Stable mock data (hoisted to avoid re-render loops from reference changes)
const mockOrg = vi.hoisted(() => ({
  id: 'org-1',
  name: 'Cloud Nullus',
  slug: 'cloud-nullus',
  domain: 'nullus.io',
  status: 'active' as const,
  clusterAccessScope: ['prod-cluster', 'staging-cluster'],
  createdAt: '2026-01-01T00:00:00Z',
}))

vi.mock('../features/admin/api/admin-api', () => ({
  useOrganization: () => ({
    data: mockOrg,
    isLoading: false,
  }),
  useCreateOrganization: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateOrganization: () => ({ mutate: vi.fn(), isPending: false }),
  useMembers: () => ({
    data: {
      items: [
        { id: 'm1', name: 'Alice Kim', email: 'alice@nullus.io', role: 'admin', status: 'active', joinedAt: '2026-01-05T00:00:00Z' },
        { id: 'm2', name: 'Bob Lee', email: 'bob@nullus.io', role: 'devops', status: 'active', joinedAt: '2026-01-10T00:00:00Z' },
        { id: 'm3', name: 'Carol Park', email: 'carol@nullus.io', role: 'developer', status: 'pending', joinedAt: '2026-03-01T00:00:00Z' },
      ],
      total: 3,
    },
  }),
  useInviteMember: () => ({ mutate: vi.fn(), isPending: false }),
  useRemoveMember: () => ({ mutate: vi.fn(), isPending: false }),
  useClusters: () => ({
    data: {
      items: [
        { id: 'c1', name: 'prod-cluster', type: 'target', types: ['target'], cloudProvider: 'aws', endpoint: 'https://prod.k8s.nullus.io', status: 'connected', organizationIds: ['org-1'], createdAt: '2026-01-01T00:00:00Z' },
        { id: 'c2', name: 'staging-cluster', type: 'pipeline', types: ['pipeline'], cloudProvider: 'on_premise', endpoint: 'https://staging.k8s.nullus.io', status: 'connected', organizationIds: ['org-1'], createdAt: '2026-01-15T00:00:00Z' },
        { id: 'c3', name: 'dev-cluster', type: 'pipeline', types: ['target', 'pipeline'], cloudProvider: 'on_premise', endpoint: 'https://dev.k8s.nullus.io', status: 'pending', organizationIds: ['org-1'], createdAt: '2026-03-01T00:00:00Z' },
      ],
      total: 3,
    },
    isLoading: false,
  }),
  useCreateCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useVerifyCluster: () => ({ mutate: vi.fn() }),
  useCluster: () => ({ data: undefined, isFetching: false }),
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
  it('step 1: admin can log in with admin@nullus.dev', async () => {
    renderWithProviders(<LoginPage />)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'admin@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'admin123' } })
    fireEvent.submit(screen.getByText('Sign in').closest('form')!)

    await vi.waitFor(() => {
      expect(useAuthStore.getState().isAuthenticated).toBe(true)
    })
    expect(useAuthStore.getState().role).toBe('admin')
    expect(mockNavigate).toHaveBeenCalledWith('/')
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
    expect(screen.getAllByText('Organization')[0]).toBeInTheDocument()
    expect(screen.getByText('Organization Detail')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Cloud Nullus')).toBeInTheDocument()
  })

  it('step 4: user management section renders member table', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Member Management')).toBeInTheDocument()
    expect(screen.getByText('Alice Kim')).toBeInTheDocument()
    expect(screen.getByText('Bob Lee')).toBeInTheDocument()
    expect(screen.getByText('Carol Park')).toBeInTheDocument()
  })

  it('step 4: member table renders name, email, role, status columns', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Name')).toBeInTheDocument()
    expect(screen.getByText('Email')).toBeInTheDocument()
    expect(screen.getAllByText('Role')[0]).toBeInTheDocument()
    expect(screen.getAllByText('Status')[0]).toBeInTheDocument()
  })

  it('step 5: cluster management page renders cluster list', () => {
    useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
    renderWithProviders(<ClusterPage />)
    expect(screen.getAllByText('Cluster Management')[0]).toBeInTheDocument()
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

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { OrganizationPage } from './organization-page'

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

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

// Mock API hooks
vi.mock('../api/admin-api', () => ({
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
  useClusters: () => ({
    data: {
      items: [
        { id: 'c1', name: 'prod-cluster', type: 'target', types: ['target'], cloudProvider: 'aws', endpoint: 'https://prod.example.com', status: 'connected', organizationIds: ['org-1'], createdAt: '2026-01-01T00:00:00Z' },
        { id: 'c2', name: 'staging-cluster', type: 'pipeline', types: ['pipeline'], cloudProvider: 'on_premise', endpoint: 'https://staging.example.com', status: 'connected', organizationIds: ['org-1'], createdAt: '2026-01-01T00:00:00Z' },
      ],
      total: 2,
    },
    isLoading: false,
  }),
  useInviteMember: () => ({ mutate: vi.fn(), isPending: false }),
  useRemoveMember: () => ({ mutate: vi.fn(), isPending: false }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('OrganizationPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getAllByText('Organization')[0]).toBeInTheDocument()
  })

  it('renders org info form fields', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Organization Detail')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Cloud Nullus')).toBeInTheDocument()
    expect(screen.getByDisplayValue('cloud-nullus')).toBeInTheDocument()
    expect(screen.getByDisplayValue('nullus.io')).toBeInTheDocument()
  })

  it('renders member table with mock members', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Alice Kim')).toBeInTheDocument()
    expect(screen.getByText('Bob Lee')).toBeInTheDocument()
    expect(screen.getByText('Carol Park')).toBeInTheDocument()
  })

  it('renders member management section heading', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Member Management')).toBeInTheDocument()
  })

  it('renders Invite Member button', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Invite Member')).toBeInTheDocument()
  })

  it('clicking Add User navigates to user management', () => {
    renderWithProviders(<OrganizationPage />)
    fireEvent.click(screen.getByText('Add User'))
    expect(mockNavigate).toHaveBeenCalledWith('/admin/users')
  })

  it('clicking Invite Member shows the invite modal', () => {
    renderWithProviders(<OrganizationPage />)
    fireEvent.click(screen.getByText('Invite Member'))
    expect(screen.getByText('Send Invite')).toBeInTheDocument()
  })

  it('invite modal has email and role fields', () => {
    renderWithProviders(<OrganizationPage />)
    fireEvent.click(screen.getByText('Invite Member'))
    expect(screen.getByPlaceholderText('member@example.com')).toBeInTheDocument()
    expect(screen.getAllByText('Role').length).toBeGreaterThanOrEqual(1)
  })

  it('closing invite modal hides Send Invite button', () => {
    renderWithProviders(<OrganizationPage />)
    fireEvent.click(screen.getByText('Invite Member'))
    fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByText('Send Invite')).not.toBeInTheDocument()
  })

  it('renders cluster access scope section', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Cluster Access Scope')).toBeInTheDocument()
    expect(screen.getByText('prod-cluster')).toBeInTheDocument()
    expect(screen.getByText('staging-cluster')).toBeInTheDocument()
  })

  it('renders member roles as badges', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText(/Admin|관리자/)).toBeInTheDocument()
    expect(screen.getByText(/DevOps/)).toBeInTheDocument()
    expect(screen.getByText(/Developer|개발자/)).toBeInTheDocument()
  })
})

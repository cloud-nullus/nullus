import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { OrganizationPage } from './organization-page'

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
        { id: 'c1', name: 'prod-cluster', type: 'eks', status: 'connected' },
        { id: 'c2', name: 'staging-cluster', type: 'kubernetes', status: 'connected' },
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
    expect(screen.getByText('admin')).toBeInTheDocument()
    expect(screen.getByText('devops')).toBeInTheDocument()
    expect(screen.getByText('developer')).toBeInTheDocument()
  })
})

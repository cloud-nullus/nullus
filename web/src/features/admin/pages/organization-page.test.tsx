import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { OrganizationPage } from './organization-page'

// Mock API hooks
vi.mock('../api/admin-api', () => ({
  useOrganization: () => ({ data: undefined }),
  useUpdateOrganization: () => ({ mutate: vi.fn(), isPending: false }),
  useMembers: () => ({ data: undefined }),
  useInviteMember: () => ({ mutate: vi.fn(), isPending: false }),
  useRemoveMember: () => ({ mutate: vi.fn(), isPending: false }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('OrganizationPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('Organization')).toBeInTheDocument()
  })

  it('renders org info form fields', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('조직 정보')).toBeInTheDocument()
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
    expect(screen.getByText('멤버 관리')).toBeInTheDocument()
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
    expect(screen.getAllByText('역할').length).toBeGreaterThanOrEqual(1)
  })

  it('closing invite modal hides Send Invite button', () => {
    renderWithProviders(<OrganizationPage />)
    fireEvent.click(screen.getByText('Invite Member'))
    fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByText('Send Invite')).not.toBeInTheDocument()
  })

  it('renders cluster access scope section', () => {
    renderWithProviders(<OrganizationPage />)
    expect(screen.getByText('클러스터 접근 범위')).toBeInTheDocument()
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

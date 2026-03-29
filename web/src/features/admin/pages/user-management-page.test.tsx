import { beforeEach, describe, expect, it, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { UserManagementPage } from './user-management-page'

const mockNavigate = vi.hoisted(() => vi.fn())
const mockUseOrganization = vi.hoisted(() => vi.fn())
const mockUseMembers = vi.hoisted(() => vi.fn())
const mockUseInviteMember = vi.hoisted(() => vi.fn())
const mockUseUpdateUserRole = vi.hoisted(() => vi.fn())
const mockUseUpdateMember = vi.hoisted(() => vi.fn())
const mockUseDeactivateUser = vi.hoisted(() => vi.fn())
const mockUseCreateInviteLink = vi.hoisted(() => vi.fn())
const mockUseInviteLinks = vi.hoisted(() => vi.fn())
const mockUseRevokeInviteLink = vi.hoisted(() => vi.fn())
const mockUseSearchUser = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({ pathname: '/admin/user-management', search: '', hash: '', state: null, key: 'test' }),
  }
})

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: vi.fn(() => ({ role: 'admin', user: null, isAuthenticated: true })),
}))

vi.mock('../api/admin-api', () => ({
  useOrganization: (...args: unknown[]) => mockUseOrganization(...args),
  useMembers: (...args: unknown[]) => mockUseMembers(...args),
  useInviteMember: (...args: unknown[]) => mockUseInviteMember(...args),
  useUpdateUserRole: (...args: unknown[]) => mockUseUpdateUserRole(...args),
  useUpdateMember: (...args: unknown[]) => mockUseUpdateMember(...args),
  useDeactivateUser: (...args: unknown[]) => mockUseDeactivateUser(...args),
  useCreateInviteLink: (...args: unknown[]) => mockUseCreateInviteLink(...args),
  useInviteLinks: (...args: unknown[]) => mockUseInviteLinks(...args),
  useRevokeInviteLink: (...args: unknown[]) => mockUseRevokeInviteLink(...args),
  useSearchUser: (...args: unknown[]) => mockUseSearchUser(...args),
}))

describe('UserManagementPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    mockUseOrganization.mockReturnValue({ data: { id: 'org-1' } })
    mockUseMembers.mockReturnValue({
      data: {
        items: [
          {
            id: 'm1',
            name: 'Alice Kim',
            email: 'alice@nullus.io',
            role: 'admin',
            status: 'active',
            joinedAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 1,
      },
      isLoading: false,
    })
    mockUseInviteMember.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseUpdateUserRole.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseUpdateMember.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseDeactivateUser.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseCreateInviteLink.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseInviteLinks.mockReturnValue({ data: { items: [] } })
    mockUseRevokeInviteLink.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseSearchUser.mockReturnValue({ data: { found: false }, isFetching: false })
  })

  it('renders without crash', () => {
    renderWithProviders(<UserManagementPage />)

    expect(screen.getAllByText('User Management').length).toBeGreaterThan(0)
  })

  it('shows loading state in users tab', () => {
    mockUseMembers.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<UserManagementPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Users' }))

    expect(screen.queryAllByText('Loading users...').length).toBeGreaterThan(0)
  })

  it('renders user data in users tab', () => {
    renderWithProviders(<UserManagementPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Users' }))

    expect(screen.getByText('Alice Kim')).toBeInTheDocument()
    expect(screen.getByText('alice@nullus.io')).toBeInTheDocument()
    expect(screen.getByText('Pending Invites')).toBeInTheDocument()
  })

  it('shows empty state when no users exist', () => {
    mockUseMembers.mockReturnValue({ data: { items: [], total: 0 }, isLoading: false })

    renderWithProviders(<UserManagementPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Users' }))

    expect(screen.queryAllByText(/No users found\.|사용자가 없습니다\./).length).toBeGreaterThan(0)
  })

  it('renders edit action button in users table', () => {
    renderWithProviders(<UserManagementPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Users' }))

    expect(screen.getByRole('button', { name: /Edit/i })).toBeInTheDocument()
  })
})

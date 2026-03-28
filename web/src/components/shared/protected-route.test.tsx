import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProtectedRoute } from './protected-route'
import type { Role } from '../../types'

const mockUseAuth = vi.fn()
const mockUseAuthStore = vi.fn()

const mockOidcConfig = {
  isOidcMode: false,
}

vi.mock('react-router-dom', () => ({
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate" data-to={to} />,
  Outlet: () => <div data-testid="outlet">protected content</div>,
}))

vi.mock('react-oidc-context', () => ({
  useAuth: () => mockUseAuth(),
}))

vi.mock('../../lib/oidc-config', () => ({
  get isOidcMode() {
    return mockOidcConfig.isOidcMode
  },
}))

vi.mock('../../stores/auth-store', () => ({
  useAuthStore: (selector: (state: { role: Role; isAuthenticated: boolean }) => unknown) =>
    mockUseAuthStore(selector),
  extractRoleFromOidc: () => 'developer' as Role,
  getHomePathForRole: (role: Role) => (role === 'admin' ? '/admin/organization' : '/'),
}))

describe('ProtectedRoute', () => {
  beforeEach(() => {
    mockOidcConfig.isOidcMode = false
    mockUseAuth.mockReset()
    mockUseAuthStore.mockReset()
  })

  it('renders outlet for authenticated and authorized user in mock mode', () => {
    const authState = { role: 'devops' as Role, isAuthenticated: true }
    mockUseAuthStore.mockImplementation((selector) => selector(authState))

    render(<ProtectedRoute allowedRoles={['devops']} />)

    expect(screen.getByTestId('outlet')).not.toBeNull()
  })

  it('redirects unauthenticated user to login in mock mode', () => {
    const authState = { role: 'developer' as Role, isAuthenticated: false }
    mockUseAuthStore.mockImplementation((selector) => selector(authState))

    render(<ProtectedRoute allowedRoles={['developer']} />)

    expect(screen.getByTestId('navigate').getAttribute('data-to')).toBe('/login')
  })

  it('redirects unauthorized user in mock mode', () => {
    const authState = { role: 'developer' as Role, isAuthenticated: true }
    mockUseAuthStore.mockImplementation((selector) => selector(authState))

    render(<ProtectedRoute allowedRoles={['admin']} />)

    expect(screen.getByTestId('navigate').getAttribute('data-to')).toBe('/')
  })

  it('redirects unauthorized user in OIDC mode', () => {
    mockOidcConfig.isOidcMode = true
    mockUseAuth.mockReturnValue({
      isLoading: false,
      activeNavigator: null,
      isAuthenticated: true,
      user: { profile: { roles: ['developer'] } },
    })

    render(<ProtectedRoute allowedRoles={['admin']} />)

    expect(screen.getByTestId('navigate').getAttribute('data-to')).toBe('/')
  })
})

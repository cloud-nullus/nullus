import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { LoginPage } from './login-page'

const mockNavigate = vi.fn()
const mockLogin = vi.fn()

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: (selector: (state: { login: (user: unknown) => void }) => unknown) =>
    selector({ login: mockLogin }),
}))

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../../../lib/oidc-providers', () => ({
  isOidcMode: false,
  getProviderConfig: () => ({ type: 'keycloak' }),
}))

describe('LoginPage', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    mockLogin.mockClear()
  })

  it('renders without crashing', () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByText('Nullus Platform')).toBeTruthy()
    expect(screen.getByText('Sign in to your account')).toBeTruthy()
  })

  it('shows login form fields', () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByLabelText('Email')).toBeTruthy()
    expect(screen.getByLabelText('Password')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeTruthy()
  })

  it('shows test accounts section in mock auth mode', () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByText('Test Accounts')).toBeTruthy()
    expect(screen.getByText('admin@nullus.dev / admin123')).toBeTruthy()
    expect(screen.getByText('devops@nullus.dev / devops123')).toBeTruthy()
    expect(screen.getByText('developer@nullus.dev / developer123')).toBeTruthy()
  })

  it('shows role labels for test accounts', () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByText(/admin@nullus\.dev/)).toBeTruthy()
    expect(screen.getByText(/devops@nullus\.dev/)).toBeTruthy()
    expect(screen.getByText(/developer@nullus\.dev/)).toBeTruthy()
  })

  it('submits valid credentials and calls auth handler', async () => {
    renderWithProviders(<LoginPage />)

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'devops@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'devops123' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Sign in' }).closest('form')!)

    await vi.waitFor(() => {
      expect(mockLogin).toHaveBeenCalledTimes(1)
    })

    expect(mockLogin).toHaveBeenCalledWith(
      expect.objectContaining({
        email: 'devops@nullus.dev',
        role: 'devops',
        orgId: '11111111-1111-1111-1111-111111111111',
      }),
    )
    expect(mockNavigate).toHaveBeenCalledWith('/stack/templates')
  })
})

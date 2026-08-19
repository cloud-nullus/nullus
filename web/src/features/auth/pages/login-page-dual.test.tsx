import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { LoginPage } from './login-page'

// OIDC 모드에서도 ID/PW 로 들어갈 수단이 있어야 한다. 예전에는 두 경로가 배타적이라
// (isOidcMode ? OIDC : mock) IdP 가 죽으면 아무도 로그인할 수 없었다.

const mockLogin = vi.fn()
const mockSigninRedirect = vi.fn()
const mockLoginWithPassword = vi.fn()

let authState: Record<string, unknown> = {}

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: (selector: (s: { login: unknown }) => unknown) => selector({ login: mockLogin }),
}))

vi.mock('../../../lib/oidc-providers', () => ({
  isOidcMode: true,
  getProviderConfig: () => ({ type: 'keycloak' }),
}))

vi.mock('react-oidc-context', () => ({
  useAuth: () => ({
    isLoading: false,
    isAuthenticated: false,
    activeNavigator: undefined,
    error: undefined,
    signinRedirect: mockSigninRedirect,
    ...authState,
  }),
}))

vi.mock('../api/auth-api', () => ({
  loginWithPassword: (...args: unknown[]) => mockLoginWithPassword(...args),
}))

describe('LoginPage — OIDC 모드의 이중 경로', () => {
  beforeEach(() => {
    authState = {}
    mockLogin.mockClear()
    mockSigninRedirect.mockClear()
    mockLoginWithPassword.mockReset()
  })

  it('IdP 로그인 버튼을 제공한다', () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByRole('button', { name: /Sign in with Keycloak/i })).toBeTruthy()
  })

  it('비밀번호 로그인으로 전환할 수단을 제공한다', () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByRole('button', { name: /Sign in with a password/i })).toBeTruthy()
  })

  it('전환하면 이메일·비밀번호 입력이 나타난다', async () => {
    renderWithProviders(<LoginPage />)
    fireEvent.click(screen.getByRole('button', { name: /Sign in with a password/i }))

    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeTruthy()
      expect(screen.getByLabelText('Password')).toBeTruthy()
    })
  })

  // IdP 가 깨졌을 때가 정확히 이 경로가 필요한 순간이다. 사용자가 전환 버튼을
  // 찾아내야만 들어갈 수 있다면 break-glass 라고 할 수 없다.
  it('IdP 오류가 나면 비밀번호 입력을 바로 보여준다', async () => {
    authState = { error: new Error('Keycloak unreachable') }
    renderWithProviders(<LoginPage />)

    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeTruthy()
    })
  })

  it('제출하면 로그인 API 를 부르고 토큰과 사용자를 저장한다', async () => {
    mockLoginWithPassword.mockResolvedValue({
      token: 'signed-token',
      user: { id: 'u-1', email: 'admin@nullus.dev', name: 'Admin', role: 'admin', orgId: 'org-1' },
    })

    renderWithProviders(<LoginPage />)
    fireEvent.click(screen.getByRole('button', { name: /Sign in with a password/i }))

    await waitFor(() => expect(screen.getByLabelText('Email')).toBeTruthy())
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'admin@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct-horse-battery' } })
    fireEvent.submit(screen.getByLabelText('Email').closest('form')!)

    await waitFor(() => {
      expect(mockLoginWithPassword).toHaveBeenCalledWith('admin@nullus.dev', 'correct-horse-battery')
      expect(mockLogin).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'u-1', role: 'admin', orgId: 'org-1' }),
        'signed-token',
      )
    })
  })

  it('자격이 틀리면 오류를 보여주고 로그인하지 않는다', async () => {
    mockLoginWithPassword.mockRejectedValue(new Error('invalid email or password'))

    renderWithProviders(<LoginPage />)
    fireEvent.click(screen.getByRole('button', { name: /Sign in with a password/i }))
    await waitFor(() => expect(screen.getByLabelText('Email')).toBeTruthy())

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'admin@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong-password' } })
    fireEvent.submit(screen.getByLabelText('Email').closest('form')!)

    await waitFor(() => {
      expect(screen.getByText(/invalid email or password/i)).toBeTruthy()
    })
    expect(mockLogin).not.toHaveBeenCalled()
  })

  // 전환했는데도 뒤에서 IdP 로 튕겨 나가면 폼을 쓸 수 없다.
  it('비밀번호 경로로 전환하면 IdP 자동 이동을 멈춘다', async () => {
    renderWithProviders(<LoginPage />)
    fireEvent.click(screen.getByRole('button', { name: /Sign in with a password/i }))
    mockSigninRedirect.mockClear()

    await waitFor(() => expect(screen.getByLabelText('Email')).toBeTruthy())
    expect(mockSigninRedirect).not.toHaveBeenCalled()
  })
})

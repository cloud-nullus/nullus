import { RouterProvider } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import { router } from './app/routes'
import { ToastProvider } from './components/ui/toast-provider'
import { queryClient } from './lib/query-client'
import i18n from './i18n'

import { Component, useEffect, type ErrorInfo, type ReactNode } from 'react'
import { useAuth } from 'react-oidc-context'
import { useAuthStore, extractRoleFromOidc } from './stores/auth-store'
import { isOidcMode } from './lib/oidc-config'
import type { User } from './types'

const ORG_ID = '11111111-1111-1111-1111-111111111111'

function Splash({ text }: { text: string }) {
  return (
    <div className="flex h-screen items-center justify-center bg-[var(--color-surface-base)] text-[var(--color-text-secondary)]">
      {text}
    </div>
  )
}

// 화면이 비는(blank) 원인을 노출하기 위한 최상위 에러 바운더리 — 콘솔 없이도 에러 표시
class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state: { error: Error | null } = { error: null }
  static getDerivedStateFromError(error: Error) {
    return { error }
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    // 콘솔에도 남김
    console.error('[AppErrorBoundary]', error, info)
  }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 24, fontFamily: 'monospace', color: '#f87171', background: '#0b0e14', minHeight: '100vh' }}>
          <h2 style={{ color: '#fca5a5' }}>App render error</h2>
          <pre style={{ whiteSpace: 'pre-wrap' }}>{this.state.error.message}</pre>
          <pre style={{ whiteSpace: 'pre-wrap', color: '#9ca3af', fontSize: 12 }}>{this.state.error.stack}</pre>
        </div>
      )
    }
    return this.props.children
  }
}

// OIDC 모드: react-oidc-context 사용자 → auth-store 브릿지 + 동기화 완료 전 렌더 차단
function OidcGate({ children }: { children: ReactNode }) {
  const auth = useAuth()
  const login = useAuthStore((s) => s.login)
  const logout = useAuthStore((s) => s.logout)
  const storeUser = useAuthStore((s) => s.user)

  useEffect(() => {
    if (auth.isAuthenticated && auth.user) {
      const role = extractRoleFromOidc(auth.user)
      const user: User = {
        id: auth.user.profile.sub || '',
        name: (auth.user.profile.name || auth.user.profile.preferred_username || 'OIDC User') as string,
        email: (auth.user.profile.email || '') as string,
        role,
        orgId: ORG_ID,
      }
      login(user, auth.user.access_token)
    } else if (!auth.isLoading && !auth.isAuthenticated && storeUser) {
      logout()
    }
  }, [auth.isAuthenticated, auth.user, auth.isLoading, login, logout, storeUser])

  if (auth.error) return <Splash text={`로그인 오류: ${auth.error.message}`} />
  if (auth.isLoading || auth.activeNavigator) return <Splash text="Authenticating…" />
  if (auth.isAuthenticated && !storeUser) return <Splash text="Loading…" />

  return <>{children}</>
}

function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          {isOidcMode ? (
            <OidcGate>
              <RouterProvider router={router} />
            </OidcGate>
          ) : (
            <RouterProvider router={router} />
          )}
          <ToastProvider />
        </I18nextProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App

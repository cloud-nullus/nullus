import { RouterProvider } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import { router } from './app/routes'
import { ToastProvider } from './components/ui/toast-provider'
import { queryClient } from './lib/query-client'
import i18n from './i18n'

import { useEffect, type ReactNode } from 'react'
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

// OIDC 모드 전용: react-oidc-context 사용자 → auth-store 브릿지 + 동기화 완료 전 렌더 차단.
// (가드/레이아웃/페이지가 store.user 를 읽으므로, 동기화 전 렌더하면 null 참조로 화면이 비는 문제 방지)
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

  if (auth.isLoading || auth.activeNavigator) return <Splash text="Authenticating…" />
  // 인증은 됐으나 store 브릿지(useEffect)가 아직 user 를 채우지 못한 구간 — 라우터 렌더 보류
  if (auth.isAuthenticated && !storeUser) return <Splash text="Loading…" />

  return <>{children}</>
}

function App() {
  return (
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
  )
}

export default App

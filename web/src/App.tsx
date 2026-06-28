import { RouterProvider } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import { router } from './app/routes'
import { ToastProvider } from './components/ui/toast-provider'
import { queryClient } from './lib/query-client'
import i18n from './i18n'

import { useEffect } from 'react'
import { useAuth } from 'react-oidc-context'
import { useAuthStore, extractRoleFromOidc } from './stores/auth-store'
import { isOidcMode } from './lib/oidc-config'
import type { User } from './types'

const ORG_ID = '11111111-1111-1111-1111-111111111111'

function OidcSync() {
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

  return null
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        {isOidcMode && <OidcSync />}
        <RouterProvider router={router} />
        <ToastProvider />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

export default App

import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from 'react-oidc-context'
import { isOidcMode } from '../../lib/oidc-config'
import { extractRoleFromOidc, getHomePathForRole, useAuthStore } from '../../stores/auth-store'
import type { Role } from '../../types'

interface ProtectedRouteProps {
  allowedRoles?: Role[]
}

export function ProtectedRoute({ allowedRoles }: ProtectedRouteProps) {
  if (isOidcMode) {
    return <OidcProtectedRoute allowedRoles={allowedRoles} />
  }

  return <MockProtectedRoute allowedRoles={allowedRoles} />
}

function MockProtectedRoute({ allowedRoles }: ProtectedRouteProps) {
  const role = useAuthStore((state) => state.role)
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  if (allowedRoles && !allowedRoles.includes(role)) {
    return <Navigate to="/" replace />
  }

  return <Outlet />
}

function OidcProtectedRoute({ allowedRoles }: ProtectedRouteProps) {
  const auth = useAuth()
  const location = useLocation()

  if (auth.isLoading || auth.activeNavigator) {
    return (
      <div className="flex h-[200px] items-center justify-center text-[var(--color-text-secondary)]">
        Authenticating...
      </div>
    )
  }

  if (!auth.isAuthenticated) {
    // 로그인 뒤 이 화면으로 돌아오도록 열려던 경로를 넘긴다 — 로그인 화면이 IdP state 에 싣는다.
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search + location.hash }} />
  }

  const role = auth.user ? extractRoleFromOidc(auth.user) : 'developer' as Role
  if (allowedRoles && !allowedRoles.includes(role)) {
    return <Navigate to={getHomePathForRole(role)} replace />
  }

  return <Outlet />
}

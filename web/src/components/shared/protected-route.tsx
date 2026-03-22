import { Navigate, Outlet } from 'react-router-dom'
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

  if (auth.isLoading || auth.activeNavigator) {
    return (
      <div className="flex h-[200px] items-center justify-center text-[var(--color-text-secondary)]">
        Authenticating...
      </div>
    )
  }

  if (!auth.isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  const role = auth.user ? extractRoleFromOidc(auth.user) : 'developer' as Role
  if (allowedRoles && !allowedRoles.includes(role)) {
    return <Navigate to={getHomePathForRole(role)} replace />
  }

  return <Outlet />
}

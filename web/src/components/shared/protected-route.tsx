import { Navigate, Outlet } from 'react-router-dom'
import { useAuthStore } from '../../stores/auth-store'
import type { Role } from '../../types'

interface ProtectedRouteProps {
  allowedRoles?: Role[]
}

export function ProtectedRoute({ allowedRoles }: ProtectedRouteProps) {
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

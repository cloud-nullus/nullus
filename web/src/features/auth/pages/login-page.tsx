import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAuth } from 'react-oidc-context'
import { isOidcMode } from '../../../lib/oidc-config'
import { extractRoleFromOidc, getHomePathForRole, useAuthStore } from '../../../stores/auth-store'
import type { User } from '../../../types'

const loginSchema = z.object({
  email: z.string().min(1, 'Email is required').email('Invalid email format'),
  password: z.string().min(1, 'Password is required').min(6, 'Password must be at least 6 characters'),
})

type LoginFormData = z.infer<typeof loginSchema>

const ORG_ID = '89226bdf-96fd-4d12-9ff5-c1d4f555c5ad'

const TEST_ACCOUNTS: Record<string, { password: string; user: User }> = {
  'admin@nullus.dev': {
    password: 'admin123',
    user: { id: '1', name: 'Admin User', email: 'admin@nullus.dev', role: 'admin', orgId: ORG_ID },
  },
  'devops@nullus.dev': {
    password: 'devops123',
    user: { id: '2', name: 'DevOps Engineer', email: 'devops@nullus.dev', role: 'devops', orgId: ORG_ID },
  },
  'developer@nullus.dev': {
    password: 'developer123',
    user: { id: '3', name: 'Developer', email: 'developer@nullus.dev', role: 'developer', orgId: ORG_ID },
  },
}

export function LoginPage() {
  if (isOidcMode) {
    return <OidcLoginPage />
  }
  return <MockLoginPage />
}

function OidcLoginPage() {
  const auth = useAuth()

  useEffect(() => {
    if (!auth.isLoading && !auth.isAuthenticated && !auth.activeNavigator) {
      void auth.signinRedirect()
    }
  }, [auth.activeNavigator, auth.isAuthenticated, auth.isLoading, auth.signinRedirect])

  if (auth.error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[var(--color-surface-base)] p-6">
        <div className="w-full max-w-[520px] rounded-2xl border border-[rgba(239,68,68,0.35)] bg-[var(--color-surface-card)] p-8 text-center">
          <h1 className="text-xl font-bold text-[var(--color-text-primary)]">Keycloak connection failed</h1>
          <p className="mt-3 text-sm text-[var(--color-text-secondary)]">
            Unable to reach the identity provider. Check Keycloak URL and realm settings, then try again.
          </p>
          <button
            type="button"
            onClick={() => void auth.signinRedirect()}
            className="mt-5 rounded-[10px] border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] px-4 py-2.5 text-sm font-bold text-[#1a1d29]"
          >
            Retry sign-in
          </button>
        </div>
      </div>
    )
  }

  if (auth.isAuthenticated) {
    const role = extractRoleFromOidc(auth.user)
    return <Navigate to={getHomePathForRole(role)} replace />
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-surface-base)] p-6">
      <div className="w-full max-w-[420px] rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-8 text-center">
        <h1 className="text-lg font-bold text-[var(--color-text-primary)]">Redirecting to Keycloak</h1>
        <p className="mt-3 text-sm text-[var(--color-text-secondary)]">
          {auth.activeNavigator ? 'Completing authentication...' : 'Starting secure sign-in...'}
        </p>
      </div>
    </div>
  )
}

function MockLoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const [error, setError] = useState<string | null>(null)
  const {
    register,
    handleSubmit,
    formState: { errors, isValid, isSubmitting },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: '', password: '' },
    mode: 'onChange',
  })

  const onSubmit = (data: LoginFormData) => {
    setError(null)

    const account = TEST_ACCOUNTS[data.email]
    if (!account || account.password !== data.password) {
      setError('Invalid email or password.')
      return
    }

    login(account.user)
    navigate(getHomePathForRole(account.user.role))
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-surface-base)] p-6">
      <div className="w-full max-w-[400px] rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-10">
        {/* Logo / Title */}
        <div className="mb-8 text-center">
          <div className="mx-auto mb-[14px] flex h-12 w-12 items-center justify-center rounded-xl bg-[linear-gradient(135deg,#ffd700,#f59e0b)] text-[22px] font-extrabold text-[#1a1d29]">
            N
          </div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Nullus Platform
          </h1>
          <p className="mb-0 mt-1.5 text-[13px] text-[var(--color-text-secondary)]">
            Sign in to your account
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1">
            <label htmlFor="email" className="text-xs font-medium text-[var(--color-text-secondary)]">
              Email
            </label>
            <input
              id="email"
              type="email"
              {...register('email')}
              placeholder="you@nullus.dev"
              className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)] outline-none"
            />
            {errors.email && <span className="text-xs text-[#ef4444]">{errors.email.message}</span>}
          </div>

          <div className="flex flex-col gap-1">
            <label htmlFor="password" className="text-xs font-medium text-[var(--color-text-secondary)]">
              Password
            </label>
            <input
              id="password"
              type="password"
              {...register('password')}
              placeholder="••••••••"
              className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2.5 text-sm text-[var(--color-text-primary)] outline-none"
            />
            {errors.password && <span className="text-xs text-[#ef4444]">{errors.password.message}</span>}
          </div>

          {error && (
            <div className="rounded-lg border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.1)] px-3 py-2.5 text-[13px] text-[#f87171]">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={!isValid || isSubmitting}
            className="mt-1 rounded-[10px] border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] p-3 text-sm font-bold text-[#1a1d29] disabled:cursor-not-allowed disabled:opacity-60"
          >
            Sign in
          </button>
        </form>

        {/* Hint */}
        <div className="mt-6 rounded-lg border border-[rgba(99,102,241,0.2)] bg-[rgba(99,102,241,0.06)] p-[14px] text-xs leading-[1.6] text-[var(--color-text-secondary)]">
          <div className="mb-1.5 font-semibold text-[#a5b4fc]">Test Accounts</div>
          <div>admin@nullus.dev / admin123</div>
          <div>devops@nullus.dev / devops123</div>
          <div>developer@nullus.dev / developer123</div>
        </div>
      </div>
    </div>
  )
}

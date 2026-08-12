import { useState, useEffect, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../../../stores/auth-store'
import { roleLandingPath } from '../role-landing'
import { isOidcMode, getProviderConfig } from '../../../lib/oidc-providers'
import { NullusMark } from '../../../components/brand/nullus-mark'
import type { User } from '../../../types'

const loginSchema = z.object({
  email: z.string().min(1, 'Email is required').email('Invalid email format'),
  password: z.string().min(1, 'Password is required').min(6, 'Password must be at least 6 characters'),
})

type LoginFormData = z.infer<typeof loginSchema>

// Must match the single local seed organization used by mock auth.
const ORG_ID = '11111111-1111-1111-1111-111111111111'

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


import { useAuth } from 'react-oidc-context'
import { TextInput } from '../../../components/ui/text-input'

function OidcLoginContent() {
  const auth = useAuth()
  const providerLabel = getProviderConfig().type === 'authentik' ? 'Authentik' : 'Keycloak'
  const triedRef = useRef(false)

  // Seamless SSO: 로그인 안 됐으면 자동으로 Keycloak 으로 redirect (버튼 클릭 불필요)
  useEffect(() => {
    if (
      !auth.isLoading &&
      !auth.isAuthenticated &&
      !auth.activeNavigator &&
      !auth.error &&
      !triedRef.current
    ) {
      triedRef.current = true
      void auth.signinRedirect()
    }
  }, [auth.isLoading, auth.isAuthenticated, auth.activeNavigator, auth.error])

  return (
    <>
      <p className="mb-4 text-center text-[13px] text-[var(--color-text-secondary)]">
        {auth.error
          ? `${providerLabel} 로그인 중 오류가 발생했습니다.`
          : `${providerLabel}(으)로 이동 중입니다…`}
      </p>
      {auth.error && (
        <div className="mb-4 rounded-lg border border-[color-mix(in_srgb,_var(--color-error)_30%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_10%,_transparent)] px-3 py-2.5 text-[13px] text-[var(--color-error)]">
          {auth.error.message}
        </div>
      )}
      <button
        type="button"
        onClick={() => {
          triedRef.current = true
          void auth.signinRedirect()
        }}
        className="w-full rounded-[10px] border-none bg-[var(--color-primary)] p-3 text-sm font-bold text-[var(--color-on-primary)]"
      >
        Sign in with {providerLabel}
      </button>
    </>
  )
}

function MockLoginContent() {
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
    navigate(roleLandingPath(account.user.role))
  }

  return (
    <>
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="email" className="text-xs font-medium text-[var(--color-text-secondary)]">
            Email
          </label>
          <TextInput
            id="email"
            type="email"
            {...register('email')}
            placeholder="you@nullus.dev"
          />
          {errors.email && <span className="text-xs text-[var(--color-error)]">{errors.email.message}</span>}
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="password" className="text-xs font-medium text-[var(--color-text-secondary)]">
            Password
          </label>
          <TextInput
            id="password"
            type="password"
            {...register('password')}
            placeholder="••••••••"
          />
          {errors.password && <span className="text-xs text-[var(--color-error)]">{errors.password.message}</span>}
        </div>

        {error && (
          <div className="rounded-lg border border-[color-mix(in_srgb,_var(--color-error)_30%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_10%,_transparent)] px-3 py-2.5 text-[13px] text-[var(--color-error)]">
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={!isValid || isSubmitting}
          className="mt-1 rounded-[10px] border-none bg-[var(--color-primary)] p-3 text-sm font-bold text-[var(--color-on-primary)] disabled:cursor-not-allowed disabled:opacity-60"
        >
          Sign in
        </button>
      </form>

      <div className="mt-6 rounded-lg border border-[color-mix(in_srgb,_var(--color-primary)_20%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_6%,_transparent)] p-[14px] text-xs leading-[1.6] text-[var(--color-text-secondary)]">
        <div className="mb-1.5 font-semibold text-[var(--color-primary)]">Test Accounts</div>
        <div>admin@nullus.dev / admin123</div>
        <div>devops@nullus.dev / devops123</div>
        <div>developer@nullus.dev / developer123</div>
      </div>
    </>
  )
}

export function LoginPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-surface-base)] p-6">
      <div className="w-full max-w-[400px] rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-10">
        <div className="mb-8 text-center">
          <NullusMark size={52} decorative className="mx-auto mb-[14px] block" />
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Nullus Platform
          </h1>
          <p className="mb-0 mt-1.5 text-[13px] text-[var(--color-text-secondary)]">
            Sign in to your account
          </p>
        </div>

        {isOidcMode ? <OidcLoginContent /> : <MockLoginContent />}
      </div>
    </div>
  )
}

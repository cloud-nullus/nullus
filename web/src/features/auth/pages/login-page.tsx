import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../../../stores/auth-store'
import type { User } from '../../../types'

const TEST_ACCOUNTS: Record<string, { password: string; user: User }> = {
  'admin@nullus.dev': {
    password: 'admin',
    user: { id: '1', name: 'Admin User', email: 'admin@nullus.dev', role: 'admin' },
  },
  'devops@nullus.dev': {
    password: 'devops',
    user: { id: '2', name: 'DevOps Engineer', email: 'devops@nullus.dev', role: 'devops' },
  },
  'developer@nullus.dev': {
    password: 'developer',
    user: { id: '3', name: 'Developer', email: 'developer@nullus.dev', role: 'developer' },
  },
}

const ROLE_HOME: Record<string, string> = {
  admin: '/admin/organization',
  devops: '/stack/templates',
  developer: '/cicd/developer-deploy',
}

export function LoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const account = TEST_ACCOUNTS[email]
    if (!account || account.password !== password) {
      setError('Invalid email or password.')
      return
    }

    login(account.user)
    navigate(ROLE_HOME[account.user.role] ?? '/')
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--color-surface-base)',
        padding: '24px',
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: '400px',
          background: 'var(--color-surface-card)',
          border: '1px solid var(--color-border-default)',
          borderRadius: '16px',
          padding: '40px',
        }}
      >
        {/* Logo / Title */}
        <div style={{ textAlign: 'center', marginBottom: '32px' }}>
          <div
            style={{
              width: '48px',
              height: '48px',
              background: 'linear-gradient(135deg, #ffd700, #f59e0b)',
              borderRadius: '12px',
              margin: '0 auto 14px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '22px',
              fontWeight: 800,
              color: '#1a1d29',
            }}
          >
            N
          </div>
          <h1
            style={{
              margin: 0,
              fontSize: '22px',
              fontWeight: 800,
              color: 'var(--color-text-primary)',
            }}
          >
            Nullus Platform
          </h1>
          <p
            style={{
              margin: '6px 0 0',
              fontSize: '13px',
              color: 'var(--color-text-secondary)',
            }}
          >
            Sign in to your account
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label
              htmlFor="email"
              style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}
            >
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@nullus.dev"
              required
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '8px',
                padding: '10px 12px',
                fontSize: '14px',
                color: 'var(--color-text-primary)',
                outline: 'none',
              }}
            />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label
              htmlFor="password"
              style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '8px',
                padding: '10px 12px',
                fontSize: '14px',
                color: 'var(--color-text-primary)',
                outline: 'none',
              }}
            />
          </div>

          {error && (
            <div
              style={{
                background: 'rgba(239,68,68,0.1)',
                border: '1px solid rgba(239,68,68,0.3)',
                borderRadius: '8px',
                padding: '10px 12px',
                fontSize: '13px',
                color: '#f87171',
              }}
            >
              {error}
            </div>
          )}

          <button
            type="submit"
            style={{
              background: 'linear-gradient(135deg, #ffd700, #f59e0b)',
              color: '#1a1d29',
              border: 'none',
              borderRadius: '10px',
              padding: '12px',
              fontSize: '14px',
              fontWeight: 700,
              cursor: 'pointer',
              marginTop: '4px',
            }}
          >
            Sign in
          </button>
        </form>

        {/* Hint */}
        <div
          style={{
            marginTop: '24px',
            padding: '14px',
            background: 'rgba(99,102,241,0.06)',
            border: '1px solid rgba(99,102,241,0.2)',
            borderRadius: '8px',
            fontSize: '12px',
            color: 'var(--color-text-secondary)',
            lineHeight: '1.6',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: '6px', color: '#a5b4fc' }}>Test Accounts</div>
          <div>admin@nullus.dev / admin</div>
          <div>devops@nullus.dev / devops</div>
          <div>developer@nullus.dev / developer</div>
        </div>
      </div>
    </div>
  )
}

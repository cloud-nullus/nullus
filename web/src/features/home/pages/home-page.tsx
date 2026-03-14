import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../../../stores/auth-store'
import { Rocket } from 'lucide-react'
import type { Role } from '../../../types'

const greetingKeys: Record<Role, string> = {
  admin: 'home.adminGreeting',
  devops: 'home.devopsGreeting',
  developer: 'home.developerGreeting',
}

export function HomePage() {
  const { t } = useTranslation()
  const { role } = useAuthStore()

  return (
    <div>
      <div
        style={{
          marginBottom: '32px',
        }}
      >
        <h1
          style={{
            fontSize: '36px',
            fontWeight: 800,
            color: 'var(--color-text-primary)',
            margin: '0 0 12px 0',
            lineHeight: 1.2,
          }}
        >
          {t('home.welcome')}
        </h1>
        <p style={{ color: 'var(--color-text-secondary)', fontSize: '14px', margin: 0 }}>
          {t(greetingKeys[role])}
        </p>
      </div>

      <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
        <button
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '8px',
            padding: '12px 24px',
            borderRadius: '10px',
            border: 'none',
            background: 'linear-gradient(135deg, #ffd700, #f59e0b)',
            color: '#1a1d29',
            fontWeight: 700,
            fontSize: '14px',
            cursor: 'pointer',
            transition: 'opacity var(--transition-fast)',
          }}
        >
          <Rocket size={16} />
          {t('home.getStarted')}
        </button>

        {(role === 'admin' || role === 'devops') && (
          <button
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '8px',
              padding: '12px 24px',
              borderRadius: '10px',
              border: '1px solid rgba(99,102,241,0.3)',
              background: 'rgba(99,102,241,0.15)',
              color: '#a5b4fc',
              fontWeight: 600,
              fontSize: '14px',
              cursor: 'pointer',
              transition: 'all var(--transition-fast)',
            }}
          >
            {t('home.viewStacks')}
          </button>
        )}

        <button
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '8px',
            padding: '12px 24px',
            borderRadius: '10px',
            border: '1px solid var(--color-border-default)',
            background: 'transparent',
            color: 'var(--color-text-primary)',
            fontWeight: 600,
            fontSize: '14px',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
          }}
        >
          {t('home.viewPipelines')}
        </button>
      </div>
    </div>
  )
}

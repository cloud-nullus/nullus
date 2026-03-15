import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
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
  const navigate = useNavigate()
  const { role } = useAuthStore()

  const getRoleLandingPath = (currentRole: Role) => {
    if (currentRole === 'admin') {
      return '/admin/organizations'
    }
    if (currentRole === 'devops') {
      return '/stack/install'
    }
    return '/cicd/templates'
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="mb-3 mt-0 text-4xl font-extrabold leading-[1.2] text-[var(--color-text-primary)]">
          {t('home.welcome')}
        </h1>
        <p className="m-0 text-sm text-[var(--color-text-secondary)]">
          {t(greetingKeys[role])}
        </p>
      </div>

      <div className="flex flex-wrap gap-3">
        <button
          type="button"
          onClick={() => navigate(getRoleLandingPath(role))}
          className="inline-flex cursor-pointer items-center gap-2 rounded-[10px] border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] px-6 py-3 text-sm font-bold text-[#1a1d29] transition-all duration-150 ease-in-out"
        >
          <Rocket size={16} />
          {t('home.getStarted')}
        </button>

        {(role === 'admin' || role === 'devops') && (
          <button
            type="button"
            onClick={() => navigate('/stack/install')}
            className="inline-flex cursor-pointer items-center gap-2 rounded-[10px] border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.15)] px-6 py-3 text-sm font-semibold text-[#a5b4fc] transition-all duration-150 ease-in-out"
          >
            {t('home.viewStacks')}
          </button>
        )}

        <button
          type="button"
          onClick={() => navigate('/cicd/templates')}
          className="inline-flex cursor-pointer items-center gap-2 rounded-[10px] border border-[var(--color-border-default)] bg-transparent px-6 py-3 text-sm font-semibold text-[var(--color-text-primary)] transition-all duration-150 ease-in-out"
        >
          {t('home.viewPipelines')}
        </button>
      </div>
    </div>
  )
}

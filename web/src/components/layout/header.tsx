import { type ReactNode } from 'react'
import { GraduationCap, HardHat, LaptopMinimal, Moon, ShieldCheck, Sun } from 'lucide-react'
import { iconProps } from '../ui/icon'
import { useTranslation } from 'react-i18next'
import { useThemeStore } from '../../stores/theme-store'
import { useAuthStore } from '../../stores/auth-store'
import { useTourStore } from '../../stores/tour-store'
import type { Role } from '../../types'
import { LanguageSwitcher } from '../shared/language-switcher'
import { IconButton } from '../ui/icon-button'

const roleIcons: Record<Role, ReactNode> = {
  admin: <ShieldCheck {...iconProps('sm')} />,
  devops: <HardHat {...iconProps('sm')} />,
  developer: <LaptopMinimal {...iconProps('sm')} />,
}

const roleLabels: Record<Role, string> = {
  admin: 'header.roles.admin',
  devops: 'header.roles.devops',
  developer: 'header.roles.developer',
}

export function Header() {
  const { i18n, t } = useTranslation()
  const { theme, toggleTheme } = useThemeStore()
  const { role } = useAuthStore()
  const startTour = useTourStore((state) => state.start)

  const handleLanguageChange = (language: string) => {
    void i18n.changeLanguage(language)
  }

  return (
    <header className="flex h-[var(--header-height)] shrink-0 items-center justify-end gap-3 border-b border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-[var(--page-padding)]">
      {/* Role badge */}
      <div className="flex items-center gap-1.5 rounded-[var(--radius-full)] bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] px-2 py-0.5 text-[11px] font-semibold text-[var(--color-primary)]">
        {roleIcons[role]}
        {t(roleLabels[role])}
      </div>

      <LanguageSwitcher currentLanguage={i18n.language} onLanguageChange={handleLanguageChange} />

      {/* 둘러보기. 역할을 넘기는 이유는 걸음 목록이 역할마다 다르기 때문이다 —
          developer 를 관리자 전용 화면으로 안내하면 ProtectedRoute 가 되돌려
          보내고 그 걸음은 빈 화면을 강조하게 된다. */}
      <IconButton onClick={() => startTour(role)} aria-label={t('tour.start', 'Tutorial')}>
        <GraduationCap {...iconProps('sm')} />
      </IconButton>

      {/* Theme toggle */}
      <IconButton
        onClick={toggleTheme}
        aria-label={theme === 'dark' ? t('header.theme.switchToLight') : t('header.theme.switchToDark')}
      >
        {theme === 'dark' ? <Sun {...iconProps('sm')} /> : <Moon {...iconProps('sm')} />}
      </IconButton>
    </header>
  )
}

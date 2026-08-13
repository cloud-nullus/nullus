import { type ReactNode } from 'react'
import { HardHat, LaptopMinimal, Moon, ShieldCheck, Sun } from 'lucide-react'
import { iconProps } from '../ui/icon'
import { useTranslation } from 'react-i18next'
import { useThemeStore } from '../../stores/theme-store'
import { useAuthStore } from '../../stores/auth-store'
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

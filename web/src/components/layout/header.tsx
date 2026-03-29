import { type ReactNode } from 'react'
import { Sun, Moon, ShieldCheck, HardHat, LaptopMinimal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useThemeStore } from '../../stores/theme-store'
import { useAuthStore } from '../../stores/auth-store'
import type { Role } from '../../types'
import { LanguageSwitcher } from '../shared/language-switcher'

const roleIcons: Record<Role, ReactNode> = {
  admin: <ShieldCheck size={14} />,
  devops: <HardHat size={14} />,
  developer: <LaptopMinimal size={14} />,
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
    <header className="flex h-[var(--header-height)] shrink-0 items-center justify-end gap-4 border-b border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-6">
      {/* Role badge */}
      <div className="flex items-center gap-1.5 rounded-full bg-[rgba(99,102,241,0.15)] px-2.5 py-1 text-xs font-semibold text-[#a5b4fc]">
        {roleIcons[role]}
        {t(roleLabels[role])}
      </div>

      <LanguageSwitcher currentLanguage={i18n.language} onLanguageChange={handleLanguageChange} />

      {/* Theme toggle */}
      <button
        type="button"
        onClick={toggleTheme}
        aria-label={theme === 'dark' ? t('header.theme.switchToLight') : t('header.theme.switchToDark')}
        className="flex cursor-pointer items-center rounded-md border-none bg-none p-1.5 text-[var(--color-text-secondary)] transition-all duration-150 ease-in-out"
      >
        {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
      </button>
    </header>
  )
}

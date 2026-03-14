import { type ReactNode, type ChangeEvent } from 'react'
import { Sun, Moon, ShieldCheck, HardHat, LaptopMinimal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useThemeStore } from '../../stores/theme-store'
import { useAuthStore } from '../../stores/auth-store'
import type { Role } from '../../types'

const roleIcons: Record<Role, ReactNode> = {
  admin: <ShieldCheck size={14} />,
  devops: <HardHat size={14} />,
  developer: <LaptopMinimal size={14} />,
}

const roleLabels: Record<Role, string> = {
  admin: 'Admin',
  devops: 'DevOps',
  developer: 'Developer',
}

export function Header() {
  const { i18n } = useTranslation()
  const { theme, toggleTheme } = useThemeStore()
  const { role } = useAuthStore()

  const handleLanguageChange = (e: ChangeEvent<HTMLSelectElement>) => {
    void i18n.changeLanguage(e.target.value)
  }

  return (
    <header
      style={{
        height: 'var(--header-height)',
        background: 'var(--color-surface-card)',
        borderBottom: '1px solid var(--color-border-default)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        padding: '0 24px',
        gap: '16px',
        flexShrink: 0,
      }}
    >
      {/* Role badge */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          padding: '4px 10px',
          borderRadius: '9999px',
          background: 'rgba(99,102,241,0.15)',
          color: '#a5b4fc',
          fontSize: '12px',
          fontWeight: 600,
        }}
      >
        {roleIcons[role]}
        {roleLabels[role]}
      </div>

      {/* Language dropdown */}
      <select
        value={i18n.language}
        onChange={handleLanguageChange}
        aria-label="Select language"
        style={{
          background: 'var(--color-surface-card)',
          border: '1px solid var(--color-border-default)',
          color: 'var(--color-text-secondary)',
          borderRadius: '6px',
          padding: '4px 8px',
          fontSize: '13px',
          cursor: 'pointer',
        }}
      >
        <option value="en">EN</option>
        <option value="ko">한국어</option>
      </select>

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
        style={{
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          color: 'var(--color-text-secondary)',
          display: 'flex',
          alignItems: 'center',
          padding: '6px',
          borderRadius: '6px',
          transition: 'color var(--transition-fast)',
        }}
      >
        {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
      </button>
    </header>
  )
}

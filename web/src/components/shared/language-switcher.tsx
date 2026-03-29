import { useMemo, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '../../lib/utils'

interface LanguageSwitcherProps {
  currentLanguage: string
  onLanguageChange: (lang: string) => void
  variant?: 'dropdown' | 'toggle'
}

const LANGUAGE_OPTIONS = [
  { code: 'en', label: 'English', shortLabel: 'EN', flag: '🇺🇸' },
  { code: 'ko', label: 'Korean', shortLabel: 'KO', flag: '🇰🇷' },
]

export function LanguageSwitcher({
  currentLanguage,
  onLanguageChange,
  variant = 'toggle',
}: LanguageSwitcherProps) {
  const [open, setOpen] = useState(false)

  const currentOption = useMemo(
    () => LANGUAGE_OPTIONS.find((option) => currentLanguage.startsWith(option.code)) ?? LANGUAGE_OPTIONS[0],
    [currentLanguage],
  )

  if (variant === 'toggle') {
    return (
      <div className="inline-flex items-center overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        {LANGUAGE_OPTIONS.map((option) => {
          const isActive = currentOption.code === option.code
          return (
            <button
              key={option.code}
              type="button"
              onClick={() => onLanguageChange(option.code)}
              className={cn(
                'min-w-[38px] cursor-pointer border-none px-2.5 py-[5px] text-xs font-bold',
                isActive
                  ? 'bg-[rgba(255,215,0,0.16)] text-[var(--color-brand-gold)]'
                  : 'bg-transparent text-[var(--color-text-secondary)]'
              )}
            >
              {option.shortLabel}
            </button>
          )
        })}
      </div>
    )
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="inline-flex h-[34px] cursor-pointer items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-3 text-[13px] font-semibold text-[var(--color-text-primary)]"
      >
        <span>{currentOption.flag}</span>
        <span>{currentOption.shortLabel}</span>
        <ChevronDown size={14} color="var(--color-text-secondary)" />
      </button>

      {open && (
        <div className="absolute top-10 right-0 z-10 min-w-[164px] overflow-hidden rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] shadow-[0_12px_28px_rgba(0,0,0,0.35)]">
          {LANGUAGE_OPTIONS.map((option) => {
            const isActive = currentOption.code === option.code
            return (
              <button
                key={option.code}
                type="button"
                onClick={() => {
                  onLanguageChange(option.code)
                  setOpen(false)
                }}
                className={cn(
                  'flex w-full cursor-pointer items-center gap-2 border-none px-3 py-[9px] text-left text-[13px]',
                  isActive
                    ? 'bg-[rgba(255,215,0,0.14)] text-[var(--color-brand-gold)]'
                    : 'bg-transparent text-[var(--color-text-primary)]'
                )}
              >
                <span>{option.flag}</span>
                <span>{option.label}</span>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}

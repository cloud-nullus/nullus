import { Code2, Shield, Wrench } from 'lucide-react'
import type { Role } from '../../types'
import { cn } from '../../lib/utils'

interface RoleSwitcherProps {
  currentRole: Role
  onRoleChange: (role: Role) => void
  compact?: boolean
}

const ROLE_OPTIONS: { role: Role; label: string; icon: typeof Shield }[] = [
  { role: 'admin', label: 'Admin', icon: Shield },
  { role: 'devops', label: 'DevOps Engineer', icon: Wrench },
  { role: 'developer', label: 'Developer', icon: Code2 },
]

export function RoleSwitcher({ currentRole, onRoleChange, compact = false }: RoleSwitcherProps) {
  return (
    <div className="inline-flex items-center gap-1.5">
      {ROLE_OPTIONS.map((option) => {
        const Icon = option.icon
        const isActive = option.role === currentRole

        return (
          <button
            key={option.role}
            type="button"
            onClick={() => onRoleChange(option.role)}
            aria-label={option.label}
            className={cn(
              'inline-flex cursor-pointer items-center justify-center rounded-lg border text-xs transition-all duration-150 ease-in-out',
              compact ? 'h-8 min-w-8 px-0' : 'h-[34px] gap-1.5 px-3',
              isActive
                ? 'border-[var(--color-brand-gold)] bg-[rgba(255,215,0,0.15)] font-bold text-[var(--color-brand-gold)]'
                : 'border-[var(--color-border-default)] bg-[var(--color-surface-card)] font-semibold text-[var(--color-text-secondary)]'
            )}
          >
            <Icon size={14} />
            {!compact && option.label}
          </button>
        )
      })}
    </div>
  )
}

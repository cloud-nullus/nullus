import { type ReactNode } from 'react'
import { CheckCircle, Clock, AlertCircle, MinusCircle } from 'lucide-react'
import type { ClusterStatus } from '../../types'
import { cn } from '../../lib/utils'

interface StatusBadgeProps {
  status: ClusterStatus
  label?: string
}

const statusConfig: Record<ClusterStatus, {
  bgClass: string
  textClass: string
  icon: ReactNode
  defaultLabel: string
}> = {
  connected: {
    bgClass: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)]',
    textClass: 'text-[var(--color-success)]',
    icon: <CheckCircle size={12} />,
    defaultLabel: 'Connected',
  },
  pending: {
    bgClass: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)]',
    textClass: 'text-[var(--color-warning)]',
    icon: <Clock size={12} />,
    defaultLabel: 'Pending',
  },
  error: {
    bgClass: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)]',
    textClass: 'text-[var(--color-error)]',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Error',
  },
  unreachable: {
    bgClass: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)]',
    textClass: 'text-[var(--color-warning)]',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Unreachable',
  },
  auth_failed: {
    bgClass: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)]',
    textClass: 'text-[var(--color-error)]',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Auth Failed',
  },
  inactive: {
    bgClass: 'bg-[color-mix(in_srgb,_var(--color-text-muted)_15%,_transparent)]',
    textClass: 'text-[var(--color-text-muted)]',
    icon: <MinusCircle size={12} />,
    defaultLabel: 'Inactive',
  },
}

export function StatusBadge({ status, label }: StatusBadgeProps) {
  const config = statusConfig[status]
  const displayLabel = label ?? config.defaultLabel

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold',
        config.bgClass,
        config.textClass
      )}
    >
      {config.icon}
      {displayLabel}
    </span>
  )
}

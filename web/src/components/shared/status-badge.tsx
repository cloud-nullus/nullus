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
    bgClass: 'bg-[rgba(34,197,94,0.15)]',
    textClass: 'text-[#22c55e]',
    icon: <CheckCircle size={12} />,
    defaultLabel: 'Connected',
  },
  pending: {
    bgClass: 'bg-[rgba(245,158,11,0.15)]',
    textClass: 'text-[#f59e0b]',
    icon: <Clock size={12} />,
    defaultLabel: 'Pending',
  },
  error: {
    bgClass: 'bg-[rgba(239,68,68,0.15)]',
    textClass: 'text-[#ef4444]',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Error',
  },
  unreachable: {
    bgClass: 'bg-[rgba(245,158,11,0.15)]',
    textClass: 'text-[#f59e0b]',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Unreachable',
  },
  auth_failed: {
    bgClass: 'bg-[rgba(239,68,68,0.15)]',
    textClass: 'text-[#ef4444]',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Auth Failed',
  },
  inactive: {
    bgClass: 'bg-[rgba(100,116,139,0.15)]',
    textClass: 'text-[#64748b]',
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

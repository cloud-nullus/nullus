import { type ReactNode } from 'react'
import { CheckCircle, Clock, AlertCircle, MinusCircle } from 'lucide-react'
import type { ClusterStatus } from '../../types'

interface StatusBadgeProps {
  status: ClusterStatus
  label?: string
}

const statusConfig: Record<ClusterStatus, {
  bg: string
  text: string
  icon: ReactNode
  defaultLabel: string
}> = {
  connected: {
    bg: 'rgba(34,197,94,0.15)',
    text: '#22c55e',
    icon: <CheckCircle size={12} />,
    defaultLabel: 'Connected',
  },
  pending: {
    bg: 'rgba(245,158,11,0.15)',
    text: '#f59e0b',
    icon: <Clock size={12} />,
    defaultLabel: 'Pending',
  },
  error: {
    bg: 'rgba(239,68,68,0.15)',
    text: '#ef4444',
    icon: <AlertCircle size={12} />,
    defaultLabel: 'Error',
  },
  inactive: {
    bg: 'rgba(100,116,139,0.15)',
    text: '#64748b',
    icon: <MinusCircle size={12} />,
    defaultLabel: 'Inactive',
  },
}

export function StatusBadge({ status, label }: StatusBadgeProps) {
  const config = statusConfig[status]
  const displayLabel = label ?? config.defaultLabel

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '4px',
        padding: '2px 8px',
        borderRadius: '9999px',
        fontSize: '12px',
        fontWeight: 600,
        backgroundColor: config.bg,
        color: config.text,
      }}
    >
      {config.icon}
      {displayLabel}
    </span>
  )
}

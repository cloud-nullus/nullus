// F8-UIUX-StatusBadgeColors — shared stack status color palette.
//
// The pre-existing per-page STATUS_STYLES lived inline in stack-list-page.tsx
// and treated rolled_back as neutral grey, which made it visually
// indistinguishable from cancelled. This shared util keeps the same
// { bg, color, label } shape so the stack-list DataTable cell renderer
// continues to consume the palette via inline styles, while giving every
// terminal state a distinct color:
//   - failed       → red      (danger)
//   - rolled_back  → amber    (error-but-recovered)
//   - cancelled    → grey     (user-initiated stop)
// plus a shared getter that falls back to pending for unknown status keys.

import type { StackStatus } from './retry-policy'

export interface StatusStyle {
  bg: string
  color: string
  label: string
}

const BLUE: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa' }
const GREEN: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(34,197,94,0.15)', color: '#22c55e' }
const HEALTHY_GREEN: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(16,185,129,0.18)', color: '#10b981' }
const RED: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(239,68,68,0.15)', color: '#ef4444' }
const AMBER: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b' }
const GREY: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(100,116,139,0.15)', color: '#64748b' }
const INDIGO: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'rgba(99,102,241,0.15)', color: '#a5b4fc' }

export const STATUS_STYLES: Record<string, StatusStyle> = {
  pending:      { ...AMBER,         label: 'Pending' },
  validating:   { ...INDIGO,        label: 'Validating' },
  installing:   { ...BLUE,          label: 'Installing' },
  configuring:  { ...BLUE,          label: 'Configuring' },
  health_check: { ...BLUE,          label: 'Health Check' },
  running:      { ...BLUE,          label: 'Running' },
  completed:    { ...GREEN,         label: 'Completed' },
  failed:       { ...RED,           label: 'Failed' },
  rolling_back: { ...AMBER,         label: 'Rolling Back' },
  rolled_back:  { ...AMBER,         label: 'Rolled Back' },
  cancelled:    { ...GREY,          label: 'Cancelled' },
  // Non-StackStatus aliases surfaced by monitoring/compatibility layers.
  success:      { ...HEALTHY_GREEN, label: 'Healthy' },
  healthy:      { ...HEALTHY_GREEN, label: 'Healthy' },
}

export function getStatusStyle(status: StackStatus | string): StatusStyle {
  return STATUS_STYLES[status] ?? STATUS_STYLES.pending
}

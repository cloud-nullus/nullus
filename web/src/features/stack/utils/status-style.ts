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

const BLUE: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-info) 15%, transparent)', color: 'var(--color-info)' }
const GREEN: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-success) 15%, transparent)', color: 'var(--color-success)' }
const HEALTHY_GREEN: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-success) 18%, transparent)', color: 'var(--color-success)' }
const RED: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-error) 15%, transparent)', color: 'var(--color-error)' }
const AMBER: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-warning) 15%, transparent)', color: 'var(--color-warning)' }
const GREY: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-text-muted) 15%, transparent)', color: 'var(--color-text-muted)' }
const INDIGO: Pick<StatusStyle, 'bg' | 'color'> = { bg: 'color-mix(in srgb, var(--color-primary) 15%, transparent)', color: 'var(--color-primary)' }

export const STATUS_STYLES: Record<string, StatusStyle> = {
  pending:      { ...AMBER,         label: 'Pending' },
  terminating:  { ...AMBER,         label: 'Terminating' },
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

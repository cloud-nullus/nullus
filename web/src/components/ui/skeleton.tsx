// F8-UIUX-EmptyLoading — shared skeleton primitive. Keep it deliberately
// minimal: a single pulsing div that callers shape with className. Avoids
// pulling in an animation library and stays consistent with the existing
// Tailwind 4 palette.

import { cn } from '../../lib/utils'

export function Skeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn('animate-pulse rounded bg-[rgba(148,163,184,0.12)]', className)}
      data-testid="skeleton"
    />
  )
}

import { cn } from '../../lib/utils'

interface SkeletonProps {
  width?: number | string
  height?: number | string
  borderRadius?: number | string
}

interface SkeletonTextProps {
  lines?: number
}

interface SkeletonTableProps {
  rows?: number
  columns?: number
}

function resolveWidth(width: number | string): string {
  if (width === '72%') return 'w-[72%]'
  if (width === '60%') return 'w-[60%]'
  if (width === '42%') return 'w-[42%]'
  return 'w-full'
}

function resolveHeight(height: number | string): string {
  if (height === '18px') return 'h-[18px]'
  if (height === '12px') return 'h-3'
  if (height === '11px') return 'h-[11px]'
  return 'h-4'
}

function resolveRadius(borderRadius: number | string): string {
  if (borderRadius === '6px') return 'rounded-md'
  return 'rounded-lg'
}

function resolveGridCols(columns: number): string {
  if (columns === 1) return 'grid-cols-1'
  if (columns === 2) return 'grid-cols-2'
  if (columns === 3) return 'grid-cols-3'
  if (columns === 5) return 'grid-cols-5'
  if (columns === 6) return 'grid-cols-6'
  return 'grid-cols-4'
}

export function Skeleton({ width = '100%', height = '16px', borderRadius = '8px' }: SkeletonProps) {
  return (
    <div
      className={cn(
        'animate-pulse bg-[linear-gradient(90deg,var(--color-surface-card)_0%,color-mix(in_srgb,var(--color-surface-card)_72%,white_28%)_50%,var(--color-surface-card)_100%)]',
        resolveWidth(width),
        resolveHeight(height),
        resolveRadius(borderRadius)
      )}
    />
  )
}

export function SkeletonText({ lines = 3 }: SkeletonTextProps) {
  const lineIds = Array.from({ length: lines }, (_, idx) => idx + 1)

  return (
    <div className="flex flex-col gap-2">
      {lineIds.map((lineNo) => (
        <Skeleton
          key={`skeleton-text-${lines}-${lineNo}`}
          width={lineNo === lines ? '72%' : '100%'}
          height="12px"
          borderRadius="6px"
        />
      ))}
    </div>
  )
}

export function SkeletonCard() {
  return (
    <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
      <Skeleton width="42%" height="18px" borderRadius="6px" />
      <div className="mt-[14px]">
        <SkeletonText lines={3} />
      </div>
    </div>
  )
}

export function SkeletonTable({ rows = 5, columns = 4 }: SkeletonTableProps) {
  const columnIds = Array.from({ length: columns }, (_, idx) => idx + 1)
  const rowIds = Array.from({ length: rows }, (_, idx) => idx + 1)
  const gridClass = resolveGridCols(columns)

  return (
    <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
      <div
        className={cn(
          'grid gap-3 border-b border-[var(--color-border-default)] px-[14px] py-3',
          gridClass
        )}
      >
        {columnIds.map((columnNo) => (
          <Skeleton key={`skeleton-header-${columns}-${columnNo}`} height="12px" width="60%" borderRadius="6px" />
        ))}
      </div>

      {rowIds.map((rowNo) => (
        <div
          key={`skeleton-row-${rows}-${rowNo}`}
          className={cn(
            'grid gap-3 px-[14px] py-3',
            gridClass,
            rowNo === rows ? 'border-b-0' : 'border-b border-[var(--color-border-default)]'
          )}
        >
          {columnIds.map((columnNo) => (
            <Skeleton key={`skeleton-cell-${rowNo}-${columnNo}`} height="11px" borderRadius="6px" />
          ))}
        </div>
      ))}
    </div>
  )
}

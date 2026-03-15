import { type ReactNode } from 'react'

interface ListDetailPanelProps {
  listWidth?: number
  listContent: ReactNode
  detailContent: ReactNode | null
  emptyDetailMessage?: string
}

export function ListDetailPanel({
  listWidth = 280,
  listContent,
  detailContent,
  emptyDetailMessage = 'Select an item to view details',
}: ListDetailPanelProps) {
  const listWidthClass = listWidth === 240 ? 'w-[240px]' : listWidth === 280 ? 'w-[280px]' : 'w-[280px]'

  return (
    <div className="flex h-full overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
      <div className={`shrink-0 overflow-y-auto border-r border-[var(--color-border-default)] ${listWidthClass}`}>
        {listContent}
      </div>

      <div className="min-w-0 flex-1 overflow-y-auto">
        {detailContent ?? (
          <div className="flex h-full min-h-[220px] items-center justify-center p-6 text-center text-sm text-[var(--color-text-secondary)]">
            {emptyDetailMessage}
          </div>
        )}
      </div>
    </div>
  )
}

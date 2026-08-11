import { type ReactNode, useMemo, useState } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type PaginationState,
} from '@tanstack/react-table'
import { ChevronUp, ChevronDown, ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '../ui/button'
import { SearchInput } from '../ui/search-input'
import { cn } from '../../lib/utils'

interface DataTableProps<T> {
  columns: ColumnDef<T, unknown>[]
  data: T[]
  getRowKey: (row: T) => string
  onSort?: (field: string, dir: 'asc' | 'desc') => void
  onRowClick?: (row: T) => void
  emptyMessage?: string
  pageSize?: number
  toolbar?: ReactNode
  /**
   * 자기 테두리·모서리를 버린다. ListDetailPanel 안에 들어갈 때 쓴다 —
   * 액자 안에 액자를 겹치지 않는다(DESIGN.md §Layout).
   */
  flush?: boolean
}

export function DataTable<T>({
  columns,
  data,
  getRowKey,
  onSort,
  onRowClick,
  emptyMessage,
  pageSize = 20,
  toolbar,
  flush = false,
}: DataTableProps<T>) {
  const { t } = useTranslation()
  const resolvedEmptyMessage = emptyMessage ?? t('dataTable.empty', 'No data available.')
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [pagination, setPagination] = useState<PaginationState>({ pageIndex: 0, pageSize })

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      globalFilter,
      pagination,
    },
    onSortingChange: (updater) => {
      setSorting((prev) => {
        const next = typeof updater === 'function' ? updater(prev) : updater
        const firstSort = next[0]
        if (firstSort) {
          onSort?.(firstSort.id, firstSort.desc ? 'desc' : 'asc')
        }
        return next
      })
    },
    onGlobalFilterChange: setGlobalFilter,
    onPaginationChange: setPagination,
    getRowId: (row) => getRowKey(row),
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  const pageCount = table.getPageCount()
  const canPrevious = table.getCanPreviousPage()
  const canNext = table.getCanNextPage()
  const pageIndex = table.getState().pagination.pageIndex
  const pageNumbers = useMemo(() => Array.from({ length: pageCount }, (_, index) => index), [pageCount])

  return (
    <div
      className={cn(
        'bg-[var(--color-surface-card)]',
        // flush 는 테두리만 버린다. h-full 을 주면 안 된다 — 스크롤 영역을 꽉 채워
        // 표 뒤에 오는 형제(목록 힌트 등)를 화면 밖으로 밀어낸다.
        flush
          ? ''
          : 'overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)]',
      )}
    >
      <div className="flex flex-wrap items-center gap-2 border-b border-[var(--color-border-default)] px-[var(--table-cell-px)] py-2">
        {toolbar ?? (
          <SearchInput
            wrapperClassName="w-full max-w-[280px]"
            value={globalFilter}
            onChange={(event) => setGlobalFilter(event.target.value)}
            placeholder="Search..."
          />
        )}
      </div>

      <table className="w-full border-collapse">
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id} className="bg-[var(--color-surface-sunken)]">
              {headerGroup.headers.map((header) => {
                const canSort = header.column.getCanSort()
                const sortedState = header.column.getIsSorted()
                return (
                  <th
                    key={header.id}
                    className={cn(
                      'h-[var(--table-header-height)] select-none whitespace-nowrap px-[var(--table-cell-px)] text-left text-[11px] font-semibold tracking-[0.06em] text-[var(--color-text-secondary)] uppercase',
                      canSort ? 'cursor-pointer' : 'cursor-default'
                    )}
                    onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                  >
                    <span className="inline-flex items-center gap-1">
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                      {sortedState === 'asc' && <ChevronUp size={12} />}
                      {sortedState === 'desc' && <ChevronDown size={12} />}
                    </span>
                  </th>
                )
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr
              key={row.id}
                className={cn(
                  'transition-all duration-150 ease-in-out hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]',
                  onRowClick ? 'cursor-pointer' : 'cursor-default'
                )}
                onClick={() => onRowClick?.(row.original)}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    key={cell.id}
                    className="h-[var(--table-row-height)] border-t border-[var(--color-border-default)] px-[var(--table-cell-px)] py-1 text-[13px] text-[var(--color-text-primary)]"
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
          ))}
        </tbody>
      </table>

      {table.getRowModel().rows.length === 0 && (
        <div className="py-10 text-center text-[13px] text-[var(--color-text-secondary)]">
          {resolvedEmptyMessage}
        </div>
      )}

      {pageCount > 1 && (
        <div className="flex items-center justify-end gap-1 border-t border-[var(--color-border-default)] px-[var(--table-cell-px)] py-1.5">
          <Button
            variant="ghost"
            size="sm"
            disabled={!canPrevious}
            onClick={() => table.previousPage()}
            className="px-2 py-1.5"
          >
            <ChevronLeft size={14} />
          </Button>
          {pageNumbers.map((number) => (
            <button
              key={number}
              type="button"
              onClick={() => table.setPageIndex(number)}
              className={cn(
                'h-7 w-7 cursor-pointer rounded-[var(--radius-sm)] border text-[12px] transition-colors duration-150 ease-in-out',
                number === pageIndex
                  ? 'border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] font-semibold text-[var(--color-primary)]'
                  : 'border-transparent bg-transparent font-normal text-[var(--color-text-secondary)]'
              )}
            >
              {number + 1}
            </button>
          ))}
          <Button
            variant="ghost"
            size="sm"
            disabled={!canNext}
            onClick={() => table.nextPage()}
            className="px-2 py-1.5"
          >
            <ChevronRight size={14} />
          </Button>
        </div>
      )}
    </div>
  )
}

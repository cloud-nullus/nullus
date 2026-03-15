import { useMemo, useState } from 'react'
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
import { Button } from '../ui/button'
import { cn } from '../../lib/utils'

interface DataTableProps<T> {
  columns: ColumnDef<T, unknown>[]
  data: T[]
  getRowKey: (row: T) => string
  onSort?: (field: string, dir: 'asc' | 'desc') => void
  onRowClick?: (row: T) => void
  emptyMessage?: string
  pageSize?: number
}

export function DataTable<T>({
  columns,
  data,
  getRowKey,
  onSort,
  onRowClick,
  emptyMessage = '데이터가 없습니다.',
  pageSize = 20,
}: DataTableProps<T>) {
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
    <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
      <div className="border-b border-[var(--color-border-default)] px-[14px] py-3">
        <input
          value={globalFilter}
          onChange={(event) => setGlobalFilter(event.target.value)}
          placeholder="Search..."
          className="w-full max-w-[280px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
        />
      </div>

      <table className="w-full border-collapse">
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id} className="bg-[rgba(255,255,255,0.02)]">
              {headerGroup.headers.map((header) => {
                const canSort = header.column.getCanSort()
                const sortedState = header.column.getIsSorted()
                return (
                  <th
                    key={header.id}
                    className={cn(
                      'select-none whitespace-nowrap px-[14px] py-2.5 text-left text-[11px] font-semibold tracking-[0.06em] text-[var(--color-text-secondary)] uppercase',
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
                'transition-all duration-150 ease-in-out hover:bg-[rgba(255,255,255,0.02)]',
                onRowClick ? 'cursor-pointer' : 'cursor-default'
              )}
              onClick={() => onRowClick?.(row.original)}
            >
              {row.getVisibleCells().map((cell) => (
                <td
                  key={cell.id}
                  className="border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm text-[var(--color-text-primary)]"
                >
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      {table.getRowModel().rows.length === 0 && (
        <div className="py-12 text-center text-sm text-[var(--color-text-secondary)]">
          {emptyMessage}
        </div>
      )}

      {pageCount > 1 && (
        <div className="flex items-center justify-end gap-1.5 border-t border-[var(--color-border-default)] px-4 py-3">
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
                'h-8 w-8 cursor-pointer rounded-md border text-[13px] transition-all duration-150 ease-in-out',
                number === pageIndex
                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.15)] font-semibold text-[#a5b4fc]'
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

import { type ReactNode } from 'react'
import { ChevronUp, ChevronDown, ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '../ui/button'

export interface Column<T> {
  key: keyof T | string
  label: string
  sortable?: boolean
  render?: (row: T) => ReactNode
  width?: string
}

interface DataTableProps<T> {
  columns: Column<T>[]
  data: T[]
  sortField?: string
  sortDir?: 'asc' | 'desc'
  onSort?: (field: string) => void
  onRowClick?: (row: T) => void
  emptyMessage?: string
  page?: number
  pageSize?: number
  total?: number
  onPageChange?: (page: number) => void
  getRowKey: (row: T) => string
}

export function DataTable<T>({
  columns,
  data,
  sortField,
  sortDir,
  onSort,
  onRowClick,
  emptyMessage = '데이터가 없습니다.',
  page = 1,
  pageSize = 20,
  total,
  onPageChange,
  getRowKey,
}: DataTableProps<T>) {
  const totalPages = total !== undefined ? Math.ceil(total / pageSize) : undefined

  const thStyle: React.CSSProperties = {
    padding: '10px 14px',
    textAlign: 'left',
    fontSize: '11px',
    fontWeight: 600,
    color: 'var(--color-text-secondary)',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    whiteSpace: 'nowrap',
    userSelect: 'none',
  }

  const tdStyle: React.CSSProperties = {
    padding: '12px 14px',
    fontSize: '14px',
    color: 'var(--color-text-primary)',
    borderTop: '1px solid var(--color-border-default)',
  }

  return (
    <div
      style={{
        background: 'var(--color-surface-card)',
        border: '1px solid var(--color-border-default)',
        borderRadius: 'var(--card-radius)',
        overflow: 'hidden',
      }}
    >
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ background: 'rgba(255,255,255,0.02)' }}>
            {columns.map((col) => (
              <th
                key={String(col.key)}
                style={{
                  ...thStyle,
                  cursor: col.sortable ? 'pointer' : 'default',
                  width: col.width,
                }}
                onClick={() => col.sortable && onSort?.(String(col.key))}
              >
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  {col.label}
                  {col.sortable && sortField === String(col.key) && (
                    sortDir === 'asc' ? <ChevronUp size={12} /> : <ChevronDown size={12} />
                  )}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row) => (
            <tr
              key={getRowKey(row)}
              style={{ transition: 'background var(--transition-fast)', cursor: onRowClick ? 'pointer' : 'default' }}
              onClick={() => onRowClick?.(row)}
              onMouseEnter={(e) => {
                ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)'
              }}
              onMouseLeave={(e) => {
                ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent'
              }}
            >
              {columns.map((col) => (
                <td key={String(col.key)} style={tdStyle}>
                  {col.render ? col.render(row) : String((row as Record<string, unknown>)[String(col.key)] ?? '')}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      {data.length === 0 && (
        <div
          style={{
            textAlign: 'center',
            padding: '48px 0',
            color: 'var(--color-text-secondary)',
            fontSize: '14px',
          }}
        >
          {emptyMessage}
        </div>
      )}

      {totalPages !== undefined && totalPages > 1 && onPageChange && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            gap: '6px',
            padding: '12px 16px',
            borderTop: '1px solid var(--color-border-default)',
          }}
        >
          <Button
            variant="ghost"
            size="sm"
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            style={{ padding: '6px 8px' }}
          >
            <ChevronLeft size={14} />
          </Button>
          {Array.from({ length: totalPages }, (_, i) => i + 1).map((p) => (
            <button
              key={p}
              onClick={() => onPageChange(p)}
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '6px',
                border: p === page ? '1px solid rgba(99,102,241,0.5)' : '1px solid transparent',
                background: p === page ? 'rgba(99,102,241,0.15)' : 'transparent',
                color: p === page ? '#a5b4fc' : 'var(--color-text-secondary)',
                fontSize: '13px',
                fontWeight: p === page ? 600 : 400,
                cursor: 'pointer',
                transition: 'all var(--transition-fast)',
              }}
            >
              {p}
            </button>
          ))}
          <Button
            variant="ghost"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
            style={{ padding: '6px 8px' }}
          >
            <ChevronRight size={14} />
          </Button>
        </div>
      )}
    </div>
  )
}

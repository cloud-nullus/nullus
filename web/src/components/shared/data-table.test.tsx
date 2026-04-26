import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import type { ColumnDef } from '@tanstack/react-table'
import { DataTable } from './data-table'

interface RowData {
  id: string
  name: string
  status: string
}

const columns: ColumnDef<RowData, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    cell: (info) => info.getValue(),
  },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: (info) => info.getValue(),
  },
]

describe('DataTable', () => {
  it('renders without crashing', () => {
    const { container } = render(
      <DataTable<RowData>
        columns={columns}
        data={[]}
        getRowKey={(row) => row.id}
      />
    )

    expect(container).toBeTruthy()
    expect(screen.getByPlaceholderText('Search...')).not.toBeNull()
  })

  it('renders provided columns and rows', () => {
    const rows: RowData[] = [
      { id: '1', name: 'Alpha', status: 'running' },
      { id: '2', name: 'Beta', status: 'failed' },
    ]

    render(
      <DataTable<RowData>
        columns={columns}
        data={rows}
        getRowKey={(row) => row.id}
      />
    )

    expect(screen.getByText('Name')).not.toBeNull()
    expect(screen.getByText('Status')).not.toBeNull()
    expect(screen.getAllByText('Alpha').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Beta').length).toBeGreaterThan(0)
    expect(screen.getAllByText('running').length).toBeGreaterThan(0)
    expect(screen.getAllByText('failed').length).toBeGreaterThan(0)
  })

  it('calls onSort when sortable header is clicked', () => {
    const onSort = vi.fn()

    render(
      <DataTable<RowData>
        columns={columns}
        data={[{ id: '1', name: 'Alpha', status: 'running' }]}
        getRowKey={(row) => row.id}
        onSort={onSort}
      />
    )

    fireEvent.click(screen.getByText('Name'))

    expect(onSort).toHaveBeenCalledWith('name', 'asc')
  })

  it('shows empty message when no data is available', () => {
    render(
      <DataTable<RowData>
        columns={columns}
        data={[]}
        getRowKey={(row) => row.id}
        emptyMessage="No rows"
      />
    )

    expect(screen.getByText('No rows')).not.toBeNull()
  })
})

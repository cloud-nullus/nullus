import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { AlertHistoryPage } from './alert-history-page'

const mockUseAlertHistory = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())

vi.mock('../api/observability-api', async () => {
  const actual = await vi.importActual('../api/observability-api')
  return {
    ...actual,
    useAlertHistory: mockUseAlertHistory,
  }
})

vi.mock('../components/cluster-stack-filter', () => ({
  useClusterStackFilterState: mockUseClusterStackFilterState,
  ClusterStackFilter: () => <div>ClusterStackFilter</div>,
}))

describe('AlertHistoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [],
      filteredStacks: [],
      selectedCluster: undefined,
      selectedStack: undefined,
    })
    mockUseAlertHistory.mockReturnValue({ data: { items: [], total: 0 } })
  })

  it('renders page title without crashing', () => {
    renderWithProviders(<AlertHistoryPage />)
    expect(screen.getByRole('heading', { level: 1, name: 'Alert History' })).not.toBeNull()
  })

  it('shows stable loading-state UI while history is not loaded yet', () => {
    mockUseAlertHistory.mockReturnValue({ data: undefined })

    renderWithProviders(<AlertHistoryPage />)

    expect(screen.getByRole('heading', { level: 1, name: 'Alert History' })).not.toBeNull()
    expect(screen.queryByText('알림 이력이 없습니다.')).not.toBeNull()
  })

  it('renders history rows when hook returns data', () => {
    mockUseAlertHistory.mockReturnValue({
      data: {
        items: [
          {
            id: 'hist-1',
            ruleName: 'High CPU',
            severity: 'critical',
            message: 'CPU usage exceeded threshold',
            firedAt: '2099-01-01T00:00:00Z',
            resolvedAt: null,
          },
        ],
        total: 1,
      },
    })

    renderWithProviders(<AlertHistoryPage />)

    expect(screen.queryByText('High CPU')).not.toBeNull()
    expect(screen.queryByText('CPU usage exceeded threshold')).not.toBeNull()
    expect(screen.queryByText('미해결')).not.toBeNull()
  })

  it('shows empty state when there is no history', () => {
    mockUseAlertHistory.mockReturnValue({ data: { items: [], total: 0 } })

    renderWithProviders(<AlertHistoryPage />)

    expect(screen.queryByText('알림 이력이 없습니다.')).not.toBeNull()
  })

  it('passes selected severity filter to API hook', () => {
    renderWithProviders(<AlertHistoryPage />)

    fireEvent.change(screen.getByDisplayValue('All Severity'), { target: { value: 'warning' } })

    expect(mockUseAlertHistory).toHaveBeenLastCalledWith({ severity: 'warning' })
  })

  it('shows alert detail when row is expanded', () => {
    mockUseAlertHistory.mockReturnValue({
      data: {
        items: [
          {
            id: 'hist-1',
            ruleName: 'Disk Full',
            severity: 'warning',
            message: 'Disk usage exceeded threshold',
            firedAt: '2099-01-01T00:00:00Z',
            resolvedAt: null,
          },
        ],
        total: 1,
      },
    })

    renderWithProviders(<AlertHistoryPage />)

    const rows = screen.getAllByRole('row')
    const firstDataRow = rows[1]
    const expandButton = firstDataRow?.querySelector('button')
    expect(expandButton).not.toBeNull()
    fireEvent.click(expandButton!)
    expect(screen.queryByText('Alert Detail')).not.toBeNull()
    expect(screen.queryAllByText('Disk usage exceeded threshold').length).toBeGreaterThan(0)
  })
})

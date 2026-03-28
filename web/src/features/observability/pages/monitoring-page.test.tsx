import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { MonitoringPage } from './monitoring-page'

const mockUseDashboard = vi.hoisted(() => vi.fn())
const mockUseAuthStore = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())

vi.mock('../api/observability-api', async () => {
  const actual = await vi.importActual('../api/observability-api')
  return {
    ...actual,
    useDashboard: mockUseDashboard,
  }
})

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: mockUseAuthStore,
}))

vi.mock('../components/cluster-stack-filter', () => ({
  useClusterStackFilterState: mockUseClusterStackFilterState,
  ClusterStackFilter: ({
    onClusterChange,
    onStackChange,
    onClear,
  }: {
    onClusterChange: (id: string) => void
    onStackChange: (id: string) => void
    onClear: () => void
  }) => (
    <div>
      <button type="button" onClick={() => onClusterChange('cluster-1')}>Mock Select Cluster</button>
      <button type="button" onClick={() => onStackChange('stack-1')}>Mock Select Stack</button>
      <button type="button" onClick={() => onClear()}>Mock Clear</button>
    </div>
  ),
}))

vi.mock('recharts', () => {
  const Mock = ({ children }: { children?: ReactNode }) => <div>{children}</div>
  return {
    AreaChart: Mock,
    Area: Mock,
    BarChart: Mock,
    Bar: Mock,
    PieChart: Mock,
    Pie: Mock,
    Cell: Mock,
    XAxis: Mock,
    YAxis: Mock,
    CartesianGrid: Mock,
    Tooltip: Mock,
    ResponsiveContainer: Mock,
    Legend: Mock,
  }
})

describe('MonitoringPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()

    mockUseAuthStore.mockImplementation((selector: (state: { role: string }) => unknown) =>
      selector({ role: 'admin' })
    )

    mockUseDashboard.mockReturnValue({
      data: {
        kpi: { cpuUsage: 77, memoryUsage: 65, storageUsage: 31, podCount: 10, podRunning: 9 },
        pipeline: { successRate: 98, totalRuns: 10, avgBuildSeconds: 120 },
        tools: [{ name: 'Grafana', version: '10.4', status: 'running' }],
      },
      isLoading: false,
      refetch: vi.fn(),
    })

    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [{ id: 'cluster-1', name: 'Prod Cluster', status: 'connected' }],
      filteredStacks: [{ id: 'stack-1', name: 'Main Stack', status: 'running' }],
      selectedCluster: { id: 'cluster-1', name: 'Prod Cluster', status: 'connected' },
      selectedStack: { id: 'stack-1', name: 'Main Stack', status: 'running' },
      hasContext: true,
    })
  })

  it('renders page title without crashing', () => {
    renderWithProviders(<MonitoringPage />)
    expect(screen.getByRole('heading', { level: 1, name: 'Monitoring Dashboard' })).not.toBeNull()
  })

  it('shows loading state in stack view while dashboard data is loading', () => {
    mockUseDashboard.mockReturnValue({ data: undefined, isLoading: true, refetch: vi.fn() })

    renderWithProviders(<MonitoringPage />)
    fireEvent.click(screen.getByText('Mock Select Stack'))

    const refreshButton = screen.getByRole('button', { name: /Refresh/i })
    const icon = refreshButton.querySelector('svg')
    expect(icon).not.toBeNull()
    expect(icon?.classList.contains('animate-spin')).toBe(true)
  })

  it('renders dashboard data in stack view when hook returns data', () => {
    mockUseDashboard.mockReturnValue({
      data: {
        kpi: { cpuUsage: 88, memoryUsage: 44, storageUsage: 22, podCount: 12, podRunning: 11 },
        pipeline: { successRate: 99, totalRuns: 100, avgBuildSeconds: 90 },
        tools: [{ name: 'Custom Tool', version: '1.0.0', status: 'running' }],
      },
      isLoading: false,
      refetch: vi.fn(),
    })

    renderWithProviders(<MonitoringPage />)
    fireEvent.click(screen.getByText('Mock Select Stack'))

    expect(screen.queryByText('88%')).not.toBeNull()
    expect(screen.queryByText('Custom Tool')).not.toBeNull()
  })

  it('shows empty state when no cluster or stack is selected', () => {
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [],
      filteredStacks: [],
      selectedCluster: undefined,
      selectedStack: undefined,
      hasContext: false,
    })

    renderWithProviders(<MonitoringPage />)

    expect(screen.queryByText('Select a Cluster or Stack above to begin')).not.toBeNull()
  })

  it('shows embed-blocked message for non-embeddable host', () => {
    renderWithProviders(<MonitoringPage />)

    fireEvent.click(screen.getByText('CI/CD'))
    fireEvent.click(screen.getByText('Grafana'))

    expect(screen.queryByText('Embedding blocked by target site')).not.toBeNull()
  })
})

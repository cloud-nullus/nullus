import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { MonitoringPage } from './monitoring-page'

const mockUseDashboard = vi.hoisted(() => vi.fn())
const mockUseAuthStore = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())
const mockStackMonitoringOverview = vi.hoisted(() => vi.fn())

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

vi.mock('../components/stack-monitoring-overview', () => ({
  StackMonitoringOverview: ({ stackId }: { stackId: string }) => {
    mockStackMonitoringOverview(stackId)
    return <div>Mock Stack Monitoring Overview: {stackId}</div>
  },
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

    // CI/CD 탭은 target 타입 클러스터에서만 열리므로 types 를 준다.
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [{ id: 'cluster-1', name: 'Prod Cluster', status: 'connected', types: ['target'] }],
      stacks: [{ id: 'stack-1', name: 'Main Stack', clusterId: 'cluster-1', status: 'running' }],
      filteredStacks: [{ id: 'stack-1', name: 'Main Stack', status: 'running' }],
      selectedCluster: { id: 'cluster-1', name: 'Prod Cluster', status: 'connected', types: ['target'] },
      selectedStack: { id: 'stack-1', name: 'Main Stack', status: 'running' },
      hasContext: true,
    })
  })

  it('renders page title without crashing', () => {
    renderWithProviders(<MonitoringPage />)
    expect(screen.getByRole('heading', { level: 1, name: 'Monitoring Dashboard' })).not.toBeNull()
  })

  it('renders cluster dashboard widgets after selecting a cluster', () => {
    renderWithProviders(<MonitoringPage />)

    fireEvent.click(screen.getByText('Mock Select Cluster'))
    fireEvent.click(screen.getByRole('button', { name: 'Cluster' }))

    expect(screen.queryByText('Pipeline Success (this week)')).not.toBeNull()
    // KPI 카드는 하드코딩된 Nodes 목업에서 실제 파드 지표로 바뀌었다.
    expect(screen.queryByText('Running Pods')).not.toBeNull()
  })

  it('shows loading state in stack view while dashboard data is loading', () => {
    mockUseDashboard.mockReturnValue({ data: undefined, isLoading: true, refetch: vi.fn() })

    renderWithProviders(<MonitoringPage />)
    fireEvent.click(screen.getByText('Mock Select Stack'))

    expect(screen.queryByText('Mock Stack Monitoring Overview: stack-1')).not.toBeNull()
    expect(mockStackMonitoringOverview).toHaveBeenCalledWith('stack-1')
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

    expect(screen.queryByText('Mock Stack Monitoring Overview: stack-1')).not.toBeNull()
    expect(mockStackMonitoringOverview).toHaveBeenCalledWith('stack-1')
  })

  // 이 카드는 클러스터/스택을 고르기 전에도 보여야 한다 — 플랫폼 전체 현황이라
  // 선택 컨텍스트와 무관하다.
  // 도구 동작 여부는 스택 상세의 파이프라인 토폴로지로 옮겼다. 배치와 동작을
  // 같은 그림에서 읽는 편이 낫고, 이 카드는 클러스터/스택을 고르기도 전에 떠
  // 있어 "선택해서 시작하라" 는 이 화면의 흐름과 어긋났다.
  it('no longer shows the platform tool health card', () => {
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [],
      stacks: [],
      filteredStacks: [],
      selectedCluster: undefined,
      selectedStack: undefined,
      hasContext: false,
    })

    renderWithProviders(<MonitoringPage />)

    expect(screen.queryByText('Platform Tool Health')).toBeNull()
  })

  it('shows empty state when no cluster or stack is selected', () => {
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [],
      stacks: [],
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

    // CI/CD 탭은 클러스터를 고른 뒤에야 열린다.
    fireEvent.click(screen.getByText('Mock Select Cluster'))
    fireEvent.click(screen.getByRole('button', { name: 'CI/CD' }))
    // Grafana 는 대시보드 도구 목록에도 있으므로 임베드 탭 버튼으로 좁힌다.
    fireEvent.click(screen.getByRole('button', { name: 'Grafana' }))

    expect(screen.queryByText('Embedding blocked by target site')).not.toBeNull()
  })
})

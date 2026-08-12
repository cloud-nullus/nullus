import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { MonitoringPage } from './monitoring-page'

const mockUseDashboard = vi.hoisted(() => vi.fn())
const mockUseAuthStore = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())
const mockStackMonitoringOverview = vi.hoisted(() => vi.fn())
const mockUsePipelines = vi.hoisted(() => vi.fn())
const mockUseDeployments = vi.hoisted(() => vi.fn())

vi.mock('../../cicd/api/cicd-api', () => ({
  usePipelines: () => mockUsePipelines(),
  useDeployments: () => mockUseDeployments(),
}))

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

    // CI/CD 탭은 그 클러스터에 파이프라인이 있어야 열린다.
    mockUsePipelines.mockReturnValue({
      data: { items: [{ id: 'pl-1', name: 'demo-app', appType: 'web-backend', clusterId: 'cluster-1', clusterName: 'Prod Cluster', namespace: 'apps' }] },
    })
    mockUseDeployments.mockReturnValue({ data: { items: [] } })

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

  // 첫 화면은 데이터가 있는 클러스터에서 시작해야 한다.
  //
  // clusters[0] 을 그냥 골랐더니 스택도 파이프라인도 없는 클러스터가 잡혔고,
  // 화면의 모든 지표가 0 으로 떴다. 필터는 정상 동작했지만 사용자에게는
  // "지표가 안 나온다" 로 보였다.
  it('auto-selects the first cluster that actually has a stack', () => {
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [
        { id: 'empty-cluster', name: 'Empty', status: 'connected', types: ['target'] },
        { id: 'cluster-1', name: 'Prod Cluster', status: 'connected', types: ['pipeline'] },
      ],
      stacks: [{ id: 'stack-1', name: 'Main Stack', clusterId: 'cluster-1', status: 'running' }],
      filteredStacks: [{ id: 'stack-1', name: 'Main Stack', status: 'running' }],
      selectedCluster: { id: 'cluster-1', name: 'Prod Cluster', status: 'connected', types: ['pipeline'] },
      selectedStack: { id: 'stack-1', name: 'Main Stack', status: 'running' },
      hasContext: true,
    })

    renderWithProviders(<MonitoringPage />)

    expect(mockStackMonitoringOverview).toHaveBeenCalledWith('stack-1')
  })

  // CI/CD 탭은 클러스터가 선언한 타입이 아니라 실제 파이프라인 유무로 열린다.
  //
  // 타입으로 걸었을 때 실제로 이런 상태였다: 파이프라인 2개가 모두 사는
  // kind-nullus-platform 은 types=['pipeline'] 이라 탭이 잠겼고, 탭이 열리는
  // kind-nullus-develop(types=['target'])에는 파이프라인이 하나도 없었다.
  // 정상 구성인데 지표가 어디서도 안 보였다.
  it('enables the CI/CD tab for a pipeline-type cluster that has pipelines', () => {
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [{ id: 'cluster-1', name: 'Platform Cluster', status: 'connected', types: ['pipeline'] }],
      stacks: [{ id: 'stack-1', name: 'Main Stack', clusterId: 'cluster-1', status: 'running' }],
      filteredStacks: [{ id: 'stack-1', name: 'Main Stack', status: 'running' }],
      selectedCluster: { id: 'cluster-1', name: 'Platform Cluster', status: 'connected', types: ['pipeline'] },
      selectedStack: { id: 'stack-1', name: 'Main Stack', status: 'running' },
      hasContext: true,
    })

    renderWithProviders(<MonitoringPage />)
    fireEvent.click(screen.getByText('Mock Select Cluster'))

    expect(screen.getByRole('button', { name: 'CI/CD' })).not.toHaveProperty('disabled', true)
  })

  it('locks the CI/CD tab when the cluster has no pipelines', () => {
    mockUsePipelines.mockReturnValue({ data: { items: [{ id: 'pl-1', name: 'demo-app', clusterId: 'other-cluster' }] } })

    renderWithProviders(<MonitoringPage />)
    fireEvent.click(screen.getByText('Mock Select Cluster'))

    expect(screen.getByRole('button', { name: 'CI/CD' })).toHaveProperty('disabled', true)
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

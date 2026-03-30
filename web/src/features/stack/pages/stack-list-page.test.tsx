import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { fireEvent, screen } from '@testing-library/react'
import { StackListPage } from './stack-list-page'

const mockNavigate = vi.fn()
const mockUseStacks = vi.fn()
const mockUseDeleteStack = vi.fn()
const mockUseStackHistory = vi.fn()
const mockUseStackMonitoring = vi.fn()
const mockUseClusters = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }

  it('hides monitoring tab when stack is completed but cluster is disconnected', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [{
          ...stackRows[0],
          status: 'completed',
        }],
        total: 1,
      },
      isLoading: false,
    })
    mockUseClusters.mockReturnValue({
      data: [{ id: 'cluster-1', name: 'prod-cluster', connection_status: 'unreachable' }],
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.queryByRole('button', { name: 'Monitoring' })).toBeNull()
  })

  it('supports cluster filter and cluster column visibility', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          ...stackRows,
          {
            ...stackRows[0],
            id: 'stack-2',
            name: 'Another Stack',
            clusterId: 'cluster-2',
            clusterName: 'dev-cluster',
          },
        ],
        total: 2,
      },
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.getAllByText('Cluster').length).toBeGreaterThan(0)
    fireEvent.change(screen.getByDisplayValue('All Clusters'), { target: { value: 'prod-cluster' } })

    expect(screen.getAllByText('DevSecOps Core').length).toBeGreaterThan(0)
    expect(screen.queryByText('Another Stack')).toBeNull()
  })

})

vi.mock('../api/stack-api', () => ({
  useStacks: (...args: unknown[]) => mockUseStacks(...args),
  useDeleteStack: (...args: unknown[]) => mockUseDeleteStack(...args),
  useStackHistory: (...args: unknown[]) => mockUseStackHistory(...args),
  useStackMonitoring: (...args: unknown[]) => mockUseStackMonitoring(...args),
  useClusters: (...args: unknown[]) => mockUseClusters(...args),
}))

vi.mock('react-chartjs-2', () => ({
  Bar: () => <div data-testid="chart-bar" />,
  Doughnut: () => <div data-testid="chart-doughnut" />,
  Line: () => <div data-testid="chart-line" />,
}))

vi.mock('chart.js', () => {
  class DummyChart {
    static register = vi.fn()
  }

  return {
    Chart: DummyChart,
    CategoryScale: {},
    LinearScale: {},
    PointElement: {},
    LineElement: {},
    BarElement: {},
    ArcElement: {},
    Tooltip: {},
    Legend: {},
    Filler: {},
  }

  it('hides monitoring tab when stack is completed but cluster is disconnected', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [{
          ...stackRows[0],
          status: 'completed',
        }],
        total: 1,
      },
      isLoading: false,
    })
    mockUseClusters.mockReturnValue({
      data: [{ id: 'cluster-1', name: 'prod-cluster', connection_status: 'unreachable' }],
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.queryByRole('button', { name: 'Monitoring' })).toBeNull()
  })

  it('supports cluster filter and cluster column visibility', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          ...stackRows,
          {
            ...stackRows[0],
            id: 'stack-2',
            name: 'Another Stack',
            clusterId: 'cluster-2',
            clusterName: 'dev-cluster',
          },
        ],
        total: 2,
      },
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.getAllByText('Cluster').length).toBeGreaterThan(0)
    fireEvent.change(screen.getByDisplayValue('All Clusters'), { target: { value: 'prod-cluster' } })

    expect(screen.getAllByText('DevSecOps Core').length).toBeGreaterThan(0)
    expect(screen.queryByText('Another Stack')).toBeNull()
  })

})

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: () => ({ role: 'devops', isAuthenticated: true }),
}))

const stackRows = [
  {
    id: 'stack-1',
    name: 'DevSecOps Core',
    templateId: 'tpl-1',
    templateName: 'GitLab + Argo CD',
    clusterId: 'cluster-1',
    clusterName: 'prod-cluster',
    status: 'success',
    createdAt: '2026-02-01T00:00:00Z',
    updatedAt: '2026-02-01T00:00:00Z',
  },
]

describe('StackListPage', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseStacks.mockReset()
    mockUseDeleteStack.mockReset()
    mockUseStackHistory.mockReset()
    mockUseStackMonitoring.mockReset()
    mockUseClusters.mockReset()

    mockUseStacks.mockReturnValue({
      data: { items: stackRows, total: stackRows.length },
      isLoading: false,
    })
    mockUseDeleteStack.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseStackHistory.mockReturnValue({ data: [], isLoading: false })
    mockUseStackMonitoring.mockReturnValue({ data: null, isLoading: false })
    mockUseClusters.mockReturnValue({ data: [{ id: 'cluster-1', name: 'prod-cluster', connection_status: 'connected' }, { id: 'cluster-2', name: 'dev-cluster', connection_status: 'connected' }], isLoading: false })
  })

  it('renders without crash', () => {
    renderWithProviders(<StackListPage />)

    expect(screen.getAllByText('Stack List').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'New Stack' })).not.toBeNull()
  })

  it('shows loading state while stacks are loading', () => {
    mockUseStacks.mockReturnValue({
      data: undefined,
      isLoading: true,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.getByText(/Loading stacks\.\.\.|스택을 불러오는 중\.\.\./)).not.toBeNull()
  })

  it('renders stack data rows', () => {
    renderWithProviders(<StackListPage />)

    expect(screen.getAllByText('DevSecOps Core').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Running').length).toBeGreaterThan(0)
  })

  it('renders empty state when no stacks exist', () => {
    mockUseStacks.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.getByText(/No stacks found\.|스택이 없습니다\./)).not.toBeNull()
  })

  it('hides monitoring tab when stack is completed but cluster is disconnected', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [{
          ...stackRows[0],
          status: 'completed',
        }],
        total: 1,
      },
      isLoading: false,
    })
    mockUseClusters.mockReturnValue({
      data: [{ id: 'cluster-1', name: 'prod-cluster', connection_status: 'unreachable' }],
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.queryByRole('button', { name: 'Monitoring' })).toBeNull()
  })

  it('supports cluster filter and cluster column visibility', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          ...stackRows,
          {
            ...stackRows[0],
            id: 'stack-2',
            name: 'Another Stack',
            clusterId: 'cluster-2',
            clusterName: 'dev-cluster',
          },
        ],
        total: 2,
      },
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.getAllByText('Cluster').length).toBeGreaterThan(0)
    fireEvent.change(screen.getByDisplayValue('All Clusters'), { target: { value: 'prod-cluster' } })

    expect(screen.getAllByText('DevSecOps Core').length).toBeGreaterThan(0)
    expect(screen.queryByText('Another Stack')).toBeNull()
  })

})

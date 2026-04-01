import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { screen } from '@testing-library/react'
import { StackListPage } from './stack-list-page'

const mockNavigate = vi.fn()
const mockUseStacks = vi.fn()
const mockUseDeleteStack = vi.fn()
const mockUseStackHistory = vi.fn()
const mockUseStackMonitoring = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('../api/stack-api', () => ({
  useStacks: (...args: unknown[]) => mockUseStacks(...args),
  useDeleteStack: (...args: unknown[]) => mockUseDeleteStack(...args),
  useStackHistory: (...args: unknown[]) => mockUseStackHistory(...args),
  useStackMonitoring: (...args: unknown[]) => mockUseStackMonitoring(...args),
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

    mockUseStacks.mockReturnValue({
      data: { items: stackRows, total: stackRows.length },
      isLoading: false,
    })
    mockUseDeleteStack.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseStackHistory.mockReturnValue({ data: [], isLoading: false })
    mockUseStackMonitoring.mockReturnValue({ data: null, isLoading: false })
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

    expect(screen.getByText('Loading stacks...')).not.toBeNull()
  })

  it('renders stack data rows', () => {
    renderWithProviders(<StackListPage />)

    expect(screen.getAllByText('DevSecOps Core').length).toBeGreaterThan(0)
    expect(screen.getAllByText('GitLab + Argo CD').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Success').length).toBeGreaterThan(0)
  })

  it('keeps a single connection info trigger per detail panel', () => {
    renderWithProviders(<StackListPage />)

    expect(screen.getAllByRole('button', { name: 'Connection Info' })).toHaveLength(2)
  })

  it('renders empty state when no stacks exist', () => {
    mockUseStacks.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
    })

    renderWithProviders(<StackListPage />)

    expect(screen.getByText('No stacks found.')).not.toBeNull()
  })
})

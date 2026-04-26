import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { fireEvent, screen } from '@testing-library/react'
import { StackHistoryPage } from './stack-history-page'

const mockNavigate = vi.fn()
const mockUseParams = vi.fn()

const mockUseStacks = vi.fn()
const mockUseStackHistory = vi.fn()
const mockUseRollbackStack = vi.fn()
const mockUseStackVersionDiff = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => mockUseParams(),
  }
})

vi.mock('../api/stack-api', () => ({
  useStacks: (...args: unknown[]) => mockUseStacks(...args),
  useStackHistory: (...args: unknown[]) => mockUseStackHistory(...args),
  useRollbackStack: (...args: unknown[]) => mockUseRollbackStack(...args),
  useStackVersionDiff: (...args: unknown[]) => mockUseStackVersionDiff(...args),
}))

vi.mock('../components/version-diff', () => ({
  VersionDiff: () => <div data-testid="version-diff">VersionDiff</div>,
}))

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: () => ({ role: 'devops', isAuthenticated: true }),
}))

const stacks = [
  {
    id: 'stack-1',
    name: 'Platform Stack',
    templateId: 'tpl-1',
    templateName: 'GitLab + Argo',
    clusterId: 'cluster-1',
    clusterName: 'prod',
    status: 'completed',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
]

describe('StackHistoryPage', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseParams.mockReset()
    mockUseStacks.mockReset()
    mockUseStackHistory.mockReset()
    mockUseRollbackStack.mockReset()
    mockUseStackVersionDiff.mockReset()

    mockUseParams.mockReturnValue({ stackId: 'stack-1' })
    mockUseStacks.mockReturnValue({ data: { items: stacks } })
    mockUseStackHistory.mockReturnValue({ data: [] })
    mockUseRollbackStack.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseStackVersionDiff.mockReturnValue({ data: null })
  })

  it('renders without crash', () => {
    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Stack History').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Compare Versions' })).not.toBeNull()
  })

  it('handles loading-like state safely when history data is undefined', () => {
    mockUseStackHistory.mockReturnValue({ data: undefined })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Stack History').length).toBeGreaterThan(0)
    expect(screen.getByText(/No data available\.|데이터가 없습니다\.|dataTable.empty/)).not.toBeNull()
  })

  it('renders history data', () => {
    mockUseStackHistory.mockReturnValue({
      data: [
        {
          id: 'h-1',
          stackId: 'stack-1',
          version: 3,
          changedBy: 'alice',
          changedAt: '2026-01-03T10:00:00Z',
          reason: 'Scale up',
          snapshot: { replicas: 3 },
        },
      ],
    })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Platform Stack').length).toBeGreaterThan(0)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Scale up').length).toBeGreaterThan(0)
    expect(screen.getAllByText('v3').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Cluster|클러스터/).length).toBeGreaterThan(0)
    expect(screen.getAllByText('prod').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: /Log|로그/ })).toBeInTheDocument()
  })

  it('renders empty state when no history entries exist', () => {
    mockUseStackHistory.mockReturnValue({ data: [] })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getByText(/No data available\.|데이터가 없습니다\.|dataTable.empty/)).not.toBeNull()
  })

  it('keeps the route stack id when list data is stale', () => {
    mockUseParams.mockReturnValue({ stackId: 'stack-new' })
    mockUseStacks.mockReturnValue({ data: { items: stacks } })

    renderWithProviders(<StackHistoryPage />)

    expect(mockUseStackHistory).toHaveBeenCalledWith('stack-new')
    expect(mockNavigate).not.toHaveBeenCalledWith('/stack/history/stack-1', { replace: true })
    expect(screen.getByRole('option', { name: 'stack-new' })).toBeInTheDocument()
  })

  it('filters stack selector by cluster filter', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          ...stacks,
          {
            id: 'stack-2',
            name: 'Dev Stack',
            templateId: 'tpl-2',
            templateName: 'GitLab + Argo',
            clusterId: 'cluster-2',
            clusterName: 'dev',
            status: 'completed',
            createdAt: '2026-01-01T00:00:00Z',
            updatedAt: '2026-01-01T00:00:00Z',
          },
        ],
      },
    })

    renderWithProviders(<StackHistoryPage />)

    fireEvent.change(screen.getByDisplayValue(/All Clusters|전체 클러스터/), { target: { value: 'prod' } })

    expect(screen.getByRole('option', { name: 'Platform Stack' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'Dev Stack' })).toBeNull()
  })

})

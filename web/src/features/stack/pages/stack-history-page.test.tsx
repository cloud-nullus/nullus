import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { screen } from '@testing-library/react'
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
    expect(screen.getByRole('combobox', { name: 'Stack' })).not.toBeNull()
  })

  it('handles loading-like state safely when history data is undefined', () => {
    mockUseStackHistory.mockReturnValue({ data: undefined })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Stack History').length).toBeGreaterThan(0)
    expect(screen.getByText(/No data available\.|데이터가 없습니다\./)).not.toBeNull()
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
  })

  it('renders empty state when no history entries exist', () => {
    mockUseStackHistory.mockReturnValue({ data: [] })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getByText(/No data available\.|데이터가 없습니다\./)).not.toBeNull()
  })
})

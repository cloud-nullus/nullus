import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { AlertRulesPage } from './alert-rules-page'

const mockUseAlertRules = vi.hoisted(() => vi.fn())
const mockUseCreateAlertRule = vi.hoisted(() => vi.fn())
const mockUseUpdateAlertRule = vi.hoisted(() => vi.fn())
const mockUseDeleteAlertRule = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())

vi.mock('../api/observability-api', async () => {
  const actual = await vi.importActual('../api/observability-api')
  return {
    ...actual,
    useAlertRules: mockUseAlertRules,
    useCreateAlertRule: mockUseCreateAlertRule,
    useUpdateAlertRule: mockUseUpdateAlertRule,
    useDeleteAlertRule: mockUseDeleteAlertRule,
  }
})

vi.mock('../components/cluster-stack-filter', () => ({
  useClusterStackFilterState: mockUseClusterStackFilterState,
  ClusterStackFilter: () => <div>ClusterStackFilter</div>,
}))

describe('AlertRulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    mockUseAlertRules.mockReturnValue({ data: { items: [], total: 0 } })
    mockUseCreateAlertRule.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseUpdateAlertRule.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseDeleteAlertRule.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [],
      filteredStacks: [],
      selectedCluster: undefined,
      selectedStack: undefined,
    })
  })

  it('renders page title without crashing', () => {
    renderWithProviders(<AlertRulesPage />)
    expect(screen.getByRole('heading', { level: 1, name: 'Alert Rules' })).not.toBeNull()
  })

  it('shows stable loading-state UI while rules are not loaded yet', () => {
    mockUseAlertRules.mockReturnValue({ data: undefined })

    renderWithProviders(<AlertRulesPage />)

    expect(screen.getByRole('heading', { level: 1, name: 'Alert Rules' })).not.toBeNull()
    expect(screen.queryByText('알림 규칙이 없습니다.')).not.toBeNull()
  })

  it('renders rules when hook returns data', () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-1',
            name: 'High CPU',
            severity: 'critical',
            condition: 'cpu_usage > 80',
            threshold: '80',
            channel: 'slack',
            enabled: true,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 1,
      },
    })

    renderWithProviders(<AlertRulesPage />)

    expect(screen.queryByText('High CPU')).not.toBeNull()
    expect(screen.queryByText('cpu_usage > 80')).not.toBeNull()
    expect(screen.queryByText('slack')).not.toBeNull()
  })

  it('shows empty state when there are no rules', () => {
    mockUseAlertRules.mockReturnValue({ data: { items: [], total: 0 } })

    renderWithProviders(<AlertRulesPage />)

    expect(screen.queryByText('알림 규칙이 없습니다.')).not.toBeNull()
  })

  it('falls back missing severity to warning badge for malformed data', () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-2',
            name: 'No Severity Rule',
            condition: 'memory_usage > 90',
            threshold: '90',
            channel: 'email',
            enabled: true,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 1,
      },
    })

    renderWithProviders(<AlertRulesPage />)

    expect(screen.queryByText('No Severity Rule')).not.toBeNull()
    expect(screen.queryByText('Warning')).not.toBeNull()
  })

  it('filters rules by search input', () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-1',
            name: 'High CPU',
            severity: 'critical',
            condition: 'cpu_usage > 80',
            threshold: '80',
            channel: 'slack',
            enabled: true,
            createdAt: '2026-01-01T00:00:00Z',
          },
          {
            id: 'rule-2',
            name: 'Disk Alert',
            severity: 'warning',
            condition: 'disk_usage > 90',
            threshold: '90',
            channel: 'email',
            enabled: true,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 2,
      },
    })

    renderWithProviders(<AlertRulesPage />)
    fireEvent.change(screen.getByPlaceholderText('규칙명 / 메트릭 검색...'), { target: { value: 'disk' } })

    expect(screen.queryByText('Disk Alert')).not.toBeNull()
    expect(screen.queryByText('High CPU')).toBeNull()
  })
})

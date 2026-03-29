import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { AlertRulesPage } from './alert-rules-page'

const mockUseAlertRules = vi.hoisted(() => vi.fn())
const mockUseAlertRule = vi.hoisted(() => vi.fn())
const mockUseCreateAlertRule = vi.hoisted(() => vi.fn())
const mockUseUpdateAlertRule = vi.hoisted(() => vi.fn())
const mockUseDeleteAlertRule = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())

vi.mock('../api/observability-api', async () => {
  const actual = await vi.importActual('../api/observability-api')
  return {
    ...actual,
    useAlertRules: mockUseAlertRules,
    useAlertRule: mockUseAlertRule,
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

    mockUseAlertRules.mockReturnValue({ data: { items: [], total: 0 }, refetch: vi.fn().mockResolvedValue(undefined) })
    mockUseAlertRule.mockReturnValue({ data: undefined, isFetching: false })
    mockUseCreateAlertRule.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false })
    mockUseUpdateAlertRule.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false })
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
    expect(screen.queryByText('No alert rules found.')).not.toBeNull()
  })

  it('renders rules when hook returns data', () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-1',
            name: 'High CPU',
            metric_name: 'cpu_usage',
            condition: 'cpu_usage >= critical_threshold',
            warning_threshold: 70,
            critical_threshold: 80,
            threshold: 80,
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
    expect(screen.queryByText('cpu_usage')).not.toBeNull()
    expect(screen.queryByText('cpu_usage >= critical_threshold')).not.toBeNull()
    expect(screen.queryByText('Warning: 70')).not.toBeNull()
    expect(screen.queryByText('Critical: 80')).not.toBeNull()
    expect(screen.queryByText('slack')).not.toBeNull()
  })

  it('shows empty state when there are no rules', () => {
    mockUseAlertRules.mockReturnValue({ data: { items: [], total: 0 } })

    renderWithProviders(<AlertRulesPage />)

    expect(screen.queryByText('No alert rules found.')).not.toBeNull()
  })

  it('renders edit action button', () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-2',
            name: 'Latency Alert',
            metric_name: 'latency_p95',
            condition: 'latency_p95 >= critical_threshold',
            warning_threshold: 250,
            critical_threshold: 300,
            threshold: 300,
            channel: 'email',
            enabled: true,
          },
        ],
        total: 1,
      },
    })

    renderWithProviders(<AlertRulesPage />)

    expect(screen.getByRole('button', { name: 'Edit' })).not.toBeNull()
  })

  it('filters rules by search input', () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-1',
            name: 'High CPU',
            metric_name: 'cpu_usage',
            condition: 'cpu_usage >= critical_threshold',
            warning_threshold: 70,
            critical_threshold: 80,
            threshold: 80,
            channel: 'slack',
            enabled: true,
            createdAt: '2026-01-01T00:00:00Z',
          },
          {
            id: 'rule-2',
            name: 'Disk Alert',
            metric_name: 'disk_usage',
            condition: 'disk_usage >= critical_threshold',
            warning_threshold: 75,
            critical_threshold: 90,
            threshold: 90,
            channel: 'email',
            enabled: true,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 2,
      },
    })

    renderWithProviders(<AlertRulesPage />)
    fireEvent.change(screen.getByPlaceholderText('Search by rule or metric...'), { target: { value: 'disk' } })

    expect(screen.queryByText('Disk Alert')).not.toBeNull()
    expect(screen.queryByText('High CPU')).toBeNull()
  })

  it('loads latest alert rule data from DB when opening edit modal', async () => {
    mockUseAlertRules.mockReturnValue({
      data: {
        items: [
          {
            id: 'rule-1',
            name: 'Old CPU',
            metric_name: 'cpu_old',
            condition: 'cpu_old >= critical_threshold',
            warning_threshold: 60,
            critical_threshold: 70,
            threshold: 70,
            channel: 'slack',
            enabled: true,
          },
        ],
        total: 1,
      },
      refetch: vi.fn().mockResolvedValue(undefined),
    })
    mockUseAlertRule.mockImplementation((id: string | null) => ({
      data: id === 'rule-1'
        ? {
            id: 'rule-1',
            name: 'Fresh CPU',
            metric_name: 'cpu_usage',
            condition: 'cpu_usage >= critical_threshold',
            warning_threshold: 75,
            critical_threshold: 90,
            threshold: 90,
            channel: 'email',
            enabled: false,
          }
        : undefined,
      isFetching: false,
    }))

    renderWithProviders(<AlertRulesPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))

    await waitFor(() => {
      expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe('Fresh CPU')
    })
    expect((screen.getByLabelText('Metric Name') as HTMLInputElement).value).toBe('cpu_usage')
  })
})

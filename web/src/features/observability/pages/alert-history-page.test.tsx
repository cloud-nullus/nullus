import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { AlertHistoryPage } from './alert-history-page'

const mockUseStackAlerts = vi.hoisted(() => vi.fn())
const mockUseClusterStackFilterState = vi.hoisted(() => vi.fn())

vi.mock('../api/observability-api', async () => {
  const actual = await vi.importActual('../api/observability-api')
  return { ...actual, useStackAlerts: mockUseStackAlerts }
})

vi.mock('../components/cluster-stack-filter', () => ({
  useClusterStackFilterState: mockUseClusterStackFilterState,
  ClusterStackFilter: () => <div>ClusterStackFilter</div>,
}))

const recent = new Date(Date.now() - 60 * 60 * 1000).toISOString()

const active = {
  id: 'a1',
  stack_id: 'stk_1',
  stack_name: 'gitlab-sca',
  alertname: 'KubePodNotReady',
  severity: 'critical',
  summary: 'Pod has been in a non-ready state for more than 15 minutes.',
  description: 'Pod nullus-gitlab/gitlab-webservice-0 has been in a non-ready state.',
  runbook_url: 'https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepodnotready',
  target: 'nullus-gitlab/gitlab-webservice-0',
  labels: { namespace: 'nullus-gitlab', pod: 'gitlab-webservice-0' },
  started_at: recent,
  last_notified_at: recent,
  active: true,
}
const resolved = {
  ...active,
  id: 'a2',
  alertname: 'NodeFilesystemAlmostOutOfSpace',
  summary: 'Filesystem has less than 3% space left.',
  target: '10.0.0.1:9100',
  resolved_at: recent,
  active: false,
}

describe('AlertHistoryPage (스택 경보)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseClusterStackFilterState.mockReturnValue({
      clusters: [],
      filteredStacks: [],
      selectedCluster: undefined,
      selectedStack: undefined,
    })
    mockUseStackAlerts.mockReturnValue({ data: [active, resolved] })
  })

  it('shows stack alerts with stack, target, and active/resolved state', () => {
    renderWithProviders(<AlertHistoryPage />)
    expect(screen.getByRole('heading', { level: 1, name: 'Alert History' })).toBeInTheDocument()

    const row = screen.getByText('KubePodNotReady').closest('tr') as HTMLElement
    expect(within(row).getByText('gitlab-sca')).toBeInTheDocument()
    expect(within(row).getByText('nullus-gitlab/gitlab-webservice-0')).toBeInTheDocument()
    expect(within(row).getByText('Active')).toBeInTheDocument()

    const resolvedRow = screen.getByText('NodeFilesystemAlmostOutOfSpace').closest('tr') as HTMLElement
    expect(within(resolvedRow).queryByText('Active')).not.toBeInTheDocument()
  })

  it('asks for every alert by default and only active ones when filtered', () => {
    renderWithProviders(<AlertHistoryPage />)
    expect(mockUseStackAlerts).toHaveBeenLastCalledWith('all')
    fireEvent.click(screen.getByRole('button', { name: 'Active only' }))
    expect(mockUseStackAlerts).toHaveBeenLastCalledWith('open')
  })

  it('shows detail with description, runbook and labels', () => {
    renderWithProviders(<AlertHistoryPage />)
    fireEvent.click(screen.getByText('KubePodNotReady'))
    expect(screen.getByText('Alert Detail')).toBeInTheDocument()
    expect(screen.getByText(/has been in a non-ready state\./)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /runbook/i })).toHaveAttribute('href', active.runbook_url)
    expect(screen.getByText('pod=gitlab-webservice-0')).toBeInTheDocument()
  })

  it('filters by search text over alert, stack and summary', () => {
    renderWithProviders(<AlertHistoryPage />)
    fireEvent.change(screen.getByPlaceholderText('Search alert, stack, or message...'), {
      target: { value: 'filesystem' },
    })
    expect(screen.queryByText('KubePodNotReady')).not.toBeInTheDocument()
    expect(screen.getByText('NodeFilesystemAlmostOutOfSpace')).toBeInTheDocument()
  })

  // 기간은 "그 기간에 걸쳐 있었는지" 로 본다 — 8일 전에 시작해 아직 진행 중인 경보가 7일 보기에서 숨으면 안 된다.
  it('keeps long-running active alerts in the default 7-day view', () => {
    const old = new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString()
    mockUseStackAlerts.mockReturnValue({
      data: [
        { ...active, id: 'old-active', alertname: 'LongRunningActive', started_at: old },
        { ...resolved, id: 'old-resolved', alertname: 'LongAgoResolved', started_at: old, resolved_at: old },
      ],
    })
    renderWithProviders(<AlertHistoryPage />)
    expect(screen.getByText('LongRunningActive')).toBeInTheDocument()
    expect(screen.queryByText('LongAgoResolved')).not.toBeInTheDocument()
  })

  it('shows an empty state that explains where alerts come from', () => {
    mockUseStackAlerts.mockReturnValue({ data: [] })
    renderWithProviders(<AlertHistoryPage />)
    expect(screen.getByText(/No critical alerts from your stacks/)).toBeInTheDocument()
  })
})

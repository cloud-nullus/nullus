import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackDeploymentLogsPage } from './stack-deployment-logs-page'
import type { Stack } from '../api/stack-api'

const mockStacks: Stack[] = [
  {
    id: 'real-completed-stack',
    name: 'analytics-prod',
    templateId: 'tpl-1',
    templateName: 'Base Template',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    namespace: 'nullus',
    status: 'completed',
    createdAt: '2026-04-01T00:00:00Z',
    updatedAt: '2026-04-19T00:00:00Z',
  },
  {
    id: 'real-failed-stack',
    name: 'checkout-prod',
    templateId: 'tpl-2',
    templateName: 'Checkout Template',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    namespace: 'nullus',
    status: 'failed',
    createdAt: '2026-04-01T00:00:00Z',
    updatedAt: '2026-04-19T00:00:00Z',
  },
  {
    id: 'real-pending-stack',
    name: 'billing-prod',
    templateId: 'tpl-3',
    templateName: 'Billing Template',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    namespace: 'nullus',
    status: 'pending',
    createdAt: '2026-04-20T00:00:00Z',
    updatedAt: '2026-04-20T00:00:00Z',
  },
]

let currentParamId: string | undefined = undefined

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useParams: () => ({ deploymentId: currentParamId }),
    useNavigate: () => vi.fn(),
  }
})

const retryHistoryItems = vi.hoisted(() => ({ current: [] as Array<{
  id: string; timestamp: string; actor: string; verdict?: string; issueCodes?: string[]; acknowledgeWarnings: boolean
}> }))

vi.mock('../api/stack-api', () => ({
  useStacks: () => ({ data: { items: mockStacks, total: mockStacks.length }, isLoading: false }),
  useStackRetryHistory: () => ({ data: { items: retryHistoryItems.current } }),
}))

vi.mock('../components/retry-stack-button', () => ({
  RetryStackButton: ({ stackId, status }: { stackId: string; status: string }) =>
    status === 'failed' || status === 'rolled_back'
      ? <button type="button" data-testid={`retry-${stackId}`}>Retry</button>
      : null,
}))

beforeEach(() => {
  vi.clearAllMocks()
  currentParamId = undefined
  retryHistoryItems.current = []
  // jsdom does not implement scrollIntoView; the legacy mock path uses it
  // inside a useEffect when it streams log lines.
  Element.prototype.scrollIntoView = vi.fn()
})

describe('StackDeploymentLogsPage', () => {
  it('falls back to the DEPLOYMENT_DATA mock entry when id matches a fixture', () => {
    currentParamId = 'deploy-v1-20260220'
    renderWithProviders(<StackDeploymentLogsPage />)
    // Legacy path renders the mocked metadata line and stage table.
    expect(screen.getByText(/Initial stack deployment/)).toBeInTheDocument()
    // No retry button on legacy render.
    expect(screen.queryByTestId(/^retry-/)).not.toBeInTheDocument()
  })

  it('renders real stack info without Retry when the stack is completed', () => {
    currentParamId = 'real-completed-stack'
    renderWithProviders(<StackDeploymentLogsPage />)
    expect(screen.getByText(/analytics-prod/)).toBeInTheDocument()
    expect(screen.queryByTestId('retry-real-completed-stack')).not.toBeInTheDocument()
    expect(screen.getByText(/Live log streaming is not yet connected/)).toBeInTheDocument()
  })

  it('renders Retry button when the real stack is in failed state', () => {
    currentParamId = 'real-failed-stack'
    renderWithProviders(<StackDeploymentLogsPage />)
    expect(screen.getByText(/checkout-prod/)).toBeInTheDocument()
    expect(screen.getByTestId('retry-real-failed-stack')).toBeInTheDocument()
  })

  it('shows "Deployment not found" when neither mock nor real stack matches', () => {
    currentParamId = 'nonexistent-stack'
    renderWithProviders(<StackDeploymentLogsPage />)
    expect(screen.getByText(/Deployment not found: nonexistent-stack/)).toBeInTheDocument()
  })

  it('real timeline renders for a pending stack without a terminal marker', () => {
    currentParamId = 'real-pending-stack'
    renderWithProviders(<StackDeploymentLogsPage />)
    const timeline = screen.getByTestId('real-timeline')
    expect(timeline).toBeInTheDocument()
    // No terminal state while still in-flight.
    expect(timeline.getAttribute('data-terminal')).toBeNull()
  })

  it('real timeline tags a failed stack with data-terminal="failed"', () => {
    currentParamId = 'real-failed-stack'
    renderWithProviders(<StackDeploymentLogsPage />)
    const timeline = screen.getByTestId('real-timeline')
    expect(timeline.getAttribute('data-terminal')).toBe('failed')
  })

  it('real timeline tags a completed stack with data-terminal="completed"', () => {
    currentParamId = 'real-completed-stack'
    renderWithProviders(<StackDeploymentLogsPage />)
    const timeline = screen.getByTestId('real-timeline')
    expect(timeline.getAttribute('data-terminal')).toBe('completed')
  })

  // F8-UIUX-RetryAuditSurface-Frontend
  it('hides the retry-history panel when there are no retry events', () => {
    currentParamId = 'real-failed-stack'
    retryHistoryItems.current = []
    renderWithProviders(<StackDeploymentLogsPage />)
    expect(screen.queryByTestId('retry-history-panel')).not.toBeInTheDocument()
  })

  it('renders retry-history rows with verdict and issue codes', () => {
    currentParamId = 'real-failed-stack'
    retryHistoryItems.current = [
      {
        id: 'a1',
        timestamp: '2026-04-21T09:15:03Z',
        actor: 'u-1',
        acknowledgeWarnings: true,
        verdict: 'warn',
        issueCodes: ['TOOL_ARCH_UNSUPPORTED'],
      },
      {
        id: 'a2',
        timestamp: '2026-04-21T09:14:00Z',
        actor: 'u-1',
        acknowledgeWarnings: false,
        verdict: 'pass',
      },
    ]
    renderWithProviders(<StackDeploymentLogsPage />)
    const panel = screen.getByTestId('retry-history-panel')
    expect(panel).toBeInTheDocument()
    expect(screen.getByText('TOOL_ARCH_UNSUPPORTED')).toBeInTheDocument()
    expect(screen.getAllByText(/warn|pass/).length).toBeGreaterThanOrEqual(2)
  })

  it('expands the retry-history toggle when there are more than 3 entries', () => {
    currentParamId = 'real-failed-stack'
    retryHistoryItems.current = Array.from({ length: 5 }, (_, i) => ({
      id: `a${i}`,
      timestamp: '2026-04-21T09:15:0' + i + 'Z',
      actor: 'u-1',
      acknowledgeWarnings: false,
      verdict: i === 4 ? 'pass' : 'warn',
      issueCodes: ['TOOL_ARCH_UNSUPPORTED'],
    }))
    renderWithProviders(<StackDeploymentLogsPage />)
    const toggle = screen.getByTestId('retry-history-toggle')
    expect(toggle).toBeInTheDocument()
    // Initial render should show 3 rows.
    const panel = screen.getByTestId('retry-history-panel')
    expect(panel.querySelectorAll('tbody tr').length).toBe(3)
    fireEvent.click(toggle)
    expect(panel.querySelectorAll('tbody tr').length).toBe(5)
  })
})

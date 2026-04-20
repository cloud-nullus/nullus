import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen } from '@testing-library/react'
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

vi.mock('../api/stack-api', () => ({
  useStacks: () => ({ data: { items: mockStacks, total: mockStacks.length }, isLoading: false }),
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
})

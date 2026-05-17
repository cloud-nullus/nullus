import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { fireEvent, screen } from '@testing-library/react'
import { StackDeployPage } from './stack-deploy-page'

const mockUseParams = vi.fn()
const mockUseDeployLog = vi.fn()
const mockUsePodWatch = vi.fn()
const mockApiGet = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useParams: () => mockUseParams(),
  }
})

vi.mock('../hooks/use-deploy-log', () => ({
  useDeployLog: (...args: unknown[]) => mockUseDeployLog(...args),
}))

vi.mock('../hooks/use-pod-watch', () => ({
  usePodWatch: (...args: unknown[]) => mockUsePodWatch(...args),
}))

vi.mock('../../../lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
  },
}))

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: () => ({ role: 'devops', isAuthenticated: true }),
}))

describe('StackDeployPage', () => {
  beforeEach(() => {
    mockUseParams.mockReset()
    mockUseDeployLog.mockReset()
    mockUsePodWatch.mockReset()
    mockApiGet.mockReset()

    mockUseParams.mockReturnValue({ id: 'deploy-1' })
    mockUseDeployLog.mockReturnValue({
      logs: [],
      status: 'connecting',
      progress: 0,
      isConnected: false,
    })
    mockUsePodWatch.mockReturnValue({
      pods: [],
      error: null,
      isConnected: false,
    })
    mockApiGet.mockReturnValue(new Promise(() => undefined))

    Element.prototype.scrollIntoView = vi.fn()
  })

  it('renders without crash', async () => {
    renderWithProviders(<StackDeployPage />)

    expect(screen.getAllByText('Deployment Log').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Deployment ID: deploy-1/).length).toBeGreaterThan(0)
    expect(screen.getByText('$ kubectl get pods -n nullus -w')).not.toBeNull()
  })

  it('shows loading state while websocket is connecting', async () => {
    renderWithProviders(<StackDeployPage />)

    expect(screen.getByText('Raw Logs (0)')).not.toBeNull()
    expect(screen.getByText('Hide')).not.toBeNull()
  })

  it('renders deployment log data when logs are available', async () => {
    mockUseDeployLog.mockReturnValue({
      logs: [
        {
          id: 'log-1',
          timestamp: '2026-01-01T10:00:00Z',
          level: 'info',
          step: 'installing_gitlab',
          message: 'Deploy started',
        },
        {
          id: 'log-2',
          timestamp: '2026-01-01T10:01:00Z',
          level: 'warn',
          step: 'installing_gitlab',
          message: 'GitLab pod still pending',
        },
      ],
      status: 'running',
      progress: 45,
      isConnected: true,
    })
    mockUsePodWatch.mockReturnValue({
      pods: [
        {
          name: 'gitlab-webservice-default-0',
          ready: '0/1',
          status: 'Pending',
          restarts: '0',
          age: '2m',
          updatedAt: '2026-01-01T10:01:00Z',
        },
      ],
      error: null,
      isConnected: true,
    })

    renderWithProviders(<StackDeployPage />)

    expect(screen.getByText(/Raw Logs \(2\)/)).not.toBeNull()
    expect(screen.getByText(/Attention \(1\)/)).not.toBeNull()
    expect(screen.getAllByText('GitLab pod still pending').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/gitlab-webservice/).length).toBeGreaterThan(0)
    expect(screen.getByText('Pending')).not.toBeNull()
    expect(screen.getByText('Deploy started')).not.toBeNull()
    expect(screen.getByText('45%')).not.toBeNull()
  })

  it('shows empty state when connected but no logs yet', async () => {
    mockUseDeployLog.mockReturnValue({
      logs: [],
      status: 'running',
      progress: 0,
      isConnected: true,
    })

    renderWithProviders(<StackDeployPage />)

    expect(screen.getByText('Waiting for logs...')).not.toBeNull()
  })
})

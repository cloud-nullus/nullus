import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { act, screen } from '@testing-library/react'
import { StackDeployPage } from './stack-deploy-page'

const mockUseParams = vi.fn()
const mockUseNavigate = vi.fn()
const mockUseDeployLog = vi.fn()
const mockApiGet = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useParams: () => mockUseParams(),
    useNavigate: () => mockUseNavigate,
  }
})

vi.mock('../hooks/use-deploy-log', () => ({
  useDeployLog: (...args: unknown[]) => mockUseDeployLog(...args),
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
    mockUseNavigate.mockReset()
    mockUseDeployLog.mockReset()
    mockApiGet.mockReset()

    mockUseParams.mockReturnValue({ id: 'deploy-1' })
    mockUseDeployLog.mockReturnValue({
      logs: [],
      status: 'connecting',
      progress: 0,
      isConnected: false,
    })
    mockApiGet.mockReturnValue(new Promise(() => undefined))

    Element.prototype.scrollIntoView = vi.fn()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders without crash', async () => {
    renderWithProviders(<StackDeployPage />)

    expect(screen.getAllByText('Deployment Log').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Deployment ID: deploy-1/).length).toBeGreaterThan(0)
  })

  it('shows loading state while websocket is connecting', async () => {
    renderWithProviders(<StackDeployPage />)

    expect(screen.getByText('Connecting to WebSocket...')).not.toBeNull()
  })

  it('renders deployment log data when logs are available', async () => {
    mockUseDeployLog.mockReturnValue({
      logs: [
        {
          id: 'log-1',
          timestamp: '2026-01-01T10:00:00Z',
          level: 'info',
          message: 'Deploy started',
        },
      ],
      status: 'running',
      progress: 45,
      isConnected: true,
    })

    renderWithProviders(<StackDeployPage />)

    expect(screen.getByText(/Logs \(1\)/)).not.toBeNull()
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

  it('shows success summary from API state even without live log frames', async () => {
    mockApiGet.mockResolvedValue({
      data: {
        data: {
          state: 'completed',
        },
      },
    })

    renderWithProviders(<StackDeployPage />)

    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByText('Deployment Completed')).toBeInTheDocument()
    expect(screen.getByText('Deployment completed. No buffered live logs were retained for this session.')).toBeInTheDocument()
  })

  it('toggles current session log filter label', async () => {
    mockUseDeployLog.mockReturnValue({
      logs: [{ id: 'l1', timestamp: '2026-01-01T10:00:00Z', level: 'info', message: 'validation complete' }],
      status: 'running',
      progress: 20,
      isConnected: true,
    })

    renderWithProviders(<StackDeployPage />)

    expect(screen.getByRole('button', { name: 'Current session only' })).toBeInTheDocument()
    act(() => {
      screen.getByRole('button', { name: 'Current session only' }).click()
    })
    expect(screen.getByRole('button', { name: 'Show all buffered logs' })).toBeInTheDocument()
  })

  it('redirects to stack list 5 seconds after true success completion', async () => {
    vi.useFakeTimers()

    mockUseDeployLog.mockReturnValue({
      logs: [
        {
          id: 'log-1',
          timestamp: '2026-01-01T10:00:00Z',
          level: 'success',
          message: 'installation completed successfully',
        },
      ],
      status: 'success',
      progress: 100,
      isConnected: true,
    })

    renderWithProviders(<StackDeployPage />)

    expect(screen.getByText(/Returning to Stack List in 5s/)).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(mockUseNavigate).toHaveBeenCalledWith('/stack/list')
  })
})

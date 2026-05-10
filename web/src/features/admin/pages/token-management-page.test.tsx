import { beforeEach, describe, expect, it, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { TokenManagementPage } from './token-management-page'

const mockUseTokenSources = vi.hoisted(() => vi.fn())
const mockUseTokenSourceEvents = vi.hoisted(() => vi.fn())
const mockRotate = vi.hoisted(() => vi.fn())
const mockApprove = vi.hoisted(() => vi.fn())
const mockPause = vi.hoisted(() => vi.fn())
const mockResume = vi.hoisted(() => vi.fn())
const mockReAuthAsync = vi.hoisted(() => vi.fn())
const mockRevealAsync = vi.hoisted(() => vi.fn())

vi.mock('../api/admin-api', () => ({
  useTokenSources: (...args: unknown[]) => mockUseTokenSources(...args),
  useTokenSourceEvents: (...args: unknown[]) => mockUseTokenSourceEvents(...args),
  useRotateTokenSource: () => ({ mutate: mockRotate }),
  useApproveTokenSource: () => ({ mutate: mockApprove }),
  usePauseTokenSource: () => ({ mutate: mockPause }),
  useResumeTokenSource: () => ({ mutate: mockResume }),
  useReAuthTokenSource: () => ({ mutateAsync: mockReAuthAsync }),
  useRevealTokenSource: () => ({ mutateAsync: mockRevealAsync }),
}))

describe('TokenManagementPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseTokenSources.mockReturnValue({
      data: {
        items: [
          {
            id: 'ts-1',
            org_id: 'org-1',
            module: 'cicd',
            provider: 'github',
            path: 'kv/nullus/dev/org-1/cicd/github',
            token_type: 'reissue',
            status: 'healthy',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ],
        total: 1,
      },
      isLoading: false,
    })
    mockUseTokenSourceEvents.mockReturnValue({ data: { items: [], total: 0 } })
    mockReAuthAsync.mockResolvedValue({ step_up_token: 'step-up-1', expires_in_seconds: 300 })
    mockRevealAsync.mockResolvedValue({ token_value: 'stored-in-openbao' })
  })

  it('renders token list', () => {
    renderWithProviders(<TokenManagementPage />)
    expect(screen.getAllByText('OpenBao Token Management').length).toBeGreaterThan(0)
    expect(screen.getByText('github')).toBeInTheDocument()
  })

  it('triggers rotate action', () => {
    renderWithProviders(<TokenManagementPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Rotate' }))
    expect(mockRotate).toHaveBeenCalled()
  })

  it('reveals token info in test mode', async () => {
    renderWithProviders(<TokenManagementPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Reveal' }))
    await waitFor(() => {
      expect(mockReAuthAsync).toHaveBeenCalled()
      expect(mockRevealAsync).toHaveBeenCalled()
      expect(screen.getByText('Reveal Result')).toBeInTheDocument()
    })
  })
})

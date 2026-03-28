import { beforeEach, describe, expect, it, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { KnownIssuesPage } from './known-issues-page'

const mockNavigate = vi.hoisted(() => vi.fn())
const mockUseKnownIssues = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({ pathname: '/admin/known-issues', search: '', hash: '', state: null, key: 'test' }),
  }
})

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: vi.fn(() => ({ role: 'admin', user: null, isAuthenticated: true })),
}))

vi.mock('../api/admin-api', () => ({
  useKnownIssues: (...args: unknown[]) => mockUseKnownIssues(...args),
}))

describe('KnownIssuesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    mockUseKnownIssues.mockReturnValue({
      data: {
        items: [
          {
            id: 'ISSUE-101',
            severity: 'high',
            title: 'Prometheus scrape timeout',
            description: 'Intermittent timeout under high load.',
            status: 'open',
            workaround: 'Increase scrape timeout to 30s.',
          },
        ],
      },
      isLoading: false,
    })
  })

  it('renders without crash', () => {
    renderWithProviders(<KnownIssuesPage />)

    expect(screen.getAllByText('Known Issues').length).toBeGreaterThan(0)
  })

  it('shows loading state', () => {
    mockUseKnownIssues.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<KnownIssuesPage />)

    expect(screen.queryAllByText('Loading known issues...').length).toBeGreaterThan(0)
  })

  it('renders issue data rows', () => {
    renderWithProviders(<KnownIssuesPage />)

    expect(screen.getByText('ISSUE-101')).toBeInTheDocument()
    expect(screen.getByText('Prometheus scrape timeout')).toBeInTheDocument()
    expect(screen.getByText('Increase scrape timeout to 30s.')).toBeInTheDocument()
  })

  it('shows empty state when there is no issue', () => {
    mockUseKnownIssues.mockReturnValue({ data: { items: [] }, isLoading: false })

    renderWithProviders(<KnownIssuesPage />)

    expect(screen.queryAllByText('No known issues.').length).toBeGreaterThan(0)
  })
})

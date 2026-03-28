import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackVersionPage } from './stack-version-page'

const mockUseCompatibilityMatrix = vi.fn()
const mockUseValidateCompatibility = vi.fn()

vi.mock('../api/stack-api', () => ({
  useCompatibilityMatrix: (...args: unknown[]) => mockUseCompatibilityMatrix(...args),
  useValidateCompatibility: (...args: unknown[]) => mockUseValidateCompatibility(...args),
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => vi.fn(),
  }
})

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: () => ({ role: 'devops', isAuthenticated: true }),
}))

const compatibilityRows = [
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + Argo CD',
    status: 'verified',
    k8sRange: '1.29-1.31',
    tools: [
      { name: 'GitLab', helmVersion: '9.5.1', appVersion: '17.0.1' },
      { name: 'Argo CD', helmVersion: '6.8.0', appVersion: '2.8.3' },
      { name: 'Prometheus', helmVersion: '24.0.0', appVersion: '2.54.1' },
      { name: 'Grafana', helmVersion: '8.6.0', appVersion: '11.0.0' },
      { name: 'OpenTelemetry', helmVersion: '0.67.0', appVersion: '0.93.0' },
    ],
  },
]

describe('StackVersionPage', () => {
  beforeEach(() => {
    mockUseCompatibilityMatrix.mockReset()
    mockUseValidateCompatibility.mockReset()

    mockUseCompatibilityMatrix.mockReturnValue({ data: compatibilityRows })
    mockUseValidateCompatibility.mockReturnValue({ mutate: vi.fn() })
  })

  it('renders without crash', () => {
    renderWithProviders(<StackVersionPage />)

    expect(screen.getAllByText('Stack Version').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Validate Current Stack' })).not.toBeNull()
  })

  it('shows loading state while compatibility validation is in progress', () => {
    mockUseValidateCompatibility.mockReturnValue({
      mutate: (_: unknown, options: { onSuccess?: (value: unknown) => void }) => {
        void options
      },
    })

    renderWithProviders(<StackVersionPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Validate Current Stack' }))

    expect(screen.getByText('검증 중...')).not.toBeNull()
  })

  it('renders compatibility matrix data', () => {
    renderWithProviders(<StackVersionPage />)

    expect(screen.getAllByText('17.0.1').length).toBeGreaterThan(0)
    expect(screen.getAllByText('2.8.3').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Verified').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Recommended').length).toBeGreaterThan(0)
  })

  it('renders empty state safely when matrix data is empty', () => {
    mockUseCompatibilityMatrix.mockReturnValue({ data: [] })

    renderWithProviders(<StackVersionPage />)

    expect(screen.getAllByText('Verified Combinations').length).toBeGreaterThan(0)
    expect(screen.queryAllByText('Verified')).toHaveLength(0)
  })
})

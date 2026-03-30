import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackVersionPage } from './stack-version-page'

const mockUseCompatibilityMatrix = vi.fn()
const mockUseValidateCompatibility = vi.fn()
const mockUseStacks = vi.fn()
const mockUseClusterK8sVersion = vi.fn()

vi.mock('../api/stack-api', () => ({
  useCompatibilityMatrix: (...args: unknown[]) => mockUseCompatibilityMatrix(...args),
  useValidateCompatibility: (...args: unknown[]) => mockUseValidateCompatibility(...args),
  useStacks: (...args: unknown[]) => mockUseStacks(...args),
  useClusterK8sVersion: (...args: unknown[]) => mockUseClusterK8sVersion(...args),
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
      { name: 'OpenTelemetry', helmVersion: '-', appVersion: '0.93.0' },
    ],
  },
]

describe('StackVersionPage', () => {
  beforeEach(() => {
    mockUseCompatibilityMatrix.mockReset()
    mockUseValidateCompatibility.mockReset()
    mockUseStacks.mockReset()
    mockUseClusterK8sVersion.mockReset()

    mockUseCompatibilityMatrix.mockReturnValue({ data: compatibilityRows })
    mockUseValidateCompatibility.mockReturnValue({ mutate: vi.fn() })
    mockUseStacks.mockReturnValue({ data: { items: [], total: 0 }, isLoading: false })
    mockUseClusterK8sVersion.mockReturnValue({ mutate: vi.fn() })
  })

  it('renders page title and validate button', () => {
    renderWithProviders(<StackVersionPage />)

    expect(screen.getAllByText('Stack Version').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Validate Current Stack' })).toBeInTheDocument()
  })

  it('renders matrix name and k8s range header', () => {
    renderWithProviders(<StackVersionPage />)

    expect(screen.getAllByText('GitLab + Argo CD').length).toBeGreaterThan(0)
    expect(screen.getByText(/k8sRange|K8s Range/i)).toBeInTheDocument()
    expect(screen.getAllByText('Verified').length).toBeGreaterThan(0)
  })

  it('shows setup breakdown and postgres fallback', () => {
    renderWithProviders(<StackVersionPage />)

    expect(screen.getAllByText(/Mixed|Helm|Deployment/).length).toBeGreaterThan(0)
    expect(screen.getAllByText('N/A').length).toBeGreaterThan(0)
  })

  it('updates validation result after selecting a stack in modal', () => {
    const validateMutate = vi.fn((stackId: string, options?: { onSuccess?: (result: unknown) => void }) => {
      expect(stackId).toBe('stack-1')
      options?.onSuccess?.({
        compatible: false,
        overall: { state: 'warn', score: 78 },
        issues: [{ tool: 'Kubernetes', message: 'Near supported range limit', severity: 'warning', code: 'K8S_NEAR_LIMIT' }],
        checkedAt: '2026-03-30T12:00:00Z',
      })
    })

    mockUseValidateCompatibility.mockReturnValue({ mutate: validateMutate })
    mockUseClusterK8sVersion.mockReturnValue({ mutate: vi.fn((clusterId: string, options?: { onSuccess?: (version: string) => void }) => {
      expect(clusterId).toBe('cluster-1')
      options?.onSuccess?.('v1.35.1')
    }) })
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          {
            id: 'stack-1',
            name: 'Team Platform Stack',
            templateName: 'GitLab + Argo CD',
            clusterId: 'cluster-1',
          },
        ],
        total: 1,
      },
      isLoading: false,
    })

    renderWithProviders(<StackVersionPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Validate Current Stack' }))
    fireEvent.click(screen.getByRole('button', { name: /Team Platform Stack/i }))

    expect(validateMutate).toHaveBeenCalledTimes(1)
    expect(screen.getByText(/Target stack|targetStack/)).toBeInTheDocument()
    expect(screen.getAllByText('Team Platform Stack').length).toBeGreaterThan(0)
    expect(screen.getByText(/Compatibility warnings found|validation.warn/)).toBeInTheDocument()
    expect(screen.getByText(/Score|validation.score/)).toHaveTextContent('78')
    expect(screen.getByText(/Kubernetes: Near supported range limit/)).toBeInTheDocument()
    expect(screen.getByText(/Checked at|validation.checkedAt/)).toBeInTheDocument()
    expect(screen.getByText(/v1\.35\.1/)).toBeInTheDocument()
  })
})

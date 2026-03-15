import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { ClusterPage } from './cluster-page'

const verifyMutate = vi.fn()

const MOCK_CLUSTERS = [
  {
    id: 'c1',
    name: 'prod-cluster',
    type: 'eks' as const,
    endpoint: 'https://prod.k8s.nullus.io',
    status: 'connected' as const,
    organizationIds: ['org-1'],
    createdAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'c2',
    name: 'staging-cluster',
    type: 'kubernetes' as const,
    endpoint: 'https://staging.k8s.nullus.io',
    status: 'connected' as const,
    organizationIds: ['org-1'],
    createdAt: '2026-01-15T00:00:00Z',
  },
  {
    id: 'c3',
    name: 'dev-cluster',
    type: 'k3s' as const,
    endpoint: 'https://dev.k8s.nullus.io',
    status: 'pending' as const,
    organizationIds: ['org-1'],
    createdAt: '2026-03-01T00:00:00Z',
  },
]

// Mock API hooks
vi.mock('../api/admin-api', () => ({
  useClusters: () => ({ data: { items: MOCK_CLUSTERS, total: MOCK_CLUSTERS.length }, isLoading: false }),
  useCreateCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useVerifyCluster: () => ({ mutate: verifyMutate }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ClusterPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Cluster Management')).toBeInTheDocument()
  })

  it('renders cluster list with mock clusters', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getAllByText('prod-cluster')[0]).toBeInTheDocument()
    expect(screen.getByText('staging-cluster')).toBeInTheDocument()
    expect(screen.getByText('dev-cluster')).toBeInTheDocument()
  })

  it('shows cluster count in list header', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Clusters (3)')).toBeInTheDocument()
  })

  it('renders cluster status badges', () => {
    renderWithProviders(<ClusterPage />)
    const connectedBadges = screen.getAllByText('Connected')
    expect(connectedBadges.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })

  it('shows detail panel for initially selected cluster', () => {
    renderWithProviders(<ClusterPage />)
    // prod-cluster is selected by default (first in list)
    expect(screen.getByText('https://prod.k8s.nullus.io')).toBeInTheDocument()
  })

  it('clicking a cluster in list shows its detail panel', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('dev-cluster'))
    expect(screen.getByText('https://dev.k8s.nullus.io')).toBeInTheDocument()
  })

  it('detail panel shows Organization Access section', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Organization Access')).toBeInTheDocument()
  })

  it('detail panel shows connection status', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('연결 상태')).toBeInTheDocument()
  })

  it('renders Register Cluster button', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Register Cluster')).toBeInTheDocument()
  })

  it('clicking Register Cluster opens modal', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    expect(screen.getByText('Register')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('예: prod-cluster')).toBeInTheDocument()
  })

  it('Register modal has cluster type select', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    expect(screen.getByText('클러스터 타입')).toBeInTheDocument()
  })

  it('closing register modal hides form', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByPlaceholderText('예: prod-cluster')).not.toBeInTheDocument()
  })

  it('calls verify cluster API for selected cluster', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Verify Connection'))
    expect(verifyMutate).toHaveBeenCalledWith('c1', expect.any(Object))
  })
})

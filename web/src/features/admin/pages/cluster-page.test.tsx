import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { ClusterPage } from './cluster-page'

const verifyMutate = vi.hoisted(() => vi.fn())

// Mock API hooks
vi.mock('../api/admin-api', () => ({
  useClusters: () => ({
    data: {
      items: [
        { id: 'c1', name: 'prod-cluster', type: 'target', types: ['target'], cloudProvider: 'aws', endpoint: 'https://prod.k8s.nullus.io', status: 'connected', organizationIds: ['org-1'], organizationNames: { 'org-1': 'Nullus Platform' }, createdAt: '2026-01-01T00:00:00Z' },
        { id: 'c2', name: 'staging-cluster', type: 'pipeline', types: ['pipeline'], cloudProvider: 'on_premise', endpoint: 'https://staging.k8s.nullus.io', status: 'connected', organizationIds: ['org-1'], createdAt: '2026-01-15T00:00:00Z' },
        { id: 'c3', name: 'dev-cluster', type: 'pipeline', types: ['target', 'pipeline'], cloudProvider: 'on_premise', endpoint: 'https://dev.k8s.nullus.io', status: 'pending', organizationIds: ['org-1'], createdAt: '2026-03-01T00:00:00Z' },
      ],
      total: 3,
    },
    isLoading: false,
  }),
  useCreateCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteCluster: () => ({ mutate: vi.fn(), isPending: false }),
  useVerifyCluster: () => ({ mutate: verifyMutate }),
  useVerifyClusterDraft: () => ({ mutate: vi.fn(), isPending: false }),
  useClusterMonitoringSummary: () => ({
    data: {
      total_nodes: 3,
      ready_nodes: 3,
      total_pods: 40,
      ready_pods: 40,
      cpu_request_millicores: 4000,
      cpu_limit_millicores: 8000,
      cpu_allocatable_millicores: 12000,
      cpu_usage_millicores: 1500,
      memory_request_mib: 8192,
      memory_limit_mib: 16384,
      memory_allocatable_mib: 24576,
      memory_usage_mib: 6144,
    },
    isLoading: false,
  }),
  useCluster: (id: string) => ({
    data: id
      ? {
        id,
        name: id === 'c1' ? 'prod-cluster' : 'staging-cluster',
        type: id === 'c1' ? 'target' : 'pipeline',
        types: id === 'c1' ? ['target'] : ['pipeline'],
        cloudProvider: id === 'c1' ? 'aws' : 'on_premise',
        endpoint: id === 'c1' ? 'https://prod.k8s.nullus.io' : 'https://staging.k8s.nullus.io',
        status: 'connected',
        organizationIds: ['org-1'],
        kubeconfig: 'apiVersion: v1\nkind: Config\nclusters: []',
        createdAt: '2026-01-01T00:00:00Z',
      }
      : undefined,
    isFetching: false,
  }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ClusterPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getAllByText('Cluster Management')[0]).toBeInTheDocument()
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

  // UUID 만 보여주면 어느 조직인지 알 수 없다. 서버가 이름을 주면 이름을 쓴다.
  it('shows the organization name instead of its id when the server resolved it', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Nullus Platform')).toBeInTheDocument()
    expect(screen.queryByText('org-1')).not.toBeInTheDocument()
  })

  // 스택을 올리기 전에 알아야 하는 것은 "지금 얼마가 남아 있는가" 다.
  it('shows how much cluster capacity is still free', () => {
    renderWithProviders(<ClusterPage />)

    expect(screen.getByText('Available Resources')).toBeInTheDocument()
    // allocatable 12 core - request 4 core = 8 core 남음
    expect(screen.getByText('8.0 of 12.0 cores free')).toBeInTheDocument()
    // allocatable 24GiB - request 8GiB = 16GiB 남음
    expect(screen.getByText('16.0 of 24.0 GiB free')).toBeInTheDocument()
    expect(screen.getByText('3/3 nodes ready')).toBeInTheDocument()
  })

  it('detail panel shows connection status', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Connection Status')).toBeInTheDocument()
  })

  it('renders Register Cluster button', () => {
    renderWithProviders(<ClusterPage />)
    expect(screen.getByText('Register Cluster')).toBeInTheDocument()
  })

  it('clicking Register Cluster opens modal', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    expect(screen.getByText('Register')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('e.g. prod-cluster')).toBeInTheDocument()
  })

  it('Register modal has cluster type select', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    expect(screen.getByText('Cluster Type')).toBeInTheDocument()
    expect(screen.getByText('DevSecOps Stack Cluster')).toBeInTheDocument()
  })

  it('closing register modal hides form', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Register Cluster'))
    fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByPlaceholderText('e.g. prod-cluster')).not.toBeInTheDocument()
  })

  it('calls verify cluster API for selected cluster', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Verify Connection'))
    expect(verifyMutate).toHaveBeenCalledWith('c1', expect.any(Object))
  })

  it('prefills existing kubeconfig when editing a cluster', () => {
    renderWithProviders(<ClusterPage />)
    fireEvent.click(screen.getByText('Edit'))
    const kubeconfigInput = screen.getByLabelText('kubeconfig (YAML)') as HTMLTextAreaElement
    expect(kubeconfigInput.value).toContain('apiVersion: v1')
  })
})

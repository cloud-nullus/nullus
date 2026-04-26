import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdListPage } from './cicd-list-page'

const mockNavigate = vi.fn()
const mockUsePipelines = vi.fn()
const mockUseDeletePipeline = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('../api/cicd-api', () => ({
  usePipelines: (...args: unknown[]) => mockUsePipelines(...args),
  useDeletePipeline: (...args: unknown[]) => mockUseDeletePipeline(...args),
}))

const pipelines = [
  {
    id: 'pipeline-1',
    name: 'frontend-web',
    appType: 'web-frontend',
    clusterId: 'c1',
    clusterName: 'prod-k8s',
    status: 'success',
    lastDeployedAt: '2026-03-03T14:28:00Z',
  },
]

describe('CicdListPage', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUsePipelines.mockReset()
    mockUseDeletePipeline.mockReset()
    mockUsePipelines.mockReturnValue({
      data: { items: pipelines, total: pipelines.length },
      isLoading: false,
    })
    mockUseDeletePipeline.mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    })
  })

  it('renders loading state safely', () => {
    mockUsePipelines.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<CicdListPage />)

    expect(screen.getAllByText('CI/CD List').length).toBeGreaterThan(0)
    expect(screen.getAllByText('No pipelines found.').length).toBeGreaterThan(0)
  })

  it('renders pipeline data', () => {
    renderWithProviders(<CicdListPage />)

    expect(screen.getAllByText('frontend-web').length).toBeGreaterThan(0)
    expect(screen.getAllByText('prod-k8s').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/success/i).length).toBeGreaterThan(0)
  })

  it('renders empty state when no pipelines returned', () => {
    mockUsePipelines.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
    })

    renderWithProviders(<CicdListPage />)

    expect(screen.getByText('No pipelines found.')).not.toBeNull()
  })

  it('navigates to templates page', () => {
    renderWithProviders(<CicdListPage />)

    fireEvent.click(screen.getByRole('button', { name: 'New Pipeline' }))
    expect(mockNavigate).toHaveBeenCalledWith('/cicd/templates')
  })
})

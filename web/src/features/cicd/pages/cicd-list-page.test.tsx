import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdListPage } from './cicd-list-page'

const mockNavigate = vi.fn()
const mockUsePipelines = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('../api/cicd-api', () => ({
  usePipelines: (...args: unknown[]) => mockUsePipelines(...args),
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
    mockUsePipelines.mockReturnValue({
      data: { items: pipelines, total: pipelines.length },
      isLoading: false,
    })
  })

  it('renders loading state safely', () => {
    mockUsePipelines.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<CicdListPage />)

    expect(screen.getAllByText('CI/CD List').length).toBeGreaterThan(0)
    expect(screen.getAllByText('파이프라인이 없습니다.').length).toBeGreaterThan(0)
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

    expect(screen.getByText('파이프라인이 없습니다.')).not.toBeNull()
  })

  it('navigates to templates and deploy pages', () => {
    renderWithProviders(<CicdListPage />)

    fireEvent.click(screen.getByRole('button', { name: 'New Pipeline' }))
    expect(mockNavigate).toHaveBeenCalledWith('/cicd/templates')

    fireEvent.click(screen.getByRole('button', { name: 'Deploy' }))
    expect(mockNavigate).toHaveBeenCalledWith('/cicd/developer-deploy?pipeline=pipeline-1')
  })
})

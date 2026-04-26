import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdPipelineSetupPage } from './cicd-pipeline-setup-page'

const mockNavigate = vi.fn()
const mockUseSearchParams = vi.fn()
const mockUseCicdTemplates = vi.fn()
const mockUseClusters = vi.fn()
const mockCreatePipelineMutate = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useSearchParams: () => mockUseSearchParams(),
  }
})

vi.mock('../../../components/shared/yaml-editor', () => ({
  YamlEditor: ({ value }: { value: string }) => <pre>{value}</pre>,
}))

vi.mock('../api/cicd-api', () => ({
  useCicdTemplates: () => mockUseCicdTemplates(),
  useCreatePipeline: () => ({ mutate: mockCreatePipelineMutate, isPending: false }),
}))

vi.mock('../../admin/api/admin-api', () => ({
  useClusters: () => mockUseClusters(),
}))

const templates = [
  {
    id: 'web-backend',
    name: 'Backend API',
    description: 'REST API 백엔드 서비스 템플릿',
    appType: 'web-backend',
    stages: ['Build', 'Test', 'Deploy'],
    createdBy: 'admin',
  },
]

const clusters = {
  items: [
    { id: 'c1', name: 'prod-k8s', type: 'target', types: ['target'] },
    { id: 'c2', name: 'nullus-develop', type: 'pipeline', types: [' TARGET '] },
    { id: 'c3', name: 'pipeline-only-k8s', type: 'pipeline', types: ['pipeline'] },
  ],
  total: 3,
}

describe('CicdPipelineSetupPage', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseSearchParams.mockReset()
    mockUseCicdTemplates.mockReset()
    mockUseClusters.mockReset()
    mockCreatePipelineMutate.mockReset()

    mockUseSearchParams.mockReturnValue([new URLSearchParams('template=web-backend')])
    mockUseCicdTemplates.mockReturnValue({ data: templates, isLoading: false })
    mockUseClusters.mockReturnValue({ data: clusters, isLoading: false })
  })

  it('renders loading state safely', () => {
    mockUseCicdTemplates.mockReturnValue({ data: undefined, isLoading: true })
    mockUseClusters.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<CicdPipelineSetupPage />)

    expect(screen.getByText('CI/CD Pipeline Setup')).not.toBeNull()
    expect(screen.getByText('Setup Summary')).not.toBeNull()
  })

  it('renders template and cluster data', () => {
    renderWithProviders(<CicdPipelineSetupPage />)

    expect(screen.getAllByText('Backend API').length).toBeGreaterThan(0)
    expect(screen.getAllByText('prod-k8s').length).toBeGreaterThan(0)
    expect(screen.getAllByText('nullus-develop').length).toBeGreaterThan(0)
    expect(screen.queryByText('pipeline-only-k8s')).toBeNull()
  })

  it('uses fallback options when API returns empty arrays', () => {
    mockUseCicdTemplates.mockReturnValue({ data: [], isLoading: false })
    mockUseClusters.mockReturnValue({ data: { items: [], total: 0 }, isLoading: false })

    renderWithProviders(<CicdPipelineSetupPage />)

    expect(screen.getAllByText('Web Frontend').length).toBeGreaterThan(0)
    expect(screen.getAllByText('prod-k8s').length).toBeGreaterThan(0)
  })

  it('navigates on change template and create pipeline actions', () => {
    mockCreatePipelineMutate.mockImplementation((_, options: { onSuccess?: () => void }) => {
      options.onSuccess?.()
    })

    renderWithProviders(<CicdPipelineSetupPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Change Template' }))
    expect(mockNavigate).toHaveBeenCalledWith('/cicd/templates')

    fireEvent.click(screen.getByRole('button', { name: 'Create Pipeline' }))
    expect(mockCreatePipelineMutate).toHaveBeenCalled()
    expect(mockNavigate).toHaveBeenCalledWith('/cicd/list')
  })
})

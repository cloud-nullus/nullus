import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdTemplatePage } from './cicd-template-page'

const mockNavigate = vi.fn()
const mockUseAuthStore = vi.fn()
const mockUseCicdTemplates = vi.fn()
const mockCreateMutate = vi.fn()
const mockUpdateMutate = vi.fn()
const mockDeleteMutate = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: (selector: (state: { role: string }) => unknown) =>
    mockUseAuthStore(selector),
}))

vi.mock('../api/cicd-api', () => ({
  useCicdTemplates: () => mockUseCicdTemplates(),
  useCreateCicdTemplate: () => ({ mutate: mockCreateMutate, isPending: false }),
  useUpdateCicdTemplate: () => ({ mutate: mockUpdateMutate, isPending: false }),
  useDeleteCicdTemplate: () => ({ mutate: mockDeleteMutate, isPending: false }),
}))

const templates = [
  {
    id: 'web-backend-standard',
    name: 'Standard Web Backend',
    description: 'REST API 백엔드 서비스 템플릿',
    appType: 'web-backend',
    stages: ['Production', 'QA'],
    createdBy: 'admin',
  },
]

describe('CicdTemplatePage', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseCicdTemplates.mockReset()
    mockCreateMutate.mockReset()
    mockUpdateMutate.mockReset()
    mockDeleteMutate.mockReset()

    mockUseAuthStore.mockImplementation(
      (selector: (state: { role: string }) => unknown) => selector({ role: 'admin' })
    )
    mockUseCicdTemplates.mockReturnValue({ data: templates, isLoading: false })
  })

  it('renders loading state safely', () => {
    mockUseCicdTemplates.mockReturnValue({ data: undefined, isLoading: true })

    renderWithProviders(<CicdTemplatePage />)

    expect(screen.getAllByText('CI/CD Template').length).toBeGreaterThan(0)
    expect(screen.getByText('검색 결과가 없습니다.')).not.toBeNull()
  })

  it('renders template cards from API data', () => {
    renderWithProviders(<CicdTemplatePage />)

    expect(screen.getByText('Standard Web Backend')).not.toBeNull()
    expect(screen.getByText('REST API 백엔드 서비스 템플릿')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Create Template' })).not.toBeNull()
  })

  it('renders empty state when template list is empty', () => {
    mockUseCicdTemplates.mockReturnValue({ data: [], isLoading: false })

    renderWithProviders(<CicdTemplatePage />)

    expect(screen.getByText('검색 결과가 없습니다.')).not.toBeNull()
  })

  it('navigates to developer deploy when using base template', () => {
    renderWithProviders(<CicdTemplatePage />)

    fireEvent.click(screen.getByRole('button', { name: 'Use Base Template' }))

    expect(mockNavigate).toHaveBeenCalledWith('/cicd/developer-deploy')
  })
})

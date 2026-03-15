import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackTemplatePage } from './stack-template-page'
import { useStackConfigStore } from '../stores/stack-config-store'
import { useAuthStore } from '../../../stores/auth-store'

const mockCreateTemplateMutate = vi.fn()
const mockUpdateTemplateMutate = vi.fn()
const mockDeleteTemplateMutate = vi.fn()

const mockTemplates = [
  {
    id: 'gitlab-allinone-v1',
    name: 'GitLab All-in-One',
    description: 'GitLab 올인원 스택',
    tools: ['GitLab', 'GitLab CI', 'Argo CD'],
    estimatedMinutes: 25,
    category: 'gitlab',
  },
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + ArgoCD',
    description: 'GitLab + ArgoCD 스택',
    tools: ['GitLab', 'Argo CD'],
    estimatedMinutes: 30,
    category: 'hybrid',
  },
  {
    id: 'github-argocd-v1',
    name: 'GitHub + ArgoCD',
    description: 'GitHub + ArgoCD 스택',
    tools: ['GitHub', 'Argo CD'],
    estimatedMinutes: 20,
    category: 'github',
  },
]

vi.mock('../api/stack-api', () => ({
  useTemplates: () => ({ data: mockTemplates }),
  useCreateTemplate: () => ({ mutate: mockCreateTemplateMutate, isPending: false }),
  useUpdateTemplate: () => ({ mutate: mockUpdateTemplateMutate, isPending: false }),
  useDeleteTemplate: () => ({ mutate: mockDeleteTemplateMutate, isPending: false }),
}))

// Mock useNavigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

beforeEach(() => {
  useStackConfigStore.getState().resetConfig()
  useAuthStore.setState({ role: 'developer', user: null, token: null, isAuthenticated: false })
  mockNavigate.mockClear()
  mockCreateTemplateMutate.mockReset()
  mockUpdateTemplateMutate.mockReset()
  mockDeleteTemplateMutate.mockReset()
})

describe('StackTemplatePage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByText('Golden Path Templates')).toBeInTheDocument()
  })

  it('renders 3 Golden Path template cards', () => {
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByText('GitLab All-in-One')).toBeInTheDocument()
    expect(screen.getByText('GitLab + ArgoCD')).toBeInTheDocument()
    expect(screen.getByText('GitHub + ArgoCD')).toBeInTheDocument()
  })

  it('renders 3 Use Template buttons', () => {
    renderWithProviders(<StackTemplatePage />)
    const buttons = screen.getAllByText('Use Template')
    expect(buttons).toHaveLength(3)
  })

  it('renders search input', () => {
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByPlaceholderText('템플릿 검색...')).toBeInTheDocument()
  })

  it('filters cards when text is entered in search', async () => {
    renderWithProviders(<StackTemplatePage />)
    const searchInput = screen.getByPlaceholderText('템플릿 검색...')
    fireEvent.change(searchInput, { target: { value: 'GitLab All' } })
    await waitFor(() => {
      expect(screen.getByText('GitLab All-in-One')).toBeInTheDocument()
      expect(screen.queryByText('GitLab + ArgoCD')).not.toBeInTheDocument()
      expect(screen.queryByText('GitHub + ArgoCD')).not.toBeInTheDocument()
    })
  })

  it('shows no results message when search yields nothing', async () => {
    renderWithProviders(<StackTemplatePage />)
    const searchInput = screen.getByPlaceholderText('템플릿 검색...')
    fireEvent.change(searchInput, { target: { value: 'xyznonexistent' } })
    await waitFor(() => {
      expect(screen.getByText('검색 결과가 없습니다.')).toBeInTheDocument()
    })
  })

  it('filters by tool name in search', async () => {
    renderWithProviders(<StackTemplatePage />)
    const searchInput = screen.getByPlaceholderText('템플릿 검색...')
    fireEvent.change(searchInput, { target: { value: 'ArgoCD' } })
    await waitFor(() => {
      expect(screen.getByText('GitLab + ArgoCD')).toBeInTheDocument()
      expect(screen.getByText('GitHub + ArgoCD')).toBeInTheDocument()
      expect(screen.queryByText('GitLab All-in-One')).not.toBeInTheDocument()
    })
  })

  it('clicking Use Template navigates to /stack/install', () => {
    renderWithProviders(<StackTemplatePage />)
    const buttons = screen.getAllByText('Use Template')
    fireEvent.click(buttons[0])
    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitlab-allinone-v1')
  })

  it('clicking Use Template sets template in store', () => {
    renderWithProviders(<StackTemplatePage />)
    const buttons = screen.getAllByText('Use Template')
    fireEvent.click(buttons[0])
    const { draft } = useStackConfigStore.getState()
    expect(draft.selectedTemplateId).toBe('gitlab-allinone-v1')
  })

  it('shows create template button only for admin role', () => {
    const first = renderWithProviders(<StackTemplatePage />)
    expect(screen.queryByRole('button', { name: 'Create Template' })).not.toBeInTheDocument()

    first.unmount()

    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByRole('button', { name: 'Create Template' })).toBeInTheDocument()
  })

  it('admin can create a template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(screen.getByRole('button', { name: 'Create Template' }))
    fireEvent.change(screen.getByLabelText('Template ID'), { target: { value: 'custom-template-v1' } })
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Custom Template' } })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Custom description' } })
    fireEvent.change(screen.getByLabelText('Tools (JSON)'), {
      target: { value: '[{"category":"cd_tool","name":"Argo CD","helm_version":"7.7.2","app_version":"2.13.2"}]' },
    })
    fireEvent.change(screen.getByLabelText('Estimated Install Time (ns)'), { target: { value: '1800000000000' } })
    fireEvent.change(screen.getByLabelText('Recommended Use Case'), { target: { value: '테스트' } })
    fireEvent.change(screen.getByLabelText('Minimum Resources'), { target: { value: '2 vCPU / 4Gi RAM / 20Gi Storage' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => {
      expect(mockCreateTemplateMutate).toHaveBeenCalled()
    })
  })

  it('admin can update a template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0])
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'GitLab All-in-One Updated' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateTemplateMutate).toHaveBeenCalled()
    })
  })

  it('admin can delete a template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    fireEvent.click(screen.getByRole('button', { name: 'Delete Template' }))

    await waitFor(() => {
      expect(mockDeleteTemplateMutate).toHaveBeenCalled()
    })
  })
})

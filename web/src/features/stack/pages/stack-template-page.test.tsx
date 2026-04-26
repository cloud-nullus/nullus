import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent, waitFor, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackTemplatePage } from './stack-template-page'
import { useStackConfigStore } from '../stores/stack-config-store'
import { useAuthStore } from '../../../stores/auth-store'

const mockCreateTemplateMutate = vi.fn()
const mockUpdateTemplateMutate = vi.fn()
const mockDeleteTemplateMutate = vi.fn()
const SEARCH_PLACEHOLDER = /Search templates\.\.\.|템플릿 검색\.\.\./
const EMPTY_RESULT_MESSAGE = /No templates found\.|No search results found\.|검색 결과가 없습니다\./

const mockTemplates = [
  {
    id: 'empty-template-v1',
    name: 'Empty Template',
    description: 'Empty stack template',
    tools: [],
    estimatedMinutes: 5,
    category: 'blank',
  },
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
    toolDetails: [
      {
        category: 'source_repository',
        name: 'GitLab CE',
        helm_version: '9.5.1',
        app_version: '18.5.1',
      },
      {
        category: 'cd_tool',
        name: 'Argo CD',
        helm_version: '6.8.0',
        app_version: 'v2.8.3',
      },
    ],
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
    expect(screen.getAllByText('Stack Template')[0]).toBeInTheDocument()
  })

  it('renders 4 Golden Path template cards', () => {
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByText('Empty Template')).toBeInTheDocument()
    expect(screen.getByText('GitLab All-in-One')).toBeInTheDocument()
    expect(screen.getByText('GitLab + ArgoCD')).toBeInTheDocument()
    expect(screen.getByText('GitHub + ArgoCD')).toBeInTheDocument()
  })

  it('renders 4 Use Base Template buttons', () => {
    renderWithProviders(<StackTemplatePage />)
    const buttons = screen.getAllByText('Use As Base')
    expect(buttons).toHaveLength(4)
  })


  it('prefers persisted custom description over seeded localized copy', () => {
    const original = mockTemplates[1].description
    mockTemplates[1].description = 'Custom persisted description from API'

    renderWithProviders(<StackTemplatePage />)

    expect(screen.getByText('Custom persisted description from API')).toBeInTheDocument()

    mockTemplates[1].description = original
  })

  it('renders search input', () => {
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByPlaceholderText(SEARCH_PLACEHOLDER)).toBeInTheDocument()
  })

  it('filters cards when text is entered in search', async () => {
    renderWithProviders(<StackTemplatePage />)
    const searchInput = screen.getByPlaceholderText(SEARCH_PLACEHOLDER)
    fireEvent.change(searchInput, { target: { value: 'GitLab All' } })
    await waitFor(() => {
      expect(screen.getByText('GitLab All-in-One')).toBeInTheDocument()
      expect(screen.queryByText('GitLab + ArgoCD')).not.toBeInTheDocument()
      expect(screen.queryByText('GitHub + ArgoCD')).not.toBeInTheDocument()
    })
  })

  it('shows no results message when search yields nothing', async () => {
    renderWithProviders(<StackTemplatePage />)
    const searchInput = screen.getByPlaceholderText(SEARCH_PLACEHOLDER)
    fireEvent.change(searchInput, { target: { value: 'xyznonexistent' } })
    await waitFor(() => {
      expect(screen.getByText(EMPTY_RESULT_MESSAGE)).toBeInTheDocument()
    })
  })

  it('filters by tool name in search', async () => {
    renderWithProviders(<StackTemplatePage />)
    const searchInput = screen.getByPlaceholderText(SEARCH_PLACEHOLDER)
    fireEvent.change(searchInput, { target: { value: 'ArgoCD' } })
    await waitFor(() => {
      expect(screen.getByText('GitLab + ArgoCD')).toBeInTheDocument()
      expect(screen.getByText('GitHub + ArgoCD')).toBeInTheDocument()
      expect(screen.queryByText('GitLab All-in-One')).not.toBeInTheDocument()
    })
  })

  it('clicking Use Base Template navigates to /stack/install', () => {
    renderWithProviders(<StackTemplatePage />)
    const templateCard = screen.getByText('GitLab All-in-One').closest('[role="button"]')
    expect(templateCard).toBeTruthy()
    fireEvent.click(within(templateCard as HTMLElement).getByText('Use As Base'))
    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitlab-allinone-v1')
  })

  it('clicking Use Base Template sets template in store and clears unselected slots', () => {
    renderWithProviders(<StackTemplatePage />)
    const templateCard = screen.getByText('GitLab All-in-One').closest('[role="button"]')
    expect(templateCard).toBeTruthy()
    fireEvent.click(within(templateCard as HTMLElement).getByText('Use As Base'))
    const { draft } = useStackConfigStore.getState()
    expect(draft.selectedTemplateId).toBe('gitlab-allinone-v1')
    expect(draft.logging.search.tool).toBe('')
    expect(draft.logging.traceLayer.tool).toBe('')
  })

  it('clicking Empty Template clears install selections in store', () => {
    renderWithProviders(<StackTemplatePage />)
    const templateCard = screen.getByText('Empty Template').closest('[role="button"]')
    expect(templateCard).toBeTruthy()

    fireEvent.click(within(templateCard as HTMLElement).getByText('Use As Base'))

    const { draft } = useStackConfigStore.getState()
    expect(draft.selectedTemplateId).toBe('empty-template-v1')
    expect(draft.artifacts.packageRegistry.tool).toBe('')
    expect(draft.pipeline.cicdPlatform.tool).toBe('')
    expect(draft.storage.planMode).toBe('none')
  })

  it('shows create template button only for admin role', () => {
    const first = renderWithProviders(<StackTemplatePage />)
    expect(screen.queryByRole('button', { name: 'Create Template' })).not.toBeInTheDocument()

    first.unmount()

    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByRole('button', { name: 'Create Template' })).toBeInTheDocument()
  })

  it('shows full Artifact, CI/CD, and Observability categories in create modal tool picker', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(screen.getByRole('button', { name: 'Create Template' }))

    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getByText('CI/CD')).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()

    expect(screen.getByText('Package Registry')).toBeInTheDocument()
    expect(screen.getByText('Source Repository')).toBeInTheDocument()
    expect(screen.getByText('Container Registry')).toBeInTheDocument()
    expect(screen.getByText('CI Platform')).toBeInTheDocument()
    expect(screen.getByText('CD Tool')).toBeInTheDocument()
    expect(screen.getByText('Metrics')).toBeInTheDocument()
    expect(screen.getByText('Visualization')).toBeInTheDocument()
    expect(screen.getByText('Logs')).toBeInTheDocument()
    expect(screen.getByText('Agent')).toBeInTheDocument()
    expect(screen.getByText('Traces')).toBeInTheDocument()


    expect(screen.getAllByRole('button', { name: 'Add Tool' }).length).toBeGreaterThanOrEqual(3)
  })

  it('admin can create a template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(screen.getByRole('button', { name: 'Create Template' }))
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Custom Template' } })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Custom description' } })
    fireEvent.change(screen.getByLabelText('Recommended Use Case'), { target: { value: '테스트' } })
    fireEvent.change(screen.getByLabelText('Minimum Resources'), { target: { value: '2 vCPU / 4Gi RAM / 20Gi Storage' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => {
      expect(mockCreateTemplateMutate).toHaveBeenCalled()
    })

    expect(mockCreateTemplateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: expect.stringMatching(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i),
        estimated_install_time: 600000000000,
      }),
      expect.any(Object)
    )
  })

  it('admin can update a template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    const templateCard = screen.getByText('GitLab All-in-One').closest('[role="button"]')
    expect(templateCard).toBeTruthy()
    fireEvent.click(within(templateCard as HTMLElement).getByRole('button', { name: 'Edit' }))
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'GitLab All-in-One Updated' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateTemplateMutate).toHaveBeenCalled()
    })
  })

  it('preserves tool version metadata when editing gitlab+argocd template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    const templateCard = screen.getByText('GitLab + ArgoCD').closest('[role="button"]')
    expect(templateCard).toBeTruthy()
    fireEvent.click(within(templateCard as HTMLElement).getByRole('button', { name: 'Edit' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateTemplateMutate).toHaveBeenCalled()
    })

    const firstCall = mockUpdateTemplateMutate.mock.calls[0]?.[0] as { tools?: Array<Record<string, string>> }
    expect(firstCall.tools).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: 'GitLab CE', helm_version: '9.5.1', app_version: '18.5.1' }),
        expect.objectContaining({ name: 'Argo CD', helm_version: '6.8.0', app_version: 'v2.8.3' }),
      ])
    )
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

  it('renders duplicate buttons for admin and duplicates template with localized suffix', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    const duplicateButtons = screen.getAllByRole('button', { name: 'Duplicate' })
    expect(duplicateButtons).toHaveLength(4)

    const templateCard = screen.getByText('GitLab All-in-One').closest('[role="button"]')
    expect(templateCard).toBeTruthy()
    fireEvent.click(within(templateCard as HTMLElement).getByRole('button', { name: 'Duplicate' }))

    await waitFor(() => {
      expect(mockCreateTemplateMutate).toHaveBeenCalled()
    })

    expect(mockCreateTemplateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'gitlab-allinone-v1-copy',
        name: 'GitLab All-in-One Duplicate',
      }),
      expect.any(Object)
    )
  })
})

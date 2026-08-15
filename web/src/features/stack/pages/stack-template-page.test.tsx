import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent, waitFor, within } from '@testing-library/react'
import { renderWithProviders, selectOptionByValue } from '../../../__tests__/test-utils'
import { StackTemplatePage } from './stack-template-page'
import { useStackConfigStore } from '../stores/stack-config-store'
import { useAuthStore } from '../../../stores/auth-store'

const mockCreateTemplateMutate = vi.fn()
const mockUpdateTemplateMutate = vi.fn()
const mockDeleteTemplateMutate = vi.fn()
const SEARCH_PLACEHOLDER = /Search templates\.\.\.|템플릿 검색\.\.\./
const EMPTY_RESULT_MESSAGE = /No templates found\.|No search results found\.|검색 결과가 없습니다\./

const getTemplateCard = (title: string) => {
  const heading = screen.getByRole('heading', { name: title })
  return heading.closest('div')?.parentElement as HTMLElement
}

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
    id: 'gitea-jenkins-argocd-lite-v1',
    name: 'Lite Template',
    description: '8Gi 노드 하나에 올라가는 최소 구성',
    tools: ['Gitea', 'Jenkins', 'Argo CD'],
    estimatedMinutes: 40,
    category: 'lite',
    planningProfile: 'local',
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

  it('labels template versions as matrix snapshot values', async () => {
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(screen.getAllByRole('button', { name: 'View Detail' })[2])

    const dialog = await screen.findByRole('dialog', { name: 'GitLab + ArgoCD' })

    expect(
      // 이 단정은 원래 키 문자열 자체를 찾고 있었다 — 키가 en.json 에 없어서
      // 테스트의 t() 목이 키를 그대로 돌려줬기 때문이다. 즉 번역 누락 버그를
      // 고정하고 있었다. 키를 채웠으니 실제 문구를 확인한다.
      within(dialog).getByText(
        'These versions reflect the validated matrix snapshot used for template guidance.',
      ),
    ).toBeInTheDocument()
    expect(within(dialog).getByText('Matrix Helm: 9.5.1')).toBeInTheDocument()
    expect(within(dialog).getByText('Matrix App: 18.5.1')).toBeInTheDocument()
  })

  it('renders one detail button per template', () => {
    renderWithProviders(<StackTemplatePage />)
    const buttons = screen.getAllByRole('button', { name: 'View Detail' })
    // 개수를 상수로 박으면 카탈로그에 하나 추가할 때마다 무관하게 깨진다.
    expect(buttons).toHaveLength(mockTemplates.length)
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
    fireEvent.click(within(getTemplateCard('GitLab All-in-One')).getByRole('button', { name: 'View Detail' }))
    fireEvent.click(screen.getByRole('button', { name: 'Use As Base' }))
    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitlab-allinone-v1')
  })

  it('clicking Use Base Template sets template in store and clears unselected slots', () => {
    renderWithProviders(<StackTemplatePage />)
    fireEvent.click(within(getTemplateCard('GitLab All-in-One')).getByRole('button', { name: 'View Detail' }))
    fireEvent.click(screen.getByRole('button', { name: 'Use As Base' }))
    const { draft } = useStackConfigStore.getState()
    expect(draft.selectedTemplateId).toBe('gitlab-allinone-v1')
    expect(draft.logging.search.tool).toBe('')
    expect(draft.logging.traceLayer.tool).toBe('')
    expect(draft.logging.traceExporter.tool).toBe('')
  })

  it('clicking Empty Template clears install selections in store', () => {
    renderWithProviders(<StackTemplatePage />)
    fireEvent.click(within(getTemplateCard('Empty Template')).getByRole('button', { name: 'View Detail' }))
    fireEvent.click(screen.getByRole('button', { name: 'Use As Base' }))

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

    // 섹션은 탭으로 갈린다. 한 번에 한 탭만 그려지므로 탭을 옮겨 가며 본다 —
    // 검사하려는 것은 "모든 카테고리를 고를 수 있는가" 이지 "한 화면에 다 있는가"
    // 가 아니다.
    const SECTION_CATEGORIES: Record<string, string[]> = {
      Artifacts: ['Package Registry', 'Source Repository', 'Container Registry'],
      'CI/CD': ['CI Platform', 'CD Tool'],
      Observability: ['Metrics', 'Visualization', 'Logs', 'Agent', 'Traces'],
    }

    for (const [section, categories] of Object.entries(SECTION_CATEGORIES)) {
      fireEvent.click(screen.getByRole('button', { name: section }))
      for (const category of categories) {
        expect(screen.getByText(category), `${section} 탭에 ${category} 가 없다`).toBeInTheDocument()
      }
    }

    // Artifacts 로 돌아와 카테고리마다 적용 버튼이 하나씩 있는지 본다.
    fireEvent.click(screen.getByRole('button', { name: 'Artifacts' }))
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

  it('shows the tabbed tool editor when editing a template that has no tools', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(within(getTemplateCard('Empty Template')).getByRole('button', { name: 'Edit' }))

    // 도구가 없다고 편집기가 사라지면 안 된다 — 만들기 모달과 같은 탭이 떠야
    // 관리자가 빈 템플릿에 도구를 채워 넣을 수 있다.
    const dialog = await screen.findByRole('dialog', { name: 'Edit Template' })
    for (const section of ['Artifacts', 'CI/CD', 'Observability']) {
      expect(
        within(dialog).getByRole('button', { name: section }),
        `편집 모달에 ${section} 탭이 없다`,
      ).toBeInTheDocument()
    }
    expect(within(dialog).getAllByRole('button', { name: 'Add Tool' }).length).toBeGreaterThanOrEqual(3)
  })

  it('keeps a section visible after its last tool is removed while editing', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(within(getTemplateCard('GitLab + ArgoCD')).getByRole('button', { name: 'Edit' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edit Template' })

    // CD Tool 은 CI/CD 섹션의 유일한 도구다. 지웠다고 탭까지 사라지면 방금 지운
    // 것을 되돌릴 방법이 없다.
    fireEvent.click(within(dialog).getByRole('button', { name: 'CI/CD' }))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove CD Tool' }))

    expect(within(dialog).getByRole('button', { name: 'CI/CD' })).toBeInTheDocument()
    expect(within(dialog).getByText('CD Tool')).toBeInTheDocument()
  })

  it('admin can update a template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(within(getTemplateCard('GitLab All-in-One')).getByRole('button', { name: 'Edit' }))
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'GitLab All-in-One Updated' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateTemplateMutate).toHaveBeenCalled()
    })
  })

  it('preserves tool version metadata when editing gitlab+argocd template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(within(getTemplateCard('GitLab + ArgoCD')).getByRole('button', { name: 'Edit' }))
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



  it('edits and saves the planning profile that sizes the stack', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    fireEvent.click(within(getTemplateCard('GitLab + ArgoCD')).getByRole('button', { name: 'Edit' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edit Template' })

    // 템플릿이 프로파일을 들고 다니지 않으면 8Gi 를 노린 조합도 standard 로
    // 설치돼 두 배 크기가 된다.
    const profileSelect = within(dialog).getByLabelText('Sizing Profile')
    selectOptionByValue(profileSelect, 'local')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateTemplateMutate).toHaveBeenCalled()
    })
    const [payload] = mockUpdateTemplateMutate.mock.calls[0] as [{ planning_profile?: string }]
    expect(payload.planning_profile).toBe('local')
  })

  it('keeps the stored planning profile when editing an untouched template', async () => {
    useAuthStore.setState({ role: 'admin', user: null, token: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    // 이름만 고치고 저장해도 프로파일이 기본값으로 되돌아가면 안 된다.
    fireEvent.click(within(getTemplateCard('Lite Template')).getByRole('button', { name: 'Edit' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateTemplateMutate).toHaveBeenCalled()
    })
    const [payload] = mockUpdateTemplateMutate.mock.calls[0] as [{ planning_profile?: string }]
    expect(payload.planning_profile).toBe('local')
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

    fireEvent.click(within(getTemplateCard('GitLab All-in-One')).getByRole('button', { name: 'View Detail' }))
    expect(screen.getByRole('button', { name: 'Duplicate' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Duplicate' }))

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

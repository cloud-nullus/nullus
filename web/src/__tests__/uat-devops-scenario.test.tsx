/**
 * UAT-1: DevOps Engineer "미정" scenario
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent, within } from '@testing-library/react'
import { renderWithProviders } from './test-utils'
import { LoginPage } from '../features/auth/pages/login-page'
import { HomePage } from '../features/home/pages/home-page'
import { StackTemplatePage } from '../features/stack/pages/stack-template-page'
import { StackInstallPage } from '../features/stack/pages/stack-install-page'
import { Sidebar } from '../components/layout/sidebar'
import { useAuthStore } from '../stores/auth-store'
import { useStackConfigStore } from '../features/stack/stores/stack-config-store'
import { useSidebarStore } from '../stores/sidebar-store'

const MOCK_TEMPLATES = [
  { id: 'empty-template-v1', name: 'Empty Template', description: 'Blank stack template.', tools: [], estimatedMinutes: 5, category: 'blank', createdBy: 'admin', recommendedUseCase: 'Custom', minResources: 'Decide later' },
  { id: 'gitlab-allinone-v1', name: 'GitLab All-in-One', description: 'GitLab CE 기반 단일 플랫폼.', tools: ['GitLab CE', 'GitLab CI'], estimatedMinutes: 90, category: 'gitlab', createdBy: 'admin', recommendedUseCase: '중견기업', minResources: '8 vCPU' },
  { id: 'gitlab-argocd-v1', name: 'GitLab + Argo CD', description: 'GitOps 패턴 구성.', tools: ['GitLab CE', 'Argo CD'], estimatedMinutes: 120, category: 'gitlab', createdBy: 'admin', recommendedUseCase: 'GitOps', minResources: '10 vCPU' },
  { id: 'github-argocd-v1', name: 'GitHub + Argo CD', description: 'GitHub Actions 사용.', tools: ['GitHub', 'Argo CD'], estimatedMinutes: 60, category: 'github', createdBy: 'admin', recommendedUseCase: 'GitHub', minResources: '6 vCPU' },
]

vi.mock('../features/stack/api/stack-api', () => ({
  useTemplates: () => ({ data: MOCK_TEMPLATES }),
  useCreateStack: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useSaveDraft: () => ({ mutate: vi.fn(), isPending: false }),
  useEstimateResources: () => ({ mutate: vi.fn(), isPending: false, data: undefined }),
  useClusters: () => ({ data: [{ id: 'cluster-1', name: 'dev-cluster', connection_status: 'connected' }] }),
  useResourceDefaults: () => ({ data: { items: [], total: 0 } }),
  useDeployStack: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../features/admin/api/admin-api', () => ({
  useClusterNamespaces: () => ({ data: [] }),
}))

vi.mock('../components/shared/yaml-editor', () => ({
  YamlEditor: ({ value }: { value: string }) => <pre data-testid="yaml-editor">{value}</pre>,
}))

vi.mock('@monaco-editor/react', () => ({
  default: ({ value }: { value: string }) => <pre data-testid="yaml-editor">{value}</pre>,
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

beforeEach(() => {
  useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: false })
  useStackConfigStore.getState().resetConfig()
  useSidebarStore.setState({ collapsed: false })
  mockNavigate.mockClear()
})

describe('UAT-1: DevOps Engineer scenario', () => {
  it('step 1: login page renders and devops can log in', async () => {
    renderWithProviders(<LoginPage />)
    expect(screen.getByText('Nullus Platform')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'devops@nullus.dev' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'devops123' } })
    fireEvent.submit(screen.getByText('Sign in').closest('form')!)

    await vi.waitFor(() => {
      expect(useAuthStore.getState().isAuthenticated).toBe(true)
    })
    expect(useAuthStore.getState().role).toBe('devops')
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('step 2: home page shows CTA buttons for devops', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<HomePage />)
    expect(screen.getByText('Stack 시작하기')).toBeInTheDocument()
    expect(screen.getByText('CI/CD 파이프라인')).toBeInTheDocument()
  })

  it('step 3: stack template page shows 4 cards', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByText('Empty Template')).toBeInTheDocument()
    expect(screen.getByText('GitLab All-in-One')).toBeInTheDocument()
    expect(screen.getByText('GitLab + Argo CD')).toBeInTheDocument()
    expect(screen.getByText('GitHub + Argo CD')).toBeInTheDocument()
  })

  it('step 4: selecting GitLab All-in-One navigates to install page', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    const templateCard = screen.getByText('GitLab All-in-One').closest('[role="button"]')
    expect(templateCard).toBeTruthy()
    fireEvent.click(within(templateCard as HTMLElement).getByText('Use Base Template'))

    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitlab-allinone-v1')
    expect(useStackConfigStore.getState().draft.selectedTemplateId).toBe('gitlab-allinone-v1')
  })

  it('step 5: install page tab traversal - workflow tabs visible', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackInstallPage />)

    // Verify all tabs are present
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getAllByText('CI/CD')[0]).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Resources' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Storage' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'YAML View' })).toBeInTheDocument()

    // Traverse through tabs
    fireEvent.click(screen.getAllByText('CI/CD')[0])
    expect(screen.getAllByText('CI/CD Platform')[0]).toBeInTheDocument()

    fireEvent.click(screen.getByText('Observability'))
    expect(screen.getAllByText('Metrics')[0]).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Resources' }))
    expect(screen.getByText('OSS별 Resource Planning')).toBeInTheDocument()
  })

  it('step 6: YAML View tab shows configuration after required selections', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackInstallPage />)
    fireEvent.change(screen.getByLabelText('Target Cluster'), { target: { value: 'cluster-1' } })
    fireEvent.change(screen.getByLabelText('Namespace'), { target: { value: '__new__' } })
    fireEvent.change(screen.getByPlaceholderText('my-namespace'), { target: { value: 'uat' } })
    fireEvent.click(screen.getByText('YAML View'))
    expect(screen.getByTestId('yaml-editor')).toBeInTheDocument()
    const yaml = screen.getByTestId('yaml-editor').textContent ?? ''
    expect(yaml).toContain('global:')
    expect(yaml).toContain('resources:')
  })

  it('step 7: devops sidebar shows DevSecOps Stack, CI/CD, Observability menus', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<Sidebar />)
    expect(screen.getByText('DevSecOps Stack')).toBeInTheDocument()
    expect(screen.getByText('CI/CD')).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.queryByText('Admin')).not.toBeInTheDocument()
  })
})

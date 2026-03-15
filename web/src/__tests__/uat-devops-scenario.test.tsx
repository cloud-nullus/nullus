/**
 * UAT-1: DevOps Engineer "미정" scenario
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from './test-utils'
import { LoginPage } from '../features/auth/pages/login-page'
import { HomePage } from '../features/home/pages/home-page'
import { StackTemplatePage } from '../features/stack/pages/stack-template-page'
import { StackInstallPage } from '../features/stack/pages/stack-install-page'
import { Sidebar } from '../components/layout/sidebar'
import { useAuthStore } from '../stores/auth-store'
import { useStackConfigStore } from '../features/stack/stores/stack-config-store'
import { useSidebarStore } from '../stores/sidebar-store'

vi.mock('../features/stack/api/stack-api', () => ({
  useTemplates: () => ({ data: undefined }),
  useCreateStack: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteTemplate: () => ({ mutate: vi.fn(), isPending: false }),
  useSaveDraft: () => ({ mutate: vi.fn(), isPending: false }),
  useEstimateResources: () => ({ mutate: vi.fn(), isPending: false, data: undefined }),
}))

vi.mock('../components/shared/yaml-editor', () => ({
  YamlEditor: ({ value }: { value: string }) => <pre data-testid="yaml-editor">{value}</pre>,
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
    expect(mockNavigate).toHaveBeenCalledWith('/stack/templates')
  })

  it('step 2: home page shows Get Started CTA for devops', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<HomePage />)
    expect(screen.getByText('Get Started')).toBeInTheDocument()
    expect(screen.getByText('View Stacks')).toBeInTheDocument()
  })

  it('step 3: stack template page shows 3 cards', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)
    expect(screen.getByText('GitLab All-in-One')).toBeInTheDocument()
    expect(screen.getByText('GitLab + ArgoCD')).toBeInTheDocument()
    expect(screen.getByText('GitHub + ArgoCD')).toBeInTheDocument()
  })

  it('step 4: selecting GitLab All-in-One navigates to install page', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackTemplatePage />)

    const buttons = screen.getAllByText('Use Template')
    fireEvent.click(buttons[0]) // GitLab All-in-One is first

    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitlab-all-in-one')
    expect(useStackConfigStore.getState().draft.selectedTemplateId).toBe('gitlab-all-in-one')
  })

  it('step 5: install page tab traversal - all 5 workflow tabs visible', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackInstallPage />)

    // Verify all tabs are present
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getByText('Pipeline')).toBeInTheDocument()
    expect(screen.getByText('Monitoring')).toBeInTheDocument()
    expect(screen.getByText('Logging')).toBeInTheDocument()
    expect(screen.getByText('Resources')).toBeInTheDocument()

    // Traverse through tabs
    fireEvent.click(screen.getByText('Pipeline'))
    expect(screen.getAllByText('CI/CD Platform')[0]).toBeInTheDocument()

    fireEvent.click(screen.getByText('Monitoring'))
    expect(screen.getByText('Metrics Collection')).toBeInTheDocument()

    fireEvent.click(screen.getByText('Logging'))
    expect(screen.getAllByText('Log Collection')[0]).toBeInTheDocument()

    fireEvent.click(screen.getByText('Resources'))
    expect(screen.getByText('개발자 수')).toBeInTheDocument()
  })

  it('step 6: YAML View tab shows configuration', () => {
    useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('YAML View'))
    expect(screen.getByTestId('yaml-editor')).toBeInTheDocument()
    const yaml = screen.getByTestId('yaml-editor').textContent ?? ''
    expect(yaml).toContain('stackName:')
    expect(yaml).toContain('artifacts:')
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

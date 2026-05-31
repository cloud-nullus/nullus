import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { DeveloperDeployPage } from './developer-deploy-page'

const mockNavigate = vi.fn()
const mockUseCicdTemplates = vi.fn()
const mockUseCreatePipeline = vi.fn()
const mockUseClusters = vi.fn()
const mockUseClusterNamespaces = vi.fn()
const mockUseStacks = vi.fn()
const mockUseStackIntegrations = vi.fn()
const mockCreatePipeline = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../api/cicd-api', () => ({
  useCicdTemplates: () => mockUseCicdTemplates(),
  useCreatePipeline: () => mockUseCreatePipeline(),
}))

vi.mock('../../admin/api/admin-api', () => ({
  useClusters: () => mockUseClusters(),
  useClusterNamespaces: () => mockUseClusterNamespaces(),
}))

vi.mock('../../stack/api/stack-api', () => ({
  useStacks: () => mockUseStacks(),
  useStackIntegrations: (stackId: string) => mockUseStackIntegrations(stackId),
}))

vi.mock('../../../components/shared/code-preview', () => ({
  CodePreview: ({ title, code }: { title: string; code: string }) => <div><div>{title}</div><pre>{code}</pre></div>,
}))

const templates = [
  {
    id: 'starter-v1',
    name: 'Starter Template',
    description: 'Starter pipeline',
    appType: 'backend',
    gitRepoUrl: 'https://github.com/cloud-nullus/starter.git',
    dockerfilePath: 'Dockerfile',
    dockerContext: '.',
    envVars: {},
  },
]

const clusters = {
  items: [{ id: 'c1', name: 'prod-k8s', types: ['target'] }],
  total: 1,
}

function completeRequiredFields() {
  fireEvent.change(screen.getByPlaceholderText('my-awesome-app'), { target: { value: 'demo-app' } })
  fireEvent.change(screen.getByLabelText('Source Repository'), {
    target: { value: 'https://github.com/cloud-nullus/demo-app.git' },
  })
}

describe('DeveloperDeployPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseCicdTemplates.mockReset()
    mockUseCreatePipeline.mockReset()
    mockUseClusters.mockReset()
    mockUseClusterNamespaces.mockReset()
    mockUseStacks.mockReset()
    mockUseStackIntegrations.mockReset()
    mockCreatePipeline.mockReset()

    mockUseCicdTemplates.mockReturnValue({ data: templates })
    mockUseCreatePipeline.mockReturnValue({ mutateAsync: mockCreatePipeline, isPending: false })
    mockCreatePipeline.mockResolvedValue({ id: 'pipeline-1' })
    mockUseClusters.mockReturnValue({ data: clusters })
    mockUseClusterNamespaces.mockReturnValue({ data: [] })
    mockUseStacks.mockReturnValue({ data: { items: [{ id: 'stack-1', name: 'app-stack' }], total: 1 } })
    mockUseStackIntegrations.mockReturnValue({
      data: {
        integrations: [{ component_type: 'code_repository', endpoint: 'https://gitlab.stack.internal' }],
      },
    })
  })

  it('renders all six pipeline sections in one scrollable page', async () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(await screen.findByRole('heading', { name: 'Pipeline Setup' })).not.toBeNull()
    expect(screen.getByText('Enter App Name')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Basic Info' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Code Checkout' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Build' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Test' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Security' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Create' })).not.toBeNull()
    expect(screen.getByText('Pipeline Configuration')).not.toBeNull()
    expect(screen.getByText('1. Code Checkout')).not.toBeNull()
    expect(screen.getByText('2. Build')).not.toBeNull()
    expect(screen.getByText('3. Deploy')).not.toBeNull()
    expect(screen.getByText('Cluster & Namespace')).not.toBeNull()
    expect(screen.queryByText(/Quick Start/)).toBeNull()
    expect(screen.queryByRole('button', { name: 'Next' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Previous' })).toBeNull()
  })

  it('keeps template-prefilled values across pipeline pages', async () => {
    renderWithProviders(<DeveloperDeployPage />, { route: '/cicd/developer-deploy?template=starter-v1' })

    await waitFor(() => {
      expect(screen.getByPlaceholderText('my-awesome-app')).toHaveValue('starter')
    })
    expect(screen.getByLabelText('Source Repository')).toHaveValue('https://github.com/cloud-nullus/starter.git')
  })

  it('shows every input section and pending review before required values are complete', () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.getByText('Pipeline Configuration')).not.toBeNull()
    expect(screen.getByText('Cluster & Namespace')).not.toBeNull()
    expect(screen.getByText('Complete required fields to preview deployment manifests.')).not.toBeNull()
  })

  it('shows required markers except for optional fields and renames Stack without optional text', () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.getAllByTestId('required-dot')).toHaveLength(11)
    expect(screen.getByLabelText('Stack')).not.toBeNull()
    expect(screen.queryByLabelText('Stack (Optional)')).toBeNull()
    expect(screen.getByLabelText('Service URL')).not.toBeNull()
  })

  it('allows exactly one phase selection and shows configuration groups by capability', () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.getByLabelText('CI')).toBeChecked()
    expect(screen.getByLabelText('CD')).toBeChecked()
    expect(screen.getByLabelText('production')).toBeChecked()
    expect(screen.getByLabelText('qa')).not.toBeChecked()
    expect(screen.getByLabelText('development')).not.toBeChecked()
    fireEvent.click(screen.getByLabelText('qa'))
    expect(screen.getByLabelText('production')).not.toBeChecked()
    expect(screen.getByLabelText('qa')).toBeChecked()
    fireEvent.click(screen.getByLabelText('qa'))
    expect(screen.getByLabelText('qa')).toBeChecked()
    fireEvent.click(screen.getByLabelText('development'))
    expect(screen.getByLabelText('qa')).not.toBeChecked()
    expect(screen.getByLabelText('development')).toBeChecked()

    expect(screen.getByLabelText('Source Repository')).not.toBeNull()
    expect(screen.getByLabelText('Dockerfile Repository')).not.toBeNull()
    expect(screen.getByLabelText('Deploy YAML Repository')).not.toBeNull()

    fireEvent.click(screen.getByLabelText('CI'))
    expect(screen.queryByText('1. Code Checkout')).toBeNull()
    expect(screen.queryByText('2. Build')).toBeNull()
    expect(screen.queryByLabelText('Source Repository')).toBeNull()
    expect(screen.getByText('3. Deploy')).not.toBeNull()
    expect(screen.getByLabelText('Deploy YAML Repository')).not.toBeNull()

    fireEvent.click(screen.getByLabelText('CD'))
    expect(screen.queryByText('3. Deploy')).toBeNull()
    expect(screen.queryByLabelText('Deploy YAML Repository')).toBeNull()
  })

  it('shows the generated manifests in review and creates without deploying', async () => {
    renderWithProviders(<DeveloperDeployPage />)

    completeRequiredFields()

    expect(await screen.findByText('demo-app-deployment.yaml')).not.toBeNull()
    expect(screen.getByText('demo-app-service.yaml')).not.toBeNull()
    expect(screen.getByText('demo-app-ingress.yaml')).not.toBeNull()
    fireEvent.click(screen.getAllByRole('button', { name: 'Create' }).at(-1)!)
    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalled()
      expect(mockNavigate).toHaveBeenCalledWith('/cicd/list')
    })
  })

  it('uses the selected stacks code repository endpoint as the code URL prefix', () => {
    renderWithProviders(<DeveloperDeployPage />)

    fireEvent.change(screen.getByLabelText('Stack'), { target: { value: 'stack-1' } })

    expect(screen.getByDisplayValue('https://gitlab.stack.internal/')).toBeDisabled()
    expect(screen.getByPlaceholderText('owner/repo-name.git')).not.toBeNull()
  })

  it('allows review and create with a repository URL assembled from the selected stack', async () => {
    renderWithProviders(<DeveloperDeployPage />)

    fireEvent.change(screen.getByPlaceholderText('my-awesome-app'), { target: { value: 'demo-app' } })
    fireEvent.change(screen.getByLabelText('Stack'), { target: { value: 'stack-1' } })
    fireEvent.change(screen.getByPlaceholderText('owner/repo-name.git'), { target: { value: 'owner/demo-app.git' } })

    expect(await screen.findByText('demo-app-deployment.yaml')).not.toBeNull()
    fireEvent.click(screen.getAllByRole('button', { name: 'Create' }).at(-1)!)
    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalledWith(expect.objectContaining({
        gitRepoUrl: 'https://gitlab.stack.internal/owner/demo-app.git',
      }))
    })
  })

  it('keeps manual code URL input visible when the selected stack has no repository endpoint', () => {
    mockUseStackIntegrations.mockReturnValue({ data: { integrations: [] } })
    renderWithProviders(<DeveloperDeployPage />)

    fireEvent.change(screen.getByLabelText('Stack'), { target: { value: 'stack-1' } })

    expect(screen.getByLabelText('Source Repository')).not.toBeNull()
  })

  it('uses text fields instead of slider controls for resources', () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.getByLabelText('CPU Request')).toHaveValue('100m')
    expect(screen.getByLabelText('CPU Limit')).toHaveValue('500m')
    expect(screen.getByLabelText('Memory Request')).toHaveValue('128Mi')
    expect(screen.getByLabelText('Memory Limit')).toHaveValue('512Mi')
    expect(screen.queryByRole('slider')).toBeNull()
  })

  it('groups code checkout, build, and deploy repository inputs together', () => {
    renderWithProviders(<DeveloperDeployPage />)

    const source = screen.getByLabelText('Source Repository')
    const dockerfile = screen.getByLabelText('Dockerfile Repository')
    const deployYaml = screen.getByLabelText('Deploy YAML Repository')

    expect(source.compareDocumentPosition(dockerfile) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(dockerfile.compareDocumentPosition(deployYaml) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.getAllByLabelText('Branch')).toHaveLength(2)
    expect(screen.getAllByLabelText('Directory')).toHaveLength(2)
  })

  it('shows separate manifests and enables create without an optional service URL', async () => {
    renderWithProviders(<DeveloperDeployPage />)

    expect(screen.queryByText('Review Manifest')).toBeNull()
    completeRequiredFields()

    expect(await screen.findByText('Review Manifest')).not.toBeNull()
    expect(screen.getByText('demo-app-deployment.yaml')).not.toBeNull()
    expect(screen.getByText('demo-app-service.yaml')).not.toBeNull()
    expect(screen.getByText('demo-app-ingress.yaml')).not.toBeNull()
    expect(screen.getByText(/host: demo-app\.internal/)).not.toBeNull()
    expect(screen.getAllByRole('button', { name: 'Create' }).at(-1)!).not.toBeDisabled()
  })

  it('uses a provided optional service URL as the ingress host', async () => {
    renderWithProviders(<DeveloperDeployPage />)
    completeRequiredFields()
    fireEvent.change(screen.getByPlaceholderText('app.example.com'), { target: { value: 'https://demo-app.example.com/health' } })

    expect(await screen.findByText(/host: demo-app\.example\.com/)).not.toBeNull()
  })

  it('loads YAML previews from a Deploy YAML Repository path', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      text: vi.fn().mockResolvedValue('kind: Deployment\nmetadata:\n  name: imported\n---\nkind: Service\nmetadata:\n  name: imported-svc\n'),
    }))
    renderWithProviders(<DeveloperDeployPage />)
    completeRequiredFields()

    fireEvent.change(screen.getByLabelText('Deploy YAML Repository'), {
      target: { value: 'https://config.example.com/manifests/' },
    })
    fireEvent.change(screen.getByPlaceholderText('deploy/app.yaml'), { target: { value: 'app.yaml' } })
    fireEvent.click(screen.getByRole('button', { name: 'Load from Deploy YAML Repository' }))

    expect(await screen.findByText(/^kind: Deployment[\s\S]*name: imported$/)).not.toBeNull()
    expect(screen.getByText(/kind: Service[\s\S]*name: imported-svc/)).not.toBeNull()
    expect(fetch).toHaveBeenCalledWith(new URL('https://config.example.com/manifests/app.yaml'))
  })

  it('navigates to the CI/CD list after creating from manifest review', async () => {
    renderWithProviders(<DeveloperDeployPage />)
    completeRequiredFields()

    fireEvent.click((await screen.findAllByRole('button', { name: 'Create' })).at(-1)!)

    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalled()
      expect(mockNavigate).toHaveBeenCalledWith('/cicd/list')
    })
  })
})

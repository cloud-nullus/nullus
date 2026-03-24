import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { screen, fireEvent, act } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackInstallPage } from './stack-install-page'
import { useStackConfigStore } from '../stores/stack-config-store'
import YAML from 'yaml'

// Mock API hooks
vi.mock('../api/stack-api', () => ({
  useCreateStack: () => ({ mutate: vi.fn(), isPending: false }),
  useSaveDraft: () => ({ mutate: vi.fn(), isPending: false }),
  useEstimateResources: () => ({ mutate: vi.fn(), isPending: false, data: undefined }),
  useClusters: () => ({ data: [{ id: 'cluster-1', name: 'test-cluster', connection_status: 'connected' }] }),
  useResourceDefaults: () => ({ data: { items: [], total: 0 } }),
  useDeployStack: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../../admin/api/admin-api', () => ({
  useClusterNamespaces: () => ({ data: [] }),
}))

// Mock useNavigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('@monaco-editor/react', () => ({
  default: ({ value, onChange }: { value?: string; onChange?: (value: string) => void }) => (
    <textarea
      data-testid="monaco-yaml-editor"
      value={value ?? ''}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}))

vi.mock('monaco-yaml', () => ({
  configureMonacoYaml: vi.fn(),
}))

beforeEach(() => {
  useStackConfigStore.getState().resetConfig()
  mockNavigate.mockClear()
  Object.assign(navigator, {
    clipboard: {
      writeText: vi.fn().mockResolvedValue(undefined),
    },
  })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('StackInstallPage', () => {
  const fillRequiredSelectionsForConfigTabs = () => {
    fireEvent.change(screen.getByLabelText('Target Cluster'), { target: { value: 'cluster-1' } })
    fireEvent.change(screen.getByLabelText('Namespace'), { target: { value: '__new__' } })
    fireEvent.change(screen.getByPlaceholderText('my-namespace'), { target: { value: 'qa-namespace' } })
  }

  it('renders the page heading', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getAllByText('Stack Install')[0]).toBeInTheDocument()
  })

  it('renders install tabs including storage and YAML view', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getAllByText('CI/CD')[0]).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Resources' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Storage' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'YAML View' })).toBeInTheDocument()
  })

  it('sets default stack name automatically', () => {
    renderWithProviders(<StackInstallPage />)
    const stackNameInput = screen.getByLabelText('Stack Name') as HTMLInputElement
    expect(stackNameInput.value).toMatch(/^nullus-devsecops-stack-\d{8}-\d{6}$/)
  })

  it('default tab shows Artifacts content', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getAllByText('Package Registry')[0]).toBeTruthy()
    expect(screen.getAllByText('Source Repository')[0]).toBeTruthy()
  })

  it('clicking CI/CD tab shows CI/CD content', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getAllByText('CI/CD')[0])
    expect(screen.getAllByText('CI/CD Platform')[0]).toBeInTheDocument()
    expect(screen.getAllByText('CD Tool')[0]).toBeInTheDocument()
  })

  it('clicking Observability tab shows merged observability content', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Observability' }))
    expect(screen.getAllByText('Visualization')[0]).toBeTruthy()
    expect(screen.getAllByText('Metrics')[0]).toBeTruthy()
    expect(screen.getAllByText('Logs')[0]).toBeTruthy()
    expect(screen.getAllByText('Traces')[0]).toBeTruthy()
  })

  it('clicking Resources tab shows Resources content', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Resources' }))
    expect(screen.getByText('OSS별 Resource Planning')).toBeTruthy()
    expect(screen.getByText('Sizing Profile')).toBeTruthy()
  })

  it('blocks YAML tab until required fields are selected', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    expect(screen.queryByTestId('monaco-yaml-editor')).toBeNull()
    expect(screen.getByText('YAML View 탭으로 이동하려면 Target Cluster 선택이 필요합니다.')).toBeInTheDocument()
  })

  it('clicking YAML View tab shows per-OSS manifest editor when required fields are set', () => {
    renderWithProviders(<StackInstallPage />)
    fillRequiredSelectionsForConfigTabs()
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    expect(screen.getByTestId('monaco-yaml-editor')).toBeTruthy()
  })

  it('clicking YAML View tab shows per-OSS manifest editor with helm/yaml tag', () => {
    renderWithProviders(<StackInstallPage />)
    fillRequiredSelectionsForConfigTabs()
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    expect(screen.getByText(/선택한 OSS별 설치 파일입니다/)).toBeInTheDocument()
    expect(screen.getAllByText(/helm|yaml/i)[0]).toBeTruthy()
    const editor = screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement
    expect(editor.value).toContain('global:')
    expect(editor.value).toContain('chart:')
    expect(editor.value).not.toContain('kind: StackToolInstall')
    expect(screen.getByText(/역할:/)).toBeInTheDocument()
    expect(screen.getByText(/동일 OSS가 여러 역할에 선택돼도 설치 파일은 하나로 통합/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Grafana/i }))
    expect((screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement).value).toContain('apiVersion: apps/v1')
  })

  it('bundles gitlab-related selections into one install file with merged roles', () => {
    const store = useStackConfigStore.getState()
    store.setTool('artifacts', 'packageRegistry', { tool: 'gitlab', version: '17.2.0' })
    store.setTool('artifacts', 'sourceRepository', { tool: 'gitlab', version: '17.2.0' })
    store.setTool('artifacts', 'containerRegistry', { tool: 'gitlab-registry', version: '17.2.0' })
    store.setTool('pipeline', 'cicdPlatform', { tool: 'gitlab-ci', version: '17.2.0' })

    renderWithProviders(<StackInstallPage />)
    fillRequiredSelectionsForConfigTabs()
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))

    expect(screen.getAllByRole('button', { name: /GitLab/i })).toHaveLength(1)
    expect(screen.getByText(/역할:/)).toHaveTextContent('Artifacts > Package Registry')
    expect(screen.getByText(/역할:/)).toHaveTextContent('Artifacts > Source Repository')
    expect(screen.getByText(/역할:/)).toHaveTextContent('Artifacts > Container Registry')

    const editor = screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement
    expect(editor.value).toContain('chart:')
    expect(editor.value).toContain('name: gitlab/gitlab')
    expect(editor.value).toContain('tag: 17.2.0')
  })

  it('keeps bundled gitlab memory value stable after install-file edit', () => {
    vi.useFakeTimers()
    const store = useStackConfigStore.getState()
    store.setTool('artifacts', 'packageRegistry', { tool: 'gitlab', version: '17.2.0' })
    store.setTool('artifacts', 'sourceRepository', { tool: 'gitlab', version: '17.2.0' })
    store.setTool('artifacts', 'containerRegistry', { tool: 'gitlab-registry', version: '17.2.0' })
    store.setTool('pipeline', 'cicdPlatform', { tool: 'gitlab-ci', version: '17.2.0' })

    renderWithProviders(<StackInstallPage />)
    fillRequiredSelectionsForConfigTabs()
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))

    const editor = screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement
    const values = YAML.parse(editor.value) as Record<string, unknown>
    const resources = (values.resources ?? {}) as Record<string, unknown>
    const requests = (resources.requests ?? {}) as Record<string, unknown>
    const limits = (resources.limits ?? {}) as Record<string, unknown>
    requests.memory = '33.00Gi'
    limits.memory = '66.00Gi'
    resources.requests = requests
    resources.limits = limits
    values.resources = resources

    fireEvent.change(editor, { target: { value: YAML.stringify(values) } })

    act(() => {
      vi.advanceTimersByTime(500)
    })

    const updated = (screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement).value
    expect(updated).toContain('memory: 33.00Gi')
    expect(updated).toContain('memory: 66.00Gi')
    expect(updated).not.toContain('memory: 88.00Gi')
  })

  it('shows access domain input and default OSS access guide', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByLabelText('Access domain')).toBeInTheDocument()
    expect(screen.getByText(/최종 접근 가이드/)).toBeInTheDocument()
  })

  it('shows Preview Deploy Script inside YAML View and script reflects selected options', () => {
    const store = useStackConfigStore.getState()
    store.setStackName('devsecops-stack')
    store.setTool('artifacts', 'packageRegistry', { tool: 'gitlab', version: '17.2.1' })
    store.setTool('monitoring', 'visualization', { tool: 'grafana', version: '11.0.0' })

    renderWithProviders(<StackInstallPage />)
    fillRequiredSelectionsForConfigTabs()
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    fireEvent.click(screen.getByRole('button', { name: 'Preview Deploy Script' }))

    expect(screen.getByText('Deploy Script Preview')).toBeInTheDocument()
    expect(screen.getByText(/helm upgrade --install gitlab/)).toBeInTheDocument()
    expect(screen.getByText(/--version 17.2.1/)).toBeInTheDocument()
    expect(screen.getByText(/kubectl apply -n qa-namespace -f manifests\/grafana.yaml/)).toBeInTheDocument()
  })

  it('selecting a tool in Artifacts updates the store', () => {
    renderWithProviders(<StackInstallPage />)
    // Click Nexus option in Package Registry
    fireEvent.click(screen.getByText('Nexus Repository'))
    expect(useStackConfigStore.getState().draft.artifacts.packageRegistry.tool).toBe('nexus')
  })

  it('selecting a tool in Pipeline updates the store', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getAllByText('CI/CD')[0])
    fireEvent.click(screen.getByText('GitHub Actions'))
    expect(useStackConfigStore.getState().draft.pipeline.cicdPlatform.tool).toBe('github-actions')
  })

  it('renders Save Draft and Deploy buttons', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Save Draft')).toBeTruthy()
    expect(screen.getByText('Deploy')).toBeTruthy()
  })

  it('renders Configuration Summary sidebar', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Configuration Summary')).toBeTruthy()
  })
})

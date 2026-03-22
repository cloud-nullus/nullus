import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { screen, fireEvent, act } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackInstallPage } from './stack-install-page'
import { useStackConfigStore } from '../stores/stack-config-store'

// Mock API hooks
vi.mock('../api/stack-api', () => ({
  useCreateStack: () => ({ mutate: vi.fn(), isPending: false }),
  useSaveDraft: () => ({ mutate: vi.fn(), isPending: false }),
  useEstimateResources: () => ({ mutate: vi.fn(), isPending: false, data: undefined }),
  useClusters: () => ({ data: [{ id: 'cluster-1', name: 'test-cluster', connection_status: 'connected' }] }),
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
  it('renders the page heading', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getAllByText('Stack Install')[0]).toBeInTheDocument()
  })

  it('renders all 5 tabs', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getAllByText('CI/CD')[0]).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByText('Resources')).toBeInTheDocument()
    expect(screen.getByText('YAML View')).toBeInTheDocument()
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
    expect(screen.getByText('개발자 수')).toBeTruthy()
    expect(screen.getByText('동시 러너 수')).toBeTruthy()
  })

  it('clicking YAML View tab shows monaco yaml editor', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    expect(screen.getByTestId('monaco-yaml-editor')).toBeTruthy()
  })

  it('YAML View shows current configuration', () => {
    useStackConfigStore.getState().setStackName('test-stack')
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    const yaml = (screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement).value
    expect(yaml).toContain('test-stack')
  })

  it('editing valid YAML updates stack config store after debounce', () => {
    vi.useFakeTimers()
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))

    const editor = screen.getByTestId('monaco-yaml-editor')
    fireEvent.change(editor, { target: { value: 'stackName: yaml-stack' } })

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(useStackConfigStore.getState().draft.stackName).toBe('yaml-stack')
  })

  it('editing invalid YAML does not update stack config store', () => {
    vi.useFakeTimers()
    useStackConfigStore.getState().setStackName('before-invalid')
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))

    const editor = screen.getByTestId('monaco-yaml-editor')
    fireEvent.change(editor, { target: { value: 'stackName: [invalid' } })

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(useStackConfigStore.getState().draft.stackName).toBe('before-invalid')
  })

  it('copy button copies current YAML content', async () => {
    useStackConfigStore.getState().setStackName('copy-stack')
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))

    await act(async () => {
      await Promise.resolve()
    })

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining('copy-stack'))
  })

  it('format button reprints YAML and keeps store synced', () => {
    vi.useFakeTimers()
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByRole('button', { name: 'YAML View' }))
    const editor = screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement

    fireEvent.change(editor, { target: { value: 'stackName: formatted' } })
    act(() => {
      vi.advanceTimersByTime(300)
    })

    fireEvent.click(screen.getByRole('button', { name: 'Format' }))

    expect((screen.getByTestId('monaco-yaml-editor') as HTMLTextAreaElement).value).toContain('stackName: formatted')
    expect(useStackConfigStore.getState().draft.stackName).toBe('formatted')
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

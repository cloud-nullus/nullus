import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackInstallPage } from './stack-install-page'
import { useStackConfigStore } from '../stores/stack-config-store'

// Mock API hooks
vi.mock('../api/stack-api', () => ({
  useCreateStack: () => ({ mutate: vi.fn(), isPending: false }),
  useSaveDraft: () => ({ mutate: vi.fn(), isPending: false }),
  useEstimateResources: () => ({ mutate: vi.fn(), isPending: false, data: undefined }),
}))

// Mock useNavigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

// Mock YamlEditor — it's not the focus of these tests
vi.mock('../../../components/shared/yaml-editor', () => ({
  YamlEditor: ({ value }: { value: string }) => <pre data-testid="yaml-editor">{value}</pre>,
}))

beforeEach(() => {
  useStackConfigStore.getState().resetConfig()
  mockNavigate.mockClear()
})

describe('StackInstallPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Stack Install')).toBeInTheDocument()
  })

  it('renders all 5 tabs', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getByText('CI/CD')).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByText('Resources')).toBeInTheDocument()
    expect(screen.getByText('YAML View')).toBeInTheDocument()
  })

  it('default tab shows Artifacts content', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getAllByText('Package Registry')[0]).toBeInTheDocument()
    expect(screen.getAllByText('Source Repository')[0]).toBeInTheDocument()
  })

  it('clicking CI/CD tab shows CI/CD content', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('CI/CD'))
    expect(screen.getAllByText('CI/CD Platform')[0]).toBeInTheDocument()
    expect(screen.getAllByText('CD Tool')[0]).toBeInTheDocument()
  })

  it('clicking Observability tab shows merged observability content', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('Observability'))
    expect(screen.getAllByText('Visualization')[0]).toBeInTheDocument()
    expect(screen.getAllByText('Metrics')[0]).toBeInTheDocument()
    expect(screen.getAllByText('Logs')[0]).toBeInTheDocument()
    expect(screen.getAllByText('Traces')[0]).toBeInTheDocument()
  })

  it('clicking Resources tab shows Resources content', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('Resources'))
    expect(screen.getByText('개발자 수')).toBeInTheDocument()
    expect(screen.getByText('동시 러너 수')).toBeInTheDocument()
  })

  it('clicking YAML View tab shows yaml editor', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('YAML View'))
    expect(screen.getByTestId('yaml-editor')).toBeInTheDocument()
  })

  it('YAML View shows current configuration', () => {
    useStackConfigStore.getState().setStackName('test-stack')
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('YAML View'))
    const yaml = screen.getByTestId('yaml-editor').textContent ?? ''
    expect(yaml).toContain('test-stack')
  })

  it('selecting a tool in Artifacts updates the store', () => {
    renderWithProviders(<StackInstallPage />)
    // Click Nexus option in Package Registry
    fireEvent.click(screen.getByText('Nexus Repository'))
    expect(useStackConfigStore.getState().draft.artifacts.packageRegistry.tool).toBe('nexus')
  })

  it('selecting a tool in Pipeline updates the store', () => {
    renderWithProviders(<StackInstallPage />)
    fireEvent.click(screen.getByText('CI/CD'))
    fireEvent.click(screen.getByText('GitHub Actions'))
    expect(useStackConfigStore.getState().draft.pipeline.cicdPlatform.tool).toBe('github-actions')
  })

  it('renders Save Draft and Deploy buttons', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Save Draft')).toBeInTheDocument()
    expect(screen.getByText('Deploy')).toBeInTheDocument()
  })

  it('renders Configuration Summary sidebar', () => {
    renderWithProviders(<StackInstallPage />)
    expect(screen.getByText('Configuration Summary')).toBeInTheDocument()
  })
})

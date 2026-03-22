import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackAddToolsPage } from './stack-add-tools-page'

const mockMutateAsync = vi.fn().mockResolvedValue({})

const FULLY_INSTALLED_STACK = {
  id: 'production-stack',
  name: 'production-stack',
  templateId: 'gitlab-all-in-one',
  templateName: 'GitLab All-in-One',
  clusterId: 'c1',
  clusterName: 'prod-k8s',
  status: 'success',
  createdAt: '2026-01-10T00:00:00Z',
  updatedAt: '2026-03-03T14:28:00Z',
  tools: [
    { category: 'package_registry', tool: 'gitlab', version: 'latest' },
    { category: 'source_repository', tool: 'gitlab', version: 'latest' },
    { category: 'container_registry', tool: 'harbor', version: 'latest' },
    { category: 'storage_backend', tool: 'minio', version: 'latest' },
    { category: 'ci_platform', tool: 'gitlab-ci', version: 'latest' },
    { category: 'cd_tool', tool: 'argocd', version: 'latest' },
    { category: 'metrics_collection', tool: 'prometheus', version: 'latest' },
    { category: 'visualization', tool: 'grafana', version: 'latest' },
    { category: 'trace_layer', tool: 'tempo', version: 'latest' },
    { category: 'log_search', tool: 'opensearch', version: 'latest' },
  ],
}

const EMPTY_STACK = {
  id: 'empty-stack',
  name: 'empty-stack',
  templateId: 'custom',
  templateName: 'Custom',
  clusterId: 'c2',
  clusterName: 'dev-k8s',
  status: 'running',
  createdAt: '2026-02-01T00:00:00Z',
  updatedAt: '2026-03-01T00:00:00Z',
  tools: [],
}

vi.mock('../api/stack-api', () => ({
  useStacks: () => ({
    data: { items: [FULLY_INSTALLED_STACK, EMPTY_STACK], total: 2 },
    isLoading: false,
  }),
  useAddTools: () => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  }),
}))

const mockNavigate = vi.fn()
let mockStackId = 'empty-stack'

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({ id: mockStackId }),
  }
})

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

beforeEach(() => {
  mockMutateAsync.mockClear()
  mockNavigate.mockClear()
  mockStackId = 'empty-stack'
})

describe('StackAddToolsPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<StackAddToolsPage />)
    expect(screen.getByRole('heading', { level: 1, name: 'Add Tools' })).toBeInTheDocument()
  })

  it('renders all 3 step tabs', () => {
    renderWithProviders(<StackAddToolsPage />)
    expect(screen.getByText('1. Category Selection')).toBeInTheDocument()
    expect(screen.getByText('2. Tool Configuration')).toBeInTheDocument()
    expect(screen.getByText('3. Review & Deploy')).toBeInTheDocument()
  })

  it('renders breadcrumb with stack name', () => {
    renderWithProviders(<StackAddToolsPage />)
    expect(screen.getByText('Stack List')).toBeInTheDocument()
    expect(screen.getByText('empty-stack')).toBeInTheDocument()
  })

  it('step 1 shows all 4 category cards', () => {
    renderWithProviders(<StackAddToolsPage />)
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
    expect(screen.getByText('Pipeline')).toBeInTheDocument()
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByText('Logging')).toBeInTheDocument()
  })

  it('clicking a category card marks it as selected', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Artifacts'))
    expect(screen.getByText('Selected')).toBeInTheDocument()
  })

  it('clicking a category card twice deselects it', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Artifacts'))
    expect(screen.getByText('Selected')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Artifacts'))
    expect(screen.queryByText('Selected')).not.toBeInTheDocument()
  })

  it('Next button is disabled when no category is selected', () => {
    renderWithProviders(<StackAddToolsPage />)
    const nextBtn = screen.getByText('Next')
    expect(nextBtn.closest('button')).toBeDisabled()
  })

  it('Next button is enabled after selecting a category', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Pipeline'))
    const nextBtn = screen.getByText('Next')
    expect(nextBtn.closest('button')).not.toBeDisabled()
  })

  it('step 2 shows tool selectors for selected categories', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Pipeline'))
    fireEvent.click(screen.getByText('Next'))
    expect(screen.getByText('CI Platform')).toBeInTheDocument()
    expect(screen.getByText('CD Tool')).toBeInTheDocument()
  })

  it('step 2 shows empty state when no categories selected', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('2. Tool Configuration'))
    expect(screen.getByText('먼저 Step 1에서 카테고리를 선택해 주세요.')).toBeInTheDocument()
  })

  it('step 3 shows review table with selected tools', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Pipeline'))
    fireEvent.click(screen.getByText('Next'))
    fireEvent.click(screen.getByText('Next'))
    expect(screen.getByText('Review & Deploy')).toBeInTheDocument()
    expect(screen.getByText('Category')).toBeInTheDocument()
    expect(screen.getByText('Slot')).toBeInTheDocument()
    expect(screen.getByText('Tool')).toBeInTheDocument()
    expect(screen.getByText('Version')).toBeInTheDocument()
  })

  it('step 3 shows Confirm & Deploy button', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Pipeline'))
    fireEvent.click(screen.getByText('Next'))
    fireEvent.click(screen.getByText('Next'))
    expect(screen.getByText('Confirm & Deploy')).toBeInTheDocument()
  })

  it('Previous button is disabled on step 0', () => {
    renderWithProviders(<StackAddToolsPage />)
    const prevBtn = screen.getByText('Previous')
    expect(prevBtn.closest('button')).toBeDisabled()
  })

  it('Previous button navigates back from step 2 to step 1', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Logging'))
    fireEvent.click(screen.getByText('Next'))
    expect(screen.getByText('Log Search')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Previous'))
    expect(screen.getByText('Artifacts')).toBeInTheDocument()
  })

  it('Back to List button navigates to /stack/list', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Back to List'))
    expect(mockNavigate).toHaveBeenCalledWith('/stack/list')
  })

  it('Confirm & Deploy calls useAddTools with correct payload shape', () => {
    renderWithProviders(<StackAddToolsPage />)
    fireEvent.click(screen.getByText('Pipeline'))
    fireEvent.click(screen.getByText('Next'))
    fireEvent.click(screen.getByText('Next'))
    fireEvent.click(screen.getByText('Confirm & Deploy'))

    expect(mockMutateAsync).toHaveBeenCalledTimes(1)
    const call = mockMutateAsync.mock.calls[0][0]
    expect(call.stackId).toBe('empty-stack')
    expect(Array.isArray(call.tools)).toBe(true)
    expect(call.tools.length).toBeGreaterThan(0)
    expect(call.tools[0]).toHaveProperty('category')
    expect(call.tools[0]).toHaveProperty('tool')
    expect(call.tools[0]).toHaveProperty('version')
  })
})

describe('StackAddToolsPage with fully installed stack', () => {
  it('shows Installed badge for categories with all tools installed', () => {
    mockStackId = 'production-stack'
    renderWithProviders(<StackAddToolsPage />)
    const installedBadges = screen.getAllByText('Installed')
    expect(installedBadges.length).toBeGreaterThan(0)
  })

  it('disables category buttons for installed categories', () => {
    mockStackId = 'production-stack'
    renderWithProviders(<StackAddToolsPage />)
    const artifactsBtn = screen.getByText('Artifacts').closest('button')
    expect(artifactsBtn).toBeDisabled()
  })
})

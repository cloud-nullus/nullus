import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackTemplatePage } from './stack-template-page'
import { useStackConfigStore } from '../stores/stack-config-store'

// Mock the API hooks — they return undefined so MOCK_TEMPLATES are used
vi.mock('../api/stack-api', () => ({
  useTemplates: () => ({ data: undefined }),
}))

// Mock useNavigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

beforeEach(() => {
  useStackConfigStore.getState().resetConfig()
  mockNavigate.mockClear()
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
    expect(mockNavigate).toHaveBeenCalledWith('/stack/install?template=gitlab-all-in-one')
  })

  it('clicking Use Template sets template in store', () => {
    renderWithProviders(<StackTemplatePage />)
    const buttons = screen.getAllByText('Use Template')
    fireEvent.click(buttons[0])
    const { draft } = useStackConfigStore.getState()
    expect(draft.selectedTemplateId).toBe('gitlab-all-in-one')
  })
})

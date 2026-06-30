import { fireEvent, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackListPage } from './stack-list-page'

const mockNavigate = vi.fn()
const mockUseStacks = vi.fn()
const mockUseDeleteStack = vi.fn()
const mockUseImportStackConfig = vi.fn()
const mockUsePreviewImportStackConfig = vi.fn()
const mockUseStackHistory = vi.fn()
const mockUseStackMonitoring = vi.fn()
const mockUseClusters = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../api/stack-api', () => ({
  useStacks: (...args: unknown[]) => mockUseStacks(...args),
  useDeleteStack: (...args: unknown[]) => mockUseDeleteStack(...args),
  useImportStackConfig: (...args: unknown[]) => mockUseImportStackConfig(...args),
  usePreviewImportStackConfig: (...args: unknown[]) => mockUsePreviewImportStackConfig(...args),
  useExportStackConfig: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useStackHistory: (...args: unknown[]) => mockUseStackHistory(...args),
  useStackMonitoring: (...args: unknown[]) => mockUseStackMonitoring(...args),
  useClusters: (...args: unknown[]) => mockUseClusters(...args),
  useRetryStack: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: () => ({ role: 'devops', isAuthenticated: true }),
}))

describe('StackListPage import flow', () => {
  let mutateAsyncSpy: ReturnType<typeof vi.fn>
  let previewMutateAsyncSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseStacks.mockReset()
    mockUseDeleteStack.mockReset()
    mockUseImportStackConfig.mockReset()
    mockUsePreviewImportStackConfig.mockReset()
    mockUseStackHistory.mockReset()
    mockUseStackMonitoring.mockReset()
    mockUseClusters.mockReset()

    mockUseStacks.mockReturnValue({ data: { items: [], total: 0 }, isLoading: false })
    mockUseDeleteStack.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mutateAsyncSpy = vi.fn().mockResolvedValue({ id: 'stk-imported' })
    previewMutateAsyncSpy = vi.fn().mockResolvedValue({ mode: 'create', name: 'demo', cluster_id: 'cluster-1' })
    mockUseImportStackConfig.mockReturnValue({ mutateAsync: mutateAsyncSpy, isPending: false })
    mockUsePreviewImportStackConfig.mockReturnValue({ mutateAsync: previewMutateAsyncSpy, isPending: false })
    mockUseStackHistory.mockReturnValue({ data: [], isLoading: false })
    mockUseStackMonitoring.mockReturnValue({ data: null, isLoading: false })
    mockUseClusters.mockReturnValue({ data: { items: [] }, isLoading: false })
  })

  it('uploads an export file and restores a stack', async () => {
    renderWithProviders(<StackListPage />)

    fireEvent.click(screen.getByRole('button', { name: /stackList\.actions\.import|Import/ }))

    const file = new File(['{"name":"demo"}'], 'stack.json', { type: 'application/json' })
    fireEvent.change(screen.getByLabelText(/stackList\.import\.file|Export file/), { target: { files: [file] } })
    fireEvent.click(screen.getByRole('button', { name: /stackList\.import\.preview|Review Import/ }))
    await waitFor(() => {
      expect(previewMutateAsyncSpy).toHaveBeenCalledWith('{"name":"demo"}')
    })
    fireEvent.click(screen.getByRole('button', { name: /stackList\.import\.confirm|Restore/ }))

    await waitFor(() => {
      expect(mutateAsyncSpy).toHaveBeenCalledWith({ payload: '{"name":"demo"}', replaceExisting: false })
    })
  })
})

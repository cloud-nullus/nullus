import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackVersionsAdminPage } from './stack-versions-page'
import type { CompatibilityMatrix } from '../../../types'

const matrices: CompatibilityMatrix[] = [
  {
    id: 'alpha-baseline',
    name: 'Alpha Baseline',
    status: 'verified',
    k8sRange: 'v1.27-v1.29',
    tools: [
      {
        name: 'argo-cd',
        helmVersion: '5.55.0',
        appVersion: '2.12',
        archSupport: ['amd64', 'arm64'],
        minK8sVersion: 'v1.27',
        tier: 'stable',
      },
    ],
  },
  {
    id: 'bravo-gamma',
    name: 'Bravo Gamma',
    status: 'untested',
    k8sRange: 'v1.28-v1.30',
    tools: [
      {
        name: 'tekton',
        helmVersion: '0.40.0',
        appVersion: '0.52',
        archSupport: ['amd64'],
        minK8sVersion: 'v1.28',
        tier: 'beta',
      },
    ],
  },
  {
    id: 'charlie-delta',
    name: 'Charlie Delta',
    status: 'unsupported',
    k8sRange: 'v1.29-v1.31',
    tools: [],
  },
]

const deleteMutate = vi.hoisted(() => vi.fn())
const createMutate = vi.hoisted(() => vi.fn())
const updateMutate = vi.hoisted(() => vi.fn())
const mockMatrices = vi.hoisted(() => ({ current: [] as unknown as CompatibilityMatrix[] }))

vi.mock('../../stack/api/stack-api', () => ({
  useCompatibilityMatrix: () => ({ data: mockMatrices.current, isLoading: false, isError: false }),
  useDeleteMatrix: () => ({ mutate: deleteMutate, isPending: false }),
  useCreateMatrix: () => ({ mutate: createMutate, isPending: false }),
  useUpdateMatrix: () => ({ mutate: updateMutate, isPending: false }),
}))

vi.mock('../api/admin-api', () => ({
  useClusters: () => ({
    data: { items: [], total: 0 },
    isLoading: false,
  }),
  useRefreshDiscovery: () => ({ mutate: vi.fn(), mutateAsync: vi.fn().mockResolvedValue(undefined), isPending: false, variables: undefined }),
}))

beforeEach(() => {
  vi.clearAllMocks()
  mockMatrices.current = matrices
})

describe('StackVersionsAdminPage', () => {
  it('renders page header, New matrix button, and the matrix list', () => {
    renderWithProviders(<StackVersionsAdminPage />)
    expect(screen.getByRole('button', { name: /new matrix/i })).toBeInTheDocument()
    expect(screen.getAllByText('Alpha Baseline').length).toBeGreaterThan(0)
    expect(screen.getByText('Bravo Gamma')).toBeInTheDocument()
    expect(screen.getByText('Charlie Delta')).toBeInTheDocument()
  })

  it('opens the New matrix modal with an enabled ID field', () => {
    renderWithProviders(<StackVersionsAdminPage />)
    fireEvent.click(screen.getByRole('button', { name: /new matrix/i }))
    const dialog = screen.getByRole('dialog', { name: /new matrix/i })
    const idInput = within(dialog).getByPlaceholderText('my-matrix-v1') as HTMLInputElement
    expect(idInput).toBeInTheDocument()
    expect(idInput.disabled).toBe(false)
    expect(idInput.value).toBe('')
  })

  it('opens the Edit modal with the selected matrix pre-filled and ID locked', () => {
    renderWithProviders(<StackVersionsAdminPage />)
    // Default selection is the first sorted matrix (alpha-baseline).
    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    const dialog = screen.getByRole('dialog', { name: /edit matrix/i })
    const idInput = within(dialog).getByPlaceholderText('my-matrix-v1') as HTMLInputElement
    const nameInput = within(dialog).getByPlaceholderText('My Matrix') as HTMLInputElement
    expect(idInput.value).toBe('alpha-baseline')
    expect(idInput.disabled).toBe(true)
    expect(nameInput.value).toBe('Alpha Baseline')
  })

  it('confirms and fires useDeleteMatrix.mutate when the user confirms the delete dialog', () => {
    renderWithProviders(<StackVersionsAdminPage />)
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))
    const confirmDialog = screen.getByRole('dialog', { name: /delete compatibility matrix/i })
    fireEvent.click(within(confirmDialog).getByRole('button', { name: /^delete$/i }))
    expect(deleteMutate).toHaveBeenCalledTimes(1)
    expect(deleteMutate).toHaveBeenCalledWith('alpha-baseline', expect.any(Object))
  })

  it('cancelling the delete dialog dismisses the dialog without firing the mutation', () => {
    renderWithProviders(<StackVersionsAdminPage />)
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))
    const confirmDialog = screen.getByRole('dialog', { name: /delete compatibility matrix/i })
    fireEvent.click(within(confirmDialog).getByRole('button', { name: /cancel/i }))
    expect(screen.queryByRole('dialog', { name: /delete compatibility matrix/i })).not.toBeInTheDocument()
    expect(deleteMutate).not.toHaveBeenCalled()
  })

  it('hides the filter bar when there are 5 or fewer matrices', () => {
    // baseline mockMatrices.current is the 3-item `matrices` seed
    renderWithProviders(<StackVersionsAdminPage />)
    expect(screen.queryByLabelText(/Matrix search/i)).not.toBeInTheDocument()
  })

  it('shows the filter bar when there are more than 5 matrices', () => {
    mockMatrices.current = [
      ...matrices,
      { id: 'd-one', name: 'Delta One', status: 'verified', k8sRange: 'v1.29', tools: [] },
      { id: 'e-two', name: 'Echo Two', status: 'untested', k8sRange: 'v1.30', tools: [] },
      { id: 'f-three', name: 'Foxtrot Three', status: 'verified', k8sRange: 'v1.31', tools: [] },
    ]
    renderWithProviders(<StackVersionsAdminPage />)
    expect(screen.getByLabelText(/Matrix search/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Filter by status/i)).toBeInTheDocument()
  })

  it('filters the list by search query', () => {
    mockMatrices.current = [
      ...matrices,
      { id: 'd-one', name: 'Delta One', status: 'verified', k8sRange: 'v1.29', tools: [] },
      { id: 'e-two', name: 'Echo Two', status: 'untested', k8sRange: 'v1.30', tools: [] },
      { id: 'f-three', name: 'Foxtrot Three', status: 'verified', k8sRange: 'v1.31', tools: [] },
    ]
    renderWithProviders(<StackVersionsAdminPage />)
    const search = screen.getByLabelText(/Matrix search/i) as HTMLInputElement
    fireEvent.change(search, { target: { value: 'echo' } })
    // Only "Echo Two" should remain in the list column.
    expect(screen.queryByText('Alpha Baseline')).not.toBeInTheDocument()
    expect(screen.getAllByText('Echo Two').length).toBeGreaterThan(0)
  })
})

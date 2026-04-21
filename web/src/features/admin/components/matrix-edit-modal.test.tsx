import { describe, it, expect, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { screen, fireEvent, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { MatrixEditModal } from './matrix-edit-modal'
import type { CompatibilityMatrix } from '../../../types'

const updateMutate = vi.hoisted(() =>
  vi.fn<(input: unknown, opts: { onSuccess?: (data: unknown) => void; onError?: (err: unknown) => void }) => void>(),
)
const createMutate = vi.hoisted(() => vi.fn())

vi.mock('../../stack/api/stack-api', () => ({
  useCreateMatrix: () => ({ mutate: createMutate, isPending: false }),
  useUpdateMatrix: () => ({ mutate: updateMutate, isPending: false }),
}))

// toolsToRows reads a `category` field via an unsafe cast, so we supply it on
// the seeded tools so canSubmit starts true (at least one row is complete).
const seed: CompatibilityMatrix = {
  id: 'alpha',
  name: 'Alpha',
  status: 'verified',
  k8sRange: 'v1.27-v1.29',
  tools: [
    {
      category: 'cicd',
      name: 'argo-cd',
      helmVersion: '5.55.0',
      appVersion: '2.12',
      archSupport: ['amd64', 'arm64'],
      minK8sVersion: 'v1.27',
      tier: 'stable',
    } as CompatibilityMatrix['tools'][number] & { category: string },
    {
      category: 'cicd',
      name: 'tekton',
      helmVersion: '0.40.0',
      appVersion: '0.52',
      archSupport: ['amd64'],
      minK8sVersion: 'v1.28',
      tier: 'stable',
    } as CompatibilityMatrix['tools'][number] & { category: string },
  ],
}

function renderEdit() {
  const onClose = vi.fn()
  const utils = renderWithProviders(
    <MatrixEditModal open mode="edit" initial={seed} onClose={onClose} />,
  )
  return { ...utils, onClose }
}

function clearCategory(rowIndex: number) {
  const inputs = screen.getAllByPlaceholderText('db') as HTMLInputElement[]
  fireEvent.change(inputs[rowIndex], { target: { value: '' } })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('MatrixEditModal dirty-drop guard', () => {
  it('first Save shows the drop-warning banner and withholds the mutation', () => {
    renderEdit()
    clearCategory(0)
    fireEvent.click(screen.getByRole('button', { name: /^Save$/ }))
    expect(screen.getByTestId('matrix-drop-warn')).toBeInTheDocument()
    expect(updateMutate).not.toHaveBeenCalled()
  })

  it('second Save commits the update after the warning is shown', () => {
    renderEdit()
    clearCategory(0)
    fireEvent.click(screen.getByRole('button', { name: /^Save$/ }))
    fireEvent.click(screen.getByRole('button', { name: /^Save$/ }))
    expect(updateMutate).toHaveBeenCalledTimes(1)
    const payload = updateMutate.mock.calls[0][0] as { tools: Record<string, unknown> }
    // The dropped row has an empty category, so its entry must not appear
    // anywhere in the tools map that reaches the server.
    const toolEntries = Object.values(payload.tools) as Array<{ name?: string }>
    expect(toolEntries.some((t) => t.name === 'argo-cd')).toBe(false)
  })

  it('fixing the empty category auto-dismisses the banner and lets one Save commit', () => {
    renderEdit()
    clearCategory(0)
    fireEvent.click(screen.getByRole('button', { name: /^Save$/ }))
    expect(screen.getByTestId('matrix-drop-warn')).toBeInTheDocument()

    // Refill the category with a non-duplicate value — banner disappears
    // and both the drop gate and the dup guard (F8-UIUX-Polish) reset.
    const inputs = screen.getAllByPlaceholderText('db') as HTMLInputElement[]
    fireEvent.change(inputs[0], { target: { value: 'observability' } })
    expect(screen.queryByTestId('matrix-drop-warn')).not.toBeInTheDocument()
    expect(screen.queryByTestId('matrix-dup-warn')).not.toBeInTheDocument()

    // Now a single Save commits.
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^Save$/ }))
    expect(updateMutate).toHaveBeenCalledTimes(1)
  })
})

describe('MatrixEditModal validation + unsaved guard', () => {
  it('shows an ID format error in create mode and blocks Save', () => {
    const onClose = vi.fn()
    renderWithProviders(<MatrixEditModal open mode="create" onClose={onClose} />)
    const idInput = screen.getByPlaceholderText('my-matrix-v1') as HTMLInputElement
    const nameInput = screen.getByPlaceholderText('My Matrix') as HTMLInputElement
    fireEvent.change(idInput, { target: { value: 'Bad_ID!' } })
    fireEvent.change(nameInput, { target: { value: 'A valid name' } })
    expect(screen.getByText(/ID는 소문자|ID must be lowercase/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Save$/ })).toBeDisabled()
    expect(createMutate).not.toHaveBeenCalled()
  })

  it('shows a name-length error when Name is one character', () => {
    const onClose = vi.fn()
    renderWithProviders(<MatrixEditModal open mode="create" onClose={onClose} />)
    const nameInput = screen.getByPlaceholderText('My Matrix') as HTMLInputElement
    fireEvent.change(nameInput, { target: { value: 'A' } })
    expect(screen.getByText(/이름은 최소 2자|at least 2 characters/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Save$/ })).toBeDisabled()
  })

  it('shows a K8s format warning for a malformed version', () => {
    const onClose = vi.fn()
    renderWithProviders(<MatrixEditModal open mode="create" onClose={onClose} />)
    const k8sMinInput = screen.getByPlaceholderText('v1.27') as HTMLInputElement
    fireEvent.change(k8sMinInput, { target: { value: 'not-a-version' } })
    expect(screen.getAllByText(/v1\.28 또는|Expected format/i).length).toBeGreaterThan(0)
  })

  it('blocks Save and warns when two tool rows share the same category', () => {
    renderEdit()
    // Seed category has both rows set to 'cicd' (see `seed`), so the modal
    // opens already holding a duplicate. Save must be disabled and the
    // dup banner must show.
    expect(screen.getByTestId('matrix-dup-warn')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Save$/ })).toBeDisabled()
    // Rename one row's category — banner disappears, Save enables.
    const categoryInputs = screen.getAllByPlaceholderText('db') as HTMLInputElement[]
    fireEvent.change(categoryInputs[1], { target: { value: 'cicd-secondary' } })
    expect(screen.queryByTestId('matrix-dup-warn')).not.toBeInTheDocument()
  })

  it('prompts to confirm close when the form is dirty and Cancel is clicked', () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const { onClose } = renderEdit()
    clearCategory(0) // make form dirty
    fireEvent.click(screen.getByRole('button', { name: /^Cancel$/ }))
    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(onClose).not.toHaveBeenCalled() // user denied → modal stays open
    confirmSpy.mockReturnValue(true)
    fireEvent.click(screen.getByRole('button', { name: /^Cancel$/ }))
    expect(onClose).toHaveBeenCalledTimes(1)
    confirmSpy.mockRestore()
  })
})

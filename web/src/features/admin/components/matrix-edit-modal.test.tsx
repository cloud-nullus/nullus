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

    // Refill the category — banner disappears and confirm gate resets.
    const inputs = screen.getAllByPlaceholderText('db') as HTMLInputElement[]
    fireEvent.change(inputs[0], { target: { value: 'cicd' } })
    expect(screen.queryByTestId('matrix-drop-warn')).not.toBeInTheDocument()

    // Now a single Save commits.
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^Save$/ }))
    expect(updateMutate).toHaveBeenCalledTimes(1)
  })
})

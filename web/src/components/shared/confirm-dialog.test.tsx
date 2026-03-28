import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { ConfirmDialog } from './confirm-dialog'

describe('ConfirmDialog', () => {
  it('renders without crashing when open', () => {
    const { container } = render(
      <ConfirmDialog
        open
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Delete stack"
        description="This action cannot be undone."
      />
    )

    expect(container).toBeTruthy()
    expect(screen.getByRole('dialog', { name: 'Delete stack' })).not.toBeNull()
    expect(screen.getByText('This action cannot be undone.')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Cancel' })).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Confirm' })).not.toBeNull()
  })

  it('calls onClose when cancel is clicked', () => {
    const onClose = vi.fn()

    render(
      <ConfirmDialog
        open
        onClose={onClose}
        onConfirm={vi.fn()}
        title="Delete stack"
        description="This action cannot be undone."
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('requires matching confirm text before confirming', () => {
    const onConfirm = vi.fn()

    render(
      <ConfirmDialog
        open
        onClose={vi.fn()}
        onConfirm={onConfirm}
        title="Delete stack"
        description="This action cannot be undone."
        confirmText="DELETE"
        confirmLabel="Delete"
      />
    )

    const confirmButton = screen.getByRole('button', { name: 'Delete' })
    const input = screen.getByPlaceholderText('DELETE')

    expect(confirmButton).toBeDisabled()

    fireEvent.change(input, { target: { value: 'DEL' } })
    expect(confirmButton).toBeDisabled()

    fireEvent.change(input, { target: { value: 'DELETE' } })
    expect(confirmButton).not.toBeDisabled()

    fireEvent.click(confirmButton)
    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect((input as HTMLInputElement).value).toBe('')
  })

  it('renders custom content when provided', () => {
    render(
      <ConfirmDialog
        open
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Delete stack"
        description="This action cannot be undone."
        customContent={<p>Custom warning message</p>}
      />
    )

    expect(screen.getAllByText('Custom warning message').length).toBeGreaterThan(0)
  })
})

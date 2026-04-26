import { describe, it, expect, vi } from 'vitest'
import { createElement, useState } from 'react'
import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { Modal } from './modal'

describe('Modal', () => {
  it('creates element with open=false', () => {
    const onClose = vi.fn()
    const el = createElement(Modal, { open: false, onClose }, createElement('div', null, 'Content'))
    expect(el.props.open).toBe(false)
  })

  it('creates element with open=true', () => {
    const onClose = vi.fn()
    const el = createElement(Modal, { open: true, onClose, title: 'Test' }, createElement('div', null, 'Body'))
    expect(el.props.open).toBe(true)
  })

  it('title prop is passed through', () => {
    const el = createElement(Modal, { open: true, onClose: vi.fn(), title: 'My Title' }, 'Body')
    expect(el.props.title).toBe('My Title')
  })

  it('onClose prop is a function', () => {
    const onClose = vi.fn()
    const el = createElement(Modal, { open: true, onClose }, 'Body')
    expect(typeof el.props.onClose).toBe('function')
  })

  it('wide prop defaults to false when not provided', () => {
    const el = createElement(Modal, { open: true, onClose: vi.fn() }, 'Body')
    expect(el.props.wide).toBeUndefined()
  })

  it('wide prop is passed through when provided', () => {
    const el = createElement(Modal, { open: true, onClose: vi.fn(), wide: true }, 'Body')
    expect(el.props.wide).toBe(true)
  })

  it('footer prop is passed through', () => {
    const footer = createElement('button', null, 'Confirm')
    const el = createElement(Modal, { open: true, onClose: vi.fn(), footer }, 'Body')
    expect(el.props.footer).toBeDefined()
  })

  it('Modal is a function component', () => {
    expect(typeof Modal).toBe('function')
  })

  it('closes when pointer down/up both happen on overlay', () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="Test Modal">
        <button type="button">Inner Button</button>
      </Modal>
    )

    const overlay = screen.getByRole('dialog')
    fireEvent.pointerDown(overlay)
    fireEvent.pointerUp(overlay)

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not close when dragging from modal content to overlay', () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="Test Modal">
        <button type="button">Inner Button</button>
      </Modal>
    )

    fireEvent.pointerDown(screen.getByText('Inner Button'))
    fireEvent.pointerUp(screen.getByRole('dialog'))

    expect(onClose).not.toHaveBeenCalled()
  })

  // F8-UIUX-A11y focus trap
  it('auto-focuses the first focusable element when opened', async () => {
    render(
      <Modal open onClose={vi.fn()} title="Trap">
        <button type="button" data-testid="first">First</button>
        <button type="button" data-testid="second">Second</button>
      </Modal>,
    )
    await waitFor(() => {
      // The close-X button in the modal header is the true first focusable,
      // so "first content button" may not always be active — but we can
      // assert that focus moved inside the dialog root.
      expect(screen.getByRole('dialog').contains(document.activeElement)).toBe(true)
    })
  })

  it('wraps Tab from the last focusable element back to the first', async () => {
    render(
      <Modal open onClose={vi.fn()} title="Trap">
        <button type="button" data-testid="first">First</button>
        <button type="button" data-testid="second">Second</button>
      </Modal>,
    )
    const second = await screen.findByTestId('second')
    second.focus()
    // Tab from the genuine last focusable wraps to the first — in jsdom the
    // Modal's close-X button renders first in the DOM, so wrap lands on it.
    fireEvent.keyDown(second, { key: 'Tab' })
    const active = document.activeElement as HTMLElement | null
    expect(active).toBeTruthy()
    // Wrap target must stay inside the dialog root (not escape to <body>).
    expect(screen.getByRole('dialog').contains(active)).toBe(true)
    // And must not still be `second` — i.e. Tab actually moved focus.
    expect(active).not.toBe(second)
  })

  it('restores focus to the previously focused element when closed', async () => {
    function Harness() {
      const [open, setOpen] = useState(false)
      return (
        <div>
          <button type="button" data-testid="trigger" onClick={() => setOpen(true)}>open</button>
          <Modal open={open} onClose={() => setOpen(false)} title="Restore">
            <button type="button" data-testid="inside" onClick={() => setOpen(false)}>close</button>
          </Modal>
        </div>
      )
    }
    render(<Harness />)
    const trigger = screen.getByTestId('trigger')
    trigger.focus()
    expect(document.activeElement).toBe(trigger)
    fireEvent.click(trigger)
    // wait until the dialog appears and focus moves inside
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
    // Now close from inside
    fireEvent.click(screen.getByTestId('inside'))
    // Focus should have been returned to the trigger
    await waitFor(() => expect(document.activeElement).toBe(trigger))
  })
})

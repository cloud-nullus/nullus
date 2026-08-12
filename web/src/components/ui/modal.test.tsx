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

  // 내부가 MUI Dialog 로 바뀌면서 배경 클릭 감지가 pointerDown/pointerUp 에서
  // mouseDown/click 으로 옮겨갔다. 검증하는 동작(배경 클릭은 닫고, 내용에서
  // 시작한 드래그는 닫지 않는다)은 그대로다 — 이벤트 방식만 옮겼다.
  it('closes when mouse down/click both happen on the overlay', () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="Test Modal">
        <button type="button">Inner Button</button>
      </Modal>
    )

    const overlay = screen.getByTestId('modal-overlay')
    fireEvent.mouseDown(overlay)
    fireEvent.click(overlay)

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not close when dragging from modal content to overlay', () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="Test Modal">
        <button type="button">Inner Button</button>
      </Modal>
    )

    // 드래그 시작점이 배경이 아니므로 닫히지 않아야 한다.
    fireEvent.mouseDown(screen.getByText('Inner Button'))
    fireEvent.click(screen.getByTestId('modal-overlay'))

    expect(onClose).not.toHaveBeenCalled()
  })

  it('closes on Escape', () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="Esc">
        <button type="button">Inner</button>
      </Modal>
    )

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })

    expect(onClose).toHaveBeenCalled()
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

  // 이전 구현은 keydown 을 직접 듣고 포커스를 옮겨서 합성 Tab 으로 검증할 수 있었다.
  // MUI 의 트랩은 콘텐츠 앞뒤에 tabindex=0 sentinel 을 두고 브라우저의 실제 Tab
  // 이동을 받아 되돌리는 방식이라, 실제 Tab 키를 구현하지 않는 jsdom 에서는
  // 합성 keydown 으로 재현할 수 없다. 그래서 트랩이 실제로 걸려 있는지를 검증한다 —
  // disableEnforceFocus 같은 설정 실수로 트랩이 사라지면 여기서 걸린다.
  it('installs a focus trap that wraps Tab inside the dialog', () => {
    render(
      <Modal open onClose={vi.fn()} title="Trap">
        <button type="button" data-testid="first">First</button>
        <button type="button" data-testid="second">Second</button>
      </Modal>,
    )

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')

    const root = dialog.closest('.MuiModal-root')
    expect(root).not.toBeNull()
    // Tab 순환을 만드는 경계 요소가 콘텐츠 앞뒤에 있어야 한다.
    expect(root?.querySelector('[data-testid="sentinelStart"]')).not.toBeNull()
    expect(root?.querySelector('[data-testid="sentinelEnd"]')).not.toBeNull()
  })

  it('keeps the title as the dialog accessible name', () => {
    render(
      <Modal open onClose={vi.fn()} title="My Title">
        <button type="button">Inner</button>
      </Modal>,
    )
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-label', 'My Title')
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

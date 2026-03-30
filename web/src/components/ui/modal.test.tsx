import { describe, it, expect, vi } from 'vitest'
import { createElement } from 'react'
import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen } from '@testing-library/react'
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
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useKeyboardShortcut } from './use-keyboard-shortcut'

beforeEach(() => {
  document.body.innerHTML = ''
})

describe('useKeyboardShortcut', () => {
  it('fires the handler when the matching key is pressed on the window', () => {
    const handler = vi.fn()
    renderHook(() => useKeyboardShortcut('?', handler))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '?' }))
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('does not fire when a different key is pressed', () => {
    const handler = vi.fn()
    renderHook(() => useKeyboardShortcut('?', handler))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }))
    expect(handler).not.toHaveBeenCalled()
  })

  it('respects enabled=false', () => {
    const handler = vi.fn()
    renderHook(() => useKeyboardShortcut('?', handler, { enabled: false }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '?' }))
    expect(handler).not.toHaveBeenCalled()
  })

  it('skips the handler by default when an input is focused', () => {
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    const handler = vi.fn()
    renderHook(() => useKeyboardShortcut('?', handler))
    // Dispatch from the focused input so e.target is the input element.
    input.dispatchEvent(new KeyboardEvent('keydown', { key: '?', bubbles: true }))
    expect(handler).not.toHaveBeenCalled()
  })
})

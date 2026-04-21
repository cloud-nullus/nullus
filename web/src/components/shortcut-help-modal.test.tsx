import { describe, it, expect, vi } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { SHORTCUT_REGISTRY, ShortcutHelpModal } from './shortcut-help-modal'

describe('ShortcutHelpModal', () => {
  it('renders every registered shortcut key as a row', () => {
    render(<ShortcutHelpModal open onClose={vi.fn()} />)
    // Each registered shortcut key should appear in a kbd element.
    for (const entry of SHORTCUT_REGISTRY) {
      expect(screen.getByText(entry.keys)).toBeInTheDocument()
    }
    // Sanity — the registry is not empty and the table lives inside the
    // expected test-id container.
    expect(SHORTCUT_REGISTRY.length).toBeGreaterThan(0)
    expect(screen.getByTestId('shortcut-help')).toBeInTheDocument()
  })
})

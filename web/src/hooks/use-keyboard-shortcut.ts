import { useEffect } from 'react'

// F8-UIUX-KeyboardHints — a deliberately minimal global shortcut hook.
// Single-key bindings only (no chord / sequence support) so the mental
// model stays narrow. Handlers are skipped while the user is typing in an
// input/textarea/contenteditable by default so shortcut keys don't fight
// with real text entry.

export interface ShortcutOpts {
  enabled?: boolean
  allowInInput?: boolean
}

export function useKeyboardShortcut(
  key: string,
  handler: (e: KeyboardEvent) => void,
  opts: ShortcutOpts = {},
): void {
  const { enabled = true, allowInInput = false } = opts
  useEffect(() => {
    if (!enabled) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== key) return
      const target = e.target as HTMLElement | null
      if (!allowInInput && target) {
        const tag = target.tagName
        const isInput = tag === 'INPUT' || tag === 'TEXTAREA' || target.isContentEditable
        if (isInput) return
      }
      handler(e)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [key, handler, enabled, allowInInput])
}

import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Sidebar } from '../components/layout/sidebar'
import { Header } from '../components/layout/header'
import { ErrorBoundary } from '../components/shared/error-boundary'
import { ShortcutHelpModal } from '../components/shortcut-help-modal'
import { useKeyboardShortcut } from '../hooks/use-keyboard-shortcut'

export function AppLayout() {
  const { t } = useTranslation()
  const [helpOpen, setHelpOpen] = useState(false)
  // F8-UIUX-KeyboardHints — single global `?` binding opens the help modal.
  useKeyboardShortcut('?', () => setHelpOpen(true))

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Header />
        <main className="flex-1 overflow-y-auto px-[var(--page-padding)] py-8">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
      <button
        type="button"
        onClick={() => setHelpOpen(true)}
        className="fixed bottom-4 right-4 z-40 rounded-full border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-3 py-1.5 text-[11px] text-[var(--color-text-secondary)] shadow hover:text-[var(--color-text-primary)]"
        data-testid="shortcut-badge"
        aria-label={t('shortcuts.help.title', '키보드 단축키')}
      >
        ? {t('shortcuts.badge', '단축키')}
      </button>
      <ShortcutHelpModal open={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  )
}

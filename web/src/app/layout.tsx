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
    // 껍데기는 뷰포트 높이에 못박고, 스크롤은 <main> 하나만 한다.
    //
    // min-h-screen 이면 내용이 길어질 때 문서 전체가 늘어나 같이 스크롤된다 —
    // 사이드바가 화면 밖으로 밀려 올라가고, 맨 아래 로그아웃 줄은 뷰포트가
    // 아니라 문서 끝에 붙어 목록이 긴 화면에서는 보이지 않았다. 사이드바 nav 의
    // overflow-y-auto 도 높이 상한이 없어 한 번도 걸린 적이 없다.
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      {/* min-h-0: flex 자식의 기본 min-height 는 auto 라 내용보다 작아지지
          않는다 — 이게 없으면 아래 main 의 overflow-y-auto 가 안 걸린다. */}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <Header />
        <main className="flex-1 overflow-y-auto px-[var(--page-padding)] py-[var(--page-padding-y)]">
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

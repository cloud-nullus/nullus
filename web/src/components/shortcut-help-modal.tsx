import { useTranslation } from 'react-i18next'
import { Modal } from './ui/modal'

// F8-UIUX-KeyboardHints — tiny central registry of user-visible keyboard
// shortcuts. Adding a new shortcut = adding a row here + calling
// useKeyboardShortcut in the relevant component. Keeps the help table in
// lockstep with actual bindings without needing a runtime registry.

export interface ShortcutEntry {
  keys: string
  descriptionKey: string
  descriptionFallback: string
  scopeKey: string
  scopeFallback: string
}

export const SHORTCUT_REGISTRY: ShortcutEntry[] = [
  {
    keys: '?',
    descriptionKey: 'shortcuts.help.description',
    descriptionFallback: '단축키 도움말',
    scopeKey: 'shortcuts.scope.global',
    scopeFallback: '전역',
  },
  {
    keys: 'n',
    descriptionKey: 'shortcuts.newStack.description',
    descriptionFallback: '새 스택 생성',
    scopeKey: 'shortcuts.scope.stackList',
    scopeFallback: '스택 목록',
  },
]

interface ShortcutHelpModalProps {
  open: boolean
  onClose: () => void
}

export function ShortcutHelpModal({ open, onClose }: ShortcutHelpModalProps) {
  const { t } = useTranslation()
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t('shortcuts.help.title', '키보드 단축키')}
    >
      <div data-testid="shortcut-help">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-[var(--color-text-secondary)]">
              <th className="w-24 py-2 font-medium">
                {t('shortcuts.help.keys', '키')}
              </th>
              <th className="py-2 font-medium">
                {t('shortcuts.help.action', '동작')}
              </th>
              <th className="py-2 font-medium">
                {t('shortcuts.help.scope', '범위')}
              </th>
            </tr>
          </thead>
          <tbody>
            {SHORTCUT_REGISTRY.map((entry) => (
              <tr key={entry.keys} className="border-t border-[var(--color-border-default)]">
                <td className="py-2">
                  <kbd className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-0.5 font-mono text-xs">
                    {entry.keys}
                  </kbd>
                </td>
                <td className="py-2">{t(entry.descriptionKey, entry.descriptionFallback)}</td>
                <td className="py-2 text-[var(--color-text-secondary)]">
                  {t(entry.scopeKey, entry.scopeFallback)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Modal>
  )
}

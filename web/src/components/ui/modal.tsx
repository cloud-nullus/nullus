import { type ReactNode, useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { cn } from '../../lib/utils'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  wide?: boolean
  footer?: ReactNode
}

export function Modal({ open, onClose, title, children, wide = false, footer }: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const cardRef = useRef<HTMLDivElement>(null)
  const pointerDownOnOverlayRef = useRef(false)

  useEffect(() => {
    if (!open) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  // F8-UIUX-A11y — focus trap: capture the previously focused element on
  // open, focus the first focusable inside the modal, cycle Tab/Shift+Tab
  // between first/last focusable, and restore focus to the previous element
  // on close. Keeps keyboard users from escaping the modal accidentally.
  useEffect(() => {
    if (!open) return
    const root = cardRef.current
    if (!root) return
    const prev = (typeof document !== 'undefined' ? document.activeElement : null) as HTMLElement | null

    const getFocusable = () =>
      Array.from(
        root.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
        ),
      ).filter((el) => !el.hasAttribute('disabled') && el.tabIndex !== -1)

    // Defer to after mount so conditional children render first.
    const raf = requestAnimationFrame(() => {
      const first = getFocusable()[0]
      first?.focus()
    })

    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return
      const items = getFocusable()
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      const active = document.activeElement as HTMLElement | null
      if (e.shiftKey && active === first) {
        last.focus()
        e.preventDefault()
      } else if (!e.shiftKey && active === last) {
        first.focus()
        e.preventDefault()
      }
    }
    root.addEventListener('keydown', onKey)
    return () => {
      cancelAnimationFrame(raf)
      root.removeEventListener('keydown', onKey)
      prev?.focus?.()
    }
  }, [open])

  if (!open) return null

  return (
    <div
      ref={overlayRef}
      role="dialog"
      aria-modal="true"
      aria-label={title}
      tabIndex={-1}
      onPointerDown={(e) => {
        pointerDownOnOverlayRef.current = e.target === overlayRef.current
      }}
      onPointerUp={(e) => {
        const releasedOnOverlay = e.target === overlayRef.current
        if (pointerDownOnOverlayRef.current && releasedOnOverlay) {
          onClose()
        }
        pointerDownOnOverlayRef.current = false
      }}
      onPointerCancel={() => {
        pointerDownOnOverlayRef.current = false
      }}
      onKeyDown={(e) => {
        if ((e.key === 'Enter' || e.key === ' ') && e.target === overlayRef.current) {
          onClose()
        }
      }}
      className="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--color-surface-overlay)] p-4"
    >
      <div
        ref={cardRef}
        className={cn(
          'flex max-h-[90vh] w-full flex-col overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)]',
          wide ? 'max-w-[980px]' : 'max-w-[480px]'
        )}
      >
        {title && (
          <div className="shrink-0 border-b border-[var(--color-border-default)] px-5 py-[18px]">
            <div className="flex items-center justify-between">
              <h2 className="m-0 break-keep text-base font-bold text-[var(--color-text-primary)]">{title}</h2>
              <button
                type="button"
                onClick={onClose}
                aria-label="Close modal"
                className="flex cursor-pointer rounded-md p-1 text-[var(--color-text-secondary)]"
              >
                <X size={18} />
              </button>
            </div>
          </div>
        )}

        <div className="flex-1 overflow-y-auto p-5">
          {children}
        </div>

        {footer && (
          <div className="flex shrink-0 justify-end gap-2.5 border-t border-[var(--color-border-default)] px-5 py-4">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}

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
  const pointerDownOnOverlayRef = useRef(false)

  useEffect(() => {
    if (!open) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

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
        className={cn(
          'flex max-h-[90vh] w-full flex-col overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)]',
          wide ? 'max-w-[800px]' : 'max-w-[480px]'
        )}
      >
        {title && (
          <div className="shrink-0 border-b border-[var(--color-border-default)] px-5 py-[18px]">
            <div className="flex items-center justify-between">
              <h2 className="m-0 text-base font-bold text-[var(--color-text-primary)]">{title}</h2>
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

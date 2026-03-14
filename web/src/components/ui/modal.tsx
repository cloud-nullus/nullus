import { type ReactNode, useEffect, useRef } from 'react'
import { X } from 'lucide-react'

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
      onClick={(e) => {
        if (e.target === overlayRef.current) onClose()
      }}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'var(--color-surface-overlay)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 'var(--z-modal)',
        padding: '16px',
      }}
    >
      <div
        style={{
          background: 'var(--color-surface-card)',
          border: '1px solid var(--color-border-default)',
          borderRadius: '12px',
          width: '100%',
          maxWidth: wide ? '800px' : '480px',
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {title && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '18px 20px',
              borderBottom: '1px solid var(--color-border-default)',
              flexShrink: 0,
            }}
          >
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
              {title}
            </h2>
            <button
              onClick={onClose}
              aria-label="Close modal"
              style={{
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--color-text-secondary)',
                display: 'flex',
                padding: '4px',
                borderRadius: '6px',
              }}
            >
              <X size={18} />
            </button>
          </div>
        )}

        <div style={{ flex: 1, overflowY: 'auto', padding: '20px' }}>
          {children}
        </div>

        {footer && (
          <div
            style={{
              padding: '16px 20px',
              borderTop: '1px solid var(--color-border-default)',
              display: 'flex',
              gap: '10px',
              justifyContent: 'flex-end',
              flexShrink: 0,
            }}
          >
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}

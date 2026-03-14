import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Modal } from '../ui/modal'
import { Button } from '../ui/button'

interface ConfirmDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: string
  confirmLabel?: string
  confirmText?: string
  loading?: boolean
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel = 'Confirm',
  confirmText,
  loading = false,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState('')

  const canConfirm = confirmText ? typed === confirmText : true

  const handleClose = () => {
    setTyped('')
    onClose()
  }

  const handleConfirm = () => {
    if (!canConfirm) return
    onConfirm()
    setTyped('')
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={title}
      footer={
        <>
          <Button variant="outline" size="md" onClick={handleClose} disabled={loading}>
            Cancel
          </Button>
          <Button
            variant="danger"
            size="md"
            onClick={handleConfirm}
            disabled={!canConfirm || loading}
            loading={loading}
            style={{
              background: canConfirm ? 'linear-gradient(135deg, #ef4444, #dc2626)' : undefined,
              color: canConfirm ? '#fff' : undefined,
            }}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
          <div
            style={{
              width: '40px',
              height: '40px',
              borderRadius: '10px',
              background: 'rgba(239,68,68,0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
              color: '#f87171',
            }}
          >
            <AlertTriangle size={20} />
          </div>
          <p style={{ margin: 0, fontSize: '14px', color: 'var(--color-text-secondary)', lineHeight: 1.6 }}>
            {description}
          </p>
        </div>

        {confirmText && (
          <div>
            <p style={{ margin: '0 0 8px', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
              확인하려면{' '}
              <code
                style={{
                  background: 'rgba(255,255,255,0.08)',
                  padding: '2px 6px',
                  borderRadius: '4px',
                  fontFamily: 'Fira Code, monospace',
                  fontSize: '12px',
                  color: '#f87171',
                }}
              >
                {confirmText}
              </code>
              을(를) 입력하세요.
            </p>
            <input
              type="text"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder={confirmText}
              style={{
                width: '100%',
                background: 'rgba(255,255,255,0.04)',
                border: `1px solid ${typed === confirmText ? 'rgba(239,68,68,0.5)' : 'var(--color-border-default)'}`,
                borderRadius: '8px',
                padding: '9px 12px',
                fontSize: '14px',
                color: 'var(--color-text-primary)',
                outline: 'none',
                boxSizing: 'border-box',
                fontFamily: 'Fira Code, monospace',
              }}
            />
          </div>
        )}
      </div>
    </Modal>
  )
}

import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Modal } from '../ui/modal'
import { Button } from '../ui/button'
import { cn } from '../../lib/utils'

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
            className={cn(canConfirm && 'bg-[linear-gradient(135deg,#ef4444,#dc2626)] text-white')}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[rgba(239,68,68,0.15)] text-[#f87171]">
            <AlertTriangle size={20} />
          </div>
          <p className="m-0 text-sm leading-[1.6] text-[var(--color-text-secondary)]">
            {description}
          </p>
        </div>

        {confirmText && (
          <div>
            <p className="mb-2 mt-0 text-[13px] text-[var(--color-text-secondary)]">
              확인하려면{' '}
              <code className="rounded bg-[rgba(255,255,255,0.08)] px-1.5 py-0.5 font-mono text-xs text-[#f87171]">
                {confirmText}
              </code>
              을(를) 입력하세요.
            </p>
            <input
              type="text"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder={confirmText}
              className={cn(
                'box-border w-full rounded-lg border bg-[rgba(255,255,255,0.04)] px-3 py-[9px] font-mono text-sm text-[var(--color-text-primary)] outline-none',
                typed === confirmText
                  ? 'border-[rgba(239,68,68,0.5)]'
                  : 'border-[var(--color-border-default)]'
              )}
            />
          </div>
        )}
      </div>
    </Modal>
  )
}

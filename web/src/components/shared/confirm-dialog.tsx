import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Modal } from '../ui/modal'
import { Button } from '../ui/button'
import { cn } from '../../lib/utils'
import { TextInput } from '../ui/text-input'

interface ConfirmDialogProps {
   open: boolean
   onClose: () => void
   onConfirm: () => void
   title: string
   description: string
   confirmLabel?: string
   confirmText?: string
   loading?: boolean
   customContent?: React.ReactNode
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
   customContent,
 }: ConfirmDialogProps) {
  const { t } = useTranslation()
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
            {t('common.cancel', 'Cancel')}
          </Button>
          <Button
            variant="danger"
            size="md"
            onClick={handleConfirm}
            disabled={!canConfirm || loading}
            loading={loading}
            className={cn(canConfirm && 'bg-[linear-gradient(135deg,var(--color-error),var(--color-error))] text-white')}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]">
            <AlertTriangle size={20} />
          </div>
          <p className="m-0 text-sm leading-[1.6] text-[var(--color-text-secondary)]">
            {description}
          </p>
        </div>

        {customContent && customContent}

         {confirmText && (
           <div>
             <p className="mb-2 mt-0 text-[13px] text-[var(--color-text-secondary)]">
               {t('confirmDialog.typeToConfirm.prefix', 'To confirm, type')}{' '}
               <code className="rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-1.5 py-0.5 font-mono text-xs text-[var(--color-error)]">
                 {confirmText}
               </code>
               {' '}
               {t('confirmDialog.typeToConfirm.suffix', 'exactly.')}
             </p>
             <TextInput
               type="text"
               value={typed}
               onChange={(e) => setTyped(e.target.value)}
               placeholder={confirmText}
               className={cn(
                 'w-full font-mono',
                 // 확인 문구가 맞으면 테두리로 알린다. 값이 틀린 게 아니라 맞은
                 // 상태를 강조하는 자리라 invalid 가 아니라 색만 바꾼다.
                 typed === confirmText &&
                   'border-[color-mix(in_srgb,_var(--color-error)_50%,_transparent)]',
               )}
             />
           </div>
         )}
      </div>
    </Modal>
  )
}

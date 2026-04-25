import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { Loader2, RotateCcw } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { useRetryStack } from '../api/stack-api'
import { canRetry, type StackStatus } from '../utils/retry-policy'
import { extractDeployCompatError } from '../utils/deploy-error'

// RetryStackButton — F8-Phase3 follow-up UI. Renders only when the stack's
// current state allows retry (failed / rolled_back). Handles the backend's
// warn-ack contract by opening an inline Modal on DEPLOY_COMPAT_WARN_UNACK
// that lets the user acknowledge the issues and re-submit with the flag.
// Non-warn outcomes are surfaced through sonner toasts so feedback shows up
// regardless of which list row is open.

interface RetryStackButtonProps {
  stackId: string
  status: StackStatus
  onRetried?: (stackId: string) => void
}

interface WarnPromptState {
  open: boolean
  issueLines: string[]
}

export function RetryStackButton({ stackId, status, onRetried }: RetryStackButtonProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const retry = useRetryStack()
  const [warnPrompt, setWarnPrompt] = useState<WarnPromptState>({ open: false, issueLines: [] })
  const [ack, setAck] = useState(false)

  if (!canRetry(status)) {
    return null
  }

  const runRetry = (acknowledgeWarnings: boolean) => {
    // F8-UIUX-RetryFeedback — progressive toast. A loading toast is
    // issued up-front and then updated in place on success/failure via the
    // returned id. warn-ack path dismisses the loading toast and falls back
    // to the modal.
    const toastId = toast.loading(
      t('stackList.retry.toasts.pending', '재배포 요청 중...'),
    )
    retry.mutate(
      { stackId, acknowledgeWarnings },
      {
        onSuccess: () => {
          setWarnPrompt({ open: false, issueLines: [] })
          setAck(false)
          toast.success(t('stackList.retry.toasts.success', 'Redeploy started.'), { id: toastId })
          onRetried?.(stackId)
        },
        onError: (err) => {
          const gate = extractDeployCompatError(err)
          if (gate?.code === 'DEPLOY_COMPAT_WARN_UNACK') {
            // User is about to decide in the ack modal; drop the loading toast.
            toast.dismiss(toastId)
            setWarnPrompt({ open: true, issueLines: gate.issueLines })
            return
          }
          const failureBase = t('stackList.retry.toasts.failure', 'Redeploy failed.')
          if (gate?.code === 'DEPLOY_COMPAT_FAIL') {
            const detail = gate.issueLines.length > 0 ? ' — ' + gate.issueLines.join('; ') : ''
            toast.error(failureBase + detail, {
              id: toastId,
              action: {
                label: t('stackList.retry.toasts.fixAction', '수정하기'),
                onClick: () => navigate(`/stack/install?stackId=${stackId}`),
              },
            })
            return
          }
          const message = (err as { message?: string })?.message
          toast.error(message ? failureBase + ' — ' + message : failureBase, { id: toastId })
        },
      },
    )
  }

  return (
    <>
      <Button
        size="sm"
        variant="outline"
        onClick={() => runRetry(false)}
        disabled={retry.isPending}
        aria-label={t('stackList.retry.button', 'Retry')}
        data-testid="retry-stack-button"
      >
        {retry.isPending ? (
          <Loader2 size={12} className="mr-1 animate-spin" />
        ) : (
          <RotateCcw size={12} className="mr-1" />
        )}
        {t('stackList.retry.button', 'Retry')}
      </Button>

      <Modal
        open={warnPrompt.open}
        onClose={() => {
          setWarnPrompt({ open: false, issueLines: [] })
          setAck(false)
        }}
        title={t('stackList.retry.confirmWarn.title', '호환성 경고 확인')}
        footer={
          <div className="flex items-center justify-end gap-2">
            <Button
              variant="outline"
              onClick={() => {
                setWarnPrompt({ open: false, issueLines: [] })
                setAck(false)
              }}
              disabled={retry.isPending}
            >
              {t('stackList.retry.confirmWarn.cancel', 'Cancel')}
            </Button>
            <Button
              onClick={() => runRetry(true)}
              disabled={!ack || retry.isPending}
              data-testid="retry-warn-confirm"
            >
              {t('stackList.retry.confirmWarn.confirm', 'Acknowledge and retry')}
            </Button>
          </div>
        }
      >
        <div className="flex flex-col gap-3 text-sm">
          <p className="text-[var(--color-text-secondary)]">
            {t(
              'stackList.retry.confirmWarn.description',
              '서버 호환성 검증에서 warn 이 감지되었습니다. 아래 항목을 확인한 뒤 재시도를 진행해 주세요.',
            )}
          </p>
          {warnPrompt.issueLines.length > 0 && (
            <ul className="rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-[11px]">
              {warnPrompt.issueLines.map((line, idx) => (
                <li key={idx} className="list-disc pl-4">
                  {line}
                </li>
              ))}
            </ul>
          )}
          <label className="inline-flex items-center gap-2 text-[11px] text-[var(--color-text-secondary)]">
            <input
              type="checkbox"
              checked={ack}
              onChange={(e) => setAck(e.target.checked)}
              data-testid="retry-warn-ack"
            />
            {t(
              'stackList.retry.confirmWarn.ackLabel',
              '위험을 감수하고 재배포를 진행합니다.',
            )}
          </label>
        </div>
      </Modal>
    </>
  )
}

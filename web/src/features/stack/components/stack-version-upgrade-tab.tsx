import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ArrowUpCircle, ExternalLink, GitBranch } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { StatusIcon, type StatusTone } from '../../../components/ui/status-icon'
import { textInputClass } from '../../../components/ui/text-input'
import { TOOL_BRAND_GRADIENT } from '../../../lib/tool-brand-colors'
import { cn } from '../../../lib/utils'
import {
  createIdempotencyKey,
  isUpgradeRunActive,
  usePreflightUpgrade,
  useStackUpgrades,
  useStartUpgrade,
} from '../api/stack-api'
import type { UpgradeBundle, UpgradeCandidate, UpgradeCheck, UpgradeRun } from '../api/stack-api-types'

// 설치된 OSS 업그레이드 탭. 검증된 번들(실클러스터 리허설을 거친 경로)만 실행할 수 있다.
// 실행은 서버에서 이어지므로 이 화면은 마지막 실행(last_run)을 보여 주기만 한다.

const TOOL_LABEL: Record<string, string> = { argocd: 'Argo CD' }

const RISK_STYLE: Record<UpgradeBundle['risk_level'], string> = {
  low: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]',
  medium: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]',
  high: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]',
}

const CHECK_TONE: Record<UpgradeCheck['status'], StatusTone> = { pass: 'success', warning: 'warning', blocked: 'error' }

const RUN_TONE: Record<UpgradeRun['status'], StatusTone> = {
  upgrading: 'running',
  rolling_back: 'running',
  succeeded: 'success',
  rolled_back: 'warning',
  manual_intervention_required: 'error',
  blocked: 'neutral',
}

const isLink = (value?: string) => !!value && /^https?:\/\//.test(value)

function versionLabel(chart: string, app?: string) {
  return app ? `${chart} (${app})` : chart
}

export function StackVersionUpgradeTab({ stackId }: { stackId: string }) {
  const { t } = useTranslation()
  const upgrades = useStackUpgrades(stackId)
  const [selected, setSelected] = useState<UpgradeCandidate | null>(null)
  const items = upgrades.data?.items ?? []
  const available = items.filter((item) => item.bundle && !item.blocked_reason).length

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center gap-3">
        <div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,var(--color-primary),var(--color-accent-alt))]" />
        <h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
          {t('stackList.upgrade.title')}
        </h3>
        <span className="rounded-full bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] px-2.5 py-0.5 text-[12px] font-semibold text-[var(--color-primary)]">
          {t('stackList.upgrade.available', { count: available })}
        </span>
      </div>

      {upgrades.isLoading && (
        <div className="mb-3 text-[13px] text-[var(--color-text-secondary)]">{t('stackList.upgrade.loading')}</div>
      )}
      {upgrades.isError && (
        <div className="mb-3 rounded-lg border border-[var(--color-error)] p-4 text-[13px] text-[var(--color-error)]">
          {t('stackList.upgrade.loadError')}
        </div>
      )}
      {!upgrades.isLoading && !upgrades.isError && items.length === 0 && (
        <div className="mb-3 rounded-lg border border-[var(--color-border-default)] p-4 text-[13px] text-[var(--color-text-secondary)]">
          {t('stackList.upgrade.empty')}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {items.map((item) => (
          <UpgradeRow key={item.release_name} item={item} onUpgrade={() => setSelected(item)} />
        ))}
      </div>

      {selected?.bundle && (
        <UpgradeDialog
          stackId={stackId}
          candidate={selected}
          bundle={selected.bundle}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  )
}

function UpgradeRow({ item, onUpgrade }: { item: UpgradeCandidate; onUpgrade: () => void }) {
  const { t } = useTranslation()
  const bundle = item.bundle
  const running = isUpgradeRunActive(item.last_run)

  return (
    <div
      className={cn(
        'flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-4',
        !bundle && 'opacity-80',
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style={{ background: TOOL_BRAND_GRADIENT.kubernetes }}>
          <GitBranch {...iconProps('sm')} className="text-white" />
        </div>
        <div className="min-w-0">
          <div className="font-semibold text-[var(--color-text-primary)]">{TOOL_LABEL[item.tool] ?? item.tool}</div>
          <div className="text-[12px] text-[var(--color-text-secondary)]">
            {t('stackList.upgrade.current')}: {versionLabel(item.current_chart_version, item.current_app_version)}
            {' → '}
            {bundle ? (
              <strong className="text-[var(--color-success)]">
                {versionLabel(bundle.target_chart_version, bundle.target_app_version)}
              </strong>
            ) : (
              <strong className="text-[var(--color-text-secondary)]">{t('stackList.upgrade.noVerifiedPath')}</strong>
            )}
          </div>
          {!bundle && (
            <div className="text-[12px] text-[var(--color-text-secondary)]">{t('stackList.upgrade.noVerifiedPathHint')}</div>
          )}
          {item.blocked_reason && <div className="text-[12px] text-[var(--color-error)]">{item.blocked_reason}</div>}
          {item.last_run && <LastRunLine run={item.last_run} />}
        </div>
      </div>
      {bundle && (
        <div className="flex items-center gap-2.5">
          <span className={cn('rounded-full px-2.5 py-0.5 text-[11px] font-semibold', RISK_STYLE[bundle.risk_level])}>
            {t(`stackList.upgrade.risk.${bundle.risk_level}`)}
          </span>
          {isLink(bundle.release_notes) && (
            <a
              href={bundle.release_notes}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-[12px] text-[var(--color-primary)] hover:underline"
            >
              <ExternalLink {...iconProps('xs')} />
              {t('stackList.upgrade.releaseNotes')}
            </a>
          )}
          <Button variant="primary" size="sm" disabled={!!item.blocked_reason || running} onClick={onUpgrade}>
            <ArrowUpCircle {...iconProps('xs')} />
            {t('stackList.upgrade.action')}
          </Button>
        </div>
      )}
    </div>
  )
}

function LastRunLine({ run }: { run: UpgradeRun }) {
  const { t } = useTranslation()
  const step = t(`stackList.upgrade.steps.${run.current_step}`, { defaultValue: run.current_step })
  const label = t(`stackList.upgrade.run.${run.status}`, {
    step,
    from: run.source_chart_version,
    to: run.target_chart_version,
  })
  return (
    <div className="mt-1 flex items-start gap-1.5 text-[12px] text-[var(--color-text-secondary)]">
      <StatusIcon tone={RUN_TONE[run.status]} size="xs" />
      <span>
        {label}
        {run.error && run.status !== 'succeeded' && <span className="text-[var(--color-error)]"> — {run.error}</span>}
      </span>
    </div>
  )
}

function UpgradeDialog({
  stackId,
  candidate,
  bundle,
  onClose,
}: {
  stackId: string
  candidate: UpgradeCandidate
  bundle: UpgradeBundle
  onClose: () => void
}) {
  const { t } = useTranslation()
  const preflight = usePreflightUpgrade()
  const startUpgrade = useStartUpgrade()
  // 대화 상자 하나 = 요청 하나. 재시도(네트워크 오류 등)에도 같은 키를 보내 서버가 실행을 하나만 만든다.
  const [idempotencyKey] = useState(createIdempotencyKey)
  const [checks, setChecks] = useState<UpgradeCheck[] | null>(null)
  const [preflightError, setPreflightError] = useState('')
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [startError, setStartError] = useState('')

  // 열 때 한 번 점검한다. StrictMode 의 이중 실행에도 한 번만 부르도록 ref 로 막는다.
  const runPreflight = preflight.mutateAsync
  const preflightRan = useRef(false)
  useEffect(() => {
    if (preflightRan.current) return
    preflightRan.current = true
    runPreflight({ stackId, bundleId: bundle.id })
      .then((result) => setChecks(result.checks))
      .catch((error: unknown) => setPreflightError(errorMessage(error)))
  }, [runPreflight, stackId, bundle.id])

  const blocked = checks?.some((check) => check.status === 'blocked') ?? false
  const canStart = checks !== null && !blocked && reason.trim() !== '' && !submitting

  const handleStart = async () => {
    setSubmitting(true)
    setStartError('')
    try {
      const run = await startUpgrade.mutateAsync({ stackId, bundleId: bundle.id, reason: reason.trim(), idempotencyKey })
      if (run.status === 'blocked') {
        setChecks(run.checks)
        return
      }
      toast.success(t('stackList.upgrade.started'))
      onClose()
    } catch (error) {
      setStartError(errorMessage(error) || t('stackList.upgrade.failed'))
    } finally {
      setSubmitting(false)
    }
  }

  const tool = TOOL_LABEL[candidate.tool] ?? candidate.tool
  return (
    <Modal
      open
      onClose={onClose}
      title={t('stackList.upgrade.dialogTitle', { tool })}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            {t('common.cancel', 'Cancel')}
          </Button>
          <Button variant="primary" disabled={!canStart} loading={submitting} onClick={() => void handleStart()}>
            {t('stackList.upgrade.start')}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4 text-[13px]">
        <div className="text-[var(--color-text-primary)]">
          {versionLabel(candidate.current_chart_version, candidate.current_app_version)}
          {' → '}
          <strong>{versionLabel(bundle.target_chart_version, bundle.target_app_version)}</strong>
        </div>
        {bundle.expected_disruption && (
          <div>
            <div className="text-[12px] font-semibold text-[var(--color-text-secondary)]">{t('stackList.upgrade.disruption')}</div>
            <div className="text-[var(--color-text-primary)]">{bundle.expected_disruption}</div>
          </div>
        )}
        {bundle.release_notes && !isLink(bundle.release_notes) && (
          <div className="whitespace-pre-wrap text-[var(--color-text-secondary)]">{bundle.release_notes}</div>
        )}

        <div>
          <div className="mb-1.5 text-[12px] font-semibold text-[var(--color-text-secondary)]">{t('stackList.upgrade.preflightTitle')}</div>
          {checks === null && !preflightError && (
            <div className="text-[var(--color-text-secondary)]">{t('stackList.upgrade.preflightRunning')}</div>
          )}
          {preflightError && (
            <div className="text-[var(--color-error)]">
              {t('stackList.upgrade.preflightFailed')} {preflightError}
            </div>
          )}
          {checks && (
            <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
              {checks.map((check) => (
                <li key={check.name} className="flex items-start gap-2">
                  <StatusIcon tone={CHECK_TONE[check.status]} size="xs" />
                  <div className="min-w-0">
                    <span className="font-semibold text-[var(--color-text-primary)]">
                      {t(`stackList.upgrade.checks.${check.name}`, { defaultValue: check.name })}
                    </span>
                    <div className="text-[var(--color-text-secondary)]">{check.message}</div>
                    {check.status !== 'pass' && check.remediation && (
                      <div className="text-[var(--color-warning)]">{check.remediation}</div>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        <label className="flex flex-col gap-1">
          <span className="text-[12px] font-semibold text-[var(--color-text-secondary)]">{t('stackList.upgrade.reasonLabel')}</span>
          <textarea
            rows={2}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={t('stackList.upgrade.reasonPlaceholder')}
            className={textInputClass('h-auto py-1.5')}
          />
        </label>
        {startError && <div className="text-[var(--color-error)]">{startError}</div>}
      </div>
    </Modal>
  )
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'object' && error !== null && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return ''
}

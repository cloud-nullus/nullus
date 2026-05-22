import { useMemo, useState } from 'react'
import { KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  useApproveTokenSource,
  usePauseTokenSource,
  useReAuthTokenSource,
  useResumeTokenSource,
  useRevealTokenSource,
  useRotateTokenSource,
  useTokenSourceEvents,
  useTokenSources,
  type TokenSource,
} from '../api/admin-api'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'

const STEP_UP_ENFORCED = false

const STATUS_BADGE: Record<string, string> = {
  healthy: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  renew_due: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]',
  renewing: 'bg-[rgba(99,102,241,0.15)] text-[#a5b4fc]',
  rotated: 'bg-[rgba(34,197,94,0.15)] text-[#34d399]',
  failed_retryable: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  failed_manual: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  expired: 'bg-[rgba(100,116,139,0.15)] text-[#94a3b8]',
}

export function TokenManagementPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useTokenSources()
  const items = useMemo(() => data?.items ?? [], [data?.items])
  const [selectedId, setSelectedId] = useState('')
  const [revealResult, setRevealResult] = useState<Record<string, unknown> | null>(null)

  const selected = useMemo<TokenSource | null>(() => {
    if (items.length === 0) return null
    if (!selectedId) return items[0]
    return items.find((item) => item.id === selectedId) ?? items[0]
  }, [items, selectedId])

  const sourceId = selected?.id ?? ''
  const { data: eventsData } = useTokenSourceEvents(sourceId)
  const events = eventsData?.items ?? []

  const rotate = useRotateTokenSource()
  const approve = useApproveTokenSource()
  const pause = usePauseTokenSource()
  const resume = useResumeTokenSource()
  const reAuth = useReAuthTokenSource()
  const reveal = useRevealTokenSource()

  const runAction = (action: 'rotate' | 'approve' | 'pause' | 'resume') => {
    if (!sourceId) return
    const reason = 'manual admin action'
    if (action === 'rotate') rotate.mutate({ id: sourceId, reason })
    if (action === 'approve') approve.mutate({ id: sourceId, reason })
    if (action === 'pause') pause.mutate({ id: sourceId, reason })
    if (action === 'resume') resume.mutate({ id: sourceId, reason })
  }

  const handleReveal = async () => {
    if (!sourceId) return
    try {
      let token = ''
      if (!STEP_UP_ENFORCED) {
        const reAuthResp = await reAuth.mutateAsync({ id: sourceId, reason: 'test-mode auto step-up' })
        token = reAuthResp.step_up_token
      }

      if (STEP_UP_ENFORCED) {
        return
      }

      const resp = await reveal.mutateAsync({ id: sourceId, stepUpToken: token })
      setRevealResult(resp as Record<string, unknown>)
    } catch {
      setRevealResult({ error: 'reveal failed' })
    }
  }

  return (
    <div>
      <Breadcrumb items={[{ label: t('sidebar.admin', 'Admin') }, { label: 'OpenBao Token Management' }]} />

      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(168,85,247,0.15)] text-[#c084fc]">
          <KeyRound size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">OpenBao Token Management</h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">토큰 상태 확인, 수동 회전, 승인/일시중지 및 조회를 관리합니다.</p>
        </div>
      </div>

      {!STEP_UP_ENFORCED && (
        <div className="mb-4 rounded-md border border-[var(--color-border-default)] bg-[rgba(245,158,11,0.1)] px-3 py-2 text-sm text-[var(--color-text-primary)]">
          테스트 모드: 재로그인(step-up) 강제는 비활성화되어 있으며 내부적으로 자동 처리됩니다.
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[1.2fr_1fr]">
        <section className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-[rgba(255,255,255,0.02)]">
                {['Provider', 'Module', 'Path', 'Status', 'Expires'].map((h) => (
                  <th key={h} className="px-3.5 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr><td colSpan={5} className="border-t border-[var(--color-border-default)] px-3.5 py-8 text-center text-sm text-[var(--color-text-secondary)]">Loading token sources...</td></tr>
              )}
              {!isLoading && items.length === 0 && (
                <tr><td colSpan={5} className="border-t border-[var(--color-border-default)] px-3.5 py-8 text-center text-sm text-[var(--color-text-secondary)]">No token sources.</td></tr>
              )}
              {!isLoading && items.map((item) => (
                <tr key={item.id} className={cn('cursor-pointer', selected?.id === item.id && 'bg-[rgba(99,102,241,0.1)]')} onClick={() => setSelectedId(item.id)}>
                  <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm">{item.provider}</td>
                  <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm">{item.module}</td>
                  <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-xs text-[var(--color-text-secondary)]">{item.path}</td>
                  <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm"><span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', STATUS_BADGE[item.status] ?? STATUS_BADGE.expired)}>{item.status}</span></td>
                  <td className="border-t border-[var(--color-border-default)] px-3.5 py-3 text-xs text-[var(--color-text-secondary)]">{item.expires_at ? new Date(item.expires_at).toLocaleString() : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        <section className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <h2 className="mb-3 text-base font-semibold text-[var(--color-text-primary)]">Actions</h2>
          <div className="mb-4 flex flex-wrap gap-2">
            <Button onClick={() => runAction('rotate')} disabled={!sourceId}>Rotate</Button>
            <Button onClick={() => runAction('approve')} disabled={!sourceId}>Approve</Button>
            <Button onClick={() => runAction('pause')} disabled={!sourceId} variant="outline">Pause</Button>
            <Button onClick={() => runAction('resume')} disabled={!sourceId} variant="outline">Resume</Button>
            <Button onClick={handleReveal} disabled={!sourceId} variant="outline">Reveal</Button>
          </div>

          <h3 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">Recent Events</h3>
          <div className="max-h-[280px] space-y-2 overflow-auto">
            {events.length === 0 && <p className="text-sm text-[var(--color-text-secondary)]">No events.</p>}
            {events.map((event) => (
              <div key={event.id} className="rounded-md border border-[var(--color-border-default)] px-3 py-2">
                <div className="text-sm font-medium text-[var(--color-text-primary)]">{event.event_type} · {event.result}</div>
                <div className="text-xs text-[var(--color-text-secondary)]">{new Date(event.created_at).toLocaleString()}</div>
              </div>
            ))}
          </div>

          {revealResult && (
            <div className="mt-4 rounded-md border border-[var(--color-border-default)] bg-[rgba(15,23,42,0.35)] p-3">
              <div className="mb-1 text-xs font-semibold uppercase text-[var(--color-text-secondary)]">Reveal Result</div>
              <pre className="overflow-auto text-xs text-[var(--color-text-primary)]">{JSON.stringify(revealResult, null, 2)}</pre>
            </div>
          )}
        </section>
      </div>
    </div>
  )
}

// 상단 내비게이션의 종 아이콘과 작업 알림 드롭다운.
//
// 데모(2026-08-22)에서 나온 지적이 출발점이다 — "보일러플레이트가 생성되는 동안
// 다른 탭으로 이동하면 진행 중 표시가 없다". 그래서 이 컴포넌트는 **화면을 떠난
// 뒤에도 보이는 자리**에 있어야 하고, 헤더는 모든 화면이 공유하는 유일한 자리다.
//
// F7 의 알림 이력(AlertRule 기반 운영 알림)과 다른 개념이다. 여기 뜨는 것은
// 사용자가 방금 시킨 작업의 진행 상황뿐이며, 서버에 저장되는 이력이 아니다.

import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Bell } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { IconButton } from '../../../components/ui/icon-button'
import { StatusIcon, type StatusTone } from '../../../components/ui/status-icon'
import { cn } from '../../../lib/utils'
import { useJobNotifications } from '../hooks/use-job-notifications'
import { formatElapsed, type JobNotification, type JobPhase } from '../utils/job-notifications'

const PHASE_TONE: Record<JobPhase, StatusTone> = {
  running: 'running',
  succeeded: 'success',
  failed: 'error',
}

/**
 * 경과 시간. 끝난 작업은 끝난 시각에서 멈춘다 — 계속 흐르면 "아직 도는 중" 으로
 * 읽힌다.
 */
function elapsedOf(job: JobNotification, now: number): string {
  const startedAt = Date.parse(job.startedAt)
  if (Number.isNaN(startedAt)) return formatElapsed(0)
  return formatElapsed((job.settledAt ?? now) - startedAt)
}

export function JobNotificationBell() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { jobs, runningCount, now } = useJobNotifications()
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  // 열려 있을 때만 문서에 귀를 붙인다. 헤더는 모든 화면에 있으므로 늘 붙여 두면
  // 화면마다 쓰이지 않는 핸들러가 하나씩 늘어난다.
  useEffect(() => {
    if (!open) return

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    const closeOnOutside = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }

    document.addEventListener('keydown', closeOnEscape)
    document.addEventListener('mousedown', closeOnOutside)
    return () => {
      document.removeEventListener('keydown', closeOnEscape)
      document.removeEventListener('mousedown', closeOnOutside)
    }
  }, [open])

  const busy = runningCount > 0
  const label = busy
    ? t('jobNotifications.triggerBusy', { running: runningCount })
    : t('jobNotifications.trigger')

  return (
    <div className="relative" ref={rootRef}>
      <IconButton
        aria-label={label}
        aria-expanded={open}
        aria-haspopup="true"
        data-testid="job-notification-bell"
        data-busy={busy}
        onClick={() => setOpen((previous) => !previous)}
      >
        <Bell
          {...iconProps('sm')}
          data-testid="job-notification-icon"
          // 애니메이션은 CSS 클래스로 건다. 움직임을 줄이도록 설정한 사용자에게는
          // index.css 의 prefers-reduced-motion 규칙이 아예 끈다.
          className={cn('nullus-bell', busy && 'nullus-bell--busy')}
        />
      </IconButton>

      {busy && (
        <span
          data-testid="job-notification-count"
          className="pointer-events-none absolute -top-0.5 -right-0.5 min-w-[15px] rounded-[var(--radius-full)] bg-[var(--color-info)] px-1 text-center text-[10px] font-bold leading-[15px] text-[var(--color-surface-card)]"
        >
          {runningCount}
        </span>
      )}

      {open && (
        <div
          data-testid="job-notification-panel"
          className="absolute top-9 right-0 z-30 w-[300px] overflow-hidden rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] shadow-[0_12px_28px_color-mix(in_srgb,_var(--color-text-primary)_35%,_transparent)]"
        >
          <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-3 py-2">
            <span className="text-[12px] font-bold text-[var(--color-text-primary)]">
              {t('jobNotifications.title')}
            </span>
            <span className="text-[11px] text-[var(--color-text-secondary)]">
              {busy
                ? t('jobNotifications.summaryRunning', { running: runningCount })
                : t('jobNotifications.summaryIdle')}
            </span>
          </div>

          {jobs.length === 0 ? (
            <p className="px-3 py-4 text-center text-[12px] text-[var(--color-text-secondary)]">
              {t('jobNotifications.empty')}
            </p>
          ) : (
            <ul className="max-h-[320px] overflow-y-auto">
              {jobs.map((job) => (
                <li key={job.id}>
                  <button
                    type="button"
                    data-testid={`job-notification-item-${job.id}`}
                    data-phase={job.phase}
                    onClick={() => {
                      setOpen(false)
                      navigate(job.detailPath)
                    }}
                    className="flex w-full cursor-pointer items-start gap-2 border-none bg-transparent px-3 py-2 text-left hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_6%,_transparent)]"
                  >
                    <span className="mt-0.5">
                      <StatusIcon tone={PHASE_TONE[job.phase]} size="sm" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[12px] font-semibold text-[var(--color-text-primary)]">
                        {job.name}
                      </span>
                      <span className="block truncate text-[11px] text-[var(--color-text-secondary)]">
                        {t(`jobNotifications.kind.${job.kind}`, job.kind)}
                        {' · '}
                        {t(`jobNotifications.stage.${job.stage}`, job.stage)}
                      </span>
                    </span>
                    <span className="shrink-0 text-[11px] tabular-nums text-[var(--color-text-muted)]">
                      {elapsedOf(job, now)}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}

import { useEffect, useId, useRef, useState } from 'react'
import { cn } from '../../../lib/utils'
import {
  nextDisplayProgress,
  progressCeiling,
  type DeployProgressStatus,
} from '../utils/deploy-progress'

/** 표시 값을 다시 계산하는 간격. 눈에는 연속으로 보이면서 렌더는 아끼는 지점. */
const TICK_MS = 140

/**
 * 서버 진행률을 "쌓여 가는" 표시 값으로 바꾼다.
 *
 * 계산 규칙은 utils/deploy-progress 가 갖고, 여기서는 시간만 흘려 준다.
 */
export function useDeployProgressDisplay(
  target: number,
  status: DeployProgressStatus,
  milestones: number[],
): number {
  const [display, setDisplay] = useState(target)
  // 타이머는 한 번만 걸고 최신 값은 ref 로 들여다본다. 값이 바뀔 때마다 타이머를
  // 다시 걸면 그때마다 주기가 처음부터 시작해 막대가 끊겨 보인다.
  const latest = useRef({ target, status, milestones })

  useEffect(() => {
    latest.current = { target, status, milestones }
  }, [target, status, milestones])

  useEffect(() => {
    if (status === 'success') {
      setDisplay(100)
      return
    }

    const timer = setInterval(() => {
      setDisplay((current) => {
        const { target: t, status: s, milestones: m } = latest.current
        return nextDisplayProgress({
          current,
          target: t,
          ceiling: progressCeiling(t, m),
          status: s,
        })
      })
    }, TICK_MS)

    return () => clearInterval(timer)
  }, [status])

  return display
}

/**
 * 로켓. 진행 막대의 앞머리를 타고 간다.
 *
 * 오른쪽을 보고 날아가는 모양이고 뒤로 불꽃이 붙는다. 배포가 끝나면 그대로
 * 화면 밖으로 날아오른다 — 막대가 100% 에서 그냥 멈추는 것보다 끝났다는 것이
 * 한눈에 읽힌다.
 */
function DeployRocket({ state }: { state: DeployProgressStatus }) {
  const uid = useId().replace(/:/g, '')
  const isFlying = state === 'running'
  const isLaunched = state === 'success'
  const isFailed = state === 'failed'

  const hull = `hull-${uid}`
  const glow = `glow-${uid}`
  const trail = `trail-${uid}`

  return (
    <span
      data-testid="deploy-rocket"
      data-state={state}
      className={cn(
        'nullus-rocket pointer-events-none absolute top-1/2 z-10',
        isFlying && 'nullus-rocket--flying',
        isLaunched && 'nullus-rocket--launched',
      )}
      aria-hidden="true"
    >
      <svg width="46" height="32" viewBox="0 0 46 32" fill="none">
        <defs>
          {/* 동체는 막대와 같은 그라디언트를 탄다 — 로켓이 진행 막대에서 튀어나온
              것처럼 보이게 하려면 같은 색을 써야 한다. */}
          <linearGradient id={hull} x1="10" y1="8" x2="40" y2="24" gradientUnits="userSpaceOnUse">
            <stop offset="0" stopColor="var(--color-primary)" />
            <stop offset="1" stopColor="var(--color-accent-alt)" />
          </linearGradient>
          <radialGradient id={glow} cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stopColor="var(--nullus-flame-outer)" stopOpacity="0.4" />
            <stop offset="1" stopColor="var(--nullus-flame-outer)" stopOpacity="0" />
          </radialGradient>
          <linearGradient id={trail} x1="0" y1="0" x2="1" y2="0">
            <stop offset="0" stopColor="var(--color-accent-alt)" stopOpacity="0" />
            <stop offset="1" stopColor="var(--color-accent-alt)" stopOpacity="0.55" />
          </linearGradient>
        </defs>

        {!isFailed && <ellipse cx="18" cy="16" rx="18" ry="9" fill={`url(#${glow})`} />}

        {/* 지나온 자리에 남는 자국. 속도를 그림 하나로 말한다. */}
        {isFlying && <path d="M0 14.6 L11 13.2 L11 18.8 L0 17.4 Z" fill={`url(#${trail})`} />}

        {!isFailed && (
          <g data-testid="deploy-rocket-flame" className="nullus-rocket-flame">
            <path d="M12 10.6 C6 12 2 14 1 16 C2 18 6 20 12 21.4 Z" fill="var(--nullus-flame-outer)" opacity="0.92" />
            <path d="M12 12.4 C8 13.4 5.4 14.8 4.6 16 C5.4 17.2 8 18.6 12 19.6 Z" fill="var(--nullus-flame-mid)" />
            <path d="M12 14 C10 14.6 8.8 15.4 8.4 16 C8.8 16.6 10 17.4 12 18 Z" fill="var(--nullus-flame-core)" />
          </g>
        )}

        {/* 날개 — 동체보다 뒤로 젖혀 속도감을 준다 */}
        <path d="M16 10.4 L11.6 2.2 L23 8.2 Z" fill="var(--color-accent-alt)" opacity={isFailed ? 0.45 : 0.95} />
        <path d="M16 21.6 L11.6 29.8 L23 23.8 Z" fill="var(--color-accent-alt)" opacity={isFailed ? 0.45 : 0.95} />

        {/* 동체 */}
        <path
          d="M12 9.4 C19 6 28 6.2 36 12.4 C38.6 14.4 40.6 15.6 42 16 C40.6 16.4 38.6 17.6 36 19.6 C28 25.8 19 26 12 22.6 C10 20.8 10 11.2 12 9.4 Z"
          fill={isFailed ? 'var(--color-text-secondary)' : `url(#${hull})`}
        />
        {/* 위쪽 하이라이트 — 금속처럼 보이게 하는 한 줄 */}
        <path
          d="M13.6 10.4 C20 7.6 27.6 8 34.6 13.2"
          stroke="var(--nullus-rocket-sheen)"
          strokeOpacity={isFailed ? 0.2 : 0.45}
          strokeWidth="1.4"
          strokeLinecap="round"
        />

        {/* 조종석 */}
        <circle cx="27.4" cy="16" r="3.6" fill="var(--color-surface-card)" opacity="0.96" />
        <circle cx="27.4" cy="16" r="2.2" fill={isFailed ? 'var(--color-text-secondary)' : 'var(--color-accent-alt)'} />
        <path d="M26 14.6 A2.2 2.2 0 0 1 28.4 14.1" stroke="var(--nullus-rocket-sheen)" strokeOpacity="0.8" strokeWidth="0.9" strokeLinecap="round" />

        {/* 엔진 링 */}
        <rect x="11" y="9.6" width="2.6" height="12.8" rx="1.3" fill="var(--nullus-rocket-sheen)" opacity={isFailed ? 0.16 : 0.3} />
      </svg>
    </span>
  )
}

/**
 * 배포 진행 막대.
 *
 * 예전에는 1% 짜리 칸 100 개를 색만 바꿔 칠했다. 서버 진행률이 단계마다 크게
 * 뛰는 값이라 칸이 한 번에 우르르 켜졌다가 몇 분씩 멈춰 있었다 — 차오르는
 * 느낌이 없었다. 이제 한 덩어리가 부드럽게 늘어나고 그 앞머리를 로켓이 탄다.
 */
export function DeployProgressBar({
  value,
  status,
}: {
  value: number
  status: DeployProgressStatus
}) {
  const clamped = Math.max(0, Math.min(100, value))
  const rounded = Math.round(clamped)

  return (
    <div>
      <div className="mb-3 flex justify-between">
        <span className="text-xs text-[var(--color-text-secondary)]">Overall Progress</span>
        <span
          data-testid="deploy-progress-value"
          className="text-xs font-bold tabular-nums text-[var(--color-text-primary)]"
        >
          {rounded}%
        </span>
      </div>
      <div
        role="progressbar"
        aria-valuenow={rounded}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Deployment progress"
        className="relative h-2.5 w-full rounded-full bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)]"
      >
        <div
          data-testid="deploy-progress-fill"
          className={cn(
            'nullus-progress-fill h-full rounded-full',
            status === 'failed'
              ? 'bg-[var(--color-error)]'
              : 'bg-[linear-gradient(90deg,var(--color-primary),var(--color-accent-alt))]',
            status === 'running' && 'nullus-progress-fill--live',
          )}
          style={{ width: `${clamped}%` }}
        />
        {/* 로켓은 채워진 끝에 선다. 100% 에서는 그대로 날아가 화면을 벗어난다. */}
        <span className="absolute inset-y-0" style={{ left: `${clamped}%` }}>
          <DeployRocket state={status} />
        </span>
      </div>
    </div>
  )
}

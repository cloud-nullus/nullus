// 상태 아이콘 단일 출처 — DESIGN.md §상태 표시 의 표를 코드로 옮긴 것이다.
//
// 개편 전에는 같은 "성공" 을 화면마다 다르게 그렸다: CheckCircle 8곳,
// CheckCircle2 11곳, 맨 Check 10곳. 실패는 XCircle 13곳과 AlertCircle 8곳으로,
// 진행중은 Loader2 / Loader / RefreshCw / CircleDashed 넷으로 갈렸다.
// 고르는 자리가 화면마다 있었기 때문이다 — 그 자리를 여기 하나로 없앤다.
//
// 여섯이 같은 원 실루엣을 쓰고 경고만 삼각형이다. 상태가 한 벌로 읽히게 하려는
// 것이고, 경고는 그 벌에서 튀어나와야 하므로 유일하게 모양을 깬다.
//
// 이름에 주의: lucide 의 `CheckCircle` 은 체크가 원을 뚫고 나오는 변형이라
// 옆에 서는 `CircleX`·`CircleMinus` 와 실루엣이 어긋난다. 담긴 형태는 `CircleCheck` 다.

import {
  CircleCheck,
  CircleDashed,
  CircleMinus,
  CircleX,
  Info,
  LoaderCircle,
  TriangleAlert,
  type LucideIcon,
} from 'lucide-react'
import { iconProps, type IconSize } from './icon'

/** 상태의 의미. 도메인 상태값이 아니라 "무슨 뜻인지" 다. */
export type StatusTone = 'success' | 'running' | 'pending' | 'warning' | 'error' | 'info' | 'neutral'

export const STATUS_TOKEN: Record<StatusTone, string> = {
  success: '--color-success',
  running: '--color-info',
  pending: '--color-text-muted',
  warning: '--color-warning',
  error: '--color-error',
  info: '--color-info',
  // 상태색으로 --color-primary 를 쓰지 않는다. 그 색은 CTA 의 색이라 상태에
  // 얹으면 "도는 중" 과 "누르세요" 가 같은 파랑이 된다.
  neutral: '--color-text-muted',
}

export const STATUS_ICON: Record<StatusTone, LucideIcon> = {
  success: CircleCheck,
  running: LoaderCircle,
  pending: CircleDashed,
  warning: TriangleAlert,
  error: CircleX,
  info: Info,
  neutral: CircleMinus,
}

/** 표를 그대로 돌려준다. 화면에서 아이콘을 고르지 않는다. */
export const statusIcon = (tone: StatusTone): LucideIcon => STATUS_ICON[tone]

type Props = {
  tone: StatusTone
  size?: IconSize
  /**
   * 옆에 상태 글자가 있으면 읽는 도구에 두 번 들리지 않게 숨긴다.
   * 아이콘만 홀로 상태를 나타낸다면 반드시 label 을 준다 — DESIGN.md 는
   * 색만으로 상태를 표시하지 못하게 하는데, 아이콘만 두는 것도 같은 문제다.
   */
  label?: string
  /** 감싼 쪽이 이미 상태색을 칠했을 때. 배지 안처럼 색이 두 번 정해지는 자리다. */
  inheritColor?: boolean
  className?: string
}

export function StatusIcon({ tone, size = 'sm', label, inheritColor = false, className }: Props) {
  const Glyph = STATUS_ICON[tone]
  return (
    <Glyph
      {...iconProps(size)}
      // 색을 여기서 함께 준다. 모양만 모아 두고 색을 화면에 맡기면 같은 tone 이
      // 화면마다 다른 초록으로 칠해진다 — 실제로 emerald-400 과 --color-success 가
      // 섞여 있었다. Tailwind 임의값은 문자열을 스캔해 만들어지므로 토큰 이름을
      // 조립해 쓸 수 없다. 그래서 클래스가 아니라 인라인 style 로 토큰을 참조한다.
      style={inheritColor ? undefined : { color: `var(${STATUS_TOKEN[tone]})` }}
      className={
        // 도는 중은 멈춰 있으면 "멈춤" 으로 읽힌다. 움직임을 줄이도록 설정한
        // 사용자에게는 Tailwind 가 애니메이션을 끈다(motion-reduce).
        tone === 'running' ? `animate-spin ${className ?? ''}`.trim() : className
      }
      // iconProps 가 기본으로 숨기므로, 이름을 줄 때는 그 숨김을 명시적으로 푼다.
      {...(label ? { role: 'img', 'aria-label': label, 'aria-hidden': undefined } : {})}
    />
  )
}

/**
 * 문자열 상태값을 tone 으로 옮긴다. 도메인마다 쓰는 낱말이 달라 여기 모아 둔다.
 *
 * pending 과 warning 을 나눈다. 개편 전에는 둘이 한 tone 이었는데, 대기는
 * 시간의 문제고 경고는 주의의 문제라 같은 색·같은 모양을 줄 이유가 없다.
 * running 도 success 에서 뗐다 — 설치·파이프라인 화면은 "도는 중" 과 "끝남" 을
 * 반드시 구별해야 한다.
 */
export function toneForStatus(status: string | null | undefined): StatusTone {
  switch ((status ?? '').toLowerCase()) {
    case 'connected':
    case 'success':
    case 'succeeded':
    case 'active':
    case 'healthy':
    case 'completed':
    case 'synced':
      return 'success'
    case 'running':
    case 'installing':
    case 'in_progress':
    case 'progressing':
    case 'health_check':
    case 'configuring':
      return 'running'
    case 'pending':
    case 'queued':
    case 'waiting':
      return 'pending'
    case 'warning':
    case 'warn':
    case 'unreachable':
    case 'degraded':
    case 'rolled_back':
    case 'terminating':
      return 'warning'
    case 'error':
    case 'failed':
    case 'auth_failed':
    case 'unhealthy':
    case 'down':
      return 'error'
    default:
      return 'neutral'
  }
}

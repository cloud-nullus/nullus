// 배지의 모양 규격.
//
// `StatusBadge` 와 역할이 다르다. StatusBadge 는 **상태**를 담고, DESIGN.md
// §Components 규칙에 따라 색 + 아이콘 + 텍스트를 함께 쓴다. 이 컴포넌트가 담는
// 건 분류다 — 역할(admin/devops/developer), 티어(stable/beta/deprecated),
// 심각도, 도구 카테고리처럼 "무슨 종류인가" 를 알리는 라벨이다. 거기에 상태
// 아이콘을 붙이면 상태가 아닌 것이 상태처럼 읽힌다.
//
// 색은 호출부가 className 으로 넘긴다. 각 도메인의 팔레트가 이미 자기 파일에
// 있고(ROLE_BADGE, TIER_BADGE_CLASS …), 그걸 여기로 끌어오면 이 파일이 모든
// 도메인을 알아야 한다. 여기서 고정하는 건 모양뿐이다.
//
// 개편 전에는 15곳이 각자 모양을 만들고 있었다:
//   - 모서리: rounded-[5px] / rounded-md / rounded-lg / rounded-[10px] / rounded-full
//   - 여백: px-[7px] / px-2 / px-2.5 / px-[9px]
//   - 글자: text-[10px] / text-[11px] / text-xs
// 같은 표 안에서 역할 배지와 상태 배지의 모서리가 서로 달랐다.

import { type CSSProperties, type ReactNode } from 'react'
import { cn } from '../../lib/utils'

interface BadgeProps {
  children: ReactNode
  /** 도메인 팔레트(배경·글자색)를 넘긴다. 모양 클래스는 넘기지 않는다. */
  className?: string
  /** 완전히 둥근 알약 모양. 티어·분류처럼 개수가 적은 축에 쓴다. */
  pill?: boolean
  /** 팔레트가 클래스가 아니라 값으로 오는 곳(status-style.ts)을 위해 남긴다. */
  style?: CSSProperties
  'aria-label'?: string
}

export function Badge({ children, className, pill = false, style, ...rest }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1 px-2 py-0.5 text-[11px] font-semibold',
        pill ? 'rounded-[var(--radius-full)]' : 'rounded-[var(--radius-sm)]',
        className,
      )}
      style={style}
      {...rest}
    >
      {children}
    </span>
  )
}

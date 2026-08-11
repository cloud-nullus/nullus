// 아이콘 하나만 담는 버튼의 규격.
//
// Button 과 나눠 두는 이유: Button 은 라벨이 있는 액션이라 좌우 여백과 최소 높이가
// 라벨 기준으로 잡혀 있다. 아이콘 하나짜리를 거기 넣으면 가로로 늘어진 상자가 된다.
//
// 개편 전에는 자리마다 손으로 만들고 있었고 크기·모서리·호버가 갈렸다:
//   - 크기: p-1 / p-1.5 / h-6 w-6 세 가지
//   - 모서리: rounded / rounded-md / rounded-lg 세 가지
//   - 호버: 배경이 바뀌는 곳, 글자색만 바뀌는 곳, 아무 반응 없는 곳
//
// 아이콘만 있으므로 aria-label 은 필수다. 개편 전 사이드바 토글과 헤더 테마 토글은
// 갖고 있었지만 cicd-list 의 닫기(X)와 stack-install 의 저장·삭제는 없었다 —
// 스크린리더에는 이름 없는 버튼이었다. 타입으로 강제한다.

import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export type IconButtonTone = 'default' | 'primary' | 'danger'

const TONE: Record<IconButtonTone, string> = {
  default:
    'text-[var(--color-text-secondary)] hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] hover:text-[var(--color-text-primary)]',
  primary:
    'text-[var(--color-primary)] hover:bg-[color-mix(in_srgb,_var(--color-primary)_12%,_transparent)]',
  danger:
    'text-[var(--color-error)] hover:bg-[color-mix(in_srgb,_var(--color-error)_12%,_transparent)]',
}

interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'color'> {
  /** 아이콘만 보이므로 접근 이름이 반드시 필요하다. */
  'aria-label': string
  tone?: IconButtonTone
  /** 테두리를 둘러 독립된 컨트롤로 보이게 한다 (툴바 밖에 홀로 설 때). */
  outlined?: boolean
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ tone = 'default', outlined = false, className, type = 'button', ...props }, ref) => (
    <button
      ref={ref}
      type={type}
      className={cn(
        'inline-flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-[var(--radius-sm)]',
        'bg-transparent transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-40',
        outlined ? 'border border-[var(--color-border-default)]' : 'border-none',
        TONE[tone],
        className,
      )}
      {...props}
    />
  ),
)

IconButton.displayName = 'IconButton'

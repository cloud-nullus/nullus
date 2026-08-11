// MUI Button 어댑터.
//
// 공개 API(variant / size / loading + 표준 button 속성)를 그대로 유지하고 내부만
// MUI 로 바꾼다. 그래서 이 컴포넌트를 쓰는 124곳은 한 줄도 고치지 않는다 —
// 기획안 §6.2 의 어댑터 전략이다.
//
// 색은 토큰만 쓴다. 이전 구현은 hex 를 직접 박아서(indigo-300, red-400 계열) 다크 기준
// 값이 라이트 테마에서 1.91:1 까지 무너져 있었다. 이제 --color-* 를 참조하므로
// 테마 전환에 따라 값이 함께 바뀐다.

import { forwardRef, type ButtonHTMLAttributes } from 'react'
import MuiButton from '@mui/material/Button'
import { brandCtaSx } from '../../theme/mui-theme'

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'lg'

// `color` 를 뺀다: HTML 의 color 속성(string)이 MUI 의 color 유니온과 충돌해
// 오버로드 해석이 깨진다. 색은 variant 로 정하므로 실사용에 영향이 없다.
interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'color'> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

const MUI_SIZE = { sm: 'small', md: 'medium', lg: 'large' } as const

/** DESIGN.md §Components. primary 는 브랜드 골드를 유지한다 — 기획안 D1 결정 대기. */
const VARIANT_SX = {
  primary: brandCtaSx,
  secondary: {
    backgroundColor: 'color-mix(in srgb, var(--color-primary) 14%, transparent)',
    borderColor: 'color-mix(in srgb, var(--color-primary) 40%, transparent)',
    color: 'var(--color-primary)',
    '&:hover': {
      backgroundColor: 'color-mix(in srgb, var(--color-primary) 22%, transparent)',
      borderColor: 'var(--color-primary)',
    },
  },
  outline: {
    borderColor: 'var(--color-border-default)',
    color: 'var(--color-text-primary)',
    '&:hover': { borderColor: 'var(--color-border-hover)' },
  },
  danger: {
    backgroundColor: 'color-mix(in srgb, var(--color-error) 14%, transparent)',
    borderColor: 'color-mix(in srgb, var(--color-error) 40%, transparent)',
    color: 'var(--color-error)',
    '&:hover': {
      backgroundColor: 'color-mix(in srgb, var(--color-error) 22%, transparent)',
      borderColor: 'var(--color-error)',
    },
  },
  ghost: {
    color: 'var(--color-text-secondary)',
    '&:hover': { color: 'var(--color-text-primary)' },
  },
} as const

/** MUI 의 시각 변형. secondary/outline/danger 는 테두리를 쓰므로 outlined 다. */
const MUI_VARIANT = {
  primary: 'contained',
  secondary: 'outlined',
  outline: 'outlined',
  danger: 'outlined',
  ghost: 'text',
} as const

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'outline', size = 'md', loading = false, disabled, children, className, style: _style, ...props }, ref) => (
    <MuiButton
      ref={ref}
      className={className}
      variant={MUI_VARIANT[variant]}
      size={MUI_SIZE[size]}
      loading={loading}
      disabled={disabled}
      // 이전 구현과 같은 로딩 UX: 스피너를 보이면서 라벨을 유지한다.
      loadingPosition="start"
      sx={VARIANT_SX[variant]}
      {...props}
    >
      {children}
    </MuiButton>
  ),
)

Button.displayName = 'Button'

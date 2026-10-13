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

/**
 * DESIGN.md §Components.
 *
 * primary 는 브랜드 블루 단색이다. 개편 전에는 골드 그라데이션이었는데, 화면마다
 * CTA 가 골드와 인디고로 갈려 통일감을 깨는 큰 축이었다. 골드는 로고 마크 전용으로
 * 내렸다 — Coinbase 구조를 따르면 브랜드 색은 한 곳에만 쓰고 액션은 primary 하나다.
 * 골드 면이 정말 필요하면 brandCtaSx 를 직접 쓴다.
 */
const BASE_SX = {
  primary: {
    backgroundColor: 'var(--color-primary)',
    color: 'var(--color-on-primary)',
    '&:hover': { backgroundColor: 'var(--color-primary)', filter: 'brightness(1.08)' },
  },
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

/** 비활성 버튼의 불투명도. 같은 색을 흐리게 해 "있지만 지금은 못 누른다" 를 보인다. */
const DISABLED_OPACITY = 0.45

/**
 * 비활성 표현. variant 색을 sx 로 덮으면서 MUI 의 비활성 색까지 덮여, 비활성 버튼이 활성 버튼과
 * 똑같이 보였다(눌러도 아무 일이 없으니 고장으로 보인다). variant 색은 그대로 두고 흐리게 한다.
 *
 * - 커서는 금지 모양이다. MUI 는 비활성 버튼의 pointer-events 를 끄는데, 그러면 커서도 바뀌지 않는다.
 *   네이티브 disabled 버튼은 pointer-events 를 켜도 클릭을 보내지 않는다.
 * - 진행 중(loading)은 흐리지 않는다 — 스피너가 상태를 말하고, 흐리면 실패한 것처럼 보인다.
 * - 호버는 반응하지 않는다.
 */
function withDisabled<T extends { '&:hover'?: object }>(base: T) {
  const { '&:hover': _hover, ...colors } = base
  const disabled = '&.Mui-disabled:not(.MuiButton-loading)'
  return {
    ...base,
    [disabled]: {
      ...colors,
      opacity: DISABLED_OPACITY,
      cursor: 'not-allowed',
      pointerEvents: 'auto',
    },
    [`${disabled}:hover`]: { ...colors, filter: 'none' },
  }
}

const VARIANT_SX = {
  primary: withDisabled(BASE_SX.primary),
  secondary: withDisabled(BASE_SX.secondary),
  outline: withDisabled(BASE_SX.outline),
  danger: withDisabled(BASE_SX.danger),
  ghost: withDisabled(BASE_SX.ghost),
}

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

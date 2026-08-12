// MUI Paper 어댑터.
//
// 공개 API(icon / iconBg / iconColor / title / description / children / onClick /
// selected / footer)를 유지한다.
//
// iconBg/iconColor 는 이전 구현에서 죽어 있었다 — 삼항의 양쪽 분기가 같은 값이라
// 무엇을 넘겨도 항상 indigo 였고, 그래서 디자인시스템의 "기능별 색상 매핑"이
// 코드에 반영되지 않았다. 이제 실제로 반영한다(기획안 부록 A3).
//
// 깊이는 DESIGN.md §Elevation & Depth 를 따른다: 라이트는 그림자(--shadow-raised),
// 다크는 표면 밝기 차. 토큰이 테마별로 다른 값을 갖기 때문에 여기서 분기하지 않는다.

import type { CSSProperties, ReactNode } from 'react'
import Paper from '@mui/material/Paper'
import { cn } from '../../lib/utils'

interface CardProps {
  icon?: ReactNode
  iconBg?: string
  iconColor?: string
  title?: string
  description?: string
  children?: ReactNode
  onClick?: () => void
  selected?: boolean
  style?: CSSProperties
  footer?: ReactNode
}

/** 기능별 색상 매핑의 기본값 — DESIGN.md §Colors 의 primary 액센트. */
const DEFAULT_ICON_BG = 'color-mix(in srgb, var(--color-primary) 15%, transparent)'
const DEFAULT_ICON_COLOR = 'var(--color-primary)'

export function Card({
  icon,
  iconBg,
  iconColor,
  title,
  description,
  children,
  onClick,
  selected = false,
  style: _style,
  footer,
}: CardProps) {
  const content = (
    <>
      {(icon || title || description) && (
        <div className="flex items-start gap-3">
          {icon && (
            <div
              className="flex h-[var(--icon-size)] w-[var(--icon-size)] shrink-0 items-center justify-center rounded-[var(--icon-radius)]"
              style={{
                backgroundColor: iconBg ?? DEFAULT_ICON_BG,
                color: iconColor ?? DEFAULT_ICON_COLOR,
              }}
            >
              {icon}
            </div>
          )}
          {(title || description) && (
            <div className="min-w-0 flex-1">
              {title && (
                <div
                  className={cn(
                    'text-sm font-bold text-[var(--color-text-primary)]',
                    description ? 'mb-1' : '',
                  )}
                >
                  {title}
                </div>
              )}
              {description && (
                <div className="text-[13px] leading-6 text-[var(--color-text-secondary)]">
                  {description}
                </div>
              )}
            </div>
          )}
        </div>
      )}
      {children}
      {footer && (
        <div className="mt-auto border-t border-[var(--color-border-default)] pt-2">{footer}</div>
      )}
    </>
  )

  return (
    <Paper
      variant="outlined"
      component={onClick ? 'button' : 'div'}
      {...(onClick ? { type: 'button' as const, onClick } : {})}
      className={cn(
        'flex flex-col gap-3',
        onClick ? 'cursor-pointer text-left' : 'cursor-default',
      )}
      sx={{
        padding: 'var(--card-padding)',
        borderRadius: 'var(--card-radius)',
        borderColor: selected ? 'var(--color-primary)' : 'var(--color-border-default)',
        backgroundColor: 'var(--color-surface-card)',
        boxShadow: 'var(--shadow-raised)',
        transitionProperty: 'border-color',
        transitionDuration: 'var(--transition-default)',
        '&:hover': { borderColor: selected ? 'var(--color-primary)' : 'var(--color-border-hover)' },
      }}
    >
      {content}
    </Paper>
  )
}

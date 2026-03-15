import { type ReactNode, type CSSProperties } from 'react'
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

export function Card({
  icon,
  iconBg = 'rgba(99,102,241,0.15)',
  iconColor = '#818cf8',
  title,
  description,
  children,
  onClick,
  selected = false,
  style: _style,
  footer,
}: CardProps) {
  const iconBgClass = iconBg === 'rgba(99,102,241,0.15)' ? 'bg-[rgba(99,102,241,0.15)]' : 'bg-[rgba(99,102,241,0.15)]'
  const iconColorClass = iconColor === '#818cf8' ? 'text-[#818cf8]' : 'text-[#818cf8]'
  const baseClassName = cn(
    'flex flex-col gap-3 rounded-[var(--card-radius)] border bg-[var(--color-surface-card)] p-[var(--card-padding)] transition-colors duration-200 ease-in-out',
    selected
      ? 'border-[#6366f1]'
      : 'border-[var(--color-border-default)] hover:border-[var(--color-border-hover)]',
    onClick ? 'cursor-pointer text-left' : 'cursor-default'
  )

  const content = (
    <>
      {(icon || title || description) && (
        <div className="flex items-start gap-3">
          {icon && (
            <div
              className={cn(
                'flex h-[var(--icon-size)] w-[var(--icon-size)] shrink-0 items-center justify-center rounded-[var(--icon-radius)]',
                iconBgClass,
                iconColorClass
              )}
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
                    description ? 'mb-1' : ''
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
        <div className="mt-auto border-t border-[var(--color-border-default)] pt-2">
          {footer}
        </div>
      )}
    </>
  )

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={baseClassName}>
        {content}
      </button>
    )
  }

  return <div className={baseClassName}>{content}</div>
}

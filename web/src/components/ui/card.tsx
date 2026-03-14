import { type ReactNode, type CSSProperties } from 'react'

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
  style,
  footer,
}: CardProps) {
  return (
    <div
      onClick={onClick}
      style={{
        background: 'var(--color-surface-card)',
        border: `1px solid ${selected ? '#6366f1' : 'var(--color-border-default)'}`,
        borderRadius: 'var(--card-radius)',
        padding: 'var(--card-padding)',
        transition: 'border-color var(--transition-default)',
        cursor: onClick ? 'pointer' : 'default',
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
        ...style,
      }}
      onMouseEnter={(e) => {
        if (!selected) {
          ;(e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-hover)'
        }
      }}
      onMouseLeave={(e) => {
        if (!selected) {
          ;(e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-default)'
        }
      }}
    >
      {(icon || title || description) && (
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '12px' }}>
          {icon && (
            <div
              style={{
                width: 'var(--icon-size)',
                height: 'var(--icon-size)',
                background: iconBg,
                borderRadius: 'var(--icon-radius)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: iconColor,
                flexShrink: 0,
              }}
            >
              {icon}
            </div>
          )}
          {(title || description) && (
            <div style={{ flex: 1, minWidth: 0 }}>
              {title && (
                <div
                  style={{
                    fontSize: '14px',
                    fontWeight: 700,
                    color: 'var(--color-text-primary)',
                    marginBottom: description ? '4px' : 0,
                  }}
                >
                  {title}
                </div>
              )}
              {description && (
                <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', lineHeight: 1.5 }}>
                  {description}
                </div>
              )}
            </div>
          )}
        </div>
      )}
      {children}
      {footer && (
        <div style={{ marginTop: 'auto', paddingTop: '8px', borderTop: '1px solid var(--color-border-default)' }}>
          {footer}
        </div>
      )}
    </div>
  )
}

import { type ButtonHTMLAttributes, forwardRef } from 'react'

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

const variantStyles: Record<ButtonVariant, React.CSSProperties> = {
  primary: {
    background: 'linear-gradient(135deg, #ffd700, #f59e0b)',
    color: '#1a1d29',
    border: 'none',
    fontWeight: 700,
  },
  secondary: {
    background: 'rgba(99,102,241,0.15)',
    color: '#a5b4fc',
    border: '1px solid rgba(99,102,241,0.3)',
    fontWeight: 600,
  },
  outline: {
    background: 'transparent',
    color: 'var(--color-text-primary)',
    border: '1px solid var(--color-border-default)',
    fontWeight: 600,
  },
  danger: {
    background: 'rgba(239,68,68,0.15)',
    color: '#f87171',
    border: '1px solid rgba(239,68,68,0.3)',
    fontWeight: 600,
  },
  ghost: {
    background: 'transparent',
    color: 'var(--color-text-secondary)',
    border: 'none',
    fontWeight: 500,
  },
}

const sizeStyles: Record<ButtonSize, React.CSSProperties> = {
  sm: { padding: '6px 14px', fontSize: '12px', borderRadius: '8px' },
  md: { padding: '10px 20px', fontSize: '14px', borderRadius: '10px' },
  lg: { padding: '12px 28px', fontSize: '15px', borderRadius: '10px' },
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'outline', size = 'md', loading = false, disabled, children, style, ...props }, ref) => {
    const isDisabled = disabled || loading

    return (
      <button
        ref={ref}
        disabled={isDisabled}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: '8px',
          cursor: isDisabled ? 'not-allowed' : 'pointer',
          opacity: isDisabled ? 0.5 : 1,
          transition: 'all var(--transition-fast)',
          ...variantStyles[variant],
          ...sizeStyles[size],
          ...style,
        }}
        {...props}
      >
        {loading && (
          <span
            style={{
              width: '14px',
              height: '14px',
              border: '2px solid currentColor',
              borderTopColor: 'transparent',
              borderRadius: '50%',
              display: 'inline-block',
              animation: 'spin 0.6s linear infinite',
            }}
          />
        )}
        {children}
      </button>
    )
  }
)

Button.displayName = 'Button'

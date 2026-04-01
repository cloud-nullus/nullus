import { type ButtonHTMLAttributes, forwardRef } from 'react'
import { cn } from '../../lib/utils'

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

const variantStyles: Record<ButtonVariant, string> = {
  primary:
    'border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] font-bold text-[#1a1d29]',
  secondary:
    'border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.15)] font-semibold text-[#a5b4fc]',
  outline:
    'border border-[var(--color-border-default)] bg-transparent font-semibold text-[var(--color-text-primary)]',
  danger:
    'border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.15)] font-semibold text-[#f87171]',
  ghost: 'border-none bg-transparent font-medium text-[var(--color-text-secondary)]',
}

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'rounded-lg px-3.5 py-1.5 text-xs',
  md: 'rounded-[10px] px-5 py-2.5 text-sm',
  lg: 'rounded-[10px] px-7 py-3 text-[15px]',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = 'outline',
      size = 'md',
      loading = false,
      disabled,
      children,
      className,
      style: _style,
      ...props
    },
    ref
  ) => {
    const isDisabled = disabled || loading

    return (
      <button
        ref={ref}
        disabled={isDisabled}
        className={cn(
          'inline-flex items-center justify-center gap-2 transition-all duration-150 ease-in-out',
          isDisabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
        {...props}
      >
        {loading && <span className="inline-block size-[14px] animate-spin rounded-full border-2 border-dashed border-current border-t-transparent [animation-duration:0.7s]" />}
        {children}
      </button>
    )
  }
)

Button.displayName = 'Button'

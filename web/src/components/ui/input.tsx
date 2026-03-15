import { type InputHTMLAttributes, forwardRef, useId } from 'react'
import { cn } from '../../lib/utils'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helperText?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, className, style: _style, ...props }, ref) => {
    const id = useId()

    return (
      <div className="flex flex-col gap-1">
        {label && (
          <label
            htmlFor={id}
            className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]"
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={id}
          className={cn(
            'box-border w-full rounded-lg border bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] outline-none transition-all duration-150 ease-in-out focus:border-[#6366f1]',
            error ? 'border-[rgba(239,68,68,0.5)]' : 'border-[var(--color-border-default)]',
            className
          )}
          {...props}
        />
        {error && (
          <span className="text-xs text-[#f87171]">{error}</span>
        )}
        {!error && helperText && (
          <span className="text-xs text-[var(--color-text-muted)]">{helperText}</span>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

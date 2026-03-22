import { type SelectHTMLAttributes, forwardRef, useId } from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '../../lib/utils'

interface NativeSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  error?: string
}

export const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  ({ label, error, className, children, ...props }, ref) => {
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
        <div className="relative">
          <select
            ref={ref}
            id={id}
            className={cn(
              'box-border w-full cursor-pointer appearance-none rounded-lg border bg-[rgba(255,255,255,0.04)] px-3 py-[9px] pr-8 text-sm text-[var(--color-text-primary)] outline-none transition-all duration-150 ease-in-out focus:border-[#6366f1]',
              error ? 'border-[rgba(239,68,68,0.5)]' : 'border-[var(--color-border-default)]',
              className
            )}
            {...props}
          >
            {children}
          </select>
          <ChevronDown
            size={14}
            className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
          />
        </div>
        {error && <span className="text-xs text-[#f87171]">{error}</span>}
      </div>
    )
  }
)

NativeSelect.displayName = 'NativeSelect'

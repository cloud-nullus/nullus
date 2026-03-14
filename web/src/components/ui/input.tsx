import { type InputHTMLAttributes, forwardRef, useId } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helperText?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, style, ...props }, ref) => {
    const id = useId()

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
        {label && (
          <label
            htmlFor={id}
            style={{
              fontSize: '12px',
              fontWeight: 500,
              color: 'var(--color-text-secondary)',
              letterSpacing: '0.02em',
            }}
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={id}
          style={{
            background: 'rgba(255,255,255,0.04)',
            border: `1px solid ${error ? 'rgba(239,68,68,0.5)' : 'var(--color-border-default)'}`,
            borderRadius: '8px',
            padding: '9px 12px',
            fontSize: '14px',
            color: 'var(--color-text-primary)',
            outline: 'none',
            transition: 'border-color var(--transition-fast)',
            width: '100%',
            boxSizing: 'border-box',
            ...style,
          }}
          onFocus={(e) => {
            e.currentTarget.style.borderColor = '#6366f1'
            props.onFocus?.(e)
          }}
          onBlur={(e) => {
            e.currentTarget.style.borderColor = error
              ? 'rgba(239,68,68,0.5)'
              : 'var(--color-border-default)'
            props.onBlur?.(e)
          }}
          {...props}
        />
        {error && (
          <span style={{ fontSize: '12px', color: '#f87171' }}>{error}</span>
        )}
        {!error && helperText && (
          <span style={{ fontSize: '12px', color: 'var(--color-text-muted)' }}>{helperText}</span>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

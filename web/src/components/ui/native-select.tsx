// MUI NativeSelect 어댑터.
//
// MUI 의 `Select` 가 아니라 `NativeSelect` 를 쓴다. Select 는 진짜 <select> 대신
// listbox 를 렌더하므로 소비자가 넘기는 <option> children 이 전부 깨지고,
// 테스트의 getByRole('combobox') / fireEvent.change 도 못 쓴다.
// NativeSelect 는 실제 <select> 를 유지해 기존 계약을 그대로 지킨다.

import { forwardRef, useId, type SelectHTMLAttributes } from 'react'
import FormControl from '@mui/material/FormControl'
import FormHelperText from '@mui/material/FormHelperText'
import InputLabel from '@mui/material/InputLabel'
import MuiNativeSelect from '@mui/material/NativeSelect'

// `color` / `size` 는 MUI 유니온과 충돌하므로 뺀다.
interface NativeSelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'color' | 'size'> {
  label?: string
  error?: string
}

export const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  ({ label, error, className, children, ...props }, ref) => {
    const id = useId()

    return (
      <FormControl fullWidth error={Boolean(error)} variant="outlined">
        {label && (
          <InputLabel htmlFor={id} shrink sx={{ color: 'var(--color-text-secondary)' }}>
            {label}
          </InputLabel>
        )}
        <MuiNativeSelect
          inputRef={ref}
          id={id}
          className={className}
          inputProps={props}
          sx={{
            backgroundColor: 'var(--color-surface-card)',
            color: 'var(--color-text-primary)',
            '& .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-default)' },
            '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-hover)' },
            '& .MuiNativeSelect-icon': { color: 'var(--color-text-secondary)' },
          }}
        >
          {children}
        </MuiNativeSelect>
        {error && <FormHelperText sx={{ color: 'var(--color-error)' }}>{error}</FormHelperText>}
      </FormControl>
    )
  },
)

NativeSelect.displayName = 'NativeSelect'

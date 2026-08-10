// MUI TextField 어댑터.
//
// 공개 API(label / error / helperText + 표준 input 속성)를 유지한다. 이 컴포넌트를
// 쓰는 95곳은 고치지 않는다.
//
// label ↔ input 연결은 MUI 가 처리한다. 스위트 전체에서 getByLabelText 를 103회
// 쓰고 있으므로 이 연결이 끊기면 즉시 드러난다.

import { forwardRef, type InputHTMLAttributes } from 'react'
import TextField from '@mui/material/TextField'

// `color` / `size` 는 MUI 의 유니온과 충돌하므로 뺀다.
interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'color' | 'size'> {
  label?: string
  error?: string
  helperText?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, className, style: _style, ...props }, ref) => (
    <TextField
      inputRef={ref}
      className={className}
      label={label}
      error={Boolean(error)}
      // 에러가 있으면 에러 문구가 helperText 자리를 차지한다 — 이전 구현과 같은 규칙이다.
      helperText={error || helperText}
      fullWidth
      slotProps={{ htmlInput: props }}
      sx={{
        '& .MuiInputBase-root': {
          backgroundColor: 'var(--color-surface-card)',
          color: 'var(--color-text-primary)',
        },
        '& .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-default)' },
        '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-hover)' },
        '& .MuiInputLabel-root': { color: 'var(--color-text-secondary)' },
        '& .MuiFormHelperText-root': { color: 'var(--color-text-muted)' },
        '& .MuiFormHelperText-root.Mui-error': { color: 'var(--color-error)' },
      }}
    />
  ),
)

Input.displayName = 'Input'

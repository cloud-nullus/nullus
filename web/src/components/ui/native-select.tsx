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
import OutlinedInput from '@mui/material/OutlinedInput'

// `color` / `size` 는 MUI 유니온과 충돌하므로 뺀다.
interface NativeSelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'color' | 'size'> {
  label?: string
  error?: string
}

export const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  ({ label, error, className, children, ...props }, ref) => {
    const id = useId()

    // fullWidth 를 쓰지 않는다. 소비자 다수가 툴바의 flex 컨테이너 안에서 이 컴포넌트를
    // 쓰는데, 100% 너비를 강제하면 나란히 있던 필터가 세로로 쌓여 화면당 정보가 줄어든다
    // — DESIGN.md §Components 의 밀도 규칙 위반이다. 너비는 호출자가 정한다.
    return (
      <FormControl error={Boolean(error)} variant="outlined">
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
          // outlined 는 FormControl 의 variant 만으로는 안 걸린다. NativeSelect 의
          // 기본 입력은 standard(밑줄)라서, 아래 notchedOutline 규칙들이 겨냥할
          // 요소 자체가 렌더되지 않았다 — sx 가 통째로 죽은 코드였고 화면에는
          // 밑줄 입력이 나왔다. 다른 컨트롤(TextInput·SearchInput·Button)은 전부
          // 테두리형이라 셀렉트만 혼자 다르게 보였다.
          input={<OutlinedInput />}
          sx={{
            backgroundColor: 'var(--color-surface-card)',
            color: 'var(--color-text-primary)',
            // 같은 줄에 서는 컨트롤과 높이를 맞춘다 (DESIGN.md §Layout).
            height: 'var(--control-height)',
            fontSize: '13px',
            borderRadius: 'var(--radius-sm)',
            '& .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-default)' },
            '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-hover)' },
            '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-primary)' },
            '& .MuiNativeSelect-icon': { color: 'var(--color-text-secondary)' },
            '& .MuiNativeSelect-select': { paddingBlock: 0, paddingInlineStart: 'var(--space-sm)' },
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

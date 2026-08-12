// MUI Select 어댑터.
//
// 0c32836 전까지는 `NativeSelect`(진짜 <select>)였다. 닫힌 상태는 1d08d8f 에서
// 테두리형으로 맞췄지만, 펼친 목록은 OS/브라우저가 그리는 네이티브 팝업이라
// 두 테마 중 어느 쪽도 따르지 않았다 — CSS 가 닿지 않는 영역이다.
// 그래서 포털 Menu 를 렌더하는 MUI `Select` 로 바꿨고, 이름도 따라왔다.
//
//   잃은 것: 모바일 네이티브 피커, <select> 의 기본 키보드 동작(타이핑 점프 등)
//   얻은 것: 두 테마를 따르는 펼침 목록, 아이콘·구분선 등 커스텀 여지
//
// 호출부 17곳은 계속 <option> 을 넘긴다. MenuItem 변환은 여기서 한다(toMenuItems).
//
// ⚠️ react-hook-form 의 `register()` 로는 쓸 수 없다. MUI Select 의 값은 React
// 상태이고, RHF 가 reset()/setValue() 에서 하는 일은 "ref 로 받은 숨은 input 의
// .value 에 직접 대입"이다. 이벤트가 없으니 MUI 는 모른다 — 폼 상태만 바뀌고
// 화면은 옛 값에 머문다(조용히 어긋나므로 더 나쁘다). 폼 자리는 `Controller` 로
// 감싼다. 이 파일 옆의 select.test.tsx 가 그 계약을 고정한다.

import {
  Children,
  Fragment,
  forwardRef,
  isValidElement,
  useCallback,
  useId,
  type ChangeEvent,
  type FocusEventHandler,
  type HTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
} from 'react'
import FormControl from '@mui/material/FormControl'
import FormHelperText from '@mui/material/FormHelperText'
import InputLabel from '@mui/material/InputLabel'
import ListSubheader from '@mui/material/ListSubheader'
import MenuItem from '@mui/material/MenuItem'
import MuiSelect from '@mui/material/Select'
import OutlinedInput from '@mui/material/OutlinedInput'

/**
 * <optgroup> 의 제목.
 *
 * ListSubheader 를 그대로 넘기면 안 된다. MUI Select 는 자식 전부를
 * cloneElement 로 감싸며 role="option" 을 덮어씌우는데, cloneElement 의 props 가
 * 항상 이기므로 밖에서는 못 막는다 — 구분선이 선택지로 읽힌다.
 * 한 겹 감싸면 그 role 이 이 컴포넌트의 props 로 들어와 그냥 버려진다.
 */
function OptionGroupLabel({ label }: { label?: string }) {
  return <ListSubheader>{label}</ListSubheader>
}

/** MUI 의 포커스 핸들러 타깃. 숨은 input 이므로 <select> 가 아니다. */
type FocusHandler = FocusEventHandler<HTMLInputElement | HTMLTextAreaElement> | undefined

interface OptionProps {
  value?: string | number | readonly string[]
  label?: string
  disabled?: boolean
  children?: ReactNode
}

/** <option> 안의 글자. value 를 생략한 <option> 의 값을 만들 때 쓴다. */
function textOf(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(textOf).join('')
  if (isValidElement(node)) return textOf((node.props as OptionProps).children)
  return ''
}

/**
 * <option> / <optgroup> 트리를 MenuItem / ListSubheader 로 옮긴다.
 *
 * MUI Select 는 MenuItem 이 **직계 자식**이어야 값을 찾는다. 그래서 Fragment 와
 * optgroup 은 펼쳐서 평평하게 만든다. Children.map 이 배열 반환을 평탄화하고
 * key 도 붙여 준다.
 */
function toMenuItems(children: ReactNode): ReactNode {
  return Children.map(children, (child) => {
    if (!isValidElement(child)) return child

    if (child.type === Fragment) {
      return toMenuItems((child.props as OptionProps).children)
    }

    if (child.type === 'option') {
      const { value, children: label, disabled } = child.props as OptionProps
      return (
        // native <select> 와 같은 규칙: value 를 생략하면 글자가 곧 값이다.
        <MenuItem value={value ?? textOf(label)} disabled={disabled}>
          {label}
        </MenuItem>
      )
    }

    if (child.type === 'optgroup') {
      const { label, children: grouped } = child.props as OptionProps
      return [<OptionGroupLabel label={label} />, toMenuItems(grouped)]
    }

    return child
  })
}

// `color` / `size` 는 MUI 유니온과 충돌하므로 뺀다.
interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'color' | 'size'> {
  label?: string
  error?: string
}

export const Select = forwardRef<HTMLDivElement, SelectProps>(
  (
    {
      label,
      error,
      className,
      children,
      value,
      defaultValue,
      onChange,
      onBlur,
      onFocus,
      name,
      disabled,
      required,
      id,
      autoFocus,
      ...rest
    },
    ref,
  ) => {
    const generatedId = useId()
    const selectId = id ?? generatedId
    const labelId = `${selectId}-label`

    // ref 는 보이는 combobox 로 넘긴다. 호출부가 하는 일이 .focus() 인데,
    // 숨은 input(inputRef)이나 FormControl 루트에 걸면 아무 일도 일어나지 않는다.
    // SelectDisplayProps 로는 줄 수 없다 — MUI 가 자기 displayRef 를 먼저 걸고
    // 그 뒤에 SelectDisplayProps 를 펼치므로 우리 ref 가 그걸 덮어써서
    // 메뉴가 아예 열리지 않는다. 그래서 루트를 받아 combobox 를 찾아 건넨다.
    const attachRef = useCallback(
      (root: HTMLDivElement | null) => {
        const display = root?.querySelector<HTMLDivElement>('[role="combobox"]') ?? null
        if (typeof ref === 'function') ref(display)
        else if (ref) ref.current = display
      },
      [ref],
    )

    // fullWidth 를 쓰지 않는다. 소비자 다수가 툴바의 flex 컨테이너 안에서 이 컴포넌트를
    // 쓰는데, 100% 너비를 강제하면 나란히 있던 필터가 세로로 쌓여 화면당 정보가 줄어든다
    // — DESIGN.md §Components 의 밀도 규칙 위반이다. 너비는 호출자가 정한다.
    return (
      <FormControl ref={attachRef} error={Boolean(error)} variant="outlined" disabled={disabled} required={required}>
        {label && (
          // htmlFor 를 쓰지 않는다. MUI 가 렌더하는 건 <select> 가 아니라 div 라
          // <label for> 가 걸리지 않는다(비-labellable). 대신 labelId 로 잇는다 —
          // MUI 가 aria-labelledby 를 달고 라벨 클릭 시 포커스까지 옮겨 준다.
          <InputLabel id={labelId} shrink sx={{ color: 'var(--color-text-secondary)' }}>
            {label}
          </InputLabel>
        )}
        <MuiSelect
          id={selectId}
          labelId={label ? labelId : undefined}
          name={name}
          className={className}
          // 값이 없으면 '' 로 시작한다. undefined 로 두면 숨은 input 이
          // uncontrolled → controlled 로 넘어가며 React 가 경고한다.
          {...(value === undefined ? { defaultValue: defaultValue ?? '' } : { value })}
          onChange={(event) =>
            // MUI 가 주는 건 DOM 이벤트가 아니라 { target: { name, value } } 다.
            // 호출부가 읽는 건 e.target.value 하나뿐이라 계약은 그대로다.
            onChange?.(event as unknown as ChangeEvent<HTMLSelectElement>)
          }
          // 포커스 이벤트는 숨은 input 에서 온다 — 타깃 타입만 다르고 계약은 같다.
          onBlur={onBlur as FocusHandler}
          onFocus={onFocus as FocusHandler}
          autoFocus={autoFocus}
          // 라벨을 항상 shrink 로 띄우므로, 값이 비어도 표시 영역이 접히지 않게 한다.
          displayEmpty
          // outlined 는 FormControl 의 variant 만으로는 안 걸린다. Select 의 기본
          // 입력이 standard(밑줄)라서 아래 notchedOutline 규칙이 겨냥할 요소가
          // 렌더되지 않는다. label 을 함께 넘겨야 테두리에 라벨 자리(notch)가 파인다
          // — 안 넘기면 테두리 선이 라벨 글자를 관통한다.
          input={<OutlinedInput label={label} />}
          MenuProps={MENU_PROPS}
          // 남은 속성(data-*, aria-*, title …)은 보이는 combobox 에 붙인다.
          SelectDisplayProps={rest as HTMLAttributes<HTMLDivElement>}
          sx={CONTROL_SX}
        >
          {toMenuItems(children)}
        </MuiSelect>
        {error && <FormHelperText sx={{ color: 'var(--color-error)' }}>{error}</FormHelperText>}
      </FormControl>
    )
  },
)

Select.displayName = 'Select'

const CONTROL_SX = {
  backgroundColor: 'var(--color-surface-card)',
  color: 'var(--color-text-primary)',
  // 같은 줄에 서는 컨트롤과 높이를 맞춘다 (DESIGN.md §Layout).
  height: 'var(--control-height)',
  fontSize: '13px',
  borderRadius: 'var(--radius-sm)',
  '& .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-default)' },
  '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-border-hover)' },
  '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-primary)' },
  '&.Mui-error .MuiOutlinedInput-notchedOutline': { borderColor: 'var(--color-error)' },
  '& .MuiSelect-icon': { color: 'var(--color-text-secondary)' },
  '& .MuiSelect-select': {
    // MUI 기본 min-height(1.4375em)가 30px 칸을 밀어낸다.
    minHeight: 'unset',
    display: 'flex',
    alignItems: 'center',
    paddingBlock: 0,
    paddingInlineStart: 'var(--space-sm)',
  },
} as const

const MENU_PROPS = {
  // Menu 의 면(배경·테두리·그림자)은 mui-theme.ts 의 MuiMenu 가 이미 정한다.
  // 여기서는 위치와 최대 높이만 잡는다.
  slotProps: {
    paper: { sx: { marginBlockStart: 'var(--space-xs)', maxHeight: 320 } },
  },
} as const

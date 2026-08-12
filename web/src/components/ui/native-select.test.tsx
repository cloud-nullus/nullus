import { createRef } from 'react'
import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { Controller, useForm } from 'react-hook-form'
import { NativeSelect } from './native-select'

// 펼침 목록은 포털 Menu 다. mouseDown 이 여는 신호고(클릭이 아니다),
// 항목은 role="option" 으로 나온다.
const open = () => fireEvent.mouseDown(screen.getByRole('combobox'))
const pick = (name: string) => fireEvent.click(screen.getByRole('option', { name }))

describe('NativeSelect', () => {
  it('renders label and shows options when opened', () => {
    render(
      <NativeSelect label="Environment" defaultValue="dev">
        <option value="dev">Development</option>
        <option value="prod">Production</option>
      </NativeSelect>
    )

    // 라벨 글자는 두 군데 나온다 — 떠 있는 InputLabel 과 테두리 notch 의 <legend>.
    // 컨트롤에 붙었는지를 보려면 getByLabelText 로 확인한다.
    expect(screen.getByLabelText('Environment')).toBe(screen.getByRole('combobox'))
    // 닫혀 있으면 선택된 값만 보인다 — 나머지는 DOM 에 없다.
    expect(screen.getByRole('combobox').textContent).toBe('Development')
    expect(screen.queryByRole('option')).toBeNull()

    open()
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['Development', 'Production'])
  })

  it('calls onChange with the selected value on e.target.value', () => {
    const onChange = vi.fn()

    render(
      <NativeSelect label="Cluster" defaultValue="cluster-a" onChange={onChange}>
        <option value="cluster-a">Cluster A</option>
        <option value="cluster-b">Cluster B</option>
      </NativeSelect>
    )

    open()
    pick('Cluster B')

    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange.mock.calls[0][0].target.value).toBe('cluster-b')
    expect(screen.getByRole('combobox').textContent).toBe('Cluster B')
  })

  it('renders error message when error prop is provided', () => {
    render(
      <NativeSelect label="Namespace" error="Namespace is required">
        <option value="">Select namespace</option>
      </NativeSelect>
    )

    expect(screen.getAllByText('Namespace is required').length).toBeGreaterThan(0)
  })

  // native <select> 의 규칙: value 를 생략한 <option> 은 글자가 곧 값이다.
  // stack-info-panels 의 버전 목록이 그렇게 쓰고 있다.
  it('falls back to the option text when value is omitted', () => {
    const onChange = vi.fn()

    render(
      <NativeSelect defaultValue="1.2.0" onChange={onChange}>
        <option>1.2.0</option>
        <option>1.3.0</option>
      </NativeSelect>
    )

    open()
    pick('1.3.0')

    expect(onChange.mock.calls[0][0].target.value).toBe('1.3.0')
  })

  it('renders optgroup as a non-selectable subheader', () => {
    render(
      <NativeSelect defaultValue="small">
        <option value="small">Small</option>
        <optgroup label="Organization Profiles">
          <option value="org-1">Team A</option>
        </optgroup>
      </NativeSelect>
    )

    open()
    expect(screen.getByText('Organization Profiles')).not.toBeNull()
    // 구분선은 선택 대상이 아니다.
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['Small', 'Team A'])
  })

  // stack-install-page 가 "클러스터를 고르세요" 안내 후 이 셀렉트로 포커스를 보낸다.
  // ref 가 숨은 input 을 가리키면 focus() 가 눈에 띄지 않는다.
  it('forwards ref to the focusable combobox', () => {
    const ref = createRef<HTMLDivElement>()

    render(
      <NativeSelect ref={ref} defaultValue="a">
        <option value="a">A</option>
      </NativeSelect>
    )

    ref.current?.focus()
    expect(document.activeElement).toBe(screen.getByRole('combobox'))
  })

  // 이 컴포넌트를 register() 로 붙이면 reset()/setValue() 가 화면에 반영되지
  // 않는다(폼 상태만 바뀌고 표시는 옛 값에 머문다). Controller 가 정답이라는
  // 것을 여기서 못박는다 — 폼 자리 6곳이 이 계약에 기대고 있다.
  it('reflects react-hook-form reset() when wrapped in Controller', () => {
    function Form() {
      const { control, reset } = useForm({ defaultValues: { role: 'developer' } })
      return (
        <>
          <Controller
            name="role"
            control={control}
            render={({ field }) => (
              <NativeSelect label="Role" {...field}>
                <option value="developer">Developer</option>
                <option value="admin">Admin</option>
              </NativeSelect>
            )}
          />
          <button onClick={() => reset({ role: 'admin' })}>reset</button>
        </>
      )
    }

    render(<Form />)
    expect(screen.getByRole('combobox').textContent).toBe('Developer')

    fireEvent.click(screen.getByText('reset'))
    expect(screen.getByRole('combobox').textContent).toBe('Admin')
  })
})

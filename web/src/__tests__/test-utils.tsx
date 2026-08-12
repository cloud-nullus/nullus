import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClientProvider, QueryClient } from '@tanstack/react-query'
import type { ReactElement } from 'react'

export function renderWithProviders(ui: ReactElement, { route = '/' } = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[route]}>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>
  )
}

// 셀렉트를 다루는 테스트 헬퍼.
//
// NativeSelect 는 더 이상 <select> 가 아니다(components/ui/native-select.tsx 참고).
// 값은 숨은 input 에 있고 보이는 것은 role="combobox" 인 div, 목록은 포털 Menu 다.
// 그래서 예전 계약이 통하지 않는다:
//
//   fireEvent.change(select, { target: { value: 'x' } })  → 아무 일도 안 일어난다
//   getByDisplayValue('All Clusters')                     → 숨은 input 은 라벨이 아니라 값을 갖는다
//
// 대신 이 두 개를 쓴다. 화면에서 하는 동작(눌러서 연다 → 고른다)과 같은 순서다.
/** 화면에 보이는 셀렉트들. 순서는 DOM 순서다. */
export function getSelects(): HTMLElement[] {
  return screen.getAllByRole('combobox')
}

/** 지금 선택된 값의 표시 글자. 예전의 getByDisplayValue 자리. */
export function selectedLabel(select: HTMLElement): string {
  return select.textContent ?? ''
}

/** 표시 글자로 셀렉트를 찾는다. */
export function getSelectByValue(label: string | RegExp): HTMLElement {
  const match = getSelects().find((select) =>
    typeof label === 'string' ? selectedLabel(select) === label : label.test(selectedLabel(select)),
  )
  if (!match) {
    throw new Error(
      `표시 값이 "${label}" 인 셀렉트를 찾지 못했다. 현재: ${getSelects().map(selectedLabel).join(' | ')}`,
    )
  }
  return match
}

/** 셀렉트를 열고 이름으로 항목을 고른다. */
export function selectOption(select: HTMLElement, optionName: string | RegExp): void {
  fireEvent.mouseDown(select)
  const listbox = screen.getByRole('listbox')
  fireEvent.click(within(listbox).getByRole('option', { name: optionName }))
}

/** 셀렉트를 열고 항목의 value 로 고른다. 라벨이 길거나 동적일 때 쓴다. */
export function selectOptionByValue(select: HTMLElement, value: string): void {
  fireEvent.mouseDown(select)
  const listbox = screen.getByRole('listbox')
  const option = listbox.querySelector<HTMLElement>(`[data-value="${value}"]`)
  if (!option) {
    const available = Array.from(listbox.querySelectorAll('[data-value]'))
      .map((element) => element.getAttribute('data-value'))
      .join(', ')
    throw new Error(`value="${value}" 인 항목이 없다. 가능한 값: ${available}`)
  }
  fireEvent.click(option)
}

/** 펼친 목록의 항목 글자들. 목록 구성을 검증할 때 쓴다. */
export function optionLabels(select: HTMLElement): string[] {
  fireEvent.mouseDown(select)
  const labels = within(screen.getByRole('listbox'))
    .getAllByRole('option')
    .map((option) => option.textContent ?? '')
  fireEvent.keyDown(screen.getByRole('listbox'), { key: 'Escape' })
  return labels
}

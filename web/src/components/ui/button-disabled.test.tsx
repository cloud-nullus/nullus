import { describe, expect, it } from 'vitest'
import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { Button, type ButtonVariant } from './button'

const VARIANTS: ButtonVariant[] = ['primary', 'secondary', 'outline', 'danger', 'ghost']

// 비활성 버튼이 활성 버튼과 똑같이 보였다 — variant 색을 sx 로 덮으면서 MUI 의 비활성 색까지
// 덮였다. 눌러도 아무 일이 없는 버튼이 눌릴 것처럼 보이면 사용자는 고장으로 안다(2026-10-13
// 알림 채널 화면에서 빈 양식의 "저장"). 같은 색을 흐리게 해서 비활성임을 보인다.
describe('Button disabled 표현', () => {
  it.each(VARIANTS)('%s 비활성은 흐리고 커서가 금지 모양이다', (variant) => {
    render(
      <Button variant={variant} disabled>
        저장
      </Button>,
    )
    const button = screen.getByRole('button', { name: '저장' })
    expect(button).toBeDisabled()
    const style = getComputedStyle(button)
    expect(style.opacity).toBe('0.45')
    expect(style.cursor).toBe('not-allowed')
  })

  it('활성 버튼은 흐리지 않는다', () => {
    render(<Button variant="primary">저장</Button>)
    expect(getComputedStyle(screen.getByRole('button', { name: '저장' })).opacity).not.toBe('0.45')
  })

  // 진행 중(loading)은 스피너가 상태를 말한다 — 흐리게 하면 실패한 것처럼 보인다.
  it('진행 중 버튼은 흐리지 않는다', () => {
    render(
      <Button variant="primary" loading>
        저장
      </Button>,
    )
    expect(getComputedStyle(screen.getByRole('button', { name: /저장/ })).opacity).not.toBe('0.45')
  })
})

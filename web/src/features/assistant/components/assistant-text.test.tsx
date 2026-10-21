import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AssistantText } from './assistant-text'

// 모델은 지시와 무관하게 마크다운으로 답하는 일이 많다. 그대로 두면 **·#### 기호가 화면에 남는다
// (로컬 시연 녹화에서 "**completed**(정상)", "#### 스토리지 구성" 이 그대로 보였다).
describe('AssistantText', () => {
  it('should render bold without the asterisks', () => {
    const { container } = render(<AssistantText text="스택 상태는 **completed** 입니다." />)

    expect(screen.getByText('completed').tagName).toBe('STRONG')
    expect(container.textContent).toBe('스택 상태는 completed 입니다.')
  })

  it('should render headings as emphasized lines without the hashes', () => {
    const { container } = render(<AssistantText text={'#### 스토리지 구성\n내용'} />)

    expect(screen.getByText('스토리지 구성')).toBeTruthy()
    expect(container.textContent).not.toContain('#')
  })

  it('should render bullet and numbered lists as lists', () => {
    const { container } = render(<AssistantText text={'- GitLab: 실행 중\n- Argo CD: 실행 중\n\n1. 첫째\n2. 둘째'} />)

    expect(container.querySelectorAll('ul > li')).toHaveLength(2)
    expect(container.querySelectorAll('ol > li')).toHaveLength(2)
    expect(container.textContent).not.toMatch(/^- /m)
  })

  it('should render inline code and fenced code blocks', () => {
    const { container } = render(<AssistantText text={'`stack_list` 를 썼습니다.\n```\nkubectl get pods\n```'} />)

    expect(screen.getByText('stack_list').tagName).toBe('CODE')
    expect(container.querySelector('pre')?.textContent).toBe('kubectl get pods')
    expect(container.textContent).not.toContain('`')
  })

  it('should keep markup in the answer as plain text', () => {
    const { container } = render(<AssistantText text={'<img src=x onerror=alert(1)> **<b>굵게</b>**'} />)

    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    expect(container.textContent).toContain('<img src=x onerror=alert(1)>')
  })

  it('should keep plain lines as separate paragraphs', () => {
    const { container } = render(<AssistantText text={'첫 줄\n둘째 줄'} />)

    expect(container.querySelectorAll('p')).toHaveLength(2)
  })
})

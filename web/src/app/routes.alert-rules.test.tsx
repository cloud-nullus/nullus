import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

// 알림 규칙은 저장만 되고 평가되지 않는다(nullus-plan#87 — 스택 기본 규칙의 심각 경보만 전달하기로 했다).
// 동작하지 않는 화면을 메뉴에 두면 "규칙을 만들었는데 알림이 안 온다" 가 된다. 규칙 편집기가 생길 때까지 숨긴다.
describe('알림 규칙 화면 숨김', () => {
  const read = (p: string) => readFileSync(join(__dirname, p), 'utf-8')

  it('사이드바에 알림 규칙이 없고 알림 이력은 있다', () => {
    const nav = read('../components/layout/nav-model.tsx')
    expect(nav).not.toContain("label: 'sidebar.alertRules'")
    expect(nav).toContain("label: 'sidebar.alertHistory'")
  })

  it('알림 규칙 라우트를 등록하지 않는다', () => {
    const routes = read('./routes.tsx')
    expect(routes).not.toContain('<AlertRulesPage />')
    expect(routes).not.toContain("path: 'observability/alert-rules'")
    expect(routes).toContain("path: 'observability/alert-history'")
  })
})

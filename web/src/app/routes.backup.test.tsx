import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

// 화면을 만들고 컴포넌트 테스트까지 통과했는데 **닿을 방법이 없던** 적이 있다.
// lazy 선언과 아이콘 import 는 들어갔는데 라우트 항목과 메뉴 항목이 빠졌고,
// 컴포넌트를 직접 렌더하는 테스트라 아무것도 잡지 못했다.
//
// 선언이 아니라 **등록**을 본다.
describe('백업 화면 배선', () => {
  const read = (p: string) => readFileSync(join(__dirname, p), 'utf-8')

  it('라우트에 /admin/backup 이 등록돼 있다', () => {
    const routes = read('./routes.tsx')
    expect(routes).toContain("path: 'admin/backup'")
    expect(routes).toContain('<BackupPage />')
  })

  it('사이드바 메뉴에 백업이 있다', () => {
    const nav = read('../components/layout/nav-model.tsx')
    expect(nav).toContain("path: '/admin/backup'")
    expect(nav).toContain("label: 'sidebar.backup'")
  })

  it('메뉴 라벨의 번역 키가 두 언어에 다 있다', () => {
    for (const locale of ['ko', 'en']) {
      const dict = JSON.parse(read(`../i18n/${locale}.json`))
      expect(dict.sidebar?.backup, `${locale}.json 에 sidebar.backup 이 없다`).toBeTruthy()
    }
  })
})

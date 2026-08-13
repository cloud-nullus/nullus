import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

// 한 화면에는 아이콘 하나다. 기준은 메뉴(nav-model)이고 화면 제목(PageHeader)이 따라간다.
//
// 개편 중 메뉴 아이콘 8곳을 고쳤더니 제목은 옛 글리프로 남아, 같은 화면이
// 사이드바에서는 벌레인데 제목에서는 경고 삼각형이 되는 일이 생겼다.
// 그중 둘(/cicd/golden-paths, /stack/version)은 이번 작업 전부터 어긋나 있었다.
// 눈으로는 두 곳을 동시에 보기 어려워 놓치기 쉬우므로 여기서 기계로 대조한다.

const SRC = join(__dirname, '../..')

const navIconByRoute = (): Record<string, string> => {
  const nav = readFileSync(join(SRC, 'components/layout/nav-model.tsx'), 'utf8')
  const out: Record<string, string> = {}
  for (const m of nav.matchAll(/key: '\w+'[^}]*?path: '([^']+)'[^}]*?icon: <(\w+)/g)) {
    out[m[1]] = m[2]
  }
  return out
}

/** routes.tsx 에서 "라우트 → 페이지 파일" 을 복원한다. */
const pageFileByRoute = (): Record<string, string> => {
  const routes = readFileSync(join(SRC, 'app/routes.tsx'), 'utf8')
  const compToRoute: Record<string, string> = {}
  const fileToComp: Record<string, string> = {}
  for (const m of routes.matchAll(/path: '([^']+)', element: withSuspense\(<(\w+)/g)) {
    compToRoute[m[2]] = `/${m[1].replace(/^\//, '')}`
  }
  for (const m of routes.matchAll(/const (\w+) = lazy\(\(\) =>\s*import\('([^']+)'/g)) {
    fileToComp[m[2].split('/').pop() as string] = m[1]
  }
  const walk = (dir: string, out: string[] = []): string[] => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry)
      if (statSync(full).isDirectory()) walk(full, out)
      else if (/-page\.tsx$/.test(entry)) out.push(full)
    }
    return out
  }
  const out: Record<string, string> = {}
  for (const file of walk(join(SRC, 'features'))) {
    const comp = fileToComp[(file.split('/').pop() as string).replace(/\.tsx$/, '')]
    if (!comp) continue
    const route = compToRoute[comp]
    if (route) out[route] = file
  }
  return out
}

describe('메뉴와 화면 제목의 아이콘', () => {
  const nav = navIconByRoute()
  const pages = pageFileByRoute()

  it('gives each screen one icon, not two', () => {
    const mismatches: string[] = []
    let checked = 0
    for (const [route, navIcon] of Object.entries(nav)) {
      const file = pages[route]
      if (!file) continue
      const header = readFileSync(file, 'utf8').match(/icon=\{<(\w+)/)
      if (!header) continue
      checked++
      if (header[1] !== navIcon) {
        mismatches.push(`${route}: 메뉴 ${navIcon} ≠ 제목 ${header[1]}`)
      }
    }
    // 대조가 0건이면 위 정규식이 깨진 것이다 — 조용히 통과하지 않게 한다.
    expect(checked).toBeGreaterThanOrEqual(12)
    expect(mismatches, mismatches.join('\n')).toEqual([])
  })

  it('never gives two menu items in one group the same icon', () => {
    const nav = readFileSync(join(SRC, 'components/layout/nav-model.tsx'), 'utf8')
    const dupes: string[] = []
    // 그룹 헤더 icon 과 그 안의 children icon 을 한 덩어리로 본다.
    for (const group of nav.split(/\n {2}\{\n {4}key: /).slice(1)) {
      const icons = [...group.matchAll(/icon: <(\w+)/g)].map((m) => m[1])
      const seen = new Set<string>()
      for (const icon of icons) {
        if (seen.has(icon)) dupes.push(`${group.slice(0, group.indexOf("'", 1) + 1)} 안에서 ${icon} 중복`)
        seen.add(icon)
      }
    }
    expect(dupes, dupes.join('\n')).toEqual([])
  })
})

import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import ts from 'typescript'

// 한 화면에는 아이콘 하나다. 기준은 메뉴(nav-model)이고 화면 제목(PageHeader)이 따라간다.
//
// 개편 중 메뉴 아이콘 8곳을 고쳤더니 제목은 옛 글리프로 남아, 같은 화면이
// 사이드바에서는 벌레인데 제목에서는 경고 삼각형이 되는 일이 생겼다.
// 그중 둘(/cicd/golden-paths, /stack/version)은 이번 작업 전부터 어긋나 있었다.
// 눈으로는 두 곳을 동시에 보기 어려워 놓치기 쉬우므로 여기서 기계로 대조한다.

const SRC = join(__dirname, '../..')

interface ParsedNavNode {
  key: string
  path?: string
  icon?: string
  children: ParsedNavNode[]
}

/**
 * nav-model 의 navGroups 를 AST 로 읽는다.
 *
 * 예전에는 정규식으로 `key … path … icon` 을 잘라 읽었다. 메뉴가 3단(그룹 › 하위 그룹 ›
 * 항목)이 되면서 하위 그룹과 그 자식이 같은 path 를 갖게 됐고, 정규식은 어느 쪽이
 * 어느 쪽의 자식인지 모른다 — 그룹 단위로 자르던 분할도 중첩을 넘지 못한다.
 * 소스를 실행하지 않고 구조만 본다(아이콘은 JSX 태그 이름 그대로).
 */
const parseNav = (): ParsedNavNode[] => {
  const source = readFileSync(join(SRC, 'components/layout/nav-model.tsx'), 'utf8')
  const sf = ts.createSourceFile('nav-model.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  let groups: ts.ArrayLiteralExpression | undefined
  const find = (node: ts.Node) => {
    if (
      ts.isVariableDeclaration(node) &&
      node.name.getText(sf) === 'navGroups' &&
      node.initializer &&
      ts.isArrayLiteralExpression(node.initializer)
    ) {
      groups = node.initializer
    }
    ts.forEachChild(node, find)
  }
  find(sf)
  if (!groups) throw new Error('nav-model.tsx 에서 navGroups 배열을 찾지 못했다')

  const toNode = (obj: ts.ObjectLiteralExpression): ParsedNavNode => {
    const prop = (name: string) =>
      obj.properties.find(
        (p): p is ts.PropertyAssignment => ts.isPropertyAssignment(p) && p.name.getText(sf) === name,
      )
    const str = (name: string) => {
      const init = prop(name)?.initializer
      return init && ts.isStringLiteral(init) ? init.text : undefined
    }
    const iconInit = prop('icon')?.initializer
    const icon = iconInit
      ? ts.isJsxSelfClosingElement(iconInit)
        ? iconInit.tagName.getText(sf)
        : ts.isJsxElement(iconInit)
          ? iconInit.openingElement.tagName.getText(sf)
          : undefined
      : undefined
    const list = (prop('items') ?? prop('children'))?.initializer
    const children =
      list && ts.isArrayLiteralExpression(list)
        ? list.elements.filter(ts.isObjectLiteralExpression).map(toNode)
        : []
    return { key: str('key') ?? '?', path: str('path'), icon, children }
  }
  return groups.elements.filter(ts.isObjectLiteralExpression).map(toNode)
}

/** 잎(화면) 메뉴의 라우트 → 아이콘. 하위 그룹은 화면이 아니라 묶음이라 빠진다. */
const navIconByRoute = (): Record<string, string> => {
  const out: Record<string, string> = {}
  const visit = (nodes: ParsedNavNode[]) => {
    for (const node of nodes) {
      if (node.children.length > 0) visit(node.children)
      else if (node.path && node.icon) out[node.path] = node.icon
    }
  }
  for (const group of parseNav()) visit(group.children)
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
    const dupes: string[] = []
    // 그룹 헤더 icon 과 그 안의 항목 · 하위 그룹 · 하위 그룹의 자식 icon 을 한 덩어리로 본다.
    const icons = (node: ParsedNavNode): string[] => [
      ...(node.icon ? [node.icon] : []),
      ...node.children.flatMap(icons),
    ]
    for (const group of parseNav()) {
      const seen = new Set<string>()
      for (const icon of icons(group)) {
        if (seen.has(icon)) dupes.push(`${group.key} 안에서 ${icon} 중복`)
        seen.add(icon)
      }
    }
    expect(dupes, dupes.join('\n')).toEqual([])
  })

  // 파서가 중첩을 못 읽으면 위 두 검사가 하위 그룹을 통째로 건너뛰고도 통과한다.
  it('reads nested sub-groups, not only top-level items', () => {
    const observability = parseNav().find((group) => group.key === 'observability')
    const security = observability?.children.find((item) => item.key === 'security')
    expect(security?.icon).toBe('Shield')
    expect(security?.children.map((child) => [child.key, child.icon])).toEqual([
      ['securityDashboard', 'ShieldCheck'],
    ])
    expect(navIconByRoute()['/observability/security/dashboard']).toBe('ShieldCheck')
  })
})

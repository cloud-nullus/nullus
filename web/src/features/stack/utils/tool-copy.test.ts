import { describe, expect, it } from 'vitest'
import en from '../../../i18n/en.json'
import ko from '../../../i18n/ko.json'
import { TOOL_OPTIONS_BY_SLOT, findSlotOption, toolCopyKeys } from './tool-copy'

/** 점으로 이어진 키를 en/ko 에서 실제로 꺼내 본다 — i18next 가 하는 일과 같다. */
function resolve(source: unknown, key: string): string | undefined {
  const value = key.split('.').reduce<unknown>(
    (cur, part) => (cur !== null && typeof cur === 'object' ? (cur as Record<string, unknown>)[part] : undefined),
    source,
  )
  return typeof value === 'string' ? value : undefined
}

/** i18next 의 키 배열 규칙: 먼저 존재하는 키가 이긴다. */
function firstExisting(source: unknown, keys: string[]): string | undefined {
  for (const key of keys) {
    const value = resolve(source, key)
    if (value !== undefined) return value
  }
  return undefined
}

describe('도구 문구 찾기', () => {
  // 이 두 건이 같은 문구를 쓰던 것이 버그였다. GitLab 은 소스 저장소이자
  // 패키지 레지스트리인데 id 가 둘 다 gitlab 이라, id 로만 찾으면
  // "GitLab Package Registry" 밑에 소스 저장소 설명이 붙는다.
  it('같은 id 라도 슬롯이 다르면 다른 설명을 준다', () => {
    for (const locale of [en, ko]) {
      const asPackage = firstExisting(locale, toolCopyKeys('packageRegistry', 'gitlab', 'description'))
      const asSource = firstExisting(locale, toolCopyKeys('sourceRepository', 'gitlab', 'description'))

      expect(asPackage).toBeTruthy()
      expect(asSource).toBeTruthy()
      expect(asPackage).not.toBe(asSource)
    }
  })

  // 슬롯별 키를 갖는 것은 지금 gitlab 하나뿐이다. 나머지는 종전의 id 키로
  // 떨어져야 한다 — 그러지 않으면 27개 도구의 번역을 옮겨 적어야 한다.
  it('슬롯별 문구가 없으면 id 문구로 떨어진다', () => {
    const keys = toolCopyKeys('cdTool', 'argocd', 'description')
    expect(resolve(en, keys[0])).toBeUndefined()
    expect(resolve(en, keys[1])).toBeTruthy()
    expect(firstExisting(en, keys)).toBe(resolve(en, 'stackAddTools.tools.argocd.description'))
  })

  it('슬롯을 모르면 id 키만 본다', () => {
    expect(toolCopyKeys(undefined, 'argocd', 'label')).toEqual(['stackAddTools.tools.argocd.label'])
  })

  // 슬롯 이름은 옵션 그룹의 키에서 온다. 그룹을 가로질러 이름이 겹치면 한쪽이
  // 조용히 덮여 엉뚱한 도구 목록을 보게 된다.
  it('슬롯 이름이 그룹을 가로질러 겹치지 않는다', () => {
    const slots = Object.keys(TOOL_OPTIONS_BY_SLOT)
    expect(slots).toEqual([...new Set(slots)])
    expect(slots).toContain('packageRegistry')
    expect(slots).toContain('sourceRepository')
  })

  it('슬롯 안에서만 도구를 찾는다', () => {
    // 소스 저장소 슬롯에 Nexus 는 없다.
    expect(findSlotOption('sourceRepository', (o) => o.id === 'nexus')).toBeUndefined()
    expect(findSlotOption('packageRegistry', (o) => o.id === 'nexus')).toBeDefined()
  })
})

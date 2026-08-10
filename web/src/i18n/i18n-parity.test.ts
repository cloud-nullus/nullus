// en/ko 번역 키 정합 — 한쪽에만 있는 키는 그 로케일에서 폴백이 노출된다는 뜻이다.
//
// UI 개편 중 문자열이 조용히 사라지는 것을 막는 안전망 4번이다.
// 기획안: docs/40_UI_UX/Nullus_UIUX_전면개편_기획안.md §8

import { describe, expect, it } from 'vitest'
import en from './en.json'
import ko from './ko.json'

type Nested = { [key: string]: string | Nested }

function flatten(source: Nested, prefix = '', out: Record<string, string> = {}) {
  for (const [key, value] of Object.entries(source)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object') flatten(value, path, out)
    else out[path] = value
  }
  return out
}

const EN = flatten(en as Nested)
const KO = flatten(ko as Nested)

describe('i18n 키 정합', () => {
  it('en 에만 있는 키가 없다', () => {
    const missing = Object.keys(EN).filter((key) => !(key in KO))
    expect(missing, `ko.json 에 없는 키:\n${missing.join('\n')}`).toEqual([])
  })

  it('ko 에만 있는 키가 없다', () => {
    const missing = Object.keys(KO).filter((key) => !(key in EN))
    expect(missing, `en.json 에 없는 키:\n${missing.join('\n')}`).toEqual([])
  })

  // 한쪽 로케일만 빈 값인 것은 정당하다. 문장을 조각으로 쪼개 조립하는 곳에서
  // 어순이 달라 특정 언어에만 조각이 필요할 수 있다
  // (예: stackOssDefault.contract.end — 영어는 마침표를 JSX 리터럴로 두고 비운다).
  // 반면 모든 로케일에서 비어 있으면 그 키는 죽은 키다.
  it('모든 로케일에서 동시에 빈 값인 키가 없다', () => {
    const isBlank = (value: unknown) => typeof value === 'string' && value.trim() === ''
    const deadKeys = Object.keys(EN).filter((key) => isBlank(EN[key]) && isBlank(KO[key]))
    expect(deadKeys, `en/ko 모두 빈 값인 죽은 키:\n${deadKeys.join('\n')}`).toEqual([])
  })

  it('en/ko 가 같은 보간 변수를 쓴다', () => {
    const variables = (value: string) =>
      [...value.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1]).sort()
    const mismatched: string[] = []
    for (const [key, enValue] of Object.entries(EN)) {
      const koValue = KO[key]
      if (typeof enValue !== 'string' || typeof koValue !== 'string') continue
      const [a, b] = [variables(enValue), variables(koValue)]
      if (a.join(',') !== b.join(',')) {
        mismatched.push(`${key}: en={{${a.join(',')}}} ko={{${b.join(',')}}}`)
      }
    }
    expect(mismatched, `보간 변수 불일치:\n${mismatched.join('\n')}`).toEqual([])
  })
})

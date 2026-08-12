// 면(surface)은 글자색으로 만들지 않는다.
//
// 배경: CI/CD 목록의 "Deployed Resources" 패널이 라이트 테마에서 진회색 덩어리로
// 떠 글자가 거의 안 보였다. 원인은 배경을
//   bg-[color-mix(in_srgb,_var(--color-text-primary)_45%,_transparent)]
// 로 준 것이다. 글자색을 면으로 쓰면 테마가 뒤집힐 때 면도 같이 뒤집힌다 —
// 다크에서는 흰색 45% 라 적당한 회색이지만, 라이트에서는 검정 45% 라 진회색이
// 되고 그 위에 얹힌 text-primary(거의 검정)가 배경에 묻힌다.
//
// 옅은 틴트(≤12%)는 칩·배지 강조로 쓸 만하다. 그 이상은 면을 만들려는 의도이고,
// 면에는 --color-surface-* 토큰이 따로 있다 (base / card / sunken / raised).
// 이 토큰들은 테마별로 값이 정의돼 있어 뒤집혀도 관계가 유지된다.

import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const SRC = join(__dirname, '..')

/** 틴트로 허용하는 상한. 이보다 진하면 면을 만들려는 것이다. */
const MAX_TINT_PERCENT = 12

function sourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules') continue
    const path = join(dir, entry)
    if (statSync(path).isDirectory()) {
      out.push(...sourceFiles(path))
    } else if (/\.tsx?$/.test(path) && !/\.test\.tsx?$/.test(path)) {
      // 테스트는 위반 예시를 그대로 인용할 수 있으므로 검사 대상이 아니다.
      out.push(path)
    }
  }
  return out
}

describe('면은 글자색으로 만들지 않는다', () => {
  it(`bg 에 --color-text-primary 를 ${MAX_TINT_PERCENT}% 넘게 쓰지 않는다`, () => {
    const pattern = /bg-\[color-mix\(in_srgb,_var\(--color-text-primary\)_(\d+)%/g
    const violations: string[] = []

    for (const file of sourceFiles(SRC)) {
      const content = readFileSync(file, 'utf8')
      const lines = content.split('\n')
      lines.forEach((line, index) => {
        for (const match of line.matchAll(pattern)) {
          const percent = Number(match[1])
          if (percent > MAX_TINT_PERCENT) {
            violations.push(`${file.slice(SRC.length + 1)}:${index + 1} — ${percent}%`)
          }
        }
      })
    }

    expect(violations, '면에는 --color-surface-* 토큰을 쓴다').toEqual([])
  })
})

// 대비 감사 — index.css 의 디자인 토큰을 파싱해 라이트/다크 양쪽에서 WCAG AA 를 강제한다.
//
// 배경: 배포본 가독성 지적(2026-08 팀 논의)의 원인이 라이트 테마에서
//   ① 카드 배경과 페이지 배경이 같은 색이라 면이 구분되지 않고
//   ② 보더가 거의 검정이라 와이어프레임처럼 보이고
//   ③ 다크 기준으로 만든 상태색이 라이트에서 대비가 무너지는 것이었다.
// 스크린샷 회귀는 백엔드가 필요하지만 이 검사는 토큰만 보므로 항상 돌 수 있다.
//
// 기획안: docs/40_UI_UX/Nullus_UIUX_전면개편_기획안.md §2.1, §10

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const CSS = readFileSync(join(__dirname, '..', 'index.css'), 'utf8')

/** `selector { ... }` 블록에서 커스텀 프로퍼티를 뽑는다. */
function tokensIn(blockPattern: RegExp): Record<string, string> {
  const out: Record<string, string> = {}
  for (const match of CSS.matchAll(blockPattern)) {
    const body = match[1] ?? ''
    for (const decl of body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
      out[decl[1]] = decl[2].trim()
    }
  }
  return out
}

// `:root { ... }` 는 여러 번 나온다(토큰 블록 + color-scheme 블록) → 전부 병합
const rootTokens = tokensIn(/:root\s*\{([^}]*)\}/g)
const lightTokens = {
  ...rootTokens,
  ...tokensIn(/\[data-theme="light"\]\s*\{([^}]*)\}/g),
}

/** body 배경에서 대표 색 하나를 뽑는다(그라데이션이면 첫 색). */
function bodyBackground(scoped: boolean): string {
  const pattern = scoped
    ? /\[data-theme="light"\]\s*body\s*\{([^}]*)\}/
    : /(?<!\]\s)\bbody\s*\{([^}]*)\}/
  const block = CSS.match(pattern)?.[1] ?? ''
  const bg = block.match(/background\s*:\s*([^;]+);/)?.[1] ?? ''
  const hex = bg.match(/#[0-9a-fA-F]{3,8}/)
  if (!hex) throw new Error(`body 배경에서 색을 찾지 못했다: ${JSON.stringify(bg)}`)
  return hex[0]
}

function srgbToLinear(channel: number): number {
  const c = channel / 255
  return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
}

function relativeLuminance(hex: string): number {
  const raw = hex.replace('#', '')
  const full =
    raw.length === 3
      ? raw
          .split('')
          .map((c) => c + c)
          .join('')
      : raw.slice(0, 6)
  const [r, g, b] = [0, 2, 4].map((i) => Number.parseInt(full.slice(i, i + 2), 16))
  return 0.2126 * srgbToLinear(r) + 0.7152 * srgbToLinear(g) + 0.0722 * srgbToLinear(b)
}

export function contrastRatio(a: string, b: string): number {
  const [la, lb] = [relativeLuminance(a), relativeLuminance(b)]
  const [hi, lo] = la > lb ? [la, lb] : [lb, la]
  return (hi + 0.05) / (lo + 0.05)
}

function resolve(tokens: Record<string, string>, name: string): string {
  const value = tokens[name]
  if (!value) throw new Error(`토큰이 없다: ${name}`)
  const hex = value.match(/#[0-9a-fA-F]{3,8}/)
  if (!hex) throw new Error(`토큰 ${name} 이 단일 hex 가 아니다: ${value}`)
  return hex[0]
}

const AA = 4.5
/** 카드가 페이지 배경과 면으로 구분되는 최소 대비. 1.00 이면 완전히 같은 색이다. */
const SURFACE_SEPARATION = 1.05

const THEMES = [
  { name: 'dark', tokens: rootTokens, pageBg: bodyBackground(false) },
  { name: 'light', tokens: lightTokens, pageBg: bodyBackground(true) },
] as const

const TEXT_TOKENS = ['--color-text-primary', '--color-text-secondary', '--color-text-muted'] as const

describe('디자인 토큰 대비 감사', () => {
  describe.each(THEMES)('$name 테마', ({ tokens, pageBg }) => {
    it.each(TEXT_TOKENS)('%s 는 카드 배경에서 WCAG AA(4.5:1) 를 넘는다', (textToken) => {
      const ratio = contrastRatio(resolve(tokens, textToken), resolve(tokens, '--color-surface-card'))
      expect(ratio, `${textToken} on --color-surface-card = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it.each(TEXT_TOKENS)('%s 는 페이지 배경에서 WCAG AA(4.5:1) 를 넘는다', (textToken) => {
      const ratio = contrastRatio(resolve(tokens, textToken), pageBg)
      expect(ratio, `${textToken} on body(${pageBg}) = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it('카드 배경이 페이지 배경과 면으로 구분된다', () => {
      const card = resolve(tokens, '--color-surface-card')
      const ratio = contrastRatio(card, pageBg)
      expect(
        ratio,
        `--color-surface-card(${card}) vs body(${pageBg}) = ${ratio.toFixed(2)}:1 — 같은 색이면 카드가 사라진다`,
      ).toBeGreaterThanOrEqual(SURFACE_SEPARATION)
    })

    it('보더가 배경 대비 과하게 튀지 않는다 (와이어프레임 룩 방지)', () => {
      const border = resolve(tokens, '--color-border-default')
      const ratio = contrastRatio(border, resolve(tokens, '--color-surface-card'))
      // 경계선은 은은해야 한다. 본문 텍스트만큼 튀면 "흰 종이에 검은 선" = 스켈레톤처럼 보인다.
      expect(ratio, `--color-border-default(${border}) on card = ${ratio.toFixed(2)}:1`).toBeLessThan(AA)
    })
  })
})

// 대비 감사 — 생성된 디자인 토큰이 라이트/다크 양쪽에서 WCAG AA 를 넘는지 강제한다.
//
// 배경: 배포본 가독성 지적(2026-08 팀 논의)의 원인이 라이트 테마에서
//   ① 카드 배경과 페이지 배경이 같은 색이라 면이 구분되지 않고
//   ② 보더가 거의 검정이라 와이어프레임처럼 보이고
//   ③ 다크 기준으로 만든 상태색이 라이트에서 대비가 무너지는 것이었다.
//
// 대상은 src/theme/tokens.generated.css — web/DESIGN.md 의 파생물이다.
// 즉 이 테스트는 DESIGN.md 의 값이 실제로 접근성 기준을 만족하는지 검사한다.
// 스크린샷 회귀는 백엔드가 필요하지만 이 검사는 토큰만 보므로 항상 돌 수 있다.
//
// 기획안: docs/40_UI_UX/Nullus_UIUX_전면개편_기획안.md §2.1, §10

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const CSS = readFileSync(join(__dirname, '..', 'theme', 'tokens.generated.css'), 'utf8')

/** `selector { ... }` 블록에서 커스텀 프로퍼티를 뽑는다. */
function tokensIn(blockPattern: RegExp): Record<string, string> {
  const out: Record<string, string> = {}
  for (const match of CSS.matchAll(blockPattern)) {
    for (const decl of (match[1] ?? '').matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
      out[decl[1]] = decl[2].trim()
    }
  }
  return out
}

const darkTokens = tokensIn(/:root\s*\{([^}]*)\}/g)
const lightTokens = {
  ...darkTokens,
  ...tokensIn(/\[data-theme="light"\]\s*\{([^}]*)\}/g),
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
  { name: 'dark', tokens: darkTokens },
  { name: 'light', tokens: lightTokens },
] as const

/** 본문·보조 텍스트 3단 — DESIGN.md §Colors */
const TEXT_TOKENS = ['--color-text-primary', '--color-text-secondary', '--color-text-muted'] as const

/**
 * 상태색과 액센트. 이 값들이 텍스트·아이콘 색으로 쓰이므로 AA 를 넘어야 한다.
 * 개편 전에는 다크용 400대 톤을 라이트에서도 그대로 써서 1.60~2.85:1 이었다.
 */
const ACCENT_TOKENS = [
  '--color-primary',
  '--color-success',
  '--color-warning',
  '--color-error',
  '--color-info',
  '--color-accent-alt',
] as const

describe('디자인 토큰 대비 감사', () => {
  // 셀렉터가 안 맞으면 lightTokens 가 darkTokens 그대로가 되어 라이트 검사가
  // 조용히 다크를 두 번 보게 된다. 실제로 생성기의 인용부호가 바뀌었을 때 그렇게 됐다.
  it('라이트 토큰이 실제로 파싱됐다', () => {
    expect(Object.keys(darkTokens).length, ':root 토큰이 파싱되지 않았다').toBeGreaterThan(20)
    expect(
      lightTokens['--color-surface-card'],
      '[data-theme="light"] 블록이 파싱되지 않았다 — 라이트 검사가 다크를 보고 있다',
    ).not.toBe(darkTokens['--color-surface-card'])
  })

  describe.each(THEMES)('$name 테마', ({ tokens }) => {
    const card = () => resolve(tokens, '--color-surface-card')
    const page = () => resolve(tokens, '--color-surface-base')

    it.each(TEXT_TOKENS)('%s 는 카드 배경에서 AA(4.5:1) 를 넘는다', (token) => {
      const ratio = contrastRatio(resolve(tokens, token), card())
      expect(ratio, `${token} on card = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it.each(TEXT_TOKENS)('%s 는 페이지 배경에서 AA(4.5:1) 를 넘는다', (token) => {
      const ratio = contrastRatio(resolve(tokens, token), page())
      expect(ratio, `${token} on page = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it.each(ACCENT_TOKENS)('%s 는 카드 배경에서 AA(4.5:1) 를 넘는다', (token) => {
      const ratio = contrastRatio(resolve(tokens, token), card())
      expect(ratio, `${token} on card = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it.each(ACCENT_TOKENS)('%s 는 페이지 배경에서 AA(4.5:1) 를 넘는다', (token) => {
      const ratio = contrastRatio(resolve(tokens, token), page())
      expect(ratio, `${token} on page = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it('primary 버튼의 텍스트가 primary 배경에서 AA 를 넘는다', () => {
      const ratio = contrastRatio(resolve(tokens, '--color-on-primary'), resolve(tokens, '--color-primary'))
      expect(ratio, `on-primary on primary = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
    })

    it('카드 배경이 페이지 배경과 면으로 구분된다', () => {
      const ratio = contrastRatio(card(), page())
      expect(
        ratio,
        `card(${card()}) vs page(${page()}) = ${ratio.toFixed(2)}:1 — 같은 색이면 카드가 사라진다`,
      ).toBeGreaterThanOrEqual(SURFACE_SEPARATION)
    })

    it('보더가 배경 대비 과하게 튀지 않는다 (와이어프레임 룩 방지)', () => {
      const border = resolve(tokens, '--color-border-default')
      const ratio = contrastRatio(border, card())
      // 경계선은 은은해야 한다. 본문 텍스트만큼 튀면 "흰 종이에 검은 선" = 스켈레톤처럼 보인다.
      expect(ratio, `border(${border}) on card = ${ratio.toFixed(2)}:1`).toBeLessThan(AA)
    })
  })

  it('브랜드 골드 위의 텍스트가 AA 를 넘는다', () => {
    const ratio = contrastRatio(
      resolve(darkTokens, '--color-on-brand-gold'),
      resolve(darkTokens, '--color-brand-gold'),
    )
    expect(ratio, `on-brand-gold on brand-gold = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA)
  })

  it('브랜드 골드는 라이트 카드에서 텍스트로 쓸 수 없다 (DESIGN.md 금지 규칙의 근거)', () => {
    const ratio = contrastRatio(resolve(darkTokens, '--color-brand-gold'), resolve(lightTokens, '--color-surface-card'))
    expect(ratio, `골드를 텍스트로 쓰면 ${ratio.toFixed(2)}:1 — 면으로만 써야 한다`).toBeLessThan(AA)
  })
})

// CSS 변수를 실제 색 문자열로 해석한다.
//
// 왜 필요한가: chart.js 는 <canvas> 에 그리므로 CSS 변수를 해석하지 못한다.
// `borderColor: 'var(--color-info)'` 를 주면 색을 못 읽고 검게 렌더된다.
// 색 토큰화 코드모드가 차트 옵션에도 var() 를 넣어서 모니터링 차트가 통째로
// 검은 공백이 됐다 — 이 도우미가 그 자리를 메운다.
//
// recharts 는 SVG 라 var() 가 그대로 동작하므로 필요 없다. 다만 범례 텍스트처럼
// 캔버스가 아닌 곳에서도 이 함수를 써도 결과는 같다.

const FALLBACK = '#8496a9'

/**
 * 문자열 안의 모든 `var(--x)` 를 계산된 색으로 치환한다.
 *
 * 단독 변수뿐 아니라 `color-mix(in srgb, var(--color-info) 18%, transparent)` 처럼
 * 감싸인 경우도 처리한다 — 캔버스는 color-mix 는 해석하지만 그 안의 var() 는
 * 요소 문맥이 없어 풀지 못한다.
 * 변수가 없는 값(#hex, rgb())은 그대로 돌려준다.
 */
export function resolveColor(value: string, element?: Element | null): string {
  if (typeof value !== 'string') return FALLBACK
  if (!value.includes('var(') && !value.startsWith('--')) return value

  if (typeof window === 'undefined' || typeof getComputedStyle !== 'function') return FALLBACK
  const style = getComputedStyle(element ?? document.documentElement)
  const lookup = (name: string) => style.getPropertyValue(name).trim() || FALLBACK

  if (value.startsWith('--')) return lookup(value.trim())
  return value.replace(/var\(\s*(--[\w-]+)\s*(?:,[^)]*)?\)/g, (_, name: string) => lookup(name))
}

/** 알파를 섞은 색. 차트의 영역 채우기처럼 반투명이 필요한 곳에 쓴다. */
export function resolveColorWithAlpha(value: string, alpha: number, element?: Element | null): string {
  const base = resolveColor(value, element)
  // color-mix 는 캔버스도 해석하지만, 이미 해석된 색과 섞어야 예측 가능하다.
  return `color-mix(in srgb, ${base} ${Math.round(alpha * 100)}%, transparent)`
}

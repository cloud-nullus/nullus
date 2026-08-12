import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { NullusMark } from './nullus-mark'
import { useThemeStore } from '../../stores/theme-store'
import {
  MARK_OVERS,
  MARK_SEGMENTS,
  MARK_STROKE,
  MARK_GAP,
  MARK_DARK_MIN_RATIO,
} from './mark-geometry.generated'

const svgOf = (c: HTMLElement) => c.querySelector('svg') as SVGSVGElement
const strokes = (c: HTMLElement) =>
  [...c.querySelectorAll('svg > g > g path, svg > g > path')].map((p) => p.getAttribute('stroke'))

beforeEach(() => useThemeStore.setState({ theme: 'dark' }))

describe('NullusMark', () => {
  it('renders an accessible image with the brand name', () => {
    render(<NullusMark />)
    expect(screen.getByRole('img', { name: 'Nullus' })).toBeTruthy()
  })

  it('takes the requested size and keeps the 32 unit viewBox', () => {
    const { container } = render(<NullusMark size={18} />)
    expect(svgOf(container).getAttribute('width')).toBe('18')
    expect(svgOf(container).getAttribute('height')).toBe('18')
    expect(svgOf(container).getAttribute('viewBox')).toBe('0 0 32 32')
  })

  it('draws every strand of the knot in brand tone', () => {
    const { container } = render(<NullusMark />)
    // 아래로 깔리는 조각 + 위로 지나가는 가닥 + 마스크에서 간격을 내는 사본
    expect(container.querySelectorAll('path').length).toBe(
      MARK_SEGMENTS.length + MARK_OVERS.length * 2
    )
  })

  it('cuts the gap wider than the strand so the under strand actually breaks', () => {
    const { container } = render(<NullusMark />)
    const cut = container.querySelector('mask g') as SVGGElement
    expect(Number(cut.getAttribute('stroke-width'))).toBe(MARK_STROKE + MARK_GAP * 2)
    expect(Number(cut.getAttribute('stroke-width'))).toBeGreaterThan(MARK_STROKE)
    // 끊긴 자리가 위 가닥과 나란하려면 잘라내는 경로가 위 가닥과 같아야 한다.
    expect([...container.querySelectorAll('mask path')].map((p) => p.getAttribute('d'))).toEqual(
      MARK_OVERS.map(([d]) => d)
    )
  })

  // 한 화면에 두 개가 놓이면 mask id 가 겹쳐 한쪽 매듭이 통째로 사라진다.
  it('gives each instance its own mask id', () => {
    const { container } = render(
      <>
        <NullusMark />
        <NullusMark />
      </>
    )
    const ids = [...container.querySelectorAll('mask')].map((m) => m.id)
    expect(ids.length).toBe(2)
    expect(new Set(ids).size).toBe(2)
  })

  it('collapses to a single strand that inherits colour when tone is current', () => {
    const { container } = render(<NullusMark tone="current" />)
    const paths = container.querySelectorAll('path')
    expect(paths.length).toBe(1 + MARK_OVERS.length * 2)
    expect(paths[paths.length - 1].getAttribute('stroke')).toBe('currentColor')
  })

  it('hides itself from assistive tech when a label sits next to it', () => {
    const { container } = render(<NullusMark decorative />)
    expect(svgOf(container).getAttribute('aria-hidden')).toBe('true')
    expect(svgOf(container).getAttribute('role')).toBeNull()
  })

  // 어두운 바탕에서는 파랑·보라 쪽 절반이 배경으로 가라앉아 매듭이 끊겨 보인다.
  it('swaps to the lifted palette on dark and back again on light', () => {
    useThemeStore.setState({ theme: 'dark' })
    const { container: dark } = render(<NullusMark />)
    expect(strokes(dark)).toEqual([...MARK_SEGMENTS, ...MARK_OVERS].map(([, , d]) => d))

    useThemeStore.setState({ theme: 'light' })
    const { container: light } = render(<NullusMark />)
    expect(strokes(light)).toEqual([...MARK_SEGMENTS, ...MARK_OVERS].map(([, l]) => l))
  })

  it('keeps the single-colour tone free of the palette swap', () => {
    useThemeStore.setState({ theme: 'light' })
    const { container } = render(<NullusMark tone="current" />)
    expect(new Set(strokes(container))).toEqual(new Set(['currentColor']))
  })
})

// 다크 벌의 존재 이유는 대비 하나뿐이다. 그 약속을 값으로 고정한다 —
// 바탕 토큰이 밝아지면 마크만 조용히 과하게 밝은 채로 남는다.
describe('dark palette contrast', () => {
  const srgb = (c: number) => (c / 255 <= 0.04045 ? c / 255 / 12.92 : ((c / 255 + 0.055) / 1.055) ** 2.4)
  const luminance = (hex: string) => {
    const n = parseInt(hex.slice(1), 16)
    return 0.2126 * srgb((n >> 16) & 255) + 0.7152 * srgb((n >> 8) & 255) + 0.0722 * srgb(n & 255)
  }
  const ratio = (a: string, b: string) => {
    const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
    return (hi + 0.05) / (lo + 0.05)
  }

  // 생성기가 기준으로 삼은 바탕을 하드코딩하지 않고 실제 토큰에서 읽는다.
  const css = readFileSync(join(__dirname, '../../theme/tokens.generated.css'), 'utf8')
  const surfaceBase = css.match(/--color-surface-base:\s*(#[0-9a-fA-F]{6})/)?.[1] as string

  it('clears the promised ratio on the dark page background', () => {
    expect(surfaceBase).toBeTruthy()
    for (const [, , dark] of [...MARK_SEGMENTS, ...MARK_OVERS]) {
      const r = ratio(dark, surfaceBase)
      expect(r, `${dark} on ${surfaceBase} = ${r.toFixed(2)}:1`).toBeGreaterThanOrEqual(
        MARK_DARK_MIN_RATIO - 0.01
      )
    }
  })
})

// 파비콘은 번들을 타지 않는 정적 파일이라 컴포넌트와 따로 논다.
// 같은 생성기에서 나왔는지 경로로 확인한다 — 도형이 갈라지면 여기서 걸린다.
describe('favicon', () => {
  const favicon = readFileSync(join(__dirname, '../../../public/favicon.svg'), 'utf8')

  it('carries the same knot as the component', () => {
    for (const [d] of MARK_SEGMENTS) expect(favicon).toContain(d)
    for (const [d] of MARK_OVERS) expect(favicon).toContain(d)
  })

  it('names itself so tabs and bookmarks read as Nullus', () => {
    expect(favicon).toContain('<title>Nullus</title>')
  })

  // 파비콘은 React 밖의 독립 문서라 테마 스토어가 닿지 않는다. 브라우저 크롬은
  // OS 설정을 따르므로 여기서는 prefers-color-scheme 이 옳은 신호다.
  it('carries the lifted palette behind a prefers-color-scheme rule', () => {
    expect(favicon).toContain('@media (prefers-color-scheme:dark)')
    for (const [, , dark] of [...MARK_SEGMENTS, ...MARK_OVERS]) {
      expect(favicon).toContain(`stroke:${dark}`)
    }
  })
})

import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { StatusIcon, STATUS_ICON, STATUS_TOKEN, statusIcon, toneForStatus } from './status-icon'
import { iconProps } from './icon'
import { icon } from '../../theme/tokens.generated'

const svgOf = (c: HTMLElement) => c.querySelector('svg') as SVGSVGElement

describe('iconProps', () => {
  it('hands back size and stroke together', () => {
    for (const step of ['xs', 'sm', 'md', 'lg'] as const) {
      expect(iconProps(step).size).toBe(icon[step].size)
      expect(iconProps(step).strokeWidth).toBe(icon[step].strokeWidth)
      // 이름 없는 그래픽 300여 개가 읽히지 않게 기본으로 숨긴다.
      expect(iconProps(step)['aria-hidden']).toBe(true)
    }
  })

  // 작은 아이콘일수록 굵기를 올려야 렌더 굵기가 비슷해진다. 순서가 뒤집히면
  // 크기를 줄일 때 선이 다시 얇아진다 — 개편 전 상태로 돌아간다.
  it('raises the stroke as the icon gets smaller', () => {
    const steps = ['xs', 'sm', 'md', 'lg'] as const
    for (let i = 1; i < steps.length; i++) {
      expect(icon[steps[i]].size).toBeGreaterThan(icon[steps[i - 1]].size)
      expect(icon[steps[i]].strokeWidth).toBeLessThan(icon[steps[i - 1]].strokeWidth)
    }
  })
})

describe('StatusIcon', () => {
  it('paints itself from the tone token so one tone is one colour everywhere', () => {
    const { container } = render(<StatusIcon tone="success" />)
    expect(svgOf(container).style.color).toBe(`var(${STATUS_TOKEN.success})`)
  })

  it('leaves the colour alone when the wrapper already set it', () => {
    const { container } = render(<StatusIcon tone="success" inheritColor />)
    expect(svgOf(container).style.color).toBe('')
  })

  // 멈춘 스피너는 "도는 중" 이 아니라 "멈춤" 으로 읽힌다.
  it('spins only while running', () => {
    const { container: running } = render(<StatusIcon tone="running" />)
    expect(svgOf(running).getAttribute('class')).toContain('animate-spin')
    const { container: done } = render(<StatusIcon tone="success" />)
    expect(svgOf(done).getAttribute('class') ?? '').not.toContain('animate-spin')
  })

  it('stays silent for assistive tech unless it carries the meaning alone', () => {
    const { container: bare } = render(<StatusIcon tone="error" />)
    expect(svgOf(bare).getAttribute('aria-hidden')).toBe('true')
    const { container: labelled } = render(<StatusIcon tone="error" label="Failed" />)
    expect(svgOf(labelled).getAttribute('aria-label')).toBe('Failed')
    expect(svgOf(labelled).getAttribute('role')).toBe('img')
  })

  it('takes the stroke from the scale, not from lucide defaults', () => {
    const { container } = render(<StatusIcon tone="success" size="xs" />)
    expect(svgOf(container).getAttribute('stroke-width')).toBe(String(icon.xs.strokeWidth))
    expect(svgOf(container).getAttribute('width')).toBe(String(icon.xs.size))
  })
})

describe('toneForStatus', () => {
  // 개편 전에는 running 이 success 와, pending 이 warning 과 한 tone 이었다.
  it('keeps running apart from success', () => {
    expect(toneForStatus('running')).toBe('running')
    expect(toneForStatus('installing')).toBe('running')
    expect(toneForStatus('succeeded')).toBe('success')
  })

  it('keeps pending apart from warning', () => {
    expect(toneForStatus('pending')).toBe('pending')
    expect(toneForStatus('degraded')).toBe('warning')
  })

  it('falls back to neutral rather than guessing', () => {
    expect(toneForStatus('something-new')).toBe('neutral')
    expect(toneForStatus(null)).toBe('neutral')
  })
})

// DESIGN.md 가 단일 출처다. 표를 고치고 코드를 안 고치면(또는 그 반대면) 여기서 걸린다.
describe('DESIGN.md 와의 대조', () => {
  const design = readFileSync(join(__dirname, '../../../DESIGN.md'), 'utf8')

  it('carries every status row of the spec table', () => {
    const rows = [...design.matchAll(/^\|\s*[^|]+?\s*\|\s*`\*(-[a-z-]+)`\s*\|\s*`([A-Za-z0-9]+)`/gm)]
    expect(rows.length).toBeGreaterThanOrEqual(7)

    const inCode = Object.entries(STATUS_ICON).map(([tone, Glyph]) => ({
      token: STATUS_TOKEN[tone as keyof typeof STATUS_TOKEN],
      glyph: (Glyph as { displayName?: string }).displayName ?? '',
    }))

    for (const [, token, glyph] of rows) {
      expect(
        inCode.some((c) => c.glyph === glyph && c.token === `--color${token}`),
        `DESIGN.md 는 ${glyph} + --color${token} 을 규정하는데 레지스트리에 그 짝이 없다`
      ).toBe(true)
    }
  })

  it('states the same size scale the generator baked', () => {
    for (const [step, { size, strokeWidth }] of Object.entries(icon)) {
      expect(design).toMatch(
        new RegExp(`\\|\\s*\`${step}\`\\s*\\|\\s*${size}px\\s*\\|\\s*${strokeWidth}\\s*\\|`)
      )
    }
  })
})

// 같은 뜻이 다시 여러 글리프로 갈리는 것을 막는다. 전수조사에서 "성공" 하나에
// CheckCircle · CheckCircle2 · Check 셋이 쓰이고 있었고 화면마다 달랐다.
describe('아이콘 이름 규율', () => {
  const SRC = join(__dirname, '../..')
  const BANNED: Record<string, string> = {
    CheckCircle: 'CircleCheck',
    CheckCircle2: 'CircleCheck',
    XCircle: 'CircleX',
    AlertCircle: 'CircleAlert',
    AlertTriangle: 'TriangleAlert',
    Loader2: 'LoaderCircle',
    MinusCircle: 'CircleMinus',
  }

  const walk = (dir: string, out: string[] = []): string[] => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry)
      if (statSync(full).isDirectory()) walk(full, out)
      else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) out.push(full)
    }
    return out
  }

  it('sticks to the canonical lucide names — 별칭은 같은 뜻을 둘로 만든다', () => {
    const offenders: string[] = []
    for (const file of walk(SRC)) {
      const source = readFileSync(file, 'utf8')
      const importLine = source.match(/import\s*\{([^}]*)\}\s*from\s*['"]lucide-react['"]/)
      if (!importLine) continue
      for (const raw of importLine[1].split(',')) {
        const name = raw.trim().split(/\s+as\s+/)[0]
        if (BANNED[name]) offenders.push(`${file.replace(SRC, 'src')}: ${name} → ${BANNED[name]}`)
      }
    }
    expect(offenders, offenders.join('\n')).toEqual([])
  })
})

describe('statusIcon', () => {
  it('returns the table row so screens never pick a glyph', () => {
    expect(statusIcon('success')).toBe(STATUS_ICON.success)
    expect(statusIcon('error')).toBe(STATUS_ICON.error)
  })
})

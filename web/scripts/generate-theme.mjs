// DESIGN.md → 코드. 디자인 단일 출처의 파생 단계다.
//
//   web/DESIGN.md  (사람이 고치는 유일한 곳)
//         │
//         ├─→ src/theme/tokens.generated.ts    MUI 테마 · AG Grid 테마가 읽는다
//         └─→ src/theme/tokens.generated.css   Tailwind 임의값 클래스가 읽는 --color-* 토큰
//
// 런타임에 MUI CSS 변수를 참조하지 않고 빌드 시점에 굽는 이유:
// index.css 는 JS 보다 먼저 로드되므로, --color-* 가 --mui-palette-* 에 의존하면
// 첫 페인트에서 값이 비어 색이 무너진다. 코드젠은 그 창을 없앤다.
//
// 사용법:
//   node scripts/generate-theme.mjs           # 생성
//   node scripts/generate-theme.mjs --check    # 생성물이 최신인지 검사 (CI)

import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
// js-yaml v5 는 순수 ESM 으로 named export 만 제공한다 (default 없음).
import { load as loadYaml } from 'js-yaml'

const WEB_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const DESIGN = join(WEB_ROOT, 'DESIGN.md')
const OUT_TS = join(WEB_ROOT, 'src', 'theme', 'tokens.generated.ts')
const OUT_CSS = join(WEB_ROOT, 'src', 'theme', 'tokens.generated.css')

const BANNER = `DESIGN.md 에서 자동 생성됨 — 직접 고치지 않는다.
값을 바꾸려면 web/DESIGN.md 를 고치고 \`npm run theme:generate\` 를 실행한다.`

function parseFrontMatter(path) {
  const raw = readFileSync(path, 'utf8')
  const match = raw.match(/^---\r?\n([\s\S]*?)\r?\n---/)
  if (!match) throw new Error(`${path} 에 YAML front matter 가 없다`)
  return loadYaml(match[1])
}

/** `{colors.dark-primary}` 같은 토큰 참조를 실제 값으로 해석한다. */
function resolveReferences(tokens) {
  const lookup = (path) => {
    const value = path.split('.').reduce((node, key) => (node == null ? node : node[key]), tokens)
    if (value === undefined) throw new Error(`깨진 토큰 참조: {${path}}`)
    return value
  }
  const resolve = (value, depth = 0) => {
    if (depth > 10) throw new Error(`토큰 참조가 순환한다: ${value}`)
    if (typeof value !== 'string') return value
    const ref = value.match(/^\{([\w.-]+)\}$/)
    return ref ? resolve(lookup(ref[1]), depth + 1) : value
  }
  const walk = (node) =>
    Object.fromEntries(
      Object.entries(node).map(([key, value]) => [
        key,
        value !== null && typeof value === 'object' ? walk(value) : resolve(value),
      ]),
    )
  return walk(tokens)
}

/** `light-primary` → `primary` 로 접두어를 벗겨 스킴별 팔레트를 만든다. */
function scheme(colors, prefix) {
  const out = {}
  for (const [key, value] of Object.entries(colors)) {
    if (key.startsWith(`${prefix}-`)) out[key.slice(prefix.length + 1)] = value
  }
  return out
}

const design = resolveReferences(parseFrontMatter(DESIGN))
const light = scheme(design.colors, 'light')
const dark = scheme(design.colors, 'dark')
const brand = {
  gold: design.colors['brand-gold'],
  goldEnd: design.colors['brand-gold-end'],
  onGold: design.colors['on-brand-gold'],
}

// 레이아웃 값은 DESIGN.md 의 `layout` 블록이 단일 출처다. 예전에는 이 파일에
// 하드코딩돼 있었는데, 그러면 "값을 바꿀 곳은 DESIGN.md 하나" 라는 계약이
// 절반만 참이 된다. 기본값은 블록이 통째로 빠졌을 때의 안전망일 뿐이다.
const LAYOUT_DEFAULTS = {
  'sidebar-width': '240px',
  'sidebar-collapsed': '48px',
  'header-height': '44px',
  'page-padding': '24px',
  'page-padding-y': '20px',
  'card-padding': '12px',
  'grid-gap': '8px',
  'icon-size': '28px',
  'table-cell-px': '12px',
  'table-row-height': '32px',
  'table-header-height': '28px',
  'control-height': '30px',
}
const layout = { ...LAYOUT_DEFAULTS, ...(design.layout ?? {}) }
const layoutVars = Object.entries(layout)
  .map(([key, value]) => `  --${key}: ${value};`)
  .join('\n')

for (const [name, palette] of [
  ['light', light],
  ['dark', dark],
]) {
  const required = [
    'bg',
    'surface',
    'divider',
    'divider-strong',
    'text',
    'text-secondary',
    'text-muted',
    'primary',
    'on-primary',
    'success',
    'warning',
    'error',
    'info',
    'accent-alt',
    'scrim',
  ]
  const missing = required.filter((key) => !(key in palette))
  if (missing.length) {
    throw new Error(`DESIGN.md 의 ${name} 스킴에 필수 토큰이 없다: ${missing.join(', ')}`)
  }
}

// ── TypeScript 토큰 ───────────────────────────────────────────────────────
const ts = `/**
 * ${BANNER.split('\n').join('\n * ')}
 */

export interface SchemeTokens {
  bg: string
  surface: string
  surfaceSunken: string
  surfaceRaised: string
  divider: string
  dividerStrong: string
  text: string
  textSecondary: string
  textMuted: string
  primary: string
  onPrimary: string
  success: string
  warning: string
  error: string
  info: string
  accentAlt: string
  scrim: string
}

const schemeFrom = (p: Record<string, string>): SchemeTokens => ({
  bg: p['bg'],
  surface: p['surface'],
  // 라이트는 sunken(표 헤더 등), 다크는 raised(오버레이)만 정의한다.
  // 없는 쪽은 surface 로 접어 두 스킴이 같은 형태를 갖게 한다.
  surfaceSunken: p['surface-sunken'] ?? p['surface'],
  surfaceRaised: p['surface-raised'] ?? p['surface'],
  divider: p['divider'],
  dividerStrong: p['divider-strong'],
  text: p['text'],
  textSecondary: p['text-secondary'],
  textMuted: p['text-muted'],
  primary: p['primary'],
  onPrimary: p['on-primary'],
  success: p['success'],
  warning: p['warning'],
  error: p['error'],
  info: p['info'],
  accentAlt: p['accent-alt'],
  scrim: p['scrim'],
})

export const lightTokens: SchemeTokens = schemeFrom(${JSON.stringify(light, null, 2)})

export const darkTokens: SchemeTokens = schemeFrom(${JSON.stringify(dark, null, 2)})

export const brandTokens = ${JSON.stringify(brand, null, 2)} as const

export const elevation = ${JSON.stringify(design.elevation ?? {}, null, 2)} as const

export const spacing = ${JSON.stringify(design.spacing ?? {}, null, 2)} as const

export const rounded = ${JSON.stringify(design.rounded ?? {}, null, 2)} as const

export const typography = ${JSON.stringify(design.typography ?? {}, null, 2)} as const

export const layout = ${JSON.stringify(layout, null, 2)} as const
`

// ── CSS 토큰 ──────────────────────────────────────────────────────────────
// 이름은 기존 코드가 이미 쓰는 것을 그대로 유지한다. TSX 의 임의값 클래스
// 1,582곳이 이 이름들을 참조하므로 바꾸면 화면이 통째로 깨진다.
const cssVars = (t, el) => `  --color-surface-base: ${t.bg};
  --color-surface-card: ${t.surface};
  --color-surface-sunken: ${t['surface-sunken'] ?? t.surface};
  --color-surface-raised: ${t['surface-raised'] ?? t.surface};
  --color-surface-overlay: ${t.scrim};

  --color-border-default: ${t.divider};
  --color-border-hover: ${t['divider-strong']};
  /* 개편 전 코드가 참조하는데 정의가 없던 이름. 별칭으로 살려 둔다. */
  --color-border: ${t.divider};

  --color-text-primary: ${t.text};
  --color-text-secondary: ${t['text-secondary']};
  --color-text-muted: ${t['text-muted']};
  /* 위와 같은 이유의 별칭 (--color-text-tertiary → muted). */
  --color-text-tertiary: ${t['text-muted']};

  --color-primary: ${t.primary};
  --color-on-primary: ${t['on-primary']};
  --color-success: ${t.success};
  --color-warning: ${t.warning};
  --color-error: ${t.error};
  --color-info: ${t.info};
  --color-accent-alt: ${t['accent-alt']};

  --shadow-raised: ${el.raised};
  --shadow-overlay: ${el.overlay};

  /* 터미널(로그 출력) — DESIGN.md §Colors */
  --color-terminal-bg: ${t['terminal-bg']};
  --color-terminal-text: ${t['terminal-text']};
  --color-terminal-muted: ${t['terminal-muted']};
  --color-terminal-info: ${t['terminal-info']};
  --color-terminal-success: ${t['terminal-success']};
  --color-terminal-warning: ${t['terminal-warning']};
  --color-terminal-error: ${t['terminal-error']};`

const sidebar = (t) => `  --color-sidebar-border: ${t.divider};
  --color-sidebar-group-text: ${t['text-secondary']};
  --color-sidebar-item-text: ${t.text};
  --color-sidebar-item-active-text: ${t.primary};
  --color-sidebar-item-active-bg: color-mix(in srgb, ${t.primary} 10%, transparent);
  --color-sidebar-item-active-border: ${t.primary};`

const el = design.elevation ?? {}
const css = `/*
 * ${BANNER.split('\n').join('\n * ')}
 */

/* 다크가 기본 스킴이다. */
:root {
  color-scheme: dark;

${cssVars(design.colors ? scheme(design.colors, 'dark') : {}, { raised: el['raised-dark'] ?? 'none', overlay: el['overlay-dark'] ?? 'none' })}

${sidebar(scheme(design.colors, 'dark'))}

  --color-brand-gold: ${brand.gold};
  --color-brand-gold-end: ${brand.goldEnd};
  --color-on-brand-gold: ${brand.onGold};

  /* 장식 그라데이션을 쓰지 않는다(§Do's and Don'ts). 히어로도 면 하나다. */
  --color-home-hero-bg: ${dark['surface-sunken'] ?? dark.surface};

  /* 레이아웃 — DESIGN.md §Layout (front matter 의 layout 블록이 단일 출처다) */
${layoutVars}
  --card-radius: ${design.rounded?.lg ?? '8px'};
  --icon-radius: ${design.rounded?.sm ?? '4px'};

  /* 간격 스케일 — DESIGN.md §Layout. 스케일 밖의 값을 쓰지 않는다. */
${Object.entries(design.spacing ?? {})
  .map(([key, value]) => `  --space-${key}: ${value};`)
  .join('\n')}

  --radius-sm: ${design.rounded?.sm ?? '6px'};
  --radius-md: ${design.rounded?.md ?? '10px'};
  --radius-lg: ${design.rounded?.lg ?? '12px'};
  --radius-full: ${design.rounded?.full ?? '9999px'};

  --transition-default: 200ms ease;
  --transition-fast: 150ms ease;

  --z-page-header: 30;
  --z-sidebar: 40;
  --z-modal: 50;
  --z-toast: 60;
}

[data-theme="light"] {
  color-scheme: light;

${cssVars(scheme(design.colors, 'light'), { raised: el.raised ?? 'none', overlay: el.overlay ?? 'none' })}

${sidebar(scheme(design.colors, 'light'))}

  --color-home-hero-bg: ${light['surface-sunken'] ?? light.surface};
}
`

const outputs = [
  [OUT_TS, ts],
  [OUT_CSS, css],
]

if (process.argv.includes('--check')) {
  const stale = outputs.filter(([path, content]) => !existsSync(path) || readFileSync(path, 'utf8') !== content)
  if (stale.length === 0) {
    console.log('테마 생성물이 DESIGN.md 와 일치한다')
    process.exit(0)
  }
  console.error('DESIGN.md 와 생성물이 어긋난다:')
  for (const [path] of stale) console.error(`  - ${path.replace(`${WEB_ROOT}/`, '')}`)
  console.error('\n`npm run theme:generate` 를 실행하고 결과를 커밋한다.')
  process.exit(1)
}

for (const [path, content] of outputs) {
  writeFileSync(path, content)
  console.log(`생성: ${path.replace(`${WEB_ROOT}/`, '')}`)
}

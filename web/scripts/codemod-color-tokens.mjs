// 하드코딩된 색을 디자인 토큰으로 치환한다. 일회성 코드모드다.
//
// 왜 필요한가: TSX 에 박힌 색은 전부 다크 기준으로 작성돼 라이트 테마에서 무너진다.
// 특히 rgba(255,255,255,0.04) 같은 흰색 오버레이는 입력 배경으로 쓰이는데,
// 라이트 테마에서는 흰 배경 위에 흰색을 얹어 아무것도 보이지 않는다.
// 토큰은 테마별로 다른 값을 가지므로 치환만으로 두 테마가 함께 맞는다.
//
// 안전장치
//   - 매핑에 없는 색은 건드리지 않고 목록으로 보고한다. 조용히 뭉개지 않는다.
//   - 알파는 백분율로 보존한다 (0.15 → 15%).
//   - 테스트 파일과 src/theme/** 는 제외한다. 테마는 색을 정의하는 곳이다.
//   - 주석 안의 색은 건드리지 않는다. 옛 값을 설명하는 문장이 뜻을 잃는다.
//
// 사용법: node scripts/codemod-color-tokens.mjs [--dry]

import { readFileSync, writeFileSync, readdirSync, statSync } from 'node:fs'
import { join, dirname, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const SRC = join(dirname(fileURLToPath(import.meta.url)), '..', 'src')
const DRY = process.argv.includes('--dry')

/** 불투명 hex → 시맨틱 토큰. 같은 의미의 여러 명암은 한 토큰으로 모은다. */
const HEX_TO_TOKEN = {
  // 상태 — DESIGN.md §Colors. 명암 변화는 테마별 토큰이 대신한다.
  '#ef4444': 'error', '#f87171': 'error', '#fca5a5': 'error', '#dc2626': 'error',
  '#22c55e': 'success', '#34d399': 'success', '#86efac': 'success', '#6ee7b7': 'success',
  '#10b981': 'success', '#4ade80': 'success',
  '#f59e0b': 'warning', '#fbbf24': 'warning', '#fcd34d': 'warning', '#fde047': 'warning',
  '#eab308': 'warning', '#d97706': 'warning',
  '#3b82f6': 'info', '#60a5fa': 'info', '#93c5fd': 'info', '#38bdf8': 'info',
  '#0ea5e9': 'info', '#2563eb': 'info',
  // 액센트
  '#6366f1': 'primary', '#818cf8': 'primary', '#a5b4fc': 'primary', '#4f46e5': 'primary',
  '#8b5cf6': 'accent-alt', '#a78bfa': 'accent-alt', '#c4b5fd': 'accent-alt',
  '#d8b4fe': 'accent-alt', '#a855f7': 'accent-alt',
  // 텍스트
  '#f1f5f9': 'text-primary', '#e2e8f0': 'text-primary', '#f8fafc': 'text-primary',
  '#94a3b8': 'text-secondary', '#cbd5e1': 'text-secondary',
  '#64748b': 'text-muted', '#475569': 'text-muted', '#9ca3af': 'text-muted',
  // 회색조 — 보더와 표면
  '#e5e7eb': 'border-default', '#374151': 'border-default', '#4a5568': 'border-default',
  '#2d3748': 'border-default', '#1f2937': 'border-default',
  // 어두운 표면 (코드/로그 뷰어 배경). 라이트 테마에서 검정 배경이 남던 자리다.
  '#0d1117': 'surface-base', '#0b1220': 'surface-base', '#111827': 'surface-base',
  '#0f1419': 'surface-card', '#0a0a0a': 'surface-base',
  // 골드 CTA 위의 텍스트색. surface-base 로 두면 라이트 테마에서 거의 흰색이 되어
  // 골드 위 흰 글자(1.9:1)가 된다 — 표면이 아니라 "골드 위 전경색" 이다.
  '#1a1d29': 'on-brand-gold',
  // 브랜드
  '#ffd700': 'brand-gold',
}

/** rgba 의 RGB 삼중항 → 시맨틱 토큰. 알파는 색 혼합 비율로 옮긴다. */
const RGB_TO_TOKEN = {
  '239,68,68': 'error', '248,113,113': 'error', '220,38,38': 'error',
  '34,197,94': 'success', '16,185,129': 'success', '52,211,153': 'success',
  '245,158,11': 'warning', '251,191,36': 'warning', '234,179,8': 'warning',
  '59,130,246': 'info', '96,165,250': 'info', '14,165,233': 'info',
  '99,102,241': 'primary', '129,140,248': 'primary', '79,70,229': 'primary',
  '139,92,246': 'accent-alt', '167,139,250': 'accent-alt', '168,85,247': 'accent-alt',
  '236,72,153': 'accent-alt',
  // 흰색 오버레이는 "본문색을 아주 옅게" 라는 뜻이다. 다크에서는 흰색에 가깝고
  // 라이트에서는 검정에 가까우므로, 미묘한 표면 강조가 두 테마에서 모두 성립한다.
  // 개편 전에는 라이트 테마에서 흰 배경 위 흰색이라 아무것도 보이지 않았다.
  '255,255,255': 'text-primary',
  // 검정 오버레이도 같은 논리의 반대편이다.
  '0,0,0': 'text-primary',
  // 회색조 틴트 — 중립 강조. 보조 텍스트색을 옅게 쓰는 것과 같은 의미다.
  '148,163,184': 'text-secondary',
  '100,116,139': 'text-muted', '107,114,128': 'text-muted',
  // 어두운 남색 오버레이 (스크림·강조 배경)
  '15,23,42': 'text-primary', '30,41,59': 'text-primary',
  // 주황은 경고 계열로 모은다
  '249,115,22': 'warning',
  '255,215,0': 'brand-gold',
}

const token = (name) => `var(--color-${name})`
const mix = (name, alpha) =>
  `color-mix(in srgb, var(--color-${name}) ${Number((alpha * 100).toFixed(2))}%, transparent)`

/**
 * Tailwind 임의값(`bg-[...]`) 안에는 공백을 쓸 수 없다 — 밑줄로 써야 클래스가 산다.
 * 이걸 빼먹어서 color-mix 를 넣은 클래스 676개가 조용히 죽었다. 테스트·빌드·시각
 * 회귀 모두 통과했는데(배지 배경이 사라진 채 스냅샷이 갱신됐다) 클래스만 무효였다.
 */
function normalizeTailwindArbitraryValues(source) {
  return source.replace(
    /\b([a-z][a-z0-9-]*)-\[([^\]]*(?:color-mix|var\(--)[^\]]*)\]/g,
    (whole, utility, inner) =>
      inner.includes(' ') ? `${utility}-[${inner.replace(/ /g, '_')}]` : whole,
  )
}

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) {
      if (entry === 'theme') continue // 토큰을 정의하는 곳
      walk(full, out)
    } else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) {
      out.push(full)
    }
  }
  return out
}

const unmapped = new Map()
const note = (value, file) => {
  if (!unmapped.has(value)) unmapped.set(value, new Set())
  unmapped.get(value).add(relative(SRC, file))
}

let changedFiles = 0
let replacements = 0

/**
 * 주석을 자리표시자로 빼두고 치환한 뒤 되돌린다.
 * 주석은 "예전에 #f87171 을 썼다" 처럼 옛 값을 설명하므로 치환하면 문장이 망가진다.
 */
function protectComments(source) {
  const comments = []
  const masked = source.replace(/\/\*[\s\S]*?\*\/|\/\/[^\n]*/g, (m) => {
    comments.push(m)
    return `\u0000C${comments.length - 1}\u0000`
  })
  const restore = (text) => text.replace(/\u0000C(\d+)\u0000/g, (_, i) => comments[Number(i)])
  return { masked, restore }
}

for (const file of walk(SRC)) {
  const before = readFileSync(file, 'utf8')
  const { masked, restore } = protectComments(before)
  let after = masked

  after = after.replace(/rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([\d.]+)\s*\)/g, (whole, r, g, b, a) => {
    const key = `${r},${g},${b}`
    const name = RGB_TO_TOKEN[key]
    if (!name) {
      note(whole, file)
      return whole
    }
    replacements += 1
    return mix(name, Number(a))
  })

  after = after.replace(/#[0-9a-fA-F]{6}\b/g, (whole) => {
    const name = HEX_TO_TOKEN[whole.toLowerCase()]
    if (!name) {
      note(whole, file)
      return whole
    }
    replacements += 1
    return token(name)
  })

  after = normalizeTailwindArbitraryValues(after)
  after = restore(after)

  if (after !== before) {
    changedFiles += 1
    if (!DRY) writeFileSync(file, after)
  }
}

console.log(`${DRY ? '[dry] ' : ''}치환 ${replacements}건 / 파일 ${changedFiles}개`)

if (unmapped.size) {
  console.log(`\n매핑에 없어 그대로 둔 색 ${unmapped.size}종 — 직접 판단이 필요하다:`)
  for (const [value, files] of [...unmapped].sort((a, b) => b[1].size - a[1].size)) {
    console.log(`  ${value.padEnd(34)} ${[...files].slice(0, 3).join(', ')}${files.size > 3 ? ` 외 ${files.size - 3}` : ''}`)
  }
}

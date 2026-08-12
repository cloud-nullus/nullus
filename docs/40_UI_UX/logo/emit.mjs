/**
 * 최종 마크(세잎 매듭)의 도형을 계산해 정적 산출물로 굽는다.
 *
 * 매듭은 손으로 그리면 반드시 틀리므로 매개변수식에서 점을 만들고
 * 자기교차점을 수치로 찾는다. 결과는 좌표가 박힌 SVG 이므로
 * 앱은 이 스크립트를 실행하지 않는다 — 도형을 고칠 때만 다시 돌린다.
 */
import { writeFileSync } from 'node:fs'

const f = (n) => String(Math.round(n * 100) / 100)

const trefoil = (t) => [Math.sin(t) + 2 * Math.sin(2 * t), Math.cos(t) - 2 * Math.cos(2 * t)]

const W = 3.4        // 가닥 굵기
const GAP = 1.6      // 위 가닥 양옆으로 비우는 폭
const SPAN = 8       // 위로 지나가는 구간의 길이(표본 수 ±)
const PAD = 3.4      // 32 좌표계 안쪽 여백
const N = 360        // 곡선 표본 수
const SEG = 48       // 그라데이션을 나눠 그릴 조각 수
const OFFSET = 0.17  // 색 흐름의 시작 위치

const K8S = '#326ce5'   // 쿠버네티스
const INFRA = '#6b3fd4' // 인프라
const APP = '#12b0a0'   // 애플리케이션

// 어두운 바탕에서의 최저 대비. 밝은 바탕에서는 이 세 색이 그대로 잘 읽히지만
// 어두운 바탕에서는 파랑·보라가 배경으로 가라앉는다 — HSL 로 섞은 탓에 두 색
// 사이가 양 끝보다도 어두운 남색으로 꺼져(휘도 .176 → .071 → .118) 매듭의 그쪽
// 절반이 흐려 보인다. 다크용 램프는 색상·채도를 그대로 두고 밝기만 끌어올린다.
const DARK_BG = '#0a0b0d' // --color-surface-base (dark)
const DARK_MIN_RATIO = 4.5

function sample() {
  const raw = []
  for (let i = 0; i < N; i++) raw.push(trefoil((i / N) * Math.PI * 2))
  const xs = raw.map((p) => p[0]), ys = raw.map((p) => p[1])
  const minX = Math.min(...xs), maxX = Math.max(...xs)
  const minY = Math.min(...ys), maxY = Math.max(...ys)
  const s = (32 - PAD * 2) / Math.max(maxX - minX, maxY - minY)
  const cx = (minX + maxX) / 2, cy = (minY + maxY) / 2
  return raw.map(([x, y]) => [16 + (x - cx) * s, 16 + (y - cy) * s])
}

function segInt(p1, p2, p3, p4) {
  const d = (p2[0] - p1[0]) * (p4[1] - p3[1]) - (p2[1] - p1[1]) * (p4[0] - p3[0])
  if (Math.abs(d) < 1e-9) return null
  const u = ((p3[0] - p1[0]) * (p4[1] - p3[1]) - (p3[1] - p1[1]) * (p4[0] - p3[0])) / d
  const v = ((p3[0] - p1[0]) * (p2[1] - p1[1]) - (p3[1] - p1[1]) * (p2[0] - p1[0])) / d
  if (u < 0 || u > 1 || v < 0 || v > 1) return null
  return { x: p1[0] + u * (p2[0] - p1[0]), y: p1[1] + u * (p2[1] - p1[1]) }
}

function crossings(pts) {
  const out = []
  for (let i = 0; i < N; i++)
    for (let j = i + 2; j < N; j++) {
      if (i === 0 && j === N - 1) continue
      const hit = segInt(pts[i], pts[(i + 1) % N], pts[j], pts[(j + 1) % N])
      if (hit) out.push({ ...hit, i, j })
    }
  return out
}

const d = (pts) => pts.map(([x, y], i) => `${i ? 'L' : 'M'}${f(x)} ${f(y)}`).join('')

const hex2hsl = (hex) => {
  const n = parseInt(hex.slice(1), 16)
  const r = ((n >> 16) & 255) / 255, g = ((n >> 8) & 255) / 255, b = (n & 255) / 255
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b), l = (mx + mn) / 2
  if (mx === mn) return [0, 0, l]
  const dd = mx - mn, s = l > 0.5 ? dd / (2 - mx - mn) : dd / (mx + mn)
  const h = mx === r ? (g - b) / dd + (g < b ? 6 : 0) : mx === g ? (b - r) / dd + 2 : (r - g) / dd + 4
  return [h * 60, s, l]
}

/** 색은 hsl 로 섞되 색상환은 짧은 쪽으로 돈다 — 그래야 중간이 탁해지지 않는다. */
function mix([h1, s1, l1], [h2, s2, l2], t) {
  const dh = ((h2 - h1 + 540) % 360) - 180
  return [h1 + dh * t, s1 + (s2 - s1) * t, l1 + (l2 - l1) * t]
}

const hsl2hex = ([h, s, l]) => {
  const a = s * Math.min(l, 1 - l)
  const ch = (n) => {
    const k = (n + h / 30) % 12
    const v = l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))
    return Math.round(v * 255).toString(16).padStart(2, '0')
  }
  return `#${ch(0)}${ch(8)}${ch(4)}`
}

const STOPS = [K8S, INFRA, APP].map(hex2hsl)
function ramp(t) {
  const x = ((((t % 1) + 1) % 1) * STOPS.length)
  const i = Math.floor(x)
  return hsl2hex(mix(STOPS[i % STOPS.length], STOPS[(i + 1) % STOPS.length], x - i))
}

/**
 * WCAG 상대 휘도. HSL 의 lightness 와 다르다 — 같은 lightness 라도 파랑은
 * 청록보다 사람 눈에 훨씬 어둡다. 밝기를 눈에 맞게 맞추려면 이 값을 봐야 한다.
 */
const luminance = (hex) => {
  const n = parseInt(hex.slice(1), 16)
  const ch = (v) => (v / 255 <= 0.04045 ? v / 255 / 12.92 : Math.pow((v / 255 + 0.055) / 1.055, 2.4))
  return 0.2126 * ch((n >> 16) & 255) + 0.7152 * ch((n >> 8) & 255) + 0.0722 * ch(n & 255)
}

/**
 * 목표 휘도에 못 미치는 색만 HSL lightness 를 올려 끌어올린다.
 * 색상·채도는 손대지 않는다 — 그 둘이 정체성이고, 밝기만이 바탕에 따라 달라진다.
 * lightness 와 휘도의 관계가 색상마다 달라 식으로 풀 수 없으므로 이분법으로 찾는다.
 */
function lift(hex, target) {
  if (luminance(hex) >= target) return hex
  const [h, s, l] = hex2hsl(hex)
  let lo = l, hi = 1
  for (let i = 0; i < 40; i++) {
    const m = (lo + hi) / 2
    if (luminance(hsl2hex([h, s, m])) < target) lo = m
    else hi = m
  }
  return hsl2hex([h, s, hi])
}

const DARK_TARGET = DARK_MIN_RATIO * (luminance(DARK_BG) + 0.05) - 0.05
const dark = (hex) => lift(hex, DARK_TARGET)

const PTS = sample()
const XS = crossings(PTS)
if (XS.length !== 3) throw new Error(`교차가 3 이 아니다: ${XS.length}`)

/** 교차 c 를 가운데 두고 ±SPAN 만큼 잘라낸 열린 구간 — 위로 지나가는 가닥. */
const arc = (c) => {
  const seg = []
  for (let k = -SPAN; k <= SPAN; k++) seg.push(PTS[(c + k + N * 2) % N])
  return d(seg)
}

// 밑에 깔리는 가닥: 색이 흐르도록 SEG 조각으로 나눈다. 마지막 조각은 첫 점으로 닫는다.
const segments = []
for (let s = 0; s < SEG; s++) {
  const from = Math.floor((s * N) / SEG), to = Math.floor(((s + 1) * N) / SEG)
  const part = PTS.slice(from, to + 1)
  if (to >= N) part.push(PTS[0])
  const fill = ramp(s / SEG + OFFSET)
  segments.push({ d: d(part), fill, dark: dark(fill), cls: `s${s}` })
}
const overs = XS.map((c, i) => {
  const fill = ramp(c.j / N + OFFSET)
  return { d: arc(c.j), fill, dark: dark(fill), cls: `o${i}` }
})
const whole = d(PTS.concat([PTS[0]]))

/**
 * 파비콘·문서용 정적 SVG. id 충돌이 없도록 접두사를 받는다.
 *
 * 파비콘은 React 밖의 독립 문서라 앱의 테마 스토어가 닿지 않는다. 대신 브라우저
 * 크롬은 OS 설정을 따르므로 `prefers-color-scheme` 이 여기서는 옳은 신호다.
 * CSS 는 presentation attribute 를 이기므로 기본값은 속성으로 두고 다크만 덮는다.
 */
function svg(prefix) {
  const cuts = overs
    .map((o) => `<path d="${o.d}" stroke="#000" stroke-width="${W + GAP * 2}" stroke-linecap="butt"/>`)
    .join('')
  const paint = (a) => a.map((s) => `<path class="${s.cls}" d="${s.d}" stroke="${s.fill}"/>`).join('')
  const darkRules = [...segments, ...overs].map((s) => `.${s.cls}{stroke:${s.dark}}`).join('')
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" role="img" aria-label="Nullus">
<title>Nullus</title>
<style>@media (prefers-color-scheme:dark){${darkRules}}</style>
<mask id="${prefix}-cut"><rect width="32" height="32" fill="#fff"/><g fill="none">${cuts}</g></mask>
<g fill="none" stroke-width="${W}" stroke-linecap="round">
<g mask="url(#${prefix}-cut)">${paint(segments)}</g>
${paint(overs)}
</g>
</svg>
`
}

/** 앱이 쓰는 도형 데이터. 색까지 구워 두어 런타임에 계산할 것이 없다. */
function ts() {
  const list = (arr) =>
    arr.map((s) => `  ['${s.d}', '${s.fill}', '${s.dark}'],`).join('\n')
  return `// 자동 생성 — 손으로 고치지 않는다.
// 생성기: docs/40_UI_UX/logo/emit.mjs (실행 방법은 같은 폴더 README 참고)
//
// 세잎 매듭(trefoil). 매개변수식 x = sin t + 2 sin 2t, y = cos t - 2 cos 2t 에서
// 점을 뽑고 자기교차 3 곳을 수치로 찾아, 아래로 지나가는 가닥을 위 가닥과
// 나란한 폭으로 비웠다. 색은 쿠버네티스 파랑 -> 인프라 보라 -> 애플리케이션
// 청록을 매듭을 따라 흐르게 한 것이다.
//
// 색은 바탕별로 두 벌이다. 밝은 바탕에서는 세 색이 그대로 읽히지만 어두운
// 바탕에서는 파랑·보라가 가라앉는다 — 두 색 사이가 양 끝보다 어두운 남색으로
// 꺼지기 때문이다. 다크 벌은 색상·채도를 그대로 두고 밝기만 ${DARK_MIN_RATIO}:1 까지 올린 것이다.

/** 가닥 굵기 (viewBox 32 기준) */
export const MARK_STROKE = ${W}
/** 위 가닥 양옆으로 비우는 폭 */
export const MARK_GAP = ${GAP}

/** 다크 벌이 보장하는 최저 대비 — 기준 바탕은 --color-surface-base (${DARK_BG}) */
export const MARK_DARK_MIN_RATIO = ${DARK_MIN_RATIO}

/** 매듭 전체를 한 붓으로 그린 경로 — 단색으로 쓸 때. */
export const MARK_WHOLE = '${whole}'

/** 아래로 깔리는 가닥. 색이 흐르도록 나눈 조각들 — [경로, 밝은 바탕 색, 어두운 바탕 색] */
export const MARK_SEGMENTS: ReadonlyArray<readonly [string, string, string]> = [
${list(segments)}
]

/** 위로 지나가는 가닥. 같은 경로를 마스크에서 굵게 그어 간격을 낸다 — [경로, 밝은 바탕 색, 어두운 바탕 색] */
export const MARK_OVERS: ReadonlyArray<readonly [string, string, string]> = [
${list(overs)}
]
`
}

writeFileSync(process.argv[2], svg('nullus'))
writeFileSync(process.argv[3], ts())
console.log(
  '교차', XS.length, '· 조각', segments.length,
  '· svg', svg('nullus').length, 'B · ts', ts().length, 'B'
)

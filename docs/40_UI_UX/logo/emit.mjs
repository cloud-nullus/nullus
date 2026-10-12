/**
 * 최종 마크(세잎 매듭)의 도형을 계산해 정적 산출물로 굽는다.
 *
 * 매듭은 손으로 그리면 반드시 틀리므로 매개변수식에서 점을 만들고
 * 자기교차점을 수치로 찾는다. 결과는 좌표가 박힌 SVG 이므로
 * 앱은 이 스크립트를 실행하지 않는다 — 도형을 고칠 때만 다시 돌린다.
 */
import { mkdirSync, writeFileSync } from 'node:fs'

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

// 배포용 가로형 로고의 글자색 — 사이드바 워드마크와 같은 --color-text-primary.
const INK = { light: '#0a0b0d', dark: '#ffffff' }

// 가로형 로고의 비율은 사이드바(`sidebar.tsx`)를 그대로 옮긴다 —
// 마크 20px · 간격 gap-2(8px) · 글자 16px bold. 32 좌표계로 환산한 값이다.
const LOCKUP_FONT = (32 * 16) / 20
const LOCKUP_GAP = (32 * 8) / 20

/**
 * "Nullus" 워드마크의 윤곽선. 앱이 Google Fonts 에서 받는 것과 같은 Inter 4.001 Bold
 * 에서 fontTools 로 뽑았다. 이 여섯 글자 사이에는 커닝 쌍이 없어(hb-shape 로 확인)
 * advance 만으로 늘어놓았다. 좌표는 폰트 단위(upm 2048, y 가 위쪽) 그대로 두고
 * SVG transform 으로 옮긴다 — 폰트가 없는 곳에서도 같은 모양이어야 하므로
 * 글자가 아니라 도형으로 싣는다.
 */
const WORDMARK = {
  d: 'M135 0V1490H475L946 736Q982 678 1019 610Q1056 542 1094.5 459Q1133 376 1171 272H1140Q1133 355 1127.5 449Q1122 543 1118 629Q1114 715 1114 775V1490H1426V0H1085L657 684Q610 761 570 833.5Q530 906 486.5 993.5Q443 1081 383 1203H422Q428 1095 434 996Q440 897 443.5 816.5Q447 736 447 685V0ZM2074 -14Q1958 -14 1871 36.5Q1784 87 1736.5 181.5Q1689 276 1689 407V1118H1989V459Q1989 355 2043 296Q2097 237 2191 237Q2255 237 2304 264.5Q2353 292 2380.5 344.5Q2408 397 2408 471V1118H2709V0H2425L2421 279H2438Q2394 138 2303.5 62Q2213 -14 2074 -14ZM3264 1490V0H2964V1490ZM3819 1490V0H3519V1490ZM4459 -14Q4343 -14 4256 36.5Q4169 87 4121.5 181.5Q4074 276 4074 407V1118H4374V459Q4374 355 4428 296Q4482 237 4576 237Q4640 237 4689 264.5Q4738 292 4765.5 344.5Q4793 397 4793 471V1118H5094V0H4810L4806 279H4823Q4779 138 4688.5 62Q4598 -14 4459 -14ZM5793 -22Q5658 -22 5554 16.5Q5450 55 5384 128.5Q5318 202 5299 306L5578 354Q5600 276 5655 237Q5710 198 5802 198Q5888 198 5937.5 230.5Q5987 263 5987 313Q5987 357 5951.5 385Q5916 413 5843 428L5650 468Q5488 501 5408 580.5Q5328 660 5328 785Q5328 893 5387 970.5Q5446 1048 5551 1090Q5656 1132 5798 1132Q5930 1132 6027 1095.5Q6124 1059 6184 992Q6244 925 6264 834L5998 787Q5981 844 5932.5 880.5Q5884 917 5802 917Q5728 917 5678 886Q5628 855 5628 803Q5628 761 5660.5 732Q5693 703 5772 687L5973 647Q6135 614 6214 539.5Q6293 465 6293 345Q6293 235 6229 152.5Q6165 70 6052.5 24Q5940 -22 5793 -22Z',
  upm: 2048,
  cap: 1490, // 대문자 높이 — 이 높이의 가운데를 마크 중심에 맞춘다
  xMax: 6293,
  yMin: -22, // s 의 오버슈트
}

const EXPORT_LONG = 2048 // 배포용 PNG 의 긴 변(px)
const EXPORT_MARGIN = 0.5 // 배포용 SVG 가장자리 여백(32 좌표계) — 안티앨리어싱이 잘리지 않을 만큼만

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
 * 파비콘용 정적 SVG. id 충돌이 없도록 접두사를 받는다.
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

/**
 * 문서·발표·외부 제출용 정적 SVG — 마크 단독 또는 워드마크를 붙인 가로형.
 *
 * 파비콘과 달리 `prefers-color-scheme` 을 싣지 않는다. 이 파일은 남의 문서나
 * 슬라이드에 이미지로 박히고, 그 바탕색은 보는 사람의 OS 설정과 무관하다 —
 * 바탕은 쓰는 쪽이 알므로 밝은 바탕용·어두운 바탕용을 따로 굽는다.
 *
 * 배포본은 수천 px 로 커지므로 앱용 도형을 그대로 쓰지 않고 모양과 색을 나눈다.
 * 앱용 48조각을 그대로 키우면 조각마다 둥근 끝이 반원 비늘로 비치고, 색 계단과
 * 360점 꺾은선의 각이 드러난다.
 *
 * - 모양: 같은 표본을 지나는 매끈한 곡선(Catmull-Rom → 3차 베지어) 한 줄을 마스크로
 *   쓴다. 가장자리 안티앨리어싱이 이 한 줄에서만 일어나 어느 배율에서도 고르다.
 * - 색: 표본마다 색을 매긴 조각을 그 마스크 안에서만 칠한다. 조각은 가닥보다 조금
 *   굵고 한 표본씩 겹쳐서, 조각의 끝과 이음새는 모두 마스크 안쪽에 묻힌다.
 *   이웃 표본 사이 색 차이는 8비트 한두 칸이라 경계가 보이지 않는다.
 *
 * 표본·교차·끊는 폭은 파비콘과 같다 — 테스트가 곡선의 앵커를 MARK_WHOLE 과 대조한다.
 */
function brand(bg, { wordmark }) {
  const tone = bg === 'dark' ? dark : (hex) => hex
  const at = (k) => PTS[((k % N) + N) % N]
  // 베지어 제어점은 앵커 곁 0.1 단위 안에 붙는다. 소수 둘째 자리로 자르면 이웃 곡선과
  // 접선이 수 도씩 어긋나 큰 배율에서 꺾임이 보이므로 셋째 자리까지 남긴다.
  const g = (n) => String(Math.round(n * 1000) / 1000)
  const pt = ([x, y]) => `${g(x)} ${g(y)}`

  /** 표본 from→to 를 지나는 매끈한 곡선. 구간의 양 끝도 닫힌 곡선의 이웃 표본으로 접선을 잡는다. */
  const smooth = (from, to) => {
    let s = `M${pt(at(from))}`
    for (let k = from; k < to; k++) {
      const [p0, p1, p2, p3] = [at(k - 1), at(k), at(k + 1), at(k + 2)]
      const c1 = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6]
      const c2 = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6]
      s += `C${pt(c1)} ${pt(c2)} ${pt(p2)}`
    }
    return s
  }

  /** 표본 from→to 의 색 조각. 색이 바뀌는 표본에서만 끊고, 끊을 때마다 다음 조각 밑으로 한 표본 겹친다. */
  const paint = (from, to) => {
    const out = []
    let start = from, color = tone(ramp(from / N + OFFSET))
    const flush = (end) => {
      const run = []
      for (let k = start; k <= end + 1; k++) run.push(at(k))
      out.push(`<path d="${d(run)}" stroke="${color}"/>`)
    }
    for (let k = from + 1; k < to; k++) {
      const c = tone(ramp(k / N + OFFSET))
      if (c !== color) {
        flush(k)
        start = k
        color = c
      }
    }
    flush(to)
    return out.join('')
  }

  // 위 가닥. 모양(흰 획)과 끊는 자리(검은 획)는 같은 호다. 앱보다 두 표본 길다 —
  // 곡선으로 바꾸면 끊는 띠의 끝선이 1~2° 돌아가는데, 아래 가닥 가장자리가 그 끝
  // 모서리를 아슬아슬하게 지나는 교차가 있어 가는 가시가 삐져나온다. 띠를 늘리면
  // 간격의 모서리가 앱과 같이 띠의 긴 변에서 생긴다.
  const reach = SPAN + 2
  const arcs = XS.map((c) => `<path d="${smooth(c.j - reach, c.j + reach)}"/>`).join('')
  // 색은 모양보다 두 표본 더 칠해, 색 조각의 끝이 같은 색 밑칠 위에 놓이게 한다.
  const overPaint = XS.map((c) => paint(c.j - reach - 2, c.j + reach + 2)).join('')
  const xs = PTS.map((p) => p[0]), ys = PTS.map((p) => p[1])
  let x0 = Math.min(...xs) - W / 2, x1 = Math.max(...xs) + W / 2
  const y0 = Math.min(...ys) - W / 2
  let y1 = Math.max(...ys) + W / 2

  let text = ''
  if (wordmark) {
    const s = LOCKUP_FONT / WORDMARK.upm
    // 대문자 높이의 가운데를 마크 중심(16)에 맞춘다 — 사이드바의 items-center 와 같은 자리다.
    const tx = 32 + LOCKUP_GAP, ty = 16 + (WORDMARK.cap * s) / 2
    x1 = Math.max(x1, tx + WORDMARK.xMax * s)
    y1 = Math.max(y1, ty - WORDMARK.yMin * s)
    text = `\n<path transform="translate(${tx} ${ty}) scale(${s} ${-s})" fill="${INK[bg]}" d="${WORDMARK.d}"/>`
  }

  x0 -= EXPORT_MARGIN; x1 += EXPORT_MARGIN
  const vw = x1 - x0, vh = y1 - y0 + EXPORT_MARGIN * 2
  const [pw, ph] = vw >= vh
    ? [EXPORT_LONG, Math.round((EXPORT_LONG * vh) / vw)]
    : [Math.round((EXPORT_LONG * vw) / vh), EXPORT_LONG]
  const box = [x0, y0 - EXPORT_MARGIN, vw, vh].map(f).join(' ')

  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${box}" width="${pw}" height="${ph}" role="img" aria-label="Nullus">
<title>Nullus</title>
<mask id="nullus-cut"><rect width="32" height="32" fill="#fff"/><g fill="none" stroke="#000" stroke-width="${W + GAP * 2}" stroke-linecap="butt">${arcs}</g></mask>
<mask id="nullus-shape"><g fill="none" stroke="#fff" stroke-width="${W}" stroke-linecap="round"><path d="${smooth(0, N)}Z" mask="url(#nullus-cut)"/>${arcs}</g></mask>
<g mask="url(#nullus-shape)" fill="none" stroke-width="${W + 0.5}" stroke-linecap="butt">
${paint(0, N)}
${overPaint}
</g>${text}
</svg>
`
}

/** 배포용 파일 이름 → SVG. PNG 는 rasterize.mjs 가 같은 이름으로 굽는다. */
const EXPORTS = {
  'nullus-logo.svg': brand('light', { wordmark: true }),
  'nullus-logo-on-dark.svg': brand('dark', { wordmark: true }),
  'nullus-mark.svg': brand('light', { wordmark: false }),
  'nullus-mark-on-dark.svg': brand('dark', { wordmark: false }),
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
// 배포용은 인자로 받지 않는다 — 놓일 자리가 이 폴더 하나뿐이다.
const exportDir = new URL('./export/', import.meta.url)
mkdirSync(exportDir, { recursive: true })
for (const [name, body] of Object.entries(EXPORTS)) writeFileSync(new URL(name, exportDir), body)
console.log(
  '교차', XS.length, '· 조각', segments.length,
  '· svg', svg('nullus').length, 'B · ts', ts().length, 'B',
  '· export', Object.keys(EXPORTS).length, '개'
)

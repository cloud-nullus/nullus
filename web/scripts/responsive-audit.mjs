#!/usr/bin/env node
/**
 * 모바일/반응형 자동 점검 스크립트 (Mobile / Responsive audit)
 *
 * 주요 화면을 여러 모바일/태블릿 뷰포트로 렌더해 반응형 깨짐을 자동 감지하고,
 * 화면별 스크린샷과 마크다운/JSON 리포트를 생성한다.
 *
 * 감지 신호 2종:
 *   1) 가로 오버플로우  — document.scrollWidth > clientWidth (가로 스크롤 발생)
 *   2) 사이드바 미collapse — 모바일 폭(<768px)에서 사이드바(<aside>)가 화면의 30% 초과 점유
 *      → 본문이 좁은 칸에 짓눌리는데 문서 폭은 안 넘어 (1)로는 안 잡히는 대표 유형.
 *
 * 사전 조건: 웹 dev 서버 + API 가 떠 있어야 한다 (mock auth 로 로그인).
 *
 * 사용:
 *   node scripts/responsive-audit.mjs                 # 리포트만 생성
 *   node scripts/responsive-audit.mjs --check         # 이슈 발견 시 exit 1 (CI 게이트용)
 *   RESPONSIVE_BASE_URL=http://localhost:5174 node scripts/responsive-audit.mjs
 *
 * 환경변수:
 *   RESPONSIVE_BASE_URL   기본 http://localhost:5173
 *   RESPONSIVE_EMAIL      기본 admin@nullus.dev
 *   RESPONSIVE_PASSWORD   기본 admin123
 *   RESPONSIVE_OUT        기본 <web>/.responsive-audit
 */
import { chromium } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const WEB_ROOT = join(__dirname, '..')

const BASE = process.env.RESPONSIVE_BASE_URL || 'http://localhost:5173'
const EMAIL = process.env.RESPONSIVE_EMAIL || 'admin@nullus.dev'
const PASSWORD = process.env.RESPONSIVE_PASSWORD || 'admin123'
const OUT = process.env.RESPONSIVE_OUT || join(WEB_ROOT, '.responsive-audit')
const CHECK = process.argv.includes('--check')
const OVERFLOW_TOLERANCE = 2 // px. 서브픽셀 반올림 노이즈 무시
const MOBILE_MAX = 768 // 이 폭 미만을 모바일로 본다 (사이드바 점검 대상)
const SIDEBAR_MAX_RATIO = 0.3 // 모바일에서 사이드바가 뷰포트의 이 비율 초과면 미collapse로 판정

// iPhone/Android/iPad 대표 폭. 가장 작은 폭에서 가장 잘 깨진다.
const VIEWPORTS = [
  { key: 'mobile-sm', width: 360, height: 640 }, // 소형 안드로이드
  { key: 'mobile', width: 390, height: 844 },    // iPhone 12~15
  { key: 'tablet', width: 768, height: 1024 },   // iPad 세로
]

// priority: EPIC #36 이 명시한 우선 점검 3종. 나머지는 커버리지 확장.
const ROUTES = [
  { key: 'login', path: '/login', auth: false, priority: false },
  { key: 'home', path: '/', auth: true, priority: true },
  { key: 'stack-install', path: '/stack/install', auth: true, priority: true },
  { key: 'cicd-list', path: '/cicd/list', auth: true, priority: true },
  { key: 'stack-templates', path: '/stack/templates', auth: true, priority: false },
  { key: 'stack-list', path: '/stack/list', auth: true, priority: false },
  { key: 'cicd-templates', path: '/cicd/templates', auth: true, priority: false },
  { key: 'monitoring', path: '/observability/monitoring', auth: true, priority: false },
  { key: 'admin-org', path: '/admin/organization', auth: true, priority: false },
]

// 뷰포트를 넘어서는(오른쪽으로 삐져나온) 보이는 요소를 찾는다.
function findOverflowOffenders(tol) {
  const vw = window.innerWidth
  const out = []
  const seen = new Set()
  for (const el of Array.from(document.querySelectorAll('*'))) {
    const r = el.getBoundingClientRect()
    if (r.width === 0 || r.height === 0) continue
    if (r.right <= vw + tol) continue
    const style = window.getComputedStyle(el)
    if (style.visibility === 'hidden' || style.display === 'none') continue
    const cls = typeof el.className === 'string' ? el.className.trim().split(/\s+/).slice(0, 3).join('.') : ''
    const sel = el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (cls ? '.' + cls : '')
    if (seen.has(sel)) continue
    seen.add(sel)
    out.push({
      selector: sel,
      right: Math.round(r.right),
      overflowPx: Math.round(r.right - vw),
      text: (el.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 60),
    })
    if (out.length >= 12) break
  }
  return out.sort((a, b) => b.overflowPx - a.overflowPx)
}

async function login(page) {
  await page.goto(BASE + '/login', { waitUntil: 'networkidle', timeout: 30000 })
  await page.locator('input[type="email"]').fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: /sign ?in|로그인/i }).click()
  await page.waitForFunction(() => !location.pathname.startsWith('/login'), { timeout: 15000 }).catch(() => {})
}

async function main() {
  await mkdir(OUT, { recursive: true })
  const browser = await chromium.launch()
  const results = []

  for (const vp of VIEWPORTS) {
    const context = await browser.newContext({ viewport: { width: vp.width, height: vp.height } })
    const page = await context.newPage()
    await login(page)

    for (const route of ROUTES) {
      try {
        await page.goto(BASE + route.path, { waitUntil: 'networkidle', timeout: 30000 })
        await page.waitForTimeout(1200)
        const metrics = await page.evaluate(() => {
          const doc = document.documentElement
          const aside = document.querySelector('aside')
          return {
            scrollWidth: doc.scrollWidth,
            clientWidth: doc.clientWidth,
            overflowPx: Math.max(0, doc.scrollWidth - doc.clientWidth),
            asideWidth: aside ? Math.round(aside.getBoundingClientRect().width) : 0,
          }
        })
        const hasOverflow = metrics.overflowPx > OVERFLOW_TOLERANCE
        // 모바일 폭에서 사이드바가 접히지 않아 본문을 짓누르는 경우 (가로 오버플로우로는 안 잡힘)
        const sidebarSquish = vp.width < MOBILE_MAX && metrics.asideWidth > vp.width * SIDEBAR_MAX_RATIO
        const hasIssue = hasOverflow || sidebarSquish
        const offenders = hasOverflow ? await page.evaluate(findOverflowOffenders, OVERFLOW_TOLERANCE) : []
        const shot = join(OUT, `${route.key}__${vp.key}.png`)
        await page.screenshot({ path: shot, fullPage: true })
        results.push({ route: route.key, path: route.path, priority: route.priority, viewport: vp.key, vw: vp.width, ...metrics, hasOverflow, sidebarSquish, hasIssue, offenders, screenshot: shot })
        process.stdout.write(`${hasIssue ? '✗' : '✓'} ${route.key.padEnd(16)} @${vp.key.padEnd(9)} overflow=${String(metrics.overflowPx).padStart(3)}px aside=${String(metrics.asideWidth).padStart(3)}px${sidebarSquish ? '  [사이드바 미collapse]' : ''}\n`)
      } catch (err) {
        results.push({ route: route.key, path: route.path, priority: route.priority, viewport: vp.key, error: String((err && err.message) || err) })
        process.stdout.write(`! ${route.key.padEnd(16)} @${vp.key.padEnd(9)} ERROR ${err && err.message}\n`)
      }
    }
    await context.close()
  }
  await browser.close()

  const now = new Date()
  const stamp = now.toISOString().slice(0, 10)
  const broken = results.filter((r) => r.hasIssue)
  const summary = {
    generatedAt: now.toISOString(),
    base: BASE,
    viewports: VIEWPORTS.map((v) => `${v.key}(${v.width}px)`),
    routes: ROUTES.length,
    totalChecks: results.length,
    issueChecks: broken.length,
    results,
  }
  await writeFile(join(OUT, `report-${stamp}.json`), JSON.stringify(summary, null, 2))

  // 마크다운 요약
  const lines = []
  lines.push(`# 모바일/반응형 자동 점검 리포트 (${stamp})`)
  lines.push('')
  lines.push(`- base: \`${BASE}\``)
  lines.push(`- 뷰포트: ${summary.viewports.join(', ')}`)
  lines.push(`- 총 점검: ${summary.totalChecks}건 · 이슈: **${summary.issueChecks}건**`)
  lines.push('')
  lines.push('| 화면 | 뷰포트 | 가로 오버플로우 | 사이드바 폭 | 상태 |')
  lines.push('|---|---|---|---|---|')
  for (const r of results) {
    let status = '✅ ok'
    if (r.error) status = '⚠️ error'
    else if (r.hasOverflow && r.sidebarSquish) status = '❌ 오버플로우+사이드바'
    else if (r.hasOverflow) status = `❌ 오버플로우 ${r.overflowPx}px`
    else if (r.sidebarSquish) status = '❌ 사이드바 미collapse'
    lines.push(`| ${r.route}${r.priority ? ' ⭐' : ''} | ${r.viewport} | ${r.error ? '-' : (r.overflowPx || 0) + 'px'} | ${r.error ? '-' : (r.asideWidth || 0) + 'px'} | ${status} |`)
  }
  const overflowCases = broken.filter((r) => r.hasOverflow)
  if (overflowCases.length) {
    lines.push('')
    lines.push('## 가로 오버플로우 유발 요소 (top offenders)')
    for (const r of overflowCases) {
      lines.push('')
      lines.push(`### ${r.route} @ ${r.viewport} (+${r.overflowPx}px)`)
      for (const o of r.offenders.slice(0, 5)) {
        lines.push(`- \`${o.selector}\` — +${o.overflowPx}px${o.text ? ` — "${o.text}"` : ''}`)
      }
    }
  }
  await writeFile(join(OUT, `report-${stamp}.md`), lines.join('\n') + '\n')

  process.stdout.write(`\n리포트: ${join(OUT, `report-${stamp}.md`)}\n`)
  process.stdout.write(`스크린샷: ${OUT}/<route>__<viewport>.png\n`)
  process.stdout.write(`이슈 ${broken.length}건 / 총 ${results.length}건\n`)

  if (CHECK && broken.length > 0) process.exit(1)
}

main().catch((e) => { console.error(e); process.exit(2) })

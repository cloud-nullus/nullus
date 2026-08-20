import { test, expect, type Page } from '@playwright/test'
import { loginAs } from './helpers/auth'

/**
 * 둘러보기 시나리오 기록지(docs/60_테스트/Nullus_둘러보기_기능확인_시나리오.md)의
 * 읽기 전용 구간을 단정(assert) 스펙으로 전환한 것 — nullus-plan#53 세부 태스크.
 * tour-walkthrough.spec.ts 가 눈검증용 스크린샷 도구라면 이 파일은 회귀 게이트다.
 *
 * 상태를 바꾸는 구간은 여기 없다. S2(클러스터 등록)·S5(스택 배포)·S7(파이프라인
 * 생성)은 실제 등록·배포를 수반해 회귀로 돌리기엔 파괴적이다 — 진입 다이얼로그가
 * 열리는 데까지만 본다. 전체 흐름 검증은 기록지의 수동 시나리오가 정본이다.
 *
 * 환경 전제(수행 가이드 §2)가 부족하면 스펙이 화면 상태를 보고 스스로 건너뛴다.
 * 어느 등급(T0/T1/T2)이 갖춰졌는지는 scripts/e2e-preflight.sh 로 먼저 본다.
 * 포트 회피 환경: E2E_BASE_URL=http://localhost:5174 npx playwright test e2e/tour-regression.spec.ts
 */

const LITE_TEMPLATE = '[data-tour-template="gitea-jenkins-argocd-lite-v1"]'
const DIALOG = '[data-modal]'
const TOUR_CARD = '[aria-label="Product tour"]'
const TOUR_NEXT = '[data-testid="tour-next"]'
const WIZARD_TABS = ['authentication', 'artifacts', 'pipeline', 'monitoring', 'storage', 'resources', 'dry-run']

async function startTour(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: /Tutorial|튜토리얼/ }).click()
  await expect(page.locator(TOUR_CARD)).toBeVisible({ timeout: 5000 })
}

test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await loginAs(page, 'admin')
})

test('S0+S1: 홈 진입 후 투어 시작 — 오버레이·진행 표시·컨트롤', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('[data-tour="hero-cta"]')).toBeVisible()
  await expect(page.locator('[data-tour="quick-start"]')).toBeVisible()

  await startTour(page)
  await expect(page.locator(TOUR_CARD)).toContainText(/1\s*\/\s*\d+/)
  await expect(page.locator('[data-testid="tour-spotlight"]')).toHaveCount(1)
  for (const control of ['tour-end', 'tour-prev', 'tour-next']) {
    await expect(page.locator(`[data-testid="${control}"]`)).toBeVisible()
  }
})

test('S2(축소): 클러스터 등록 다이얼로그가 열린다 — 등록은 하지 않는다', async ({ page }) => {
  await page.goto('/admin/clusters')
  const register = page.locator('[data-tour="register-cluster"]')
  await expect(register).toBeVisible()
  await register.click()
  await expect(page.locator(DIALOG).first()).toBeVisible()
  await page.keyboard.press('Escape')
})

test('S3: Lite 템플릿 상세 → "이 템플릿 사용" → 설치 마법사 진입', async ({ page }) => {
  await page.goto('/stack/templates')
  const lite = page.locator(LITE_TEMPLATE)
  await expect(lite).toBeVisible({ timeout: 10000 })
  await lite.locator('[data-tour="template-detail"]').click()
  await expect(page.locator(DIALOG).first()).toBeVisible()
  await page.locator('[data-tour="use-base-template"]').click()
  await page.waitForURL('**/stack/install**', { timeout: 10000 })
})

test('S4: 설치 마법사 7탭이 전부 전환된다', async ({ page }) => {
  await page.goto('/stack/install')
  for (const tab of WIZARD_TABS) {
    await page.locator(`[data-tab="${tab}"]`).click()
    await expect(page.locator('[data-tour="install-panel"]')).toBeVisible()
  }
})

test('S6: 스택 상세 — workloads 파드 집계·연결 정보 복사 버튼', async ({ page }) => {
  await page.goto('/stack/list')
  await expect(page.locator('[data-tour="stack-list"]')).toBeVisible({ timeout: 10000 })

  const rows = page.locator('[data-tour="stack-list"] tbody tr')
  if ((await rows.count()) === 0) {
    test.skip(true, '배포된 스택 없음 — T2 전제(수행 가이드 §2 kind-up + 스택 배포) 후 유효')
  }

  await rows.first().click()
  await page.locator('[data-tab="workloads"]').click()
  const panel = page.locator('[data-tour="stack-detail-panel"]')
  await expect(panel).toBeVisible()
  // 파드 준비 집계(n / m)가 렌더되면 충분하다 — 도구 없는 스택의 0/0 도 정상(기록지 S6).
  await expect(panel).toContainText(/\d+\s*\/\s*\d+/, { timeout: 10000 })

  await page.locator('[data-tab="info"]').click()
  await expect(page.locator('[data-tour="gateway-pf-copy"]')).toBeVisible()
  await expect(page.locator('[data-tour="hosts-copy"]')).toBeVisible()
})

test('S8: 관측 — 대시보드 렌더와 알림 규칙 신규 폼 열림(저장 안 함)', async ({ page }) => {
  await page.goto('/observability/monitoring')
  await expect(page.locator('[data-tour="monitoring"]')).toBeVisible({ timeout: 10000 })

  await page.goto('/observability/alerts')
  const newRule = page.locator('[data-tour="alert-rule-new"]')
  await expect(newRule).toBeVisible()
  await newRule.click()
  await expect(page.locator(`${DIALOG}, form`).first()).toBeVisible()
})

test('S9: 투어 완주 — 마지막 걸음이 Quick Start 로 돌려보낸다', async ({ page }) => {
  test.setTimeout(240_000)
  await startTour(page)

  let finished = false
  for (let i = 0; i < 40; i += 1) {
    // 걸음 전환(라우트 이동·팝업 열기) 중 컨트롤이 잠깐 사라질 수 있다.
    await expect(page.locator(TOUR_NEXT)).toBeVisible({ timeout: 15000 })
    const label = (await page.locator(TOUR_NEXT).innerText()).trim()
    if (/Finish|마치기/.test(label)) {
      await expect(page.locator(TOUR_CARD)).toContainText(/30\s*\/\s*30/)
      await page.locator(TOUR_NEXT).click({ force: true })
      finished = true
      break
    }
    await page.locator(TOUR_NEXT).click({ force: true })
    await page.waitForTimeout(1100)
  }

  expect(finished, '40회 안에 Finish 에 도달하지 못했다').toBe(true)
  await expect(page.locator(TOUR_CARD)).toHaveCount(0)
  await expect(page).toHaveURL(/\/$/)
  await expect(page.locator('[data-tour="quick-start"]')).toBeVisible()
})

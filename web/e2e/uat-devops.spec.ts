import { test, expect, type Page } from '@playwright/test'

async function loginAsDevOps(page: Page) {
  await page.goto('/login')
  await page.fill('#email', 'devops@nullus.dev')
  await page.fill('#password', 'devops123')
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
  await page.click('button[type="submit"]')
  await page.waitForURL('**/stack/templates')
}

test.describe('UAT: DevOps Engineer 미정', () => {
  test('로그인 페이지 접속 (/login)', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('h1')).toContainText('Nullus Platform')
    await expect(page.locator('#email')).toBeVisible()
  })

  test('devops@nullus.dev 이메일 입력 + Sign In → Stack Templates로 리다이렉트', async ({ page }) => {
    await loginAsDevOps(page)
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })
  })

  test('"Get Started" CTA 버튼 표시 확인', async ({ page }) => {
    await loginAsDevOps(page)
    await page.goto('/')
    await expect(page.locator('button').filter({ hasText: /Start Stack|Get Started|시작하기/ })).toBeVisible()
  })

  test('사이드바에 DevSecOps Stack, CI/CD, Observability 메뉴 표시', async ({ page }) => {
    await loginAsDevOps(page)

    // Group headers are buttons in the sidebar
    await expect(page.locator('aside button').filter({ hasText: 'DevSecOps Stack' }).first()).toBeVisible()
    await expect(page.locator('aside button').filter({ hasText: 'CI/CD' }).first()).toBeVisible()
    await expect(page.locator('aside button').filter({ hasText: 'Observability' }).first()).toBeVisible()
  })

  test('Stack Templates 이동 → 4개 템플릿 카드 확인', async ({ page }) => {
    await loginAsDevOps(page)
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })

    const cards = page.locator('main [class*="card"]').filter({ hasText: /Use Base Template/ })
    await expect(cards).toHaveCount(4)
  })

  test('Stack Install 이동 → 5개 탭 표시 확인', async ({ page }) => {
    await loginAsDevOps(page)

    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })

    for (const tab of ['Artifacts', 'CI/CD', 'Observability', 'Resources', 'YAML View']) {
      const tabBtn = page.locator('main').getByRole('tab', { name: tab }).or(
        page.locator('main button').filter({ hasText: new RegExp(`^${tab}$`) })
      ).first()
      await expect(tabBtn).toBeVisible()
    }
  })

  test('Monitoring Dashboard 이동 → KPI 카드 표시 확인', async ({ page }) => {
    await loginAsDevOps(page)

    await page.goto('/observability/monitoring')
    await expect(page.locator('h1')).toContainText('Monitoring', { timeout: 10000 })
  })
})

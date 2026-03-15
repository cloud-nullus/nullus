import { test, expect, type Page } from '@playwright/test'

async function loginAs(page: Page, email: string, password: string, redirectPath: string) {
  await page.goto('/login')
  await page.fill('#email', email)
  await page.fill('#password', password)
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
  await page.click('button[type="submit"]')
  await page.waitForURL(`**${redirectPath}`)
}

test.describe('Navigation E2E', () => {
  test('홈 페이지 로딩 확인 - Nullus 텍스트', async ({ page }) => {
    await loginAs(page, 'developer@nullus.dev', 'developer123', '/cicd/developer-deploy')
    await page.goto('/')
    // Sidebar shows "Nullus" brand text
    await expect(page.locator('aside').getByText('Nullus')).toBeVisible()
  })

  test('Stack Templates 페이지 이동 (/stack/templates)', async ({ page }) => {
    await loginAs(page, 'devops@nullus.dev', 'devops123', '/stack/templates')
    await page.goto('/stack/templates')
    await expect(page.locator('h1')).toContainText('Golden Path Templates', { timeout: 10000 })
  })

  test('Stack Install 페이지 이동 (/stack/install)', async ({ page }) => {
    await loginAs(page, 'devops@nullus.dev', 'devops123', '/stack/templates')
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
  })

  test('Stack List 페이지 이동 (/stack/list)', async ({ page }) => {
    await loginAs(page, 'devops@nullus.dev', 'devops123', '/stack/templates')
    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText('Stack List', { timeout: 10000 })
  })

  test('CI/CD Templates 페이지 이동 (/cicd/templates)', async ({ page }) => {
    await loginAs(page, 'devops@nullus.dev', 'devops123', '/stack/templates')
    await page.goto('/cicd/templates')
    await expect(page.locator('h1')).toBeVisible({ timeout: 10000 })
  })

  test('Monitoring 페이지 이동 (/observability/monitoring)', async ({ page }) => {
    await loginAs(page, 'developer@nullus.dev', 'developer123', '/cicd/developer-deploy')
    await page.goto('/observability/monitoring')
    await expect(page.locator('h1')).toContainText('Monitoring', { timeout: 10000 })
  })

  test('Organization 페이지 이동 (/admin/organization)', async ({ page }) => {
    await loginAs(page, 'admin@nullus.dev', 'admin123', '/admin/organization')
    await expect(page).toHaveURL(/\/admin\/organization|\/login/)
  })

  test('Clusters 페이지 이동 (/admin/clusters)', async ({ page }) => {
    await loginAs(page, 'admin@nullus.dev', 'admin123', '/admin/organization')
    await page.goto('/admin/clusters')
    await expect(page).toHaveURL(/\/admin\/clusters|\/login/)
  })

  test('404 페이지 검증 (/nonexistent → "Page not found")', async ({ page }) => {
    await page.goto('/nonexistent')
    await expect(page.locator('h1')).toContainText('Page not found')
  })
})

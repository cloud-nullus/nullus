import { test, expect } from '@playwright/test'

test.describe('UAT: Developer 지은', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'developer@nullus.dev')
    await page.fill('#password', 'developer123')
    await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
    await page.click('button[type="submit"]')
    await page.waitForURL('**/cicd/developer-deploy')
  })

  test('로그인 성공 - developer role로 리다이렉트 확인', async ({ page }) => {
    expect(page.url()).toContain('/cicd/developer-deploy')
  })

  test('사이드바에서 DevSecOps Stack 메뉴 숨김 확인', async ({ page }) => {
    // Developer role: devsecops group has roles ['admin','devops'] only
    await expect(
      page.locator('aside button').filter({ hasText: 'DevSecOps Stack' })
    ).toHaveCount(0)
  })

  test('사이드바에서 Admin 메뉴 숨김 확인', async ({ page }) => {
    // Admin group only for 'admin' role
    await expect(
      page.locator('aside button').filter({ hasText: 'Admin' })
    ).toHaveCount(0)
  })

  test('CI/CD 그룹 표시 확인 - developer에게 접근 가능', async ({ page }) => {
    // CI/CD group is visible to developer role
    await expect(page.locator('aside button').filter({ hasText: 'CI/CD' }).first()).toBeVisible()
  })

  test('Monitoring Dashboard 접근 가능 확인', async ({ page }) => {
    await page.goto('/observability/monitoring')
    await expect(page.getByText('Loading dashboard...')).toBeVisible({ timeout: 10000 })
  })
})

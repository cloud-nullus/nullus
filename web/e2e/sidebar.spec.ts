import { test, expect } from '@playwright/test'

test.describe('Sidebar E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops123')
    await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
    await page.click('button[type="submit"]')
    await page.waitForURL('**/stack/templates')
    await page.evaluate(() => localStorage.removeItem('nullus-sidebar-collapsed'))
    await page.reload()
  })

  test('사이드바 펼친 상태에서 메뉴 텍스트 표시 확인', async ({ page }) => {
    // Sidebar brand text visible when expanded
    await expect(page.locator('aside').getByText('Nullus')).toBeVisible()
    // CI/CD group header button (not nav links) - use the button role
    await expect(page.locator('aside button').filter({ hasText: 'CI/CD' }).first()).toBeVisible()
  })

  test('사이드바 접기/펼치기 토글 동작', async ({ page }) => {
    // Sidebar starts expanded - brand text visible
    await expect(page.locator('aside').getByText('Nullus')).toBeVisible()

    // Click toggle button
    await page.click('button[aria-label="Toggle sidebar"]')

    // After collapse, "Nullus" text should be hidden
    await expect(page.locator('aside').getByText('Nullus')).not.toBeVisible()

    // Click again to expand
    await page.click('button[aria-label="Toggle sidebar"]')

    // After expand, "Nullus" text should be visible again
    await expect(page.locator('aside').getByText('Nullus')).toBeVisible()
  })

  test('접힌 상태에서 아이콘만 표시 확인', async ({ page }) => {
    await page.evaluate(() => localStorage.setItem('nullus-sidebar-collapsed', 'true'))
    await page.reload()

    // "Nullus" text not visible in collapsed state
    await expect(page.locator('aside').getByText('Nullus')).not.toBeVisible()

    // Toggle button still visible
    await expect(page.locator('button[aria-label="Toggle sidebar"]')).toBeVisible()
  })

  test('펼친 상태에서 메뉴 텍스트 표시 확인', async ({ page }) => {
    await page.evaluate(() => localStorage.setItem('nullus-sidebar-collapsed', 'false'))
    await page.reload()

    // CI/CD group header button text visible (default developer role sees CI/CD)
    await expect(page.locator('aside button').filter({ hasText: 'CI/CD' }).first()).toBeVisible()
  })
})

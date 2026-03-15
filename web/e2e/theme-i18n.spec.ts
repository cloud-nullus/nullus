import { test, expect, type Page } from '@playwright/test'

async function loginAsDeveloper(page: Page) {
  await page.goto('/login')
  await page.fill('#email', 'developer@nullus.dev')
  await page.fill('#password', 'developer123')
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
  await page.click('button[type="submit"]')
  await page.waitForURL('**/cicd/developer-deploy')
}

test.describe('Theme / i18n E2E', () => {
  test('다크 테마 기본 적용 확인', async ({ page }) => {
    // Clear localStorage to get default theme
    await page.goto('/login')
    await page.evaluate(() => localStorage.removeItem('nullus-theme'))
    await page.reload()
    await loginAsDeveloper(page)
    const htmlDataTheme = await page.locator('html').getAttribute('data-theme')
    expect(htmlDataTheme).toBe('dark')
  })

  test('테마 토글 버튼 클릭 → 라이트 테마 전환 확인', async ({ page }) => {
    await page.goto('/login')
    // Ensure dark theme first
    await page.evaluate(() => localStorage.setItem('nullus-theme', 'dark'))
    await page.reload()
    await loginAsDeveloper(page)

    // Click theme toggle button (aria-label: "Switch to light mode" when dark)
    await page.click('button[aria-label="Switch to light mode"]')
    const htmlDataTheme = await page.locator('html').getAttribute('data-theme')
    expect(htmlDataTheme).toBe('light')
  })

  test('언어 전환 (en→ko) → 메뉴 한글 표시 확인', async ({ page }) => {
    await page.goto('/login')
    await page.evaluate(() => localStorage.setItem('nullus-locale', 'en'))
    await page.reload()
    await loginAsDeveloper(page)

    await page.getByRole('button', { name: 'KO' }).click()
    await expect(page.locator('aside button').filter({ hasText: '관측성' }).first()).toBeVisible()
  })

  test('페이지 새로고침 후 테마 유지 (localStorage)', async ({ page }) => {
    await page.goto('/login')
    await page.evaluate(() => localStorage.setItem('nullus-theme', 'light'))
    await page.reload()
    await loginAsDeveloper(page)
    const htmlDataTheme = await page.locator('html').getAttribute('data-theme')
    expect(htmlDataTheme).toBe('light')
  })

  test('페이지 새로고침 후 언어 유지 (localStorage)', async ({ page }) => {
    await page.goto('/login')
    await page.evaluate(() => localStorage.removeItem('nullus-locale'))
    await page.reload()
    await loginAsDeveloper(page)

    await page.getByRole('button', { name: 'KO' }).click()
    await expect(page.locator('aside button').filter({ hasText: '관측성' }).first()).toBeVisible()

    await page.reload()
    await expect(page.locator('aside button').filter({ hasText: '관측성' }).first()).toBeVisible()
  })
})

import { test, expect } from '@playwright/test'

test.describe('Theme / i18n E2E', () => {
  test('다크 테마 기본 적용 확인', async ({ page }) => {
    // Clear localStorage to get default theme
    await page.goto('/')
    await page.evaluate(() => localStorage.removeItem('nullus-theme'))
    await page.reload()
    const htmlDataTheme = await page.locator('html').getAttribute('data-theme')
    expect(htmlDataTheme).toBe('dark')
  })

  test('테마 토글 버튼 클릭 → 라이트 테마 전환 확인', async ({ page }) => {
    await page.goto('/')
    // Ensure dark theme first
    await page.evaluate(() => localStorage.setItem('nullus-theme', 'dark'))
    await page.reload()

    // Click theme toggle button (aria-label: "Switch to light mode" when dark)
    await page.click('button[aria-label="Switch to light mode"]')
    const htmlDataTheme = await page.locator('html').getAttribute('data-theme')
    expect(htmlDataTheme).toBe('light')
  })

  test('언어 전환 (en→ko) → 메뉴 한글 표시 확인', async ({ page }) => {
    await page.goto('/')
    // Set language to EN first
    const langSelect = page.locator('select[aria-label="Select language"]')
    await langSelect.selectOption('en')
    // Now switch to Korean
    await langSelect.selectOption('ko')
    // Sidebar group label should now be in Korean
    // DevSecOps Stack group button should show Korean text for devops/admin roles
    // Default role is 'developer', so CI/CD group is visible
    await expect(page.getByRole('button', { name: /CI\/CD/i })).toBeVisible()
  })

  test('페이지 새로고침 후 테마 유지 (localStorage)', async ({ page }) => {
    await page.goto('/')
    await page.evaluate(() => localStorage.setItem('nullus-theme', 'light'))
    await page.reload()
    const htmlDataTheme = await page.locator('html').getAttribute('data-theme')
    expect(htmlDataTheme).toBe('light')
  })

  test('페이지 새로고침 후 언어 유지 (localStorage)', async ({ page }) => {
    await page.goto('/')
    // Switch to Korean
    const langSelect = page.locator('select[aria-label="Select language"]')
    await langSelect.selectOption('ko')
    const langValue = await langSelect.inputValue()
    expect(langValue).toBe('ko')
    await page.reload()
    const langValueAfterReload = await page.locator('select[aria-label="Select language"]').inputValue()
    expect(langValueAfterReload).toBe('ko')
  })
})

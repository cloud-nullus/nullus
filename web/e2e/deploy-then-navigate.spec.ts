import { test, expect } from '@playwright/test'
import { loginAs } from './helpers/auth'

test('배포 완료 후 메뉴 전환 가능 확인', async ({ page }) => {
  await loginAs(page, 'developer')
  await page.goto('/cicd/developer-deploy')
  await page.waitForLoadState('networkidle')

  await page.fill('input[placeholder*="app"]', 'nav-test')
  await page.getByRole('button', { name: '다음' }).click()
  await page.waitForTimeout(300)

  const gitInput = page.locator('input[placeholder*="github"], input[placeholder*="repo"]').first()
  if (await gitInput.isVisible()) await gitInput.fill('https://github.com/cloud-nullus/sample-go-api')
  await page.getByRole('button', { name: '다음' }).click()
  await page.waitForTimeout(300)

  await page.getByRole('button', { name: '다음' }).click()
  await page.waitForTimeout(300)
  await page.getByRole('button', { name: '다음' }).click()
  await page.waitForTimeout(300)
  await page.getByRole('button', { name: '다음' }).click()
  await page.waitForTimeout(300)

  await page.getByRole('button', { name: /Deploy/i }).click()
  await page.waitForTimeout(10000)

  const sidebar = page.locator('aside')
  await sidebar.getByText('CI/CD List').click()
  await page.waitForTimeout(2000)
  await expect(page).toHaveURL(/\/cicd\/list/, { timeout: 5000 })
  await page.screenshot({ path: 'e2e/screenshots/after-deploy-navigate.png' })
})

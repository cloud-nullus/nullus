import { test, expect } from '@playwright/test'
import { loginAs } from './helpers/auth'

test.describe('Developer 역할 화면 검증', () => {
  test('홈 — Pipeline Setup & Deploy + 사이드바 메뉴 검증', async ({ page }) => {
    await loginAs(page, 'developer')
    await expect(page).toHaveURL(/\/cicd\/developer-deploy/)
    await page.waitForTimeout(500)
    await page.screenshot({ path: 'e2e/screenshots/dev-01-home.png' })

    const sidebar = page.locator('aside')
    await expect(sidebar.getByText('CI/CD List')).toBeVisible()
    await expect(sidebar.getByText('CI/CD History')).toBeVisible()
    await expect(sidebar.getByText('Monitoring', { exact: false }).first()).toBeVisible()
    await expect(sidebar.getByText('Alert History')).toBeVisible()
    await expect(sidebar.getByText('CI/CD Template')).toHaveCount(0)
    await expect(sidebar.getByText('Alert Rules')).toHaveCount(0)
    await expect(sidebar.getByText('Organization')).toHaveCount(0)
    await expect(sidebar.getByText('Stack Template')).toHaveCount(0)
  })

  test('CI/CD List 페이지', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.goto('/cicd/list')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({ path: 'e2e/screenshots/dev-02-cicd-list.png' })
  })

  test('CI/CD History 페이지', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.goto('/cicd/history')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({ path: 'e2e/screenshots/dev-03-cicd-history.png' })
  })

  test('Monitoring Dashboard 페이지', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.goto('/observability/monitoring')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({ path: 'e2e/screenshots/dev-04-monitoring.png' })
  })

  test('Alert History 페이지', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.goto('/observability/alert-history')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({ path: 'e2e/screenshots/dev-05-alert-history.png' })
  })
})

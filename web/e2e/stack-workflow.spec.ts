import { test, expect } from '@playwright/test'

test.describe('Stack Workflow E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops123')
    await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
    await page.click('button[type="submit"]')
    await page.waitForURL('**/stack/templates')
  })

  test('Stack Templates 페이지 → 3개 카드 표시', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })
    const cards = page.locator('main [class*="card"]').filter({ hasText: /Use Base Template/ })
    await expect(cards).toHaveCount(3)
  })

  test('"Use Template" 클릭 → Install 페이지 이동', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })
    const cards = page.locator('main [class*="card"]').filter({ hasText: /Use Base Template/ })
    await cards.first().getByRole('button', { name: 'Use Base Template' }).click()
    await page.waitForURL('**/stack/install*', { timeout: 10000 })
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
  })

  test('5단계 탭 전환 (Artifacts → CI/CD → Observability → Resources → YAML View)', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })

    const tabs = ['Artifacts', 'CI/CD', 'Observability', 'Resources', 'YAML View']
    for (const tab of tabs) {
      const tabBtn = page.locator('main').getByRole('tab', { name: tab }).or(
        page.locator('main button').filter({ hasText: new RegExp(`^${tab}$`) })
      ).first()
      await tabBtn.click()
      await expect(tabBtn).toBeVisible()
    }
  })

  test('YAML View 탭에서 현재 설정 표시 확인', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
    await page.click('button:has-text("YAML View")')
    await expect(page.getByText('stackName:')).toBeVisible()
    await expect(page.getByText('artifacts:')).toBeVisible()
  })

  test('Resources 탭에서 입력 필드 확인', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
    await page.click('button:has-text("Resources")')
    await expect(page.getByText('개발자 수')).toBeVisible()
    await expect(page.getByText('동시 러너 수')).toBeVisible()
    await expect(page.getByText('일일 커밋 수')).toBeVisible()
  })

  test('Stack List 페이지 렌더링 확인', async ({ page }) => {
    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText('Stack List', { timeout: 10000 })
  })
})

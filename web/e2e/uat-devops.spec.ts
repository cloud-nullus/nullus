import { test, expect } from '@playwright/test'

test.describe('UAT: DevOps Engineer 미정', () => {
  test('로그인 페이지 접속 (/login)', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('h1')).toContainText('Nullus Platform')
    await expect(page.locator('#email')).toBeVisible()
  })

  test('devops@nullus.dev 이메일 입력 + Sign In → Stack Templates로 리다이렉트', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops')
    await page.click('button[type="submit"]')
    await page.waitForURL('/stack/templates')
    await expect(page.locator('h1')).toContainText('Golden Path Templates', { timeout: 10000 })
  })

  test('"Get Started" CTA 버튼 표시 확인', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops')
    await page.click('button[type="submit"]')
    await page.waitForURL('/stack/templates')
    await page.goto('/')
    await expect(page.locator('button').filter({ hasText: /Get Started|시작하기/ })).toBeVisible()
  })

  test('사이드바에 DevSecOps Stack, CI/CD, Observability 메뉴 표시', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops')
    await page.click('button[type="submit"]')
    await page.waitForURL('/stack/templates')

    // Group headers are buttons in the sidebar
    await expect(page.locator('aside button').filter({ hasText: 'DevSecOps Stack' }).first()).toBeVisible()
    await expect(page.locator('aside button').filter({ hasText: 'CI/CD' }).first()).toBeVisible()
    await expect(page.locator('aside button').filter({ hasText: 'Observability' }).first()).toBeVisible()
  })

  test('Stack Templates 이동 → 3개 템플릿 카드 확인', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops')
    await page.click('button[type="submit"]')
    await page.waitForURL('/stack/templates')
    await expect(page.locator('h1')).toContainText('Golden Path Templates', { timeout: 10000 })

    // 3 template card headings
    await expect(page.locator('h3').filter({ hasText: 'GitLab All-in-One' })).toBeVisible()
    await expect(page.locator('h3').filter({ hasText: 'GitLab + ArgoCD' })).toBeVisible()
    await expect(page.locator('h3').filter({ hasText: 'GitHub + ArgoCD' })).toBeVisible()
  })

  test('Stack Install 이동 → 5개 탭 표시 확인', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops')
    await page.click('button[type="submit"]')
    await page.waitForURL('/stack/templates')

    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })

    for (const tab of ['Artifacts', 'Pipeline', 'Monitoring', 'Logging', 'Resources']) {
      await expect(page.locator(`button:has-text("${tab}")`)).toBeVisible()
    }
  })

  test('Monitoring Dashboard 이동 → KPI 카드 표시 확인', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops')
    await page.click('button[type="submit"]')
    await page.waitForURL('/stack/templates')

    await page.goto('/observability/monitoring')
    await expect(page.locator('h1')).toContainText('Monitoring Dashboard', { timeout: 10000 })
    await expect(page.getByText('CPU 사용률')).toBeVisible()
    await expect(page.getByText('메모리 사용률')).toBeVisible()
  })
})

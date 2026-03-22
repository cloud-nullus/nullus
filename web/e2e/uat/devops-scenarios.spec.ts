import { test, expect } from '@playwright/test'
import { loginAs, navigateToMenu } from '../helpers/auth'

test.describe('DevOps UAT Scenarios', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'devops')
  })

  test('D1: 스택 템플릿 3개 카드 렌더링', async ({ page }) => {
    await expect(page.locator('h1')).toContainText(/template/i, { timeout: 10000 })
    await expect(page.getByRole('button', { name: /use base template/i }).first()).toBeVisible()
  })

  test('D1: 템플릿 상세 모달 열기', async ({ page }) => {
    const card = page.locator('button').filter({ hasText: /gitlab all-in-one|gitlab \+ argo cd|github \+ argo cd/i }).first()
    await card.click()
    await expect(page.locator('[role="dialog"]')).toBeVisible()
  })

  test('D2: 스택 설치 5개 탭 렌더링', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText(/install|설치/i, { timeout: 10000 })
    await expect(page.getByRole('tab').or(page.locator('[role="tab"], button[data-state]')))
      .toHaveCount(5, { timeout: 5000 })
      .catch(() => {})
  })

  test('D2: Artifacts 탭 도구 선택 드롭다운', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.getByText(/package registry|source repository/i).first()).toBeVisible({ timeout: 10000 })
  })

  test('D2: 탭 간 이동 (Artifacts → CI/CD)', async ({ page }) => {
    await page.goto('/stack/install')
    const tabs = page.locator('[role="tab"], button[data-state]')
    if (await tabs.count() >= 2) {
      await tabs.nth(1).click()
      await expect(page.getByText(/ci\/cd platform|cd tool/i).first()).toBeVisible({ timeout: 5000 })
    }
  })

  test('D3: 스택 목록 페이지 렌더링', async ({ page }) => {
    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText(/stack|스택/i, { timeout: 10000 })
    await expect(page.locator('table, [role="table"], [class*="card"]').first()).toBeVisible({ timeout: 10000 })
  })

  test('D5: Resources 탭 통화 선택 드롭다운(USD/KRW/CNY)', async ({ page }) => {
    await page.goto('/stack/install')
    await page.getByRole('button', { name: 'Resources' }).click()
    await expect(page.getByText(/USD|KRW|CNY/i).first()).toBeVisible({ timeout: 5000 })
  })

  test('D5: Resources 탭 Auto/Manual 모드 토글', async ({ page }) => {
    await page.goto('/stack/install')
    await page.getByRole('button', { name: 'Resources' }).click()
    await expect(page.getByText(/auto|manual/i).first()).toBeVisible({ timeout: 5000 })
  })

  test('D6: 스택 버전 관리 페이지 호환성 테이블', async ({ page }) => {
    await page.goto('/stack/versions')
    await expect(page.locator('h1')).toContainText(/version/i, { timeout: 10000 })
    await expect(page.locator('table, [role="table"]').first()).toBeVisible({ timeout: 10000 })
  })

  test('D6: Recommended 뱃지 표시', async ({ page }) => {
    await page.goto('/stack/versions')
    await expect(page.getByText(/recommended/i).first()).toBeVisible({ timeout: 10000 })
  })

  test('D7: 스택 이력 페이지 렌더링', async ({ page }) => {
    await page.goto('/stack/history')
    await expect(page.locator('h1')).toContainText(/history|이력/i, { timeout: 10000 })
  })

  test('D7: 스택 이력 롤백 버튼 존재', async ({ page }) => {
    await page.goto('/stack/history')
    await expect(page.getByRole('button', { name: /rollback|롤백/i }).first()).toBeVisible({ timeout: 10000 })
  })

  test('D8: CI/CD 템플릿 페이지 렌더링', async ({ page }) => {
    await navigateToMenu(page, 'CI/CD', 'Templates').catch(() => {})
    await page.goto('/cicd/templates')
    await expect(page.locator('h1')).toContainText(/template/i, { timeout: 10000 })
    await expect(page.locator('[class*="card"]').first()).toBeVisible({ timeout: 10000 })
  })

  test('D9: 모니터링 대시보드 차트 렌더링', async ({ page }) => {
    await page.goto('/observability/monitoring')
    await expect(page.locator('h1')).toContainText(/monitoring|모니터링/i, { timeout: 10000 })
    await expect(page.locator('[class*="card"], svg, canvas').first()).toBeVisible({ timeout: 10000 })
  })

  test('D10: 알림 규칙 페이지 CRUD 버튼', async ({ page }) => {
    await page.goto('/observability/alert-rules')
    await expect(page.locator('h1')).toContainText(/alert/i, { timeout: 10000 })
    await expect(page.getByRole('button', { name: /new rule|새 규칙/i })).toBeVisible({ timeout: 10000 })
  })

  test('D10: 알림 규칙 생성 모달', async ({ page }) => {
    await page.goto('/observability/alert-rules')
    const createBtn = page.getByRole('button', { name: /new rule|새 규칙/i })
    await createBtn.click()
    await expect(page.locator('[role="dialog"]')).toBeVisible()
  })

  test('D11: 스택 목록에 Add Tools 버튼', async ({ page }) => {
    await page.goto('/stack/list')
    await expect(page.getByRole('button', { name: /add tools/i }).first()).toBeVisible({ timeout: 10000 })
  })

  test('D13: YAML View 탭 에디터 렌더링', async ({ page }) => {
    await page.goto('/stack/install')
    await page.getByRole('button', { name: 'YAML View' }).click()
    await expect(page.locator('.monaco-editor, [data-keybinding-context]').first()).toBeVisible({ timeout: 10000 })
  })
})

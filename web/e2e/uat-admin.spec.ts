import { test, expect } from '@playwright/test'

test.describe('UAT: Admin 관리자', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'admin@nullus.dev')
    await page.fill('#password', 'admin')
    await page.click('button[type="submit"]')
    await page.waitForURL('/admin/organization')
  })

  test('로그인 성공 - admin role로 /admin/organization 리다이렉트', async ({ page }) => {
    expect(page.url()).toContain('/admin/organization')
  })

  test('Organization 페이지 접근 → 폼 필드 확인', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Organization', { timeout: 10000 })
    await expect(page.getByText('조직 이름')).toBeVisible()
    await expect(page.getByText('슬러그 (Slug)')).toBeVisible()
    await expect(page.getByText('도메인')).toBeVisible()
  })

  test('User Management 페이지 접근 → 테이블 표시', async ({ page }) => {
    await page.goto('/admin/users')
    await expect(page.locator('h1')).toContainText('User Management', { timeout: 10000 })
    await expect(page.locator('table')).toBeVisible()
    await expect(page.getByText('Alice Kim')).toBeVisible()
    await expect(page.getByText('Bob Lee')).toBeVisible()
  })

  test('Cluster Management 페이지 접근 → 리스트+상세 레이아웃', async ({ page }) => {
    await page.goto('/admin/clusters')
    await expect(page.locator('h1')).toContainText('Cluster Management', { timeout: 10000 })
    // List panel shows cluster names - use first() to avoid strict mode violation
    await expect(page.getByText('prod-cluster').first()).toBeVisible()
    await expect(page.getByText('staging-cluster').first()).toBeVisible()
    // Detail panel shows connection status for first auto-selected cluster
    await expect(page.getByText('연결 상태')).toBeVisible()
  })

  test('사이드바에 Admin, DevSecOps Stack 메뉴 표시 확인', async ({ page }) => {
    await expect(page.locator('aside button').filter({ hasText: 'Admin' }).first()).toBeVisible()
    await expect(page.locator('aside button').filter({ hasText: 'DevSecOps Stack' }).first()).toBeVisible()
  })
})

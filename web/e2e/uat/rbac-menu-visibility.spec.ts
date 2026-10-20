import { expect, test } from '@playwright/test'

import { expectMenuHidden, expectMenuVisible, loginAs } from '../helpers/auth'

test.describe('RBAC Menu Visibility', () => {
  test('Admin: 모든 메뉴 그룹 표시 (데브섹옵스, CI/CD, 관측성, 관리)', async ({ page }) => {
    await loginAs(page, 'admin')
    await expectMenuVisible(page, 'DevSecOps Stack')
    await expectMenuVisible(page, 'CI/CD')
    await expectMenuVisible(page, 'Observability')
    await expectMenuVisible(page, 'Admin')
  })

  test('DevOps: 데브섹옵스/CI-CD/관측성 표시, 관리 숨김', async ({ page }) => {
    await loginAs(page, 'devops')
    await expectMenuVisible(page, 'DevSecOps Stack')
    await expectMenuVisible(page, 'CI/CD')
    await expectMenuVisible(page, 'Observability')
    await expectMenuHidden(page, 'Admin')
  })

  test('Developer: CI-CD/관측성 표시, 데브섹옵스/관리 숨김', async ({ page }) => {
    await loginAs(page, 'developer')
    await expectMenuHidden(page, 'DevSecOps Stack')
    await expectMenuVisible(page, 'CI/CD')
    await expectMenuVisible(page, 'Observability')
    await expectMenuHidden(page, 'Admin')
  })

  test('Admin: 관리 하위 메뉴 (조직, 사용자, 클러스터, Known Issues)', async ({ page }) => {
    await loginAs(page, 'admin')
    await expect(page.locator('aside a[href="/admin/organization"]').first()).toBeVisible({ timeout: 5000 })
    await expect(page.locator('aside a[href="/admin/users"]').first()).toBeVisible({ timeout: 5000 })
    await expect(page.locator('aside a[href="/admin/clusters"]').first()).toBeVisible({ timeout: 5000 })
    await expect(page.locator('aside a[href="/admin/known-issues"]').first()).toBeVisible({ timeout: 5000 })
  })

  test('Developer: CI/CD 하위 메뉴에서 Templates 숨김', async ({ page }) => {
    await loginAs(page, 'developer')
    await expect(page.locator('aside a[href="/cicd/templates"]')).toHaveCount(0)
    await expect(page.locator('aside a[href="/cicd/list"]').first()).toBeVisible({ timeout: 5000 })
    await expect(page.locator('aside a[href="/cicd/history"]').first()).toBeVisible({ timeout: 5000 })
  })

  // 알림 규칙은 평가되지 않아 메뉴에서 뺐다(nullus-plan#87). 알림 이력(스택 경보)은 보인다.
  test('DevOps: 알림 이력 메뉴 표시 · 알림 규칙 메뉴 없음', async ({ page }) => {
    await loginAs(page, 'devops')
    await expect(page.locator('aside a[href="/observability/alert-history"]').first()).toBeVisible({ timeout: 5000 })
    await expect(page.locator('aside a[href="/observability/alerts"]')).toHaveCount(0)
  })

  test('Developer: 알림 규칙 메뉴 숨김', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.locator('aside button').filter({ hasText: 'Observability' }).first().click()
    await expectMenuHidden(page, 'Alert Rules')
  })

  // 보안은 관측성 아래 하위 그룹이다(메뉴 3단 — Security Dashboard 설계 §4.1 · §7).
  // 세 역할 모두 보안 대시보드를 본다. 설치 OSS 탭은 admin · devops 만이다(§4.4).
  for (const role of ['admin', 'devops', 'developer'] as const) {
    test(`${role}: 관측성 › 보안 › 보안 대시보드 메뉴 표시`, async ({ page }) => {
      await loginAs(page, role)
      await expect(page.locator('aside button[aria-label="Security"]').first()).toBeVisible({ timeout: 5000 })
      await expect(page.locator('aside a[href="/observability/security/dashboard"]').first()).toBeVisible({ timeout: 5000 })
    })
  }

  test('Developer: 보안 대시보드에 설치 OSS 탭 없음', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.goto('/observability/security/dashboard')
    await expect(page.locator('button[data-tab="apps"]')).toBeVisible({ timeout: 10000 })
    await expect(page.locator('button[data-tab="oss"]')).toHaveCount(0)
  })

  test('DevOps: 보안 대시보드에 설치 OSS 탭 표시', async ({ page }) => {
    await loginAs(page, 'devops')
    await page.goto('/observability/security/dashboard')
    await expect(page.locator('button[data-tab="oss"]')).toBeVisible({ timeout: 10000 })
  })

  test('/observability/security → 보안 대시보드 리다이렉트', async ({ page }) => {
    await loginAs(page, 'developer')
    await page.goto('/observability/security')
    await expect(page).toHaveURL(/\/observability\/security\/dashboard/, { timeout: 10000 })
  })

  test('Admin 로그인 → /admin/organization 리다이렉트', async ({ page }) => {
    await loginAs(page, 'admin')
    await expect(page).toHaveURL(/\/admin\/organization/, { timeout: 10000 })
  })

  test('DevOps 로그인 → /stack/templates 리다이렉트', async ({ page }) => {
    await loginAs(page, 'devops')
    await expect(page).toHaveURL(/\/stack\/templates/, { timeout: 10000 })
  })

  test('Developer 로그인 → /cicd/developer-deploy 리다이렉트', async ({ page }) => {
    await loginAs(page, 'developer')
    await expect(page).toHaveURL(/\/cicd\/developer-deploy/, { timeout: 10000 })
  })
})

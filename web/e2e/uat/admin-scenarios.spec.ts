import { expect, test } from '@playwright/test'

import { loginAs } from '../helpers/auth'

test.describe('Admin UAT Scenarios', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'admin')
  })

  test('A1: Organization 페이지 폼 필드 렌더링', async ({ page }) => {
    await expect(page.locator('h1')).toContainText(/organization/i, { timeout: 10000 })
    const detailName = page.locator('input[name="name"]').first()
    if (await detailName.isVisible()) {
      await expect(detailName).toBeVisible()
      await expect(page.locator('input[name="slug"]').first()).toBeVisible()
      await expect(page.locator('input[name="domain"]').first()).toBeVisible()
      // Select 는 네이티브 <select> 가 아니라 MUI 기반이라 role=combobox 로 뜬다.
      // select[name="status"] 는 이 화면에서 영원히 0 개다.
      await expect(page.getByRole('combobox', { name: /status/i })).toBeVisible()
      return
    }

    await page.getByRole('button', { name: /new organization/i }).click()
    await expect(page.locator('[role="dialog"]')).toBeVisible({ timeout: 10000 })
    await expect(page.locator('[role="dialog"] input[name="name"]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] input[name="slug"]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] input[name="domain"]').first()).toBeVisible()
  })

  test('A1: Organization 정보 수정 및 저장', async ({ page }) => {
    const unique = Date.now().toString().slice(-6)

    const detailName = page.locator('input[name="name"]').first()
    if (await detailName.isVisible()) {
      await detailName.fill(`Nullus Org ${unique}`)
      await page.locator('input[name="slug"]').first().fill(`nullus-org-${unique}`)

      const domainField = page.locator('input[name="domain"]').first()
      if (await domainField.isVisible()) {
        await domainField.fill(`admin-${unique}.nullus.dev`)
      }

      await page.getByRole('button', { name: /save changes|save|저장/i }).first().click()
      return
    }

    await page.getByRole('button', { name: /new organization/i }).click()
    await page.locator('[role="dialog"] input[name="name"]').first().fill(`Nullus Org ${unique}`)
    await page.locator('[role="dialog"] input[name="slug"]').first().fill(`nullus-org-${unique}`)
    await page.locator('[role="dialog"] input[name="domain"]').first().fill(`admin-${unique}.nullus.dev`)
    await expect(page.getByRole('button', { name: /create organization/i })).toBeEnabled()
    await page.getByRole('button', { name: /create organization/i }).click()
  })

  test('A2: 사용자 관리 페이지 멤버 초대 모달', async ({ page }) => {
    await page.goto('/admin/users')
    await expect(page.locator('h1')).toContainText(/user/i, { timeout: 10000 })
    await page.getByRole('button', { name: /^Users$/ }).click()

    const inviteBtn = page.getByRole('button', { name: /invite user|사용자 초대/i }).first()
    await inviteBtn.click()

    await expect(page.locator('[role="dialog"]').first()).toBeVisible({ timeout: 10000 })
    await expect(page.locator('[role="dialog"] input[placeholder="user@example.com"], [role="dialog"] input[type="email"]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] select, [role="dialog"] [role="combobox"]').first()).toBeVisible()
  })

  test('A2: 초대 링크 생성 모달 열기', async ({ page }) => {
    await page.goto('/admin/users')
    await expect(page.locator('h1')).toContainText(/user/i, { timeout: 10000 })
    await page.getByRole('button', { name: /^Users$/ }).click()

    const linkBtn = page.getByRole('button', { name: /generate invite link|invite link|generate/i }).first()
    await linkBtn.click()

    await expect(page.locator('[role="dialog"]').first()).toBeVisible({ timeout: 10000 })
    await expect(page.locator('[role="dialog"] [role="combobox"]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] [role="combobox"]')).toHaveCount(2)
  })

  test('A3: 사용자 관리 멤버 목록 테이블 렌더링', async ({ page }) => {
    await page.goto('/admin/users')
    await page.getByRole('button', { name: /^Users$/ }).click()
    await expect(page.locator('table, [role="table"]').first()).toBeVisible({ timeout: 10000 })
  })

  test('A4: 클러스터 등록 모달 — kubeconfig textarea + 파일 업로드', async ({ page }) => {
    await page.goto('/admin/clusters')
    await expect(page.locator('h1')).toContainText(/cluster/i, { timeout: 10000 })

    const registerBtn = page.getByRole('button', { name: /register|등록/i }).first()
    await registerBtn.click()

    await expect(page.locator('[role="dialog"]').first()).toBeVisible({ timeout: 10000 })
    await expect(page.locator('[role="dialog"] input[name="name"], [role="dialog"] input[placeholder*="name" i]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] select, [role="dialog"] [role="combobox"]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] textarea').first()).toBeVisible()
    await expect(page.getByRole('button', { name: /choose file/i })).toBeVisible()
  })

  test('A5: 클러스터 관리 목록 및 상태 배지', async ({ page }) => {
    await page.goto('/admin/clusters')
    await expect(page.getByText(/clusters \(/i)).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('button', { name: /register cluster/i })).toBeVisible({ timeout: 10000 })
  })

  test('A6: Organization 상태 드롭다운 존재', async ({ page }) => {
    // 예전에는 select[name="status"] 를 찾고 못 찾으면 "organization 이라는 글자가
    // 있는지" 로 넘어갔다. Select 가 MUI 로 바뀌어 앞 조건은 늘 거짓이 됐고,
    // 뒤 조건은 사이드바만 있어도 통과한다 — 드롭다운이 사라져도 초록불이었다.
    await expect(page.getByRole('combobox', { name: /status/i })).toBeVisible({ timeout: 10000 })
  })

  test('A7: Known Issues 페이지 렌더링', async ({ page }) => {
    await page.goto('/admin/known-issues')
    await expect(page.locator('h1')).toContainText(/known issues/i, { timeout: 10000 })
  })
})

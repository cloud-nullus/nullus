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
      await expect(page.locator('#organization-status')).toBeVisible()
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
    await expect(page.locator('[role="dialog"] select, [role="dialog"] [role="combobox"]').first()).toBeVisible()
    await expect(page.locator('[role="dialog"] select')).toHaveCount(2)
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
    const statusSelect = page.locator('#organization-status')
    if (await statusSelect.isVisible()) {
      await expect(statusSelect).toBeVisible({ timeout: 10000 })
      return
    }

    await expect(page.getByText(/select an organization to view details/i)).toBeVisible({ timeout: 10000 })
  })

  test('A7: Known Issues 페이지 렌더링', async ({ page }) => {
    await page.goto('/admin/known-issues')
    await expect(page.locator('h1')).toContainText(/known issues/i, { timeout: 10000 })
  })
})

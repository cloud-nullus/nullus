import { type Page, expect } from '@playwright/test'

export type TestRole = 'admin' | 'devops' | 'developer'

const ACCOUNTS: Record<TestRole, { email: string; password: string; homePath: string }> = {
  admin: { email: 'admin@nullus.dev', password: 'admin123', homePath: '/admin/organization' },
  devops: { email: 'devops@nullus.dev', password: 'devops123', homePath: '/stack/templates' },
  developer: { email: 'developer@nullus.dev', password: 'developer123', homePath: '/cicd/developer-deploy' },
}

export async function loginAs(page: Page, role: TestRole): Promise<void> {
  const { email, password, homePath } = ACCOUNTS[role]
  await page.goto('/login')
  await page.fill('#email', email)
  await page.fill('#password', password)
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
  await page.click('button[type="submit"]')
  await page.waitForURL(`**${homePath}`, { timeout: 10000 })
}

export async function expectMenuVisible(page: Page, menuText: string): Promise<void> {
  await expect(page.locator('aside').getByText(menuText, { exact: false }).first()).toBeVisible({ timeout: 5000 })
}

export async function expectMenuHidden(page: Page, menuText: string): Promise<void> {
  await expect(page.locator('aside').getByText(menuText, { exact: false })).toHaveCount(0, { timeout: 5000 })
}

export async function navigateToMenu(page: Page, groupText: string, itemText: string): Promise<void> {
  const group = page.locator('aside button').filter({ hasText: groupText }).first()
  await group.click()
  const item = page.locator('aside').getByText(itemText, { exact: false }).first()
  await item.click()
  await page.waitForLoadState('networkidle')
}

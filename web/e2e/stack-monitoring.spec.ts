import { test, expect, type APIRequestContext } from '@playwright/test'

const apiBase = 'http://localhost:8090/api/v1'

async function getCompletedStack(request: APIRequestContext): Promise<{ id: string; name: string }> {
  const res = await request.get(`${apiBase}/stacks`)
  expect(res.ok()).toBeTruthy()

  const body = (await res.json()) as {
    items?: Array<{ id: string; name: string; state?: string; namespace?: string }>
  }

  const stack = (body.items ?? []).find((item) => item.state === 'completed' && item.namespace === 'nullus')
  expect(stack).toBeTruthy()

  return { id: stack!.id, name: stack!.name }
}

test.describe('Stack Monitoring E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops123')
    await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
    await page.click('button[type="submit"]')
    // Login page now navigates to '/' (Home) regardless of role — F8 Task 5 +
    // Phase 1 (2026-04-20). Each test explicitly navigates to its target
    // stack-* route, so beforeEach only needs to confirm auth succeeded.
    await page.waitForURL('**/')
  })

  test('@stack-critical completed stack monitoring renders live values', async ({ page, request }) => {
    test.setTimeout(90000)
    const healthCheck = await request.get('http://localhost:8090/health').catch(() => null)
    test.skip(!healthCheck?.ok(), 'Requires running Go backend on port 8090')
    const stackListRes = await request.get(`${apiBase}/stacks`).catch(() => null)
    test.skip(!stackListRes?.ok(), 'Requires access to the stacks API')

    const stackList = (await stackListRes.json()) as {
      items?: Array<{ id: string; name: string; state?: string; namespace?: string }>
    }
    const target = (stackList.items ?? []).find((item) => item.state === 'completed' && item.namespace === 'nullus')
    test.skip(!target, 'Requires a completed stack in the nullus namespace')

    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText('Stack List', { timeout: 10000 })

    await page.getByRole('textbox').first().fill(target.name)
    await expect(page.getByText(target.name).first()).toBeVisible({ timeout: 10000 })
    await page.getByText(target.name).first().click()

    await page.getByRole('button', { name: 'Monitoring' }).click()
    await expect(page.getByRole('heading', { name: 'Tool Health' })).toBeVisible({ timeout: 10000 })
    await expect(page.getByText('Resource Trend')).toBeVisible({ timeout: 10000 })

    await expect(page.getByText('Ready Pods')).toBeVisible({ timeout: 10000 })

    const readyPodsMetric = page.getByText(/\d+\s*\/\s*\d+/).first()
    await expect(readyPodsMetric).toBeVisible({ timeout: 10000 })

    await expect(page.getByAltText(/icon$/i).first()).toBeVisible({ timeout: 10000 })

    await expect(page.getByRole('cell', { name: /gitlab/i }).first()).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('cell', { name: /argocd/i }).first()).toBeVisible({ timeout: 10000 })
  })
})

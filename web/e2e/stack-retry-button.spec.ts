import { test, expect, type Page } from '@playwright/test'

// F8 cleanup Phase 3 — pure-UI smoke for the Retry button on /stack/list.
//
// No Kind cluster dependency: the spec stubs /api/v1/stacks (list) and
// /api/v1/stacks/:id/retry via page.route so the UI flow can run against
// the frontend dev server alone. Auth is seeded into sessionStorage via
// addInitScript to skip the login form.
//
// Tag: @stack-critical (picked up by `pnpm e2e:stack-critical`).

const FAILED_STACK = {
  id: 'stack-fail',
  name: 'mock-failed-stack',
  template_id: 'tpl-x',
  cluster_id: 'c1',
  cluster_name: 'mock-cluster',
  namespace: 'nullus',
  state: 'failed',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-19T00:00:00Z',
}

const OK_STACK = {
  id: 'stack-ok',
  name: 'mock-completed-stack',
  template_id: 'tpl-y',
  cluster_id: 'c1',
  cluster_name: 'mock-cluster',
  namespace: 'nullus',
  state: 'completed',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-19T00:00:00Z',
}

const WARN_VERDICT_BODY = {
  error: {
    code: 'DEPLOY_COMPAT_WARN_UNACK',
    message: 'compat warn unack',
    verdict: {
      overall: { state: 'warn', score: 70 },
      issues: [
        { code: 'TOOL_ARCH_UNSUPPORTED', message: 'argo 2.12 lacks arm64', severity: 'warning' },
      ],
    },
  },
}

async function seedAuth(page: Page): Promise<void> {
  await page.addInitScript(() => {
    sessionStorage.setItem('nullus-token', 'mock-e2e-token')
    sessionStorage.setItem(
      'nullus-user',
      JSON.stringify({
        id: 'a1000000-0000-0000-0000-000000000001',
        name: 'Admin User',
        email: 'admin@nullus.dev',
        role: 'admin',
        orgId: '11111111-1111-1111-1111-111111111111',
      }),
    )
  })
}

async function stubStackList(page: Page, items: unknown[]): Promise<void> {
  await page.route('**/api/v1/stacks*', async (route, request) => {
    const url = new URL(request.url())
    // Only stub the list GET; retry posts go to /stacks/:id/retry which has a
    // segment after /stacks so this glob still matches — differentiate by
    // path instead.
    if (request.method() === 'GET' && url.pathname.endsWith('/api/v1/stacks')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items, total: items.length }),
      })
      return
    }
    await route.fallback()
  })
}

test.describe('@stack-critical retry button smoke', () => {
  test.beforeEach(async ({ page }) => {
    await seedAuth(page)
    // Absorb other API calls the page may issue (history, monitoring, clusters)
    // by letting them 404 — the page renders its core info panel regardless,
    // and react-query swallows those failures with retry: false in the
    // QueryClient default. We only need to pin the /stacks list and retry.
    await page.route('**/api/v1/clusters*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [{ id: 'c1', name: 'mock-cluster', type: 'target', types: ['target'], cloudProvider: 'on_premise', endpoint: 'https://c1', status: 'connected', organizationIds: ['org-1'], createdAt: '2026-01-01T00:00:00Z' }], total: 1 }),
      })
    })
  })

  test('failed stack exposes the Retry button, completed stack does not', async ({ page }) => {
    await stubStackList(page, [FAILED_STACK, OK_STACK])
    await page.goto('/stack/list')
    // First row auto-selects; FAILED_STACK is first in the stub array.
    await expect(page.locator('[data-testid="retry-stack-button"]')).toBeVisible({ timeout: 5000 })

    // Switch selection to the completed stack — Retry button must disappear.
    await page.getByRole('cell', { name: /mock-completed-stack/i }).click()
    await expect(page.locator('[data-testid="retry-stack-button"]')).toHaveCount(0)
  })

  test('clicking Retry fires POST /stacks/:id/retry', async ({ page }) => {
    await stubStackList(page, [FAILED_STACK])
    let retryRequestCount = 0
    await page.route('**/api/v1/stacks/stack-fail/retry', async (route) => {
      retryRequestCount += 1
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ stack_id: 'stack-fail', status: 'pending' }),
      })
    })
    await page.goto('/stack/list')
    await page.locator('[data-testid="retry-stack-button"]').click()
    await expect.poll(() => retryRequestCount, { timeout: 5000 }).toBe(1)
  })

  test('warn-ack flow surfaces the modal and re-submits with acknowledge_warnings=true', async ({ page }) => {
    await stubStackList(page, [FAILED_STACK])
    const retryBodies: Array<Record<string, unknown> | null> = []
    await page.route('**/api/v1/stacks/stack-fail/retry', async (route, request) => {
      const body = request.postDataJSON() as Record<string, unknown> | null
      retryBodies.push(body)
      if (retryBodies.length === 1) {
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify(WARN_VERDICT_BODY),
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ stack_id: 'stack-fail', status: 'pending' }),
      })
    })
    await page.goto('/stack/list')
    await page.locator('[data-testid="retry-stack-button"]').click()

    // First click → WARN_UNACK → Modal with issue list + ack checkbox.
    await expect(page.locator('[data-testid="retry-warn-ack"]')).toBeVisible({ timeout: 5000 })
    await expect(page.getByText(/TOOL_ARCH_UNSUPPORTED/)).toBeVisible()

    // Acknowledge and re-submit.
    await page.locator('[data-testid="retry-warn-ack"]').check()
    await page.locator('[data-testid="retry-warn-confirm"]').click()

    await expect.poll(() => retryBodies.length, { timeout: 5000 }).toBe(2)
    expect(retryBodies[0]).toBeNull()
    expect(retryBodies[1]).toEqual({ acknowledge_warnings: true })
  })
})

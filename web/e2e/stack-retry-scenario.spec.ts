import { test, expect, type Page } from '@playwright/test'

// F8-UIUX-E2EDeepScenario — deploy-fail → retry warn-ack → success.
// Backend-free (Tier B): all API calls are stubbed via page.route so the
// spec runs against the Vite dev server alone. Complements the existing
// stack-retry-button.spec.ts by walking the full recovery path in a
// single scenario instead of three single-purpose checks.

const FAILED_STACK = {
  id: 'stack-scenario',
  name: 'scenario-stack',
  template_id: 'tpl-a',
  cluster_id: 'c-1',
  cluster_name: 'scenario-cluster',
  namespace: 'nullus',
  state: 'failed',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-21T00:00:00Z',
}

const WARN_BODY = {
  error: {
    code: 'DEPLOY_COMPAT_WARN_UNACK',
    message: 'compat warn unack',
    verdict: {
      overall: { state: 'warn', score: 60 },
      issues: [
        { code: 'TOOL_ARCH_UNSUPPORTED', message: 'argo lacks arm64', severity: 'warning', tool: 'argo-cd' },
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

test.describe('@stack-critical deep retry scenario', () => {
  test('deploy-fail → retry warn → ack → success', async ({ page }) => {
    await seedAuth(page)
    await page.route('**/api/v1/clusters*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [
            {
              id: 'c-1',
              name: 'scenario-cluster',
              type: 'target',
              types: ['target'],
              cloudProvider: 'on_premise',
              endpoint: 'https://c-1',
              status: 'connected',
              organizationIds: ['org-1'],
              createdAt: '2026-01-01T00:00:00Z',
            },
          ],
          total: 1,
        }),
      })
    })
    await page.route('**/api/v1/stacks*', async (route, request) => {
      const url = new URL(request.url())
      if (request.method() === 'GET' && url.pathname.endsWith('/api/v1/stacks')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [FAILED_STACK], total: 1 }),
        })
        return
      }
      await route.fallback()
    })

    const retryBodies: Array<Record<string, unknown> | null> = []
    await page.route('**/api/v1/stacks/stack-scenario/retry', async (route, request) => {
      const body = request.postDataJSON() as Record<string, unknown> | null
      retryBodies.push(body)
      if (retryBodies.length === 1) {
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify(WARN_BODY),
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ stack_id: 'stack-scenario', status: 'pending' }),
      })
    })

    await page.goto('/stack/list')
    // Trigger retry — first call returns WARN_UNACK, opening the ack modal.
    await page.locator('[data-testid="retry-stack-button"]').click()
    await expect(page.locator('[data-testid="retry-warn-ack"]')).toBeVisible({ timeout: 5000 })
    await expect(page.getByText(/TOOL_ARCH_UNSUPPORTED/)).toBeVisible()

    // Acknowledge and confirm — second call returns 200.
    await page.locator('[data-testid="retry-warn-ack"]').check()
    await page.locator('[data-testid="retry-warn-confirm"]').click()

    // Two retry calls observed, second one flagged ack.
    await expect.poll(() => retryBodies.length, { timeout: 5000 }).toBe(2)
    expect(retryBodies[0]).toBeNull()
    expect(retryBodies[1]).toEqual({ acknowledge_warnings: true })

    // Success toast surfaces after the second retry.
    await expect(page.getByText(/재배포를 시작|Redeploy started/)).toBeVisible({ timeout: 5000 })
  })
})

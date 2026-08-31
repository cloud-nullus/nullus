import { test, expect, type APIRequestContext } from '@playwright/test'
import { loginAs } from './helpers/auth'

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
    // 각 테스트가 자기 stack-* 경로로 직접 이동하므로 여기서는 인증만 확인한다.
    // 도착지는 role-landing.ts 가 정하고 helpers/auth.ts 가 그 값을 안다.
    await loginAs(page, 'devops')
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

    await page.fill('input[placeholder="스택 검색..."]', target.name)
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

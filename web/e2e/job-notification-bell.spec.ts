import { test, expect, type Page } from '@playwright/test'

// N-3 — 상단 작업 알림 종의 진행 → 완료 전환 E2E.
//
// 백엔드 없이 돈다(Tier B): 목록 API 두 개를 page.route 로 세워 두고, 스택
// 상태를 도중에 installing → completed 로 바꿔 종과 드롭다운이 따라오는지 본다.
// 클러스터가 필요 없으므로 CI 에서 그대로 돌릴 수 있다.

const STACK_ID = 'e2e-bell-stack'

function stackPayload(state: string) {
  return {
    id: STACK_ID,
    name: 'bell-demo-stack',
    template_id: 'tpl-e2e',
    template_name: 'E2E Template',
    cluster_id: 'c1',
    cluster_name: 'e2e-cluster',
    namespace: 'nullus',
    state,
    created_at: new Date(Date.now() - 120_000).toISOString(),
    updated_at: new Date(Date.now() - 120_000).toISOString(),
  }
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

test('진행 중 작업이 종에 뜨고 완료로 전환된다', async ({ page }) => {
  // 라우트 핸들러가 매 폴링마다 읽는다. 테스트 중간에 값을 바꿔 전환을 만든다.
  let stackState = 'installing'

  await seedAuth(page)

  await page.route('**/api/v1/stacks*', async (route, request) => {
    const url = new URL(request.url())
    if (request.method() === 'GET' && url.pathname.endsWith('/api/v1/stacks')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [stackPayload(stackState)], total: 1 }),
      })
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], total: 0 }),
    })
  })

  // getDeployments 는 배포와 파이프라인을 함께 부른다. 둘 다 비워 스택만 남긴다.
  for (const path of ['**/api/v1/cicd/deployments*', '**/api/v1/cicd/pipelines*']) {
    await page.route(path, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], total: 0 }),
      })
    })
  }

  await page.route('**/api/v1/clusters*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], total: 0 }),
    })
  })

  await page.goto('/')

  const bell = page.getByTestId('job-notification-bell')
  await expect(bell).toHaveAttribute('data-busy', 'true', { timeout: 10000 })
  await expect(page.getByTestId('job-notification-count')).toHaveText('1')

  await bell.click()
  const panel = page.getByTestId('job-notification-panel')
  await expect(panel).toBeVisible()
  await expect(panel.getByText('bell-demo-stack')).toBeVisible()
  await expect(panel.getByText(/Installing|설치 중/)).toBeVisible()

  const item = page.getByTestId(`job-notification-item-stack:${STACK_ID}`)
  await expect(item).toHaveAttribute('data-phase', 'running')

  // 여기서 서버 상태를 바꾼다. 다음 폴링(3초)이 전환을 물어 온다.
  stackState = 'completed'

  await expect(item).toHaveAttribute('data-phase', 'succeeded', { timeout: 15000 })
  await expect(bell).toHaveAttribute('data-busy', 'false')
  await expect(page.getByTestId('job-notification-count')).toHaveCount(0)

  // 끝난 작업도 상세로 갈 수 있어야 한다 — 실패 원인을 보러 가는 경로다.
  await item.click()
  await expect(page).toHaveURL(new RegExp(`/stack/logs/${STACK_ID}`))
  await expect(panel).toHaveCount(0)
})

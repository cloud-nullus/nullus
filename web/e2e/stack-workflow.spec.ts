import { test, expect, type APIRequestContext } from '@playwright/test'

const apiBase = 'http://localhost:8090/api/v1'

async function pollStackState(request: APIRequestContext, stackId: string, timeoutMs = 180000): Promise<string> {
  const started = Date.now()
  while (Date.now() - started < timeoutMs) {
    const res = await request.get(`${apiBase}/stacks/${stackId}/status`)
    expect(res.ok()).toBeTruthy()
    const body = (await res.json()) as { data?: { state?: string } }
    const state = body.data?.state ?? ''
    if (state === 'completed' || state === 'failed' || state === 'rolled_back') {
      return state
    }
    await new Promise((resolve) => setTimeout(resolve, 5000))
  }
  return 'timeout'
}

test.describe('Stack Workflow E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'devops@nullus.dev')
    await page.fill('#password', 'devops123')
    await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
    await page.click('button[type="submit"]')
    await page.waitForURL('**/stack/templates')
  })

  test('Stack Templates 페이지 → 3개 카드 표시', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })
    const cards = page.locator('main [class*="card"]').filter({ hasText: /Use Base Template/ })
    await expect(cards).toHaveCount(3)
  })

  test('"Use Template" 클릭 → Install 페이지 이동', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })
    const cards = page.locator('main [class*="card"]').filter({ hasText: /Use Base Template/ })
    await cards.first().getByRole('button', { name: 'Use Base Template' }).click()
    await page.waitForURL('**/stack/install*', { timeout: 10000 })
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
  })

  test('5단계 탭 전환 (Artifacts → CI/CD → Observability → Resources → YAML View)', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })

    const tabs = ['Artifacts', 'CI/CD', 'Observability', 'Resources', 'YAML View']
    for (const tab of tabs) {
      const tabBtn = page.locator('main').getByRole('tab', { name: tab }).or(
        page.locator('main button').filter({ hasText: new RegExp(`^${tab}$`) })
      ).first()
      await tabBtn.click()
      await expect(tabBtn).toBeVisible()
    }
  })

  test('YAML View 탭에서 현재 설정 표시 확인', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
    await page.click('button:has-text("YAML View")')
    await expect(page.getByText('stackName:')).toBeVisible()
    await expect(page.getByText('artifacts:')).toBeVisible()
  })

  test('Resources 탭에서 입력 필드 확인', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
    await page.click('button:has-text("Resources")')
    await expect(page.getByText('개발자 수')).toBeVisible()
    await expect(page.getByText('동시 러너 수')).toBeVisible()
    await expect(page.getByText('일일 커밋 수')).toBeVisible()
  })

  test('Stack List 페이지 렌더링 확인', async ({ page }) => {
    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText('Stack List', { timeout: 10000 })
  })

  test('@stack-critical Stack List 기반 배포 상태/로그 페이지 및 템플릿 도메인 검증', async ({ page, request }) => {
    test.setTimeout(240000)

    const stackName = `pw-list-domain-${Date.now()}`
    const createPayload = {
      name: stackName,
      cluster_id: '60223295-ea34-4319-8cc9-6e5417803445',
      namespace: 'nullus',
      golden_path_id: 'gitlab-argocd-v1',
      config: {
        artifacts: {
          package_registry: { name: '', version: '', enabled: false },
          source_repository: { name: 'gitlab', version: '17.7.0', enabled: true },
          container_registry: { name: '', version: '', enabled: false },
          storage_backend: { name: '', version: '', enabled: false },
        },
        pipeline: {
          ci_platform: { name: '', version: '', enabled: false },
          cd_tool: { name: '', version: '', enabled: false },
        },
        monitoring: {
          collection: { name: '', version: '', enabled: false },
          visualization: { name: '', version: '', enabled: false },
        },
        logging: {
          collection: { name: '', version: '', enabled: false },
          search: { name: '', version: '', enabled: false },
        },
        resources: {
          developers: 4,
          concurrent_runners: 1,
          weekly_commits: 20,
          build_frequency: 'daily',
        },
      },
    }

    const createRes = await request.post(`${apiBase}/stacks`, {
      data: createPayload,
      headers: { 'Content-Type': 'application/json' },
    })
    expect(createRes.ok()).toBeTruthy()
    const createBody = (await createRes.json()) as { id: string }
    const stackId = createBody.id

    const deployRes = await request.post(`${apiBase}/stacks/${stackId}/deploy`, {
      data: {},
      headers: { 'Content-Type': 'application/json' },
    })
    expect(deployRes.status()).toBe(202)

    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText('Stack List', { timeout: 10000 })
    await expect(page.getByText(stackName)).toBeVisible({ timeout: 15000 })

    await page.goto(`/stack/logs/${stackId}`)
    await expect(page).toHaveURL(new RegExp(`/stack/logs/${stackId}`), { timeout: 10000 })
    await expect(page.getByText(`Deployment ID: ${stackId}`)).toBeVisible({ timeout: 10000 })

    const detailRes = await request.get(`${apiBase}/stacks/${stackId}`)
    expect(detailRes.ok()).toBeTruthy()
    const detailBody = (await detailRes.json()) as { config?: { access_domain?: string } }
    expect(detailBody.config?.access_domain).toBe(`${stackName}.internal`)

    const terminalState = await pollStackState(request, stackId)
    expect(['completed', 'failed', 'rolled_back']).toContain(terminalState)
  })
})

import { test, expect, type APIRequestContext } from '@playwright/test'
import { loginAs } from './helpers/auth'
import { countTemplates, templateCards, USE_AS_BASE, VIEW_DETAIL } from './helpers/templates'

const apiBase = 'http://localhost:8090/api/v1'

async function getConnectedPipelineClusterId(request: APIRequestContext): Promise<string> {
  const res = await request.get(`${apiBase}/admin/clusters`)
  expect(res.ok()).toBeTruthy()

  const body = (await res.json()) as {
    items?: Array<{ id: string; type?: string; connection_status?: string }>
  }

  const cluster = (body.items ?? []).find(
    (item) => item.type === 'pipeline' && item.connection_status === 'connected',
  )

  expect(cluster).toBeTruthy()
  return cluster!.id
}

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
    // 로그인 뒤 도착지는 역할마다 다르다(role-landing.ts). 그 규칙을 아는 곳은
    // helpers/auth.ts 하나이므로 여기서 다시 적지 않는다 — 예전에 '무조건 /' 로
    // 적어 두었다가 역할별 랜딩이 단일 출처로 모이면서 통째로 어긋났다.
    await loginAs(page, 'devops')
  })

  // 개수를 숫자로 박지 않는다. 예전에는 4 로 박혀 있었는데 템플릿이 9 개로 늘면서
  // 화면은 멀쩡한데 테스트만 틀렸다. 서버가 주는 목록과 화면 카드가 같은지를 본다 —
  // 템플릿이 늘어도 깨지지 않고, 렌더가 빠지면 잡힌다.
  test('Stack Templates 페이지 → 서버 템플릿 수만큼 카드 표시', async ({ page, request }) => {
    const expected = await countTemplates(request, apiBase)

    await page.goto('/stack/templates')
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })
    await expect(templateCards(page)).toHaveCount(expected)
  })

  test('템플릿 상세 → "Use As Base" → Install 페이지 이동', async ({ page }) => {
    await page.goto('/stack/templates')
    await expect(page.locator('h1')).toContainText('Stack Template', { timeout: 10000 })

    // 카드에는 상세 열기 버튼만 있다. 기본 템플릿으로 쓰는 버튼은 상세 모달 안이다.
    await templateCards(page).first().getByRole('button', { name: VIEW_DETAIL }).click()
    await page.getByRole('dialog').getByRole('button', { name: USE_AS_BASE }).click()

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
    await expect(
      page.getByText('Target Cluster 선택이 필요합니다')
        .or(page.getByText('설치 대상 OSS가 없습니다'))
        .or(page.locator('.monaco-editor').first())
    ).toBeVisible({ timeout: 10000 })
  })

  test('Resources 탭에서 입력 필드 확인', async ({ page }) => {
    await page.goto('/stack/install')
    await expect(page.locator('h1')).toContainText('Stack Install', { timeout: 10000 })
    await page.click('button:has-text("Resources")')
    await expect(page.getByText(/Resource Planning/i).first()).toBeVisible({ timeout: 10000 })
    await expect(page.getByText(/Sizing Profile/i).first()).toBeVisible()
  })

  test('Stack List 페이지 렌더링 확인', async ({ page }) => {
    await page.goto('/stack/list')
    await expect(page.locator('h1')).toContainText('Stack List', { timeout: 10000 })
  })

  test('@stack-critical Stack List 기반 배포 상태/로그 페이지 및 템플릿 도메인 검증', async ({ page, request }) => {
    test.setTimeout(240000)

    const clustersRes = await request.get(`${apiBase}/clusters`)
    const clustersBody = (await clustersRes.json()) as { items?: Array<{ id: string }> }
    //const clusterId = await getConnectedPipelineClusterId(request)
    const clusterId = clustersBody.items?.[0]?.id
    if (!clusterId) {
      test.skip(true, 'No clusters available — seed data required')
      return
    }

    const stackName = `pw-list-domain-${Date.now()}`
    const createPayload = {
      name: stackName,
      cluster_id: clusterId,
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
    await expect(page.getByText(stackName).first()).toBeVisible({ timeout: 15000 })

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

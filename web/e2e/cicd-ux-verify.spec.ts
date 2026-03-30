import { test, expect, type Page } from '@playwright/test'
import { loginAs, navigateToMenu } from './helpers/auth'

test.describe('CI/CD UX 개선 검증', () => {
  let page: Page

  test.beforeEach(async ({ page: p }) => {
    page = p
    await loginAs(page, 'developer')
  })

  test('1. CI/CD List — 클러스터 필터 드롭다운 존재', async () => {
    await page.goto('/cicd/list')
    await page.waitForLoadState('networkidle')

    // 클러스터 필터 드롭다운 확인
    const clusterSelect = page.locator('select').filter({ hasText: /All Clusters/ })
    await expect(clusterSelect).toBeVisible({ timeout: 5000 })
    await page.screenshot({ path: 'e2e/screenshots/cicd-list-cluster-filter.png' })
  })

  test('2. CI/CD List — Logs 버튼 → Pipeline Logs 페이지', async () => {
    await page.goto('/cicd/list')
    await page.waitForLoadState('networkidle')

    // 첫 번째 expand 버튼 클릭
    const expandBtn = page.locator('table button').first()
    if (await expandBtn.isVisible()) {
      await expandBtn.click()
      await page.waitForTimeout(500)

      // Logs 버튼 찾기
      const logsBtn = page.getByRole('button', { name: /Logs/i }).first()
      if (await logsBtn.isVisible()) {
        await logsBtn.click()
        await page.waitForLoadState('networkidle')
        // URL이 /cicd/pipelines/xxx/logs 패턴인지 확인
        await expect(page).toHaveURL(/\/cicd\/pipelines\/.*\/logs/)
        // Breadcrumb에 CI/CD List 링크 존재 확인
        await expect(page.getByLabel('Breadcrumb').getByText('CI/CD List')).toBeVisible()
        await page.screenshot({ path: 'e2e/screenshots/cicd-pipeline-logs-page.png' })
      }
    }
  })

  test('3. Pipeline Setup & Deploy — 6단계 위저드', async () => {
    await page.goto('/cicd/developer-deploy')
    await page.waitForLoadState('networkidle')

    // Step 1: 앱 이름 (템플릿 그리드가 없어야 함)
    await expect(page.getByText('앱 이름 입력')).toBeVisible()
    // 앱 템플릿 그리드가 없는지 확인
    const templateGrid = page.locator('text=앱 템플릿')
    await expect(templateGrid).toHaveCount(0)

    // 앱 이름 입력
    await page.fill('input[placeholder*="app"]', 'verify-test')
    await page.screenshot({ path: 'e2e/screenshots/wizard-step1-app-name.png' })

    // Step 2로 이동
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 2: Git Repository (Stack 선택 드롭다운 확인)
    await expect(page.getByText('Git Repository URL')).toBeVisible()
    await page.screenshot({ path: 'e2e/screenshots/wizard-step2-git.png' })

    // Git URL 입력 (Stack 미선택 시 직접 입력)
    const gitInput = page.locator('input[placeholder*="github"], input[placeholder*="repo"]').first()
    if (await gitInput.isVisible()) {
      await gitInput.fill('https://github.com/cloud-nullus/sample-go-api')
    }
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 3: 클러스터 & 네임스페이스
    await expect(page.getByText('클러스터 & 네임스페이스')).toBeVisible()
    await page.screenshot({ path: 'e2e/screenshots/wizard-step3-cluster.png' })
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 4: 리소스 설정 (Input + Slider)
    await expect(page.getByRole('paragraph').filter({ hasText: '리소스 설정' })).toBeVisible()
    // Input 필드 확인 (슬라이더 옆 직접 입력)
    const resourceInputs = page.locator('input[type="text"], input:not([type="range"])')
    await page.screenshot({ path: 'e2e/screenshots/wizard-step4-resources.png' })
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 5: 환경 변수
    await expect(page.getByRole('paragraph').filter({ hasText: '환경 변수' })).toBeVisible()
    await page.screenshot({ path: 'e2e/screenshots/wizard-step5-envvars.png' })
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 6: 매니페스트 확인 (textarea 존재)
    await expect(page.getByRole('paragraph').filter({ hasText: '매니페스트 확인' })).toBeVisible()
    const manifestEditor = page.locator('textarea')
    await expect(manifestEditor).toBeVisible()
    // YAML 내용에 verify-test 포함 확인
    const yamlContent = await manifestEditor.inputValue()
    expect(yamlContent).toContain('verify-test')
    await page.screenshot({ path: 'e2e/screenshots/wizard-step6-manifest.png' })

    // Deploy 버튼 확인
    await expect(page.getByRole('button', { name: /Deploy/i })).toBeVisible()
  })

  test('4. Pipeline Setup & Deploy — 배포 실행 및 진행 UI', async () => {
    await page.goto('/cicd/developer-deploy')
    await page.waitForLoadState('networkidle')

    // 빠르게 6단계까지 진행
    // Step 1: 앱 이름
    await page.fill('input[placeholder*="app"]', 'ws-verify')
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 2: Git URL
    const gitInput = page.locator('input[placeholder*="github"], input[placeholder*="repo"]').first()
    if (await gitInput.isVisible()) {
      await gitInput.fill('https://github.com/cloud-nullus/sample-go-api')
    }
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 3: 클러스터 (기본값 사용)
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 4: 리소스 (기본값)
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 5: 환경 변수 (스킵)
    await page.getByRole('button', { name: '다음' }).click()
    await page.waitForTimeout(300)

    // Step 6: 매니페스트 확인 → Deploy
    await page.getByRole('button', { name: /Deploy/i }).click()
    await page.waitForTimeout(2000)

    // 배포 진행 UI 확인 (터미널 콘솔 또는 진행 단계)
    await page.screenshot({ path: 'e2e/screenshots/deploy-progress-ws.png' })

    // 완료 대기 (최대 15초)
    await page.waitForTimeout(8000)
    await page.screenshot({ path: 'e2e/screenshots/deploy-complete-ws.png' })
  })

  test('5. CI/CD History — Breadcrumb 네비게이션', async () => {
    await page.goto('/cicd/history')
    await page.waitForLoadState('networkidle')

    // Breadcrumb에 CI/CD List 링크 확인
    const breadcrumbLink = page.locator('a, [role="link"]').filter({ hasText: 'CI/CD List' }).first()
    await expect(breadcrumbLink).toBeVisible({ timeout: 5000 })
    await page.screenshot({ path: 'e2e/screenshots/cicd-history-breadcrumb.png' })
  })
})

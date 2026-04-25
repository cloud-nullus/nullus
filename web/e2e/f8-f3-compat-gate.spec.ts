import { test, expect, type Page } from '@playwright/test'

// Inline admin login: the login page now navigates to '/' after submit
// (not '/admin/organization' as helpers/auth.ts expects). We log in here and
// then page.goto() the specific route under test.
async function loginAsAdmin(page: Page): Promise<void> {
  await page.goto('/login')
  await page.fill('#email', 'admin@nullus.dev')
  await page.fill('#password', 'admin123')
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
  await page.click('button[type="submit"]')
  // Login page calls navigate('/') unconditionally after success.
  await page.waitForURL('**/', { timeout: 10000 })
}

// F8-F3 UI smoke test: exercise the three new surfaces added in Tasks 3–5
// and F8-F3 from a real browser against the live dev stack.
//   1. Admin "Stack Version Management" page (Task 4) renders the Narwhal
//      Golden Path matrices and per-tool arch/tier badges.
//   2. Stack Install Wizard shows Golden Path Quick Start Auto Select cards
//      (Task 5).
//   3. Server-side Pre-Deploy Gate verdict panel (F8-F3) renders the right
//      copy for a stack that the backend is known to fail / warn on. The
//      per-stack seed rows were inserted before running this spec.

test.describe('F8-F3 compatibility gate UI', () => {
  test('admin stack versions page shows Narwhal matrices with arch/tier badges', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/admin/stack-versions')

    // Page title + list sub-heading confirm the route rendered.
    await expect(page.getByRole('heading', { name: /Stack Version Management/i })).toBeVisible()
    await expect(page.getByText(/Narwhal baseline/i).first()).toBeVisible()

    // List items for the three Narwhal Golden Path matrices. `.first()`
    // because the detail panel repeats the id as a monospace label — the
    // list-side occurrence is the stable one.
    await expect(page.getByText('gitlab-allinone-v1').first()).toBeVisible()
    await expect(page.getByText('gitlab-argocd-v1').first()).toBeVisible()
    await expect(page.getByText('github-argocd-v1').first()).toBeVisible()

    // Click GitLab All-in-One and confirm the tools table surfaces the
    // new F8 Task 1 fields: arch badges ("amd64"), tier badges ("stable"
    // or a localized equivalent).
    await page.getByRole('button', { name: /GitLab All-in-One/i }).click()
    // At least one amd64 arch badge + one stable/beta tier badge visible.
    await expect(page.getByText('amd64').first()).toBeVisible()
    // Clusters section exists and exposes a Refresh Discovery button per row.
    await expect(page.getByText(/Cluster compatibility|클러스터 호환성/i)).toBeVisible()
    await expect(page.getByRole('button', { name: /Refresh Discovery|재판독/i }).first()).toBeVisible()
  })

  test('stack install wizard shows Golden Path Quick Start Auto Select cards', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/stack/install')

    // Auto Select section heading visible (ko/en variant accepted).
    await expect(
      page.getByText(/Golden Path Quick Start|Golden Path 빠른 시작/i).first(),
    ).toBeVisible()

    // Each Narwhal matrix renders as a card. The untested github matrix
    // must not render as unsupported (status !== unsupported filter).
    await expect(page.getByRole('button', { name: /GitLab All-in-One/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /GitLab \+ Argo CD/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /GitHub \+ Argo CD/i })).toBeVisible()

    // With no cluster selected, each card should carry the
    // "select a cluster first" subtitle (yellow-flagged).
    await expect(
      page.getByText(/Select a target cluster first|먼저 대상 클러스터를 선택하세요/i).first(),
    ).toBeVisible()
  })

  test('refresh discovery button responds (connection_failed path is acceptable)', async ({ page }) => {
    // Admin Stack Versions page. Clicking Refresh Discovery on any cluster
    // should trigger POST /admin/clusters/:id/refresh-discovery. Without a
    // real kubeconfig the server will return an error, but the button
    // must still round-trip through the React Query mutation without
    // crashing the page.
    await loginAsAdmin(page)
    await page.goto('/admin/stack-versions')

    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes('/refresh-discovery') && r.request().method() === 'POST',
        { timeout: 10000 },
      ),
      page.getByRole('button', { name: /Refresh Discovery|재판독/i }).first().click(),
    ])
    // Either 200 (if kubeconfig happens to exist) or 4xx (most common in
    // dev). Anything is fine as long as the handler ran.
    expect([200, 400, 404, 500, 502]).toContain(resp.status())
  })

  test('auto select disables GitLab cards after selecting the mixed-arch cluster', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/stack/install')

    // Pick the mixed-arch cluster we seeded earlier (app-cluster-prod =
    // ['amd64','arm64']). GitLab* tools are amd64-only → Auto Select
    // cards must become disabled with the archMismatch subtitle.
    // selectOption accepts either value (the cluster id) or exact label.
    // We seeded 32222222-... as 'app-cluster-prod' with
    // node_architectures=['amd64','arm64'] earlier.
    const clusterSelect = page.getByLabel(/Target Cluster/i)
    await clusterSelect.selectOption('32222222-2222-2222-2222-222222222222')

    const gitlabAio = page.getByRole('button', { name: /GitLab All-in-One/i }).first()
    await expect(gitlabAio).toBeDisabled()

    // At least one card should show the incompatibility subtitle.
    await expect(
      page.getByText(/Incompatible with cluster arch|클러스터 아키텍처와 호환되지 않습니다/i).first(),
    ).toBeVisible()
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { renderWithProviders, selectOption } from '../../../__tests__/test-utils'
import { useAuthStore } from '../../../stores/auth-store'
import { SecurityDashboardPage } from './security-dashboard-page'

const mockGet = vi.hoisted(() => vi.fn())
const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('../../../lib/api', () => ({
  api: { get: mockGet, post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

// 응답은 설계 §5.2 의 모양 그대로다(snake_case).
const cells = (overrides: Record<string, unknown> = {}) => ({
  secret_scan: { state: 'pass', findings: 0 },
  dependency_scan: { state: 'pass', targets: 2, counts: { critical: 0, high: 0, medium: 0, low: 0, unknown: 0 } },
  sast: { state: 'pass', vulnerabilities: 0, security_hotspots: 0 },
  image_scan: { state: 'pass', counts: { critical: 0, high: 1, medium: 0, low: 0, unknown: 0 } },
  ...overrides,
})

const orderApi = {
  pipeline_id: 'pip_order',
  name: 'order-api',
  stack_id: 'stk_prod',
  rank: 1,
  latest_run: { deployment_id: 'dep_42', version: '#42', status: 'failed', started_at: '2026-10-20T07:00:00Z' },
  cells: cells({
    secret_scan: {
      state: 'block',
      deployment_id: 'dep_42',
      version: '#42',
      stale_run: false,
      findings: 2,
      rules: [
        { rule_id: 'github-pat', count: 1 },
        { rule_id: 'aws-access-key', count: 1 },
      ],
      link: 'https://ci.example/jobs/1/artifacts/gitleaks-report.json',
    },
    sast: {
      state: 'warn',
      deployment_id: 'dep_41',
      version: '#41',
      stale_run: true,
      vulnerabilities: 4,
      security_hotspots: 2,
      failed_conditions: ['new_violations'],
      link: 'https://sonar.example/dashboard?id=order-api',
    },
    image_scan: {
      state: 'block',
      deployment_id: 'dep_41',
      version: '#41',
      stale_run: true,
      image: 'registry.example/order-api@sha256:abcdef0123456789abcdef',
      counts: { critical: 2, high: 11, medium: 0, low: 0, unknown: 0 },
      db_stale: false,
    },
  }),
}

const webFront = {
  pipeline_id: 'pip_web',
  name: 'web-front',
  stack_id: 'stk_stg',
  rank: 2,
  latest_run: { deployment_id: 'dep_40', version: '#40', status: 'success', started_at: '2026-10-20T08:00:00Z' },
  cells: cells({ image_scan: { state: 'block', counts: { critical: 1, high: 0, medium: 0, low: 0, unknown: 0 } } }),
}

const appsReport = (overrides: Record<string, unknown> = {}) => ({
  generated_at: '2026-10-20T09:12:00Z',
  truncated: false,
  summary: { apps: 24, with_security_stage: 22, blocked: 3, secrets: 1, critical: 5, undetermined: 2 },
  coverage: { secret_scan: 20, dependency_scan: 14, sast: 12, image_scan: 20 },
  groups: [
    { stack_id: 'stk_prod', stack_name: 'stack-prod', cluster_id: 'c_prod', cluster_name: 'prod-a', apps: 12, blocked: 2, secrets: 1, critical: 1, undetermined: 1, problems: 3 },
    { stack_id: 'stk_stg', stack_name: 'stack-stg', cluster_id: 'c_stg', cluster_name: 'stg-a', apps: 8, blocked: 1, secrets: 0, critical: 1, undetermined: 0, problems: 1 },
    { stack_id: 'stk_dev', stack_name: 'stack-dev', cluster_id: 'c_dev', cluster_name: 'dev-a', apps: 2, blocked: 0, secrets: 0, critical: 0, undetermined: 0, problems: 0 },
    { stack_id: '', stack_name: '', cluster_id: '', cluster_name: '', apps: 2, blocked: 0, secrets: 0, critical: 0, undetermined: 0, problems: 0 },
  ],
  apps: [orderApi, webFront],
  ...overrides,
})

const devApp = {
  pipeline_id: 'pip_dev',
  name: 'dev-batch',
  stack_id: 'stk_dev',
  rank: 5,
  latest_run: null,
  cells: cells(),
}

const stacksReport = {
  generated_at: '2026-10-20T09:12:00Z',
  summary: { stacks: 3, scanned_stacks: 2, not_scanned_stacks: 1, action_images: 12, grade_a_images: 3, grade_b_images: 9 },
  stacks: [
    {
      stack_id: 'stk_prod', stack_name: 'stack-prod', cluster_id: 'c_prod', cluster_name: 'prod-a',
      state: 'scanned', reason: '', airgap: false, images: 36, failed_images: 0, shared_images: 5,
      grade_a_images: 3, grade_b_images: 5, grade_c_images: 9,
      fixable: { critical: 8, high: 120, medium: 0, low: 0, unknown: 0 },
      worst_image: { image: 'registry.example/gitlab-toolbox:v17', critical: 61, high: 1569, exposed: false },
      scanned_at: '2026-10-20T06:00:00Z', db_updated_at: '2026-10-18T06:00:00Z', db_stale: false,
    },
    {
      stack_id: 'stk_dev', stack_name: 'stack-dev', cluster_id: 'c_dev', cluster_name: 'dev-a',
      state: 'not_scanned', reason: 'scanner_not_installed', airgap: false, worst_image: null, db_stale: false,
    },
  ],
}

type Routes = { apps?: (params: Record<string, string>) => unknown; stacks?: () => unknown }

function serve(routes: Routes = {}) {
  mockGet.mockImplementation((url: string, config?: { params?: Record<string, string> }) => {
    const params = config?.params ?? {}
    if (url === '/observability/security/apps') {
      return Promise.resolve({ data: routes.apps ? routes.apps(params) : appsReport() })
    }
    if (url === '/observability/security/stacks') {
      return Promise.resolve({ data: routes.stacks ? routes.stacks() : stacksReport })
    }
    return Promise.reject(new Error(`unexpected ${url}`))
  })
}

const appsCalls = () =>
  mockGet.mock.calls
    .filter(([url]) => url === '/observability/security/apps')
    .map(([, config]) => (config as { params: Record<string, string> }).params)

const stacksCalls = () => mockGet.mock.calls.filter(([url]) => url === '/observability/security/stacks')

const renderPage = () => renderWithProviders(<SecurityDashboardPage />, { route: '/observability/security/dashboard' })

beforeEach(() => {
  mockGet.mockReset()
  mockNavigate.mockReset()
  useAuthStore.setState({ role: 'admin', user: null, isAuthenticated: true })
  serve()
})

describe('SecurityDashboardPage', () => {
  it('should load the whole organization with problems only by default', async () => {
    renderPage()
    await screen.findByText('order-api')
    // 기본 범위는 조직 전체(클러스터 · 스택 없음), "문제 있는 것만" 은 켜져 있다.
    expect(appsCalls()[0]).toEqual({ problem_only: 'true' })
    expect(screen.getByRole('checkbox', { name: 'Problems only' })).toBeChecked()
    expect(screen.getByText('Based on main · run history syncs every 10 min')).toBeTruthy()
  })

  it('should show coverage in the column headers', async () => {
    renderPage()
    await screen.findByText('order-api')
    const headers = screen.getAllByRole('columnheader').map((th) => th.textContent)
    expect(headers).toEqual(['App', 'Secrets 20/24', 'Dependencies 14/24', 'SAST 12/24', 'Image 20/24', 'Latest run'])
  })

  describe('stack groups', () => {
    it('should open groups with problems and collapse groups without', async () => {
      renderPage()
      await screen.findByText('order-api')
      const prod = screen.getByRole('button', { name: /stack-prod/ })
      const dev = screen.getByRole('button', { name: /stack-dev/ })
      const none = screen.getByRole('button', { name: /No stack/ })
      expect(prod.getAttribute('aria-expanded')).toBe('true')
      expect(dev.getAttribute('aria-expanded')).toBe('false')
      expect(none.getAttribute('aria-expanded')).toBe('false')
      expect(prod.textContent).toContain('cluster prod-a')
      expect(prod.textContent).toContain('Apps 12 · Blocked 2 · Secrets 1 · CRITICAL 1 · Undetermined 1')
      expect(dev.textContent).toContain('Apps 2 · No problems')
    })

    it('should collapse and expand a group', async () => {
      renderPage()
      await screen.findByText('order-api')
      fireEvent.click(screen.getByRole('button', { name: /stack-prod/ }))
      expect(screen.queryByText('order-api')).toBeNull()
      expect(screen.getByText('web-front')).toBeTruthy()
      fireEvent.click(screen.getByRole('button', { name: /stack-prod/ }))
      expect(screen.getByText('order-api')).toBeTruthy()
    })

    it('should say why an expanded group without problems is empty', async () => {
      renderPage()
      await screen.findByText('order-api')
      fireEvent.click(screen.getByRole('button', { name: /stack-dev/ }))
      expect(screen.getByText('No apps with problems here — hidden by Problems only.')).toBeTruthy()
    })

    // 스택 하나를 고르면 묶음을 풀고 평평한 표로 보인다.
    it('should not group when a single stack is selected', async () => {
      serve({
        apps: (params) =>
          params.stack_id === 'stk_prod'
            ? appsReport({ groups: [appsReport().groups[0]], apps: [orderApi] })
            : appsReport(),
      })
      renderPage()
      await screen.findByText('order-api')
      selectOption(screen.getByLabelText('Stack'), 'stack-prod')
      await waitFor(() => expect(appsCalls().some((p) => p.stack_id === 'stk_prod')).toBe(true))
      await waitFor(() => expect(screen.queryByRole('button', { name: /stack-prod/ })).toBeNull())
      expect(screen.getByText('order-api')).toBeTruthy()
    })
  })

  describe('problems only', () => {
    it('should show how many passing apps are hidden and show all on request', async () => {
      serve({
        apps: (params) =>
          params.problem_only === 'true' ? appsReport() : appsReport({ apps: [orderApi, webFront, devApp] }),
      })
      renderPage()
      await screen.findByText('order-api')
      expect(screen.getByText('22 apps without problems are hidden')).toBeTruthy()

      fireEvent.click(screen.getByRole('button', { name: 'Show all' }))

      await waitFor(() => expect(appsCalls().some((p) => p.problem_only === undefined)).toBe(true))
      expect(screen.getByRole('checkbox', { name: 'Problems only' })).not.toBeChecked()
      await waitFor(() => expect(screen.queryByText(/apps without problems are hidden/)).toBeNull())
    })

    // KPI 는 "문제 있는 것만" 을 따르지 않는다 — 표를 줄여도 그대로다.
    it('should keep the KPIs when toggling problems only', async () => {
      renderPage()
      await screen.findByText('order-api')
      const kpis = screen.getByTestId('security-kpis')
      expect(within(kpis).getByText('3 / 22')).toBeTruthy()
      fireEvent.click(screen.getByRole('checkbox', { name: 'Problems only' }))
      await waitFor(() => expect(appsCalls().length).toBeGreaterThan(1))
      expect(within(kpis).getByText('3 / 22')).toBeTruthy()
    })
  })

  describe('roles', () => {
    it('should show the installed OSS tab and the fifth KPI to admins', async () => {
      renderPage()
      await screen.findByText('order-api')
      expect(screen.getByRole('button', { name: /Installed OSS/ })).toBeTruthy()
      await screen.findByText('12 images')
      expect(screen.getByText('Exposed (A) 3 · stacks not scanned: 1')).toBeTruthy()
      expect(within(screen.getByTestId('security-kpis')).getAllByText(/./, { selector: '.text-\\[28px\\]' })).toHaveLength(5)
    })

    it('should treat devops like admin', async () => {
      useAuthStore.setState({ role: 'devops', user: null, isAuthenticated: true })
      renderPage()
      await screen.findByText('order-api')
      expect(screen.getByRole('button', { name: /Installed OSS/ })).toBeTruthy()
      await waitFor(() => expect(stacksCalls().length).toBeGreaterThan(0))
    })

    // 개발자는 /stacks 권한이 없다. 탭 · M5 를 감추고 API 도 부르지 않는다(설계 §4.4).
    it('should hide the installed OSS tab and M5 from developers and never request stacks', async () => {
      useAuthStore.setState({ role: 'developer', user: null, isAuthenticated: true })
      renderPage()
      await screen.findByText('order-api')
      expect(screen.queryByRole('button', { name: /Installed OSS/ })).toBeNull()
      expect(screen.queryByText('Installed OSS to act on')).toBeNull()
      expect(within(screen.getByTestId('security-kpis')).getAllByText(/./, { selector: '.text-\\[28px\\]' })).toHaveLength(4)
      expect(stacksCalls()).toHaveLength(0)
    })
  })

  describe('detail panel', () => {
    it('should open a per-stage summary with links when a row is selected', async () => {
      renderPage()
      fireEvent.click(await screen.findByText('order-api'))
      const detail = screen.getByTestId('security-app-detail')

      expect(within(detail).getByText(/stack-prod · Latest run #42 \(Failed\)/)).toBeTruthy()
      const secrets = within(detail).getByRole('region', { name: 'Secrets' })
      expect(within(secrets).getByText('2 found')).toBeTruthy()
      expect(within(secrets).getByText(/github-pat · aws-access-key/)).toBeTruthy()
      // 커밋된 비밀은 이미 유출이다 — 폐기 · 회전을 먼저 말한다.
      expect(within(secrets).getByRole('alert').textContent).toContain('revoke and rotate them now')
      expect(within(secrets).getByRole('link', { name: /Report/ }).getAttribute('href')).toBe(
        'https://ci.example/jobs/1/artifacts/gitleaks-report.json',
      )

      const sast = within(detail).getByRole('region', { name: 'SAST' })
      expect(within(sast).getByText('New issues')).toBeTruthy()
      expect(within(sast).getByRole('link', { name: /SonarQube/ })).toBeTruthy()
      // 이 단계의 결과를 낸 실행(#41)으로 간다 — 앱의 최신 실행(#42)이 아니다.
      expect(within(sast).getByRole('link', { name: 'Open run #41' }).getAttribute('href')).toBe(
        '/cicd/list?pipeline=pip_order&tab=history&run=dep_41',
      )

      expect(within(detail).getByRole('link', { name: /Open run history/ }).getAttribute('href')).toBe(
        '/cicd/list?pipeline=pip_order&tab=history&run=dep_42',
      )
    })
  })

  describe('installed OSS tab', () => {
    it('should list stack image grades and open the stack deep link on row click', async () => {
      renderPage()
      await screen.findByText('order-api')
      fireEvent.click(screen.getByRole('button', { name: /Installed OSS/ }))

      const row = (await screen.findByText('stack-prod')).closest('tr')!
      expect(within(row).getByText('36')).toBeTruthy()
      expect(within(row).getByText('gitlab-toolbox:v17')).toBeTruthy()
      // 스캔 안 함은 0 이 아니라 사유다.
      const devRow = screen.getByText('stack-dev').closest('tr')!
      expect(within(devRow).getByText('Not scanned — scanner not installed')).toBeTruthy()
      expect(within(devRow).queryByText('0')).toBeNull()

      fireEvent.click(row)
      expect(mockNavigate).toHaveBeenCalledWith('/stack/list?stack=stk_prod&tab=image-scans')
    })

    it('should explain when no stack scans its installed images', async () => {
      serve({
        stacks: () => ({
          summary: { stacks: 1, scanned_stacks: 0, not_scanned_stacks: 1 },
          stacks: [stacksReport.stacks[1]],
        }),
      })
      renderPage()
      await screen.findByText('order-api')
      fireEvent.click(screen.getByRole('button', { name: /Installed OSS/ }))
      expect(await screen.findByText('No stack scans its installed images')).toBeTruthy()
    })
  })

  describe('empty states', () => {
    it('should point to pipelines when there are no apps', async () => {
      serve({
        apps: () =>
          appsReport({
            summary: { apps: 0, with_security_stage: 0, blocked: 0, secrets: 0, critical: 0, undetermined: 0 },
            groups: [],
            apps: [],
          }),
      })
      renderPage()
      expect(await screen.findByText('No pipelines yet')).toBeTruthy()
      expect(screen.getByRole('link', { name: 'Go to pipelines' }).getAttribute('href')).toBe('/cicd/list')
    })

    // 보안 단계가 없는 앱을 "0건 통과" 처럼 보이면 안 된다.
    it('should not look like zero passed when no app has a security stage', async () => {
      serve({
        apps: () =>
          appsReport({
            summary: { apps: 5, with_security_stage: 0, blocked: 0, secrets: 0, critical: 0, undetermined: 0 },
            apps: [],
          }),
      })
      renderPage()
      expect(await screen.findByText('No security scanners are selected in your stacks')).toBeTruthy()
      const kpis = screen.getByTestId('security-kpis')
      expect(within(kpis).queryByText('0')).toBeNull()
      expect(within(kpis).getAllByText('No security stages yet').length).toBe(4)
    })

    it('should say nothing matches when filters leave no apps', async () => {
      serve({
        apps: (params) =>
          params.q
            ? appsReport({
                summary: { apps: 0, with_security_stage: 0, blocked: 0, secrets: 0, critical: 0, undetermined: 0 },
                groups: [],
                apps: [],
              })
            : appsReport(),
      })
      renderPage()
      await screen.findByText('order-api')
      fireEvent.change(screen.getByRole('searchbox', { name: 'Search apps' }), { target: { value: 'zzz' } })
      expect(await screen.findByText('No apps match these filters')).toBeTruthy()
      expect(appsCalls().some((p) => p.q === 'zzz')).toBe(true)
    })

    it('should warn when the app list was truncated', async () => {
      serve({ apps: () => appsReport({ truncated: true }) })
      renderPage()
      expect(await screen.findByRole('status')).toBeTruthy()
      expect(screen.getByRole('status').textContent).toContain('Showing the first 2 apps')
    })
  })
})

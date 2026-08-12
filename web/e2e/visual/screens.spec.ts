// 시각 회귀 베이스라인 — UI 전면 개편의 "정보 유실 0" 안전망 3번.
//
// 화면 28개 × {dark, light} 를 스냅샷으로 고정한다. 개편 중에는 스냅샷이 "달라지는" 것이
// 정상이고, 검사 대상은 레이아웃 붕괴 · 섹션 누락 · 텍스트 잘림이다.
//
// 로그인은 프론트엔드 목 인증(login-page.tsx)이라 백엔드 없이도 돌아간다.
// API(8090)가 없으면 데이터 영역은 빈 상태/에러 상태로 렌더되는데, 그 상태의 룩도
// 개편 대상이므로 그대로 스냅샷에 담는다.
//
// 회귀 검사:   npm run e2e:visual
// 베이스라인:  npm run e2e:visual:update      (= --update-snapshots=all)
//
// ⚠️ `--update-snapshots` 만 쓰면 안 된다. 기본 모드가 `changed` 라서 아래
// maxDiffPixelRatio 허용치 안에 든 변경은 "통과"로 판정돼 스냅샷이 갱신되지 않는다.
// 토큰 색을 바꿨는데 스냅샷이 그대로인 상황이 실제로 나왔다 — 반드시 `=all` 을 쓴다.
//
// 이 허용치 때문에 미묘한 색 회귀는 여기서 못 잡는다. 색 대비는
// src/__tests__/contrast-audit.test.ts 가 정확히 검사하고, 이 스펙은
// 레이아웃 붕괴·섹션 누락·텍스트 잘림을 담당한다. 역할이 나뉘어 있다.
//
// 기획안: docs/40_UI_UX/Nullus_UIUX_전면개편_기획안.md §8 (안전망 3)

import { test, expect, type Page } from '@playwright/test'

type Theme = 'dark' | 'light'

/** 라우트 → 스냅샷 이름. 파라미터 라우트는 대표 값을 박아 렌더 가능한 형태로 만든다. */
const SCREENS: { name: string; path: string }[] = [
  { name: 'home', path: '/' },
  { name: 'stack-templates', path: '/stack/templates' },
  { name: 'stack-list', path: '/stack/list' },
  { name: 'stack-install', path: '/stack/install' },
  { name: 'stack-add-tools', path: '/stack/1/add-tools' },
  { name: 'stack-deploy', path: '/stack/deploy/1' },
  { name: 'stack-deploy-logs', path: '/stack/logs/1' },
  { name: 'stack-retry-history', path: '/stack/deployments/1/retry-history' },
  { name: 'stack-history', path: '/stack/history' },
  { name: 'stack-versions', path: '/stack/versions' },
  { name: 'stack-oss-resource-default', path: '/stack/oss-resource-default' },
  { name: 'cicd-developer-deploy', path: '/cicd/developer-deploy' },
  { name: 'cicd-templates', path: '/cicd/templates' },
  { name: 'cicd-create', path: '/cicd/create' },
  { name: 'cicd-golden-paths', path: '/cicd/golden-paths' },
  { name: 'cicd-list', path: '/cicd/list' },
  { name: 'cicd-history', path: '/cicd/history' },
  { name: 'cicd-pipeline-logs', path: '/cicd/pipelines/1/logs' },
  { name: 'observability-monitoring', path: '/observability/monitoring' },
  { name: 'observability-alert-rules', path: '/observability/alert-rules' },
  { name: 'observability-alert-history', path: '/observability/alert-history' },
  { name: 'admin-organization', path: '/admin/organization' },
  { name: 'admin-users', path: '/admin/users' },
  { name: 'admin-clusters', path: '/admin/clusters' },
  { name: 'admin-known-issues', path: '/admin/known-issues' },
  { name: 'admin-token-management', path: '/admin/token-management' },
  { name: 'admin-stack-versions', path: '/admin/stack-versions' },
  { name: 'not-found', path: '/this-route-does-not-exist' },
]

const ADMIN = { email: 'admin@nullus.dev', password: 'admin123' }

async function setTheme(page: Page, theme: Theme): Promise<void> {
  await page.addInitScript((value) => {
    window.localStorage.setItem('nullus-theme', value)
  }, theme)
}

async function login(page: Page): Promise<void> {
  await page.goto('/login')
  await page.fill('#email', ADMIN.email)
  await page.fill('#password', ADMIN.password)
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 5000 })
  await page.click('button[type="submit"]')
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 10000 })
}

/**
 * API 를 빈 응답으로 스텁한다.
 *
 * 백엔드 없이도 각 화면이 **빈 상태(empty state)** 까지 렌더되게 만드는 것이 목적이다.
 * 개편 대상은 빈 상태의 룩도 포함하므로 이게 오히려 결정적인 스냅샷이다.
 * 실데이터 스냅샷은 백엔드가 있는 환경에서 별도로 뜬다.
 */
async function stubApi(page: Page): Promise<void> {
  // 경로를 정확히 맞춘다. `**\/api\/**` 같은 느슨한 글롭은 Vite 가 서빙하는 소스 모듈
  // (`/src/features/admin/api/cluster-api.ts`)까지 가로채 JS 대신 JSON 을 돌려주고,
  // 그러면 lazy 라우트가 "Failed to fetch dynamically imported module" 로 깨진다.
  await page.route(
    (url) => url.pathname.startsWith('/api/v1/'),
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], total: 0, data: [] }),
      })
    },
  )
}

/** lazy 라우트의 Suspense 폴백이 사라질 때까지 기다린다. */
async function waitForPageContent(page: Page): Promise<void> {
  await page.locator('main').waitFor({ state: 'visible', timeout: 10000 })
  await expect(page.locator('main').getByText('Loading...', { exact: true })).toHaveCount(0, {
    timeout: 15000,
  })
}

/** 스냅샷이 흔들리지 않게 애니메이션·캐럿·시간 표시를 죽인다. */
async function stabilize(page: Page): Promise<void> {
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation-duration: 0s !important;
        animation-delay: 0s !important;
        transition-duration: 0s !important;
        transition-delay: 0s !important;
        caret-color: transparent !important;
      }
    `,
  })
}

for (const theme of ['dark', 'light'] as Theme[]) {
  test.describe(`시각 회귀 — ${theme} 테마`, () => {
    test.beforeEach(async ({ page }) => {
      // 시계를 고정한다. 화면 일부가 현재 시각으로 기본값을 만든다 —
      // stack-install 의 스택 이름이 `nullus-...-YYYYMMDD-HHmm` 이라 분이 바뀌면
      // 문자열 길이가 달라지고 그 줄이 접히면서 페이지 전체가 몇 px 밀린다.
      // 실제로 자정 넘어 재실행했을 때 이 화면 하나가 통째로 diff 로 떴다.
      await page.clock.setFixedTime(new Date('2026-01-02T03:04:05Z'))
      await setTheme(page, theme)
      await stubApi(page)
    })

    test(`login (${theme})`, async ({ page }) => {
      await page.goto('/login')
      await stabilize(page)
      await expect(page).toHaveScreenshot(`login-${theme}.png`, { fullPage: true })
    })

    for (const screen of SCREENS) {
      test(`${screen.name} (${theme})`, async ({ page }) => {
        await login(page)
        await page.goto(screen.path)
        // 데이터가 없어도 페이지 헤더와 빈 상태는 떠야 한다.
        await waitForPageContent(page)
        await stabilize(page)
        await expect(page).toHaveScreenshot(`${screen.name}-${theme}.png`, {
          fullPage: true,
          // 실시간 로그·차트·상대시간은 픽셀이 흔들린다. 레이아웃 붕괴를 잡는 게 목적이므로 여유를 준다.
          maxDiffPixelRatio: 0.02,
        })
      })
    }
  })
}

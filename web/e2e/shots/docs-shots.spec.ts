// 문서 사이트용 화면 캡처.
//
// 산출물: website/docs/assets/screens/<이름>-{dark,light}.png
// 실행:   npm run shots:docs
//
// 검증이 아니라 그림을 만드는 스펙이다. 실패해도 CI 를 막지 않게 기본 설정과
// 분리된 playwright.shots.config.ts 로 돈다.
//
// 데이터는 fixtures.ts 의 고정 응답이다. 살아 있는 환경을 찍으면 IP · 토큰 ·
// 타임스탬프가 매번 달라져 문서 그림으로 쓸 수 없고, 백엔드 없이 찍으면 목록이
// 전부 빈 상태가 된다.
//
// 두 테마를 다 찍는 이유: 문서 사이트가 OS 설정에 따라 밝은/어두운 화면을
// 쓴다. MkDocs Material 의 `#only-light` · `#only-dark` 로 짝을 지어 쓴다.

import { test, expect, type Page } from '@playwright/test'
import * as fx from './fixtures'

type Theme = 'dark' | 'light'

const OUT = '../website/docs/assets/screens'
const ADMIN = { email: 'admin@nullus.dev', password: 'admin123' }
const DEVELOPER = { email: 'developer@nullus.dev', password: 'developer123' }

/** 찍을 화면. `as` 는 로그인 계정 — 역할에 따라 사이드바와 랜딩이 달라진다. */
const SCREENS: {
  name: string
  path: string
  as?: typeof DEVELOPER
  /** 화면에 보여야 할 글자. 입력칸의 값이면 `value:` 를 앞에 붙인다. */
  wait?: string
  /** 찍기 전에 화면을 만져야 할 때. 빈 폼보다 채워진 쪽이 문서 그림으로 낫다. */
  prepare?: (page: Page) => Promise<void>
}[] = [
  { name: 'home', path: '/' },
  { name: 'stack-templates', path: '/stack/templates', wait: 'GitLab + Argo CD + SonarQube' },
  { name: 'stack-install', path: '/stack/install' },
  { name: 'stack-list', path: '/stack/list', wait: 'nullus-gitlab-argocd-sonarqube' },
  { name: 'stack-history', path: '/stack/history' },
  { name: 'stack-versions', path: '/stack/version' },
  { name: 'stack-oss-defaults', path: '/stack/oss-resource-default' },
  { name: 'cicd-templates', path: '/cicd/templates', wait: 'React 프론트엔드' },
  { name: 'cicd-golden-paths', path: '/cicd/golden-paths', wait: 'GitLab All-in-One' },
  { name: 'cicd-list', path: '/cicd/list', wait: 'web-frontend' },
  { name: 'cicd-history', path: '/cicd/history' },
  {
    name: 'app-deploy',
    path: '/cicd/developer-deploy',
    as: DEVELOPER,
    // 스택을 골라야 저장소·레지스트리 주소가 채워진다. 빈 폼보다 그 상태가 낫다.
    prepare: async (page) => {
      // MUI Select 는 네이티브 <select> 가 아니라 role=combobox 인 div 다.
      // 열어서 항목을 눌러야 선택 상태가 실제로 바뀐다.
      await page.getByRole('combobox', { name: 'Stack 선택' }).click()
      await page.getByRole('option', { name: 'nullus-gitlab-argocd-sonarqube' }).click()
      // 누르면 포커스 테두리가 남는다. 그림에 파란 테가 끼지 않게 뗀다.
      await page.locator('body').click({ position: { x: 5, y: 5 } })
      // 드롭다운이 사라지는 중에 찍으면 잔상이 남는다. MUI 는 목록(listbox)보다
      // 바깥 래퍼(Modal/Popover)가 늦게 사라진다 — 그쪽까지 기다린다.
      await page.getByRole('listbox').waitFor({ state: 'detached', timeout: 5000 })
      await page
        .locator('.MuiPopover-root, .MuiModal-root')
        .first()
        .waitFor({ state: 'detached', timeout: 5000 })
        .catch(() => undefined)
    },
    // 주소가 채워진 것까지 본다 — 연동 픽스처의 component_type 이 서버와 어긋나면
    // 화면은 뜨지만 주소 앞단이 빈 채로 찍힌다.
    wait: 'value:gitlab.nullus.local',
  },
  { name: 'monitoring', path: '/observability/monitoring' },
  { name: 'alert-history', path: '/observability/alert-history', wait: 'KubePodCrashLooping' },
  { name: 'admin-organization', path: '/admin/organization' },
  { name: 'admin-users', path: '/admin/users', wait: 'devops@nullus.dev' },
  { name: 'admin-clusters', path: '/admin/clusters', wait: 'kind-nullus-platform' },
  { name: 'admin-notifications', path: '/admin/notifications', wait: '플랫폼 운영 Slack' },
  { name: 'admin-token-management', path: '/admin/token-management' },
  { name: 'admin-known-issues', path: '/admin/known-issues' },
]

/** pathname → 고정 응답. 목록은 `{items,total}`, 단건은 리소스 그대로다(서버와 같은 모양). */
function fixtureFor(pathname: string): unknown | undefined {
  const list = (items: unknown[]) => ({ items, total: items.length })

  // 긴 경로를 먼저 본다 — `/stacks` 가 `/stacks/templates` 를 가로채지 않게.
  if (pathname.endsWith('/integrations')) return fx.STACK_INTEGRATIONS
  if (pathname.endsWith('/monitoring')) return fx.STACK_MONITORING
  if (pathname.endsWith('/monitoring-summary')) return fx.CLUSTER_MONITORING
  if (pathname.includes('/history')) return list(fx.STACK_HISTORY)
  if (pathname.includes('/token-sources')) return list(fx.TOKEN_SOURCES)

  // 서버가 배열을 그대로 돌려주는 엔드포인트들. {items} 로 싸면 화면이 빈 목록으로 읽는다.
  if (pathname.endsWith('/stacks/templates')) return fx.STACK_TEMPLATES
  if (pathname.endsWith('/cicd/templates')) return fx.CICD_TEMPLATES
  if (pathname.endsWith('/cicd/app-templates')) return fx.APP_TEMPLATES
  if (pathname.endsWith('/stacks/compatibility')) return list(fx.COMPATIBILITY)
  if (pathname.endsWith('/stacks/resource-defaults')) return list(fx.RESOURCE_DEFAULTS)
  if (pathname.endsWith('/stacks')) return list(fx.STACKS)

  if (pathname.endsWith('/cicd/golden-paths')) return fx.GOLDEN_PATHS
  if (pathname.endsWith('/cicd/pipelines')) return list(fx.PIPELINES)
  if (pathname.endsWith('/cicd/deployments')) return list(fx.DEPLOYMENTS)

  if (pathname.endsWith('/admin/clusters')) return list(fx.CLUSTERS)
  if (pathname.includes('/namespaces')) return { items: fx.NAMESPACES, total: fx.NAMESPACES.length }
  if (pathname.endsWith('/members')) return list(fx.MEMBERS)
  if (pathname.endsWith('/admin/organization')) return fx.ORGANIZATION
  if (pathname.endsWith('/admin/known-issues')) return list(fx.KNOWN_ISSUES)
  if (pathname.endsWith('/notifications/channels')) return list(fx.NOTIFICATION_CHANNELS)
  if (pathname.endsWith('/notifications/events')) return list(fx.NOTIFICATION_EVENTS)
  if (pathname.endsWith('/notifications/history')) return list(fx.NOTIFICATION_HISTORY)
  if (pathname.endsWith('/stack-alerts')) return list(fx.STACK_ALERTS)
  if (pathname.endsWith('/alert-history')) return list(fx.STACK_ALERTS)

  return undefined
}

/**
 * API 를 고정 응답으로 바꾼다.
 *
 * 경로를 `/api/v1/` 로 정확히 거른다. `**\/api\/**` 같은 느슨한 글롭은 Vite 가
 * 서빙하는 소스 모듈까지 가로채 JS 대신 JSON 을 돌려주고, lazy 라우트가
 * "Failed to fetch dynamically imported module" 로 깨진다.
 */
async function mockApi(page: Page): Promise<void> {
  await page.route(
    (url) => url.pathname.startsWith('/api/v1/'),
    async (route) => {
      const { pathname } = new URL(route.request().url())

      if (pathname.endsWith('/auth/login')) {
        return route.fulfill({ json: { token: 'docs-shot', user: null } })
      }

      const body = fixtureFor(pathname)
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        // 모르는 경로는 빈 목록이다. 화면이 깨지는 것보다 비어 보이는 편이 낫다.
        body: JSON.stringify(body ?? { items: [], total: 0, data: [] }),
      })
    },
  )
}

async function setTheme(page: Page, theme: Theme): Promise<void> {
  await page.addInitScript((value) => {
    window.localStorage.setItem('nullus-theme', value)
    // 문서가 한국어라 화면도 한국어로 찍는다. 기본값은 en 이다.
    window.localStorage.setItem('nullus-locale', 'ko')
    // 첫 방문 투어·온보딩 패널이 화면을 덮는다. 본 것으로 표시해 둔다.
    window.localStorage.setItem('nullus-tour-done', 'true')
    window.localStorage.setItem('nullus-onboarding-dismissed', 'true')
    // 모니터링의 "스택 컴포넌트 연결" 패널이 화면 절반을 차지한다 — 건너뛴 것으로
    // 둬서 지표와 차트가 보이게 한다. 키는 뷰·대상별이다(monitoring-utils.ts).
    for (const view of ['cluster', 'stack', 'cicd']) {
      for (const scope of ['none', 'stk_7a31c0', 'stk_41b9de', 'cls_platform_0001', 'cls_develop_0001']) {
        window.localStorage.setItem(`nullus_skip_connect_${view}_${scope}_v2`, 'true')
      }
    }
  }, theme)
}

async function login(page: Page, account: typeof ADMIN): Promise<void> {
  await page.goto('/login')
  await page.fill('#email', account.email)
  await page.fill('#password', account.password)
  await page.waitForSelector('button[type="submit"]:not([disabled])', { timeout: 10000 })
  await page.click('button[type="submit"]')
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 15000 })
}

/** lazy 라우트의 Suspense 폴백이 사라질 때까지 기다린다. */
async function waitForPageContent(page: Page): Promise<void> {
  await page.locator('main').waitFor({ state: 'visible', timeout: 15000 })
  await expect(page.locator('main').getByText('Loading...', { exact: true })).toHaveCount(0, {
    timeout: 20000,
  })
}

/** 애니메이션·캐럿을 죽인다. 전환 중에 찍으면 반투명하게 나온다. */
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

test.describe.configure({ mode: 'parallel' })

for (const theme of ['dark', 'light'] as Theme[]) {
  test.describe(`문서 캡처 — ${theme}`, () => {
    test.beforeEach(async ({ page }) => {
      // 시계를 고정한다. 화면 일부가 현재 시각으로 기본값을 만들어(설치 화면의
      // 스택 이름) 다시 찍을 때마다 그림이 달라진다.
      await page.clock.setFixedTime(new Date('2026-10-12T09:30:00Z'))
      await page.setViewportSize({ width: 1440, height: 900 })
      await setTheme(page, theme)
      await mockApi(page)
    })

    test(`login (${theme})`, async ({ page }) => {
      await page.goto('/login')
      // 로그인 화면에는 <main> 이 없다 — 폼이 뜨는 것으로 기다린다.
      await page.waitForSelector('button[type="submit"]', { timeout: 15000 })
      await stabilize(page)
      await page.screenshot({ path: `${OUT}/login-${theme}.png` })
    })

    for (const screen of SCREENS) {
      test(`${screen.name} (${theme})`, async ({ page }) => {
        await login(page, screen.as ?? ADMIN)
        await page.goto(screen.path)
        await waitForPageContent(page)

        if (screen.prepare) {
          await screen.prepare(page)
        }

        if (screen.wait?.startsWith('value:')) {
          // 입력칸의 값은 getByDisplayValue 로 잡히지 않는 경우가 있다 — 이 화면의
          // 주소 칸은 disabled 다. 값을 직접 읽어 본다.
          const needle = screen.wait.slice('value:'.length)
          await expect
            .poll(
              () =>
                page.evaluate(
                  (text) =>
                    Array.from(document.querySelectorAll('input')).some((input) =>
                      input.value.includes(text),
                    ),
                  needle,
                ),
              { timeout: 20000 },
            )
            .toBe(true)
        } else if (screen.wait) {
          // 고정 응답이 실제로 그려졌는지 본다. 빈 화면을 문서에 싣지 않기 위한 장치다.
          await expect(page.getByText(screen.wait, { exact: false }).first()).toBeVisible({
            timeout: 20000,
          })
        }

        await stabilize(page)
        await page.screenshot({ path: `${OUT}/${screen.name}-${theme}.png` })
      })
    }
  })
}

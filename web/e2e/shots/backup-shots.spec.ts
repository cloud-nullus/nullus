import { test, expect } from '@playwright/test'

// 문서용 화면 캡처.
//
// 사용자 환경의 API 에는 백업 라우트가 없고(이전 빌드) 시드 계정도 없다.
// 그쪽을 갈아끼우는 대신 응답을 고정해 캡처한다 — 환경을 건드리지 않고,
// 매번 같은 그림이 나온다.
const RUNS = [
  {
    id: 'b1', mode: 'full', trigger: 'manual', status: 'succeeded',
    total_bytes: 371533847, quiesce_seconds: 50.3, created_at: '2026-09-03T04:24:04Z',
  },
  {
    id: 'b2', mode: 'full', trigger: 'schedule', status: 'partial',
    total_bytes: 368268809, quiesce_seconds: 49.4,
    error: 'keycloak_db: pg_dump 실패', created_at: '2026-09-02T14:59:24Z',
  },
  {
    id: 'b3', mode: 'platform_only', trigger: 'schedule', status: 'succeeded',
    total_bytes: 8438912, created_at: '2026-09-01T18:20:00Z',
  },
]

async function openBackupPage(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/login', (route) =>
    route.fulfill({
      json: {
        token: 'screenshot-token',
        user: {
          id: '00000000-0000-0000-0000-000000000001',
          email: 'admin@nullus.dev',
          name: 'Admin',
          role: 'admin',
          orgId: '00000000-0000-0000-0000-000000000001',
        },
      },
    }),
  )
  await page.route('**/admin/backups**', (route) =>
    route.fulfill({ json: route.request().method() === 'GET' ? { items: RUNS } : RUNS[0] }),
  )

  await page.goto('/login')
  await page.fill('#email', 'admin@nullus.dev')
  await page.fill('#password', 'admin123')
  await page.click('button[type="submit"]:not([disabled])')
  await page.waitForURL('**/admin/**', { timeout: 10000 })
  await page.goto('/admin/backup')
}

test.describe('백업 화면 캡처', () => {
  test('backup-list', async ({ page }) => {
    await openBackupPage(page)
    await expect(page.getByText('354.3 MB')).toBeVisible({ timeout: 10000 })
    await page.screenshot({ path: '../docs/assets/backup/backup-list.png' })
  })

  test('backup-dialog', async ({ page }) => {
    await openBackupPage(page)
    await page.getByRole('button', { name: /백업 실행|Run backup/ }).click()
    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5000 })
    // MUI Dialog 는 페이드인한다. 애니메이션 중에 찍으면 반투명하게 나와
    // 뒤 화면이 비친다 — 문서 그림으로 쓸 수 없다.
    await page.waitForTimeout(700)
    await page.getByRole('dialog').screenshot({ path: '../docs/assets/backup/backup-dialog.png' })
  })

  test('backup-dialog-nostop', async ({ page }) => {
    // 볼륨을 빼면 경고와 확인 입력이 사라진다 — 무중단 백업의 모습.
    await openBackupPage(page)
    await page.getByRole('button', { name: /백업 실행|Run backup/ }).click()
    await page.locator('#backup-target-volume').click()
    await expect(page.getByRole('alert')).toHaveCount(0)
    await page.waitForTimeout(700)
    await page.getByRole('dialog').screenshot({ path: '../docs/assets/backup/backup-dialog-nostop.png' })
  })
})

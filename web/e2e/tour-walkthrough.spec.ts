import { test, type Page } from '@playwright/test'
import { loginAs } from './helpers/auth'

/**
 * 투어를 처음부터 끝까지 걸어 보며 걸음마다 화면을 남긴다.
 *
 * 검증이 아니라 눈으로 확인하기 위한 도구다 — 강조가 엉뚱한 것을 잡거나, 팝업이
 * 열리지 않거나, 설명 상자가 화면 밖으로 나가는 것은 단정으로 잡기 어렵고 그림을
 * 봐야 안다. `npx playwright test e2e/tour-walkthrough.spec.ts` 로 돌린다.
 */
test('투어 전 걸음 스크린샷', async ({ page }: { page: Page }) => {
  test.setTimeout(300_000)

  await loginAs(page, 'admin')
  await page.setViewportSize({ width: 1440, height: 900 })

  // 헤더의 튜토리얼 버튼으로 시작한다 — 실제 사용자가 들어오는 문이다.
  await page.getByRole('button', { name: /Tutorial|튜토리얼/ }).click()
  await page.waitForTimeout(1200)

  // 걸음 수에 묶지 않는다 — 마지막 걸음의 "다음"(끝내기)이 투어를 닫으면 거기서 멈춘다.
  // 상한은 투어가 닫히지 않는 회귀에 대비한 안전판일 뿐이다.
  for (let index = 0; index < 60; index += 1) {
    const label = String(index + 1).padStart(2, '0')

    // 걸음이 화면을 옮기고 팝업을 열고 스크롤을 맞추는 데 시간이 걸린다.
    // 보정이 두 번(150ms·900ms) 도므로 그보다 넉넉히 기다린다.
    await page.waitForTimeout(2500)

    if ((await page.locator('[data-testid="tour-overlay"]').count()) === 0) break

    // 시간 제한을 둔다 — 없으면 제목이 안 나타날 때 테스트 시간을 통째로 기다린다.
    const heading = await page
      .locator('[data-testid="tour-overlay"] h3')
      .textContent({ timeout: 5000 })
      .catch(() => '(없음)')
    const hasSpotlight = await page.locator('[data-testid="tour-spotlight"]').count()
    // eslint-disable-next-line no-console
    console.log(`STEP ${label} | ${heading} | spotlight=${hasSpotlight}`)

    await page.screenshot({ path: `e2e/screenshots/tour/${label}.png` })

    // 역할(role)로 찾지 않는다 — 앱 모달이 열리면 MUI 가 나머지를 aria-hidden 으로
    // 덮어 ARIA 질의에서 투어가 사라진다.
    const next = page.locator('[data-testid="tour-next"]')
    if ((await next.count()) === 0) break
    await next.click({ force: true })
  }
})

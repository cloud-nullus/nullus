import { expect, type APIRequestContext, type Page } from "@playwright/test";

/**
 * 스택 템플릿 화면의 공용 로케이터.
 *
 * 세 스펙이 각자 'Use Base Template' 문구로 카드를 찾고 있었고, 셋 다 같은 이유로
 * 동시에 썩었다 — 그 버튼은 상세 모달 안으로 옮겨졌고 라벨도 바뀌었다. 화면 구조를
 * 아는 곳을 여기 하나로 둔다.
 */

/** 카드에 남아 있는 것은 상세 열기 버튼뿐이다. 이것으로 카드를 센다. */
export const templateCards = (page: Page) =>
  page
    .locator('main [class*="card"]')
    .filter({ has: page.getByRole("button", { name: VIEW_DETAIL }) });

export const VIEW_DETAIL = /view detail/i;

/**
 * 상세 모달의 "기본 템플릿으로 사용" 버튼.
 *
 * en.json 의 stackTemplatePage.actions.useBaseTemplate 값이다. 소스에는 t() 의
 * 인라인 폴백으로 'Use Base Template' 이 적혀 있지만 키가 존재하므로 화면에는
 * 절대 그 문구가 뜨지 않는다 — 폴백을 보고 셀렉터를 쓰면 영원히 0 개가 잡힌다.
 */
export const USE_AS_BASE = /use as base/i;

/** 카드 하나의 상세 모달을 연다. */
export async function openTemplateDetail(page: Page, index = 0) {
  await templateCards(page)
    .nth(index)
    .getByRole("button", { name: VIEW_DETAIL })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
}

/**
 * 서버가 주는 템플릿 개수.
 *
 * 화면 카드 수를 숫자로 박아 두면 템플릿이 늘 때마다 화면은 멀쩡한데 테스트만
 * 틀린다(실제로 4 로 박혀 있었고 지금은 9 개다). 서버 목록과 대조한다.
 */
export async function countTemplates(
  request: APIRequestContext,
  apiBase: string,
): Promise<number> {
  const res = await request.get(`${apiBase}/stacks/templates`);
  expect(res.ok()).toBeTruthy();
  const body = (await res.json()) as unknown;
  const items = Array.isArray(body)
    ? body
    : ((body as { items?: unknown[] }).items ?? []);
  expect(items.length).toBeGreaterThan(0);
  return items.length;
}

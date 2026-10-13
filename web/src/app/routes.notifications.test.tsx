import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// 화면을 만들어도 라우트·메뉴에 등록하지 않으면 닿을 길이 없다(routes.backup.test 와 같은 이유).
describe("알림 채널 화면 배선", () => {
  const read = (p: string) => readFileSync(join(__dirname, p), "utf-8");

  it("라우트에 /admin/notifications 가 등록돼 있다", () => {
    const routes = read("./routes.tsx");
    expect(routes).toContain("path: 'admin/notifications'");
    expect(routes).toContain("<NotificationChannelsPage />");
  });

  it("사이드바 관리 메뉴에 알림 채널이 있다(관리자 전용)", () => {
    const nav = read("../components/layout/nav-model.tsx");
    expect(nav).toMatch(
      /path: '\/admin\/notifications'[^\n]*roles: \['admin'\]/,
    );
    expect(nav).toContain("label: 'sidebar.notifications'");
  });

  it("메뉴 라벨의 번역 키가 두 언어에 다 있다", () => {
    for (const locale of ["ko", "en"]) {
      const dict = JSON.parse(read(`../i18n/${locale}.json`));
      expect(
        dict.sidebar?.notifications,
        `${locale}.json 에 sidebar.notifications 가 없다`,
      ).toBeTruthy();
    }
  });
});

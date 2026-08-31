import { test, expect } from "@playwright/test";
import { loginAs } from "./helpers/auth";
import { requireConnectedCluster } from "./helpers/preconditions";

test("배포 완료 후 메뉴 전환 가능 확인", async ({ page, request }) => {
  await requireConnectedCluster(request);

  await loginAs(page, "developer");
  await page.goto("/cicd/developer-deploy");
  await page.waitForLoadState("networkidle");

  await page.fill('input[placeholder*="app"]', "nav-test");
  const gitInput = page
    .locator('input[placeholder*="github"], input[placeholder*="repo"]')
    .first();
  // 보이지만 비활성인 칸이라 isVisible() 가드는 통과하고 fill() 이 30 초를 태운다.
  if (await gitInput.isEditable())
    await gitInput.fill("https://github.com/cloud-nullus/sample-go-api");

  await page
    .getByRole("button", { name: /^Create$/ })
    .last()
    .click();
  await expect(page).toHaveURL(/\/cicd\/list/, { timeout: 10000 });
  await page.getByText("nav-test", { exact: true }).first().click();
  await page.getByRole("button", { name: /^Deploy$/ }).click();
  await expect(page).toHaveURL(/\/cicd\/pipelines\/.*\/logs/, {
    timeout: 10000,
  });

  const sidebar = page.locator("aside");
  await sidebar.getByText("CI/CD List").click();
  await page.waitForTimeout(2000);
  await expect(page).toHaveURL(/\/cicd\/list/, { timeout: 5000 });
  await page.screenshot({ path: "e2e/screenshots/after-deploy-navigate.png" });
});

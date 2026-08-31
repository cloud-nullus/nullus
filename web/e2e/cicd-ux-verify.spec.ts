import { test, expect, type Page } from "@playwright/test";
import { loginAs, navigateToMenu } from "./helpers/auth";
import { requireConnectedCluster } from "./helpers/preconditions";

test.describe("CI/CD UX 개선 검증", () => {
  let page: Page;

  test.beforeEach(async ({ page: p }) => {
    page = p;
    await loginAs(page, "developer");
  });

  test("1. CI/CD List — 클러스터 필터 드롭다운 존재", async () => {
    await page.goto("/cicd/list");
    await page.waitForLoadState("networkidle");

    // 클러스터 필터 드롭다운 확인.
    // Select 는 <select> 가 아니라 role="combobox" 인 요소를 렌더한다.
    const clusterSelect = page
      .getByRole("combobox")
      .filter({ hasText: /All Clusters/ });
    await expect(clusterSelect).toBeVisible({ timeout: 5000 });
    await page.screenshot({
      path: "e2e/screenshots/cicd-list-cluster-filter.png",
    });
  });

  test("2. CI/CD List — Logs 버튼 → Pipeline Logs 페이지", async () => {
    await page.goto("/cicd/list");
    await page.waitForLoadState("networkidle");

    // 첫 번째 expand 버튼 클릭
    const expandBtn = page.locator("table button").first();
    if (await expandBtn.isVisible()) {
      await expandBtn.click();
      await page.waitForTimeout(500);

      // Logs 버튼 찾기
      const logsBtn = page.getByRole("button", { name: /Logs/i }).first();
      if (await logsBtn.isVisible()) {
        await logsBtn.click();
        await page.waitForLoadState("networkidle");
        // URL이 /cicd/pipelines/xxx/logs 패턴인지 확인
        await expect(page).toHaveURL(/\/cicd\/pipelines\/.*\/logs/);
        // Breadcrumb에 CI/CD List 링크 존재 확인
        await expect(
          page.getByLabel("Breadcrumb").getByText("CI/CD List"),
        ).toBeVisible();
        await page.screenshot({
          path: "e2e/screenshots/cicd-pipeline-logs-page.png",
        });
      }
    }
  });

  test("3. Pipeline Setup — 세로 섹션과 Review", async () => {
    await page.goto("/cicd/developer-deploy");
    await page.waitForLoadState("networkidle");

    // 모든 설정 섹션이 한 화면의 세로 흐름으로 표시되어야 함
    await expect(page.getByText("Enter App Name")).toBeVisible();
    const templateGrid = page.locator("text=앱 템플릿");
    await expect(templateGrid).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Code Checkout" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Build" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Test" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Security" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Create" })).toBeVisible();
    // 섹션 제목은 화면의 것과 같아야 한다. 예전에는 "Pipeline Configuration" 과
    // "3. Deploy" 로 적혀 있었는데, 실제 화면은 "Pipeline Setup" 과
    // "3. Image Registry" 다 — 문구가 바뀔 때 테스트가 함께 오지 않았다.
    await expect(page.getByRole("heading", { name: "Pipeline Setup" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "1. Code Checkout" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "2. Build" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "3. Image Registry" })).toBeVisible();
    await expect(page.getByText("Cluster & Namespace")).toBeVisible();

    await page.fill('input[placeholder*="app"]', "verify-test");
    const gitInput = page
      .locator('input[placeholder*="github"], input[placeholder*="repo"]')
      .first();
    // 보이지만 비활성인 칸이다 — 플랫폼이 만든 저장소 URL 을 되비추는 읽기 전용
    // 칸이라서다. isVisible() 로 거르면 가드를 통과한 뒤 fill() 이 활성화를 기다리다
    // 30 초 만에 죽는다. 채울 수 있을 때만 채운다.
    if (await gitInput.isEditable()) {
      await gitInput.fill("https://github.com/cloud-nullus/sample-go-api");
    }

    await expect(page.getByText("Resource Configuration").last()).toBeVisible();
    await expect(page.getByText("Environment Variables").last()).toBeVisible();
    await expect(page.getByText("Manifest Types")).toHaveCount(0);
    await expect(page.locator('input[type="range"]')).toHaveCount(0);
    await page.screenshot({
      path: "e2e/screenshots/developer-deploy-steps.png",
    });
    await expect(
      page.getByRole("button", { name: /^Create$/ }).last(),
    ).toBeVisible();
  });

  // Review Manifest 는 클러스터를 고른 뒤에만 그려진다(canReview 가 clusterId 를
  // 요구한다). 위의 레이아웃 검사와 한 테스트에 묶어 두면, 클러스터가 없다는 이유로
  // 섹션 배치 회귀까지 함께 못 보게 된다.
  test("3-1. Pipeline Setup — Review Manifest 미리보기", async ({ request }) => {
    await requireConnectedCluster(request);

    await page.goto("/cicd/developer-deploy");
    await page.waitForLoadState("networkidle");
    await page.fill('input[placeholder*="app"]', "verify-test");

    await expect(page.getByText(/Review.*Manifest/).last()).toBeVisible();
    await expect(page.getByText("verify-test-deployment.yaml")).toBeVisible();
    await expect(page.getByText("verify-test-service.yaml")).toBeVisible();
    await expect(page.getByText("verify-test-ingress.yaml")).toBeVisible();
  });

  test("4. Pipeline Setup — 생성 후 목록에서 배포 실행", async ({ request }) => {
    // 파이프라인 생성은 대상 클러스터를 고르지 않으면 검증에서 막힌다.
    await requireConnectedCluster(request);

    await page.goto("/cicd/developer-deploy");
    await page.waitForLoadState("networkidle");

    await page.fill('input[placeholder*="app"]', "ws-verify");
    const gitInput = page
      .locator('input[placeholder*="github"], input[placeholder*="repo"]')
      .first();
    if (await gitInput.isEditable()) {
      await gitInput.fill("https://github.com/cloud-nullus/sample-go-api");
    }
    await page
      .getByRole("button", { name: /^Create$/ })
      .last()
      .click();
    await expect(page).toHaveURL(/\/cicd\/list/, { timeout: 10000 });
    await page.getByText("ws-verify", { exact: true }).first().click();
    await page.getByRole("button", { name: /^Deploy$/ }).click();
    await expect(page).toHaveURL(/\/cicd\/pipelines\/.*\/logs/, {
      timeout: 10000,
    });
    await page.screenshot({ path: "e2e/screenshots/deploy-logs-page.png" });
  });

  test("5. CI/CD History — Breadcrumb 네비게이션", async () => {
    await page.goto("/cicd/history");
    await page.waitForLoadState("networkidle");

    // Breadcrumb에 CI/CD List 링크 확인
    const breadcrumbLink = page
      .locator('a, [role="link"]')
      .filter({ hasText: "CI/CD List" })
      .first();
    await expect(breadcrumbLink).toBeVisible({ timeout: 5000 });
    await page.screenshot({
      path: "e2e/screenshots/cicd-history-breadcrumb.png",
    });
  });
});

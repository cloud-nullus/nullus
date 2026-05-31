import { test, expect } from "@playwright/test";

import { loginAs, expectMenuVisible, expectMenuHidden } from "../helpers/auth";

test.describe("Developer UAT Scenarios", () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, "developer");
  });

  test("V1: Pipeline Setup 페이지 렌더링", async ({ page }) => {
    await expect(page.locator("h1")).toContainText(/pipeline setup/i, {
      timeout: 10000,
    });
  });

  test("V1: 앱 배포 세로 섹션 및 이동 버튼 존재", async ({ page }) => {
    await expect(page.getByRole("button", { name: "Basic Info" })).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page.getByRole("button", { name: "Code Checkout" }),
    ).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole("button", { name: "Build" })).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByRole("button", { name: "Test" })).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByRole("button", { name: "Security" })).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByRole("button", { name: "Create" })).toBeVisible({
      timeout: 10000,
    });
  });

  test("V1: 앱 이름 입력 필드", async ({ page }) => {
    await expect(page.getByPlaceholder("my-awesome-app")).toBeVisible({
      timeout: 10000,
    });
  });

  test("V1: Source Repository 입력 필드", async ({ page }) => {
    await expect(page.getByLabel("Source Repository")).toBeVisible({
      timeout: 10000,
    });
  });

  test("V1: 클러스터/네임스페이스 드롭다운", async ({ page }) => {
    await expect(page.getByText(/cluster|클러스터/i).first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("V1: Create 버튼 존재", async ({ page }) => {
    await page.getByPlaceholder("my-awesome-app").fill("uat-app");
    await page
      .getByLabel("Source Repository")
      .fill("https://github.com/cloud-nullus/draft.git");
    await expect(
      page.getByRole("button", { name: /^Create$/ }).last(),
    ).toBeVisible({ timeout: 10000 });
  });

  test("V2: CI/CD 이력 페이지 접근", async ({ page }) => {
    await page.goto("/cicd/history");
    await expect(page.locator("h1")).toContainText(/history|이력/i, {
      timeout: 10000,
    });
  });

  test("V2: 배포 이력 테이블 렌더링", async ({ page }) => {
    await page.goto("/cicd/history");
    await expect(page.locator('table, [role="table"]').first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("V2: 배포 이력 상태 필터", async ({ page }) => {
    await page.goto("/cicd/history");
    await expect(
      page
        .locator('select, input[type="search"], input[placeholder*="search" i]')
        .first(),
    ).toBeVisible({ timeout: 10000 });
  });

  test("V4: 모니터링 대시보드 접근 가능", async ({ page }) => {
    await page.goto("/observability/monitoring");
    await expect(page.locator("h1")).toContainText(/monitoring|모니터링/i, {
      timeout: 10000,
    });
  });

  test("V4: Developer는 알림 규칙 메뉴 숨김", async ({ page }) => {
    await expectMenuHidden(page, "Alert Rules");
  });

  test("V4: 모니터링 대시보드 차트 렌더링", async ({ page }) => {
    await page.goto("/observability/monitoring");
    await expect(
      page.locator('[class*="card"], svg, canvas').first(),
    ).toBeVisible({ timeout: 10000 });
  });

  test("Developer: CI/CD 메뉴 표시", async ({ page }) => {
    await expectMenuVisible(page, "CI/CD");
  });

  test("Developer: DevSecOps Stack 메뉴 숨김", async ({ page }) => {
    await expectMenuHidden(page, "DevSecOps Stack");
  });

  test("Developer: Admin 메뉴 숨김", async ({ page }) => {
    await expectMenuHidden(page, "Admin");
  });

  test("Developer: CI/CD 파이프라인 목록 접근 가능", async ({ page }) => {
    await expectMenuVisible(page, "Observability");
    await page.goto("/cicd/list");
    await expect(page.locator("h1")).toContainText(/ci\/cd list/i, {
      timeout: 10000,
    });
  });
});

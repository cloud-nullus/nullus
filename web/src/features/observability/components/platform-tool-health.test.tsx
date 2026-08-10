import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { PlatformToolHealth } from "./platform-tool-health";
import type { ToolHealth } from "../../../types";

const tools: ToolHealth[] = [
  { name: "GitLab CE", version: "18.5.1", status: "running" },
  { name: "Harbor", version: "2.11.0", status: "warning" },
  { name: "Nexus", version: "3.64.0", status: "error" },
];

describe("PlatformToolHealth", () => {
  it("lists every tool with its version", () => {
    renderWithProviders(<PlatformToolHealth tools={tools} isLoading={false} />);

    for (const tool of tools) {
      expect(screen.queryByText(tool.name)).not.toBeNull();
      expect(screen.queryByText(tool.version)).not.toBeNull();
    }
  });

  it("labels each tool with its status so it is readable without color", () => {
    renderWithProviders(<PlatformToolHealth tools={tools} isLoading={false} />);

    expect(screen.getByRole("listitem", { name: /GitLab CE/ }).textContent).toContain("running");
    expect(screen.getByRole("listitem", { name: /Harbor/ }).textContent).toContain("warning");
    expect(screen.getByRole("listitem", { name: /Nexus/ }).textContent).toContain("error");
  });

  // 한눈에 "지금 문제 있나" 를 보는 게 이 카드의 목적이다.
  it("summarizes how many tools are unhealthy", () => {
    renderWithProviders(<PlatformToolHealth tools={tools} isLoading={false} />);

    expect(screen.queryByText("1 / 3 healthy")).not.toBeNull();
  });

  it("sorts unhealthy tools first", () => {
    renderWithProviders(<PlatformToolHealth tools={tools} isLoading={false} />);

    const names = screen.getAllByRole("listitem").map((el) => el.getAttribute("aria-label"));
    expect(names).toEqual(["Nexus", "Harbor", "GitLab CE"]);
  });

  it("shows a loading hint while the first fetch is in flight", () => {
    renderWithProviders(<PlatformToolHealth tools={undefined} isLoading />);

    expect(screen.queryByText("Loading...")).not.toBeNull();
  });

  // 설치된 스택이 없으면 도구도 없다 — 오류가 아니라 정상 상태다.
  it("explains the empty case instead of rendering a bare card", () => {
    renderWithProviders(<PlatformToolHealth tools={[]} isLoading={false} />);

    expect(screen.queryByText(/No installed tools yet/i)).not.toBeNull();
  });
});

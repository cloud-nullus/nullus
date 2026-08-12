import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { StackWorkloadsTab } from "./stack-workloads-tab";

const mockUseStackMonitoring = vi.hoisted(() => vi.fn());

vi.mock("../api/stack-api", () => ({
  useStackMonitoring: (...args: unknown[]) => mockUseStackMonitoring(...args),
}));

const snapshot = {
  namespace: "nullus-gitlab31",
  oss_statuses: [
    {
      key: "cd_tool",
      name: "argocd",
      version: "6.8.0",
      enabled: true,
      status: "running",
      pod_count: 1,
      ready_pods: 1,
      pods: [
        {
          name: "argo-cd-server-788bbc9654-tdd74",
          phase: "Running",
          ready: true,
          restart_count: 0,
          node_name: "nullus-platform",
          cpu_usage_millicores: 4,
          cpu_limit_millicores: 0,
          memory_usage_mib: 42,
          memory_limit_mib: 0,
          status: "running",
        },
      ],
    },
    {
      key: "source_repository",
      name: "gitlab",
      version: "9.5.1",
      enabled: true,
      status: "error",
      pod_count: 1,
      ready_pods: 0,
      pods: [
        {
          name: "gitlab-gitaly-0",
          phase: "Running",
          ready: false,
          restart_count: 7,
          node_name: "nullus-platform",
          cpu_usage_millicores: 7,
          cpu_limit_millicores: 2000,
          memory_usage_mib: 2048,
          memory_limit_mib: 4096,
          status: "error",
        },
      ],
    },
    // 설치하지 않은 도구는 목록에 없어야 한다.
    {
      key: "package_registry",
      name: "nexus",
      version: "3.64.0",
      enabled: false,
      status: "running",
      pod_count: 0,
      ready_pods: 0,
      pods: [
        {
          name: "nexus-0",
          phase: "Running",
          ready: true,
          restart_count: 0,
          node_name: "nullus-platform",
          cpu_usage_millicores: 1,
          cpu_limit_millicores: 0,
          memory_usage_mib: 10,
          memory_limit_mib: 0,
          status: "running",
        },
      ],
    },
  ],
};

describe("StackWorkloadsTab", () => {
  beforeEach(() => {
    mockUseStackMonitoring.mockReset();
    mockUseStackMonitoring.mockReturnValue({ data: snapshot, isLoading: false });
  });

  // 조회 조건이 화면에 박혀 있지 않아야 한다 — 스택 id 로만 부른다.
  it("queries by the stack it is given", () => {
    renderWithProviders(<StackWorkloadsTab stackId="stk-42" />);

    expect(mockUseStackMonitoring).toHaveBeenCalledWith("stk-42", 30_000);
  });

  // 네임스페이스도 응답에서 온다. 화면에 'nullus' 를 박아 두면 스택마다 다른
  // 네임스페이스를 쓰는 배포에서 거짓말이 된다.
  it("shows the namespace reported by the server", () => {
    renderWithProviders(<StackWorkloadsTab stackId="stk-42" />);

    expect(screen.getByText("ns/nullus-gitlab31")).toBeTruthy();
  });

  it("lists pods of installed tools only, worst status first", () => {
    renderWithProviders(<StackWorkloadsTab stackId="stk-42" />);

    const rows = screen.getAllByRole("row").slice(1);
    expect(rows[0].textContent).toContain("gitlab-gitaly-0");
    expect(rows[1].textContent).toContain("argo-cd-server-788bbc9654-tdd74");
    expect(screen.queryByText("nexus-0")).toBeNull();
  });

  // Running 인데 준비가 안 된 파드가 장애의 대부분이다. phase 만 적으면 놓친다.
  it("marks a running-but-not-ready pod", () => {
    renderWithProviders(<StackWorkloadsTab stackId="stk-42" />);

    const row = screen.getByText("gitlab-gitaly-0").closest("tr")!;
    expect(within(row).getByText(/Running · not ready/)).toBeTruthy();
  });

  it("counts ready pods in the summary", () => {
    renderWithProviders(<StackWorkloadsTab stackId="stk-42" />);

    expect(screen.getByText("1 / 2 pods ready")).toBeTruthy();
  });
});

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { CicdListPage } from "./cicd-list-page";

const mockNavigate = vi.fn();
const mockUsePipelines = vi.fn();
const mockUseStackWorkloads = vi.fn();
const mockUseStackWorkloadLogs = vi.fn();
const mockUseDeletePipeline = vi.fn();
const mockUseDeployPipeline = vi.fn();
const mockUseTemplateById = vi.fn();
const mockUsePipelineDeployments = vi.fn();
const mockUsePipelineResources = vi.fn();
const mockUseDeploymentStatus = vi.fn();
const mockDeployPipeline = vi.fn();

vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock("../../stack/api/stack-api", () => ({
  useStacks: () => ({ data: { items: [{ id: "stack-1", name: "prod-stack" }], total: 1 } }),
  useStackWorkloads: (...args: unknown[]) => mockUseStackWorkloads(...args),
  useStackWorkloadLogs: (...args: unknown[]) => mockUseStackWorkloadLogs(...args),
}));

vi.mock("../api/cicd-api", () => ({
  usePipelines: (...args: unknown[]) => mockUsePipelines(...args),
  useDeletePipeline: (...args: unknown[]) => mockUseDeletePipeline(...args),
  useDeployPipeline: (...args: unknown[]) => mockUseDeployPipeline(...args),
  useTemplateById: (...args: unknown[]) => mockUseTemplateById(...args),
  usePipelineDeployments: (...args: unknown[]) =>
    mockUsePipelineDeployments(...args),
  usePipelineResources: (...args: unknown[]) =>
    mockUsePipelineResources(...args),
  useDeploymentStatus: (...args: unknown[]) => mockUseDeploymentStatus(...args),
}));

const pipelines = [
  {
    id: "pipeline-1",
    name: "frontend-web",
    appType: "web-frontend",
    clusterId: "c1",
    clusterName: "prod-k8s",
    status: "success",
    lastDeployedAt: "2026-03-03T14:28:00Z",
    stackId: "stack-1",
  },
];

describe("CicdListPage", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockUsePipelines.mockReset();
    mockUseDeletePipeline.mockReset();
    mockUseDeployPipeline.mockReset();
    mockUseTemplateById.mockReset();
    mockUsePipelineDeployments.mockReset();
    mockUsePipelineResources.mockReset();
    mockUseDeploymentStatus.mockReset();
    mockDeployPipeline.mockReset();
    mockUsePipelines.mockReturnValue({
      data: { items: pipelines, total: pipelines.length },
      isLoading: false,
    });
    mockUseDeletePipeline.mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    });
    mockUseDeployPipeline.mockReturnValue({
      mutateAsync: mockDeployPipeline,
      isPending: false,
    });
    mockDeployPipeline.mockResolvedValue({ deploymentId: "deployment-1" });
    mockUseTemplateById.mockReturnValue({ data: undefined, isLoading: false });
    mockUsePipelineDeployments.mockReturnValue({
      data: { items: [] },
      isLoading: false,
    });
    mockUsePipelineResources.mockReturnValue({
      data: { items: [] },
      isLoading: false,
    });
    mockUseDeploymentStatus.mockReturnValue({
      data: undefined,
      isLoading: false,
    });
    mockUseStackWorkloads.mockReturnValue({ data: undefined, dataUpdatedAt: 0 });
    mockUseStackWorkloadLogs.mockReturnValue({ data: undefined, isLoading: false });
  });

  it("renders loading state safely", () => {
    mockUsePipelines.mockReturnValue({ data: undefined, isLoading: true });

    renderWithProviders(<CicdListPage />);

    expect(screen.getAllByText("CI/CD List").length).toBeGreaterThan(0);
    expect(screen.getAllByText("No pipelines found.").length).toBeGreaterThan(
      0,
    );
  });

  it("renders pipeline data", () => {
    renderWithProviders(<CicdListPage />);

    expect(screen.getAllByText("frontend-web").length).toBeGreaterThan(0);
    expect(screen.getAllByText("prod-k8s").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/success/i).length).toBeGreaterThan(0);
  });

  it("renders empty state when no pipelines returned", () => {
    mockUsePipelines.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
    });

    renderWithProviders(<CicdListPage />);

    expect(screen.getByText("No pipelines found.")).not.toBeNull();
  });

  it("navigates to templates page", () => {
    renderWithProviders(<CicdListPage />);

    fireEvent.click(screen.getByRole("button", { name: "New Pipeline" }));
    expect(mockNavigate).toHaveBeenCalledWith("/cicd/templates");
  });

  it("navigates to phase setup page from Add Phase", () => {
    renderWithProviders(<CicdListPage />);

    fireEvent.click(screen.getByRole("button", { name: "Add Phase" }));
    expect(mockNavigate).toHaveBeenCalledWith("/cicd/developer-deploy");
  });

  // 파이프라인이 어느 스택 위에서 도는지 보여야 한다. 스택마다 레지스트리가
  // 달라 이미지가 어디로 올라가는지가 스택에 따라 달라진다.
  it("shows the stack a pipeline belongs to", () => {
    renderWithProviders(<CicdListPage />);

    expect(screen.getAllByText("prod-stack").length).toBeGreaterThan(0);
  });

  // 스택이 지워져도 파이프라인 행은 남는다. 그때 stack_id 를 이름 자리에 넣으면
  // 화면에 stk_c073c556ed8c 가 스택 "이름" 으로 뜬다 — 사용자는 그게 이름인 줄
  // 알고, 실제로는 가리키는 스택이 없다는 사실을 놓친다.
  it("없는 스택을 가리키면 id 를 이름처럼 보여주지 않는다", () => {
    mockUsePipelines.mockReturnValue({
      data: {
        items: [{ ...pipelines[0], stackId: "stk_gone" }],
        total: 1,
      },
      isLoading: false,
    });

    renderWithProviders(<CicdListPage />);

    // 이름은 목록 행과 상세 패널 양쪽에 있다. 표 행으로 좁힌다.
    const row = screen
      .getAllByText("frontend-web")
      .map((node) => node.closest("tr"))
      .find(Boolean)!;
    expect(within(row).queryByText("stk_gone")).toBeNull();
    expect(within(row).getByText(/삭제됨|Deleted/)).toBeTruthy();
  });

  // 모니터링 탭은 실행 이력 KPI 만 보여줬다. 그런데 GitOps 로 도는 배포는
  // pipeline_deployments 에 남지 않아 그 KPI 가 전부 0 이고, 앱이 실제로 어떤
  // 상태인지 알 방법이 없었다. 모니터링 대시보드와 같은 실시간 패널을 붙인다.
  describe("모니터링 탭", () => {
    function openMonitoring() {
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /Monitoring/i }));
    }

    it("실시간 자원 그래프와 로그를 보여준다", () => {
      openMonitoring();

      expect(screen.getByText("App CPU (Live)")).toBeTruthy();
      expect(screen.getByText("App Memory (Live)")).toBeTruthy();
      expect(screen.getByText("Application Logs")).toBeTruthy();
    });

    // 파이프라인의 스택으로 조회해야 한다. 스택을 모르면 워크로드를 못 찾는다.
    it("파이프라인의 스택으로 조회한다", () => {
      openMonitoring();

      const workloadCalls = mockUseStackWorkloads.mock.calls;
      const logCalls = mockUseStackWorkloadLogs.mock.calls;
      expect(workloadCalls[workloadCalls.length - 1][0]).toBe("stack-1");
      expect(logCalls[logCalls.length - 1][0]).toBe("stack-1");
    });

    // 한 스택에 앱이 여럿이면 옆 앱의 로그가 섞인다. 이 파이프라인 것만 남긴다.
    it("이 파이프라인의 로그만 남긴다", () => {
      mockUseStackWorkloadLogs.mockReturnValue({
        isLoading: false,
        data: {
          pods: ["frontend-web-aaaaaa", "other-app-bbbbbb"],
          truncated: false,
          lines: [
            { pod: "frontend-web-aaaaaa", app: "frontend-web", timestamp: "2026-08-12T10:20:30.000Z", message: "mine" },
            { pod: "other-app-bbbbbb", app: "other-app", timestamp: "2026-08-12T10:20:31.000Z", message: "not mine" },
          ],
        },
      });

      openMonitoring();

      expect(screen.getByText("mine")).toBeTruthy();
      expect(screen.queryByText("not mine")).toBeNull();
    });

    // 스택에 연결되지 않은 파이프라인은 조회할 대상이 없다. 그 이유를 말한다.
    it("스택이 없으면 이유를 알려 준다", () => {
      mockUsePipelines.mockReturnValue({
        data: { items: [{ ...pipelines[0], stackId: "" }], total: 1 },
        isLoading: false,
      });

      openMonitoring();

      expect(screen.getAllByText(/스택에 연결되어야/).length).toBeGreaterThan(0);
    });
  });

  it("deploys the selected pipeline from the list detail panel and opens logs", async () => {
    renderWithProviders(<CicdListPage />);

    // 상세 패널의 Execute 는 곧바로 배포하지 않고 확인 모달을 연다.
    fireEvent.click(screen.getByRole("button", { name: "Execute" }));

    // 모달의 확인 버튼도 이름이 Execute 라 둘이 함께 잡힌다.
    // 나중에 렌더되는 모달 쪽이 실제 배포를 실행한다.
    const executeButtons = screen.getAllByRole("button", { name: "Execute" });
    expect(executeButtons.length).toBe(2);
    fireEvent.click(executeButtons[executeButtons.length - 1]);

    await waitFor(() => {
      expect(mockDeployPipeline).toHaveBeenCalledWith({
        pipelineId: "pipeline-1",
      });
      // 로그 화면이 어느 배포를 보여줄지 알아야 하므로 deploymentId 를 넘긴다.
      expect(mockNavigate).toHaveBeenCalledWith(
        "/cicd/pipelines/pipeline-1/logs?deploymentId=deployment-1",
      );
    });
  });
});

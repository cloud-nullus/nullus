import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { CicdListPage } from "./cicd-list-page";

const mockNavigate = vi.fn();
const mockUsePipelines = vi.fn();
const mockUseStackWorkloads = vi.fn();
const mockUseStackWorkloadLogs = vi.fn();
const mockUseDeletePipeline = vi.fn();
const mockUseRefreshPipelineDefinition = vi.fn();
const mockRefreshDefinition = vi.fn();
const mockUseDeployPipeline = vi.fn();
const mockUseTemplateById = vi.fn();
const mockUsePipelineDeployments = vi.fn();
const mockUsePipelineResources = vi.fn();
const mockUseDeploymentStatus = vi.fn();
const mockUsePipelineImageScans = vi.fn();
const mockUsePipelineSASTResults = vi.fn();
const mockUsePipelineScanVulnerabilities = vi.fn();
const mockUsePipelineDependencyScans = vi.fn();
const mockUsePipelineSecretScans = vi.fn();
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
  useRefreshPipelineDefinition: (...args: unknown[]) =>
    mockUseRefreshPipelineDefinition(...args),
  useDeployPipeline: (...args: unknown[]) => mockUseDeployPipeline(...args),
  useTemplateById: (...args: unknown[]) => mockUseTemplateById(...args),
  usePipelineDeployments: (...args: unknown[]) =>
    mockUsePipelineDeployments(...args),
  usePipelineResources: (...args: unknown[]) =>
    mockUsePipelineResources(...args),
  useDeploymentStatus: (...args: unknown[]) => mockUseDeploymentStatus(...args),
  usePipelineImageScans: (...args: unknown[]) =>
    mockUsePipelineImageScans(...args),
  usePipelineSASTResults: (...args: unknown[]) =>
    mockUsePipelineSASTResults(...args),
  usePipelineScanVulnerabilities: (...args: unknown[]) =>
    mockUsePipelineScanVulnerabilities(...args),
  usePipelineDependencyScans: (...args: unknown[]) =>
    mockUsePipelineDependencyScans(...args),
  usePipelineSecretScans: (...args: unknown[]) =>
    mockUsePipelineSecretScans(...args),
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
    mockUsePipelineImageScans.mockReset();
    mockDeployPipeline.mockReset();
    mockUsePipelineImageScans.mockReturnValue({ data: undefined, isError: false });
    mockUsePipelineSASTResults.mockReset();
    mockUsePipelineSASTResults.mockReturnValue({ data: undefined, isError: false });
    mockUsePipelineScanVulnerabilities.mockReset();
    mockUsePipelineDependencyScans.mockReset();
    mockUsePipelineDependencyScans.mockReturnValue({ data: undefined, isError: false });
    mockUsePipelineSecretScans.mockReset();
    mockUsePipelineSecretScans.mockReturnValue({ data: undefined, isError: false });
    mockUsePipelineScanVulnerabilities.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      isFetching: false,
    });
    mockUsePipelines.mockReturnValue({
      data: { items: pipelines, total: pipelines.length },
      isLoading: false,
    });
    mockUseDeletePipeline.mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    });
    mockRefreshDefinition.mockReset();
    mockUseRefreshPipelineDefinition.mockReset();
    mockUseRefreshPipelineDefinition.mockReturnValue({
      mutateAsync: mockRefreshDefinition,
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

  // SAST 리포트 전에 만든 파이프라인은 정의를 다시 써야 결과가 붙는다. 사용자가 손본 정의를
  // 덮어쓰므로 확인을 거치고, 결과(단계·채울 변수)를 화면에 남긴다.
  describe("CI 정의 갱신", () => {
    it("확인 뒤 정의를 다시 쓰고 결과를 보여 준다", async () => {
      mockRefreshDefinition.mockResolvedValue({
        stages: ["Build", "SAST", "Deploy"],
        warnings: [],
        missingVariables: ["SONAR_TOKEN"],
        repositoryRecreated: false,
      });
      renderWithProviders(<CicdListPage />);

      fireEvent.click(screen.getByRole("button", { name: /Refresh definition/i }));
      // 대화상자가 열리면 같은 이름의 확인 버튼이 하나 더 생긴다.
      const confirm = screen.getAllByRole("button", { name: /Refresh definition/i }).at(-1)!;
      fireEvent.click(confirm);

      await waitFor(() => {
        expect(mockRefreshDefinition).toHaveBeenCalledWith("pipeline-1");
      });
      const notice = await screen.findByTestId("refresh-definition-notice");
      expect(within(notice).getByText(/Build → SAST → Deploy/)).toBeTruthy();
      expect(within(notice).getByText(/SONAR_TOKEN/)).toBeTruthy();
    });

    it("실패하면 이유를 화면에 남긴다", async () => {
      mockRefreshDefinition.mockRejectedValue(new Error("gitlab down"));
      renderWithProviders(<CicdListPage />);

      fireEvent.click(screen.getByRole("button", { name: /Refresh definition/i }));
      fireEvent.click(screen.getAllByRole("button", { name: /Refresh definition/i }).at(-1)!);

      const notice = await screen.findByTestId("refresh-definition-notice");
      expect(within(notice).getByText(/gitlab down/)).toBeTruthy();
    });
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

  // 실행 이력에서 그 실행이 만든 이미지가 스캔에서 어떻게 판정됐는지 보여야 한다.
  // 예전에는 ImageScan 단계가 돌았는지만 보였고, 차단·경고·오류가 구분되지 않았다.
  describe("실행 이력의 이미지 스캔", () => {
    const deployments = [
      {
        id: "dep_ci_pip_x_2",
        pipelineId: "pipeline-1",
        pipelineName: "frontend-web",
        version: "v0.1.2",
        status: "success",
        triggeredBy: "kim.dev",
        startedAt: "2026-09-13T13:20:00Z",
        completedAt: "2026-09-13T13:30:00Z",
      },
      {
        // 취소된 실행에는 스캔이 없다.
        id: "dep_ci_pip_x_1",
        pipelineId: "pipeline-1",
        pipelineName: "frontend-web",
        version: "v0.1.1",
        status: "failed",
        triggeredBy: "kim.dev",
        startedAt: "2026-09-12T13:20:00Z",
        completedAt: "2026-09-12T13:21:00Z",
      },
    ];

    const blockedScan = {
      id: "scan_dep_ci_pip_x_2",
      pipelineId: "pipeline-1",
      deploymentId: "dep_ci_pip_x_2",
      imageRepository: "harbor.example/nullus/app",
      imageTag: "09b48b6e",
      imageDigest: "sha256:0c92aaaaaaaaaaaaaaaa",
      scanSource: "central",
      scanner: "trivy",
      scannerVersion: "0.74.0",
      dbUpdatedAt: "2026-09-13T07:13:14Z",
      counts: { critical: 2, high: 6, medium: 23, low: 19, unknown: 0 },
      gateResult: "block",
      reportUri: "",
      scannedAt: "2026-09-13T13:29:51Z",
      dbStale: false,
    };

    function rowOf(version: string) {
      return screen.getByRole("button", { name: version }).parentElement!;
    }

    function openHistory() {
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /^History$/ }));
    }

    beforeEach(() => {
      mockUsePipelineDeployments.mockReturnValue({
        data: { items: deployments, total: deployments.length },
        isLoading: false,
        dataUpdatedAt: 1234,
      });
    });

    // 실행 목록을 불러오는 요청이 서버에서 스캔 기록을 만든다. 실행 목록이
    // 새로 올 때마다 스캔도 다시 읽어야 방금 끝난 실행의 결과가 보인다.
    it("실행 목록이 갱신된 시각과 함께 스캔을 조회한다", () => {
      mockUsePipelineImageScans.mockReturnValue({
        data: { items: [blockedScan], total: 1 },
      });

      openHistory();

      const calls = mockUsePipelineImageScans.mock.calls;
      expect(calls[calls.length - 1]).toEqual(["pipeline-1", 1234]);
    });

    it("deployment_id 로 이어진 실행 행에만 게이트 배지를 붙인다", () => {
      mockUsePipelineImageScans.mockReturnValue({
        data: { items: [blockedScan], total: 1 },
      });

      openHistory();

      expect(within(rowOf("v0.1.2")).getByText("Blocked by policy")).toBeTruthy();
      expect(within(rowOf("v0.1.2")).getByLabelText("Critical 2")).toBeTruthy();
      expect(within(rowOf("v0.1.1")).queryByText("Blocked by policy")).toBeNull();
      expect(within(rowOf("v0.1.1")).queryByText("Counts unknown")).toBeNull();
      // 선택된(첫) 실행의 상세에 스캔 블록이 붙는다.
      expect(screen.getByText("Image scan")).toBeTruthy();
    });

    it("건수가 없는 스캔은 0 이 아니라 모름으로 보인다", () => {
      mockUsePipelineImageScans.mockReturnValue({
        data: {
          items: [{ ...blockedScan, counts: undefined, gateResult: "error" }],
          total: 1,
        },
      });

      openHistory();

      const row = rowOf("v0.1.2");
      expect(within(row).getByText("Scan error")).toBeTruthy();
      expect(within(row).getByText("Counts unknown")).toBeTruthy();
      expect(within(row).queryByLabelText(/Critical/)).toBeNull();
    });

    // 건수만으로는 무엇을 고쳐야 하는지 알 수 없다. 선택한 실행의 스캔 상세에서 목록을 펼친다.
    it("선택한 실행의 스캔 상세에서 취약점 목록을 펼치면 그 스캔으로 조회한다", () => {
      mockUsePipelineImageScans.mockReturnValue({
        data: { items: [blockedScan], total: 1 },
      });

      openHistory();

      const before = mockUsePipelineScanVulnerabilities.mock.calls;
      expect(before[before.length - 1]?.[3]).toBe(false);

      fireEvent.click(
        screen.getByRole("button", { name: "View vulnerability list" }),
      );

      const calls = mockUsePipelineScanVulnerabilities.mock.calls;
      expect(calls[calls.length - 1]).toEqual([
        "pipeline-1",
        "scan_dep_ci_pip_x_2",
        expect.objectContaining({ offset: 0, severities: [] }),
        true,
      ]);
    });

    // 스캔 API 가 아직 배선되지 않은 환경(503)에서도 이력 탭은 그대로 떠야 한다.
    it("스캔 조회가 실패해도 이력은 그대로 보인다", () => {
      mockUsePipelineImageScans.mockReturnValue({
        data: undefined,
        isError: true,
      });

      openHistory();

      expect(screen.getByRole("button", { name: "v0.1.2" })).toBeTruthy();
      expect(screen.queryByText("Image scan")).toBeNull();
    });
  });

  describe("실행 이력의 소스 정적 분석", () => {
    const deployments = [
      {
        id: "dep_ci_pip_x_2",
        pipelineId: "pipeline-1",
        pipelineName: "frontend-web",
        version: "v0.1.2",
        status: "failed",
        triggeredBy: "kim.dev",
        startedAt: "2026-10-11T01:20:00Z",
        completedAt: "2026-10-11T01:30:00Z",
      },
      {
        // 리포트를 남기기 전에 만든 파이프라인의 실행 — 분석 결과가 없다.
        id: "dep_ci_pip_x_1",
        pipelineId: "pipeline-1",
        pipelineName: "frontend-web",
        version: "v0.1.1",
        status: "success",
        triggeredBy: "kim.dev",
        startedAt: "2026-10-10T13:20:00Z",
        completedAt: "2026-10-10T13:21:00Z",
      },
    ];

    const blocked = {
      id: "sast_dep_ci_pip_x_2",
      pipelineId: "pipeline-1",
      deploymentId: "dep_ci_pip_x_2",
      projectKey: "frontend-web",
      qualityGateStatus: "ERROR",
      gateResult: "block",
      conditions: [
        { metric: "new_violations", comparator: "GT", threshold: "0", actual: "1", status: "ERROR" },
      ],
      metrics: { vulnerabilities: 3, bugs: 0 },
      dashboardUrl: "https://sonarqube.example.com/dashboard?id=frontend-web",
      analyzedAt: "2026-10-11T01:21:00Z",
    };

    function rowOf(version: string) {
      return screen.getByRole("button", { name: version }).parentElement!;
    }

    function openHistory() {
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /^History$/ }));
    }

    beforeEach(() => {
      mockUsePipelineDeployments.mockReturnValue({
        data: { items: deployments, total: deployments.length },
        isLoading: false,
        dataUpdatedAt: 1234,
      });
    });

    // 실행 목록 요청이 서버에서 분석 결과를 들인다. 실행 목록이 새로 올 때마다 다시 읽는다.
    it("실행 목록이 갱신된 시각과 함께 분석 결과를 조회한다", () => {
      mockUsePipelineSASTResults.mockReturnValue({ data: { items: [blocked], total: 1 } });

      openHistory();

      const calls = mockUsePipelineSASTResults.mock.calls;
      expect(calls[calls.length - 1]).toEqual(["pipeline-1", 1234]);
    });

    it("이어진 실행 행에만 판정을 붙이고, 선택한 실행에 걸린 조건과 지표를 보여준다", () => {
      mockUsePipelineSASTResults.mockReturnValue({ data: { items: [blocked], total: 1 } });

      openHistory();

      expect(within(rowOf("v0.1.2")).getByText("Blocked by Quality Gate")).toBeTruthy();
      expect(within(rowOf("v0.1.1")).queryByText("SAST")).toBeNull();
      expect(screen.getByText("Static analysis (SonarQube)")).toBeTruthy();
      expect(screen.getByRole("list", { name: "Failed conditions" }).textContent).toContain("New issues");
      expect(screen.getByLabelText("Vulnerabilities 3")).toBeTruthy();
      expect(screen.getByRole("link", { name: /Open in SonarQube/ })).toBeTruthy();
    });

    // 미배선(503)이어도 이력 탭은 그대로 뜬다.
    it("분석 결과 조회가 실패해도 이력은 그대로 보인다", () => {
      mockUsePipelineSASTResults.mockReturnValue({ data: undefined, isError: true });

      openHistory();

      expect(screen.getByRole("button", { name: "v0.1.2" })).toBeTruthy();
      expect(screen.queryByText("Static analysis (SonarQube)")).toBeNull();
    });
  });

  // 의존성 스캔 · 시크릿 탐지(Security Dashboard 설계 §8 G1 · G2)도 이미지 스캔 · SAST 옆에
  // 붙는다. 시크릿은 탐지되면 "폐기 · 회전" 이 먼저다 — 경고 정책이어도 이미 유출이다.
  describe("실행 이력의 의존성 스캔 · 시크릿 탐지", () => {
    const deployments = [
      {
        id: "dep_ci_pip_x_2",
        pipelineId: "pipeline-1",
        pipelineName: "frontend-web",
        version: "v0.1.2",
        status: "failed",
        triggeredBy: "kim.dev",
        startedAt: "2026-10-20T01:20:00Z",
        completedAt: "2026-10-20T01:30:00Z",
      },
      {
        id: "dep_ci_pip_x_1",
        pipelineId: "pipeline-1",
        pipelineName: "frontend-web",
        version: "v0.1.1",
        status: "success",
        triggeredBy: "kim.dev",
        startedAt: "2026-10-19T13:20:00Z",
        completedAt: "2026-10-19T13:21:00Z",
      },
    ];

    const secretScan = {
      id: "secret_dep_ci_pip_x_2",
      pipelineId: "pipeline-1",
      deploymentId: "dep_ci_pip_x_2",
      scanner: "gitleaks",
      scannerVersion: "8.21.0",
      findingsCount: 2,
      rules: [
        { ruleId: "github-pat", count: 1 },
        { ruleId: "aws-access-key", count: 1 },
      ],
      findings: [
        {
          ruleId: "github-pat",
          file: "src/config.ts",
          startLine: 12,
          commit: "abcdef1234567890",
          fingerprint: "abcdef1234567890:src/config.ts:github-pat:12",
        },
      ],
      gateResult: "warn",
      scannedAt: "2026-10-20T01:21:00Z",
    };

    const dependencyScan = {
      id: "depscan_dep_ci_pip_x_1",
      pipelineId: "pipeline-1",
      deploymentId: "dep_ci_pip_x_1",
      scanner: "trivy",
      dbStale: false,
      counts: { critical: 0, high: 0, medium: 0, low: 0, unknown: 0 },
      targets: 0,
      gateResult: "pass",
      scannedAt: "2026-10-19T13:21:00Z",
    };

    function rowOf(version: string) {
      return screen.getByRole("button", { name: version }).parentElement!;
    }

    beforeEach(() => {
      mockUsePipelineDeployments.mockReturnValue({
        data: { items: deployments, total: deployments.length },
        isLoading: false,
        dataUpdatedAt: 4321,
      });
      mockUsePipelineSecretScans.mockReturnValue({ data: { items: [secretScan], total: 1 } });
      mockUsePipelineDependencyScans.mockReturnValue({ data: { items: [dependencyScan], total: 1 } });
    });

    it("should query both results with the deployments refresh time", () => {
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /^History$/ }));

      const dep = mockUsePipelineDependencyScans.mock.calls;
      const sec = mockUsePipelineSecretScans.mock.calls;
      expect(dep[dep.length - 1]).toEqual(["pipeline-1", 4321]);
      expect(sec[sec.length - 1]).toEqual(["pipeline-1", 4321]);
    });

    it("should show the secret summary on its run and the rotate warning in the detail", () => {
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /^History$/ }));

      const row = rowOf("v0.1.2");
      expect(within(row).getByText("Secrets found (warn)")).toBeTruthy();
      expect(within(row).getAllByText("2 found").length).toBeGreaterThan(0);
      // 경고 정책이어도 유출이다 — 폐기 · 회전 안내가 먼저 보인다.
      expect(screen.getByRole("alert").textContent).toContain(
        "Committed secrets are already leaked — revoke and rotate them now.",
      );
      expect(screen.getByText("src/config.ts:12")).toBeTruthy();
      expect(screen.getByText("Secret values are never stored or shown — only where they were found.")).toBeTruthy();
    });

    // 락파일이 없어 아무것도 검사하지 않은 통과는 초록이 아니다.
    it("should show a zero-target dependency pass as No targets, not as passed", () => {
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /^History$/ }));

      const row = rowOf("v0.1.1");
      expect(within(row).getByText("No targets")).toBeTruthy();
      expect(within(row).queryByText("Passed")).toBeNull();
      expect(within(row).queryByLabelText(/Critical/)).toBeNull();
    });

    it("should keep the history visible when the new results fail to load", () => {
      mockUsePipelineSecretScans.mockReturnValue({ data: undefined, isError: true });
      mockUsePipelineDependencyScans.mockReturnValue({ data: undefined, isError: true });
      renderWithProviders(<CicdListPage />);
      fireEvent.click(screen.getByRole("button", { name: /^History$/ }));

      expect(screen.getByRole("button", { name: "v0.1.2" })).toBeTruthy();
      expect(screen.queryByText("Secret detection (gitleaks)")).toBeNull();
    });
  });

  // 보안 대시보드가 "이 앱의 실행 기록" 으로 보낸다(설계 §8 G5). 파이프라인 · 탭 · 실행이
  // URL 로 열려야 한다.
  describe("딥링크", () => {
    const otherPipeline = {
      ...pipelines[0],
      id: "pipeline-2",
      name: "order-api",
    };
    const deployments = [
      {
        id: "dep_new",
        pipelineId: "pipeline-2",
        pipelineName: "order-api",
        version: "#42",
        status: "failed",
        triggeredBy: "kim.dev",
        startedAt: "2026-10-20T01:20:00Z",
        completedAt: "2026-10-20T01:30:00Z",
      },
      {
        id: "dep_old",
        pipelineId: "pipeline-2",
        pipelineName: "order-api",
        version: "#38",
        status: "success",
        triggeredBy: "kim.dev",
        startedAt: "2026-10-19T13:20:00Z",
        completedAt: "2026-10-19T13:21:00Z",
      },
    ];

    beforeEach(() => {
      mockUsePipelines.mockReturnValue({
        data: { items: [pipelines[0], otherPipeline], total: 2 },
        isLoading: false,
      });
      mockUsePipelineDeployments.mockReturnValue({
        data: { items: deployments, total: deployments.length },
        isLoading: false,
        dataUpdatedAt: 99,
      });
    });

    it("should open the linked pipeline, history tab and run from the URL", async () => {
      renderWithProviders(<CicdListPage />, {
        route: "/cicd/list?pipeline=pipeline-2&tab=history&run=dep_old",
      });

      // 첫 파이프라인이 아니라 링크가 가리킨 파이프라인이 열린다.
      expect(screen.getByRole("heading", { name: "order-api" })).toBeTruthy();
      expect(screen.getByRole("button", { name: /^History$/ }).getAttribute("aria-pressed")).toBe("true");
      // 최신 실행이 아니라 링크가 가리킨 실행(#38)의 상세가 열린다.
      await waitFor(() => expect(screen.getByText("dep_old")).toBeTruthy());
      expect(screen.queryByText("dep_new")).toBeNull();
    });

    it("should fall back to the latest run when the linked run is not listed", async () => {
      renderWithProviders(<CicdListPage />, {
        route: "/cicd/list?pipeline=pipeline-2&tab=history&run=dep_gone",
      });

      await waitFor(() => expect(screen.getByText("dep_new")).toBeTruthy());
    });

    it("should ignore an unknown tab value", () => {
      renderWithProviders(<CicdListPage />, { route: "/cicd/list?pipeline=pipeline-2&tab=bogus" });

      expect(screen.getByRole("heading", { name: "order-api" })).toBeTruthy();
      expect(screen.getByRole("button", { name: /^History$/ }).getAttribute("aria-pressed")).toBe("false");
    });
  });

  // 실행은 파이프라인에 정해진 클러스터로 간다 — 고른 값을 서버에 보내지 않는다.
  // 고를 수 있게 두면 사용자는 고른 클러스터에 배포된 줄 안다. 스택 파이프라인은
  // 스택의 Argo CD 가 스택 클러스터에 올리므로 더욱 그렇다.
  it("실행 확인 창은 배포될 클러스터를 고르게 하지 않고 보여 준다", () => {
    // 지운 선택지의 가짜 보기(c1/prod-k8s)와 겹치지 않는 이름을 쓴다.
    mockUsePipelines.mockReturnValue({
      data: {
        items: [{ ...pipelines[0], clusterId: "c-stack", clusterName: "stack-cluster-a" }],
        total: 1,
      },
      isLoading: false,
    });
    renderWithProviders(<CicdListPage />);

    fireEvent.click(screen.getByRole("button", { name: "Execute" }));

    const dialog = screen.getByRole("dialog", { name: "Execute Pipeline" });
    expect(within(dialog).queryByRole("combobox")).toBeNull();
    expect(within(dialog).getByText("stack-cluster-a")).toBeTruthy();
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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import {
  optionLabels,
  renderWithProviders,
  selectOptionByValue,
  selectedLabel,
} from "../../../__tests__/test-utils";
import { DeveloperDeployPage } from "./developer-deploy-page";

const mockNavigate = vi.fn();
const mockUseCicdTemplates = vi.fn();
const mockUseCreatePipeline = vi.fn();
const mockUseClusters = vi.fn();
const mockUseClusterNamespaces = vi.fn();
const mockUseStacks = vi.fn();
const mockUseStackIntegrations = vi.fn();
const mockCreatePipeline = vi.fn();
const mockUseDeployPipeline = vi.fn();
const mockDeployPipeline = vi.fn();
const mockUseRepositoryLocation = vi.fn();

vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock("../api/cicd-api", () => ({
  useCicdTemplates: () => mockUseCicdTemplates(),
  useCreatePipeline: () => mockUseCreatePipeline(),
  useDeployPipeline: () => mockUseDeployPipeline(),
  useRepositoryLocation: (stackId: string) => mockUseRepositoryLocation(stackId),
}));

vi.mock("../../admin/api/admin-api", () => ({
  useClusters: () => mockUseClusters(),
  useClusterNamespaces: () => mockUseClusterNamespaces(),
}));

vi.mock("../../stack/api/stack-api", () => ({
  useStacks: () => mockUseStacks(),
  useStackIntegrations: (stackId: string) => mockUseStackIntegrations(stackId),
}));

vi.mock("../../../components/shared/code-preview", () => ({
  CodePreview: ({ title, code }: { title: string; code: string }) => (
    <div>
      <div>{title}</div>
      <pre>{code}</pre>
    </div>
  ),
}));

const templates = [
  {
    id: "starter-v1",
    name: "Starter Template",
    description: "Starter pipeline",
    appType: "backend",
    gitRepoUrl: "https://github.com/cloud-nullus/starter.git",
    dockerfilePath: "Dockerfile",
    dockerContext: ".",
    envVars: {},
  },
];

const clusters = {
  items: [{ id: "c1", name: "prod-k8s", types: ["target"] }],
  total: 1,
};

// 제출 버튼. "Create" 는 섹션 헤더 이름이기도 해서 역할+이름으로 찾으면
// 헤더가 함께 잡힌다 — 제출 버튼을 아는 곳을 여기 하나로 둔다.
function submitButton() {
  return screen.getByRole("button", { name: "Execute" });
}

function completeRequiredFields() {
  fireEvent.change(screen.getByPlaceholderText("my-awesome-app"), {
    target: { value: "demo-app" },
  });
  selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");
  fireEvent.change(screen.getByLabelText("Source Repository"), {
    target: { value: "owner/demo-app.git" },
  });
}

describe("DeveloperDeployPage", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    mockNavigate.mockReset();
    mockUseCicdTemplates.mockReset();
    mockUseCreatePipeline.mockReset();
    mockUseClusters.mockReset();
    mockUseClusterNamespaces.mockReset();
    mockUseStacks.mockReset();
    mockUseStackIntegrations.mockReset();
    mockCreatePipeline.mockReset();

    mockUseCicdTemplates.mockReturnValue({ data: templates });
    mockUseRepositoryLocation.mockReset();
    mockUseRepositoryLocation.mockReturnValue({
      data: { platform: "gitlab", groupPath: "nullus", webBaseUrl: "https://gitlab.stack.internal" },
    });
    mockUseCreatePipeline.mockReturnValue({
      mutateAsync: mockCreatePipeline,
      isPending: false,
    });
    mockCreatePipeline.mockResolvedValue({ id: "pipeline-1" });
    mockUseDeployPipeline.mockReturnValue({
      mutateAsync: mockDeployPipeline,
      isPending: false,
    });
    mockDeployPipeline.mockResolvedValue({ deploymentId: "deployment-1" });
    mockUseClusters.mockReturnValue({ data: clusters });
    mockUseClusterNamespaces.mockReturnValue({ data: [] });
    mockUseStacks.mockReturnValue({
      data: { items: [{ id: "stack-1", name: "app-stack" }], total: 1 },
    });
    mockUseStackIntegrations.mockReturnValue({
      data: {
        integrations: [
          {
            component_type: "code_repository",
            endpoint: "https://gitlab.stack.internal",
          },
          {
            component_type: "package_registry",
            endpoint: "https://artifactory.stack.internal",
          },
          {
            component_type: "image_registry",
            endpoint: "https://registry.stack.internal",
          },
        ],
      },
    });
  });

  it("renders all six pipeline sections in one scrollable page", async () => {
    renderWithProviders(<DeveloperDeployPage />);

    expect(
      await screen.findByRole("heading", { name: "Pipeline Setup" }),
    ).not.toBeNull();
    expect(screen.getByText("Enter App Name")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Basic Info" })).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Code Checkout" }),
    ).not.toBeNull();
    expect(screen.getByRole("button", { name: "Build" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Test" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Security" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Create" })).not.toBeNull();
    expect(screen.getByText("Build Configuration")).not.toBeNull();
    expect(screen.getByText("Deploy Configuration")).not.toBeNull();
    expect(screen.getByText("1. Code Checkout")).not.toBeNull();
    expect(screen.getByText("2. Build")).not.toBeNull();
    expect(screen.getByText("3. Image Registry")).not.toBeNull();
    expect(screen.queryByText("3. Deploy")).toBeNull();
    expect(screen.getByText("Cluster & Namespace")).not.toBeNull();
    expect(screen.queryByText(/Quick Start/)).toBeNull();
    expect(screen.queryByRole("button", { name: "Next" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Previous" })).toBeNull();
  });

  it("keeps template-prefilled values across pipeline pages", async () => {
    renderWithProviders(<DeveloperDeployPage />, {
      route: "/cicd/developer-deploy?template=starter-v1",
    });

    await waitFor(() => {
      expect(screen.getByPlaceholderText("my-awesome-app")).toHaveValue(
        "starter",
      );
    });
    // 스택에 묶이면 저장소는 서버가 <그룹>/<앱 이름> 으로 만든다 — 그 주소를 보여 준다.
    expect(screen.getByLabelText("Source Repository")).toHaveValue(
      "https://gitlab.stack.internal/nullus/starter",
    );
  });

  it("should tell the user that a source template copies its source into the new repository", async () => {
    mockUseCicdTemplates.mockReturnValue({
      data: [
        ...templates,
        {
          id: "egovframe-boot-sample-v1",
          name: "eGovFrame Boot Sample (Spring Boot)",
          description: "",
          appType: "backend",
          stages: ["Build", "Deploy"],
          source: {
            repoUrl: "https://github.com/eGovFramework/egovframe-boot-sample-java-config",
            commit: "c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb",
            label: "v5.0.0",
            buildProfile: "spring-boot",
          },
        },
      ],
    });
    renderWithProviders(<DeveloperDeployPage />, {
      route: "/cicd/developer-deploy?template=egovframe-boot-sample-v1",
    });

    expect(await screen.findByText("Imports the template source")).not.toBeNull();
    expect(
      screen.getByText(
        "Creating the pipeline copies eGovFramework/egovframe-boot-sample-java-config (v5.0.0) into the new repository as its first commit. The repository does not follow later changes to the source. Requires internet access.",
      ),
    ).not.toBeNull();
  });

  it("should not show the source notice for templates without a source", async () => {
    renderWithProviders(<DeveloperDeployPage />, {
      route: "/cicd/developer-deploy?template=starter-v1",
    });
    await waitFor(() => {
      expect(screen.getByPlaceholderText("my-awesome-app")).toHaveValue("starter");
    });
    expect(screen.queryByText("Imports the template source")).toBeNull();
  });

  it("selects the first stack and shows where the repository will be created", async () => {
    renderWithProviders(<DeveloperDeployPage />);

    await waitFor(() => {
      expect(selectedLabel(screen.getByLabelText("Select Stack"))).toBe(
        "app-stack",
      );
    });
    // 앱 이름을 넣기 전에는 자리만 보여 준다.
    expect(screen.getByLabelText("Source Repository")).toHaveValue("");
    expect(screen.getByLabelText("Source Repository")).toHaveAttribute(
      "placeholder",
      "https://gitlab.stack.internal/nullus/<app>",
    );
    expect(screen.getByLabelText("Deploy YAML Repository")).toHaveValue(
      "root/spring-sample/deploy",
    );
  });

  it("selects the first stack and fills stack path defaults when the repository location is unknown", async () => {
    mockUseRepositoryLocation.mockReturnValue({ data: undefined });
    renderWithProviders(<DeveloperDeployPage />);

    await waitFor(() => {
      expect(selectedLabel(screen.getByLabelText("Select Stack"))).toBe(
        "app-stack",
      );
    });
    expect(optionLabels(screen.getByLabelText("Select Stack"))).not.toContain(
      "Manual Input",
    );
    expect(screen.getByLabelText("Source Repository")).toHaveValue(
      "root/spring-sample.git",
    );
    expect(screen.getByLabelText("Dockerfile Repository")).toHaveValue(
      "root/spring-sample/-/blob/master/Dockerfile",
    );
    expect(screen.getByLabelText("Image Registry")).toHaveValue(
      "root/spring-sample",
    );
    expect(screen.getByLabelText("Deploy YAML Repository")).toHaveValue(
      "root/spring-sample/deploy",
    );
  });

  it("shows every input section and pending review before required values are complete", () => {
    renderWithProviders(<DeveloperDeployPage />);

    expect(screen.getByText("Build Configuration")).not.toBeNull();
    expect(screen.getByText("Cluster & Namespace")).not.toBeNull();
    expect(
      screen.getByText(
        "Complete required fields to preview deployment manifests.",
      ),
    ).not.toBeNull();
  });

  it("shows required markers and marks Stack selection as required", () => {
    renderWithProviders(<DeveloperDeployPage />);

    expect(screen.getAllByTestId("required-dot")).toHaveLength(12);
    expect(screen.getByLabelText("Select Stack")).not.toBeNull();
    expect(screen.queryByLabelText("Stack (Optional)")).toBeNull();
    expect(screen.getByLabelText("Service URL")).not.toBeNull();
  });

  it("allows exactly one phase selection and shows configuration groups by capability", () => {
    renderWithProviders(<DeveloperDeployPage />);

    const testCapability = screen
      .getAllByLabelText("Test")
      .find((item) => item.tagName === "INPUT");
    const securityCapability = screen
      .getAllByLabelText("Security")
      .find((item) => item.tagName === "INPUT");

    expect(screen.getByLabelText("CI")).toBeChecked();
    expect(screen.getByLabelText("CD")).toBeChecked();
    expect(testCapability).not.toBeChecked();
    expect(testCapability).toBeDisabled();
    expect(securityCapability).not.toBeChecked();
    expect(securityCapability).toBeDisabled();
    expect(screen.getByLabelText("production")).toBeChecked();
    expect(screen.getByLabelText("qa")).not.toBeChecked();
    expect(screen.getByLabelText("development")).not.toBeChecked();
    fireEvent.click(screen.getByLabelText("qa"));
    expect(screen.getByLabelText("production")).not.toBeChecked();
    expect(screen.getByLabelText("qa")).toBeChecked();
    fireEvent.click(screen.getByLabelText("qa"));
    expect(screen.getByLabelText("qa")).toBeChecked();
    fireEvent.click(screen.getByLabelText("development"));
    expect(screen.getByLabelText("qa")).not.toBeChecked();
    expect(screen.getByLabelText("development")).toBeChecked();

    expect(screen.getByLabelText("Source Repository")).not.toBeNull();
    expect(screen.getByLabelText("Dockerfile Repository")).not.toBeNull();
    expect(screen.getByLabelText("Image Registry")).not.toBeNull();
    expect(screen.getByLabelText("Deploy YAML Repository")).not.toBeNull();

    fireEvent.click(screen.getByLabelText("CI"));
    expect(screen.queryByText("1. Code Checkout")).toBeNull();
    expect(screen.queryByText("2. Build")).toBeNull();
    expect(screen.queryByText("3. Image Registry")).toBeNull();
    expect(screen.queryByLabelText("Source Repository")).toBeNull();
    expect(screen.queryByText("3. Deploy")).toBeNull();
    expect(screen.getByLabelText("Deploy YAML Repository")).not.toBeNull();

    fireEvent.click(screen.getByLabelText("CD"));
    expect(screen.queryByText("3. Deploy")).toBeNull();
    expect(screen.queryByLabelText("Deploy YAML Repository")).toBeNull();
  });

  it("shows the generated manifests in review and creates then deploys", async () => {
    renderWithProviders(<DeveloperDeployPage />);

    completeRequiredFields();

    expect(await screen.findByText("demo-app-deployment.yaml")).not.toBeNull();
    expect(screen.getByText("demo-app-service.yaml")).not.toBeNull();
    expect(screen.getByText("demo-app-ingress.yaml")).not.toBeNull();
    fireEvent.click(submitButton());
    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalled();
      // 생성 직후 배포까지 실행하고 그 배포의 로그로 보낸다.
      expect(mockDeployPipeline).toHaveBeenCalledWith({
        pipelineId: "pipeline-1",
      });
      expect(mockNavigate).toHaveBeenCalledWith(
        "/cicd/pipelines/pipeline-1/logs?deploymentId=deployment-1",
      );
    });
  });

  // 저장소 자리를 모르는 구성(조회 API 가 없거나 실패)에서는 예전처럼 경로를 입력받는다.
  it("uses the selected stacks code repository endpoint as the code URL prefix when the repository location is unknown", () => {
    mockUseRepositoryLocation.mockReturnValue({ data: undefined });
    renderWithProviders(<DeveloperDeployPage />);

    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");
    fireEvent.change(screen.getByLabelText("Source Repository"), {
      target: { value: "owner/demo-app/main/src" },
    });
    fireEvent.change(
      screen.getByPlaceholderText(
        "root/spring-sample/-/blob/master/Dockerfile",
      ),
      { target: { value: "owner/demo-app/main/docker" } },
    );
    fireEvent.change(screen.getByPlaceholderText("root/spring-sample"), {
      target: { value: "owner/demo-app/main/image" },
    });

    expect(
      screen.getAllByDisplayValue("https://gitlab.stack.internal/"),
    ).toHaveLength(2);
    screen
      .getAllByDisplayValue("https://gitlab.stack.internal/")
      .forEach((input) => {
        expect(input).toBeDisabled();
      });
    expect(
      screen.getByDisplayValue("https://registry.stack.internal/"),
    ).toBeDisabled();
    expect(
      screen.getByDisplayValue("https://artifactory.stack.internal/"),
    ).toBeDisabled();
    expect(screen.getByLabelText("Source Repository")).toHaveValue(
      "owner/demo-app/main/src",
    );
    expect(screen.getByLabelText("Dockerfile Repository")).toHaveValue(
      "owner/demo-app/main/docker",
    );
    expect(screen.getByLabelText("Image Registry")).toHaveValue(
      "owner/demo-app/main/image",
    );
    expect(screen.getByLabelText("Deploy YAML Repository")).toHaveValue(
      "root/spring-sample/deploy",
    );
  });

  it("allows review and create with a repository URL assembled from the selected stack when the repository location is unknown", async () => {
    mockUseRepositoryLocation.mockReturnValue({ data: undefined });
    renderWithProviders(<DeveloperDeployPage />);

    fireEvent.change(screen.getByPlaceholderText("my-awesome-app"), {
      target: { value: "demo-app" },
    });
    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");
    fireEvent.change(screen.getByLabelText("Source Repository"), {
      target: { value: "owner/demo-app.git" },
    });

    expect(await screen.findByText("demo-app-deployment.yaml")).not.toBeNull();
    fireEvent.click(submitButton());
    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalledWith(
        expect.objectContaining({
          gitRepoUrl: "https://gitlab.stack.internal/owner/demo-app.git",
          stackId: "stack-1",
        }),
      );
    });
  });

  // 스택 모드에서는 저장소 경로를 입력받지 않는다. 서버는 그 값을 쓰지 않고 언제나 <그룹>/<앱 이름> 으로
  // 만들었다 — 입력한 주소와 실제 저장소가 달랐다. 실제로 만들어질 주소를 보여 준다.
  it("shows the repository the pipeline will create instead of asking for a path", () => {
    renderWithProviders(<DeveloperDeployPage />);

    fireEvent.change(screen.getByPlaceholderText("my-awesome-app"), {
      target: { value: "demo-app" },
    });
    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");

    expect(mockUseRepositoryLocation).toHaveBeenCalledWith("stack-1");
    const source = screen.getByLabelText("Source Repository");
    expect(source).toHaveValue("https://gitlab.stack.internal/nullus/demo-app");
    expect(source).toHaveAttribute("readonly");
    expect(
      screen.getByText(
        "Creating the pipeline creates this repository. If a repository with that name already exists, it is used as is.",
      ),
    ).not.toBeNull();
    expect(screen.getByLabelText("Dockerfile Repository")).toHaveValue(
      "Dockerfile at the repository root",
    );
    expect(screen.getByLabelText("Image Registry")).toHaveValue(
      "Chosen by the platform in the stack registry",
    );
    expect(screen.getByLabelText("Dockerfile Repository")).toHaveAttribute("readonly");
    expect(screen.getByLabelText("Image Registry")).toHaveAttribute("readonly");
  });

  it("creates the pipeline with the repository the platform will make", async () => {
    renderWithProviders(<DeveloperDeployPage />);

    fireEvent.change(screen.getByPlaceholderText("my-awesome-app"), {
      target: { value: "demo-app" },
    });
    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");

    expect(await screen.findByText("demo-app-deployment.yaml")).not.toBeNull();
    fireEvent.click(submitButton());
    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalled();
    });
    const request = mockCreatePipeline.mock.calls[0][0];
    expect(request.gitRepoUrl).toBe("https://gitlab.stack.internal/nullus/demo-app");
    expect(request.stackId).toBe("stack-1");
    // 쓰이지 않는 값은 보내지 않는다 — Dockerfile 은 저장소 루트, 이미지 경로는 서버가 레지스트리로 정한다.
    expect(request.dockerfilePath).toBe("");
    expect(request.envVars).not.toHaveProperty("IMAGE_REGISTRY_URL");
  });

  // 스택에 묶인 파이프라인의 앱은 스택의 Argo CD 가 스택 클러스터에 올린다.
  // 다른 클러스터를 고르게 두면 앱은 스택 클러스터에 뜨는데 화면은 고른
  // 클러스터를 보여 준다 — 모니터링 탭이 비어 보이던 원인이다.
  it("locks the cluster to the stack's cluster when a stack is selected", async () => {
    mockUseClusters.mockReturnValue({
      data: {
        items: [
          { id: "c1", name: "prod-k8s", types: ["target"] },
          // 스택 클러스터는 target 타입이 아니어도 고정 대상이다.
          { id: "c-stack", name: "stack-k8s", types: ["pipeline"] },
        ],
        total: 2,
      },
    });
    mockUseStacks.mockReturnValue({
      data: {
        items: [{ id: "stack-1", name: "app-stack", clusterId: "c-stack" }],
        total: 1,
      },
    });
    renderWithProviders(<DeveloperDeployPage />);

    const clusterSelect = screen.getByLabelText("Cluster");
    await waitFor(() => {
      expect(selectedLabel(clusterSelect)).toBe("stack-k8s");
    });
    // MUI 콤보박스는 disabled 속성 대신 aria-disabled 로 알린다.
    expect(clusterSelect).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByText(
        "Fixed to the cluster the stack is installed on. The stack's Argo CD deploys the app there.",
      ),
    ).not.toBeNull();

    completeRequiredFields();
    expect(await screen.findByText("demo-app-deployment.yaml")).not.toBeNull();
    fireEvent.click(submitButton());
    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalledWith(
        expect.objectContaining({ clusterId: "c-stack", stackId: "stack-1" }),
      );
    });
  });

  it("leaves the cluster choice open when the selected stack has no cluster", async () => {
    renderWithProviders(<DeveloperDeployPage />);

    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");

    const clusterSelect = screen.getByLabelText("Cluster");
    await waitFor(() => {
      expect(selectedLabel(clusterSelect)).toBe("prod-k8s");
    });
    expect(clusterSelect).not.toHaveAttribute("aria-disabled", "true");
  });

  it("keeps manual code URL input visible when the selected stack has no repository endpoint", () => {
    mockUseStackIntegrations.mockReturnValue({ data: { integrations: [] } });
    renderWithProviders(<DeveloperDeployPage />);

    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");

    expect(screen.getByLabelText("Source Repository")).not.toBeNull();
  });

  it("uses text fields instead of slider controls for resources", () => {
    renderWithProviders(<DeveloperDeployPage />);

    expect(screen.getByLabelText("CPU Request")).toHaveValue("100m");
    expect(screen.getByLabelText("CPU Limit")).toHaveValue("500m");
    expect(screen.getByLabelText("Memory Request")).toHaveValue("128Mi");
    expect(screen.getByLabelText("Memory Limit")).toHaveValue("512Mi");
    expect(screen.queryByRole("slider")).toBeNull();
  });

  it("groups code checkout, build, image registry, and deploy repository inputs in order", () => {
    renderWithProviders(<DeveloperDeployPage />);

    const source = screen.getByLabelText("Source Repository");
    const dockerfile = screen.getByLabelText("Dockerfile Repository");
    const imageRegistry = screen.getByLabelText("Image Registry");
    const deployYaml = screen.getByLabelText("Deploy YAML Repository");

    expect(
      source.compareDocumentPosition(dockerfile) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      dockerfile.compareDocumentPosition(imageRegistry) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      imageRegistry.compareDocumentPosition(deployYaml) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryAllByLabelText("Branch")).toHaveLength(0);
    expect(screen.queryAllByLabelText("Directory")).toHaveLength(0);
  });

  it("shows separate manifests and enables create without an optional service URL", async () => {
    renderWithProviders(<DeveloperDeployPage />);

    expect(screen.queryByText("Review Manifest")).toBeNull();
    completeRequiredFields();

    expect(await screen.findByText("Review Manifest")).not.toBeNull();
    expect(screen.getByText("demo-app-deployment.yaml")).not.toBeNull();
    expect(screen.getByText("demo-app-service.yaml")).not.toBeNull();
    expect(screen.getByText("demo-app-ingress.yaml")).not.toBeNull();
    expect(screen.getByText(/host: demo-app\.internal/)).not.toBeNull();
    expect(
      screen.getAllByRole("button", { name: "Create" }).at(-1)!,
    ).not.toBeDisabled();
  });

  it("uses a provided optional service URL as the ingress host", async () => {
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();
    fireEvent.change(screen.getByPlaceholderText("app.example.com"), {
      target: { value: "https://demo-app.example.com/health" },
    });

    expect(
      await screen.findByText(/host: demo-app\.example\.com/),
    ).not.toBeNull();
  });

  it("loads manifests from the Deploy YAML Repository directory", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((url: URL) => {
        const href = String(url);
        const body = href.endsWith("/deployment.yaml")
          ? "kind: Deployment\nmetadata:\n  name: imported\n"
          : href.endsWith("/service.yaml")
            ? "kind: Service\nmetadata:\n  name: imported-svc\n"
            : "kind: Ingress\nmetadata:\n  name: imported-ingress\n";
        return Promise.resolve({
          ok: true,
          text: () => Promise.resolve(body),
        });
      }),
    );
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();

    fireEvent.click(
      screen.getByRole("button", { name: "Reload Deploy YAML files" }),
    );

    expect(await screen.findByText(/name: imported$/)).not.toBeNull();
    expect(screen.getByText(/name: imported-svc/)).not.toBeNull();
    expect(screen.getByText(/name: imported-ingress/)).not.toBeNull();
    expect(fetch).toHaveBeenCalledWith(
      new URL(
        "root/spring-sample/deploy/deployment.yaml",
        "https://artifactory.stack.internal/",
      ),
    );
  });

  it("shows network connection error when manifest fetch fails with network exception", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.reject(new TypeError("Failed to fetch"))),
    );
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();

    fireEvent.click(
      screen.getByRole("button", { name: "Reload Deploy YAML files" }),
    );

    expect(
      await screen.findByText(
        "저장소에 연결할 수 없습니다. 기준 URL 또는 도메인 해석(DNS/hosts), 네트워크 연결 상태를 확인하세요.",
      ),
    ).not.toBeNull();
  });

  it("shows 404 not found error when manifest files are not found", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => {
        return Promise.resolve({
          ok: false,
          status: 404,
          statusText: "Not Found",
          text: () => Promise.resolve("404 page not found"),
        });
      }),
    );
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();

    fireEvent.click(
      screen.getByRole("button", { name: "Reload Deploy YAML files" }),
    );

    expect(
      await screen.findByText(
        /해당 directory에서 .*을 찾을 수 없습니다 \(404\)\. 경로와 파일명을 확인하세요\./,
      ),
    ).not.toBeNull();
  });

  it("navigates to the deployment logs after creating from manifest review", async () => {
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();

    fireEvent.click(submitButton());

    await waitFor(() => {
      expect(mockCreatePipeline).toHaveBeenCalledWith(
        expect.objectContaining({
          envVars: expect.objectContaining({
            NULLUS_MANIFEST_DEPLOYMENT:
              expect.stringContaining("kind: Deployment"),
            NULLUS_MANIFEST_SERVICE: expect.stringContaining("kind: Service"),
            NULLUS_MANIFEST_INGRESS: expect.stringContaining("kind: Ingress"),
          }),
        }),
      );
      expect(mockNavigate).toHaveBeenCalledWith(
        "/cicd/pipelines/pipeline-1/logs?deploymentId=deployment-1",
      );
    });
  });

  it("shows the create error when pipeline creation fails", async () => {
    mockCreatePipeline.mockRejectedValueOnce({
      message: "stack belongs to a different organization",
    });
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();

    fireEvent.click(submitButton());

    expect(
      await screen.findByText(
        "선택한 Stack에 접근할 수 없습니다. 현재 조직의 Stack을 다시 선택하거나 관리자에게 권한을 요청하세요.",
      ),
    ).not.toBeNull();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("includes the backend error for unmapped create failures", async () => {
    mockCreatePipeline.mockRejectedValueOnce({
      message: "create pipeline: insert pipeline failed",
    });
    renderWithProviders(<DeveloperDeployPage />);
    completeRequiredFields();

    fireEvent.click(submitButton());

    expect(
      await screen.findByText(
        "Pipeline을 생성하지 못했습니다. 관리자에게 아래 오류를 전달하세요: create pipeline: insert pipeline failed",
      ),
    ).not.toBeNull();
  });
  // 서버가 `integrations` 키 없이 응답하면 화면이 통째로 죽었다.
  // 읽는 쪽이 `integrationsData?.integrations.find(...)` 로 **중간만** 옵셔널
  // 체이닝해서, 응답 객체는 있고 그 키만 없을 때 TypeError 를 던졌다.
  //
  // developer 가 로그인하면 바로 오는 화면이라 흰 화면이 그대로 첫인상이 된다.
  // 설치가 끝나지 않은 스택, 프록시가 돌려주는 다른 모양의 본문에서 실제로 난다.
  it.each([
    ["integrations 키가 없을 때", {}],
    ["integrations 가 null 일 때", { integrations: null }],
    ["응답이 빈 문자열일 때", ""],
  ])("스택 연동 응답이 모양을 어겨도 화면은 뜬다 — %s", async (_name, data) => {
    mockUseStackIntegrations.mockReturnValue({ data });

    renderWithProviders(<DeveloperDeployPage />);

    // 에러 경계로 떨어지지 않고 폼이 그대로 뜬다.
    expect(screen.getByPlaceholderText("my-awesome-app")).not.toBeNull();
    expect(screen.getByLabelText("Select Stack")).not.toBeNull();
  });

  // 연동을 못 읽으면 주소 앞단만 비고 나머지는 평소대로 쓴다.
  // (주소가 제대로 올 때 앞단이 채워지는 것은 위의 "prefills ..." 테스트가 지킨다.)
  it("연동을 못 읽으면 스택 주소 앞단이 비고 화면은 평소대로다", () => {
    mockUseStackIntegrations.mockReturnValue({ data: {} });

    renderWithProviders(<DeveloperDeployPage />);
    selectOptionByValue(screen.getByLabelText("Select Stack"), "stack-1");

    expect(screen.queryAllByDisplayValue("https://gitlab.stack.internal/")).toHaveLength(0);
    expect(screen.getByLabelText("Source Repository")).not.toBeNull();
  });
});

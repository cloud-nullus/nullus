import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { CicdTemplatePage } from "./cicd-template-page";

const mockNavigate = vi.fn();
const mockUseAuthStore = vi.fn();
const mockUseCicdTemplates = vi.fn();
const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();
const mockDeleteMutate = vi.fn();

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

vi.mock("../../../stores/auth-store", () => ({
  useAuthStore: (selector: (state: { role: string }) => unknown) =>
    mockUseAuthStore(selector),
}));

vi.mock("../api/cicd-api", () => ({
  useCicdTemplates: () => mockUseCicdTemplates(),
  useCreateCicdTemplate: () => ({ mutate: mockCreateMutate, isPending: false }),
  useUpdateCicdTemplate: () => ({ mutate: mockUpdateMutate, isPending: false }),
  useDeleteCicdTemplate: () => ({ mutate: mockDeleteMutate, isPending: false }),
}));

const templates = [
  {
    id: "web-backend-standard",
    name: "Standard Web Backend",
    description: "REST API 백엔드 서비스 템플릿",
    appType: "web-backend",
    stages: [
      "Git Clone",
      "Docker Build",
      "Image Load",
      "Deploy",
      "Test",
      "Security Scan",
    ],
    createdBy: "admin",
  },
  {
    id: "helm-release-v1",
    name: "Helm Release",
    description: "Helm based deployment",
    appType: "web",
    stages: ["Build", "HelmDeploy"],
    createdBy: "admin",
  },
  {
    id: "data-job-v1",
    name: "Data Job",
    description: "One time job",
    appType: "batch-job",
    stages: ["Build", "JobDeploy"],
    createdBy: "admin",
  },
  {
    id: "nightly-cronjob-v1",
    name: "Nightly CronJob",
    description: "Scheduled job",
    appType: "batch-job",
    stages: ["Build", "CronJobDeploy"],
    createdBy: "admin",
  },
  {
    id: "web-backend-v1",
    name: "Web Backend Pipeline",
    description: "Backend custom pipeline",
    appType: "backend",
    stages: ["CI", "CD"],
  },
  {
    id: "web-frontend-v1",
    name: "Web Frontend Pipeline",
    description: "Frontend custom pipeline",
    appType: "web",
    stages: ["CI", "CD"],
  },
];
const EMPTY_SEARCH_RESULT_LABEL =
  /No search results found\.|검색 결과가 없습니다\./;

describe("CicdTemplatePage", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockUseCicdTemplates.mockReset();
    mockCreateMutate.mockReset();
    mockUpdateMutate.mockReset();
    mockDeleteMutate.mockReset();

    mockUseAuthStore.mockImplementation(
      (selector: (state: { role: string }) => unknown) =>
        selector({ role: "admin" }),
    );
    mockUseCicdTemplates.mockReturnValue({ data: templates, isLoading: false });
  });

  it("renders loading state safely", () => {
    mockUseCicdTemplates.mockReturnValue({ data: undefined, isLoading: true });

    renderWithProviders(<CicdTemplatePage />);

    expect(screen.getAllByText("CI/CD Template").length).toBeGreaterThan(0);
    expect(screen.getByText(EMPTY_SEARCH_RESULT_LABEL)).not.toBeNull();
  });

  it("renders template cards from API data", () => {
    renderWithProviders(<CicdTemplatePage />);

    expect(screen.getByText("Standard Web Backend")).not.toBeNull();
    expect(screen.getByText("User Custom Pipeline")).not.toBeNull();
    expect(screen.queryByText("Web Frontend Pipeline")).toBeNull();
    expect(
      screen.getByText(
        /REST API backend service template|REST API 백엔드 서비스 템플릿/,
      ),
    ).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Create Template" }),
    ).not.toBeNull();
  });

  it("groups templates into CI/CD workload type sections", () => {
    renderWithProviders(<CicdTemplatePage />);

    expect(
      screen.getAllByText(/CI\/CD Type|CI\/CD 유형/).length,
    ).toBeGreaterThan(0);
    expect(screen.getByRole("heading", { name: "Default" })).not.toBeNull();
    expect(screen.getByRole("heading", { name: "Helm" })).not.toBeNull();
    expect(screen.getByRole("heading", { name: "Cronjob/Job" })).not.toBeNull();
    expect(screen.getByText("Helm Release")).not.toBeNull();
    expect(screen.getByText("Data Job")).not.toBeNull();
    expect(screen.getByText("Nightly CronJob")).not.toBeNull();
  });

  it("shows capabilities instead of low-level pipeline steps", () => {
    renderWithProviders(<CicdTemplatePage />);

    expect(screen.getAllByText("CI").length).toBeGreaterThan(0);
    expect(screen.getAllByText("CD").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Test").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Security").length).toBeGreaterThan(0);
    expect(screen.queryByText("Git Clone")).toBeNull();
    expect(screen.queryByText("Docker Build")).toBeNull();
    expect(screen.queryByText("Image Load")).toBeNull();
  });

  it("uses capability choices in the create template modal", () => {
    renderWithProviders(<CicdTemplatePage />);

    fireEvent.click(screen.getByRole("button", { name: "Create Template" }));

    expect(screen.getAllByText(/Capabilities|기능/).length).toBeGreaterThan(0);
    expect(screen.getByLabelText("CI")).not.toBeNull();
    expect(screen.getByLabelText("CD")).not.toBeNull();
    expect(screen.getByLabelText("Test")).not.toBeNull();
    expect(screen.getByLabelText("Security")).not.toBeNull();
  });

  it("renders empty state when template list is empty", () => {
    mockUseCicdTemplates.mockReturnValue({ data: [], isLoading: false });

    renderWithProviders(<CicdTemplatePage />);

    expect(screen.getByText(EMPTY_SEARCH_RESULT_LABEL)).not.toBeNull();
  });

  it("navigates to developer deploy when using base template", () => {
    renderWithProviders(<CicdTemplatePage />);

    fireEvent.click(
      screen.getAllByRole("button", { name: "Use Base Template" })[0],
    );

    expect(mockNavigate).toHaveBeenCalledWith(
      "/cicd/developer-deploy?template=web-backend-standard&appType=web-backend",
    );
  });

  // 서버는 web·backend·batch 만 받는다. 폼에 유형 선택이 없던 동안 "web-backend" 를 하드코딩해 보냈고,
  // 저장이 되기 시작하면 그 템플릿으로 만드는 파이프라인이 ENUM 열에서 거부된다.
  const pickAppType = (name: string) => {
    fireEvent.mouseDown(screen.getByLabelText("Application Type"));
    fireEvent.click(screen.getByRole("option", { name }));
  };

  it("submits the create payload with the chosen application type", () => {
    renderWithProviders(<CicdTemplatePage />);

    fireEvent.click(screen.getByRole("button", { name: "Create Template" }));
    fireEvent.change(screen.getByLabelText("Template ID"), {
      target: { value: "team-web-v1" },
    });
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Team Web" },
    });
    fireEvent.click(screen.getByLabelText("CI"));
    pickAppType("Web");
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(mockCreateMutate).toHaveBeenCalledTimes(1);
    expect(mockCreateMutate.mock.calls[0][0]).toEqual({
      id: "team-web-v1",
      name: "Team Web",
      description: "",
      appType: "web",
      stages: ["CI"],
    });
  });

  it("defaults the application type to backend", () => {
    renderWithProviders(<CicdTemplatePage />);

    fireEvent.click(screen.getByRole("button", { name: "Create Template" }));
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Plain Backend" },
    });
    fireEvent.click(screen.getByLabelText("CD"));
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(mockCreateMutate.mock.calls[0][0]).toMatchObject({
      id: "plain-backend",
      appType: "backend",
    });
  });

  it("prefills the application type when editing and sends it on save", () => {
    renderWithProviders(<CicdTemplatePage />);

    // Edit 버튼 순서: Default(2) → Helm(1) → Cronjob/Job(2). 세 번째가 Helm Release(web)다.
    fireEvent.click(screen.getAllByRole("button", { name: "Edit" })[2]);

    expect(screen.getByLabelText("Application Type").textContent).toBe("Web");

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(mockUpdateMutate).toHaveBeenCalledTimes(1);
    expect(mockUpdateMutate.mock.calls[0][0]).toMatchObject({
      id: "helm-release-v1",
      appType: "web",
    });
  });

  it("maps a legacy application type to backend when editing", () => {
    renderWithProviders(<CicdTemplatePage />);

    // 첫 Edit 은 web-backend-standard(appType "web-backend")다.
    fireEvent.click(screen.getAllByRole("button", { name: "Edit" })[0]);

    expect(screen.getByLabelText("Application Type").textContent).toBe("Backend");
  });
});

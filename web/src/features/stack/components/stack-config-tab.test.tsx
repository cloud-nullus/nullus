import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";

import { renderWithProviders } from "../../../__tests__/test-utils";
import { StackConfigTab } from "./stack-config-tab";

const mockUseStackReleases = vi.hoisted(() => vi.fn());
const mockUseReleaseValues = vi.hoisted(() => vi.fn());
const mockPreview = vi.hoisted(() => vi.fn());
const mockApply = vi.hoisted(() => vi.fn());

vi.mock("../api/stack-api", () => ({
  useStackReleases: (...args: unknown[]) => mockUseStackReleases(...args),
  useReleaseValues: (...args: unknown[]) => mockUseReleaseValues(...args),
  usePreviewReleaseValues: () => ({ mutateAsync: mockPreview, isPending: false }),
  useApplyReleaseValues: () => ({ mutateAsync: mockApply, isPending: false }),
}));

// Monaco 는 jsdom 에서 뜨지 않는다. 값이 흘러가는지만 본다.
vi.mock("../../../components/shared/monaco-yaml-editor", () => ({
  MonacoYamlEditor: ({ value }: { value: string }) => <pre data-testid="yaml-editor">{value}</pre>,
}));

const releases = [
  {
    release_name: "harbor",
    step_name: "installing_harbor",
    chart_name: "harbor",
    chart_version: "1.15.0",
    namespace: "nullus",
    revision: 2,
    status: "deployed",
  },
];

const LIVE_YAML = "externalURL: http://harbor.nullus.local\n";

beforeEach(() => {
  vi.clearAllMocks();
  mockUseStackReleases.mockReturnValue({
    data: releases,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
  mockUseReleaseValues.mockReturnValue({
    data: {
      release_name: "harbor",
      step_name: "installing_harbor",
      namespace: "nullus",
      revision: 2,
      mode: "live",
      yaml: LIVE_YAML,
      protected_paths: ["externalURL"],
    },
    isLoading: false,
  });
});

describe("StackConfigTab", () => {
  it("배포된 values 를 에디터에 실어 준다", async () => {
    renderWithProviders(<StackConfigTab stackId="stk_1" />);

    expect(await screen.findByTestId("yaml-editor")).toHaveTextContent("externalURL");
  });

  it("플랫폼이 관리하는 경로를 미리 알려 준다", async () => {
    renderWithProviders(<StackConfigTab stackId="stk_1" />);

    expect(await screen.findByText(/Platform-managed values/)).toBeInTheDocument();
  });

  it("편집 단위를 오버라이드로 바꾸면 그 모드로 원본을 다시 읽는다", async () => {
    renderWithProviders(<StackConfigTab stackId="stk_1" />);

    fireEvent.click(await screen.findByRole("button", { name: "My overrides only" }));

    await waitFor(() => {
      expect(mockUseReleaseValues).toHaveBeenCalledWith("stk_1", "harbor", "override");
    });
  });

  it("미리보기는 클러스터를 바꾸지 않고 경고만 보여 준다", async () => {
    mockPreview.mockResolvedValue({
      release_name: "harbor",
      namespace: "nullus",
      mode: "live",
      revision: 2,
      dry_run: true,
      warnings: [
        { path: "externalURL", kind: "removed", message: "externalURL is computed by the platform." },
      ],
      effective_yaml: "trivy:\n  enabled: false\n",
    });

    renderWithProviders(<StackConfigTab stackId="stk_1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Preview changes" }));

    await waitFor(() => expect(mockPreview).toHaveBeenCalledTimes(1));
    expect(mockApply).not.toHaveBeenCalled();
    expect(
      await screen.findByText("You changed platform-managed values"),
    ).toBeInTheDocument();
  });

  it("적용은 확인 절차를 거친다 — 파드가 재시작될 수 있기 때문이다", async () => {
    mockApply.mockResolvedValue({
      release_name: "harbor",
      namespace: "nullus",
      mode: "live",
      revision: 3,
      dry_run: false,
    });

    renderWithProviders(<StackConfigTab stackId="stk_1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Apply" }));

    expect(mockApply).not.toHaveBeenCalled();
    expect(await screen.findByText(/helm upgrade will run against the harbor release/)).toBeInTheDocument();

    const applyButtons = await screen.findAllByRole("button", { name: "Apply" });
    fireEvent.click(applyButtons[applyButtons.length - 1]);

    await waitFor(() => {
      expect(mockApply).toHaveBeenCalledWith({
        stackId: "stk_1",
        releaseName: "harbor",
        mode: "live",
        yaml: LIVE_YAML,
      });
    });
  });

  it("릴리스가 없으면 편집할 것이 없다고 말한다", async () => {
    mockUseStackReleases.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    renderWithProviders(<StackConfigTab stackId="stk_1" />);

    expect(
      await screen.findByText("This stack has no Helm release to edit."),
    ).toBeInTheDocument();
  });
});

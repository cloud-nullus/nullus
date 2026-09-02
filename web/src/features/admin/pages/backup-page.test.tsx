import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { screen, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { BackupPage } from "./backup-page";

const mockUseBackupRuns = vi.hoisted(() => vi.fn());
const mockMutate = vi.hoisted(() => vi.fn());

vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useLocation: () => ({
      pathname: "/admin/backup",
      search: "",
      hash: "",
      state: null,
      key: "t",
    }),
  };
});

vi.mock("../../../stores/auth-store", () => ({
  useAuthStore: vi.fn(() => ({
    role: "admin",
    user: null,
    isAuthenticated: true,
  })),
}));

vi.mock("../api/backup-api", async () => {
  const actual =
    await vi.importActual<typeof import("../api/backup-api")>(
      "../api/backup-api",
    );
  return {
    ...actual,
    useBackupRuns: () => mockUseBackupRuns(),
    useCreateBackup: () => ({ mutate: mockMutate, isPending: false }),
  };
});

describe("BackupPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseBackupRuns.mockReturnValue({
      data: [
        {
          id: "b1",
          mode: "full",
          trigger: "manual",
          status: "succeeded",
          total_bytes: 368481580,
          quiesce_seconds: 48.7,
          created_at: "2026-09-03T01:02:00Z",
        },
        {
          id: "b2",
          mode: "full",
          trigger: "manual",
          status: "partial",
          total_bytes: 1024,
          error: "keycloak_db: pg_dump 를 실행할 수 없습니다",
          created_at: "2026-09-02T23:58:00Z",
        },
      ],
    });
  });

  it("should list past runs with size and downtime", () => {
    renderWithProviders(<BackupPage />);

    expect(screen.getByText("351.4 MB")).toBeInTheDocument();
    expect(screen.getByText("48.7s")).toBeInTheDocument();
  });

  it("should show a partial run as a warning, not a success", () => {
    // partial 을 성공처럼 보이게 하면 빠진 산출물을 아무도 보지 않는다.
    renderWithProviders(<BackupPage />);

    const badge = screen.getByText("partial");
    expect(badge.className).toContain("--color-warning");
    expect(screen.getByText(/pg_dump/)).toBeInTheDocument();
  });

  it("should submit the chosen targets from the dialog", () => {
    renderWithProviders(<BackupPage />);

    fireEvent.click(
      screen.getByRole("button", { name: /백업 실행|Run backup/i }),
    );
    // 볼륨을 빼면 확인 입력 없이 바로 시작할 수 있다.
    fireEvent.click(document.getElementById("backup-target-volume")!);
    fireEvent.click(
      screen.getByRole("button", { name: /백업 시작|Start backup/i }),
    );

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        mode: "full",
        namespace: "nullus-app",
        scope: ["platform_db", "keycloak_db", "openbao_kv", "ns_resources"],
        confirm: undefined,
      }),
      expect.anything(),
    );
  });

  it("should say so when there is nothing yet", () => {
    mockUseBackupRuns.mockReturnValue({ data: [] });
    renderWithProviders(<BackupPage />);

    expect(
      screen.getByText(/아직 백업이 없습니다|No backups yet/),
    ).toBeInTheDocument();
  });
});

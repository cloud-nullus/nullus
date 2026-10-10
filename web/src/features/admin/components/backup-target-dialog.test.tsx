import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { screen, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { BackupTargetDialog } from "./backup-target-dialog";

const ALL = [
  "platform_db",
  "keycloak_db",
  "openbao_kv",
  "ns_resources",
  "volume",
];

const STACKS = [
  { id: "stk_a", name: "gitlab-stack", namespace: "nullus-gitlab" },
  { id: "stk_b", name: "jenkins-stack", namespace: "nullus-jenkins" },
];

function setup(stacks = STACKS) {
  const onSubmit = vi.fn();
  renderWithProviders(
    <BackupTargetDialog
      open
      onClose={vi.fn()}
      stacks={stacks}
      onSubmit={onSubmit}
    />,
  );
  return { onSubmit };
}

const box = (id: string) =>
  document.getElementById(`backup-target-${id}`) as HTMLInputElement;
const startButton = () =>
  screen.getByRole("button", { name: /백업 시작|Start backup/i });

describe("BackupTargetDialog", () => {
  it("should preselect every target so the complete backup needs no extra action", () => {
    setup();
    for (const id of ALL) expect(box(id)).toBeChecked();
  });

  it("should warn about downtime and hold the button until the namespace is typed", () => {
    // 볼륨이 기본 선택이므로 정지 창이 생긴다 — 경고와 확인 입력이 떠 있어야 한다.
    setup();
    expect(screen.getByRole("alert")).toHaveTextContent(/멈춥|stops/i);
    expect(startButton()).toBeDisabled();
  });

  it("should drop the confirmation once volumes are excluded, because nothing stops", () => {
    setup();
    fireEvent.click(box("volume"));

    expect(screen.queryByRole("alert")).toBeNull();
    expect(startButton()).toBeEnabled();
  });

  it("should submit only the chosen targets", () => {
    const { onSubmit } = setup();
    for (const id of ["volume", "keycloak_db", "openbao_kv", "ns_resources"]) {
      fireEvent.click(box(id));
    }
    fireEvent.click(startButton());

    expect(onSubmit).toHaveBeenCalledWith({
      stackId: "stk_a",
      namespace: "nullus-gitlab",
      scope: ["platform_db"],
      confirm: undefined,
    });
  });

  it("should send the typed namespace when volumes are included", () => {
    const { onSubmit } = setup();
    fireEvent.change(screen.getByPlaceholderText("nullus-gitlab"), {
      target: { value: "nullus-gitlab" },
    });
    fireEvent.click(startButton());

    expect(onSubmit).toHaveBeenCalledWith({
      stackId: "stk_a",
      namespace: "nullus-gitlab",
      scope: ALL,
      confirm: "nullus-gitlab",
    });
  });

  it("should refuse an empty selection", () => {
    setup();
    for (const id of ALL) fireEvent.click(box(id));

    expect(startButton()).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      /골라야|Select at least/i,
    );
  });

  // 스택을 고르지 않으면 서버는 스택 클러스터에 닿을 수단이 없다 — 리소스·볼륨은 뜰 수 없다.
  // 예전 화면은 대상을 묻지 않고 nullus-app 으로 고정해, 스택 백업이 되지 않았다.
  it("should back up the chosen stack and confirm with its own namespace", () => {
    const { onSubmit } = setup();
    fireEvent.click(screen.getByRole("radio", { name: /jenkins-stack/ }));

    fireEvent.change(screen.getByPlaceholderText("nullus-jenkins"), {
      target: { value: "nullus-jenkins" },
    });
    fireEvent.click(startButton());

    expect(onSubmit).toHaveBeenCalledWith({
      stackId: "stk_b",
      namespace: "nullus-jenkins",
      scope: ALL,
      confirm: "nullus-jenkins",
    });
  });

  it("should preselect the first stack", () => {
    setup();
    expect(screen.getByRole("radio", { name: /gitlab-stack/ })).toBeChecked();
  });

  // 플랫폼만 뜨는 백업에는 스택 대상(금고·리소스·볼륨)이 없다. 고를 수 있게 두면 서버가 거절한다.
  it("should leave out stack-only targets when backing up the platform only", () => {
    const { onSubmit } = setup();
    fireEvent.click(screen.getByRole("radio", { name: /Platform only|플랫폼만/ }));

    for (const id of ["openbao_kv", "ns_resources", "volume"]) {
      expect(box(id)).toBeDisabled();
      expect(box(id)).not.toBeChecked();
    }
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.click(startButton());

    expect(onSubmit).toHaveBeenCalledWith({
      stackId: undefined,
      namespace: undefined,
      scope: ["platform_db", "keycloak_db"],
      confirm: undefined,
    });
  });

  it("should offer only the platform when there is no stack", () => {
    setup([]);
    expect(screen.getByRole("radio", { name: /Platform only|플랫폼만/ })).toBeChecked();
    expect(box("volume")).toBeDisabled();
  });
});

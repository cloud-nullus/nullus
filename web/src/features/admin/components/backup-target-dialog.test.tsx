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

function setup() {
  const onSubmit = vi.fn();
  renderWithProviders(
    <BackupTargetDialog
      open
      onClose={vi.fn()}
      namespace="nullus-app"
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
      scope: ["platform_db"],
      confirm: undefined,
    });
  });

  it("should send the typed namespace when volumes are included", () => {
    const { onSubmit } = setup();
    fireEvent.change(screen.getByPlaceholderText("nullus-app"), {
      target: { value: "nullus-app" },
    });
    fireEvent.click(startButton());

    expect(onSubmit).toHaveBeenCalledWith({
      scope: ALL,
      confirm: "nullus-app",
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
});

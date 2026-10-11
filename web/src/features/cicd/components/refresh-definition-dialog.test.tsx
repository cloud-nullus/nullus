import { describe, it, expect, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { RefreshDefinitionDialog } from "./refresh-definition-dialog";

const baseProps = {
  open: true,
  pipelineName: "orders",
  onCancel: vi.fn(),
};

function confirmButton() {
  return screen.getByRole("button", { name: /Refresh definition|정의 갱신/ });
}

// SAST 리포트 전에 만든 파이프라인은 정의를 다시 써야 결과가 붙는다. 사용자가 손본 정의
// 파일을 덮어쓰므로 무엇을 덮어쓰고 무엇은 건드리지 않는지를 말하고 확인을 받는다.
describe("RefreshDefinitionDialog", () => {
  it("explains what is overwritten and what is left alone", () => {
    render(<RefreshDefinitionDialog {...baseProps} onConfirm={vi.fn()} />);

    expect(screen.getByText(/orders/)).toBeInTheDocument();
    expect(screen.getByText(/\.gitlab-ci\.yml|Jenkinsfile|workflow/i)).toBeInTheDocument();
    expect(screen.getByText(/Dockerfile/)).toBeInTheDocument();
  });

  it("calls onConfirm when confirmed and onCancel when dismissed", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <RefreshDefinitionDialog {...baseProps} onConfirm={onConfirm} onCancel={onCancel} />,
    );

    fireEvent.click(confirmButton());
    expect(onConfirm).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /Cancel|취소/ }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("holds the confirm button while the refresh is running", () => {
    render(<RefreshDefinitionDialog {...baseProps} busy onConfirm={vi.fn()} />);
    expect(confirmButton()).toBeDisabled();
  });

  it("renders nothing when closed", () => {
    render(<RefreshDefinitionDialog {...baseProps} open={false} onConfirm={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /Refresh definition|정의 갱신/ })).toBeNull();
  });
});

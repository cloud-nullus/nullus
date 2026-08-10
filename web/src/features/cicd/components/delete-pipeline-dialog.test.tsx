import { describe, it, expect, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { DeletePipelineDialog } from "./delete-pipeline-dialog";

const baseProps = {
  open: true,
  pipelineName: "nullus-e2e-demo",
  repositoryPath: "devlos0322/nullus-e2e-demo",
  imageRepository: "ghcr.io/devlos0322/nullus-e2e-demo",
  onCancel: vi.fn(),
};

function confirmButton() {
  return screen.getByRole("button", { name: /삭제|Delete/ });
}

describe("DeletePipelineDialog", () => {
  it("defaults every option to off so confirming deletes only the record", () => {
    const onConfirm = vi.fn();
    render(<DeletePipelineDialog {...baseProps} onConfirm={onConfirm} />);

    fireEvent.click(confirmButton());

    expect(onConfirm).toHaveBeenCalledWith({
      deleteClusterResources: false,
      deleteRepository: false,
      deleteImages: false,
    });
  });

  it("passes the selected options through", () => {
    const onConfirm = vi.fn();
    render(<DeletePipelineDialog {...baseProps} onConfirm={onConfirm} />);

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[0]); // 클러스터 리소스
    fireEvent.click(checkboxes[1]); // 이미지
    fireEvent.click(confirmButton());

    expect(onConfirm).toHaveBeenCalledWith({
      deleteClusterResources: true,
      deleteRepository: false,
      deleteImages: true,
    });
  });

  // 목록에서 잘못된 행의 삭제 버튼을 눌렀을 때 마지막으로 걸러지는 지점이다.
  it("blocks confirmation until the repository name is typed exactly", () => {
    const onConfirm = vi.fn();
    render(<DeletePipelineDialog {...baseProps} onConfirm={onConfirm} />);

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[2]); // 소스 저장소
    expect(confirmButton()).toBeDisabled();

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "devlos0322/wrong-repo" } });
    expect(confirmButton()).toBeDisabled();

    fireEvent.change(input, {
      target: { value: "devlos0322/nullus-e2e-demo" },
    });
    expect(confirmButton()).toBeEnabled();

    fireEvent.click(confirmButton());
    expect(onConfirm).toHaveBeenCalledWith({
      deleteClusterResources: false,
      deleteRepository: true,
      deleteImages: false,
    });
  });

  // 이전 선택이 남아 있으면 확인만 누르던 사용자가 의도치 않게 저장소를 지운다.
  it("resets the selection when reopened", () => {
    const onConfirm = vi.fn();
    const { rerender } = render(
      <DeletePipelineDialog {...baseProps} onConfirm={onConfirm} />,
    );

    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    rerender(
      <DeletePipelineDialog {...baseProps} open={false} onConfirm={onConfirm} />,
    );
    rerender(
      <DeletePipelineDialog {...baseProps} open onConfirm={onConfirm} />,
    );

    fireEvent.click(confirmButton());
    expect(onConfirm).toHaveBeenCalledWith({
      deleteClusterResources: false,
      deleteRepository: false,
      deleteImages: false,
    });
  });

  // 저장소 경로를 모르면 확인 문자열을 만들 수 없으므로 선택지를 감춘다.
  it("hides the repository option when no repository path is known", () => {
    render(
      <DeletePipelineDialog
        {...baseProps}
        repositoryPath={undefined}
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getAllByRole("checkbox")).toHaveLength(2);
  });
});

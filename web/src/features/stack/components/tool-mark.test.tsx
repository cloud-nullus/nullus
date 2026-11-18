import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { ToolMark } from "./tool-mark";

describe("ToolMark", () => {
  // 제품 로고를 싣지 않는다 — 상표권자 다수가 제품 화면의 로고 사용을 서면 허락 대상으로 둔다.
  it("writes the tool's initials instead of drawing a brand logo", () => {
    const { container } = render(<ToolMark name="gitlab" />);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    const mark = container.querySelector("[data-tool-mark]");
    expect(mark).toHaveAttribute("data-tool-mark", "GL");
    expect(mark).toHaveTextContent("GL");
  });

  // 옆에 도구 이름이 늘 함께 있으므로 약자는 장식이다. 읽는 도구에는 숨긴다.
  it("is hidden from assistive technology", () => {
    const { container } = render(<ToolMark name="grafana" />);

    expect(container.querySelector("[data-tool-mark]")).toHaveAttribute("aria-hidden", "true");
  });

  // 모든 화면이 같은 타일을 쓴다 — 크기만 다르다.
  it("keeps the same look across sizes", () => {
    const small = render(<ToolMark name="harbor" size="sm" />).container.querySelector("[data-tool-mark]")!;
    const large = render(<ToolMark name="harbor" size="lg" />).container.querySelector("[data-tool-mark]")!;

    expect(small.className).toBe(large.className);
    expect((small as HTMLElement).style.width).toBe("16px");
    expect((large as HTMLElement).style.width).toBe("28px");
  });
});

import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { PipelineTopologyRail } from "./pipeline-topology";
import type { PipelineNode } from "../utils/stack-list-utils";

const nodes: PipelineNode[] = [
  {
    category: "Source",
    tools: [{ name: "GitLab CE", version: "18.5.1", instances: 2 }],
    instances: 2,
  },
  {
    category: "Container Registry",
    tools: [{ name: "Nexus", version: "3.64.0", instances: 1 }],
    instances: 1,
  },
  {
    category: "Package Registry",
    tools: [{ name: "Nexus", version: "3.64.0", instances: 1, shared: true }],
    instances: 0,
  },
];

const stage = (category: string) =>
  screen.getByText(category).closest("div")!.parentElement!;

describe("PipelineTopologyRail", () => {
  it("renders one stage per node in order", () => {
    render(<PipelineTopologyRail nodes={nodes} />);

    expect(
      screen.getAllByRole("listitem").map((item) => item.textContent),
    ).toEqual([
      expect.stringContaining("Source"),
      expect.stringContaining("Container Registry"),
      expect.stringContaining("Package Registry"),
    ]);
  });

  // 개편 전에는 이름과 버전이 각각 join 된 문자열 두 개("a + b", "1 / 2")여서
  // 짝을 맞추려면 자리를 세야 했다. 한 줄 안에 같이 있어야 한다.
  it("keeps a tool name and its version in the same row", () => {
    render(<PipelineTopologyRail nodes={nodes} />);

    const row = screen.getByText("GitLab CE").parentElement!;
    expect(row.textContent).toBe("GitLab CE18.5.1");
  });

  it("shows the pod count per stage", () => {
    render(<PipelineTopologyRail nodes={nodes} />);

    expect(within(stage("Source")).getByText("2 instances")).toBeTruthy();
    expect(
      within(stage("Container Registry")).getByText("1 instance"),
    ).toBeTruthy();
  });

  // Nexus 는 레지스트리 두 칸에 다 나오지만 파드는 하나다. 뒤쪽 칸에 "0" 이라고
  // 쓰면 안 떠 있는 것처럼 읽힌다.
  it("labels a stage whose tools all belong to an earlier stage", () => {
    render(<PipelineTopologyRail nodes={nodes} />);

    expect(
      within(stage("Package Registry")).getByText("shared instance"),
    ).toBeTruthy();
    expect(
      within(stage("Package Registry")).queryByText("0 instances"),
    ).toBeNull();
  });

  it("renders a brand logo per tool", () => {
    const { container } = render(<PipelineTopologyRail nodes={nodes} />);

    const logos = Array.from(container.querySelectorAll("img")).map((img) =>
      img.getAttribute("src"),
    );
    expect(logos).toEqual([
      "/tool-icons/gitlab.svg",
      "/tool-icons/sonatype.svg",
      "/tool-icons/sonatype.svg",
    ]);
  });
});

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

  // Nexus 는 레지스트리 두 칸에 다 나오지만 파드는 하나다. 뒤쪽 칸에 "0" 이라고
  // 쓰면 안 떠 있는 것처럼 읽힌다.
  it("marks a tool that belongs to an earlier stage instead of counting it again", () => {
    render(
      <PipelineTopologyRail
        nodes={[
          {
            category: "Container Registry",
            tools: [
              { name: "Nexus", version: "3.64.0", instances: 1, status: "running", runtimeInstances: 1, readyInstances: 1 },
            ],
            instances: 1,
          },
          {
            category: "Package Registry",
            tools: [
              {
                name: "Nexus",
                version: "3.64.0",
                instances: 1,
                shared: true,
                status: "running",
                runtimeInstances: 1,
                readyInstances: 1,
              },
            ],
            instances: 0,
          },
        ]}
      />,
    );

    expect(within(stage("Package Registry")).getByText("· shared")).toBeTruthy();
    expect(within(stage("Container Registry")).getByText("· 1/1 pods")).toBeTruthy();
  });

  // 동작 여부는 모니터링에서 겹쳐 넣는다. 색만으로 구분하지 않으므로 글자도 함께.
  it("shows runtime status and ready pods when monitoring data is present", () => {
    render(
      <PipelineTopologyRail
        nodes={[
          {
            category: "Source",
            tools: [
              {
                name: "GitLab CE",
                version: "18.5.1",
                instances: 4,
                status: "warning",
                runtimeInstances: 4,
                readyInstances: 3,
              },
            ],
            instances: 4,
          },
        ]}
      />,
    );

    expect(screen.getByText("warning")).toBeTruthy();
    expect(screen.getByText("· 3/4 pods")).toBeTruthy();
  });

  // 모니터링을 아직 못 받았으면 설치할 때 고른 대수만 안다.
  it("falls back to the configured instance count without monitoring", () => {
    render(<PipelineTopologyRail nodes={nodes} />);

    expect(within(stage("Source")).getByText("2 instances")).toBeTruthy();
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

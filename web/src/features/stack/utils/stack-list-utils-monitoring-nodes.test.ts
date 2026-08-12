import { describe, expect, it } from "vitest";
import {
  buildPipelineNodesFromMonitoring,
  type MonitoringToolView,
} from "./stack-list-utils";

function tool(overrides: Partial<MonitoringToolView>): MonitoringToolView {
  return {
    key: "source_repository",
    name: "GitLab CE",
    version: "17.7.0",
    enabled: true,
    pod_count: 1,
    ...overrides,
  };
}

const stageOf = (
  nodes: ReturnType<typeof buildPipelineNodesFromMonitoring>,
  category: string,
) => nodes.find((node) => node.category === category);

describe("buildPipelineNodesFromMonitoring", () => {
  // 스냅샷 경로와 스테이지 구성이 같아야 한다. 다르면 데이터 출처(스냅샷/모니터링)에
  // 따라 같은 스택이 다른 모양으로 보인다.
  it("splits roles into the same stages the snapshot path uses", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "source_repository", name: "GitLab CE" }),
      tool({ key: "container_registry", name: "Harbor", version: "2.11.0" }),
      tool({ key: "package_registry", name: "Nexus", version: "3.70.0" }),
      tool({ key: "storage_backend", name: "MinIO" }),
    ]);

    expect(nodes.map((node) => node.category)).toEqual([
      "Source",
      "Container Registry",
      "Package Registry",
      "Storage",
    ]);
  });

  it("keeps each tool paired with its own version", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "container_registry", name: "Harbor", version: "2.11.0", pod_count: 7 }),
    ]);

    expect(stageOf(nodes, "Container Registry")?.tools).toEqual([
      { name: "Harbor", version: "2.11.0", instances: 7 },
    ]);
  });

  it("sums pod counts within a stage", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "collection", name: "Prometheus", pod_count: 2 }),
      tool({ key: "visualization", name: "Grafana", pod_count: 1 }),
    ]);

    expect(stageOf(nodes, "Monitoring")?.instances).toBe(3);
  });

  // Nexus 가 두 역할을 겸하면 두 칸에 다 나오되 파드는 한 번만 센다.
  it("shows a dual-role tool in both stages but counts its pod once", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "container_registry", name: "Nexus", version: "3.64.0", pod_count: 1 }),
      tool({ key: "package_registry", name: "Nexus", version: "3.64.0", pod_count: 1 }),
    ]);

    expect(stageOf(nodes, "Container Registry")?.instances).toBe(1);
    expect(stageOf(nodes, "Package Registry")?.tools[0]?.shared).toBe(true);
    expect(stageOf(nodes, "Package Registry")?.instances).toBe(0);
  });

  it("includes the CI stage", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "ci_platform", name: "GitLab CI", version: "18.5.1" }),
    ]);

    expect(stageOf(nodes, "CI")?.tools[0]?.name).toBe("GitLab CI");
  });

  it("omits disabled tools", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "container_registry", name: "Harbor", enabled: false }),
    ]);

    expect(stageOf(nodes, "Container Registry")).toBeUndefined();
  });
});

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

describe("buildPipelineNodesFromMonitoring", () => {
  it("includes the container registry in the Artifacts node", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "source_repository", name: "GitLab CE" }),
      tool({ key: "container_registry", name: "Harbor", version: "2.11.0" }),
    ]);

    const artifacts = nodes.find((node) => node.category === "Artifacts");
    expect(artifacts?.oss).toContain("Harbor");
    expect(artifacts?.version).toContain("2.11.0");
  });

  it("includes the package registry in the Artifacts node", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "package_registry", name: "Nexus", version: "3.70.0" }),
    ]);

    const artifacts = nodes.find((node) => node.category === "Artifacts");
    expect(artifacts?.oss).toContain("Nexus");
  });

  it("sums pod counts across every artifacts tool", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "source_repository", pod_count: 4 }),
      tool({ key: "container_registry", name: "Harbor", pod_count: 7 }),
      tool({ key: "storage_backend", name: "MinIO", pod_count: 1 }),
    ]);

    const artifacts = nodes.find((node) => node.category === "Artifacts");
    expect(artifacts?.instances).toBe(12);
  });

  // Nexus 는 컨테이너 레지스트리와 패키지 저장소를 겸할 수 있다. 같은 제품을 두 번
  // 세면 화면에 "Nexus + GitLab CE + Nexus + MinIO" 처럼 뜨고 인스턴스도 이중 계상된다.
  it("lists a tool serving two roles only once", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "container_registry", name: "Nexus", version: "3.64.0", pod_count: 1 }),
      tool({ key: "package_registry", name: "Nexus", version: "3.64.0", pod_count: 1 }),
      tool({ key: "source_repository", name: "GitLab CE", version: "18.5.1", pod_count: 2 }),
    ]);

    const artifacts = nodes.find((node) => node.category === "Artifacts");
    expect(artifacts?.oss).toBe("Nexus + GitLab CE");
    expect(artifacts?.version).toBe("3.64.0 / 18.5.1");
    expect(artifacts?.instances).toBe(3);
  });

  it("omits disabled tools", () => {
    const nodes = buildPipelineNodesFromMonitoring([
      tool({ key: "container_registry", name: "Harbor", enabled: false }),
    ]);

    expect(nodes.find((node) => node.category === "Artifacts")).toBeUndefined();
  });
});

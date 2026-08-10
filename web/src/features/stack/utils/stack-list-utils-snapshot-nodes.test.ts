import { describe, expect, it } from "vitest";
import { buildPipelineNodesFromSnapshot } from "./stack-list-utils";

// gitlab-nexus-v1 처럼 Nexus 가 컨테이너 레지스트리와 패키지 저장소를 겸하는 스택은
// 저장된 설정에도 같은 제품이 두 번 들어 있다. 그대로 그리면 스택 상세의 Artifacts
// 노드가 "Nexus + GitLab CE + Nexus + MinIO" 로 나온다.
describe("buildPipelineNodesFromSnapshot", () => {
  const snapshot = {
    config: {
      artifacts: {
        package_registry: { name: "Nexus", version: "3.64.0", enabled: true },
        source_repository: { name: "GitLab CE", version: "18.5.1", enabled: true },
        container_registry: { name: "Nexus", version: "3.64.0", enabled: true },
        storage_backend: { name: "MinIO", version: "RELEASE.2024", enabled: true },
      },
    },
  };

  it("lists a tool serving two roles only once", () => {
    const artifacts = buildPipelineNodesFromSnapshot(snapshot).find(
      (node) => node.category === "Artifacts",
    );

    expect(artifacts?.oss).toBe("Nexus + GitLab CE + MinIO");
  });

  it("does not repeat the version of the deduplicated tool", () => {
    const artifacts = buildPipelineNodesFromSnapshot(snapshot).find(
      (node) => node.category === "Artifacts",
    );

    expect(artifacts?.version).toBe("3.64.0 / 18.5.1 / RELEASE.2024");
  });

  it("keeps distinct tools that merely share a category", () => {
    const artifacts = buildPipelineNodesFromSnapshot({
      config: {
        artifacts: {
          container_registry: { name: "Harbor", version: "2.11.0", enabled: true },
          storage_backend: { name: "MinIO", version: "RELEASE.2024", enabled: true },
        },
      },
    }).find((node) => node.category === "Artifacts");

    expect(artifacts?.oss).toBe("Harbor + MinIO");
  });
});

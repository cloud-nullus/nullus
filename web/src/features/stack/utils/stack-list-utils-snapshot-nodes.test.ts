import { describe, expect, it } from "vitest";
import { buildPipelineNodesFromSnapshot } from "./stack-list-utils";

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

  const stageOf = (nodes: ReturnType<typeof buildPipelineNodesFromSnapshot>, category: string) =>
    nodes.find((node) => node.category === category);

  // Artifacts 한 칸에 몰아 넣으면 "Nexus + GitLab CE + MinIO" 가 되어 어느 도구가
  // 어떤 역할인지 사라진다. 역할별로 칸을 나눈다.
  it("splits artifacts into one stage per role", () => {
    const nodes = buildPipelineNodesFromSnapshot(snapshot);

    expect(nodes.map((node) => node.category)).toEqual([
      "Source",
      "Container Registry",
      "Package Registry",
      "Storage",
    ]);
  });

  it("keeps each tool paired with its own version", () => {
    const nodes = buildPipelineNodesFromSnapshot(snapshot);

    expect(stageOf(nodes, "Source")?.tools).toEqual([
      { name: "GitLab CE", version: "18.5.1", instances: 1 },
    ]);
    expect(stageOf(nodes, "Storage")?.tools).toEqual([
      { name: "MinIO", version: "RELEASE.2024", instances: 1 },
    ]);
  });

  // Nexus 는 컨테이너 레지스트리이면서 패키지 저장소다. 두 칸에 다 나와야 배치가
  // 보이지만 파드는 하나다 — 뒤에 나오는 쪽을 shared 로 표시하고 합계에서 뺀다.
  it("shows a dual-role tool in both stages but counts its pod once", () => {
    const nodes = buildPipelineNodesFromSnapshot(snapshot);
    const container = stageOf(nodes, "Container Registry");
    const pkg = stageOf(nodes, "Package Registry");

    expect(container?.tools[0]).toEqual({
      name: "Nexus",
      version: "3.64.0",
      instances: 1,
    });
    expect(container?.instances).toBe(1);

    expect(pkg?.tools[0]?.name).toBe("Nexus");
    expect(pkg?.tools[0]?.shared).toBe(true);
    expect(pkg?.instances).toBe(0);
  });

  // GitLab 한 설치가 소스·CI·컨테이너 레지스트리·패키지 저장소를 다 맡는다.
  // 스냅샷은 역할별 이름(gitlab-ci …)을 쓰지만 파드는 gitlab 하나에 들어 있다.
  it("treats role-suffixed names as the same deployment", () => {
    const nodes = buildPipelineNodesFromSnapshot({
      config: {
        artifacts: {
          source_repository: { name: "gitlab", version: "9.5.1", enabled: true },
          container_registry: { name: "gitlab-registry", version: "9.5.1", enabled: true },
          package_registry: { name: "gitlab-package-registry", version: "9.5.1", enabled: true },
        },
        pipeline: {
          ci_platform: { name: "gitlab-ci", version: "9.5.1", enabled: true },
        },
      },
    });

    const shared = nodes
      .flatMap((node) => node.tools)
      .filter((tool) => tool.shared)
      .map((tool) => [tool.name, tool.sharedWith]);

    expect(shared).toEqual([
      ["gitlab-ci", "gitlab"],
      ["gitlab-registry", "gitlab"],
      ["gitlab-package-registry", "gitlab"],
    ]);
    // 파드는 gitlab 한 곳에서만 센다.
    expect(nodes.map((node) => node.instances)).toEqual([1, 0, 0, 0]);
  });

  // 로고를 공유할 뿐 별개 배포인 것을 묶으면 안 된다 (tempo 는 grafana 아이콘을 쓴다).
  it("does not merge unrelated tools that merely share a logo", () => {
    const nodes = buildPipelineNodesFromSnapshot({
      config: {
        monitoring: {
          visualization: { name: "grafana", version: "8.5.0", enabled: true },
        },
        logging: {
          trace_layer: { name: "tempo", version: "2.6.0", enabled: true },
        },
      },
    });

    expect(nodes.flatMap((node) => node.tools).every((tool) => !tool.shared)).toBe(true);
  });

  it("omits a stage whose role has no tool", () => {
    const nodes = buildPipelineNodesFromSnapshot({
      config: {
        artifacts: {
          container_registry: { name: "Harbor", version: "2.11.0", enabled: true },
          storage_backend: { name: "MinIO", version: "RELEASE.2024", enabled: true },
        },
      },
    });

    expect(nodes.map((node) => node.category)).toEqual([
      "Container Registry",
      "Storage",
    ]);
  });

  it("orders stages the way code flows: source to build to store to deploy", () => {
    const nodes = buildPipelineNodesFromSnapshot({
      config: {
        artifacts: {
          source_repository: { name: "GitLab CE", version: "18.5.1", enabled: true },
          container_registry: { name: "Harbor", version: "2.11.0", enabled: true },
        },
        pipeline: {
          ci_platform: { name: "GitLab CI", version: "18.5.1", enabled: true },
          cd_tool: { name: "ArgoCD", version: "2.13.0", enabled: true },
        },
        monitoring: {
          collection: { name: "Prometheus", version: "2.54.0", enabled: true },
        },
      },
    });

    expect(nodes.map((node) => node.category)).toEqual([
      "Source",
      "CI",
      "Container Registry",
      "CD",
      "Monitoring",
    ]);
  });
});

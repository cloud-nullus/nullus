import { describe, expect, it } from "vitest";
import { toolDisplayName, toolLogoURL } from "./tool-logo";

describe("toolLogoURL", () => {
  it.each([
    ["GitLab CE", "gitlab"],
    ["GitLab Package Registry", "gitlab"],
    ["GitHub Actions", "githubactions"],
    ["GitHub Packages", "github"],
    ["Nexus Repository", "sonatype"],
    ["JFrog Artifactory", "jfrog"],
    ["Gitea", "gitea"],
    ["Docker Hub", "docker"],
    ["Google Cloud Storage", "googlecloud"],
    ["Argo CD", "argo"],
    ["Flux CD", "flux"],
    ["Jenkins", "jenkins"],
    ["Spinnaker", "spinnaker"],
    ["Tekton", "tekton"],
    ["Grafana", "grafana"],
    ["Prometheus", "prometheus"],
    ["Thanos", "thanos"],
    ["VictoriaMetrics", "victoriametrics"],
    ["OpenSearch Dashboards", "opensearch"],
    ["OpenSearch", "opensearch"],
    ["Elasticsearch", "elasticsearch"],
    ["OpenTelemetry Collector", "opentelemetry"],
    ["Grafana Loki", "grafana"],
    ["Fluentd", "fluentd"],
    ["Tempo", "grafana"],
    ["Jaeger", "jaeger"],
    ["Harbor Registry", "harbor"],
    // 백엔드는 이 슬롯을 minio 로 부르지만 설치되는 실물은 포크인 SILO 다.
    ["MinIO", "silo"],
    ["minio", "silo"],
    ["MinIO (SILO)", "silo"],
    ["SILO", "silo"],
    ["SeaweedFS", "seaweedfs"],
    ["seaweedfs", "seaweedfs"],
    ["OpenBao", "openbao"],
  ])("uses a local icon for %s", (toolName, slug) => {
    expect(toolLogoURL(toolName)).toBe(`/tool-icons/${slug}.svg`);
  });

  it("uses a local Kubernetes icon when a tool is unmapped", () => {
    expect(toolLogoURL("custom-tool")).toBe("/tool-icons/kubernetes.svg");
  });
});

// 이름과 마크는 한 쌍으로 읽힌다. 마크가 SILO 인데 이름이 MinIO 면 다른 제품처럼 보인다.
describe("toolDisplayName", () => {
  it.each([
    ["MinIO", "MinIO (SILO)"],
    ["minio", "MinIO (SILO)"],
    ["SILO", "MinIO (SILO)"],
    ["MinIO (SILO)", "MinIO (SILO)"],
    ["seaweedfs", "SeaweedFS"],
    ["SeaweedFS", "SeaweedFS"],
  ])("names the object storage %s as %s", (toolName, label) => {
    expect(toolDisplayName(toolName)).toBe(label);
  });

  it("keeps every other name as stored", () => {
    expect(toolDisplayName("argocd")).toBe("argocd");
    expect(toolDisplayName("GitLab CE")).toBe("GitLab CE");
  });
});

import { describe, expect, it } from "vitest";
import { toolLogoURL } from "./tool-logo";

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
    ["MinIO", "minio"],
    ["OpenBao", "openbao"],
  ])("uses a local icon for %s", (toolName, slug) => {
    expect(toolLogoURL(toolName)).toBe(`/tool-icons/${slug}.svg`);
  });

  it("uses a local Kubernetes icon when a tool is unmapped", () => {
    expect(toolLogoURL("custom-tool")).toBe("/tool-icons/kubernetes.svg");
  });
});

import { describe, expect, it } from "vitest";
import { toolDisplayName, toolMonogram } from "./tool-identity";

// 화면은 도구를 브랜드 로고가 아니라 글자 약자로 표시한다. 스택 설정·서버 응답에 오는
// 이름 표기가 제각각이라(id, 카탈로그 이름, 역할별 이름) 모두 같은 약자로 모여야 한다.
describe("toolMonogram", () => {
  it.each([
    ["GitLab", "GL"],
    ["GitHub", "GH"],
    ["GitHub Actions", "GA"],
    ["Gitea", "GT"],
    ["Jenkins", "JE"],
    ["Argo CD", "AR"],
    ["Harbor", "HB"],
    ["Nexus Repository", "NX"],
    ["SILO", "SI"],
    ["SeaweedFS", "SW"],
    ["Prometheus", "PR"],
    ["Grafana", "GF"],
    ["OpenSearch", "OS"],
    ["Grafana Loki", "LK"],
    ["Grafana Tempo", "TP"],
    ["Jaeger", "JG"],
    ["OpenTelemetry Collector", "OT"],
    ["OpenBao", "OB"],
    ["Trivy", "TV"],
    ["SonarQube", "SQ"],
  ])("writes %s as %s", (toolName, monogram) => {
    expect(toolMonogram(toolName)).toBe(monogram);
  });

  // 한 제품이 역할마다 다른 이름으로 온다 — 같은 설치이므로 같은 약자다.
  it.each([
    ["GL", ["gitlab", "GitLab CE", "gitlab-ci", "gitlab-runner", "gitlab-registry", "GitLab Package Registry"]],
    ["GH", ["github", "ghcr", "GitHub Packages"]],
    ["AR", ["argocd", "argo-cd"]],
    ["NX", ["nexus", "Nexus", "nexus3", "Nexus Repository Manager"]],
    ["SI", ["minio", "MinIO", "MinIO (SILO)"]],
    ["LK", ["loki"]],
    ["TP", ["tempo", "Tempo"]],
    ["OT", ["opentelemetry-collector", "OpenTelemetry"]],
  ])("writes every name of one product as %s", (monogram, names) => {
    for (const name of names) {
      expect(toolMonogram(name), name).toBe(monogram);
    }
  });

  it("derives two letters from an unmapped name", () => {
    expect(toolMonogram("custom-tool")).toBe("CU");
    expect(toolMonogram("Velero")).toBe("VE");
  });

  // 묶음 이름의 단어 첫 글자를 따면 GA(GitHub Actions) 처럼 다른 제품의 약자가 된다.
  it("does not borrow another product's initials for a combined name", () => {
    expect(toolMonogram("GitLab + Argo CD + SonarQube")).toBe("GI");
  });
});

// 스택 설정의 minio 를 고르면 서는 것은 MinIO 포크 SILO 다. 이름이 설치되는 실물을 가리켜야
// 하고, MinIO 상표 정책은 수정판에 MinIO 이름을 붙이는 것을 금지한다 — 그래서 SILO 로만 적는다.
// 예전 화면이 저장해 둔 "MinIO (SILO)" 도 같은 이름으로 모은다.
describe("toolDisplayName", () => {
  it.each([
    ["MinIO", "SILO"],
    ["minio", "SILO"],
    ["SILO", "SILO"],
    ["MinIO (SILO)", "SILO"],
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

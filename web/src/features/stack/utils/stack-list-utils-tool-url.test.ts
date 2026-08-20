import { describe, expect, it } from "vitest";

import { resolveToolLaunchURL, toolLaunchURL } from "./stack-list-utils";

describe("도구 접속 주소", () => {
  // 접속 도메인은 게이트웨이 TLS 리스너 뒤에 선다. 서버(domain.ToolAccessURL)도
  // https 로 내려주므로 화면이 http 를 안내하면 두 화면이 어긋난다.
  it("https 로 만든다", () => {
    expect(toolLaunchURL("gitlab", "nullus.local")).toBe("https://gitlab.nullus.local");
    expect(toolLaunchURL("Argo CD", "nullus.local")).toBe("https://argocd.nullus.local");
    expect(toolLaunchURL("harbor", "nullus.local")).toBe("https://harbor.nullus.local");
    expect(toolLaunchURL("elasticsearch", "nullus.local")).toBe("https://kibana.nullus.local");
  });

  // 서버(domain.ToolAccessURL)가 아는 도구는 화면의 대비책도 알아야 한다 —
  // 한쪽에만 있으면 Gitea/Jenkins 스택에서 서버 응답 전에 링크가 사라진다.
  it("Gitea·Jenkins·Nexus 스택도 링크를 만든다", () => {
    expect(toolLaunchURL("gitea", "nullus.local")).toBe("https://gitea.nullus.local");
    expect(toolLaunchURL("Jenkins", "nullus.local")).toBe("https://jenkins.nullus.local");
    expect(toolLaunchURL("nexus", "nullus.local")).toBe("https://nexus.nullus.local");
  });

  it("접속 도메인이나 규칙을 모르면 링크를 걸지 않는다", () => {
    expect(toolLaunchURL("gitlab", "")).toBeNull();
    expect(toolLaunchURL("some-unknown-tool", "nullus.local")).toBeNull();
  });

  // 주소의 단일 출처는 서버다. 화면의 규칙은 서버가 아직 주소를 주지 못할 때만 쓴다.
  it("서버가 준 주소를 먼저 쓴다", () => {
    const serverTools = [{ name: "grafana", url: "https://metrics.example.com" }];

    expect(resolveToolLaunchURL("Grafana", serverTools, "nullus.local")).toBe(
      "https://metrics.example.com",
    );
  });

  it("서버가 주소를 주지 않은 도구는 접속 도메인으로 만든다", () => {
    const serverTools = [{ name: "grafana", url: "" }];

    expect(resolveToolLaunchURL("grafana", serverTools, "nullus.local")).toBe(
      "https://grafana.nullus.local",
    );
    expect(resolveToolLaunchURL("harbor", serverTools, "nullus.local")).toBe(
      "https://harbor.nullus.local",
    );
  });

  it("도구 이름은 대소문자를 가리지 않고 맞춘다", () => {
    const serverTools = [{ name: "ArgoCD", url: "https://cd.example.com" }];

    expect(resolveToolLaunchURL("argocd", serverTools, "nullus.local")).toBe(
      "https://cd.example.com",
    );
  });
});

import { describe, it, expect } from "vitest";
import {
  buildOssLoginHint,
  findToolCredential,
  toConnectionInfoView,
} from "./stack-list-utils";
import type { StackConnectionInfoResponse } from "../api/stack-api-types";

// 서버가 실제로 내려주는 모양. 리소스 이름의 단일 출처는
// internal/stack/domain/connection.go 이고, 프론트는 그대로 옮기기만 한다.
function serverResponse(namespace: string): StackConnectionInfoResponse {
  return {
    stack_id: "stk_1",
    namespace,
    access_domain: "nullus-devsecops-stack.internal",
    database: {
      mode: "create",
      engine: "postgres",
      endpoint: "nullus-postgresql:5432",
      resource_name: "gitlabhq_production",
      auth_id: "gitlab",
      secret_ref: "nullus-postgresql-credentials",
      secret_key: "password",
    },
    object_storage: {
      mode: "create",
      engine: "minio",
      endpoint: "http://nullus-minio:9000",
      resource_name: "gitlab-artifacts",
      auth_id: "nullus-admin",
      secret_ref: "nullus-minio-credentials",
      secret_key: "rootPassword",
    },
    tools: [
      {
        name: "GitLab CE",
        username: "root",
        secret_ref: "gitlab-gitlab-initial-root-password",
        secret_key: "password",
      },
      {
        name: "Argo CD",
        username: "admin",
        secret_ref: "argocd-initial-admin-secret",
        secret_key: "password",
      },
      { name: "Prometheus", note: "기본 설정에서는 로그인이 필요 없습니다." },
    ],
  };
}

describe("buildOssLoginHint", () => {
  const tools = serverResponse("devsecops").tools;

  // 스택은 사용자가 정한 네임스페이스에 설치된다. nullus 로 고정하면
  // 안내대로 명령을 실행해도 NotFound 가 난다.
  it("uses the stack namespace instead of a hardcoded one", () => {
    const hint = buildOssLoginHint(
      findToolCredential(tools, "Argo CD"),
      "devsecops",
    );
    expect(hint).toContain("-n devsecops");
    expect(hint).not.toContain("-n nullus");
  });

  // Secret 이름은 서버가 준 값을 그대로 쓴다. 화면에서 릴리스 접두사를
  // 붙이거나 떼면 차트 규칙과 갈린다.
  it("uses the secret name the server returned verbatim", () => {
    const hint = buildOssLoginHint(
      findToolCredential(tools, "Argo CD"),
      "devsecops",
    );
    expect(hint).toContain("secret argocd-initial-admin-secret");
    expect(hint).toContain("ID: admin");
  });

  it("keeps the GitLab root secret name and scopes it to the namespace", () => {
    const hint = buildOssLoginHint(
      findToolCredential(tools, "GitLab CE"),
      "devsecops",
    );
    expect(hint).toContain("gitlab-gitlab-initial-root-password");
    expect(hint).toContain("-n devsecops");
  });

  // 네임스페이스를 모르면 그럴듯한 명령을 지어내지 않는다.
  it("falls back to a placeholder when namespace is unknown", () => {
    const hint = buildOssLoginHint(findToolCredential(tools, "Argo CD"), "");
    expect(hint).toContain("<namespace>");
  });

  // 조회할 Secret 이 없는 도구는 명령 대신 안내문을 보여준다.
  it("shows the note when the tool has no secret", () => {
    const hint = buildOssLoginHint(
      findToolCredential(tools, "Prometheus"),
      "devsecops",
    );
    expect(hint).not.toContain("kubectl");
    expect(hint).toContain("로그인이 필요 없습니다");
  });

  // 응답이 오기 전에 그럴듯한 명령을 만들어 보여주면 안 된다.
  it("says it is loading when the server has not answered yet", () => {
    expect(buildOssLoginHint(undefined, "devsecops", true)).toContain(
      "불러오는 중",
    );
  });
});

describe("findToolCredential", () => {
  // 카탈로그 표기가 "Argo CD" / "argocd" 로 흔들린다.
  it("matches tool name variants", () => {
    const tools = serverResponse("devsecops").tools;
    for (const name of ["Argo CD", "argocd", "argo-cd", "ArgoCD"]) {
      expect(findToolCredential(tools, name)?.secret_ref).toBe(
        "argocd-initial-admin-secret",
      );
    }
  });

  it("returns undefined for a tool the server did not list", () => {
    expect(findToolCredential(serverResponse("x").tools, "Jenkins")).toBeUndefined();
  });
});

describe("toConnectionInfoView", () => {
  it("carries the server namespace and secret names through", () => {
    const view = toConnectionInfoView(serverResponse("devsecops"));
    expect(view.namespace).toBe("devsecops");
    expect(view.database.accessSecretRef).toBe("nullus-postgresql-credentials");
    expect(view.objectStorage.accessSecretRef).toBe("nullus-minio-credentials");
    expect(view.objectStorage.authId).toBe("nullus-admin");
  });

  // 응답 전에는 값을 지어내지 않고 비워 둔다 — 화면에는 "-" 로 보인다.
  it("stays empty until the server answers", () => {
    const view = toConnectionInfoView(undefined, "example.internal");
    expect(view.accessDomain).toBe("example.internal");
    expect(view.namespace).toBe("");
    expect(view.database.endpoint).toBe("");
    expect(view.database.accessSecretRef).toBe("");
  });
});

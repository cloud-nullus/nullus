import { describe, it, expect } from "vitest";
import {
  buildOssLoginHint,
  extractConnectionInfo,
  type StackConnectionInfo,
} from "./stack-list-utils";

function connInfo(namespace: string): StackConnectionInfo {
  return {
    accessDomain: "nullus-devsecops-stack.internal",
    namespace,
    database: {
      mode: "create",
      providerOrEngine: "postgres",
      endpoint: "nullus-postgresql:5432",
      resourceName: "nullus",
      authId: "gitlab",
      accessSecretRef: "nullus-postgresql-credentials",
      authPasswordKey: "password",
    },
    objectStorage: {
      mode: "create",
      providerOrEngine: "minio",
      endpoint: "http://nullus-minio:9000",
      resourceName: "nullus-artifacts",
      authId: "nullus-admin",
      accessSecretRef: "nullus-minio-credentials",
      authPasswordKey: "rootPassword",
    },
  };
}

describe("buildOssLoginHint", () => {
  // 스택은 사용자가 정한 네임스페이스에 설치된다. nullus 로 고정하면
  // 안내대로 명령을 실행해도 NotFound 가 난다.
  it("uses the stack namespace instead of a hardcoded one", () => {
    const hint = buildOssLoginHint("Argo CD", connInfo("devsecops"));
    expect(hint).toContain("-n devsecops");
    expect(hint).not.toContain("-n nullus");
  });

  // Argo CD 차트는 Secret 에 릴리스 접두사를 붙이지 않는다.
  // argo-cd-argocd-initial-admin-secret 은 존재하지 않는 이름이다.
  it("points at the real Argo CD admin secret name", () => {
    const hint = buildOssLoginHint("Argo CD", connInfo("devsecops"));
    expect(hint).toContain("secret argocd-initial-admin-secret");
    expect(hint).not.toContain("argo-cd-argocd-initial-admin-secret");
  });

  it("handles Argo CD name variants", () => {
    for (const name of ["Argo CD", "argocd", "argo-cd", "ArgoCD"]) {
      expect(buildOssLoginHint(name, connInfo("devsecops"))).toContain(
        "secret argocd-initial-admin-secret",
      );
    }
  });

  it("keeps the GitLab root secret name and scopes it to the namespace", () => {
    const hint = buildOssLoginHint("GitLab CE", connInfo("devsecops"));
    expect(hint).toContain("gitlab-gitlab-initial-root-password");
    expect(hint).toContain("-n devsecops");
  });

  // 네임스페이스를 모르면 그럴듯한 명령을 지어내지 않는다.
  it("falls back to a placeholder when namespace is unknown", () => {
    const hint = buildOssLoginHint("Argo CD", connInfo(""));
    expect(hint).toContain("<namespace>");
  });

  it("uses connection info for MinIO", () => {
    const hint = buildOssLoginHint("MinIO", connInfo("devsecops"));
    expect(hint).toContain("nullus-admin");
    expect(hint).toContain("nullus-minio-credentials");
  });
});

describe("extractConnectionInfo", () => {
  // 힌트가 네임스페이스를 쓰려면 여기서 보존되어야 한다.
  it("carries the namespace through", () => {
    const conn = extractConnectionInfo({}, "devsecops", "example.internal");
    expect(conn.namespace).toBe("devsecops");
  });
});

describe("extractConnectionInfo fallbacks", () => {
  // 폴백은 스냅샷에 값이 없을 때 화면에 그대로 노출된다.
  // 틀린 값을 보여주면 사용자가 그대로 실행했다가 NotFound 를 만난다.
  //
  // PostgreSQL/MinIO 서비스와 시크릿 이름은 Helm 릴리스명 기준이라
  // 네임스페이스와 무관하다. ${ns} 로 조립하면 항상 어긋난다.
  // (Go 쪽 상수: internal/stack/adapter/helm/secret-provisioning.go)
  const conn = extractConnectionInfo({}, "devsecops", "example.internal");

  it("points at the provisioned PostgreSQL secret", () => {
    expect(conn.database.accessSecretRef).toBe("nullus-postgresql-credentials");
    expect(conn.database.authPasswordKey).toBe("password");
    expect(conn.database.endpoint).toBe("nullus-postgresql:5432");
    expect(conn.database.accessSecretRef).not.toContain("devsecops");
  });

  it("points at the provisioned MinIO secret", () => {
    expect(conn.objectStorage.accessSecretRef).toBe("nullus-minio-credentials");
    expect(conn.objectStorage.authPasswordKey).toBe("rootPassword");
    expect(conn.objectStorage.authId).toBe("nullus-admin");
    expect(conn.objectStorage.endpoint).toBe("http://nullus-minio:9000");
    expect(conn.objectStorage.accessSecretRef).not.toContain("devsecops");
  });

  it("does not derive service names from the namespace", () => {
    const other = extractConnectionInfo({}, "team-a", "example.internal");
    expect(other.database.endpoint).toBe(conn.database.endpoint);
    expect(other.objectStorage.endpoint).toBe(conn.objectStorage.endpoint);
  });
});

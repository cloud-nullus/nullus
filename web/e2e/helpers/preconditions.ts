import { expect, test, type APIRequestContext } from "@playwright/test";

/**
 * 시드 데이터가 있어야만 의미가 있는 테스트의 전제조건.
 *
 * CI 의 DB 는 마이그레이션만 돌린 빈 상태다. 클러스터 등록에는 실제 kubeconfig 가
 * 필요해 워크플로에서 만들 수 없고, 스택은 그 클러스터가 있어야 생긴다. 그래서
 * 이런 테스트는 CI 에서 통과할 수 없다.
 *
 * 그렇다고 실패로 두면 잡 전체가 빨간불이라 **다른 회귀를 덮는다** — 실제로 그
 * 이유로 잡이 continue-on-error 로 묶여 있었고, 그동안 어떤 회귀도 PR 을 막지
 * 못했다. 전제가 없을 때는 명시적으로 skip 하고, 사유를 리포트에 남긴다.
 *
 * skip 은 "검사하지 않았다" 는 뜻이지 "통과했다" 가 아니다. 로컬이나 실 클러스터가
 * 붙은 환경에서는 조건이 채워져 그대로 돈다.
 */

const API_BASE = "http://localhost:8090/api/v1";

async function listJson(
  request: APIRequestContext,
  path: string,
): Promise<unknown[]> {
  const res = await request.get(`${API_BASE}${path}`).catch(() => null);
  if (!res?.ok()) return [];
  const body = (await res.json().catch(() => null)) as unknown;
  if (Array.isArray(body)) return body;
  const obj = (body ?? {}) as { items?: unknown[]; data?: unknown[] };
  return obj.items ?? obj.data ?? [];
}

/** 연결된 파이프라인 클러스터가 없으면 skip. 파이프라인 생성·배포 흐름이 여기 걸린다. */
export async function requireConnectedCluster(
  request: APIRequestContext,
): Promise<void> {
  const clusters = (await listJson(request, "/admin/clusters")) as Array<{
    connection_status?: string;
  }>;
  const connected = clusters.some((c) => c.connection_status === "connected");
  test.skip(
    !connected,
    "연결된 클러스터가 없다 — 시드 데이터가 필요하다(CI 는 빈 DB)",
  );
}

/** 클러스터가 한 대도 없으면 skip. 클러스터 행이 있어야만 그리는 UI 가 여기 걸린다. */
export async function requireAnyCluster(
  request: APIRequestContext,
): Promise<void> {
  const clusters = await listJson(request, "/admin/clusters");
  test.skip(
    clusters.length === 0,
    "등록된 클러스터가 없다 — 시드 데이터가 필요하다(CI 는 빈 DB)",
  );
}

/** 설치된 스택이 없으면 skip. 스택 목록 위에 그려지는 UI 가 여기 걸린다. */
export async function requireAnyStack(
  request: APIRequestContext,
): Promise<void> {
  const stacks = await listJson(request, "/stacks");
  test.skip(
    stacks.length === 0,
    "설치된 스택이 없다 — 시드 데이터가 필요하다(CI 는 빈 DB)",
  );
}

/** 백엔드가 안 떠 있으면 skip. */
export async function requireBackend(
  request: APIRequestContext,
): Promise<void> {
  const res = await request
    .get("http://localhost:8090/health")
    .catch(() => null);
  test.skip(!res?.ok(), "Go 백엔드(8090)가 필요하다");
  expect(res?.ok()).toBeTruthy();
}

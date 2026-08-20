import { describe, expect, it } from "vitest";

import { defaultStackNamespace } from "./install-manifest-builders";

// 서버(internal/stack/domain/stack_namespace.go)와 같은 규칙이어야 한다.
// 화면이 다른 기본값을 보내면 사용자가 고르지도 않은 네임스페이스에 깔린다.
describe("기본 스택 네임스페이스", () => {
  it("스택 이름에서 만든다", () => {
    expect(defaultStackNamespace("gitea-jenkins-v1")).toBe("nullus-gitea-jenkins-v1");
    expect(defaultStackNamespace("My Stack")).toBe("nullus-my-stack");
    expect(defaultStackNamespace("team_a")).toBe("nullus-team-a");
  });

  // 플랫폼이 사는 곳에 깔면 설치는 Helm 소유권 충돌로 실패하고 삭제는 플랫폼을 지운다.
  it("플랫폼 네임스페이스와 절대 같지 않다", () => {
    for (const name of ["nullus", "NULLUS", "  nullus  ", ""]) {
      expect(defaultStackNamespace(name)).not.toBe("nullus");
    }
  });

  it("쓸 수 없는 이름은 기본값으로 떨어진다", () => {
    expect(defaultStackNamespace("")).toBe("nullus-stack");
    expect(defaultStackNamespace("!!!")).toBe("nullus-stack");
  });

  it("쿠버네티스 63자 제한에 맞춘다", () => {
    const ns = defaultStackNamespace(
      "this-is-a-very-long-stack-name-that-goes-well-past-the-kubernetes-limit-for-labels",
    );
    expect(ns.length).toBeLessThanOrEqual(63);
    expect(ns.endsWith("-")).toBe(false);
  });
});

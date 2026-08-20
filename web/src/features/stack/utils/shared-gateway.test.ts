import { describe, expect, it } from "vitest";

import { SHARED_GATEWAY_NAME, SHARED_GATEWAY_NAMESPACE, buildGatewayPortForwardCommand } from "./stack-list-utils";

// 게이트웨이는 스택마다가 아니라 클러스터에 하나 선다. 포트포워드도 그 자리를
// 가리켜야 한다 — 스택 네임스페이스를 가리키면 서비스를 못 찾는다.
describe("게이트웨이 포트포워드 명령", () => {
  it("공용 게이트웨이 네임스페이스와 이름을 넘긴다", () => {
    const command = buildGatewayPortForwardCommand("nullus.io", true);

    expect(command).toContain(`GATEWAY_NAMESPACE='${SHARED_GATEWAY_NAMESPACE}'`);
    expect(command).toContain(`GATEWAY_NAME='${SHARED_GATEWAY_NAME}'`);
    expect(command).toContain("ACCESS_HOST='nullus.io'");
    expect(command).toContain("./scripts/port-forward-gateway.sh");
  });

  it("스택 이름에서 만든 옛 게이트웨이 이름을 더 이상 쓰지 않는다", () => {
    const command = buildGatewayPortForwardCommand("demo-stack.internal", true);

    expect(command).not.toContain("demo-stack-internal-gateway");
    expect(command).toContain(`GATEWAY_NAME='${SHARED_GATEWAY_NAME}'`);
  });

  // 서버(internal/shared/domain/platform_resources.go)와 같은 값이어야 한다.
  it("공용 게이트웨이 상수는 서버와 같다", () => {
    expect(SHARED_GATEWAY_NAMESPACE).toBe("nullus-gateway");
    expect(SHARED_GATEWAY_NAME).toBe("nullus-gateway");
  });
});

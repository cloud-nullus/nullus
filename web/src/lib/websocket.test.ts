import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { connect } from "./websocket";
import { useAuthStore } from "../stores/auth-store";

const constructed: Array<{ url: string; protocols?: string | string[] }> = [];

class FakeWebSocket {
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();

  constructor(url: string, protocols?: string | string[]) {
    constructed.push({ url, protocols });
  }
}

describe("connect", () => {
  beforeEach(() => {
    constructed.length = 0;
    vi.stubGlobal("WebSocket", FakeWebSocket);
    useAuthStore.setState({ token: null });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // 브라우저 WebSocket 은 Authorization 헤더를 못 붙인다. 서버가 토큰을 받을 수 있는
  // 유일한 통로가 서브프로토콜이라, 토큰이 있으면 반드시 실려 나가야 한다.
  it("carries the auth token as a bearer subprotocol", () => {
    useAuthStore.setState({ token: "jwt-abc" });

    connect("ws://localhost/ws/deployments/d-1/logs", { onMessage: () => {} });

    expect(constructed[0]?.protocols).toEqual(["bearer", "jwt-abc"]);
  });

  // 토큰이 없을 때 빈 서브프로토콜을 보내면 핸드셰이크 자체가 깨진다.
  it("omits the subprotocol entirely when there is no token", () => {
    connect("ws://localhost/ws/deployments/d-1/logs", { onMessage: () => {} });

    expect(constructed[0]?.protocols).toBeUndefined();
  });

  it("still passes the url through", () => {
    connect("ws://localhost/ws/deployments/d-1/logs", { onMessage: () => {} });

    expect(constructed[0]?.url).toBe("ws://localhost/ws/deployments/d-1/logs");
  });
});

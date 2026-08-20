import { describe, expect, it } from "vitest";

import { nextDisplayProgress, progressCeiling } from "./deploy-progress";

describe("배포 진행률 표시", () => {
  // 서버가 주는 값은 단계가 바뀔 때만 뛴다(5 → 15 → 90). 그 사이 몇 분 동안
  // 막대가 한 픽셀도 움직이지 않아 멈춘 것처럼 보였다.
  it("값이 그대로여도 조금씩 차오른다", () => {
    const first = nextDisplayProgress({ current: 15, target: 15, ceiling: 18, status: "running" });
    const second = nextDisplayProgress({ current: first, target: 15, ceiling: 18, status: "running" });

    expect(first).toBeGreaterThan(15);
    expect(second).toBeGreaterThan(first);
  });

  // 다만 다음 단계까지 가 있으면 거짓말이 된다. 이정표 앞에서 멈춘다.
  // 스텝이 아직 안 끝났는데 다음 스텝의 몫까지 채우면, 하지 않은 일을 한 것처럼
  // 보여 주는 거짓말이 된다.
  it("이 스텝의 몫을 넘지 않는다", () => {
    let value = 15;
    for (let i = 0; i < 5000; i += 1) {
      value = nextDisplayProgress({ current: value, target: 15, ceiling: 18, status: "running" });
    }

    expect(value).toBeLessThan(18);
    expect(value).toBeGreaterThan(15);
  });

  it("실제 값이 앞서면 그쪽으로 빠르게 따라붙는다", () => {
    const value = nextDisplayProgress({ current: 20, target: 90, ceiling: 93, status: "running" });

    expect(value).toBeGreaterThan(20);
    expect(value).toBeLessThanOrEqual(90);
  });

  // 되감기면 사용자는 뭔가 잘못됐다고 읽는다.
  it("뒤로 가지 않는다", () => {
    const value = nextDisplayProgress({ current: 40, target: 15, ceiling: 43, status: "running" });

    expect(value).toBeGreaterThanOrEqual(40);
  });

  it("성공하면 정확히 100 이다", () => {
    expect(nextDisplayProgress({ current: 62, target: 100, ceiling: 100, status: "success" })).toBe(100);
  });

  // 실패한 배포에서 막대가 계속 차오르면 아직 진행 중인 것처럼 보인다.
  it("실패하면 그 자리에 멈춘다", () => {
    const value = nextDisplayProgress({ current: 42, target: 42, ceiling: 90, status: "failed" });

    expect(value).toBe(42);
  });
});

describe("진행률 상한", () => {
  // 서버가 "이 스텝이 끝났을 때 닿을 값" 을 함께 보낸다(domain.StepProgressCeiling).
  it("서버가 준 상한을 그대로 쓴다", () => {
    expect(progressCeiling(19, 22)).toBe(22);
  });

  // 상한을 못 받으면 스텝 하나의 몫만큼만 가정한다. 멀리 있는 다음 단계를 목표로
  // 삼으면 그 사이를 시간으로만 메우게 되어, 시크릿만 깔린 시점에 막대가 절반을
  // 넘기던 문제가 그대로 돌아온다.
  it("상한이 없으면 스텝 하나의 몫만 가정한다", () => {
    expect(progressCeiling(19)).toBe(22);
    expect(progressCeiling(19, 0)).toBe(22);
  });

  it("100 을 넘지 않는다", () => {
    expect(progressCeiling(99, 120)).toBe(100);
    expect(progressCeiling(100)).toBe(100);
  });
});

describe("진행률 속도", () => {
  // 처음에는 남은 거리에 비례해서만 움직였다. install 단계는 15 에서 90 까지
  // 75 포인트가 남아 있어 비례식이 초당 6%p 를 넘겼고, 막대가 시작하자마자
  // 확 달려 절반을 넘겼다. 거리가 멀어도 걸음 폭은 같아야 한다.
  it("이정표가 멀어도 한 틱에 확 달리지 않는다", () => {
    const stalled = nextDisplayProgress({ current: 15, target: 15, ceiling: 90, status: "running" });
    // 부동소수 오차를 감안한 여유. 요지는 "한 틱에 0.05%p 수준" 이다.
    expect(stalled - 15).toBeLessThan(0.06);
  });

  it("멈춰 있는 동안 1초에 1%p 를 넘지 않는다", () => {
    // 한 틱은 약 140ms — 1초는 대략 7틱이다.
    let value = 15;
    for (let i = 0; i < 7; i += 1) {
      value = nextDisplayProgress({ current: value, target: 15, ceiling: 18, status: "running" });
    }

    expect(value - 15).toBeLessThan(1);
    expect(value).toBeGreaterThan(15);
  });

  // 실제 값이 뛰었을 때는 따라가야 하지만, 순간이동처럼 보이면 안 된다.
  it("큰 도약도 여러 틱에 걸쳐 미끄러진다", () => {
    const one = nextDisplayProgress({ current: 15, target: 90, ceiling: 93, status: "running" });

    expect(one - 15).toBeLessThanOrEqual(1.2);
    expect(one).toBeGreaterThan(15);
  });

  // 따라붙은 뒤에는 다시 다음 이정표를 향해 기어간다 — 멈춰 서지 않는다.
  it("도약이 실제 값을 지나 다음 이정표 앞까지 이어진다", () => {
    let value = 15;
    for (let i = 0; i < 400; i += 1) {
      value = nextDisplayProgress({ current: value, target: 90, ceiling: 93, status: "running" });
    }

    expect(value).toBeGreaterThanOrEqual(90);
    expect(value).toBeLessThan(93);
  });
});

describe("첫 값", () => {
  // 새로고침하면 표시 값이 0 에서 시작한다. 기어오르게 두면 이미 40% 인 배포가
  // 0 부터 다시 차오르는 것처럼 보인다.
  it("0 에서 시작할 때는 곧바로 서버 값에 앉는다", () => {
    expect(nextDisplayProgress({ current: 0, target: 40, ceiling: 43, status: "running" })).toBe(40);
  });

  it("이미 자리를 잡은 뒤에는 다시 미끄러진다", () => {
    const value = nextDisplayProgress({ current: 40, target: 90, ceiling: 93, status: "running" });

    expect(value).toBeGreaterThan(40);
    expect(value).toBeLessThan(90);
  });
});

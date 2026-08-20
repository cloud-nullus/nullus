/**
 * 배포 진행률을 "쌓여 가는" 값으로 바꾼다.
 *
 * 서버가 주는 진행률은 단계가 바뀔 때만 뛴다(5 → 15 → 90 → 96 → 100). 설치는
 * 그 사이에서 대부분의 시간을 보내므로, 막대가 몇 분 동안 한 픽셀도 움직이지
 * 않는다 — 실제로는 잘 돌고 있는데 멈춘 것처럼 보인다.
 *
 * 그래서 표시용 값을 따로 둔다. 실제 값이 앞서면 그쪽으로 따라붙고, 값이 멈춰
 * 있는 동안에는 다음 이정표를 향해 천천히 차오른다. 다만 이정표에는 닿지 않는다 —
 * 닿으면 아직 시작하지도 않은 단계를 끝난 것처럼 보여 주는 거짓말이 된다.
 */

export type DeployProgressStatus = "running" | "success" | "failed" | "idle";

/** 실제 값이 앞설 때 따라붙는 비율. 한 번에 남은 거리의 이만큼을 좁힌다. */
const CATCH_UP_RATIO = 0.34;
/** 값이 멈춰 있을 때 이정표를 향해 좁히는 비율. 눈에 띄지만 성급하지 않은 속도. */
const CREEP_RATIO = 0.012;
/** 이정표 바로 앞에 남겨 두는 여백. 0 이면 다음 단계를 끝낸 것처럼 보인다. */
const CEILING_MARGIN = 1.5;

export function progressCeiling(target: number, milestones: number[]): number {
  const next = milestones.find((milestone) => milestone > target);
  return next ?? 100;
}

export function nextDisplayProgress(input: {
  current: number;
  target: number;
  ceiling: number;
  status: DeployProgressStatus;
}): number {
  const { current, target, ceiling, status } = input;

  if (status === "success") {
    return 100;
  }
  // 실패한 배포에서 막대가 계속 차오르면 아직 진행 중인 것처럼 읽힌다.
  if (status === "failed") {
    return current;
  }

  // 되감기지 않는다. 뒤로 가면 사용자는 뭔가 잘못됐다고 읽는다.
  if (target > current) {
    const stepped = current + (target - current) * CATCH_UP_RATIO;
    return Math.min(target, Math.max(stepped, current + 0.4));
  }

  if (status !== "running") {
    return current;
  }

  const limit = Math.max(current, ceiling - CEILING_MARGIN);
  if (current >= limit) {
    return current;
  }
  return current + (limit - current) * CREEP_RATIO;
}

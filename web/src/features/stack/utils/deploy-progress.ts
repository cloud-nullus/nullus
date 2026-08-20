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

/**
 * 속도는 전부 "한 틱(약 140ms)에 몇 %p" 로 생각한다.
 *
 * 처음에는 남은 거리에 비례해서만 움직였다. 그런데 install 단계는 15 에서
 * 다음 이정표 90 까지 75 포인트가 남아 있어서, 비례식이 초당 6%p 가 넘는
 * 속도를 냈다 — 막대가 시작하자마자 확 달려 절반을 넘겼다.
 *
 * 그래서 비율에 더해 한 틱에 움직일 수 있는 최대치를 둔다. 거리가 멀어도
 * 걸음 폭은 같고, 가까워지면 비율이 줄어들며 자연히 느려진다.
 */

/** 실제 값이 앞설 때 좁히는 비율. */
const CATCH_UP_RATIO = 0.22;
/** 따라붙을 때의 한 틱 최대 폭. 큰 도약도 몇 초에 걸쳐 미끄러진다. */
const CATCH_UP_MAX_STEP = 1.2;
/** 따라붙을 때의 한 틱 최소 폭. 없으면 남은 거리가 작을 때 영원히 못 닿는다. */
const CATCH_UP_MIN_STEP = 0.15;

/** 값이 멈춰 있을 때 이정표를 향해 좁히는 비율. */
const CREEP_RATIO = 0.0018;
/** 멈춰 있을 때의 한 틱 최대 폭. 초당 약 0.36%p — 분 단위 설치에 맞춘 속도다. */
const CREEP_MAX_STEP = 0.05;

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
    const remaining = target - current;
    const step = Math.min(
      Math.max(remaining * CATCH_UP_RATIO, CATCH_UP_MIN_STEP),
      CATCH_UP_MAX_STEP,
    );
    return Math.min(target, current + step);
  }

  if (status !== "running") {
    return current;
  }

  const limit = Math.max(current, ceiling - CEILING_MARGIN);
  if (current >= limit) {
    return current;
  }
  const step = Math.min((limit - current) * CREEP_RATIO, CREEP_MAX_STEP);
  return Math.min(limit, current + step);
}

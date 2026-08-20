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

/**
 * 값이 멈춰 있을 때 상한을 향해 좁히는 비율.
 *
 * 상한은 이제 "이 스텝이 끝났을 때 닿을 값" 이라 폭이 3%p 안팎이다. 그 폭을
 * 스텝 하나가 걸리는 시간(대개 1~2분)에 걸쳐 메우도록 잡았다 — 시간 상수가
 * 약 1분이다. 막대가 눈에 띄게 움직이는 것은 이 폭이 아니라 표면의 빛과
 * 로켓이 맡는다. 폭은 실제로 한 일만큼만 정직하게 간다.
 */
const CREEP_RATIO = 0.0025;
/** 멈춰 있을 때의 한 틱 최대 폭. 상한이 멀 때(대비책 경로) 급발진을 막는다. */
const CREEP_MAX_STEP = 0.05;

/** 이정표 바로 앞에 남겨 두는 여백. 0 이면 다음 단계를 끝낸 것처럼 보인다. */
const CEILING_MARGIN = 1.5;

/** 서버가 상한을 주지 못했을 때 스텝 하나의 몫으로 가정하는 폭. */
const FALLBACK_STEP_BAND = 3;

/**
 * 표시 값이 넘지 않아야 할 지점.
 *
 * 서버가 "이 스텝이 끝났을 때 닿을 값" 을 함께 보낸다(domain.StepProgressCeiling).
 * 그 값을 그대로 쓴다 — 화면이 단계 경계를 따로 적어 두면 스텝이 늘 때마다
 * 어긋나고, 실제로 그렇게 시크릿만 깔린 시점에 막대가 절반을 넘겼다.
 *
 * 상한을 못 받은 경우에만 스텝 하나의 몫만큼을 가정한다. 멀리 있는 다음 단계를
 * 목표로 삼으면 그 사이를 시간으로만 메우게 되어 같은 문제가 돌아온다.
 */
export function progressCeiling(target: number, serverCeiling?: number): number {
  if (serverCeiling !== undefined && serverCeiling > target) {
    return Math.min(100, serverCeiling);
  }
  return Math.min(100, target + FALLBACK_STEP_BAND);
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
  // 첫 값은 애니메이션 없이 그 자리에 앉힌다.
  //
  // 새로고침하면 표시 값이 0 에서 시작한다. 이때도 기어오르게 두면 이미 40% 인
  // 배포가 0 부터 다시 차오르는 것처럼 보인다 — 새로고침해도 같은 자리여야 한다.
  if (current <= 0 && target > 0) {
    return target;
  }

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

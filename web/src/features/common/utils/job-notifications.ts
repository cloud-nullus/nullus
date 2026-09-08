// 인앱 "작업 알림" 의 도메인 규칙.
//
// 여기서 말하는 작업(job)은 사용자가 시작해 두고 화면을 떠나도 계속 도는 것이다 —
// 스택 설치와 CI/CD 배포 둘뿐이다. F7 의 알림 이력(AlertRule 기반 운영 알림)과는
// 다른 개념이므로 용어를 섞지 않는다: 저쪽은 alert, 이쪽은 job 이다.
//
// 집계 방식은 신규 API 없이 기존 목록 API 두 개(GET /stacks, GET /cicd/deployments)를
// 폴링해 합치는 것으로 정했다. 근거는
// docs/plans/2026-08-23-중간점검-피드백-백로그.md 의 N-1 결정 기록에 있다.
// 두 API 모두 서버가 조직 스코프로 걸러 주므로 화면에서 다시 거르지 않는다.

import type { Deployment, Stack } from '../../../types'

export type JobKind = 'stack' | 'deployment'

/** 작업이 지금 어디에 있는지. 서버의 상태 문자열보다 거친 3분류다. */
export type JobPhase = 'running' | 'succeeded' | 'failed'

export interface JobNotification {
  /** 종류까지 포함한 식별자. 스택 id 와 배포 id 가 겹쳐도 섞이지 않는다. */
  id: string
  kind: JobKind
  refId: string
  name: string
  /** 지금 단계. 서버 상태 문자열 그대로 둔다 — 화면에서 번역한다. */
  stage: string
  phase: JobPhase
  startedAt: string
  /** 진행 → 완료/실패로 바뀐 것을 화면이 본 시각(ms). 진행 중이면 없다. */
  settledAt?: number
  detailPath: string
}

const RUNNING_STATES = new Set([
  'pending',
  'validating',
  'installing',
  'configuring',
  'health_check',
  'rolling_back',
  'running',
  'terminating',
])

const SUCCEEDED_STATES = new Set(['completed', 'success'])

// rolled_back 을 실패로 둔다. 되돌리기 자체는 성공했더라도 사용자가 시킨 일
// (설치)은 남지 않았고, 알림은 시킨 일의 결과를 말해야 한다.
const FAILED_STATES = new Set(['failed', 'cancelled', 'rolled_back'])

/** 상태 문자열의 분류. 작업으로 셀 수 없는 상태면 null 이다. */
export function jobPhaseOf(status: string | undefined | null): JobPhase | null {
  const value = (status ?? '').trim()
  if (RUNNING_STATES.has(value)) return 'running'
  if (SUCCEEDED_STATES.has(value)) return 'succeeded'
  if (FAILED_STATES.has(value)) return 'failed'
  return null
}

/**
 * 두 목록 응답을 하나의 작업 목록으로 옮긴다.
 *
 * 이동 경로는 두 화면 모두 세 역할이 볼 수 있는 라우트를 고른다. 스택은
 * /stack/deploy/:id 가 admin·devops 전용이라 developer 가 누르면 되돌려 보내지므로,
 * 같은 화면을 여는 /stack/logs/:id 를 쓴다.
 */
export function toJobNotifications(
  stacks: Stack[] | undefined,
  deployments: Deployment[] | undefined,
): JobNotification[] {
  const jobs: JobNotification[] = []

  for (const stack of stacks ?? []) {
    const phase = jobPhaseOf(stack.status)
    if (!phase) continue
    jobs.push({
      id: `stack:${stack.id}`,
      kind: 'stack',
      refId: stack.id,
      name: stack.name,
      stage: stack.status,
      phase,
      startedAt: stack.updatedAt || stack.createdAt,
      detailPath: `/stack/logs/${stack.id}`,
    })
  }

  for (const deployment of deployments ?? []) {
    const phase = jobPhaseOf(deployment.status)
    if (!phase) continue
    jobs.push({
      id: `deployment:${deployment.id}`,
      kind: 'deployment',
      refId: deployment.id,
      name: deployment.pipelineName || deployment.pipelineId,
      stage: deployment.status,
      phase,
      startedAt: deployment.startedAt,
      detailPath: `/cicd/pipelines/${deployment.pipelineId}/logs?deploymentId=${deployment.id}`,
    })
  }

  return jobs
}

function rank(job: JobNotification): number {
  return job.phase === 'running' ? 0 : 1
}

function sortJobs(jobs: JobNotification[]): JobNotification[] {
  return [...jobs].sort((a, b) => {
    if (rank(a) !== rank(b)) return rank(a) - rank(b)
    // 끝난 것끼리는 방금 끝난 것이 위로. 진행 중끼리는 늦게 시작한 것이 위로.
    if (a.settledAt && b.settledAt) return b.settledAt - a.settledAt
    return Date.parse(b.startedAt) - Date.parse(a.startedAt) || a.id.localeCompare(b.id)
  })
}

function sameList(a: JobNotification[], b: JobNotification[]): boolean {
  if (a.length !== b.length) return false
  return a.every((job, index) => {
    const other = b[index]
    return (
      job.id === other.id &&
      job.phase === other.phase &&
      job.stage === other.stage &&
      job.name === other.name &&
      job.settledAt === other.settledAt
    )
  })
}

/**
 * 지난 폴링 결과와 이번 응답을 합친다.
 *
 * 규칙은 하나다 — **진행 중인 것을 본 적 있는 작업만 알림이다.** 스택 목록에는
 * 완료된 스택이 계속 들어 있으므로, 그것까지 세면 종이 늘 켜져 있고 드롭다운은
 * 그냥 이력 화면이 된다. 그래서 완료/실패는 "직전에 진행 중으로 싣고 있던 것"
 * 에서만 전환으로 인정하고, retentionMs 동안 보여 준 뒤 내린다.
 *
 * 바뀐 것이 없으면 previous 를 그대로 돌려준다 — 훅이 상태를 갈아 끼우지 않아
 * 렌더가 도는 것을 막는다.
 */
export function mergeJobNotifications(
  previous: JobNotification[],
  incoming: JobNotification[],
  now: number,
  retentionMs: number,
): JobNotification[] {
  const byId = new Map(incoming.map((job) => [job.id, job]))
  const next: JobNotification[] = []

  for (const job of incoming) {
    // 진행 중인 것은 조건 없이 싣는다. 끝났다가 같은 id 로 다시 도는 경우
    // (재시도)도 여기로 들어와 settledAt 없는 새 진행으로 덮인다.
    if (job.phase === 'running') next.push({ ...job })
  }

  for (const job of previous) {
    const current = byId.get(job.id)

    if (job.phase === 'running') {
      if (!current) continue // 목록에서 사라졌다 — 지워졌거나 조회 범위를 벗어났다.
      if (current.phase === 'running') continue // 위에서 이미 실었다.
      next.push({ ...current, settledAt: now })
      continue
    }

    // 이미 끝난 것으로 싣고 있던 작업. 보존 시간이 지나면 내린다.
    if (job.settledAt !== undefined && now - job.settledAt >= retentionMs) continue
    next.push(current ? { ...current, settledAt: job.settledAt } : job)
  }

  const sorted = sortJobs(next)
  return sameList(previous, sorted) ? previous : sorted
}

/** 경과 시간 표기. 로케일을 타지 않게 숫자와 단위 문자만 쓴다. */
export function formatElapsed(elapsedMs: number): string {
  const totalSeconds = Math.max(0, Math.floor(elapsedMs / 1000))
  if (totalSeconds < 60) return `${totalSeconds}s`

  const minutes = Math.floor(totalSeconds / 60)
  if (minutes < 60) return `${minutes}m ${totalSeconds % 60}s`

  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
}

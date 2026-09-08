// 상단 종 아이콘이 보는 데이터.
//
// 새 집계 API 를 만들지 않고 이미 있는 목록 API 두 개를 합친다(N-1 결정,
// docs/plans/2026-08-23-중간점검-피드백-백로그.md). 두 훅 모두 도는 작업이
// 있을 때만 3초 폴링하고, 다 끝나면 스스로 멈춘다. 새 작업을 시작하는
// mutation 이 같은 쿼리 키를 무효화하므로 시작 시점은 폴링 없이도 잡힌다.

import { useEffect, useMemo, useRef, useState } from 'react'
import { useStacks } from '../../stack/api/stack-api'
import { useDeployments } from '../../cicd/api/cicd-api'
import {
  mergeJobNotifications,
  toJobNotifications,
  type JobNotification,
} from '../utils/job-notifications'

/** 도는 작업이 있을 때의 폴링 간격. 스택 목록의 기존 값과 맞춘다. */
const ACTIVE_POLL_MS = 3000

/** 끝난 작업을 종에 남겨 두는 시간. 다른 탭에 있다 돌아와도 결과를 볼 수 있어야 한다. */
export const SETTLED_RETENTION_MS = 5 * 60_000

/** 경과 시간 표시를 갱신하는 주기. */
const TICK_MS = 1000

export interface JobNotificationsState {
  jobs: JobNotification[]
  runningCount: number
  /** 경과 시간 계산의 기준 시각. 초마다 갱신된다. */
  now: number
}

export function useJobNotifications(
  retentionMs: number = SETTLED_RETENTION_MS,
): JobNotificationsState {
  const { data: stackData } = useStacks()
  const { data: deploymentData } = useDeployments(undefined, {
    pollWhileActiveMs: ACTIVE_POLL_MS,
  })

  const incoming = useMemo(
    () => toJobNotifications(stackData?.items, deploymentData?.items),
    [stackData?.items, deploymentData?.items],
  )

  const [jobs, setJobs] = useState<JobNotification[]>([])
  const [now, setNow] = useState(() => Date.now())

  // 응답이 바뀔 때마다 지난 목록과 합쳐 진행 → 완료/실패 전환을 잡는다.
  // mergeJobNotifications 는 바뀐 것이 없으면 같은 배열을 돌려주므로
  // setState 가 렌더를 되풀이시키지 않는다.
  useEffect(() => {
    setJobs((previous) => mergeJobNotifications(previous, incoming, Date.now(), retentionMs))
  }, [incoming, retentionMs])

  // 보여 줄 것이 있을 때만 시계를 돌린다. 경과 시간을 갱신하고, 보존 시간이
  // 지난 완료 항목을 내리는 것도 이 박자에 얹는다 — 응답이 더 오지 않아도
  // (폴링이 멈춘 뒤) 목록이 스스로 비워져야 한다.
  const hasJobs = jobs.length > 0

  // 시계는 응답이 바뀔 때마다 다시 걸지 않는다(초 단위 박자가 끊긴다). 대신
  // 최신 응답을 ref 로 건네준다 — 갱신은 렌더가 아니라 효과에서 한다.
  const incomingRef = useRef(incoming)
  useEffect(() => {
    incomingRef.current = incoming
  }, [incoming])

  useEffect(() => {
    if (!hasJobs) return
    const timer = setInterval(() => {
      const at = Date.now()
      setNow(at)
      setJobs((previous) => mergeJobNotifications(previous, incomingRef.current, at, retentionMs))
    }, TICK_MS)
    return () => clearInterval(timer)
  }, [hasJobs, retentionMs])

  return {
    jobs,
    runningCount: jobs.filter((job) => job.phase === 'running').length,
    now,
  }
}

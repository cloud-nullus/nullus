import type { InternalAxiosRequestConfig } from 'axios'
import type { QueryClient } from '@tanstack/react-query'

import { useTourStore } from '../../stores/tour-store'
import {
  TOUR_CLUSTERS,
  TOUR_PIPELINES,
  TOUR_STACKS,
  TOUR_STACK_MONITORING,
  TOUR_STACK_WORKLOADS,
  TOUR_TEMPLATES,
} from './tour-fixtures'

/**
 * 투어 중에 목록 화면을 채워 줄 재료. 경로 → 응답.
 *
 * 목록만 담는다. 상세까지 흉내 내기 시작하면 화면마다 다른 가짜를 만들게 되고,
 * 그 가짜가 진짜 화면의 계약과 조용히 갈라진다.
 */
const FIXTURES: Record<string, unknown> = {
  '/admin/clusters': TOUR_CLUSTERS,
  '/stacks': TOUR_STACKS,
  '/stacks/templates': TOUR_TEMPLATES,
  '/cicd/pipelines': TOUR_PIPELINES,
}

/**
 * 투어가 직접 열어 보는 상세 화면.
 *
 * 상세를 통째로 흉내 내지는 않는다. 투어가 "한 단계씩" 보여 주겠다고 약속한
 * 곳만 열어 준다 — 워크로드와 모니터링이 비어 있으면 그 걸음이 설명할 것이
 * 없어진다.
 */
const DETAIL_FIXTURES: Array<{ pattern: RegExp; data: unknown }> = [
  { pattern: /^\/stacks\/[^/]+\/workloads$/, data: TOUR_STACK_WORKLOADS },
  { pattern: /^\/stacks\/[^/]+\/monitoring$/, data: TOUR_STACK_MONITORING },
]

/**
 * 이 요청을 투어 재료로 대신할 수 있으면 그 재료를, 아니면 undefined 를 준다.
 *
 * 읽기(GET)만 가로챈다 — 투어 중에 실수로 눌린 생성·삭제가 성공한 것처럼
 * 보이면 사용자는 있지도 않은 변경을 믿게 된다. 그런 요청은 그대로 서버로
 * 보내고 서버의 대답을 받는다.
 */
export function tourFixtureFor(method: string | undefined, url: string | undefined): unknown {
  if ((method ?? 'get').toLowerCase() !== 'get') return undefined
  const path = (url ?? '').split('?')[0].replace(/\/+$/, '')
  if (path in FIXTURES) return FIXTURES[path]
  return DETAIL_FIXTURES.find((rule) => rule.pattern.test(path))?.data
}

/**
 * 투어가 끝나면 목업을 캐시에서 걷어 낸다.
 *
 * 이게 없으면 투어를 끝낸 뒤에도 새로고침 전까지 가짜 스택·파이프라인이 목록에
 * 그대로 남는다 — 사용자는 자기 계정에 없는 것을 있다고 믿게 된다. 화면이 다시
 * 물어보게 만들어야 하므로 무효화가 아니라 제거다.
 */
export function clearTourData(queryClient: QueryClient): void {
  queryClient.clear()
}

/**
 * 투어가 도는 동안에만 응답을 갈아 끼운다.
 *
 * 화면 코드에 "투어일 때는 이 값" 분기를 넣지 않기 위해 경계 한 곳에서만
 * 처리한다. axios 의 adapter 를 이 요청에 한해 바꿔 끼우는 방식이라, 투어가
 * 꺼지면 다음 요청부터 아무 흔적도 남지 않는다.
 */
export function applyTourFixture(config: InternalAxiosRequestConfig): InternalAxiosRequestConfig {
  if (!useTourStore.getState().isActive) return config

  const fixture = tourFixtureFor(config.method, config.url)
  if (fixture === undefined) return config

  config.adapter = async () => ({
    data: fixture,
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  })
  return config
}

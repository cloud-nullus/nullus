import { describe, expect, it } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { clearTourData, tourFixtureFor } from './tour-mock-adapter'
import { TOUR_CLUSTERS, TOUR_PIPELINES, TOUR_STACKS } from './tour-fixtures'

describe('tourFixtureFor', () => {
  it('투어가 훑는 목록 화면에는 재료를 채워 준다', () => {
    expect(tourFixtureFor('get', '/admin/clusters')).toEqual(TOUR_CLUSTERS)
    expect(tourFixtureFor('get', '/stacks')).toEqual(TOUR_STACKS)
    expect(tourFixtureFor('get', '/cicd/pipelines')).toEqual(TOUR_PIPELINES)
  })

  it('쿼리스트링이 붙어도 같은 경로로 본다', () => {
    expect(tourFixtureFor('get', '/stacks?status=running')).toEqual(TOUR_STACKS)
  })

  it('투어가 실제로 훑는 상세 경로만 열어 준다', () => {
    // 워크로드·모니터링은 투어가 "한 단계씩" 보여 주는 곳이라 비워 둘 수 없다.
    expect(tourFixtureFor('get', '/stacks/stk_tour0001/workloads')).toBeDefined()
    expect(tourFixtureFor('get', '/stacks/stk_tour0001/monitoring')).toBeDefined()
  })

  it('그 밖의 상세 경로는 가로채지 않는다', () => {
    // 상세를 통째로 흉내 내기 시작하면 화면마다 다른 가짜를 만들게 되고,
    // 그 가짜가 진짜 화면의 계약과 조용히 갈라진다.
    expect(tourFixtureFor('get', '/stacks/stk_real')).toBeUndefined()
    expect(tourFixtureFor('get', '/stacks/stk_real/history')).toBeUndefined()
  })

  it('읽기가 아닌 요청은 건드리지 않는다', () => {
    // 투어 중에 실수로 눌린 삭제·생성이 성공한 것처럼 보이면 안 된다.
    expect(tourFixtureFor('post', '/stacks')).toBeUndefined()
    expect(tourFixtureFor('delete', '/cicd/pipelines')).toBeUndefined()
  })

  it('모르는 경로는 그대로 서버로 보낸다', () => {
    expect(tourFixtureFor('get', '/observability/alerts')).toBeUndefined()
  })
})

describe('clearTourData', () => {
  it('투어가 끝나면 목업이 캐시에 남지 않는다', () => {
    // 남겨 두면 투어를 끝낸 뒤에도 새로고침 전까지 가짜 스택·파이프라인이
    // 목록에 그대로 보인다.
    const queryClient = new QueryClient()
    queryClient.setQueryData(['stacks'], { items: [{ id: 'tour' }] })

    clearTourData(queryClient)

    expect(queryClient.getQueryData(['stacks'])).toBeUndefined()
  })
})

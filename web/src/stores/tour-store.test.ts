import { beforeEach, describe, expect, it } from 'vitest'
import { useTourStore } from './tour-store'
import { stepsForRole } from '../features/tour/tour-steps'

beforeEach(() => {
  useTourStore.getState().stop()
})

describe('useTourStore', () => {
  it('starts at the first step of the role', () => {
    useTourStore.getState().start('devops')

    const { isActive, stepIndex, steps } = useTourStore.getState()
    expect(isActive).toBe(true)
    expect(stepIndex).toBe(0)
    expect(steps).toEqual(stepsForRole('devops'))
  })

  it('관리자 전용 걸음은 다른 역할의 투어에 끼지 않는다', () => {
    // ProtectedRoute 가 되돌려 보내는 화면으로 안내하면 그 걸음은 빈 화면을
    // 강조하게 된다.
    useTourStore.getState().start('developer')

    expect(useTourStore.getState().steps.map((step) => step.id)).not.toContain('registerCluster')
  })

  it('next 는 마지막 걸음에서 투어를 끝낸다', () => {
    useTourStore.getState().start('admin')
    const last = useTourStore.getState().steps.length - 1
    useTourStore.getState().goTo(last)

    useTourStore.getState().next()

    expect(useTourStore.getState().isActive).toBe(false)
  })

  it('prev 는 첫 걸음 아래로 내려가지 않는다', () => {
    // 0 에서 뒤로 누르면 -1 이 되어 현재 걸음이 undefined 가 된다.
    useTourStore.getState().start('admin')

    useTourStore.getState().prev()

    expect(useTourStore.getState().stepIndex).toBe(0)
    expect(useTourStore.getState().isActive).toBe(true)
  })

  it('멈추면 걸음 위치가 처음으로 돌아간다', () => {
    useTourStore.getState().start('admin')
    useTourStore.getState().next()
    useTourStore.getState().stop()

    expect(useTourStore.getState().isActive).toBe(false)
    expect(useTourStore.getState().stepIndex).toBe(0)
  })
})

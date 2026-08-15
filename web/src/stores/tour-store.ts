import { create } from 'zustand'

import { stepsForRole, type TourStep } from '../features/tour/tour-steps'
import type { Role } from '../types'

interface TourState {
  isActive: boolean
  stepIndex: number
  /** 시작할 때 역할로 걸러 둔 걸음들. 투어가 도는 동안 바뀌지 않는다. */
  steps: TourStep[]
  start: (role: Role) => void
  stop: () => void
  next: () => void
  prev: () => void
  goTo: (index: number) => void
}

export const useTourStore = create<TourState>((set, get) => ({
  isActive: false,
  stepIndex: 0,
  steps: [],

  start: (role) => set({ isActive: true, stepIndex: 0, steps: stepsForRole(role) }),

  // 걸음 위치도 함께 되돌린다. 남겨 두면 다음 시작이 중간부터 열린다.
  stop: () => set({ isActive: false, stepIndex: 0 }),

  next: () => {
    const { stepIndex, steps } = get()
    if (stepIndex >= steps.length - 1) {
      set({ isActive: false, stepIndex: 0 })
      return
    }
    set({ stepIndex: stepIndex + 1 })
  },

  // 첫 걸음 아래로 내려가면 현재 걸음이 undefined 가 되어 화면이 빈 상자를 그린다.
  prev: () => set((state) => ({ stepIndex: Math.max(0, state.stepIndex - 1) })),

  goTo: (index) =>
    set((state) => ({ stepIndex: Math.min(Math.max(0, index), Math.max(0, state.steps.length - 1)) })),
}))

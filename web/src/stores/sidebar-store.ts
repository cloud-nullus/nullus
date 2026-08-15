import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'

interface SidebarState {
  collapsed: boolean
  toggleSidebar: () => void
}

// 모바일 폭 미만에서는 240px 사이드바가 본문을 짓눌러 화면이 깨진다(EPIC #36).
// 1차 최소 대응: 진입 시 기본 collapse(48px 레일)로 두어 본문 폭을 확보한다.
// (오프캔버스 드로어는 2차 후속. resize 중 재판정은 1차 범위 밖 — 진입 시점 기준.)
const MOBILE_BREAKPOINT = 768

const getInitialCollapsed = (): boolean => {
  if (typeof window !== 'undefined' && window.innerWidth < MOBILE_BREAKPOINT) {
    return true
  }
  const stored = localStorage.getItem('nullus-sidebar-collapsed')
  return stored === 'true'
}

export const useSidebarStore = create<SidebarState>()(
  subscribeWithSelector((set) => ({
    collapsed: getInitialCollapsed(),
    toggleSidebar: () => set((state) => ({ collapsed: !state.collapsed })),
  }))
)

useSidebarStore.subscribe(
  (state) => state.collapsed,
  (collapsed) => {
    localStorage.setItem('nullus-sidebar-collapsed', String(collapsed))
  }
)

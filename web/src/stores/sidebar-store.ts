import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'

interface SidebarState {
  collapsed: boolean
  toggleSidebar: () => void
}

const getInitialCollapsed = (): boolean => {
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

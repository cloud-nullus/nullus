import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'
import type { Theme } from '../types'

interface ThemeState {
  theme: Theme
  toggleTheme: () => void
}

const getInitialTheme = (): Theme => {
  const stored = localStorage.getItem('nullus-theme')
  if (stored === 'light' || stored === 'dark') return stored
  return 'dark'
}

export const useThemeStore = create<ThemeState>()(
  subscribeWithSelector((set) => ({
    theme: getInitialTheme(),
    toggleTheme: () =>
      set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),
  }))
)

useThemeStore.subscribe(
  (state) => state.theme,
  (theme) => {
    localStorage.setItem('nullus-theme', theme)
    document.documentElement.setAttribute('data-theme', theme)
  },
  { fireImmediately: true }
)

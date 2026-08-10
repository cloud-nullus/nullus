import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'
import type { Theme } from '../types'

interface ThemeState {
  theme: Theme
  toggleTheme: () => void
}

/** 테마 영속화 키. MUI 의 modeStorageKey 와 공유해 두 시스템이 갈라지지 않게 한다. */
export const THEME_STORAGE_KEY = 'nullus-theme'

export const getInitialTheme = (): Theme => {
  const stored = localStorage.getItem(THEME_STORAGE_KEY)
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
    localStorage.setItem(THEME_STORAGE_KEY, theme)
    document.documentElement.setAttribute('data-theme', theme)
  },
  { fireImmediately: true }
)

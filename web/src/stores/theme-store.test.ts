import { describe, it, expect, beforeEach } from 'vitest'
import { useThemeStore } from './theme-store'

beforeEach(() => {
  localStorage.clear()
  useThemeStore.setState({ theme: 'dark' })
})

describe('theme-store', () => {
  it('initial theme is dark by default', () => {
    expect(useThemeStore.getState().theme).toBe('dark')
  })

  it('toggleTheme switches dark to light', () => {
    useThemeStore.setState({ theme: 'dark' })
    useThemeStore.getState().toggleTheme()
    expect(useThemeStore.getState().theme).toBe('light')
  })

  it('toggleTheme switches light to dark', () => {
    useThemeStore.setState({ theme: 'light' })
    useThemeStore.getState().toggleTheme()
    expect(useThemeStore.getState().theme).toBe('dark')
  })

  it('persists theme to localStorage on toggle', () => {
    useThemeStore.setState({ theme: 'dark' })
    useThemeStore.getState().toggleTheme()
    expect(localStorage.getItem('nullus-theme')).toBe('light')
  })
})

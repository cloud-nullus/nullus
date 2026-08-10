// MUI 와 zustand 가 data-theme 속성을 다투지 않는지 검증한다.
//
// 회귀 배경: MUI ThemeProvider 에 cssVariables + colorSchemeSelector '[data-theme=%s]' 를
// 주면 MUI 가 <html data-theme> 를 직접 관리한다. 앱은 이미 theme-store 가 같은 속성을
// 쓰고 있었고, MUI 의 defaultMode="dark" 가 스토어의 light 를 덮어써서 라이트 테마가
// 다크로 렌더됐다. 시각 회귀 스냅샷에서 발견됐다.

import { describe, expect, it, beforeEach } from 'vitest'
import { render, waitFor } from '@testing-library/react'
import { ThemeProvider } from '@mui/material/styles'
import { THEME_STORAGE_KEY, getInitialTheme, useThemeStore } from '../stores/theme-store'
import { nullusTheme } from './mui-theme'
import { ThemeSync } from './theme-sync'

function renderWithTheme() {
  return render(
    <ThemeProvider theme={nullusTheme} defaultMode={getInitialTheme()} modeStorageKey={THEME_STORAGE_KEY}>
      <ThemeSync />
      <div>content</div>
    </ThemeProvider>,
  )
}

describe('테마 소유권 브릿지', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('스토어가 light 면 MUI 도 light 로 맞춘다', async () => {
    useThemeStore.setState({ theme: 'light' })
    renderWithTheme()

    await waitFor(() => {
      expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    })
  })

  it('스토어가 dark 면 MUI 도 dark 로 맞춘다', async () => {
    useThemeStore.setState({ theme: 'dark' })
    renderWithTheme()

    await waitFor(() => {
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })
  })

  it('토글이 두 시스템에 함께 반영된다', async () => {
    useThemeStore.setState({ theme: 'dark' })
    renderWithTheme()

    await waitFor(() => {
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })

    useThemeStore.getState().toggleTheme()

    await waitFor(() => {
      expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    })
  })

  it('MUI 와 스토어가 같은 localStorage 키를 쓴다', () => {
    // 키가 갈라지면 새로고침 후 두 시스템의 초기값이 어긋난다.
    expect(THEME_STORAGE_KEY).toBe('nullus-theme')

    // 스토어 구독은 값이 실제로 바뀔 때만 발화하므로, 반대 값을 거쳐 전이를 만든다.
    // (같은 값으로 setState 하면 아무 일도 일어나지 않는다)
    useThemeStore.setState({ theme: 'dark' })
    useThemeStore.setState({ theme: 'light' })
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
  })
})

// 테마 소유권 브릿지.
//
// 문제: MUI 테마에 cssVariables + colorSchemeSelector '[data-theme=%s]' 를 주면
// MUI 가 <html data-theme> 속성을 직접 관리한다. 그런데 이 앱은 이미 zustand
// theme-store 가 같은 속성을 쓴다. 둘이 같은 속성을 다투면 나중에 쓴 쪽이 이기고,
// 실제로 MUI 의 defaultMode 가 스토어 값을 덮어써서 라이트 테마가 다크로 렌더됐다.
//
// 해결: 스토어를 단일 소유자로 두고 MUI 가 따라오게 한다.
//   - 영속화 키를 공유한다 (THEME_STORAGE_KEY ↔ modeStorageKey)
//   - 스토어가 바뀌면 MUI 의 setMode 로 밀어 넣는다
// 두 시스템이 같은 값을 쓰게 되므로 속성 경쟁이 사라진다.

import { useEffect } from 'react'
import { useColorScheme } from '@mui/material/styles'
import { useThemeStore } from '../stores/theme-store'

export function ThemeSync() {
  const theme = useThemeStore((state) => state.theme)
  const { mode, setMode } = useColorScheme()

  useEffect(() => {
    if (mode !== theme) setMode(theme)
  }, [theme, mode, setMode])

  return null
}

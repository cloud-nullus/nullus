// AG Grid 테마 — web/DESIGN.md 에서 파생된다.
//
// AG Grid v33+ 는 CSS 파일 import 대신 Theming API 를 쓴다. themeQuartz 에
// withParams(파라미터, 모드이름) 을 체이닝하면 모드별 값이 생기고,
// `document.body.dataset.agThemeMode` 로 모드를 고른다.
//
// 앱의 테마 토글은 `document.documentElement[data-theme]` 를 쓰므로(stores/theme-store.ts),
// 두 축을 잇는 동기화는 useAgThemeMode() 가 담당한다.
//
// Community(MIT) 만 쓴다. ag-grid-enterprise 를 import 하지 않는다 — 상용 라이선스다.

import { themeQuartz } from 'ag-grid-community'
import { useEffect } from 'react'
import { darkTokens, lightTokens, rounded, typography } from './tokens.generated'

const px = (value: string) => Number.parseFloat(value)

const paramsFor = (t: typeof lightTokens, browserColorScheme: 'light' | 'dark') => ({
  backgroundColor: t.surface,
  foregroundColor: t.text,
  borderColor: t.divider,
  chromeBackgroundColor: t.surfaceSunken,
  headerBackgroundColor: t.surfaceSunken,
  headerTextColor: t.textSecondary,
  oddRowBackgroundColor: t.surface,
  rowHoverColor: `color-mix(in srgb, ${t.primary} 6%, transparent)`,
  selectedRowBackgroundColor: `color-mix(in srgb, ${t.primary} 12%, transparent)`,
  accentColor: t.primary,
  invalidColor: t.error,
  browserColorScheme,

  // 밀도 — DESIGN.md §Components. 행 40px / 헤더 36px 을 맞춘다.
  spacing: 6,
  rowVerticalPaddingScale: 0.9,
  headerHeight: 36,
  rowHeight: 40,
  fontFamily: `'${typography['body-sm'].fontFamily}', 'Pretendard', sans-serif`,
  fontSize: px(typography['body-sm'].fontSize) * 16,
  headerFontSize: px(typography.overline.fontSize) * 16,
  headerFontWeight: typography.overline.fontWeight,
  borderRadius: px(rounded.lg),
  wrapperBorderRadius: px(rounded.lg),
})

/**
 * 두 모드를 가진 단일 테마 객체. `<AgGridReact theme={nullusGridTheme} />` 로 넘긴다.
 * 모드 이름은 AG Grid 규약대로 'light' / 'dark' 를 쓴다.
 */
export const nullusGridTheme = themeQuartz
  .withParams(paramsFor(lightTokens, 'light'), 'light')
  .withParams(paramsFor(darkTokens, 'dark'), 'dark')

/**
 * 앱 테마([data-theme])를 AG Grid 모드(body[data-ag-theme-mode])에 동기화한다.
 * AppLayout 에서 한 번만 호출한다.
 */
export function useAgThemeMode(theme: 'light' | 'dark'): void {
  useEffect(() => {
    document.body.dataset.agThemeMode = theme
  }, [theme])
}

// MUI 테마 — web/DESIGN.md 에서 파생된다.
//
// 값은 tokens.generated.ts 에서만 읽는다. 이 파일에 hex 를 쓰지 않는다.
// 여기서 하는 일은 두 가지다:
//   1. DESIGN.md 의 토큰을 MUI 팔레트 구조로 옮긴다
//   2. DESIGN.md §Components 의 밀도 규칙을 컴포넌트 기본값으로 못박는다
//
// 밀도가 중요하다: MUI 기본 여백을 그대로 쓰면 한 화면에 들어가던 표 행이 줄고,
// 그건 정보 손실과 같다(기획안 §3 원칙 6). 그래서 size="small" 을 전역 기본값으로 둔다.

import { createTheme, type Theme } from '@mui/material/styles'
import { brandTokens, darkTokens, elevation, lightTokens, rounded, typography } from './tokens.generated'

const px = (value: string) => Number.parseFloat(value)

/** DESIGN.md 의 typography 토큰을 MUI variant 로 옮긴다. */
const typographyVariant = (token: {
  fontFamily: string
  fontSize: string
  fontWeight?: number
  lineHeight?: number
  letterSpacing?: string
}) => ({
  fontFamily: `'${token.fontFamily}', 'Pretendard', -apple-system, BlinkMacSystemFont, sans-serif`,
  fontSize: token.fontSize,
  fontWeight: token.fontWeight,
  lineHeight: token.lineHeight,
  letterSpacing: token.letterSpacing,
})

const paletteFor = (t: typeof lightTokens, mode: 'light' | 'dark') => ({
  mode,
  primary: { main: t.primary, contrastText: t.onPrimary },
  secondary: { main: t.accentAlt, contrastText: t.onPrimary },
  success: { main: t.success },
  warning: { main: t.warning },
  error: { main: t.error },
  info: { main: t.info },
  background: { default: t.bg, paper: t.surface },
  text: { primary: t.text, secondary: t.textSecondary, disabled: t.textMuted },
  divider: t.divider,
})

export const nullusTheme: Theme = createTheme({
  // 앱은 이미 `document.documentElement[data-theme]` 로 테마를 토글한다(stores/theme-store.ts).
  // MUI 도 같은 셀렉터를 쓰게 해 전환 지점을 하나로 유지한다.
  cssVariables: { colorSchemeSelector: '[data-theme=%s]' },

  // Tailwind v4 와 함께 쓰기 위한 레이어 순서. index.css 의 @layer 선언과 반드시 같아야 한다.
  modularCssLayers: '@layer theme, base, mui, components, utilities;',

  colorSchemes: {
    light: { palette: paletteFor(lightTokens, 'light') },
    dark: { palette: paletteFor(darkTokens, 'dark') },
  },

  shape: { borderRadius: px(rounded.lg) },

  typography: {
    fontFamily: `'Inter', 'Pretendard', -apple-system, BlinkMacSystemFont, sans-serif`,
    h1: typographyVariant(typography.h1),
    h2: typographyVariant(typography.h2),
    h3: typographyVariant(typography.h3),
    body1: typographyVariant(typography['body-md']),
    body2: typographyVariant(typography['body-sm']),
    button: { ...typographyVariant(typography['label-sm']), textTransform: 'none' },
    caption: typographyVariant(typography['label-sm']),
    overline: { ...typographyVariant(typography.overline), textTransform: 'uppercase' },
  },

  components: {
    // ── 밀도 (DESIGN.md §Components) ──────────────────────────────
    MuiButton: {
      defaultProps: { size: 'small', disableElevation: true },
      styleOverrides: {
        root: {
          borderRadius: rounded.md,
          paddingInline: 'var(--space-md)',
          minHeight: 'var(--control-height)',
          // 아이콘을 startIcon 이 아니라 children 으로 넘기는 호출부가 대부분이라
          // JSX 줄바꿈이 공백을 만드느냐에 따라 아이콘이 글자에 붙었다 떨어졌다 했다.
          gap: 'var(--space-xs)',
        },
      },
    },
    MuiTextField: { defaultProps: { size: 'small' } },
    MuiSelect: { defaultProps: { size: 'small' } },
    MuiOutlinedInput: {
      defaultProps: { size: 'small' },
      styleOverrides: { root: { borderRadius: rounded.sm } },
    },
    MuiIconButton: { defaultProps: { size: 'small' } },
    MuiChip: {
      defaultProps: { size: 'small' },
      styleOverrides: { root: { borderRadius: rounded.full, fontWeight: 600 } },
    },
    MuiTable: { defaultProps: { size: 'small' } },
    MuiTableCell: {
      styleOverrides: {
        root: { borderColor: 'var(--color-border-default)' },
        head: {
          backgroundColor: 'var(--color-surface-sunken)',
          color: 'var(--color-text-secondary)',
          textTransform: 'uppercase',
          letterSpacing: typography.overline.letterSpacing,
          fontSize: typography.overline.fontSize,
          fontWeight: typography.overline.fontWeight,
        },
      },
    },

    // ── 깊이 (DESIGN.md §Elevation & Depth) ────────────────────────
    // 라이트는 그림자로, 다크는 표면 밝기 차로 깊이를 만든다.
    // 그래서 shadow 값을 하드코딩하지 않고 테마별 토큰(--shadow-*)을 참조한다.
    MuiPaper: {
      defaultProps: { elevation: 0 },
      styleOverrides: {
        root: {
          backgroundImage: 'none',
          backgroundColor: 'var(--color-surface-card)',
          borderRadius: rounded.lg,
        },
        outlined: { borderColor: 'var(--color-border-default)' },
      },
    },
    MuiDialog: {
      styleOverrides: {
        paper: {
          backgroundColor: 'var(--color-surface-raised)',
          boxShadow: 'var(--shadow-overlay)',
          border: '1px solid var(--color-border-default)',
        },
      },
    },
    MuiPopover: {
      styleOverrides: {
        paper: {
          backgroundColor: 'var(--color-surface-raised)',
          boxShadow: 'var(--shadow-overlay)',
          border: '1px solid var(--color-border-default)',
        },
      },
    },
    MuiMenu: {
      styleOverrides: {
        paper: {
          backgroundColor: 'var(--color-surface-raised)',
          boxShadow: 'var(--shadow-overlay)',
          border: '1px solid var(--color-border-default)',
        },
      },
    },
    MuiBackdrop: {
      styleOverrides: { root: { backgroundColor: 'var(--color-surface-overlay)' } },
    },

    // 포커스 링은 index.css 의 *:focus-visible 규칙과 같은 모양을 쓴다.
    MuiCssBaseline: {
      styleOverrides: {
        'a, button': { WebkitTapHighlightColor: 'transparent' },
      },
    },
  },
})

/** 브랜드 골드 CTA. 면(배경)으로만 쓴다 — DESIGN.md §Colors */
export const brandCtaSx = {
  background: `linear-gradient(135deg, ${brandTokens.gold}, ${brandTokens.goldEnd})`,
  color: brandTokens.onGold,
  fontWeight: 700,
  '&:hover': {
    background: `linear-gradient(135deg, ${brandTokens.gold}, ${brandTokens.goldEnd})`,
    filter: 'brightness(1.05)',
  },
} as const

export { elevation }

/**
 * DESIGN.md 에서 자동 생성됨 — 직접 고치지 않는다.
 * 값을 바꾸려면 web/DESIGN.md 를 고치고 `npm run theme:generate` 를 실행한다.
 */

export interface SchemeTokens {
  bg: string
  surface: string
  surfaceSunken: string
  surfaceRaised: string
  divider: string
  dividerStrong: string
  text: string
  textSecondary: string
  textMuted: string
  primary: string
  onPrimary: string
  success: string
  warning: string
  error: string
  info: string
  accentAlt: string
  scrim: string
}

const schemeFrom = (p: Record<string, string>): SchemeTokens => ({
  bg: p['bg'],
  surface: p['surface'],
  // 라이트는 sunken(표 헤더 등), 다크는 raised(오버레이)만 정의한다.
  // 없는 쪽은 surface 로 접어 두 스킴이 같은 형태를 갖게 한다.
  surfaceSunken: p['surface-sunken'] ?? p['surface'],
  surfaceRaised: p['surface-raised'] ?? p['surface'],
  divider: p['divider'],
  dividerStrong: p['divider-strong'],
  text: p['text'],
  textSecondary: p['text-secondary'],
  textMuted: p['text-muted'],
  primary: p['primary'],
  onPrimary: p['on-primary'],
  success: p['success'],
  warning: p['warning'],
  error: p['error'],
  info: p['info'],
  accentAlt: p['accent-alt'],
  scrim: p['scrim'],
})

export const lightTokens: SchemeTokens = schemeFrom({
  "bg": "#f7f7f7",
  "surface": "#ffffff",
  "surface-sunken": "#eef0f3",
  "divider": "#dee1e6",
  "divider-strong": "#c8ccd2",
  "text": "#0a0b0d",
  "text-secondary": "#5b616e",
  "text-muted": "#686d75",
  "primary": "#0052ff",
  "on-primary": "#ffffff",
  "success": "#047a48",
  "warning": "#a35a00",
  "error": "#cf202f",
  "info": "#0052ff",
  "accent-alt": "#6b3fd4",
  "terminal-bg": "#16181c",
  "terminal-text": "#d8dbe0",
  "terminal-muted": "#9aa2ad",
  "terminal-info": "#7db3ff",
  "terminal-success": "#4fd18b",
  "terminal-warning": "#f2c14e",
  "terminal-error": "#ff8a80",
  "scrim": "rgba(10, 11, 13, 0.40)"
})

export const darkTokens: SchemeTokens = schemeFrom({
  "bg": "#0a0b0d",
  "surface": "#16181c",
  "surface-sunken": "#1e2126",
  "surface-raised": "#1e2126",
  "divider": "#2a2e35",
  "divider-strong": "#3a3f47",
  "text": "#ffffff",
  "text-secondary": "#a8acb3",
  "text-muted": "#8b9098",
  "primary": "#4d8cff",
  "on-primary": "#0a0b0d",
  "success": "#2ecc84",
  "warning": "#f4b000",
  "error": "#ff6b74",
  "info": "#4d8cff",
  "accent-alt": "#b39bff",
  "terminal-bg": "#0d0f17",
  "terminal-text": "#c9d1d9",
  "terminal-muted": "#8b949e",
  "terminal-info": "#58a6ff",
  "terminal-success": "#3fb950",
  "terminal-warning": "#d29922",
  "terminal-error": "#f85149",
  "scrim": "rgba(0, 0, 0, 0.72)"
})

export const brandTokens = {
  "gold": "#f4b000",
  "goldEnd": "#d99b00",
  "onGold": "#0a0b0d"
} as const

export const elevation = {
  "flat": "none",
  "raised": "0 1px 2px rgba(15, 23, 42, 0.06)",
  "overlay": "0 8px 24px rgba(15, 23, 42, 0.12)",
  "raised-dark": "none",
  "overlay-dark": "none"
} as const

export const spacing = {
  "xs": "4px",
  "sm": "8px",
  "md": "12px",
  "lg": "16px",
  "xl": "24px",
  "2xl": "32px",
  "3xl": "48px"
} as const

export const rounded = {
  "sm": "4px",
  "md": "6px",
  "lg": "8px",
  "full": "9999px"
} as const

export const typography = {
  "h1": {
    "fontFamily": "Inter",
    "fontSize": "1.375rem",
    "fontWeight": 700,
    "lineHeight": 1.25,
    "letterSpacing": "-0.01em"
  },
  "h2": {
    "fontFamily": "Inter",
    "fontSize": "1.125rem",
    "fontWeight": 700,
    "lineHeight": 1.3
  },
  "h3": {
    "fontFamily": "Inter",
    "fontSize": "0.875rem",
    "fontWeight": 700,
    "lineHeight": 1.4
  },
  "body-md": {
    "fontFamily": "Inter",
    "fontSize": "0.875rem",
    "fontWeight": 400,
    "lineHeight": 1.6
  },
  "body-sm": {
    "fontFamily": "Inter",
    "fontSize": "0.8125rem",
    "fontWeight": 400,
    "lineHeight": 1.5
  },
  "label-sm": {
    "fontFamily": "Inter",
    "fontSize": "0.75rem",
    "fontWeight": 600,
    "lineHeight": 1.5
  },
  "overline": {
    "fontFamily": "Inter",
    "fontSize": "0.6875rem",
    "fontWeight": 600,
    "lineHeight": 1.4,
    "letterSpacing": "0.06em"
  },
  "code": {
    "fontFamily": "Fira Code",
    "fontSize": "0.8125rem",
    "fontWeight": 400,
    "lineHeight": 1.5
  }
} as const

export const layout = {
  "sidebar-width": "240px",
  "sidebar-collapsed": "48px",
  "header-height": "44px",
  "page-padding": "24px",
  "page-padding-y": "20px",
  "card-padding": "12px",
  "grid-gap": "8px",
  "icon-size": "28px",
  "table-cell-px": "12px",
  "table-row-height": "32px",
  "table-header-height": "28px",
  "control-height": "30px"
} as const

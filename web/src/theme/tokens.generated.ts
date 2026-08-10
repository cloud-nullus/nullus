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
  "bg": "#f4f6f8",
  "surface": "#ffffff",
  "surface-sunken": "#eef2f6",
  "divider": "#cbd5e1",
  "divider-strong": "#b9c3cf",
  "text": "#0f172a",
  "text-secondary": "#475569",
  "text-muted": "#5f6f85",
  "primary": "#4338ca",
  "on-primary": "#ffffff",
  "success": "#047857",
  "warning": "#a15c07",
  "error": "#c81e1e",
  "info": "#1d4ed8",
  "accent-alt": "#6d28d9",
  "scrim": "rgba(15, 23, 42, 0.45)"
})

export const darkTokens: SchemeTokens = schemeFrom({
  "bg": "#0a0a0a",
  "surface": "#0f1419",
  "surface-raised": "#161d26",
  "divider": "#2d3748",
  "divider-strong": "#4a5568",
  "text": "#f1f5f9",
  "text-secondary": "#94a3b8",
  "text-muted": "#8496a9",
  "primary": "#8f9bff",
  "on-primary": "#0a0a0a",
  "success": "#3ddc97",
  "warning": "#f5b544",
  "error": "#ff8080",
  "info": "#6aa8fb",
  "accent-alt": "#c4b5fd",
  "scrim": "rgba(0, 0, 0, 0.7)"
})

export const brandTokens = {
  "gold": "#ffd700",
  "goldEnd": "#f59e0b",
  "onGold": "#1a1d29"
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
  "sm": "6px",
  "md": "10px",
  "lg": "12px",
  "full": "9999px"
} as const

export const typography = {
  "h1": {
    "fontFamily": "Inter",
    "fontSize": "2rem",
    "fontWeight": 800,
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

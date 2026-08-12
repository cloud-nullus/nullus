// 외부 도구의 브랜드 색.
//
// 여기 있는 값들은 **디자인 토큰이 아니다.** GitLab 의 주황, Kubernetes 의 파랑은
// 그 프로젝트가 정한 고정값이고, 우리 팔레트를 바꾼다고 따라 바뀌면 안 된다 —
// 로고를 알아보게 하는 것이 목적이라서다. 우리 로고의 brand-gold 가 CTA 색이
// 아닌 것과 같은 이유로, 이 색들도 UI 색으로 쓰지 않는다. 도구 아이콘의 배경
// 한 곳에만 쓴다.
//
// TSX 에 흩어져 있으면 "TSX 에 hex 를 박지 않는다" 규칙과 계속 충돌한다.
// (그 규칙은 우리 팔레트를 지키려는 것이지 외부 브랜드를 막으려는 게 아니다.)
// 여기로 모으고 eslint.config.js 에서 이 파일 하나만 면제한다.

export const TOOL_BRAND_GRADIENT = {
  gitlab: 'linear-gradient(135deg,#fc6d26,#e24329)',
  nexus: 'linear-gradient(135deg,#e6522c,#cc3918)',
  argocd: 'linear-gradient(135deg,#f46800,#d45a00)',
  kubernetes: 'linear-gradient(135deg,#326ce5,#1e4db8)',
} as const

export type ToolBrandKey = keyof typeof TOOL_BRAND_GRADIENT

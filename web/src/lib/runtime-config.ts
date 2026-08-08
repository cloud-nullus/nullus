/**
 * 런타임 설정.
 *
 * Vite 는 import.meta.env 를 빌드 시점에 문자열로 인라인한다. 그래서 OIDC issuer
 * 같은 환경별 값을 VITE_* 로만 받으면 **환경마다 이미지를 다시 빌드해야 한다**.
 * 실제로 ghcr 에 게시된 web 이미지는 issuer 가 `keycloak.nullus.internal` 로 박혀
 * 있어 다른 클러스터에서는 로그인이 되지 않았다.
 *
 * 그래서 컨테이너 기동 시 nginx 가 `/config.js` 를 만들어 window 에 값을 실어 주고,
 * 앱은 그 값을 우선 읽는다. 없으면 빌드 시 값으로 폴백하므로 로컬 개발은 그대로다.
 *
 *   우선순위: window.__NULLUS_CONFIG__  >  import.meta.env.VITE_*  >  기본값
 */

export interface NullusRuntimeConfig {
  authMode?: string
  oidcProvider?: string
  oidcAuthority?: string
  oidcClientId?: string
}

declare global {
  interface Window {
    __NULLUS_CONFIG__?: NullusRuntimeConfig
  }
}

/**
 * 컨테이너가 값을 주입하지 않은 자리는 치환되지 않은 플레이스홀더(`__NULLUS_...__`)
 * 로 남는다. 그런 값은 설정이 없는 것으로 본다.
 */
function clean(value: string | undefined): string | undefined {
  if (!value) return undefined
  const trimmed = value.trim()
  if (!trimmed || trimmed.startsWith('__NULLUS_')) return undefined
  return trimmed
}

function runtime(): NullusRuntimeConfig {
  return (typeof window !== 'undefined' && window.__NULLUS_CONFIG__) || {}
}

/** 런타임 값 → 빌드 시 값 → 기본값 순으로 고른다. */
export function resolveConfig(
  key: keyof NullusRuntimeConfig,
  buildTimeValue: string | undefined,
  fallback: string,
): string {
  return clean(runtime()[key]) ?? clean(buildTimeValue) ?? fallback
}

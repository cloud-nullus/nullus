// When oidc-client-ts / react-oidc-context are installed, replace these with:
//   import type { User } from 'oidc-client-ts'
//   import type { AuthProviderProps } from 'react-oidc-context'

import { resolveConfig } from './runtime-config'

export interface OIDCUser {
  profile: Record<string, unknown>
  id_token?: string
  access_token?: string
}

export interface OIDCAuthProviderProps {
  authority: string
  client_id: string
  redirect_uri: string
  post_logout_redirect_uri?: string
  scope?: string
  automaticSilentRenew?: boolean
  onSigninCallback?: () => void
}

export type OIDCProviderType = 'keycloak' | 'authentik'

export interface OIDCProviderConfig {
  type: OIDCProviderType
  authority: string
  clientId: string
  scope: string
  extractRoles: (user: OIDCUser) => string[]
  getLogoutUrl?: (idToken: string, redirectUri: string) => string
}

/**
 * Keycloak 의 realm 롤을 읽는다.
 *
 * Keycloak 은 `realm_access` 를 기본적으로 **액세스 토큰에만** 싣는다. ID 토큰에도
 * 넣으려면 realm-roles 매퍼에 `id.token.claim=true` 를 켜야 하는데, 그 설정에
 * 의존하면 서버 구성이 조금만 달라져도 모든 사용자가 최저 권한으로 떨어진다
 * (실제로 admin·devops 계정이 전부 developer 로 보였다).
 *
 * 그래서 ID 토큰(profile)을 먼저 보고, 없으면 액세스 토큰을 직접 열어 본다.
 * 액세스 토큰의 realm_access 는 Keycloak 이 항상 넣어 주므로 설정과 무관하게 동작한다.
 */
function keycloakExtractRoles(user: OIDCUser): string[] {
  const fromProfile = (user.profile?.realm_access as { roles?: string[] } | undefined)?.roles
  if (fromProfile?.length) return fromProfile
  const claims = decodeJwtPayload(user.access_token)
  return (claims?.realm_access as { roles?: string[] } | undefined)?.roles ?? []
}

// Authentik stores roles at profile.groups (flat array via profile scope)
function authentikExtractRoles(user: OIDCUser): string[] {
  return (user.profile?.groups as string[]) ?? []
}

/**
 * ID 토큰이 지금 쓰는 클라이언트로 발급된 것인지 확인한다.
 *
 * Keycloak 은 `id_token_hint` 가 오면 그 토큰의 발급 대상과 `client_id` 가 같은지
 * 대조하고, 다르면 로그아웃을 통째로 거부한다.
 *
 *   We are sorry... Invalid parameter: id_token_hint
 *
 * 클라이언트 ID 를 바꾼 뒤 브라우저에 이전 세션이 남아 있으면 정확히 이 상태가 되고,
 * 사용자는 **로그아웃도 못 하는** 막다른 화면에 갇힌다. 그래서 어긋난 토큰은
 * 힌트로 쓰지 않고 client_id 만으로 로그아웃한다.
 *
 * 서명은 검증하지 않는다 — 여기서 필요한 건 "이 힌트를 보내도 되는가" 뿐이고,
 * 실제 검증은 Keycloak 이 한다.
 */
export function decodeJwtPayload(token: string | undefined): Record<string, unknown> | null {
  const payload = token?.split('.')[1]
  if (!payload) return null
  try {
    return JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/'))) as Record<string, unknown>
  } catch {
    return null
  }
}

export function isTokenForClient(idToken: string, clientId: string): boolean {
  if (!idToken || !clientId) return false
  const claims = decodeJwtPayload(idToken) as { azp?: string; aud?: string | string[] } | null
  if (!claims) return false
  if (claims.azp) return claims.azp === clientId
  if (Array.isArray(claims.aud)) return claims.aud.includes(clientId)
  return claims.aud === clientId
}

export function getProviderConfig(): OIDCProviderConfig {
  const provider = resolveConfig(
    'oidcProvider',
    import.meta.env.VITE_OIDC_PROVIDER,
    'keycloak',
  ) as OIDCProviderType
  const authority = resolveConfig(
    'oidcAuthority',
    import.meta.env.VITE_OIDC_AUTHORITY,
    'http://localhost:8180/realms/nullus',
  )
  // 기본값은 'nullus-app' 이다 — scripts/setup-keycloak.sh 가 만드는 클라이언트이고
  // API 의 audience 기본값(configs/config.yaml, 차트 values)도 같은 값이다.
  const clientId = resolveConfig('oidcClientId', import.meta.env.VITE_OIDC_CLIENT_ID, 'nullus-app')

  if (provider === 'authentik') {
    return {
      type: 'authentik',
      authority,
      clientId,
      scope: 'openid profile email',
      extractRoles: authentikExtractRoles,
      // Authentik's post_logout_redirect_uri is unreliable — requires manual end-session URL
      getLogoutUrl: (idToken, redirectUri) => {
        const url = new URL(`${authority}/end-session/`)
        if (isTokenForClient(idToken, clientId)) {
          url.searchParams.set('id_token_hint', idToken)
        }
        url.searchParams.set('post_logout_redirect_uri', redirectUri)
        return url.toString()
      },
    }
  }

  return {
    type: 'keycloak',
    authority,
    clientId,
    scope: 'openid profile email',
    extractRoles: keycloakExtractRoles,
    // Keycloak end-session: id_token_hint 또는 client_id + post_logout_redirect_uri 필요
    // (client 의 post.logout.redirect.uris 에 redirectUri 가 등록돼 있어야 함)
    getLogoutUrl: (idToken, redirectUri) => {
      const url = new URL(`${authority}/protocol/openid-connect/logout`)
      url.searchParams.set('client_id', clientId)
      url.searchParams.set('post_logout_redirect_uri', redirectUri)
      // 다른 클라이언트로 발급된 토큰을 힌트로 보내면 Keycloak 이 로그아웃 자체를
      // 거부한다. 남은 세션이 어긋난 경우에도 빠져나올 수 있도록 힌트를 버린다.
      if (isTokenForClient(idToken, clientId)) {
        url.searchParams.set('id_token_hint', idToken)
      }
      return url.toString()
    },
  }
}

export function toAuthProviderProps(config: OIDCProviderConfig): OIDCAuthProviderProps {
  return {
    authority: config.authority,
    client_id: config.clientId,
    redirect_uri: window.location.origin + '/',
    post_logout_redirect_uri: window.location.origin + '/',
    scope: config.scope,
    automaticSilentRenew: true,
    onSigninCallback: () => {
      window.history.replaceState({}, document.title, window.location.pathname)
    },
  }
}

// 모듈 로드 시점에 한 번 평가된다. /config.js 는 앱 번들보다 먼저 실행되므로
// (index.html 에서 일반 script 로 선언) 이 시점에 런타임 값이 이미 들어와 있다.
export const isOidcMode =
  resolveConfig('authMode', import.meta.env.VITE_AUTH_MODE, 'session') === 'oidc'

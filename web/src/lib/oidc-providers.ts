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

// Keycloak stores roles at profile.realm_access.roles (nested object)
function keycloakExtractRoles(user: OIDCUser): string[] {
  const realmAccess = user.profile?.realm_access as { roles?: string[] } | undefined
  return realmAccess?.roles ?? []
}

// Authentik stores roles at profile.groups (flat array via profile scope)
function authentikExtractRoles(user: OIDCUser): string[] {
  return (user.profile?.groups as string[]) ?? []
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
        url.searchParams.set('id_token_hint', idToken)
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
      if (idToken) url.searchParams.set('id_token_hint', idToken)
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

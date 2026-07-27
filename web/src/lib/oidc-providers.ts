// When oidc-client-ts / react-oidc-context are installed, replace these with:
//   import type { User } from 'oidc-client-ts'
//   import type { AuthProviderProps } from 'react-oidc-context'

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
  const provider = (import.meta.env.VITE_OIDC_PROVIDER || 'keycloak') as OIDCProviderType
  const authority = import.meta.env.VITE_OIDC_AUTHORITY || 'http://localhost:8180/realms/nullus'
  const clientId = import.meta.env.VITE_OIDC_CLIENT_ID || 'nullus-web'

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

export const isOidcMode = import.meta.env.VITE_AUTH_MODE === 'oidc'

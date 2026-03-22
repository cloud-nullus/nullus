import type { AuthProviderProps } from 'react-oidc-context'

export const oidcConfig: AuthProviderProps = {
  authority: import.meta.env.VITE_KEYCLOAK_URL || 'http://localhost:8180/realms/nullus',
  client_id: import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'nullus-web',
  redirect_uri: `${window.location.origin}/`,
  post_logout_redirect_uri: `${window.location.origin}/`,
  scope: 'openid profile email',
  automaticSilentRenew: true,
  onSigninCallback: () => {
    window.history.replaceState({}, document.title, window.location.pathname)
  },
}

export const isOidcMode = import.meta.env.VITE_AUTH_MODE === 'oidc'

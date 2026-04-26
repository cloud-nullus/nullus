import { StrictMode, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'
import { isOidcMode } from './lib/oidc-providers'

// When react-oidc-context is installed, replace OIDCWrapper body with:
//   import { getProviderConfig, toAuthProviderProps } from './lib/oidc-providers'
//   import { AuthProvider } from 'react-oidc-context'
//   const authProps = toAuthProviderProps(getProviderConfig())
//   return <AuthProvider {...authProps}>{children}</AuthProvider>
function OIDCWrapper({ children }: { children: ReactNode }) {
  return <>{children}</>
}

function AppWrapper() {
  if (isOidcMode) {
    return (
      <OIDCWrapper>
        <App />
      </OIDCWrapper>
    )
  }
  return <App />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppWrapper />
  </StrictMode>,
)

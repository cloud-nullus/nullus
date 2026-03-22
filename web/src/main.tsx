import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AuthProvider } from 'react-oidc-context'
import './index.css'
import App from './App.tsx'
import { isOidcMode, oidcConfig } from './lib/oidc-config'

function AppWrapper() {
  if (isOidcMode) {
    return (
      <AuthProvider {...oidcConfig}>
        <App />
      </AuthProvider>
    )
  }

  return <App />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppWrapper />
  </StrictMode>,
)

import { RouterProvider } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import { router } from './app/routes'
import { ToastProvider } from './components/ui/toast-provider'
import { queryClient } from './lib/query-client'
import i18n from './i18n'

import { Component, useEffect, useRef, type ErrorInfo, type ReactNode } from 'react'
import { useAuth } from 'react-oidc-context'
import { useAuthStore, extractRoleFromOidc } from './stores/auth-store'
import {
  clearChunkReloadMarker,
  isChunkLoadError,
  shouldReloadForChunkError,
} from './lib/chunk-recovery'
import {
  clearOidcStorage,
  clearRecoveryMarker,
  isRecoverableAuthError,
  shouldAttemptRecovery,
} from './lib/oidc-recovery'
import { isOidcMode } from './lib/oidc-config'
import type { User } from './types'

const ORG_ID = '11111111-1111-1111-1111-111111111111'

function Splash({ text, children }: { text: string; children?: ReactNode }) {
  return (
    <div className="flex h-screen flex-col items-center justify-center bg-[var(--color-surface-base)] text-[var(--color-text-secondary)]">
      {text}
      {children}
    </div>
  )
}

// 화면이 비는(blank) 원인을 노출하기 위한 최상위 에러 바운더리 — 콘솔 없이도 에러 표시
class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state: { error: Error | null } = { error: null }
  static getDerivedStateFromError(error: Error) {
    return { error }
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    // 콘솔에도 남김
    console.error('[AppErrorBoundary]', error, info)

    // 재배포로 청크 해시가 바뀌면, 배포 전에 열려 있던 탭은 사라진 파일을 lazy
    // import 하려다 여기서 죽는다. 「다시 시도」로는 같은 옛 모듈 그래프를 다시
    // 쓸 뿐이라 복구되지 않고, 새 index.html 을 받아야 풀린다.
    // 한 번만 새로고침한다 — 원인이 다른 것이면 무한 새로고침이 더 나쁘다.
    if (isChunkLoadError(error) && shouldReloadForChunkError()) {
      window.location.reload()
    }
  }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 24, fontFamily: 'monospace', color: '#f87171', background: '#0b0e14', minHeight: '100vh' }}>
          <h2 style={{ color: '#fca5a5' }}>App render error</h2>
          <pre style={{ whiteSpace: 'pre-wrap' }}>{this.state.error.message}</pre>
          <pre style={{ whiteSpace: 'pre-wrap', color: '#9ca3af', fontSize: 12 }}>{this.state.error.stack}</pre>
        </div>
      )
    }
    clearChunkReloadMarker()
    return this.props.children
  }
}

// OIDC 모드: react-oidc-context 사용자 → auth-store 브릿지 + 동기화 완료 전 렌더 차단
function OidcGate({ children }: { children: ReactNode }) {
  const auth = useAuth()
  const login = useAuthStore((s) => s.login)
  const logout = useAuthStore((s) => s.logout)
  const storeUser = useAuthStore((s) => s.user)

  // 로그인 동기화: auth 상태 변화에만 반응. storeUser 를 deps 에 넣으면
  // login() 이 storeUser 를 갱신 → effect 재실행 → 무한 루프(React #185)가 되므로 제외.
  // 토큰이 동일하면 set 안 하도록 가드해 추가 렌더도 방지.
  useEffect(() => {
    if (!auth.isAuthenticated || !auth.user) return
    const role = extractRoleFromOidc(auth.user)
    const user: User = {
      id: auth.user.profile.sub || '',
      name: (auth.user.profile.name || auth.user.profile.preferred_username || 'OIDC User') as string,
      email: (auth.user.profile.email || '') as string,
      role,
      orgId: ORG_ID,
    }
    const st = useAuthStore.getState()
    if (st.token === auth.user.access_token && st.user?.id === user.id) return
    login(user, auth.user.access_token)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [auth.isAuthenticated, auth.user])

  // 로그아웃 동기화: OIDC 세션이 끝났는데 store 에 user 가 남아있으면 정리
  useEffect(() => {
    if (!auth.isLoading && !auth.isAuthenticated && storeUser) logout()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [auth.isLoading, auth.isAuthenticated, storeUser])

  // 브라우저에 남은 세션과 서버 상태가 어긋나면 로그인이 막다른 화면에서 끝난다
  // (Session not active / No matching state found in storage 등). 사용자가 스스로
  // 빠져나올 방법이 없으므로, 저장소를 비우고 한 번만 다시 시도한다.
  // 같은 오류로는 재시도하지 않는다 — 무한 리다이렉트가 더 나쁘다.
  const recoveringRef = useRef(false)
  useEffect(() => {
    if (!auth.error || recoveringRef.current) return
    if (!isRecoverableAuthError(auth.error)) return
    if (!shouldAttemptRecovery(auth.error.message)) return
    recoveringRef.current = true
    clearOidcStorage()
    void auth.signinRedirect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [auth.error])

  // 로그인에 성공하면 다음 오류에서 다시 복구할 수 있도록 마커를 지운다.
  useEffect(() => {
    if (auth.isAuthenticated) clearRecoveryMarker()
  }, [auth.isAuthenticated])

  if (auth.error) {
    return (
      <Splash text={`로그인 오류: ${auth.error.message}`}>
        <button
          type="button"
          onClick={() => {
            clearOidcStorage()
            void auth.signinRedirect()
          }}
          className="mt-4 rounded-lg border border-[var(--color-border)] bg-transparent px-4 py-2 text-sm text-[var(--color-text-primary)]"
        >
          다시 로그인
        </button>
      </Splash>
    )
  }
  if (auth.isLoading || auth.activeNavigator) return <Splash text="Authenticating…" />
  if (auth.isAuthenticated && !storeUser) return <Splash text="Loading…" />

  return <>{children}</>
}

function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          {isOidcMode ? (
            <OidcGate>
              <RouterProvider router={router} />
            </OidcGate>
          ) : (
            <RouterProvider router={router} />
          )}
          <ToastProvider />
        </I18nextProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App

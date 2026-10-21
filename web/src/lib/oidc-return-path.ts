// 로그인 뒤 돌아갈 앱 안 경로.
//
// 세션(액세스 토큰)이 끝난 뒤 북마크·링크로 들어오면 ProtectedRoute 가 /login 으로 보내고, 로그인 화면은
// IdP 로 보낸다. IdP 는 redirect_uri(/)로만 돌아오므로 예전에는 열려던 화면 대신 홈이 떴다.
// 열려던 경로를 signinRedirect 의 state 에 실어 보내고, 콜백(onSigninCallback)에서 그 경로로 옮긴다.
//
// state 는 브라우저 저장소에 남는 값이지만, 그래도 앱 밖으로 나가는 주소는 받지 않는다.

/** 앱 안의 절대 경로만 돌려준다. 앱 밖 주소 · 로그인 화면 · 홈(콜백이 이미 닿는 곳)은 null. */
export function safeReturnPath(value: unknown): string | null {
  if (typeof value !== 'string' || !value.startsWith('/')) return null
  // 문자열로 보지 않고 해석한 결과로 본다 — 탭·줄바꿈은 지워지고 역슬래시·점 경로는 접혀
  // '/\t/host' · '/.//host' 같은 값이 //host(다른 출처)가 된다.
  let url: URL
  try {
    url = new URL(value, window.location.origin)
  } catch {
    return null
  }
  if (url.origin !== window.location.origin || url.pathname.startsWith('//')) return null
  // 라우터는 대소문자와 끝 슬래시를 가리지 않는다.
  const page = url.pathname.replace(/\/+$/, '').toLowerCase()
  if (page === '' || page === '/login') return null
  return url.pathname + url.search + url.hash
}

/** signinRedirect 에 실어 보낸 state 에서 돌아갈 경로를 꺼낸다. */
export function returnPathFromState(state: unknown): string | null {
  if (!state || typeof state !== 'object' || !('returnTo' in state)) return null
  return safeReturnPath((state as { returnTo: unknown }).returnTo)
}

/**
 * 지금 화면에서 다시 로그인할 때 돌아올 경로. 토큰 갱신이 실패해(IdP 세션 만료 등) 복구 경로로
 * 다시 로그인할 때 쓴다. 콜백 주소(code · state 가 붙은 것)는 돌아갈 곳이 아니고, 로그인 화면이면
 * ProtectedRoute 가 넘긴 경로(라우터가 history.state.usr 에 둔다)를 본다.
 */
export function currentReturnPath(): string | null {
  const params = new URLSearchParams(window.location.search)
  if (params.has('code') || params.has('state') || params.has('error')) return null
  const { pathname, search, hash } = window.location
  const fromLogin = (window.history.state as { usr?: { from?: unknown } } | null)?.usr?.from
  return safeReturnPath(pathname + search + hash) ?? safeReturnPath(fromLogin)
}

/** signinRedirect 의 인자. 돌아갈 곳이 없으면 아무것도 싣지 않는다. */
export function signinArgs(returnTo: string | null): { state: { returnTo: string } } | undefined {
  return returnTo ? { state: { returnTo } } : undefined
}

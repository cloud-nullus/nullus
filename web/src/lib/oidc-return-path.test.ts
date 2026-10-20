import { describe, it, expect, vi, beforeEach } from 'vitest'
import { currentReturnPath, returnPathFromState, safeReturnPath, signinArgs } from './oidc-return-path'
import { toAuthProviderProps, type OIDCProviderConfig } from './oidc-providers'

// 세션이 끝난 뒤 북마크·링크로 들어오면 /login → IdP 를 거쳐 redirect_uri(/)로만 돌아와,
// 열려던 화면 대신 홈이 떴다. 열려던 경로를 signinRedirect 의 state 에 실어 보내고 콜백에서 되돌린다.

describe('safeReturnPath', () => {
  it('should keep an in-app path with its query and hash', () => {
    expect(safeReturnPath('/cicd/list')).toBe('/cicd/list')
    expect(safeReturnPath('/cicd/history?pipeline=p1#run-2')).toBe('/cicd/history?pipeline=p1#run-2')
  })

  it('should drop anything that could leave the app', () => {
    for (const v of ['https://evil.example/x', '//evil.example/x', '/\\evil.example', 'javascript:alert(1)', 'cicd/list']) {
      expect(safeReturnPath(v)).toBeNull()
    }
  })

  // 문자열로는 앱 안 경로처럼 보여도 URL 파서가 //host 로 읽는 값들이다.
  it('should drop paths that the URL parser turns into another origin', () => {
    for (const v of ['/\t/evil.example', '/\n/evil.example', '/.//evil.example/x', '/%2e//evil.example/x', '/a/..//evil.example']) {
      expect(safeReturnPath(v)).toBeNull()
    }
  })

  it('should drop the login page and the home path that the callback already lands on', () => {
    for (const v of ['/login', '/login?next=/x', '/login#x', '/login/', '/Login', '/']) {
      expect(safeReturnPath(v)).toBeNull()
    }
  })

  it('should drop non-string values', () => {
    for (const v of [undefined, null, 42, {}, '']) expect(safeReturnPath(v)).toBeNull()
  })
})

describe('returnPathFromState', () => {
  it('should read returnTo from the signin state', () => {
    expect(returnPathFromState({ returnTo: '/stack/list' })).toBe('/stack/list')
  })

  it('should ignore a missing or unsafe returnTo', () => {
    expect(returnPathFromState(undefined)).toBeNull()
    expect(returnPathFromState('state-string')).toBeNull()
    expect(returnPathFromState({ returnTo: '//evil.example' })).toBeNull()
  })
})

describe('currentReturnPath', () => {
  it('should return the page being viewed', () => {
    window.history.replaceState(null, '', '/stack/list?stack=s1#tab')
    expect(currentReturnPath()).toBe('/stack/list?stack=s1#tab')
  })

  it('should not return the IdP callback address', () => {
    window.history.replaceState(null, '', '/?code=abc&state=xyz')
    expect(currentReturnPath()).toBeNull()
    window.history.replaceState(null, '', '/cicd/list?error=login_required&state=xyz')
    expect(currentReturnPath()).toBeNull()
  })

  // 로그인 화면에서는 ProtectedRoute 가 넘긴 경로(라우터가 history.state.usr 에 둔다)를 본다.
  it('should return the page handed to the login screen', () => {
    window.history.replaceState({ usr: { from: '/cicd/list?tab=history' }, key: 'k', idx: 0 }, '', '/login')
    expect(currentReturnPath()).toBe('/cicd/list?tab=history')
  })
})

describe('signinArgs', () => {
  it('should carry the return path as signin state, or nothing', () => {
    expect(signinArgs('/cicd/list')).toEqual({ state: { returnTo: '/cicd/list' } })
    expect(signinArgs(null)).toBeUndefined()
  })
})

describe('toAuthProviderProps onSigninCallback', () => {
  const config: OIDCProviderConfig = {
    type: 'keycloak',
    authority: 'http://keycloak.test/realms/nullus',
    clientId: 'nullus-app',
    scope: 'openid profile email',
    extractRoles: () => [],
  }

  beforeEach(() => {
    window.history.replaceState({}, '', '/?code=abc&state=xyz&session_state=s')
  })

  it('should send the user back to the page they were opening', () => {
    const onReturn = vi.fn()
    const props = toAuthProviderProps(config, onReturn)

    props.onSigninCallback?.({ state: { returnTo: '/cicd/list' } })

    expect(window.location.search).toBe('')
    expect(onReturn).toHaveBeenCalledWith('/cicd/list')
  })

  // AuthProvider 는 콜백이 끝난 뒤 로딩을 끝낸다 — 이동을 기다리지 않으면 홈이 잠깐 비친다.
  it('should hand back the navigation so the provider waits for it', () => {
    const navigation = Promise.resolve()
    const props = toAuthProviderProps(config, () => navigation)

    expect(props.onSigninCallback?.({ state: { returnTo: '/cicd/list' } })).toBe(navigation)
  })

  it('should stay on the callback page when no page was requested', () => {
    const onReturn = vi.fn()
    const props = toAuthProviderProps(config, onReturn)

    props.onSigninCallback?.(undefined)

    expect(window.location.pathname).toBe('/')
    expect(window.location.search).toBe('')
    expect(onReturn).not.toHaveBeenCalled()
  })
})

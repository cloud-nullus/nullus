import { afterEach, describe, expect, it } from 'vitest'
import { getProviderConfig, isTokenForClient } from './oidc-providers'

/** 서명 없이 payload 만 유효한 JWT 를 만든다 — isTokenForClient 는 서명을 보지 않는다. */
function fakeIdToken(claims: Record<string, unknown>): string {
  const b64 = (o: unknown) =>
    btoa(JSON.stringify(o)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  return `${b64({ alg: 'RS256', typ: 'JWT' })}.${b64(claims)}.sig`
}

afterEach(() => {
  delete window.__NULLUS_CONFIG__
})

describe('isTokenForClient', () => {
  it('should accept a token whose azp matches the client', () => {
    expect(isTokenForClient(fakeIdToken({ azp: 'nullus-app', aud: 'nullus-app' }), 'nullus-app')).toBe(true)
  })

  it('should reject a token issued to a different client', () => {
    expect(isTokenForClient(fakeIdToken({ azp: 'nullus-web', aud: 'nullus-web' }), 'nullus-app')).toBe(false)
  })

  it('should fall back to aud when azp is absent', () => {
    expect(isTokenForClient(fakeIdToken({ aud: 'nullus-app' }), 'nullus-app')).toBe(true)
  })

  it('should accept an audience array containing the client', () => {
    expect(isTokenForClient(fakeIdToken({ aud: ['account', 'nullus-app'] }), 'nullus-app')).toBe(true)
  })

  it('should reject a malformed or empty token', () => {
    expect(isTokenForClient('', 'nullus-app')).toBe(false)
    expect(isTokenForClient('not-a-jwt', 'nullus-app')).toBe(false)
    expect(isTokenForClient('a.!!!not-base64!!!.c', 'nullus-app')).toBe(false)
  })
})

describe('keycloak getLogoutUrl', () => {
  const REDIRECT = 'https://app.example.com/'

  function logoutUrl(idToken: string): URL {
    window.__NULLUS_CONFIG__ = {
      oidcProvider: 'keycloak',
      oidcAuthority: 'https://kc.example.com/realms/nullus',
      oidcClientId: 'nullus-app',
    }
    const config = getProviderConfig()
    return new URL(config.getLogoutUrl!(idToken, REDIRECT))
  }

  it('should send id_token_hint when the token belongs to the client', () => {
    const token = fakeIdToken({ azp: 'nullus-app' })
    const url = logoutUrl(token)
    expect(url.searchParams.get('id_token_hint')).toBe(token)
    expect(url.searchParams.get('client_id')).toBe('nullus-app')
    expect(url.searchParams.get('post_logout_redirect_uri')).toBe(REDIRECT)
  })

  it('should drop a stale hint from a previous client so logout still works', () => {
    // 클라이언트 ID 를 바꾼 뒤 남은 세션. 힌트를 그대로 보내면 Keycloak 이
    // "Invalid parameter: id_token_hint" 로 로그아웃을 거부한다.
    const url = logoutUrl(fakeIdToken({ azp: 'nullus-web' }))
    expect(url.searchParams.has('id_token_hint')).toBe(false)
    expect(url.searchParams.get('client_id')).toBe('nullus-app')
    expect(url.searchParams.get('post_logout_redirect_uri')).toBe(REDIRECT)
  })

  it('should omit the hint when no token is available', () => {
    const url = logoutUrl('')
    expect(url.searchParams.has('id_token_hint')).toBe(false)
    expect(url.searchParams.get('client_id')).toBe('nullus-app')
  })
})

describe('keycloak role 추출', () => {
  // Keycloak 은 realm_access 를 기본적으로 액세스 토큰에만 싣는다. ID 토큰에만
  // 의존하면 서버 매퍼 설정에 따라 모든 사용자가 최저 권한으로 떨어진다.
  function roles(profile: Record<string, unknown>, accessToken?: string): string[] {
    window.__NULLUS_CONFIG__ = {
      oidcProvider: 'keycloak',
      oidcAuthority: 'https://kc.example.com/realms/nullus',
      oidcClientId: 'nullus-app',
    }
    return getProviderConfig().extractRoles({ profile, access_token: accessToken })
  }

  it('should read roles from the access token when the id token has none', () => {
    const at = fakeIdToken({ realm_access: { roles: ['default-roles-nullus', 'admin'] } })
    expect(roles({}, at)).toContain('admin')
  })

  it('should prefer roles already present in the id token', () => {
    const at = fakeIdToken({ realm_access: { roles: ['developer'] } })
    expect(roles({ realm_access: { roles: ['admin'] } }, at)).toEqual(['admin'])
  })

  it('should return an empty list when neither token carries roles', () => {
    expect(roles({}, fakeIdToken({ sub: 'x' }))).toEqual([])
    expect(roles({})).toEqual([])
  })

  it('should survive a malformed access token', () => {
    expect(roles({}, 'not-a-jwt')).toEqual([])
  })
})

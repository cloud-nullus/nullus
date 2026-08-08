import { beforeEach, describe, expect, it } from 'vitest'
import {
  clearOidcStorage,
  clearRecoveryMarker,
  isRecoverableAuthError,
  shouldAttemptRecovery,
} from './oidc-recovery'

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
})

describe('isRecoverableAuthError', () => {
  it.each([
    'Session not active',
    'No matching state found in storage',
    'login_required',
    'State mismatch',
  ])('should treat %s as recoverable', (message) => {
    expect(isRecoverableAuthError({ message })).toBe(true)
  })

  it('should not treat an unrelated failure as recoverable', () => {
    // 네트워크나 설정 오류는 저장소를 비워도 풀리지 않는다 — 반복 리다이렉트만 는다.
    expect(isRecoverableAuthError({ message: 'Failed to fetch' })).toBe(false)
    expect(isRecoverableAuthError({ message: 'invalid_client' })).toBe(false)
  })

  it('should handle a missing error object', () => {
    expect(isRecoverableAuthError(null)).toBe(false)
    expect(isRecoverableAuthError(undefined)).toBe(false)
    expect(isRecoverableAuthError({})).toBe(false)
  })
})

describe('clearOidcStorage', () => {
  it('should remove oidc entries from both storages and leave other data alone', () => {
    sessionStorage.setItem('oidc.abc', 'state')
    sessionStorage.setItem('oidc.user:https://kc/realms/nullus:nullus-app', 'user')
    sessionStorage.setItem('nullus.locale', 'ko')
    localStorage.setItem('oidc.stale', 'x')
    localStorage.setItem('theme', 'dark')

    clearOidcStorage()

    expect(Object.keys(sessionStorage).filter((k) => k.startsWith('oidc.'))).toHaveLength(0)
    expect(Object.keys(localStorage).filter((k) => k.startsWith('oidc.'))).toHaveLength(0)
    expect(sessionStorage.getItem('nullus.locale')).toBe('ko')
    expect(localStorage.getItem('theme')).toBe('dark')
  })
})

describe('shouldAttemptRecovery', () => {
  it('should allow the first attempt and block a repeat of the same error', () => {
    expect(shouldAttemptRecovery('Session not active')).toBe(true)
    expect(shouldAttemptRecovery('Session not active')).toBe(false)
  })

  it('should allow a different error after one was already handled', () => {
    expect(shouldAttemptRecovery('Session not active')).toBe(true)
    expect(shouldAttemptRecovery('No matching state found in storage')).toBe(true)
  })

  it('should allow retrying the same error after a successful login clears the marker', () => {
    expect(shouldAttemptRecovery('Session not active')).toBe(true)
    clearRecoveryMarker()
    expect(shouldAttemptRecovery('Session not active')).toBe(true)
  })
})

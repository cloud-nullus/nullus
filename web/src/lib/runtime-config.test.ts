import { afterEach, describe, expect, it } from 'vitest'
import { resolveConfig } from './runtime-config'

afterEach(() => {
  delete window.__NULLUS_CONFIG__
})

describe('resolveConfig', () => {
  it('should prefer the runtime value over the build-time value', () => {
    window.__NULLUS_CONFIG__ = { oidcAuthority: 'https://kc.example.com/realms/nullus' }
    expect(resolveConfig('oidcAuthority', 'http://baked-in/realms/nullus', 'fallback')).toBe(
      'https://kc.example.com/realms/nullus',
    )
  })

  it('should fall back to the build-time value when no runtime value is injected', () => {
    expect(resolveConfig('oidcAuthority', 'http://baked-in/realms/nullus', 'fallback')).toBe(
      'http://baked-in/realms/nullus',
    )
  })

  it('should fall back to the default when neither is set', () => {
    window.__NULLUS_CONFIG__ = {}
    expect(resolveConfig('oidcAuthority', undefined, 'fallback')).toBe('fallback')
  })

  it('should ignore an unsubstituted placeholder left by the container entrypoint', () => {
    window.__NULLUS_CONFIG__ = { oidcAuthority: '__NULLUS_OIDC_AUTHORITY__' }
    expect(resolveConfig('oidcAuthority', 'http://baked-in/realms/nullus', 'fallback')).toBe(
      'http://baked-in/realms/nullus',
    )
  })

  it('should ignore a blank runtime value', () => {
    window.__NULLUS_CONFIG__ = { oidcAuthority: '   ' }
    expect(resolveConfig('oidcAuthority', undefined, 'fallback')).toBe('fallback')
  })

  it('should trim surrounding whitespace from the resolved value', () => {
    window.__NULLUS_CONFIG__ = { oidcClientId: '  nullus-app  ' }
    expect(resolveConfig('oidcClientId', undefined, 'fallback')).toBe('nullus-app')
  })
})

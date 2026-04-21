import { describe, it, expect } from 'vitest'
import { isDeployServerGateLocked } from './deploy-gate'

describe('isDeployServerGateLocked', () => {
  it('returns false when no server verdict has been computed yet', () => {
    expect(isDeployServerGateLocked(null, false)).toBe(false)
    expect(isDeployServerGateLocked(undefined, false)).toBe(false)
  })

  it('returns false on pass regardless of acknowledgement', () => {
    expect(isDeployServerGateLocked({ overall: { state: 'pass' } }, false)).toBe(false)
    expect(isDeployServerGateLocked({ overall: { state: 'pass' } }, true)).toBe(false)
  })

  it('returns true on fail regardless of acknowledgement', () => {
    expect(isDeployServerGateLocked({ overall: { state: 'fail' } }, false)).toBe(true)
    expect(isDeployServerGateLocked({ overall: { state: 'fail' } }, true)).toBe(true)
  })

  it('returns true on warn until the user acknowledges', () => {
    expect(isDeployServerGateLocked({ overall: { state: 'warn' } }, false)).toBe(true)
    expect(isDeployServerGateLocked({ overall: { state: 'warn' } }, true)).toBe(false)
  })
})

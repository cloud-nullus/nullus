import { describe, it, expect } from 'vitest'
import { STATUS_STYLES, getStatusStyle } from './status-style'

describe('STATUS_STYLES palette', () => {
  it('covers all canonical StackStatus keys', () => {
    const required = [
      'pending',
      'validating',
      'installing',
      'configuring',
      'health_check',
      'running',
      'completed',
      'failed',
      'rolling_back',
      'rolled_back',
      'cancelled',
    ] as const
    for (const key of required) {
      expect(STATUS_STYLES[key], `missing status style for ${key}`).toBeDefined()
    }
  })

  it('gives failed a red palette', () => {
    expect(STATUS_STYLES.failed.bg).toContain('239,68,68')
  })

  it('gives rolled_back an amber palette distinct from cancelled grey', () => {
    expect(STATUS_STYLES.rolled_back.bg).toContain('245,158,11')
    expect(STATUS_STYLES.cancelled.bg).toContain('100,116,139')
  })

  it('getStatusStyle falls back to pending for unknown keys', () => {
    expect(getStatusStyle('bogus-status')).toBe(STATUS_STYLES.pending)
  })
})

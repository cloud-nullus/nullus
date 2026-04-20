import { describe, expect, it } from 'vitest'
import { canRetry, type StackStatus } from './retry-policy'

describe('canRetry', () => {
  it('returns true only for failed and rolled_back', () => {
    const expected: Record<StackStatus, boolean> = {
      pending: false,
      validating: false,
      installing: false,
      configuring: false,
      health_check: false,
      completed: false,
      failed: true,
      rolling_back: false,
      rolled_back: true,
      cancelled: false,
    }
    for (const [status, want] of Object.entries(expected) as [StackStatus, boolean][]) {
      expect(canRetry(status)).toBe(want)
    }
  })
})

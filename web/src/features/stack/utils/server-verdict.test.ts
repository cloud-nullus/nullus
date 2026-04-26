import { describe, expect, it } from 'vitest'
import type { CompatibilityValidationResult } from '../../../types'
import { shouldBlockOnServerVerdict } from './server-verdict'

const verdict = (state: 'pass' | 'warn' | 'fail'): CompatibilityValidationResult => ({
  compatible: state !== 'fail',
  overall: { state, score: state === 'pass' ? 100 : state === 'warn' ? 70 : 0 },
  issues: [],
  nodeArchitectures: [],
  matrix: undefined,
  message: undefined,
  checkedAt: '2026-04-19T00:00:00Z',
})

describe('shouldBlockOnServerVerdict', () => {
  it('pass verdict → mode=pass, do not block', () => {
    const decision = shouldBlockOnServerVerdict(verdict('pass'), false)
    expect(decision).toEqual({ mode: 'pass', block: false })
  })

  it('fail verdict → mode=block regardless of ack flag', () => {
    expect(shouldBlockOnServerVerdict(verdict('fail'), false)).toEqual({ mode: 'block', block: true })
    expect(shouldBlockOnServerVerdict(verdict('fail'), true)).toEqual({ mode: 'block', block: true })
  })

  it('warn verdict requires ack: without ack → block, with ack → warn-ack + flag', () => {
    expect(shouldBlockOnServerVerdict(verdict('warn'), false)).toEqual({ mode: 'block', block: true })
    expect(shouldBlockOnServerVerdict(verdict('warn'), true)).toEqual({
      mode: 'warn-ack',
      block: false,
      acknowledgeWarnings: true,
    })
  })
})

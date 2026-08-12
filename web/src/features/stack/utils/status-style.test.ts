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

  // 원래 이 두 테스트는 원시 RGB 값('239,68,68')을 단정했다. 팔레트가 디자인
  // 토큰으로 옮겨가면서 값 표기가 바뀌었으므로, 파일 주석이 밝힌 실제 계약
  // ("각 종료 상태가 서로 구별된다")을 토큰 기준으로 검증한다.
  it('gives failed the error token', () => {
    expect(STATUS_STYLES.failed.color).toContain('--color-error')
    expect(STATUS_STYLES.failed.bg).toContain('--color-error')
  })

  it('keeps rolled_back, failed, cancelled visually distinct', () => {
    expect(STATUS_STYLES.rolled_back.color).toContain('--color-warning')
    expect(STATUS_STYLES.cancelled.color).toContain('--color-text-muted')

    // 세 종료 상태가 서로 다른 토큰을 써야 한다 — 같으면 구별이 사라진다.
    const tones = [
      STATUS_STYLES.failed.color,
      STATUS_STYLES.rolled_back.color,
      STATUS_STYLES.cancelled.color,
    ]
    expect(new Set(tones).size).toBe(3)
  })

  it('getStatusStyle falls back to pending for unknown keys', () => {
    expect(getStatusStyle('bogus-status')).toBe(STATUS_STYLES.pending)
  })
})

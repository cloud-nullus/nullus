import { describe, it, expect, beforeEach, vi } from 'vitest'
import { warnAckKey, readAck, writeAck } from './warn-ack-storage'

beforeEach(() => {
  try {
    sessionStorage.clear()
  } catch {
    /* jsdom fallback */
  }
})

describe('warnAckKey', () => {
  it('produces the same key for identical inputs', () => {
    const issues = [{ code: 'A', message: 'one' }]
    expect(warnAckKey('client', 'stack', 'c1', issues)).toBe(
      warnAckKey('client', 'stack', 'c1', issues),
    )
  })

  it('rotates the key when the issue list changes', () => {
    const a = warnAckKey('server', 'stack', 'c1', [{ code: 'X' }])
    const b = warnAckKey('server', 'stack', 'c1', [{ code: 'Y' }])
    expect(a).not.toBe(b)
  })

  it('routes client and server to distinct namespaces', () => {
    const issues = [{ code: 'A' }]
    expect(warnAckKey('client', 'stack', 'c1', issues)).not.toBe(
      warnAckKey('server', 'stack', 'c1', issues),
    )
  })
})

describe('readAck / writeAck', () => {
  it('round-trips an acknowledgement', () => {
    const key = warnAckKey('client', 'stack', 'c1', [{ code: 'A' }])
    expect(readAck(key)).toBe(false)
    writeAck(key, true)
    expect(readAck(key)).toBe(true)
    writeAck(key, false)
    expect(readAck(key)).toBe(false)
  })

  it('never throws when sessionStorage raises', () => {
    const getSpy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage unavailable')
    })
    const setSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage unavailable')
    })
    const key = 'nullus.warnAck.client.blocked.0.deadbeef'
    expect(readAck(key)).toBe(false)
    expect(() => writeAck(key, true)).not.toThrow()
    getSpy.mockRestore()
    setSpy.mockRestore()
  })
})

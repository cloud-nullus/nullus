import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  clearChunkReloadMarker,
  isChunkLoadError,
  shouldReloadForChunkError,
} from './chunk-recovery'

beforeEach(() => {
  sessionStorage.clear()
  vi.restoreAllMocks()
})

describe('isChunkLoadError', () => {
  it.each([
    'Failed to fetch dynamically imported module: https://nullus.io/assets/x-DZOunU2s.js',
    'error loading dynamically imported module',
    'Importing a module script failed',
    'Loading chunk 42 failed',
  ])('should recognise %s', (message) => {
    expect(isChunkLoadError(new Error(message))).toBe(true)
  })

  it('should recognise the error by name as well as message', () => {
    const error = new Error('boom')
    error.name = 'ChunkLoadError'
    expect(isChunkLoadError(error)).toBe(true)
  })

  it('should accept a bare string', () => {
    expect(isChunkLoadError('Failed to fetch dynamically imported module')).toBe(true)
  })

  it('should not treat an ordinary render error as a chunk failure', () => {
    // 새로고침해도 낫지 않는 오류를 여기서 잡으면 무한 새로고침이 된다.
    expect(isChunkLoadError(new Error("Cannot read properties of undefined"))).toBe(false)
    expect(isChunkLoadError(new TypeError('x is not a function'))).toBe(false)
  })

  it('should handle empty input', () => {
    expect(isChunkLoadError(null)).toBe(false)
    expect(isChunkLoadError(undefined)).toBe(false)
    expect(isChunkLoadError({})).toBe(false)
  })
})

describe('shouldReloadForChunkError', () => {
  it('should allow exactly one reload', () => {
    expect(shouldReloadForChunkError()).toBe(true)
    expect(shouldReloadForChunkError()).toBe(false)
    expect(shouldReloadForChunkError()).toBe(false)
  })

  it('should allow another reload after a successful render clears the marker', () => {
    expect(shouldReloadForChunkError()).toBe(true)
    clearChunkReloadMarker()
    expect(shouldReloadForChunkError()).toBe(true)
  })

  it('should refuse when storage is unavailable', () => {
    // 반복 여부를 판단할 수 없으면 시도하지 않는다 — 무한 새로고침을 피한다.
    vi.stubGlobal('sessionStorage', {
      getItem() {
        throw new Error('denied')
      },
      setItem() {
        throw new Error('denied')
      },
      removeItem() {
        throw new Error('denied')
      },
    })
    expect(shouldReloadForChunkError()).toBe(false)
    vi.unstubAllGlobals()
  })
})

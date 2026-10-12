import { describe, expect, it } from 'vitest'
import type { StackImageScanItem } from '../api/stack-api-types'
import { sortStackImageScanItems } from './image-scan-report'

function item(image: string, overrides: Partial<StackImageScanItem> = {}): StackImageScanItem {
  return {
    image,
    workloads: [],
    status: 'scanned',
    dbStale: false,
    counts: { critical: 0, high: 0, medium: 0, low: 0, unknown: 0 },
    ...overrides,
  }
}

const counts = (critical: number, high: number, medium = 0, low = 0) => ({
  critical,
  high,
  medium,
  low,
  unknown: 0,
})

describe('sortStackImageScanItems', () => {
  it('Critical 이 많은 순, 같으면 High 가 많은 순으로 둔다', () => {
    const sorted = sortStackImageScanItems([
      item('a', { counts: counts(0, 5) }),
      item('b', { counts: counts(2, 0) }),
      item('c', { counts: counts(0, 9) }),
      item('d', { counts: counts(2, 3) }),
    ])

    expect(sorted.map((i) => i.image)).toEqual(['d', 'b', 'c', 'a'])
  })

  it('Critical·High 가 같으면 Medium, Low, 이미지 이름 순이다', () => {
    const sorted = sortStackImageScanItems([
      item('z', { counts: counts(0, 1, 0, 0) }),
      item('y', { counts: counts(0, 1, 0, 4) }),
      item('x', { counts: counts(0, 1, 3, 0) }),
      item('w', { counts: counts(0, 1, 0, 0) }),
    ])

    expect(sorted.map((i) => i.image)).toEqual(['x', 'y', 'w', 'z'])
  })

  // 건수를 모르는 항목(실패·건수 누락)은 심각도로 줄 세울 수 없다. 0 으로 치면
  // 깨끗한 이미지들 사이에 섞이므로 뒤에 따로 모은다.
  it('건수를 모르는 항목은 알려진 항목 뒤에 둔다', () => {
    const sorted = sortStackImageScanItems([
      item('failed', { status: 'failed', counts: undefined, error: 'boom' }),
      item('clean', { counts: counts(0, 0) }),
      item('missing', { counts: undefined }),
      item('bad', { counts: counts(1, 0) }),
    ])

    expect(sorted.map((i) => i.image)).toEqual(['bad', 'clean', 'failed', 'missing'])
  })

  it('원본 배열을 바꾸지 않는다', () => {
    const input = [item('a', { counts: counts(0, 1) }), item('b', { counts: counts(1, 0) })]
    sortStackImageScanItems(input)

    expect(input.map((i) => i.image)).toEqual(['a', 'b'])
  })
})

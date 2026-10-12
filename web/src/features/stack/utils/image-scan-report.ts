import type { StackImageScanItem } from '../api/stack-api-types'

// 설치 이미지 보고의 표시 순서.
//
// 가장 위험한 이미지가 먼저 보여야 한다: Critical 많은 순, 같으면 High 많은 순,
// 그다음 Medium·Low, 마지막으로 이미지 이름. 건수를 모르는 항목(스캔 실패·건수 누락)은
// 심각도로 줄 세울 수 없다. 0 으로 치면 깨끗한 이미지들 사이에 섞여 묻히므로 뒤에 모은다.
export function sortStackImageScanItems(items: readonly StackImageScanItem[]): StackImageScanItem[] {
  return [...items].sort((a, b) => {
    if (!a.counts || !b.counts) {
      if (a.counts) return -1
      if (b.counts) return 1
      return a.image.localeCompare(b.image)
    }
    return (
      b.counts.critical - a.counts.critical ||
      b.counts.high - a.counts.high ||
      b.counts.medium - a.counts.medium ||
      b.counts.low - a.counts.low ||
      a.image.localeCompare(b.image)
    )
  })
}

/**
 * 재배포 후 열려 있던 탭을 되살린다.
 *
 * 빌드 산출물은 파일명에 내용 해시가 들어간다. 재배포하면 해시가 바뀌고 이전 청크는
 * 사라지므로, 배포 전에 열려 있던 탭이 lazy import 를 시도하면 없는 파일을 요청하게
 * 되어 화면이 죽는다.
 *
 *   Failed to fetch dynamically imported module: /assets/…-DZOunU2s.js
 *
 * 이 상태에서는 「다시 시도」를 눌러도 같은(옛) 모듈 그래프를 다시 쓰므로 복구되지
 * 않는다. 새 index.html 을 받아야 하니 결국 새로고침이 유일한 해법이고, 그걸
 * 사용자에게 시키는 대신 앱이 알아서 한다.
 *
 * 단 **한 번만** 한다. 새로고침해도 낫지 않는 원인(네트워크 차단, 배포 자체가 깨진
 * 경우)이라면 무한 새로고침이 되어 막다른 화면보다 나쁘다. 한 번 시도한 뒤에도
 * 같은 오류가 나면 그냥 오류 화면을 보여 주는 편이 낫다.
 */

const RELOAD_MARKER = 'nullus.chunk-reload'

const CHUNK_ERROR_SIGNATURES = [
  'failed to fetch dynamically imported module',
  'error loading dynamically imported module',
  'importing a module script failed',
  'chunkloaderror',
  'loading chunk',
]

/** 재배포로 청크가 사라져서 난 오류인지 판단한다. */
export function isChunkLoadError(error: unknown): boolean {
  if (!error) return false
  const parts: string[] = []
  if (typeof error === 'string') parts.push(error)
  if (error instanceof Error) {
    parts.push(error.message, error.name)
  } else if (typeof error === 'object') {
    const e = error as { message?: unknown; name?: unknown }
    if (typeof e.message === 'string') parts.push(e.message)
    if (typeof e.name === 'string') parts.push(e.name)
  }
  const haystack = parts.join(' ').toLowerCase()
  if (!haystack) return false
  return CHUNK_ERROR_SIGNATURES.some((needle) => haystack.includes(needle))
}

/**
 * 새로고침을 시도해도 되는지 판단하고, 시도한다고 표시한다.
 *
 * 마커를 sessionStorage 에 두므로 새로고침 뒤에도 남아 반복을 막고, 탭을 닫으면
 * 사라져 다음 방문에는 다시 복구할 수 있다.
 */
export function shouldReloadForChunkError(): boolean {
  try {
    if (sessionStorage.getItem(RELOAD_MARKER)) return false
    sessionStorage.setItem(RELOAD_MARKER, '1')
    return true
  } catch {
    // 저장소를 못 쓰면 반복 여부를 알 수 없다. 무한 새로고침을 피해 시도하지 않는다.
    return false
  }
}

/** 앱이 정상적으로 뜨면 호출해 다음 배포에서 다시 복구할 수 있게 한다. */
export function clearChunkReloadMarker(): void {
  try {
    sessionStorage.removeItem(RELOAD_MARKER)
  } catch {
    // 무시 — 마커가 없으면 어차피 복구를 시도한다.
  }
}

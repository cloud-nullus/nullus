/**
 * OIDC 인증 오류에서 스스로 빠져나오기.
 *
 * 브라우저에 남은 세션과 서버 상태가 어긋나면 로그인 흐름이 막다른 화면에서 끝난다.
 * 사용자는 화면에 문구만 보고 아무것도 할 수 없어, 개발자 도구로 저장소를 비우는
 * 것 말고는 방법이 없다. 실제로 겪은 것들:
 *
 *   Session not active                    Keycloak 세션이 서버에서 사라졌는데
 *                                         브라우저에 토큰이 남아 있을 때
 *   No matching state found in storage    로그인 요청의 state 가 사라졌을 때
 *                                         (다른 탭에서 콜백을 열거나 저장소가 비워짐)
 *   login_required                        SSO 세션 만료
 *
 * 이런 오류는 저장소를 비우고 로그인을 다시 시작하면 대부분 해결된다. 다만 무한
 * 반복은 더 나쁘므로 **원인당 한 번만** 시도하고, 그래도 실패하면 오류를 그대로
 * 보여 준다.
 */

/** 저장소를 비우고 재시도하면 풀릴 가능성이 높은 오류인지 판단한다. */
export function isRecoverableAuthError(error: { message?: string } | null | undefined): boolean {
  const message = error?.message
  if (!message) return false
  const normalized = message.toLowerCase()
  return [
    'session not active',
    'no matching state found in storage',
    'login_required',
    'no end session endpoint',
    'state mismatch',
  ].some((needle) => normalized.includes(needle))
}

/** oidc-client-ts 가 남긴 항목을 모두 지운다. 다른 앱 데이터는 건드리지 않는다. */
export function clearOidcStorage(): void {
  for (const storage of [sessionStorage, localStorage]) {
    try {
      for (const key of Object.keys(storage)) {
        if (key.startsWith('oidc.')) storage.removeItem(key)
      }
    } catch {
      // 저장소 접근이 막힌 환경(사생활 보호 모드 등)에서는 조용히 넘어간다.
    }
  }
}

const RECOVERY_MARKER = 'nullus.oidc-recovery'

/**
 * 같은 오류로 한 번만 복구를 시도한다.
 *
 * 마커를 sessionStorage 에 두므로 페이지를 새로고침해도 반복되지 않고, 탭을 닫으면
 * 초기화되어 다음 방문에는 다시 시도할 수 있다.
 */
export function shouldAttemptRecovery(errorMessage: string): boolean {
  try {
    if (sessionStorage.getItem(RECOVERY_MARKER) === errorMessage) return false
    sessionStorage.setItem(RECOVERY_MARKER, errorMessage)
    return true
  } catch {
    // 저장소를 못 쓰면 반복 여부를 판단할 수 없다. 무한 루프를 피해 시도하지 않는다.
    return false
  }
}

/** 로그인에 성공했을 때 호출해 다음 오류에서 다시 복구할 수 있게 한다. */
export function clearRecoveryMarker(): void {
  try {
    sessionStorage.removeItem(RECOVERY_MARKER)
  } catch {
    // 무시 — 마커가 없으면 어차피 복구를 시도한다.
  }
}

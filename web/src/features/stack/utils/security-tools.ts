import type { StackConfigDraft } from '../stores/stack-config-store'

/**
 * SonarQube 와 스택 DB 의 조합을 검사한다. 문제가 있으면 사용자에게 보일 이유를, 없으면 null 을 돌려준다.
 *
 * SonarQube 는 스택이 만든 PostgreSQL 안에 전용 DB 를 만들어 쓴다. 백엔드는 DB 를 외부 연결로
 * 고른 스택에서 이 조합을 거부하는데(sonarqube-provisioning.go), 그 판정이 installing_sonarqube
 * 단계에서야 나와 앞 단계를 다 깐 뒤에 실패한다. 화면에서 먼저 막는다.
 *
 * 저장소 계획을 쓰지 않으면(planMode none) 스택이 PostgreSQL 을 함께 깔므로 막지 않는다.
 */
export function sastStorageConflict(draft: Pick<StackConfigDraft, 'security' | 'storage'>): string | null {
  if (!draft.security.sast.tool) return null
  if (draft.storage.planMode === 'none') return null
  if (draft.storage.database.mode !== 'existing') return null
  return 'SonarQube 는 스택이 만드는 PostgreSQL 안에 전용 DB 를 만들어 씁니다. Storage 탭에서 DB 를 새로 만들기로 두거나, Security 탭에서 SonarQube 를 빼 주세요.'
}

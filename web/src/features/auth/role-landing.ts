import type { Role } from '../../types'

/**
 * 역할별 시작 화면.
 *
 * 로그인 리다이렉트와 홈의 시작 버튼이 각각 자기 목록을 들고 있어서 developer 만
 * 서로 다른 곳을 가리켰다. 홈은 `/cicd/templates` 로 보냈는데 사이드바는 그 메뉴를
 * developer 에게 숨기므로, 한 번 들어가면 메뉴로 다시 찾아갈 수 없었다.
 *
 * 두 곳이 같은 값을 봐야 하므로 여기를 단일 출처로 둔다.
 */
const ROLE_LANDING: Record<Role, string> = {
  admin: '/admin/organization',
  devops: '/stack/templates',
  developer: '/cicd/developer-deploy',
}

/** 알 수 없는 역할은 홈으로 보낸다 — 권한 밖 경로로 보내면 즉시 튕긴다. */
export function roleLandingPath(role: Role | null | undefined): string {
  if (!role) return '/'
  return ROLE_LANDING[role] ?? '/'
}

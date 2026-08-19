import { api } from '../../../lib/api'
import type { User } from '../../../types'

interface LoginResponse {
  token: string
  user: {
    id: string
    email: string
    name: string
    role: string
    orgId: string
  }
}

export interface PasswordLoginResult {
  token: string
  user: User
}

/**
 * ID/PW 로 로그인한다 (OIDC 와 나란히 서는 두 번째 경로).
 *
 * IdP 가 죽어도 들어갈 수단이라, IdP 상태와 무관하게 동작해야 한다.
 * 서버는 없는 계정과 틀린 비밀번호를 같은 응답으로 답하므로 여기서도 구분하지
 * 않는다 — 구분해 보여주면 어떤 이메일이 가입돼 있는지 알아낼 수 있다.
 */
export async function loginWithPassword(email: string, password: string): Promise<PasswordLoginResult> {
  const { data } = await api.post<LoginResponse>('/auth/login', { email, password })
  return {
    token: data.token,
    user: {
      id: data.user.id,
      email: data.user.email,
      name: data.user.name,
      role: data.user.role as User['role'],
      orgId: data.user.orgId,
    },
  }
}

# 관리

<span class="role role-admin">admin</span> 전용 영역입니다. 사이드바에 7개 화면이
있고, 백엔드도 `/api/v1/admin/**` 전체에 `RequireRole("admin")` 을 걸어
devops·developer 에게는 403 을 돌려줍니다.

<div class="nullus-cards" markdown>

[**조직**<span>조직 정보와 클러스터 접근 범위.</span>](organization.md)

[**사용자 관리**<span>구성원 추가와 역할 부여.</span>](users.md)

[**클러스터 관리**<span>대상 Kubernetes 클러스터 등록과 연결 검증.</span>](clusters.md)

[**토큰 관리**<span>스택이 쓰는 비밀의 출처와 회전 상태.</span>](tokens.md)

[**알려진 이슈**<span>알려진 제약과 우회 방법.</span>](known-issues.md)

[**백업·복구**<span>플랫폼 상태를 떠 두고 되돌리기.</span>](backup.md)

[**알림 채널**<span>경보를 받을 Telegram·이메일·Slack 채널.</span>](notifications.md)

</div>

## 인증 모드를 먼저 확인하세요

Nullus 는 인증 모드가 둘입니다. **운영에서는 반드시 `oidc` 를 써야 합니다.**

| 모드 | 설정 | 동작 |
|------|------|------|
| `oidc` | `NULLUS_AUTH_MODE=oidc` + `VITE_AUTH_MODE=oidc` | Keycloak(또는 Authentik) 토큰을 검증합니다. 운영용 |
| `session` (기본값) | 그 밖의 값 | **개발 전용** |

!!! danger "session 모드는 인증이 아닙니다"
    `session` 모드에서 로그인 화면은 브라우저 안에 들어 있는 테스트 계정 표와
    비밀번호를 맞춰 보고, 서버에 묻지 않습니다. 서버 쪽도 `X-User-ID` ·
    `X-User-Role` 헤더를 **그대로 믿습니다.** 즉 아무나 원하는 역할을 주장할 수
    있습니다. 기본값이 `session` 이므로, 운영 배포에서는 `auth.mode` 를 `oidc` 로
    바꾸고 프런트의 `authMode` 도 같이 맞춰야 합니다.

!!! danger "development 모드는 권한 검사를 전부 끕니다"
    `server.mode` 가 `development` 면 인증 미들웨어와 모든 `RequireRole` 가
    **등록되지 않습니다.** 네 그룹이 전부 열립니다. Helm 차트 기본값은
    `production` 입니다.

## 역할은 IdP 에서 옵니다

역할은 Nullus 가 정하는 값이 아니라 토큰에서 읽는 값입니다.

- Keycloak — `realm_access.roles`. ID 토큰에 없으면 **액세스 토큰을 열어** 다시 찾습니다
- Authentik — `groups`

우선순위는 admin → devops → **그 외 전부 developer** 입니다. 역할 주장이 없는
토큰은 조용히 developer 가 됩니다.

!!! warning "조직 ID 가 코드에 박혀 있습니다"
    프런트엔드는 `X-Org-ID` 로 보낼 조직 ID 를
    `11111111-1111-1111-1111-111111111111` 로 **고정**하고 있습니다. 다중 조직
    운영은 아직 이 경로로 되지 않습니다.

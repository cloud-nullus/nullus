# 토큰 관리

<span class="route">/admin/token-management</span>
<span class="role role-admin">admin</span>

설치된 스택이 쓰는 비밀(토큰)의 출처와 상태를 보는 화면입니다.

!!! note "화면 제목이 영문입니다"
    사이드바 라벨은 "토큰 관리" 인데 화면 제목은 `OpenBao Token Management`
    입니다.

## 토큰 출처란

플랫폼이 사용자를 대신해 관리하는 비밀 하나를 가리키는 행입니다.

| 항목 | 뜻 |
|------|-----|
| `provider` | 어떤 도구의 토큰인가 |
| `module` | 어느 모듈이 쓰는가 |
| `path` | 비밀 저장소(기본은 OpenBao) 안의 경로 |
| `token_type` | 종류 |
| `status` | 아래 표 |
| `expires_at` · `last_rotated_at` · `next_check_at` | 만료·마지막 회전·다음 점검 |
| `requires_approval` | 회전에 승인이 필요한가 |

행은 **스택 설정에서 만들어집니다** — 설치한 스택이 필요로 하는 SCM·레지스트리
토큰들입니다. 출처마다 회전 이벤트 기록이 덧붙기만 하는 형태로 쌓입니다.

상태 값: `healthy` · `renew_due` · `renewing` · `rotated` ·
`failed_retryable` · `failed_manual` · `expired`

## 화면 구성

왼쪽 표(Provider · Module · Path · Status · Expires)에서 행을 고르면 오른쪽에
버튼 묶음 **Rotate · Approve · Pause · Resume · Reveal** 과 **Recent Events**
목록, 그리고 결과 JSON 블록이 나옵니다.

!!! danger "네 버튼은 상태 글자만 바꿉니다"
    `Rotate` · `Approve` · `Pause` · `Resume` 는 **실제로 토큰을 재발급하지
    않습니다.** 상태 값을 새로 쓰고 이벤트 한 줄을 `success` 로 남기는 것이
    전부입니다.

    | 버튼 | 바뀌는 상태 |
    |------|------------|
    | Rotate | `rotated` |
    | Approve · Resume | `renewing` |
    | **Pause** | `failed_manual` |

    진짜 재발급은 서버 안의 **자동 회전 스케줄러**가 합니다(기본 5분마다 점검).
    이 화면에서는 거기에 손댈 수 없습니다. 즉 버튼을 눌러 "회전됨" 이 떠도
    토큰은 그대로입니다.

## Reveal — 비밀 보기

서버는 두 단계를 요구합니다.

1. `POST …/{id}/re-auth` — 5분짜리 1회용 승급 토큰을 발급
2. `POST …/{id}/reveal` — 그 토큰과 사용자 ID 를 함께 보내면 비밀 원문을 돌려줌

승급 토큰은 **프로세스 메모리**에 들고 있습니다. 서버를 재시작하면 사라지고,
여러 복제본 사이에 공유되지 않습니다.

!!! danger "이 화면은 시험 모드로 돌고 있습니다"
    화면 자체가 "test mode" 배너로 알리고 있듯이, 지금은 승급 단계를 **사람이
    다시 인증하지 않고** 자동으로 통과합니다. 또한 Reveal 을 포함한 이 화면의
    동작은 **감사 로그에 기록되지 않습니다** — 다른 관리 화면들과 다릅니다.

    운영에 쓰기 전에 승급 강제와 감사 기록을 켜야 합니다. 설계 문서도 둘 다
    "필수" 로 적고 있습니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 목록 | `GET /api/v1/admin/token-sources` |
| 이벤트 | `GET /api/v1/admin/token-sources/{id}/events` |
| 상태 변경 | `POST …/{id}/rotate` · `/approve` · `/pause` · `/resume` |
| 승급 | `POST …/{id}/re-auth` |
| 비밀 보기 | `POST …/{id}/reveal` |

## `token-source-sync` 와의 관계

`cmd/token-source-sync` 는 **한 번 돌리는 백필 도구**입니다. 설치가 끝났고
인증 제공자가 OpenBao 인 스택을 돌며 설정에서 토큰 값을 다시 만들어 금고에
넣습니다.

- 사용자가 직접 넣은 PAT(예: GitHub)은 **일부러 비워** 건드리지 않습니다
- `token_sources` 행을 만들지는 **않습니다** — 금고를 고치는 도구이고, 이 화면이
  보여 주는 목록의 출처가 아닙니다

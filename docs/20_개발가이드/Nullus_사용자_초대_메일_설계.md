# Nullus 사용자 초대 메일 설계

조직에 멤버를 초대하면 초대받은 사람이 **메일을 받고, 그 메일로 계정을 완성해 로그인까지** 하는 흐름을 정의합니다.

- 작성일: 2026-08-17
- 대상 모듈: `admin` (주), `auth` (Keycloak 어댑터)
- 선행 문서: `Nullus_OIDC_Provider_가이드.md`, `Nullus_접속_도메인_SSO_가이드.md`
- 상태: **설계 확정 전** — §9의 미결 사항 두 건이 정해져야 착수합니다

---

## 1. 요약

**메일 발송기를 새로 만들지 않습니다. Keycloak이 보냅니다.**

Keycloak Admin API의 `execute-actions-email` 은 계정 완성 링크가 담긴 메일을 직접 보내고, 토큰 생성·만료·1회 사용까지 전부 처리합니다. 우리가 만들면 토큰 테이블·해시 저장·만료·수락 엔드포인트·템플릿·재발송을 모두 짜야 하는데, 그러고도 **Keycloak 계정 생성은 여전히 따로 해야 합니다** — 로그인이 OIDC이기 때문입니다.

| | 공수 | 우리가 짜는 것 |
|---|---|---|
| **A. Keycloak 발송 (채택)** | 약 3일 | 계정 생성 + 메일 트리거 호출 2개 |
| B. 자체 토큰 + 자체 메일러 | 약 5~6일 | A 전부 + 토큰 테이블·수락 API·템플릿·재발송 |

---

## 2. 현재 상태

착수 전에 알아야 할 사실은 **초대 메일이 없다는 것이 아니라, 초대 자체가 동작하지 않는다는 것**입니다.

### 2.1 초대가 반쪽짜리 두 개로 갈려 있다

| 경로 | 실제 동작 |
|---|---|
| `POST /admin/organizations/:orgId/members` | `users` 행 생성(`is_active = false`) + `org_members` 추가 |
| `GET/POST/DELETE .../invites` | **스텁** — `member_handler.go:46-54` 가 하드코딩 응답을 돌려준다 |

```go
// internal/admin/adapter/handler/member_handler.go:49
g.POST("/organizations/:orgId/invites", func(c echo.Context) error {
    return c.JSON(http.StatusOK, map[string]any{"token": "", "url": "", "role": "developer", "expiresAt": ""})
})
```

프런트엔드에는 초대 링크 UI가 완성돼 있어(`useCreateInviteLink`·`useInviteLinks`·`revokeInviteLink`) **지금도 화면에서 빈 링크가 발급됩니다.** `invite_links` 테이블은 마이그레이션에 없습니다.

### 2.2 초대받은 사람은 로그인할 수 없다

기본 인증이 `oidc` 입니다(`values.yaml` 의 `config.auth.mode`). API는 토큰의 `email` 클레임으로 `users` 행을 찾으므로, **계정이 Keycloak에 없으면 애초에 토큰을 받을 수 없습니다.**

`InviteMember`(`internal/admin/usecase/user_usecase.go:41`)는 `userRepo` 만 씁니다. Go 코드 어디에도 Keycloak 사용자 생성 호출이 없습니다 — `scripts/setup-keycloak.sh` 의 부트스트랩이 유일합니다.

### 2.3 SMTP 코드는 있지만 죽어 있다

`internal/shared/notification/notifier.go` 의 `EmailNotifier`(`net/smtp`)는 **생성자가 없고 호출부가 0건**입니다. 필드가 전부 비공개라 패키지 밖에서 만들 수조차 없어, 자기 테스트만 쓰는 코드입니다. 차트 values·`setup-keycloak.sh`·`config.go` 어디에도 SMTP 설정이 없습니다.

> 이 설계는 이 파일을 되살리지 않습니다. §3의 이유로 쓸 일이 없고, 남겨 두면 "메일 보내는 곳이 두 군데" 로 읽힙니다. 별도 PR에서 제거를 제안합니다.

---

## 3. 결정

### D1 — 메일은 Keycloak이 보낸다

```
POST /admin/realms/{realm}/users
     {"email": …, "username": …, "firstName": …, "enabled": true, "emailVerified": false}

PUT  /admin/realms/{realm}/users/{id}/execute-actions-email
     ?client_id=nullus-app&redirect_uri=<앱 주소>&lifespan=<초>
     ["UPDATE_PASSWORD", "VERIFY_EMAIL"]
```

두 번째 호출이 메일 발송까지 끝냅니다. 링크의 토큰·만료·1회 사용은 Keycloak 소관입니다.

**비용**: 초대 기능이 Keycloak에 묶입니다(§6에서 모드별로 분기).
**탈출구**: `MemberInviter` 포트(§4.2)로 추상화해 두므로, 다른 IdP 어댑터를 붙이면 교체됩니다.

### D2 — 계정 완성 액션은 `UPDATE_PASSWORD` + `VERIFY_EMAIL`

비밀번호를 우리가 만들지 않고, 임시 비밀번호를 화면에 띄우지도 않습니다. **자격증명이 우리 코드와 로그를 통과하지 않습니다.**

### D3 — 쓰기 순서는 DB 먼저, Keycloak 나중

두 시스템이라 원자적일 수 없습니다. 부분 실패 시 어느 쪽이 덜 위험한지로 정합니다.

| 순서 | 부분 실패 시 남는 것 | 위험 |
|---|---|---|
| Keycloak 먼저 | 로그인은 되는데 `users` 행이 없는 계정 | **높음** — 인증은 통과하고 인가에서 헤맨다 |
| **DB 먼저 (채택)** | 로그인 불가인 비활성 `users` 행 | 낮음 — 재초대로 이어서 복구된다 |

보상 삭제 대신 **멱등 재시도**로 처리합니다(§4.4). 보상 로직은 그 자체가 실패할 수 있어, 실패 경로가 하나 더 느는 것을 피합니다.

### D4 — 초대 "링크 복사" 기능은 성립하지 않는다

Keycloak Admin API에는 액션 링크를 **돌려받는** 엔드포인트가 없습니다. `execute-actions-email` 은 보내기만 합니다. 링크를 우리가 쥐려면 자체 토큰(B안)으로 가야 합니다.

따라서 `/invites` 스텁 3개를 제거하고, UI의 "초대 링크 생성" 을 **"초대 메일 재발송"** 으로 바꿉니다. 이건 제품 결정이라 §9에 미결로 둡니다.

### D5 — 메일 발송 실패는 초대 실패가 아니다

계정은 만들어졌고 메일만 못 갔다면 초대 자체는 유효합니다. `202 Accepted` 로 응답하고 응답 본문에 `emailSent: false` 와 사유를 담아, 화면이 "재발송" 을 유도합니다. 발송 실패로 초대를 롤백하면 D3의 부분 실패가 다시 생깁니다.

---

## 4. 설계

### 4.1 흐름

```
관리자                API                     DB              Keycloak            초대받은 사람
  │                    │                       │                  │                    │
  ├─ 초대(email,role) ─▶│                       │                  │                    │
  │                    ├─ 기존 사용자 조회 ──────▶│                  │                    │
  │                    ├─ users INSERT ────────▶│  is_active=false │                    │
  │                    ├─ org_members INSERT ──▶│                  │                    │
  │                    ├─ 계정 생성 ─────────────────────────────────▶│  emailVerified=false
  │                    ├─ execute-actions-email ────────────────────▶│                    │
  │                    │                       │                  ├─ 메일 발송 ──────────▶│
  │◀─ 202 emailSent ───┤                       │                  │                    │
  │                    │                       │                  │◀─ 비밀번호 설정 ──────┤
  │                    │                       │                  │◀─ 이메일 인증 ────────┤
  │                    │◀─ OIDC 로그인 ─────────────────────────────┤                    │
  │                    ├─ users UPDATE ────────▶│  is_active=true  │                    │
```

`is_active` 를 `true` 로 올리는 지점은 **첫 로그인 시점**입니다. 초대만으로는 올리지 않습니다 — 초대 상태와 활성 상태가 같은 컬럼이면 "초대했는데 안 들어온 사람" 을 구분할 수 없습니다.

### 4.2 모듈 경계

`admin` 이 `auth` 의 Keycloak 어댑터를 직접 import하면 모듈 규칙 위반입니다(`CLAUDE.md`: 다른 모듈의 internal 패키지 참조 금지). 포트를 `admin` 쪽에 두고 구현체를 주입합니다.

```
internal/admin/port/repository.go
    type MemberInviter interface {
        // 계정을 만들고 계정 완성 메일을 보낸다. 이미 있으면 메일만 다시 보낸다.
        InviteByEmail(ctx context.Context, in InviteInput) error
    }

internal/auth/adapter/keycloak/member_inviter.go   ← 신규, 위 인터페이스를 구현
internal/auth/adapter/keycloak/client.go           ← CreateUser / ExecuteActionsEmail 추가

cmd/api/main.go                                    ← KEYCLOAK_URL 있으면 주입, 없으면 nil
```

`UserUseCase` 는 `MemberInviter` 가 `nil` 이면 **메일 없이 기존 동작 그대로** 갑니다(§6).

### 4.3 API 계약

`POST /api/v1/admin/organizations/:orgId/members` — 요청은 그대로, 응답만 확장합니다.

```jsonc
// 201 Created — 초대 + 메일 발송 성공
{ "id": "…", "email": "…", "role": "developer", "isActive": false,
  "invite": { "emailSent": true, "expiresAt": "2026-08-20T09:00:00Z" } }

// 202 Accepted — 초대는 됐고 메일만 실패 (D5)
{ "id": "…", "…": "…",
  "invite": { "emailSent": false, "reason": "SMTP_UNCONFIGURED" } }
```

신규 — `POST /api/v1/admin/organizations/:orgId/members/:memberId/invite-email` (재발송)

```jsonc
{ "emailSent": true, "expiresAt": "…" }
```

제거 — `GET/POST/DELETE /organizations/:orgId/invites` 스텁 3개 (D4)

`reason` 은 화면에 그대로 띄울 수 있는 코드로 한정합니다: `SMTP_UNCONFIGURED`, `IDP_UNAVAILABLE`, `INVALID_EMAIL`, `RATE_LIMITED`.

### 4.4 멱등성

같은 이메일로 다시 초대하면 **오류가 아니라 재발송**입니다.

| 상태 | 동작 |
|---|---|
| `users` 없음 | 생성 → Keycloak 생성 → 메일 |
| `users` 있고 조직 멤버 아님 | 멤버 추가 → Keycloak 확인/생성 → 메일 |
| `users` 있고 멤버이며 `is_active=false` | **메일만 재발송** (현재는 `USER_ALREADY_MEMBER` 409) |
| `users` 있고 멤버이며 `is_active=true` | `USER_ALREADY_MEMBER` 409 유지 |

세 번째 행이 D3의 부분 실패를 흡수합니다. 지금은 409로 막혀 재시도할 방법이 없습니다.

### 4.5 설정

```yaml
# deploy/helm/nullus/values.yaml
config:
  auth:
    invite:
      # 비우면 초대 메일을 보내지 않는다 (계정만 만들고 202 로 알린다).
      enabled: false
      # 계정 완성 링크 유효 시간. Keycloak 의 lifespan 파라미터로 넘어간다.
      lifespanHours: 72
      # 메일의 링크가 되돌아올 곳. 비우면 web ingress 호스트를 쓴다.
      redirectUri: ""

keycloak:
  smtp:
    # realm 에 설정된다 (setup-keycloak.sh). 차트가 Keycloak 을 띄우지 않는
    # BYO 구성에서는 IdP 쪽에서 직접 설정해야 한다.
    host: ""
    port: 587
    from: "no-reply@nullus.io"
    fromDisplayName: "Nullus"
    starttls: true
    auth: true
    user: ""
    existingSecret: ""       # 비밀번호는 값으로 받지 않는다
```

realm SMTP는 `scripts/setup-keycloak.sh` 의 realm 생성/갱신 payload(현재 `{"realm":"nullus","enabled":true,"sslRequired":"none"}`, 125~135행)에 `smtpServer` 를 더해 설정합니다.

**비밀번호는 values로 받지 않습니다.** `existingSecret` 만 받고, 스크립트가 시크릿에서 읽습니다.

---

## 5. 작업 분해

TDD 순서입니다. 각 단계는 실패하는 테스트부터 씁니다.

| # | 작업 | 산출물 | 테스트 |
|---|---|---|---|
| 1 | `MemberInviter` 포트 + `InviteInput` | `internal/admin/port/repository.go` | 인터페이스 준수 테스트(`interface_compliance_test.go` 패턴) |
| 2 | `InviteMember` 멱등화 (§4.4) | `internal/admin/usecase/user_usecase.go` | 4개 상태 각각 단위 테스트, `MemberInviter` 는 목 |
| 3 | Keycloak `CreateUser` / `ExecuteActionsEmail` | `internal/auth/adapter/keycloak/client.go` | `httptest` 로 Admin API 흉내, 경로·payload·에러 매핑 검증 |
| 4 | `member_inviter.go` 어댑터 | `internal/auth/adapter/keycloak/` | 위 클라이언트를 목으로 두고 포트 계약 검증 |
| 5 | 핸들러 응답 확장 + 재발송 엔드포인트 | `member_handler.go` | `httptest` 로 201/202/409 분기 |
| 6 | 스텁 3개 제거 | `member_handler.go` | — |
| 7 | 배선 | `cmd/api/main.go` | — |
| 8 | 차트 values + realm SMTP | `values.yaml`, `setup-keycloak.sh` | `helm template` 렌더 확인 |
| 9 | UI: 초대 결과 표시, 재발송 버튼, 링크 UI 정리 | `web/src/features/admin/` | `testing-library` |
| 10 | 실 SMTP 종단 검증 | — | 실제 수신 확인 |

10번을 건너뛰면 이 기능은 검증되지 않은 것입니다. 타입 체크와 목 테스트는 메일이 도착한다는 증거가 아닙니다.

---

## 6. 배포 모드별 동작

| 모드 | 초대 메일 | 근거 |
|---|---|---|
| `oidc` + 플랫폼 Keycloak | 동작 | `KEYCLOAK_URL` 로 어댑터 주입 |
| `oidc` + 외부 IdP (BYO) | **미동작** | Admin API 접근 권한이 없다. 계정만 만들고 `202` + `IDP_UNAVAILABLE` |
| `session` | **미동작** | Keycloak 자체가 없다. 화면에서 초대 메일 안내를 감춘다 |

BYO·`session` 에서 기능을 조용히 실패시키지 않습니다 — 응답의 `emailSent: false` 와 `reason` 으로 화면이 사실대로 말하게 합니다.

---

## 7. 운영 준비물

코드보다 이쪽이 병목입니다.

1. **발신 SMTP** — Zadara PoC에서 25/587 아웃바운드 개방 여부가 미확인이고, 어차피 발신 서버가 필요합니다(SES·SendGrid·사내 릴레이). §9의 미결 1.
2. **도달률** — `nullus.io` 에 SPF·DKIM·DMARC. 도메인은 Spaceship에 있어 DNS 레코드 추가 작업입니다. 없으면 스팸함으로 갑니다.
3. **발신 주소** — `no-reply@nullus.io` 수신함 필요 여부(반송 처리).

---

## 8. 리스크

| 리스크 | 완화 |
|---|---|
| 초대 메일이 스팸으로 분류 | SPF·DKIM·DMARC 선행. 종단 검증(작업 10)에 실제 수신 확인 포함 |
| Keycloak 관리자 자격증명이 API 프로세스에 상주 | 이미 그렇다(`cmd/api/main.go:146`). 이 설계가 노출면을 넓히지는 않으나, 서비스 계정 + 최소 역할(`manage-users`)로 좁히는 것을 후속으로 |
| 초대 남용으로 메일 폭주 | 조직당 발송 레이트리밋. 기존 사용자 키 리미터(`rate_limiter.go`) 재사용 |
| 이메일 주소 오타로 남의 메일함에 초대 | 초대 목록에서 취소(계정 삭제) 제공. §9 미결 2와 함께 결정 |
| Keycloak 장애 시 초대 실패 | D5로 초대 자체는 성립, 재발송으로 복구 |

---

## 9. 미결 사항

착수 전에 정해야 합니다.

1. **발신 인프라** — SES / SendGrid / 사내 릴레이 중 무엇인가. 이게 정해져야 §4.5의 values와 §7이 확정됩니다.
2. **초대 링크 공유를 접는가** (D4) — 기획 문서(`Nullus_기능목록.md` 128~129행)는 "초대 링크 생성 → 공유 → 수락" 을 Beta로 잡고 있습니다. Keycloak 경로에서는 링크를 우리가 쥘 수 없으므로, 링크 공유가 필수 요건이면 B안(자체 토큰)을 병행해야 하고 공수가 5~6일로 늘어납니다.

부수로 정리할 것:

- `internal/shared/notification/notifier.go` 의 죽은 `EmailNotifier` 제거 (별도 PR)
- `/invites` 스텁 3개 제거 — 지금도 화면에서 빈 링크가 발급되는 결함

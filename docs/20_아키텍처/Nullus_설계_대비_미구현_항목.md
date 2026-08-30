# Nullus 설계 대비 미구현 항목

**작성일**: 2026-03-30
**최종 재검증**: 2026-08-31
**버전**: 1.2
**기준 문서**: `docs/20_아키텍처/Nullus_시스템_아키텍처.md`
**비교 대상 구현**: `draft` 실제 코드베이스 (`cmd/`, `internal/`, `web/`, `db/migrations/`, `deploy/`)

---

## 1. 문서 목적

이 문서는 설계 문서에 명시되었지만, 현재 `draft` 구현에서 아직 **실제 동작 코드 또는 운영 가능한 연결 형태로 확인되지 않은 항목만** 별도로 정리한다.

다음 항목은 본 문서에서 제외한다.

- 경로/이름만 바뀐 항목
- Stack 영역처럼 설계 문서의 "구현 동기화 업데이트"에 이미 반영된 항목
- 부분 구현은 되었지만 설계와 방식이 달라진 항목

---

## 2. 미구현 항목 요약 (2026-08-31 재검증)

3월 30일 목록을 코드로 다시 확인한 결과다. 그 사이 해소된 항목은 아래 2.1 로 옮겼다.

| 영역 | 미구현 항목 | 확인 근거 |
|------|-------------|-----------|
| 인증/API | `/api/v1/auth` 의 `logout`·`me`·`token/refresh` | `login` 은 구현됐다(2.1). 나머지 셋은 라우트가 없다 — 로그아웃은 클라이언트가 토큰을 버리는 것으로, 갱신은 IdP·`pkg/nullusclient` 가 직접 처리한다 |
| 설치 엔진 | Operator/Agent 런타임, 파일 기반 카탈로그 엔진 | 설치는 API 프로세스 내 오케스트레이터가 직접 수행 |
| 설치 엔진 | `TIMEOUT`, `PARTIAL_SUCCESS` 상태 | `internal/stack/domain` 의 상태 집합에 없다. 재시도·이어하기는 상태가 아니라 API 로 표현된다(2.1) |
| 데이터 모델 | `sessions`, `rbac_policies`, `menu_permissions`, `pipeline_configs` | 마이그레이션 `000074` 기준으로 존재하지 않음. 인가는 라우트 그룹의 `RequireRole` 하나로 판정한다(DB 스키마 문서 0.1 참고) |
| 이벤트/연동 | 도메인 이벤트 기반 컨텍스트 간 자동 동기화 | 모듈 간 연결은 포트 직접 호출 방식 |
| 관측성 | `metrics/summary` API | observability 라우트는 `/dashboard`, `/deployed-apps`, `/alert-rules`, `/alert-history` 뿐 |
| 배선 | `CompatibilityHandler.RegisterAdminRoutes` | 핸들러는 있는데 `main.go` 가 부르지 않는다. 관리자 화면의 매트릭스 생성·수정·삭제 버튼 3개가 404 로 끝난다 — 상세는 3.7 |

### 2.1 해소된 항목

| 항목 | 해소 시점 | 현재 상태 |
|------|-----------|-----------|
| retry/partial_success 상태 및 재시도 API | ~2026-08 | **구현됨** — `POST /api/v1/stacks/:id/retry`, `POST /api/v1/stacks/:id/continue` (`deploy_handler.go`). 이력은 `GET /api/v1/stacks/:id/retry-history` |
| ~~Alert Rule 과 실제 알림 발송의 운영 연결~~ | — | **정정 (2026-08-31)** — 2026-08-11 판에서 해소로 옮겼으나 **오판이었다.** API·테이블·notifier 구현체는 다 있지만 셋을 잇는 배선이 없다. 3.5 B 로 되돌린다 |
| 상위 `users` RBAC 관리 API | — | **부분 구현** — `/api/v1/admin/organizations/:orgId/members/*` 로 조직 멤버·역할을 관리한다. 설계안의 전역 `users` RBAC 테이블 방식은 아니다 |
| **설치 로그 DB 영속화** | 2026-08-21 (PR #207) | **구현됨** — `stack_deploy_logs` 테이블(마이그레이션 `000074`)과 `PersistentStreamer`. `main.go` 는 `NewPersistentStreamer(NewMemoryStreamer(), NewPostgresDeployLogStore(pool))` 를 쓴다. 메모리 스트리머는 사라진 게 아니라 **실시간 구독 계층으로 남고** 그 뒤에 DB 가 붙는 구조다 |
| **`deployments`·`deployment_logs` 테이블** | 2026-08-21 | **이름을 바꿔 구현됨** — 배포 기록은 `pipeline_deployments`, 설치 로그는 `stack_deploy_logs`. 설계안의 `deployment_steps` 는 테이블이 아니라 `pipeline_deployments.steps` JSONB 로 들어갔다(`000071`) |
| **`/api/v1/auth/login`** | 2026-08-19 | **구현됨** — ID/PW 로그인이 OIDC 와 나란히 선다. 자격은 `users.password_hash`(bcrypt, `000073`), 토큰은 HS256 로컬 발급. `auth.session.secret` 이 비면 라우트가 등록되지 않는다 |
| **프런트 OIDC 런타임 연결** | 2026-08-19 | **구현됨** — 로그인 화면이 OIDC 리디렉션과 ID/PW 를 나란히 제공하고, 역할별 랜딩은 `web/src/features/auth/role-landing.ts` 한 곳에서 나온다. `VITE_AUTH_MODE=mock` 은 여전히 **로컬 개발 기본값**이지만 배포 차트 기본값은 `oidc` 다 |
| **도달할 수 없던 화면 4개** | 2026-08-11 (PR #133) | **배선됨** — `cicd-golden-path`, `cicd-pipeline-setup`, `stack-deployment-logs`, `token-management` 모두 라우트에 있고 앞 둘·넷째는 사이드바에도 있다 |
| **`PipelineHandler.RegisterStackRoutes` 미등록** | 2026-08-11 (PR #133) | **배선됨** — `GET /api/v1/stacks/:stackId/pipelines` 가 `stacks` 그룹에 붙는다 |

---

## 3. 상세 목록

### 3.1 인증 및 사용자 API

#### A. `/api/v1/auth` 중 `logout`·`me`·`token/refresh` 미구현 — **login 은 해소됨**

설계 문서는 `/auth/login`, `/auth/logout`, `/auth/me`를 API Server의 기본 진입점으로 상정한다.
이 중 **`POST /api/v1/auth/login` 은 2026-08-19 에 구현됐다**(`internal/auth/adapter/handler/login_handler.go`).
나머지 셋은 여전히 라우트가 없고, 이는 미룬 것이 아니라 필요하지 않다고 판단한 결과다.

| 엔드포인트 | 상태 | 사유 |
|-----------|------|------|
| `POST /auth/login` | **구현됨** | ID/PW 경로. IdP 장애가 곧 전면 잠금이 되지 않도록 OIDC 와 나란히 둔다 |
| `POST /auth/logout` | 미구현 | 로컬 토큰은 서버가 상태를 들고 있지 않아 무효화할 대상이 없다. OIDC 로그아웃은 IdP 의 end-session 이 맡는다 |
| `GET /auth/me` | 미구현 | 사용자 정보는 로그인 응답과 토큰 클레임에서 온다. 설계안의 `permissions` 배열 권한 모델도 채택하지 않았다 — 인가는 라우트 그룹의 `RequireRole` 하나로 판정한다 |
| `POST /auth/token/refresh` | 미구현 | 브라우저는 `oidc-client-ts`, CLI·MCP 는 `pkg/nullusclient.EnsureFreshToken` 이 IdP 와 직접 갱신한다. API 가 중계하면 refresh token 이 서버를 한 번 더 지나간다 |

- 설계 근거: `Nullus_시스템_아키텍처.md` 5.1 API Server
- 구현 확인:
  - `cmd/api/main.go` (`loginHandler.RegisterRoutes`)
  - `internal/auth/adapter/handler/login_handler.go`

#### B. 세션 **쿠키** 기반 인증은 채택하지 않았다 — 설계 변경

설계 문서는 Alpha/Beta 단계 인증을 세션 쿠키(gorilla/sessions + PostgreSQL 저장소)로 정의했다.
현재 구현은 그 길로 가지 않았고, 두 갈래로 갈렸다.

- **`auth.mode=session`** — 여전히 `X-User-*` 헤더를 그대로 믿는 단순화 버전이다.
  클라이언트가 `X-User-Role: admin` 을 붙이면 관리자가 되므로 **로컬 개발 전용**이다.
  브라우저 WebSocket 이 이 헤더를 실을 수 없어, session 모드에서는 WS 인증을 아예 끈다
  (켜면 검증이 아니라 기능만 죽는다).
- **ID/PW 로그인** — 쿠키 대신 HS256 로컬 발급 토큰을 응답 본문으로 돌려주고, 클라이언트가
  `Authorization: Bearer` 로 싣는다. 세션 저장소가 없으므로 `sessions` 테이블도 만들지 않았다.

즉 이 항목은 "미완료" 가 아니라 **설계와 다른 방식으로 정리된** 항목이다.

- 현재 구현:
  - `internal/auth/adapter/middleware/auth_middleware.go` (헤더 방식)
  - `internal/auth/adapter/token/local_token.go` (HS256 발급·검증)
  - `authmw.DualAuthMiddleware` — OIDC JWT 와 로컬 토큰을 함께 받는다

#### C. 상위 `users` RBAC 관리 API 미구현

설계 문서는 `/api/v1/users`, `PUT /:userId/role`, `DELETE /:userId`를 별도 User/RBAC Handler로 정의한다.
현재 구현은 `admin/organizations/:orgId/members` 중심의 멤버 관리만 제공하고, 설계 문서 수준의 상위 `users` 관리 API는 없다.

- 설계 근거: 기능 9, API Server `users` 섹션
- 현재 구현:
  - `internal/admin/adapter/handler/member_handler.go`

#### D. 프런트 OIDC 런타임 연결 — **해소됨 (2026-08-19)**

placeholder 상태였던 항목이 실제 연결로 바뀌었다. 로그인 화면은 OIDC 리디렉션과 ID/PW 를
나란히 제공하고, 로그인 뒤 이동할 곳은 `web/src/features/auth/role-landing.ts` 한 곳에서
나온다 — 예전에는 로그인 리다이렉트와 홈의 시작 버튼이 각자 목록을 들고 있어 developer 만
서로 다른 곳을 가리켰다.

- 현재 구현:
  - `web/src/lib/oidc-providers.ts` — `VITE_AUTH_MODE` 로 provider 설정을 푼다
  - `web/src/features/auth/pages/login-page.tsx` — OIDC / ID+PW 두 경로
  - `web/src/features/auth/role-landing.ts` — admin `/admin/organization`,
    devops `/stack/templates`, developer `/cicd/developer-deploy`
- 남은 것: 로컬 기본값은 여전히 `VITE_AUTH_MODE=mock` 이다(`web/.env.example`).
  배포 차트 기본값은 `oidc` 이므로 운영 경로와는 어긋나지 않는다.

---

### 3.2 설치 엔진 및 대상 클러스터 런타임

#### A. Nullus Operator / Agent 런타임 미구현

설계 문서는 대상 Kubernetes 클러스터 내부 `nullus-system` 네임스페이스에 Operator/Agent가 설치되어 상태 감시와 헬스체크를 담당하는 구조를 전제한다.
현재 저장소에는 해당 런타임 컴포넌트나 별도 배포 단위가 없다.

- 설계 근거:
  - 아키텍처 개요
  - 7.2 네임스페이스 구조
- 현재 구현:
  - Helm 기반 Stack 오케스트레이터는 있으나 Operator 모듈/배포 없음

#### B. 설치 상태 `TIMEOUT`, `PARTIAL_SUCCESS` 미구현 — `RETRYING` 은 상태 대신 API 로

설계 문서의 상태 머신에는 `RETRYING`, `TIMEOUT`, `PARTIAL_SUCCESS`가 포함된다.
실제 상태 집합은 `pending`, `validating`, `installing`, `configuring`, `health_check`,
`completed`, `cancelled`, `failed`, `rolling_back`, `rolled_back` 이다.

`RETRYING` 은 상태로 만들지 않았다 — 재시도는 실패한 배포를 다시 `installing` 으로
되돌리는 일이고, 별도 상태를 두면 진행률 계산(`InstallStepOrder`)이 두 갈래가 된다.
대신 재시도 자체를 API 로 노출하고(3.2 C), 시도 이력은 `GET /stacks/:id/retry-history` 로
읽는다. `TIMEOUT`·`PARTIAL_SUCCESS` 는 여전히 없다 — 둘 다 `failed` 로 수렴한다.

- 설계 근거: 5.2 Install Engine 상태 머신
- 현재 구현:
  - `internal/stack/domain/` (상태 집합), `internal/stack/domain/deploy_steps.go` (진행률)

#### C. 설치 재시도 API — **해소됨**

설계 문서에는 실패 단계 재시도용 `POST /api/v1/installations/:id/retry`가 있다.
경로 이름은 다르지만 같은 일을 하는 API 가 있고, 오히려 두 갈래로 나뉘어 있다.

| 엔드포인트 | 하는 일 |
|-----------|---------|
| `POST /api/v1/stacks/:id/retry` | 실패한 배포를 처음부터 다시 시도한다 |
| `POST /api/v1/stacks/:id/continue` | 실패 지점부터 이어서 진행한다 |
| `GET /api/v1/stacks/:id/retry-history` | 시도 이력을 읽는다 |

- 현재 구현:
  - `internal/stack/adapter/handler/deploy_handler.go`
  - `internal/stack/adapter/handler/retry_history_handler.go`

#### D. 배포 로그 DB 영속화 — **해소됨 (2026-08-21, 마이그레이션 `000074`)**

설계 문서는 설치 로그를 WebSocket 전송과 함께 DB에도 저장한다고 적고 있다.
2026-08-11 시점에는 `MemoryStreamer` 뿐이라 미구현이었고, 실제로 그 때문에 사고가 났다 —
설치는 20~30분짜리인데 그 사이 API 파드가 재시작되면 로그가 통째로 사라져, 무엇이 왜
멈췄는지 사후에 알 방법이 없었다.

지금은 `PersistentStreamer` 가 메모리 스트리머를 감싸고 그 뒤에 DB 를 둔다. 메모리
계층을 걷어내지 않은 이유는 실시간 구독이 여전히 그쪽 일이기 때문이다 — DB 는 재기동을
견디는 몫만 맡는다.

- 현재 구현:
  - `internal/stack/adapter/log/persistent_streamer.go`
  - `internal/stack/adapter/repository/postgres_deploy_log.go`
  - `cmd/api/main.go` — `NewPersistentStreamer(NewMemoryStreamer(), NewPostgresDeployLogStore(pool))`
- 스키마: `stack_deploy_logs (seq, deployment_id, logged_at, level, step, phase, message)`.
  정렬 기준이 타임스탬프가 아니라 `seq` 인 이유는 같은 밀리초에 여러 줄이 들어오기 때문이다.

#### E. 파일 기반 Compatibility / Known Issues 카탈로그 엔진 미구현

설계 문서는 `templates/compatibility/compatibility-matrix.yaml`, `known-issues.yaml` 기반 엔진을 설명한다.
현재 `draft/templates/compatibility`, `draft/templates/known-issues` 디렉터리는 비어 있고, 실제 구현은 DB 테이블과 PostgreSQL repository를 사용한다.

- 설계 근거:
  - 5.2 known-issues.yaml
  - 5.3 Compatibility Matrix Engine
- 현재 구현:
  - `db/migrations/000004_compatibility.up.sql`
  - `db/migrations/000012_known_issues.up.sql`
  - `internal/stack/adapter/repository/postgres_compatibility.go`

#### F. 명시적 도메인 DAG/병렬 Step 실행기 미구현

설계 문서는 "각 Step은 독립 goroutine, 이전 Step 완료 대기 후 실행(DAG 기반)"을 설명한다.
현재 구현은 `installPhases` 고정 순서와 Helm 오케스트레이터의 사전 정의된 step order를 중심으로 동작하며, 별도 `engine/dag` 형태의 실행 계층은 확인되지 않는다.

- 설계 근거: 5.2 Step Runner 설명
- 현재 구현:
  - `internal/stack/usecase/install_stack.go`
  - `internal/stack/adapter/helm/orchestrator.go`

---

### 3.3 데이터 모델 및 저장소

#### A. `deployments`·`deployment_logs`·`deployment_steps` — **이름을 바꿔 구현됨**

설계 문서는 배포 이력·로그·단계를 세 테이블로 분리한다. 이름은 셋 다 남지 않았지만,
셋이 하던 일은 지금 모두 어딘가에 있다.

| 설계안 | 현행 | 형태가 달라진 이유 |
|--------|------|-------------------|
| `deployments` | `pipeline_deployments` | 배포 기록의 주인이 Stack 이 아니라 파이프라인으로 정리됐다 |
| `deployment_logs` | `stack_deploy_logs` (`000074`) | 스택 **설치** 로그에 한정한다. 파이프라인 실행 로그는 저장하지 않고 CI 서버에서 읽는다 — 같은 내용을 두 벌 들고 있으면 어느 쪽이 사실인지 정해야 한다 |
| `deployment_steps` | `pipeline_deployments.steps` JSONB (`000071`) | 단계 목록은 항상 배포 하나와 함께 읽고 따로 질의하지 않아 테이블로 쪼갤 이유가 없었다 |

Stack 설정 이력은 그대로 `stacks` + `stack_config_versions` 다.

- 현재 구현:
  - `db/migrations/000071_deployment_steps.up.sql`
  - `db/migrations/000074_stack_deploy_logs.up.sql`

#### B. `sessions`, `rbac_policies`, `menu_permissions` 테이블 미구현 — 유지

설계 문서와 DB 스키마 문서는 Auth Context에 `sessions`, `rbac_policies`, `menu_permissions`를 제시한다.
`000074` 기준으로 셋 다 없고, 앞으로도 만들 계획이 아니다.

- `sessions` — 세션이 서버 상태가 아니라 토큰이다. 다만 그 자리 **일부**는
  `users.password_hash`(`000073`)가 대신한다. DB 에 남는 것은 자격뿐이고 세션은 남지 않는다.
- `rbac_policies` / `menu_permissions` — 인가는 라우트 그룹에 걸린 `RequireRole` 하나로
  판정하고, 메뉴 노출은 프런트의 `nav-model.tsx` 가 같은 역할값으로 거른다. 정책을 DB 로
  옮기면 두 곳이 각자 판단하게 되고, 어긋나도 아무도 눈치채지 못한다.

- 현재 구현:
  - `db/migrations/*.sql` 기준 미존재
  - `internal/auth/adapter/middleware/`, `web/src/components/layout/nav-model.tsx`

#### C. 설계 ERD 기준 `pipeline_configs`, `alert_configs` 테이블 미구현

설계는 `pipeline_configs`, `alert_configs`라는 이름의 설정 테이블을 사용한다.
현재 구현은 `pipelines`, `pipeline_deployments`, `alert_rules`, `alerts` 구조를 사용하므로, 설계안의 해당 테이블 구조는 별도로 구현되지 않았다.

- 설계 근거: 6.1 ERD
- 현재 구현:
  - `db/migrations/000002_cicd.up.sql`
  - `db/migrations/000003_observability.up.sql`

---

### 3.4 컨텍스트 간 이벤트 기반 동기화

#### A. 도메인 이벤트 기반 자동 동기화 미구현

설계 문서와 DB 스키마 문서는 `OrganizationDeleted`, `ClusterDeleted`, `StackDeployed`, `PipelineDeployed` 같은 도메인 이벤트 기반 동기화를 제안한다.
현재 `internal/shared/domain/event.go`에는 EventBus 추상화가 있지만, 실제 publish/subscribe wiring은 확인되지 않는다.

- 설계 근거:
  - 설계 문서 6.3 데이터 소스 경계
  - DB 스키마 문서 4.3 도메인 이벤트 기반 동기화
- 현재 구현:
  - `internal/shared/domain/event.go`
  - 실제 사용처 검색 시 production wiring 부재

---

### 3.5 관측성 및 알림

#### A. `metrics/summary` API 미구현

설계 문서는 Monitoring Handler에 `GET /metrics/summary`를 포함한다.
현재 구현은 `GET /observability/dashboard`와 Alert Rule/History API 위주이며, 별도 summary endpoint는 없다.

- 설계 근거: 5.1 API Server, 기능 7
- 현재 구현:
  - `internal/observability/adapter/handler/dashboard_handler.go`
  - `internal/observability/adapter/handler/alert_handler.go`

#### B. Alert Rule과 실제 알림 발송 파이프라인의 운영 연결 미구현 — **유지 (2026-08-31 재확인)**

> 2026-08-11 판은 이 항목을 2.1(해소)로 옮겼다. **그 판단이 틀렸다.** 조각은 다
> 있지만 조각을 잇는 코드가 없다. 되돌린다.

끊긴 자리는 세 군데다.

1. **규칙을 평가하는 것이 없다.** `internal/observability/usecase/` 에는 alert rule 의
   생성·조회·수정·삭제만 있다. 임계값을 실제 메트릭과 견주는 루프도, 스케줄러도 없다.
2. **notifier 를 만드는 곳이 없다.** `SlackNotifier`·`EmailNotifier`·`MultiNotifier` 는
   구현돼 있고 테스트도 있지만, `internal/shared/notification` 밖에서 이 타입을 참조하는
   프로덕션 코드가 **하나도 없다**(`cmd/api/main.go` 포함).
3. **`notification_history` 에 쓰는 코드가 없다.** `notification_handler.go` 는 `SELECT`
   만 한다. 화면에 보이는 이력 행은 데모 시드(`000021_seed_demo_data`)가 넣은 것이다.

즉 알림 규칙을 만들고 Slack webhook 을 등록해도 **알림은 오지 않는다.** 화면상으로는
설정이 저장되므로 동작하는 것처럼 보인다 — 이 항목의 위험은 여기에 있다.

- 설계 근거: 기능 7 알림 연동 (Slack)
- 현재 구현(조각):
  - `internal/observability/adapter/handler/alert_handler.go` — 규칙 CRUD
  - `internal/admin/adapter/handler/notification_handler.go` — 채널 설정 CRUD + 이력 조회
  - `internal/shared/notification/notifier.go` — 발송 구현체, 호출부 없음

---

### 3.6 문서/도구 체계

#### A. swaggo/swag 기반 OpenAPI 자동 생성 연결 미확인

설계 문서는 OpenAPI 3.0을 Go 구조체에서 자동 생성한다고 적고 있다.
하지만 현재 코드에서 `swaggo/swag` 연동 흔적은 확인되지 않았고, `api/openapi.yaml`은 정적 산출물로 보인다.

- 설계 근거: 기술 스택, API 문서 자동 생성
- 현재 구현:
  - `api/openapi.yaml` 파일은 존재
  - `go.mod`, `cmd/`, `internal/`에서 swaggo 연동 흔적 미확인

---

### 3.7 배선 누락 (2026-08-31 신규)

#### A. `CompatibilityHandler.RegisterAdminRoutes` 미등록 — 관리자 화면의 버튼 3개가 404

핸들러는 호환성 매트릭스 생성·수정·삭제 3개를 정의하지만 `cmd/api/main.go` 는
`RegisterRoutes`(조회·검증)만 호출한다. 그래서 이 3개는 어느 라우트 그룹에도 붙지 않는다.

프런트는 반대로 그 API 가 있다고 보고 이미 호출한다 —
`web/src/features/stack/api/stack-api.ts` 의 `createMatrix`·`updateMatrix`·`deleteMatrix`
가 `POST`·`PUT`·`DELETE /api/v1/admin/compatibility/matrices[/:id]` 를 부르고,
`/admin/stack-versions` 화면의 "새 매트릭스"·"수정"·"삭제" 버튼이 모두 여기에 걸린다.

결과적으로 **호환성 매트릭스를 늘리거나 고치는 유일한 방법은 마이그레이션 시드**다.
등록 위치를 `admin` 그룹으로 잡으면 프런트가 이미 부르는 경로와 그대로 맞는다.

- 현재 구현:
  - `internal/stack/adapter/handler/compatibility_handler.go` (`RegisterAdminRoutes`)
  - `cmd/api/main.go` — `compatHandler.RegisterRoutes(stacks)` 만 있음
  - `web/src/features/admin/pages/stack-versions-page.tsx`

---

## 4. 해석 시 주의사항

- 본 문서는 "설계 대비 아직 없음"만 모은 목록이다.
- 현재 구현이 설계와 다른 방식으로 동작하지만 충분히 운영 가능한 항목은 의도적으로 제외했다.
- 특히 Stack 영역은 설계 문서 후반의 "구현 동기화 업데이트"가 이미 일부 차이를 흡수하고 있으므로, 해당 범위는 미구현보다 "설계 수정 반영"으로 보는 편이 맞다.

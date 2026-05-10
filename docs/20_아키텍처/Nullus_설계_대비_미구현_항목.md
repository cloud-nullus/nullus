# Nullus 설계 대비 미구현 항목

**작성일**: 2026-03-30
**버전**: 1.0
**기준 문서**: `docs/20_아키텍처/Nullus 상세 기능 명세 및 시스템 아키텍처.md`
**비교 대상 구현**: `draft` 실제 코드베이스 (`cmd/`, `internal/`, `web/`, `db/migrations/`, `deploy/`)

---

## 1. 문서 목적

이 문서는 설계 문서에 명시되었지만, 현재 `draft` 구현에서 아직 **실제 동작 코드 또는 운영 가능한 연결 형태로 확인되지 않은 항목만** 별도로 정리한다.

다음 항목은 본 문서에서 제외한다.

- 경로/이름만 바뀐 항목
- Stack 영역처럼 설계 문서의 "구현 동기화 업데이트"에 이미 반영된 항목
- 부분 구현은 되었지만 설계와 방식이 달라진 항목

---

## 2. 미구현 항목 요약

| 영역 | 미구현 항목 |
|------|-------------|
| 인증/API | `/api/v1/auth` REST 엔드포인트 세트, 세션 쿠키 기반 인증 완료형 플로우, 상위 `users` RBAC 관리 API |
| 설치 엔진 | Operator/Agent 런타임, retry/timeout/partial_success 상태 및 재시도 API, 로그 DB 영속화, 파일 기반 카탈로그 엔진 |
| 데이터 모델 | `deployments`, `deployment_logs`, `sessions`, `rbac_policies`, `menu_permissions` 등 설계 테이블 |
| 이벤트/연동 | 도메인 이벤트 기반 컨텍스트 간 자동 동기화 |
| 관측성/알림 | `metrics/summary` API, Alert Rule과 실제 알림 발송의 운영 연결 |
| 프론트 인증 | OIDC 리디렉션/콜백/로그아웃의 완전한 프런트 런타임 연결 |

---

## 3. 상세 목록

### 3.1 인증 및 사용자 API

#### A. `/api/v1/auth` REST 엔드포인트 세트 미구현

설계 문서는 `/auth/login`, `/auth/logout`, `/auth/me`를 API Server의 기본 진입점으로 상정한다.
하지만 현재 `main.go`에는 `admin`, `stacks`, `cicd`, `observability` 그룹만 등록되어 있고, 별도의 `/api/v1/auth` 라우트는 없다.

- 설계 근거: `Nullus 상세 기능 명세 및 시스템 아키텍처.md` 5.1 API Server
- 구현 확인:
  - `cmd/api/main.go`
  - `internal/*/adapter/handler/*.go`

#### B. 세션 쿠키 기반 인증의 운영형 구현 미완료

설계 문서는 Alpha/Beta 단계 인증을 세션 기반으로 정의하지만, 현재 `AuthMiddleware`는 실제 세션 저장소/쿠키 검증 대신 `X-User-*` 헤더를 읽는 단순화 버전이다.

- 설계 근거: 인증 (Alpha/Beta) = 세션 기반
- 현재 구현:
  - `internal/auth/adapter/middleware/auth_middleware.go`
  - 주석상 "production 에서는 gorilla/sessions 로 대체"라고 명시

#### C. 상위 `users` RBAC 관리 API 미구현

설계 문서는 `/api/v1/users`, `PUT /:userId/role`, `DELETE /:userId`를 별도 User/RBAC Handler로 정의한다.
현재 구현은 `admin/organizations/:orgId/members` 중심의 멤버 관리만 제공하고, 설계 문서 수준의 상위 `users` 관리 API는 없다.

- 설계 근거: 기능 9, API Server `users` 섹션
- 현재 구현:
  - `internal/admin/adapter/handler/member_handler.go`

#### D. 프런트 OIDC 런타임 연결 미완료

프런트는 OIDC 관련 패키지와 provider 추상화는 포함하고 있지만, 실제 `AuthProvider` 래핑과 로그인 리디렉션 호출은 placeholder 상태다.

- 현재 구현:
  - `web/package.json`에 `react-oidc-context`, `oidc-client-ts` 포함
  - `web/src/main.tsx`의 `OIDCWrapper`는 TODO 상태
  - `web/src/features/auth/pages/login-page.tsx`의 OIDC 로그인 버튼은 실제 redirect 호출이 없음

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

#### B. 설치 상태 `RETRYING`, `TIMEOUT`, `PARTIAL_SUCCESS` 미구현

설계 문서의 상태 머신에는 `RETRYING`, `TIMEOUT`, `PARTIAL_SUCCESS`가 포함된다.
현재 실제 상태 enum은 `pending`, `validating`, `installing`, `configuring`, `health_check`, `completed`, `cancelled`, `failed`, `rolling_back`, `rolled_back`까지만 구현되어 있다.

- 설계 근거: 5.2 Install Engine 상태 머신
- 현재 구현:
  - `internal/stack/domain/stack.go`

#### C. 설치 재시도 API 미구현

설계 문서에는 실패 단계 재시도용 `POST /api/v1/installations/:id/retry`가 있다.
현재 배포 API는 시작/상태/로그/히스토리/롤백까지만 있고, retry 전용 엔드포인트는 없다.

- 설계 근거: 기능 4 하위 API 표
- 현재 구현:
  - `internal/stack/adapter/handler/deploy_handler.go`
  - `internal/stack/adapter/handler/history_handler.go`

#### D. 배포 로그 DB 영속화 미구현

설계 문서는 설치 로그를 WebSocket 전송과 함께 DB에도 저장한다고 적고 있다.
현재 Stack 로그 스트리밍은 `MemoryStreamer` 기반 인메모리 버퍼로만 유지된다.

- 설계 근거: 5.2 Log Streamer
- 현재 구현:
  - `internal/stack/adapter/log/memory_streamer.go`
- 참고:
  - Stack 버전 이력은 `stack_config_versions`로 저장되지만, 배포 로그 전용 테이블은 없음

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

#### A. Stack 배포 전용 `deployments`, `deployment_logs` 테이블 미구현

설계 문서는 Stack 배포 이력과 로그를 별도 `deployments`, `deployment_logs` 테이블로 분리한다.
현재 Stack 영역은 `stacks`와 `stack_config_versions` 중심이고, Stack 배포 로그는 DB에 저장되지 않는다.

- 설계 근거: 6.1 ERD
- 현재 구현:
  - `db/migrations/000001_init.up.sql`
  - `db/migrations/000005_history.up.sql`

#### B. `sessions`, `rbac_policies`, `menu_permissions` 테이블 미구현

설계 문서와 DB 스키마 문서는 Auth Context에 `sessions`, `rbac_policies`, `menu_permissions`를 제시한다.
현재 마이그레이션에는 이들 테이블이 없다.

- 설계 근거:
  - 설계 문서 6.1 ERD
  - DB 스키마 문서 Auth Context
- 현재 구현:
  - `db/migrations/*.sql` 기준 미존재

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

#### B. Alert Rule과 실제 알림 발송 파이프라인의 운영 연결 미구현

현재는 Alert Rule CRUD와 Notification Config CRUD는 존재하지만, Alert 발생 시 Slack/Email notifier를 실제로 호출하는 운영 경로는 코드상 확인되지 않는다.
`shared/notification/notifier.go`는 존재하지만 테스트 외 production wiring 사용처가 없다.

- 설계 근거: 기능 7 알림 연동 (Slack)
- 현재 구현:
  - `internal/observability/adapter/handler/alert_handler.go`
  - `internal/admin/adapter/handler/notification_handler.go`
  - `internal/shared/notification/notifier.go`

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

## 4. 해석 시 주의사항

- 본 문서는 "설계 대비 아직 없음"만 모은 목록이다.
- 현재 구현이 설계와 다른 방식으로 동작하지만 충분히 운영 가능한 항목은 의도적으로 제외했다.
- 특히 Stack 영역은 설계 문서 후반의 "구현 동기화 업데이트"가 이미 일부 차이를 흡수하고 있으므로, 해당 범위는 미구현보다 "설계 수정 반영"으로 보는 편이 맞다.

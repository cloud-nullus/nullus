# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).


## [Unreleased] - 2026-03-28

### Verified (E2E / UAT / Demo 시나리오 전체 검증)

전체 405건 테스트 실행 — **통과율 97.8%** (396 pass / 9 fail)

#### Vitest 단위 + UAT — 305/305 ✅ (100%)
- 46개 테스트 파일, 305개 테스트 케이스 전체 통과
- UAT-1 DevOps 시나리오 7건, UAT-2 Developer 시나리오 10건, UAT-3 Admin 시나리오 11건 포함

#### Go 단위 + E2E/UAT — 31/31 ✅ (100%)
- `internal/...` 28개 패키지 전부 PASS (admin, auth, cicd, observability, stack, shared)
- Go E2E UAT 3건 PASS: TestUAT1_Mijeong_JuniorDevOps, TestUAT2_Jieun_Developer, TestUAT3_Admin_PlatformSetup

#### Playwright E2E — 88/97 ⚠️ (90.7%)
- 전체 통과 스위트 (0 실패): navigation, theme-i18n, sidebar, uat-devops, uat-admin, uat-developer, rbac-menu-visibility
- 9건 실패 — **기능 결함 0건**, 전부 Phase 1–2 UI 리팩토링 후 E2E 셀렉터 미갱신:
  - `stack-workflow` 3건: YAML View `stackName:` 셀렉터, Resources `개발자 수` 라벨, Stack 생성 API 시드 의존
  - `admin-scenarios` 2건: `#organization-status` ID 변경, `select an organization` 텍스트 변경
  - `devops-scenarios` 4건: 템플릿 카드 `button`→`div[role="button"]` 변경, Auto/Manual 토글 UI 변경, 이력 시드 미존재, YAML 에디터 조건부 렌더링

#### 기능 커버리지 — F0–F12 전체 커버 ✅
- 13개 기능 모두 최소 2개 이상의 테스트 레이어(Playwright + Vitest + Go E2E)에서 검증됨

### Added (테스트 커버리지 보강 — Phase 1–4)

#### Backend 테스트 (16개 신규 파일)

- **Stack domain 단위 테스트**: `compatibility_test.go`, `history_test.go`, `resource_default_test.go`, `template_test.go` — 순수 도메인 엔티티 검증 (domain 45%→90%)
- **Stack Postgres repository 통합 테스트**: `postgres_integration_test.go` — testcontainers 기반 Stack/Template/History/Compatibility/ResourceDefault CRUD 검증
- **Admin Postgres repository 통합 테스트**: `postgres_integration_test.go` — Org/Cluster/User/Member/KnownIssues CRUD + kubeconfig 저장/조회 검증
- **CICD Postgres repository 통합 테스트**: `postgres_integration_test.go` — Pipeline/Template/Deployment CRUD + 정렬 검증
- **Observability domain 단위 테스트**: `alert_test.go`, `dashboard_test.go`, `errors_test.go` — AlertRule/Dashboard/sentinel error 검증
- **Observability Postgres repository 통합 테스트**: `postgres_integration_test.go` — AlertRule CRUD, Alert 생성/목록 검증
- **Port 인터페이스 컴파일 타임 검증**: `stack/port`, `admin/port`, `cicd/port`, `observability/port` — `var _ Interface = (*Impl)(nil)` 패턴으로 모든 구현체 계약 보장
- **Helm 배포 E2E 테스트**: `e2e/helm_deploy_test.go` (`//go:build e2e`) — kind 클러스터 대상 실제 HelmOrchestrator 구조 검증 + 차트 설치/언설치

#### Frontend 테스트 (27개 신규 파일, 248→305 tests)

- **Observability 모듈** (0%→100%): `monitoring-page`, `alert-rules-page`, `alert-history-page`, `observability-api`, `cluster-stack-filter` 테스트
- **CI/CD 모듈** (14%→86%): `cicd-list-page`, `cicd-template-page`, `cicd-pipeline-setup-page`, `developer-deploy-page`, `cicd-api` 테스트
- **Auth 모듈** (0%→100%): `login-page` 테스트 — 폼 필드, 테스트 계정, 역할별 네비게이션 검증
- **Admin 모듈** (40%→80%): `user-management-page`, `known-issues-page`, `admin-api` 테스트
- **Stack 모듈** (45%→82%): `stack-list-page`, `stack-deploy-page`, `stack-history-page`, `stack-version-page` 테스트
- **Shared components** (23%→60%): `data-table`, `error-boundary`, `protected-route`, `confirm-dialog`, `yaml-editor`, `step-wizard`, `list-detail-panel`, `native-select` 테스트

#### 인프라

- `scripts/check-coverage.sh`: Go 테스트 커버리지 임계값(60%) 검증 스크립트
- `web/vite.config.ts`: Vitest coverage thresholds 추가 (statements 60%, branches 50%, functions 55%, lines 60%)

### Changed

#### F4 InstallStack simulation fallback 명시화

**변경 이유:**
`install_stack.go`의 `executeStep()`이 executor nil 시 `time.After()`로 시뮬레이션하지만, 로그에 아무 표시가 없어 실제 Helm 배포와 구분 불가.

**변경 내용:**
- executor nil 시 `slog.Warn("step executor is nil; running simulated install step", ...)` 경고 출력
- 기존 graceful fallback 동작은 유지 (개발 환경 호환)
- `cmd/api/main.go`에 이미 `WithExecutorFactory(HelmOrchestrator)` 와이어링이 존재함을 확인 — kubeconfig 제공 환경에서는 실제 Helm 실행

### Fixed

- `delete_stack_test.go`: brittle assertion 안정화 (환경 의존 비교 제거)
- `cicd-api.test.ts`: `vi.mock` 호이스팅 오류 수정 (외부 const 참조 → 인라인 객체)
- `cicd-list-page.test.tsx`, `alert-history-page.test.tsx`: `getByText` → `getAllByText` 변경 (중복 DOM 노드 이슈)

### Added

#### Backend — Stack Install Engine 고도화

- **Helm Orchestrator 전면 개편**: 다중 Phase DAG 실행, 실제 Helm install/upgrade/rollback 로직 구현 (`orchestrator.go` +1,100 lines)
- **Helm Values Generator**: Stack config → Helm values.yaml 자동 생성기 (`values.go`, OSS별 이미지 태그·차트 버전 분리 적용)
- **Stack Monitoring Handler**: 스택 수준 모니터링 API 엔드포인트 신규 추가 (`monitoring_handler.go` +605 lines)
- **OSS Resource Defaults 도메인 전체 구현**: Domain Entity(`resource_default.go`), Port(`resource_default.go`), Repository(Postgres/Memory), UseCase(`manage_resource_defaults.go`) — Clean Architecture 레이어 완비
- **Stack Create UseCase**: 스택 생성 전용 유스케이스 분리 (`create_stack.go`)
- **Stack Delete UseCase 대폭 확장**: Helm uninstall 연동, 네임스페이스 정리, PVC 보존 옵션 처리 (`delete_stack.go` +835 lines)
- **Stack Install UseCase 보강**: 설치 전 검증 로직 강화, 리소스 체크 연동

#### Frontend — Stack Install / List 대규모 확장

- **Stack Install 5단계 Wizard 완성** (`stack-install-page.tsx` +3,988 lines):
  - Resource Planning UX (단위 전환 Gi/Mi, 수식 설명, clamp 경고, 총합 연동)
  - Storage Plan 단계 (기존 연결/통합 생성, DB·Object Storage 입력, 연결 정보 검증)
  - YAML View (Helm values.yaml / Kubernetes manifest, 역할 태깅, GitLab 번들 통합)
  - Preview Deploy Script (EOF 기반 values.yaml/manifest 생성 + Helm/Kubectl 적용 스크립트)
  - Dry Run (사전 검증 체크리스트, PASS/WARN/FAIL/READY 요약, Kubernetes Objects 미리보기)
- **Stack List 상세 정보 강화** (`stack-list-page.tsx` +1,984 lines): 커넥션 정보, 상태 상세, 인라인 디테일 패널
- **OSS Resource Default 관리 페이지** 신규 (`stack-oss-resource-default-page.tsx`): Admin 전용 OSS별 리소스 기본값 관리
- **Stack API 클라이언트 확장** (`stack-api.ts` +245 lines): 리소스 기본값 조회/업서트 API 연동
- **Stack Config Store 확장** (`stack-config-store.ts` +241 lines): Install Wizard 상태 관리 고도화

#### Database Migrations (000022–000028)

- `000022_stack_resource_defaults`: `stack_resource_defaults` 테이블 생성 + 12개 기본 OSS 리소스 시드
- `000023_seed_missing_stack_resource_defaults`: 21개 추가 OSS 리소스 기본값 시드 (GitLab, Jenkins, Flux, Tempo, Jaeger 등)
- `000024_align_template_and_compatibility_versions`: Golden Path 템플릿/호환성 매트릭스 버전 정렬 (GitLab 18.5.1/chart 9.5.1, Argo CD v2.8.3/chart 6.8.0)
- `000025_stack_state_cancelled`: `deployment_state` enum에 `cancelled` 상태 추가
- `000026_align_gitlab_argocd_registry`: `gitlab-argocd-v1` 템플릿 registry를 Harbor → GitLab Registry로 변경
- `000027_seed_devsecops_stack_mock`: DevSecOps 목업 스택 3종 시드 (Enterprise/GitOps/Lean)
- `000028`: 기존 `000022_seed_mock_and_extra_demo` 재번호 부여

#### Testing

- Stack API 클라이언트 단위 테스트 (`stack-api.test.ts`)
- Stack Install Page 테스트 확장 (`stack-install-page.test.tsx`)
- Stack List 커넥션 정보 테스트 (`stack-list-connection-info.test.ts`)
- Stack Template Page 테스트 (`stack-template-page.test.tsx`)
- Stack Config Store 테스트 (`stack-config-store.test.ts`)
- Helm Orchestrator/Installer/Values 테스트 (`orchestrator_test.go`, `installer_test.go`, `values_test.go`)
- Stack Create/Delete/Install UseCase 테스트 (`create_stack_test.go`, `delete_stack_test.go`, `install_stack_test.go`)
- Resource Defaults UseCase 테스트 (`manage_resource_defaults_test.go`)
- E2E API 테스트 확장 (`e2e/api_test.go`)
- Memory Streamer 테스트 (`memory_streamer_test.go`)

#### Infrastructure

- `scripts/verify-db-migration.sh`: DB 마이그레이션 무결성 검증 스크립트 추가
- `scripts/runbook_local.sh` 확장: mock 시드 연동, 마이그레이션 자동화 개선
- `scripts/kind-cluster.yaml` 구조 개선 (dual kind 클러스터 설정)

#### 기존 (2026-03-24)

- Mock auth 사용자(`@nullus.dev`) 3명 DB 시드 등록 (login-page.tsx TEST_ACCOUNTS와 ID 동기화)
- 데모 조직 2개 추가 (Acme Corp, Startup Labs), 사용자 5명, 클러스터 3개 시드
- 클러스터 `connection_status` 전체 enum 커버 (connected, pending, unreachable, auth_failed)
- Stack Install OSS 버전 카탈로그 추가 (GitLab app `18.5.1`/chart `9.5.1`, Argo CD app `v2.8.3`/chart `6.8.0` 포함)
- 4개 스택 전체에 config version 시드 추가 (총 11개, 스택별 2~4개)
- 로컬 kind 클러스터(`kind-nullus-test`)를 데모 조직에 기본 등록, runbook에서 엔드포인트 동적 갱신

### Changed

#### Stack Install 최종 배포 검토 흐름 고도화

**변경 이유:**
배포 직전 설정 검토가 분산되어 있어(리소스/스토리지/설치파일/스크립트) 실제 설치 전에 오류를 놓치기 쉬웠습니다.

**변경 내용:**
- 탭 흐름을 `YAML View → Preview Deploy Script → Dry Run` 단계로 확장하여 배포 전 검토를 일원화
- 설치파일 편집 시 유효한 변경만 이전 단계 설정으로 역반영하고, 이전 단계 수정은 설치파일/스크립트에 재반영
- GitLab 계열(`gitlab`, `gitlab-registry`, `gitlab-ci`)은 단일 Helm 번들로 통합해 중복 values.yaml 생성 방지

#### Access Domain 설정 추가

**변경 이유:**
Stack 이름과 실제 접근 도메인 규칙을 사용자에게 명확히 안내할 필요가 있었습니다.

**변경 내용:**
- Stack Name 하단에 `Access domain` 입력란 추가 (기본값: `{StackName}.internal`)
- OSS 접근 가이드 표기: `{OSS}.{StackName}.internal`

#### OSS 버전 명시/동기화 강화

**변경 이유:**
GitLab + Argo CD 템플릿 편집 시 tool 버전(`helm_version`, `app_version`)이 저장되지 않아,
재편집/재생성 시 버전 정보가 유실되는 문제가 있었습니다.

**변경 내용:**
- Stack Template 편집 로직에서 `toolDetails`를 우선 로딩하여 기존 버전 메타데이터를 보존
- 템플릿 업데이트 payload에 `helm_version`/`app_version`이 유지되도록 API 정규화 로직 보강
- Stack Install YAML/Deploy Script 생성 시 앱 버전과 차트 버전을 분리 적용
  - Helm values: `image.tag` = app version, `chart.version` = chart version
  - Deploy Script: `helm --version` = chart version

#### Mock 데이터 제거 (전 페이지)

**변경 이유:**
프론트엔드 페이지들이 API 응답 실패 시 하드코딩된 MOCK 데이터를 fallback으로 사용하고 있었습니다.
이 패턴은 두 가지 문제를 야기합니다:
1. **오류 은폐**: API가 실패해도 화면에 가짜 데이터가 표시되어 문제를 인지하지 못함
2. **개발 혼란**: 실제 API 연동 여부를 UI만으로 판단할 수 없음

권장 전략: API 실패 시 빈 배열(`[]`) 또는 `null` 반환 → 에러 상태를 명시적으로 처리

**변경된 파일:**
- `web/src/features/cicd/pages/cicd-list-page.tsx` — `MOCK_PIPELINES` 제거, fallback `[]`
- `web/src/features/cicd/pages/cicd-template-page.tsx` — `MOCK_CICD_TEMPLATES` 제거, fallback `[]`
- `web/src/features/cicd/pages/cicd-history-page.tsx` — `MOCK_DEPLOYMENTS` 제거, fallback `[]`
- `web/src/features/stack/pages/stack-template-page.tsx` — `MOCK_TEMPLATES` 제거, fallback `[]`
- `web/src/features/stack/pages/stack-history-page.tsx` — `MOCK_STACKS_FOR_HISTORY` 제거, fallback `[]`
- `web/src/features/stack/pages/stack-list-page.tsx` — MOCK 제거, fallback `[]`
- `web/src/features/stack/pages/stack-version-page.tsx` — MOCK 제거, fallback `[]`
- `web/src/features/stack/pages/stack-add-tools-page.tsx` — `MOCK_STACKS` 제거, fallback `null`
- `web/src/features/admin/pages/user-management-page.tsx` — `MOCK_INVITES` 유지 (초대 링크 API 미연동 시 샘플 표시)
- `web/src/features/observability/pages/monitoring-page.tsx` — `MOCK_APPS` 유지 (실제 API 연동 전 시각화 데이터로 사용)

#### 접근성 개선 (stack-template-page.tsx)

**변경 이유:**
HTML 명세상 `<button>` 내부에 `<button>`을 중첩할 수 없음 (interactive content model 위반).
브라우저 콘솔 경고 및 스크린리더 동작 불일치 발생.

**변경 내용:**
- 카드 외곽 `<button>` → `<div role="button" tabIndex={0}>` + `onKeyDown` 핸들러 추가
- 내부 `<Button>` (Use Template)은 유지

#### 마이그레이션 파일 정리

**변경 이유:**
- `000016_sync_org_members` → `000021_sync_org_members`: `phase1`이 이미 `000019_seed_demo_data`, `000020_pipeline_stack_relation`을 사용하므로 번호 충돌 방지
- `000017_fix_healthcheck_enum.up.down.sql` (잘못된 이중 확장자) → `000017_fix_healthcheck_enum.down.sql`로 수정
- `000007_seed_templates.up.sql`: `golden_path_templates` 테이블 CREATE TABLE 구문 추가 (INSERT 전 테이블이 없어 마이그레이션 실패하던 문제 수정)

### Fixed

- Organization 생성 시 DB 미저장 — 프론트엔드 API 경로 `/admin/organizations` → `/admin/orgs` 수정
- Mock auth ORG_ID/User ID가 DB 시드와 불일치하여 멤버 조회 등 org 기반 API 실패하던 문제 수정
- 백엔드 fallback Org ID가 DB에 없는 값(`00000000-...`)이어서 스택/CI/CD 목록 빈 결과 반환 → 시드 org(`11111111-...`)로 수정
- CORS `AllowHeaders`에 `X-Org-ID` 누락 → 프록시 없이도 헤더 전달 가능하도록 추가
- 스택 시드 config JSON이 `StackConfig` 구조체와 불일치하여 unmarshal 실패 → `ToolSelection` 형식으로 수정
- 스택 히스토리 config version도 동일 포맷 불일치 수정 (플랫 문자열 → 중첩 객체)
- 스택 히스토리 diff `versionA`/`versionB` 초기값 `4`, `5` 하드코딩 → `0`으로 변경, 이력 로드 후 최신 2개 버전 자동 선택
- 스택 히스토리 스냅샷 `Object.entries(snapshot)` null 크래시 → `snapshot ?? {}` null-safe 처리
- 클러스터 페이지 `unreachable`/`auth_failed` 상태 매핑 누락으로 크래시 → `ClusterStatus` 타입 및 `STATUS_CONFIG` 추가
- `/organizations/:orgId/invites` 엔드포인트 미구현 404 → stub 핸들러 추가
- 개발 모드에서 rate limiter(30/min)로 smoke test 및 프론트엔드 요청 차단 → 프로덕션에서만 적용
- `web/src/__tests__/uat-devops-scenario.test.tsx`: `useTemplates` mock을 `data: MOCK_TEMPLATES`로 변경하여 MOCK 제거 후에도 UAT 시나리오 테스트 통과

### Merged

- Phase1 (#15) — Stack Install Engine 고도화, Helm Orchestrator 전면 개편, Resource Defaults 도메인 구현, 테스트 확충
- Phase1 (#13) — Stack List 상세 확장, DB 마이그레이션 000022–000028, 검증 스크립트 추가
- Phase1 (#10) — Mock fallback 제거, 접근성 개선, 마이그레이션 정리, 포트 설정 통일

---

## [0.1.0-alpha] - 2026-03-15

### Core Features (F0-F10)
- Organization 설정 등록 (F0) — Postgres Repository, CRUD API
- K8S Cluster 등록/검증 (F1) — client-go 검증 어댑터, Kubeconfig AES-256 암호화
- DevSecOps Stack 설정 UI (F2) — React 5단계 Wizard, RHF + Zod 검증
- Golden Path 템플릿 (F3) — 3개 템플릿, Postgres Repository, Seed 데이터
- Stack 자동 설치/배포/이력 (F4) — Helm SDK 3-Phase DAG, WebSocket 로그 스트리밍, Rollback
- CI/CD Pipeline 템플릿 (F5) — Pipeline 템플릿, K8s Manifest Generator
- CI/CD Pipeline 배포/이력 (F6) — client-go Dynamic Applier, 배포 추적
- 모니터링/알림 (F7) — Prometheus HTTP Client, Dashboard, Alert CRUD
- 버전 호환성 관리 (F8) — 호환성 매트릭스, 검증 API, JSONB Diff
- UI 권한 체계 (F9) — Keycloak OIDC JWT, dual-mode 인증, 라우트별 RBAC
- 리소스 예상량 계산 (F10) — 리소스 계산기, 비용 추정

### Backend
- Clean Architecture + DDD 모듈 구조 (5 Bounded Context)
- Helm SDK (helm.sh/helm/v3) 기반 설치 엔진, StepExecutor 인터페이스
- PostgreSQL 6종 레포지토리 마이그레이션 + 시드 데이터
- Keycloak SSO 자동 프로비저닝 (GitLab, Grafana, ArgoCD, MinIO)
- Rate Limiting 미들웨어 (sliding window, 4 categories)
- Audit Logging (DB 기반) + 알림 시스템 (Slack/Email)

### Frontend
- React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- 15개 페이지 (Admin, Stack, CI/CD, Observability, Developer Self-Service)
- TanStack Query API 연동, React Hook Form + Zod 검증
- Recharts 4종 차트 (CPU, Memory, Pipeline, Pod Status)
- 다국어 지원 (i18n), 역할별 UI 분기, Toast 알림 (sonner)
- Auth 세션 영속성 (sessionStorage), Route Guard, Error Boundary

### Infrastructure
- GitHub Actions CI (Go test + Vite build + Vitest + Playwright E2E)
- Docker 멀티스테이지 빌드 + Helm 차트 (9 templates)
- testcontainers-go 통합 테스트 인프라
- 로컬 개발 환경 스크립트 (runbook_local.sh)

### Fixed
- 프론트엔드-백엔드 호환성: 템플릿 tools[] 매핑, cluster status 필드 통일
- PostgresOrgRepository NULL default_admin_id 스캔 수정
- estimated_install_time 나노초→분 변환 오버플로우 수정

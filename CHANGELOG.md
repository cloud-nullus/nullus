# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `runbook_local.sh`에 `refresh` 커맨드 추가 — 마이그레이션 포함 백엔드 + 프론트엔드 재빌드/재시작
- Home CTA 권한 상태와 Roadmap 연동 개선 — 로그인 사용자의 역할/권한 기반 CTA 동적 표시 및 Roadmap 페이지 연동
- 풀 CI/CD 빌드 파이프라인: Git Clone → Docker Build → Kind Load → K8s Deploy 6단계 자동화
- `ImagePreparer` Port + `docker/builder.go` 어댑터: git clone, docker build, kind load docker-image 실행
- `ClusterTargetProvider` Port: 클러스터 이름 + kubeconfig 통합 조회 (Kind 클러스터명 자동 추출)
- Pipeline에 `dockerfile_path`, `docker_context` 필드 추가 — Dockerfile 경로와 빌드 컨텍스트 지정
- Pipeline에 `env_vars` 필드 추가 — 환경변수를 K8s Deployment 매니페스트 container spec에 반영
- CI/CD 템플릿에 빌드 설정 지원: `git_repo_url`, `dockerfile_path`, `docker_context`, `env_vars` 필드 추가
- Nullus Sample App 배포 템플릿 2종 추가 (`nullus-sample-backend-v1`, `nullus-sample-frontend-v1`)
- Deploy 위저드 Step 2에 "Build Configuration (Optional)" 섹션: Dockerfile Path, Docker Build Context 입력
- Deploy 위저드 상단에 "Quick Start — Select a Template" UI: 템플릿 클릭 시 폼 자동 채움 + Step 3으로 점프
- 환경변수(Step 5)가 파이프라인 생성 시 저장되어 배포 시 K8s 매니페스트에 반영
- 템플릿 선택 시 기본 환경변수 자동 상속 (예: 프론트엔드 템플릿의 `BACKEND_HOST=sample-backend:8080`)
- `scripts/register-kind-clusters.sh`: nullus-platform, nullus-develop Kind 클러스터 자동 등록 스크립트
- Stack ↔ CI/CD 교차 컨텍스트 검증: StackReader Port 인터페이스를 통해 CI/CD 모듈이 Stack 도메인을 직접 import하지 않고 Stack 존재/조직 일치/상태를 검증합니다 (Direction B)
- `POST /cicd/pipelines` 요청 시 `stack_id`를 지정하면 Stack 존재 여부, 조직 일치, 배포 상태를 자동 검증합니다
- `GET /stacks/:stackId/pipelines` 엔드포인트 추가 (Stack 기준 Pipeline 조회)
- `GET /cicd/pipelines?stack_id=xxx` Stack 필터 지원
- Stack 배포 로그 DB 영속화(`deployment_logs`) 및 `PostgresStreamer`를 추가해 API 재시작/재구독 이후에도 로그 replay가 가능해졌습니다.
- Stack History에 Cluster 컬럼/클러스터 이름 필터/Log 바로가기 버튼을 추가해 최근 배포 로그 접근성을 개선했습니다.
- Stack Version 검증 응답에 `overall`/`issues`/`checkedAt`를 포함해 pass/warn/fail 기반 호환성 피드백을 제공하도록 확장했습니다.
- v0.1 아키텍처 설계와 실제 구현 코드를 비교 분석한 v0.2 아키텍처 문서 추가 (`docs/20_아키텍처/Nullus 상세 기능 명세 및 시스템 아키텍처_v0.2_claude.md`)
  - 설계-구현 차이 분석표 (아키텍처 변경, 기능 상태, 미구현 항목)
  - Clean Architecture + DDD 기반 실제 코드 구조 문서화
  - 5개 Bounded Context별 도메인 모델, API, 상태 머신 상세
  - 3-Phase Helm DAG Orchestrator 구현 기준 명세
  - 전체 API 엔드포인트 목록 (v0.1 대비 경로 변경 추적)
  - 데이터 모델 ERD (구현 기준 15+ 테이블)
  - 보안 아키텍처 (AES-256-GCM, Dual Auth, RBAC) 상세
  - ADR 16건 (v0.1 10건 + 신규 6건)
  - 로드맵 (v0.2-alpha → v0.2-beta → v1.0 GA)
- Nullus 설계 대비 미구현 항목만 정리한 문서 추가 (`docs/20_아키텍처/Nullus_설계_대비_미구현_항목.md`)
- 현재 `draft` 구현 기준 As-Is 아키텍처 다이어그램 문서 추가 (`docs/20_아키텍처/Nullus_As-Is_아키텍처_다이어그램.md`)
- 기존 v0.1 설계 문서를 현재 구현 기준으로 재구성한 `Nullus 상세 기능 명세 및 시스템 아키텍처_v0.2.md` 추가
- Alert Rules edit modal now loads the latest rule payload directly from the database through `GET /observability/alert-rules/:id` before editing.
- Stack Install supports leaving Storage unselected for Empty Template flows by omitting the storage block from create requests when no storage plan is chosen.
- Alert Rules edit modal now loads the latest rule payload directly from the database through `GET /observability/alert-rules/:id` before editing.
- Stack Install supports leaving Storage unselected for Empty Template flows by omitting the storage block from create requests when no storage plan is chosen.
- CI/CD List에 클러스터 필터 드롭다운 추가 (`useClusters` 훅 연동)
- Pipeline Logs 전용 페이지 신규 생성 (`/cicd/pipelines/:id/logs`, 터미널 콘솔 + 배포 이력 뷰)
- CI/CD 배포 진행 UI를 WebSocket 기반 실시간 스트리밍으로 전환 (Stack Deploy 페이지 스타일)
- WebSocket 핸들러 `/ws/cicd/deployments/:id/logs` 추가 (`StepTracker` pub/sub 패턴)
- `useCicdDeployLog` 프론트엔드 훅 (WebSocket 연결, 로그/진행률/상태 관리)
- Deploy 위저드 Step 6 매니페스트 편집 단계 추가 (textarea로 YAML 수정 가능, 기본값 초기화)
- Deploy 위저드 Step 2에 Stack Git 서비스 URL 자동 연동 (Stack 선택 시 base URL + repo 이름 분리 입력)
- Deploy 위저드 Step 3 네임스페이스를 K8s API에서 실제 조회 (`useClusterNamespaces` 훅)
- Deploy 위저드 Step 4 리소스 설정에 슬라이더 + Input 동시 지원 (커스텀 값 직접 입력 가능)
- RUN 버튼이 pipeline 정보(clusterId, namespace, appName)를 Deploy 위저드에 프리필
- 모든 CI/CD 페이지에 Breadcrumb 상위 네비게이션 추가 (뒤로가기 지원)
- CI/CD 파이프라인이 kind 클러스터에 실제 K8s 리소스(Deployment, Service, Namespace)를 생성
- 배포 진행 화면에 Deploy Output 터미널 박스 (kubectl 명령어 및 결과 실시간 표시, 색상 구분)
- 배포 완료 시 생성된 K8s 리소스 목록 표시 및 `kubectl get` 확인 명령어 복사 기능 (`--context` 포함)
- `DeployStep`에 `Logs` 필드 추가, `StepTracker.AppendLog`로 스텝별 kubectl 로그 축적
- `StepTracker`에 `Subscribe`/`Unsubscribe`/`publish` 메서드 추가 (WebSocket 실시간 이벤트 전파)
- 인메모리 `StepTracker`로 배포 단계별 진행 상태 추적 (30초 후 자동 정리)
- GET `/cicd/deployments/:id` 엔드포인트 (배포 상태 + 스텝 로그 병합)
- CI/CD List 상세 패널 4개 탭: Info (Pipeline + Target + Stages + Variables), Monitoring, History, Actions
- `DataTable`에 `renderExpanded` prop 추가 (행 아래 인라인 상세 패널)
- CI/CD History 페이지에서 특정 파이프라인 배포 이력만 필터링 (`?pipeline=<id>`)
- 배포 시 현재 로그인 사용자가 `deployed_by`로 자동 기록
- Helm 차트 ServiceAccount 템플릿 추가
- API Deployment에 wait-for-db initContainer 추가 (PostgreSQL 준비 대기)
- CI/CD kind 클러스터 배포 시연 가이드 (`docs/guides/cicd-pipeline-kind-deploy-guide.md`)
- 시행착오 및 해결 방법 레퍼런스 (`docs/agent-reference.md`)
- Pipeline 타입에 `dockerfilePath`, `dockerContext`, `envVars` 필드 추가 — 프론트엔드에서 백엔드 빌드 설정 데이터를 표시
- CI/CD List 상세 Info 탭에 "Build Configuration" 카드 추가 (Dockerfile, Build Context 표시)
- CI/CD List 상세 Info 탭에 실제 환경변수 표시 (하드코딩 3개 → 백엔드 `env_vars` 기반, 마스킹 토글)
- CI/CD List 상세 Monitoring/History 탭에 로딩 상태 표시 추가

### Changed

- 로그인 후 기본 진입 경로를 Home으로 통일 (모든 역할에서 로그인 완료 시 Home 페이지로 리다이렉트)
- 매니페스트 생성기: `ImageRef` 필드 추가 — 설정 시 템플릿 하드코딩 이미지 대신 빌드된 이미지 사용
- `ManifestApplier.ApplyWithTracking`에 `stepOffset` variadic 파라미터 추가 (빌드 단계 이후 인덱스 보정)
- `NewDeployPipeline`에 옵셔널 DI 패턴 도입 (`WithImagePreparer`, `WithClusterTargetProvider`)
- StepTracker 클린업 타이머 30초 → 5분 (빌드 시간 고려)
- Playwright E2E 테스트: Korean 셀렉터 → English 셀렉터 전환 (기본 언어 en)
- Rollback 테스트 제거 (CI/CD History에서 Rollback 기능 제거됨)
- Stack 템플릿 카운트 3 → 4 (github-argocd-v1 추가 반영)
- `POST /cicd/pipelines` 응답 포맷을 `{"pipeline": {...}, "warning": "..."}` 구조로 변경 (warning은 Stack 미완료 시 optional 포함)
- `PipelineRepository.List` 시그니처에 `stackID ...string` variadic 파라미터 추가
- Stack History 라우팅 동작을 조정해 설치 직후 목록 캐시가 늦게 갱신되더라도 URL의 `stackId`를 우선 유지하도록 변경했습니다.
- Stack Deploy 화면의 상태 계산 로직을 개선해 WS 연결 여부만으로 `running`으로 오인하지 않고 API의 최종 상태를 우선 반영하도록 변경했습니다.
- 템플릿 생성/수정 화면의 OSS 분류를 스택 설치 분류 체계와 일치시키고 모달 폭/ID 입력 UX를 개선했습니다.
- Stack create request mapping now translates UI storage modes (`existing-all`, `existing`) to the backend storage contract (`existing-connect`) before submission.
- Deploy 위저드를 5단계에서 6단계로 재구성 (앱 이름 → Git → 클러스터 → 리소스 → 환경변수 → 매니페스트 확인)
- 앱 템플릿 그리드 제거, CI/CD Template의 `app_type`으로 앱 타입 자동 결정
- CI/CD 배포 진행 UI를 polling 방식에서 WebSocket 실시간 스트리밍으로 전환
- CI/CD List Logs 버튼이 Pipeline Logs 전용 페이지(`/cicd/pipelines/:id/logs`)로 이동
- `DeployPipeline` usecase를 `Start`(동기, DB 저장) + `ApplyAsync`(비동기, K8s 배포)로 분리, HTTP 202 즉시 반환
- `ManifestApplier.ApplyWithTracking`이 각 매니페스트 적용 결과와 로그를 `StepTracker`에 기록
- CI/CD List/History 페이지가 각 항목 아래 인라인 상세 패널로 변경 (하단 패널 → 행 아래 인라인)
- CI/CD History에서 Rollback 기능 전체 제거 (백엔드 미구현)
- CI/CD List/History 페이지가 백엔드 API 응답을 정확히 매핑 (앱 타입, 클러스터명, 상태, 배포일)
- CI/CD List 테이블에서 Deploy 버튼 제거 (상세 패널의 Run으로 통합)
- `go-web-api` 템플릿 이미지를 빌드 이미지에서 런타임 서버로 변경 (`nginx:alpine`)
- Migration Job을 pre-install Hook에서 외부 마이그레이션 패턴으로 전환
- Cluster/CI/CD 모니터링 뷰의 목업 데이터를 실제 API 데이터로 교체 (`useDashboard()` 폴링 축적, `usePipelines()`+`useDeployments()` 연동)
- `useScopedClusters()` 훅을 `admin-api.ts`로 통합하고 `stack-api.ts` 중복 정의 제거
- CI/CD List 상세 History 탭 배포 소요 시간 포맷 개선 (`42s` → `1m 42s`)

### Fixed

- Use Base Template 진입 시 선택하지 않은 리소스가 자동 선택되던 문제를 수정해 실제 템플릿 선택 상태가 그대로 반영되도록 했습니다.
- Stack List 상태/클러스터 표시 정합성을 수정해 `connected + completed` 케이스가 `Running`으로 노출되고 모니터링 탭 조건이 일관되게 동작하도록 보정했습니다.
- Stack Compatibility 검증 시 스택이 실제 배포된 클러스터의 Kubernetes 버전을 기준으로 평가되도록 수정했습니다.
- Alert Rules edits now reflect immediately after Save by awaiting the update mutation, refetching the DB-backed list, and reopening the modal with fresh server data.
- Empty Template에서 Observability만 선택해도 `storage.plan_mode` 검증 오류가 나지 않도록 storage payload 생성 조건을 수정했습니다.
- Organization 화면의 `Add User` 버튼이 존재하지 않는 `/admin/user-management` 대신 실제 라우트인 `/admin/users`로 이동하도록 수정했습니다.
- Stack gateway deploy now skips `BackendTLSPolicy` manifests when the cluster does not provide the `BackendTLSPolicy` CRD, so Gateway and HTTPRoute resources can still be applied.
- Breadcrumb에서 동일한 key `/cicd/list`가 2회 사용되어 React 경고 발생하던 문제
- Dockerfile Go 버전이 `go.mod`와 불일치 (`1.24` → `1.26`)
- `web/Dockerfile` 빌드 컨텍스트 경로 오류 (`web/nginx.conf` → `nginx.conf`)
- `web/Dockerfile` npm ci peer dependency 충돌 (`--legacy-peer-deps` 추가)
- API Deployment에 ConfigMap 볼륨 마운트 누락으로 config 파일을 찾지 못하던 문제
- `getPipelineStatusLabel`의 커스텀 `Translate` 타입을 i18next `TFunction`으로 변경하여 CI/CD 3개 페이지 9건의 TypeScript 에러 해소
- Stack 페이지에서 클러스터 `connection_status` 필드 접근 오류 (`Cluster` 타입 통합 후 `status`로 수정)
- 클러스터 API 매핑과 스택 템플릿/설치 동작 정합성 수정 (클러스터 필드 매핑, 스택 템플릿 카운트, 설치 페이지 선택 동작 보정)
- 관리 화면 유효성 검증 흐름과 다국어 표시 개선 (Cluster/CI-CD/Stack/OSS 리소스 페이지 ko/en 번역 보완)
- 스택 템플릿 설명 로케일 오버라이드 보강 — 템플릿별 언어 설명 오버라이드 로직 추가
- CI/CD 템플릿 설명 다국어 처리 및 우선순위 정렬 개선
- 클러스터 등록/수정 검증 흐름 통합 — 백엔드 핸들러와 프론트엔드 클러스터 페이지의 검증 로직 단일화
- Register Cluster 다이얼로그의 클러스터 타입 옵션 순서 조정
- Stack List 삭제 후 목록 즉시 반영 및 상세 패널 중복 표시 수정

## [0.2.0-alpha] - 2026-03-28

### Added

- Stack Install 5단계 Wizard 완성 (Resource Planning, Storage Plan, YAML View, Deploy Script, Dry Run)
- Stack List 상세 패널 (커넥션 정보, 상태, 인라인 디테일)
- Helm Orchestrator 다중 Phase DAG 실행 및 실제 Helm install/upgrade/rollback
- Helm Values Generator (Stack config → values.yaml 자동 생성)
- Stack Monitoring API 엔드포인트
- OSS Resource Defaults 도메인 전체 구현 (Entity, Port, Repository, UseCase)
- Stack Create/Delete UseCase 분리 및 확장
- OSS Resource Default 관리 페이지 (Admin 전용)
- Go 테스트 커버리지 검증 스크립트 (`scripts/check-coverage.sh`)
- DB 마이그레이션 000022-000028 (리소스 기본값, 템플릿 버전 정렬, 상태 enum 확장, 목업 스택)

### Changed

- Stack Install 최종 배포 검토 흐름을 YAML View → Deploy Script → Dry Run 단계로 확장
- Stack Name 하단에 Access Domain 입력란 추가 (`{StackName}.internal`)
- OSS 버전 메타데이터가 템플릿 편집 시에도 보존되도록 API 정규화
- GitLab 계열 OSS를 단일 Helm 번들로 통합
- Mock 데이터 fallback을 전 페이지에서 제거 (API 실패 시 빈 배열 반환)
- Stack Template 카드의 접근성 개선 (중첩 button → `div[role="button"]`)
- 마이그레이션 파일 번호 정리 및 중복 방지

### Fixed

- Organization 생성 시 DB 미저장 (API 경로 `/admin/organizations` → `/admin/orgs`)
- Mock auth ORG_ID/User ID가 DB 시드와 불일치하여 org 기반 API 실패
- CORS `AllowHeaders`에 `X-Org-ID` 누락
- 스택 시드 config JSON이 `StackConfig` 구조체와 불일치하여 unmarshal 실패
- 스택 히스토리 diff 초기값 하드코딩 → 최신 2개 버전 자동 선택
- 스택 히스토리 스냅샷 null 크래시 → null-safe 처리
- 클러스터 `unreachable`/`auth_failed` 상태 매핑 누락
- 개발 모드에서 rate limiter가 프론트엔드 요청 차단 → 프로덕션 전용으로 변경

## [0.1.0-alpha] - 2026-03-15

### Added

- Organization 설정 등록 — PostgreSQL Repository, CRUD API
- K8s Cluster 등록/검증 — client-go 검증 어댑터, Kubeconfig AES-256-GCM 암호화
- DevSecOps Stack 설정 5단계 Wizard — React Hook Form + Zod 검증
- Golden Path 템플릿 3종 — PostgreSQL Repository, Seed 데이터
- Stack 자동 설치/배포/이력 — Helm SDK 3-Phase DAG, WebSocket 로그 스트리밍, Rollback
- CI/CD Pipeline 템플릿 — Pipeline 템플릿, K8s Manifest Generator
- CI/CD Pipeline 배포/이력 — client-go Dynamic Applier, 배포 추적
- 모니터링/알림 — Prometheus HTTP Client, Dashboard, Alert CRUD
- 버전 호환성 관리 — 호환성 매트릭스, 검증 API, JSONB Diff
- UI 권한 체계 — Keycloak OIDC JWT, dual-mode 인증, 라우트별 RBAC
- 리소스 예상량 계산 — 리소스 계산기, 비용 추정
- React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui 프론트엔드 (15개 페이지)
- Go 1.26 + Echo v4 + PostgreSQL 백엔드 (Clean Architecture + DDD, 5 Bounded Context)
- GitHub Actions CI (Go test + Vite build + Vitest + Playwright E2E)
- Docker 멀티스테이지 빌드 + Helm 차트
- testcontainers-go 통합 테스트 인프라
- 로컬 개발 환경 스크립트 (`runbook_local.sh`)

### Fixed

- 프론트엔드-백엔드 호환성: 템플릿 tools 매핑, cluster status 필드 통일
- PostgresOrgRepository NULL default_admin_id 스캔 오류
- estimated_install_time 나노초→분 변환 오버플로우

[unreleased]: https://github.com/cloud-nullus/draft/compare/v0.2.0-alpha...HEAD
[0.2.0-alpha]: https://github.com/cloud-nullus/draft/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/cloud-nullus/draft/releases/tag/v0.1.0-alpha

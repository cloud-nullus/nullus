# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Stack Continue 배포** (`POST /api/v1/stacks/:id/continue`): 실패한 스택 배포를 rollback 없이 재개. 이미 설치된 Helm 릴리즈를 보존하고 실패 지점부터 재시작. `InstallStackInput`에 `Continue`/`PreserveLogs` 필드 추가, 실패 시 UI에 Continue 버튼 노출.
- **Pod Watch WebSocket** (`GET /ws/deployments/:id/pods`): kubectl get pods -n <namespace> -w 출력을 WebSocket으로 실시간 스트리밍. 배포 로그 페이지에 Pod Watch 패널 추가 (네임스페이스, Ready, Status, Restarts, Age 표시).
- **Org Resource Profile 저장** (`/api/v1/admin/org-resource-profiles`): 조직 단위 리소스 프로파일 CRUD. Stack Install Wizard Sizing 탭에서 프로파일 저장·불러오기 드롭다운 지원. DB 마이그레이션 `000049_org_resource_profiles`, `000050_allow_local_resource_profile`.
- **`Orchestrator.IsStepEnabled` 공개 메서드**: `stepEnabledChecker` 인터페이스를 통해 usecase 레이어에서 각 설치 단계 활성화 여부를 조회 가능.

### Changed

- **배포 로그 페이지 UI 개선**: 타임라인 스텝, 세그먼트 프로그레스 바, Raw Logs 콘솔, Attention 패널(warn/error 필터), Pod Watch 패널로 구성한 새 레이아웃. WS 연결 전 "Connecting..." / 연결 후 파드 없음 "No pods in namespace yet." 으로 상태 구분.
- **Status API `namespace` 필드**: `omitempty` 제거 — 스택 네임스페이스가 빈 값이어도 항상 필드 포함해 반환.
- **`podNamespace` 폴백 처리**: `??` → `||` 변경으로 빈 문자열까지 폴백 처리.
- **Stack Install 페이지**: Quick Start 카드 및 Kubernetes Preview 섹션 제거.

### Fixed

- **Sizing Profile 드롭다운 즉시 반영**: 프로파일 저장 후 드롭다운에 즉시 반영되지 않는 버그 수정 (캐시 invalidation 누락).
- **`usePodWatch` 재연결 시 에러 초기화**: WS 재연결 성공 시 이전 연결의 stale 에러 메시지가 남는 문제 수정.

### Added (기존)

- OpenBao 선택형 배포 경로 구현: `authentication.provider=openbao` 선택 시 `installing_openbao` 단계에서 OpenBao(공식 이미지) Deployment/Service를 생성하고 Gateway 기본 번들에 `openbao.<access_domain>` 라우트를 자동 추가합니다.
- Secret Manager 추상화 계층 추가: `internal/shared/secrets`에 provider 라우터(`Router`)와 OpenBao 구현체(`OpenBaoStore`)를 도입해, 토큰 저장/조회를 provider별 어댑터로 분리했습니다.
- Token source OpenBao 실연동: stack token source 등록 시 `metadata.secret_manager`를 저장하고, Admin `POST /api/v1/admin/token-sources/:id/reveal`가 OpenBao 실조회 값을 우선 반환하도록 확장했습니다.
- 로컬 실행 기본값 보강: `runbook_local.sh`가 `OPENBAO_ADDR`/`OPENBAO_TOKEN` 기본값을 export 하여 로컬에서 OpenBao read/write 경로를 즉시 검증할 수 있도록 개선했습니다.

- Phase A (F8-Phase5 재개) — 매트릭스 CRUD UI + 백엔드: **backend** — `port.CompatibilityRepository` 에 `Create/Update/Delete` 3 메서드 + `ErrCompatibilityMatrixNotFound`/`ErrCompatibilityMatrixExists` sentinel. Memory + Postgres 리포지토리 구현 (멱등 Delete, Create-on-conflict-404, Update updated_at touch). `ManageCompatibility` usecase + `validateMatrixPayload` (id regex / status enum / semver / tools arr ≤32 / tier/arch 화이트리스트). `CompatibilityHandler` 가 `WithManageCompatibility` + `WithCompatibilityAuditSink` 옵션 + `RegisterAdminRoutes` 로 `POST/PUT/DELETE /admin/compatibility/matrices[/:id]` 노출. `mapCompatibilityError` 가 sentinel → HTTP 400/404/409 매핑. 성공 시 audit `compatibility_matrix_{create,update,delete}` 기록 + verdict 캐시 `Clear()`. **frontend** — `MatrixInput` / `matrixInputToPayload` 타입 + `useCreate/Update/DeleteMatrix` 훅. `MatrixEditModal` (identity + k8s 범위 + 동적 tools rows, arch 체크박스 + tier select) + `ConfirmDialog` 재사용 삭제 확인. `stack-versions-page` 헤더에 "New matrix" 버튼 + 상세 패널에 Edit/Delete 버튼. i18n `stackVersionsAdmin.{actions,modal,deleteConfirm}.*` ko/en. **tests** — repo CRUD 6 케이스, usecase 8 케이스 (validation + cache clear), handler 6 케이스 (201/409/400/400-id-mismatch/404/200/204).
- Phase B (F8-Phase3 follow-up) — Retry UI 버튼: 신규 `stack/utils/retry-policy.ts` `canRetry(status)` 순수 헬퍼 (failed/rolled_back만 허용). 신규 `stack/utils/deploy-error.ts` 로 `extractDeployCompatError` 추출 (Install Wizard 와 Retry UI 양쪽 재사용). 신규 `RetryStackButton` 컴포넌트 — `canRetry` 로 자기검열 렌더, `DEPLOY_COMPAT_WARN_UNACK` 응답 시 Modal 이 ack 체크박스 표시, ack 후 재시도 시 `acknowledgeWarnings=true` 전달. `stack-list-page` Info 탭 action row 에 Delete 앞에 배치. i18n `stackList.retry.{button,confirmWarn,toasts}.*` ko/en. `retry-policy.test.ts` 가 10 enum 전체 truthy 매트릭스 검증.
- F8 follow-up 일괄 (Phase 1~7, Phase 5 drop): **Phase 1** — `web/e2e/stack-workflow.spec.ts` / `stack-monitoring.spec.ts` `beforeEach` 의 `waitForURL('**/stack/templates')` 를 `'**/'` 로 보정 (login-page 가 Home 으로 리다이렉트). `@stack-critical` 3/3 그린(새 spec 1 pass + 기존 2 는 내부 data precondition skip). **Phase 2** — `internal/shared/audit/sink.go` + `memsink.go` 도입, `*AuditLogger` / `*MemorySink` 모두 `Sink` 인터페이스 만족. DeployHandler / StackHandler / ClusterHandler / MemberHandler / OrgHandler 5곳의 필드와 variadic 파라미터 타입을 `audit.Sink` 로 narrowing. `TestDeployHandler_Gate_AuditRecordsAckAndVerdict` / `NoAuditOnBlockedWarn` 2 신규 케이스로 `acknowledge_warnings` / `compatibility_verdict` / `issue_codes` 기록 검증. **Phase 3 (F8-Retry-API)** — `POST /api/v1/stacks/:id/retry` + DeployHandler 에 `runPreDeployGate` 공통 헬퍼 추출해 Deploy/Retry 공유. `rolled_back`/`failed` → pending rewind + InstallStack 재실행. `TestRetry_*` 5 케이스 (verified pass / rolled_back pass / completed 409 / warn unack 400 / warn ack 202). 프론트 `retryStack` + `useRetryStack` 훅 추가 (UI 버튼은 follow-up). **Phase 4 (orphan stack / updateStack)** — `UpdateStack` usecase + `PUT /stacks/:id` 엔드포인트. `{pending, failed}` state 에서만 허용(409 STACK_UPDATE_INVALID_STATE), 성공 시 prior config 를 history 에 스냅샷. `WithOptions(WithUpdateStack)` 로 StackHandler 에 주입. 4 단위 테스트. **Phase 5 (Matrix CRUD UI)** — **DROP**. 백엔드 3 엔드포인트 + modal 컴포넌트 스코프가 세션 budget 초과로 별도 작업 분리 (본 프롬프트 §0 "Phase 중간 큰 회귀 시 drop 허용" 규칙 적용). **Phase 6** — `stack/usecase/verdict_cache.go`: `VerdictCache` 인터페이스 + `MemoryVerdictCache` (sync.Map + TTL + prefix invalidation). `WithVerdictCache` 옵션으로 `ValidateCompatibility` 에 주입. `VerdictCacheKey` 가 StackID/ClusterID/NodeArchitectures/Tools 를 정렬된 SHA256 으로 절충 — map 순서 독립. 5 단위 테스트. `VERDICT_CACHE_TTL_SEC` env override. **Phase 7** — `admin/scheduler/refresh_discovery.go`: `RefreshDiscoveryScheduler` 가 interval 마다 모든 cluster 의 `RefreshDiscovery` 를 sweep. `atomic.Bool` in-flight guard 로 overlap 방지, context cancel 시 graceful stop. main.go 에서 signal handler 가 schedulerCancel() 호출. 4 fake-clock/goroutine 테스트 (first sweep / failure isolation / ctx cancel / overlap skip). `REFRESH_DISCOVERY_INTERVAL` env override (기본 24h).
- Warn-forced Retry/Rollback 통합 E2E 검증 (F8 Task 7): `e2e/warn_forced_retry_rollback_test.go` (`//go:build e2e`) 신규 — 독립 `httptest` 서버 + in-memory 리포지토리 + `fakeStepExecutor` (atomic.Bool 로 성공/실패 주입) + `warnClusterReader` (arm64-only 시뮬레이션) 로 6종 subtest 실행. **A** persisted-mode validate 가 `warn` + `TOOL_ARCH_UNSUPPORTED` 반환. **B** ack 없는 deploy 는 400 `DEPLOY_COMPAT_WARN_UNACK` 로 차단되고 스택 state 는 `pending` 유지. **C** `acknowledge_warnings=true` + 정상 executor → 202 + `completed` 도달. **D** ack=true + executor 실패 → 202 수락 후 handleFailure 경로를 타고 `rolled_back` 으로 종료. **E** 실패 후 `rolled_back → pending` 테스트 레벨 rewind + executor 복구 → 재배포 → `completed` (state-machine 계약 검증; 프로덕션 `POST /stacks/:id/retry` 엔드포인트는 plan §6 follow-up 으로 분리). **F** rollback 엔드포인트 (`POST /stacks/:id/rollback`) 가 prior 버전 config 를 복원하고 새 history row 를 append 함. Playwright `web/e2e/stack-warn-forced-retry.spec.ts` (`@stack-critical`) UI 스모크도 추가 — `/admin/stack-versions` 페이지가 warn-prone 매트릭스(`github-argocd-v1`, untested)를 노출하고, `/stack/install` 의 Golden Path Quick Start 카드가 동일 매트릭스를 렌더하는지 계약 검증. 실제 deploy→terminal-state UI 구동은 Kind 클러스터 의존성 때문에 F8 Task 6 / F8-F6-Cloud 에 위임.
- Golden Path 로컬 Kind 배포 검증 스캐폴딩 (F8 Task 6): `e2e/golden_path_kind_test.go` (`//go:build e2e`) 신규 추가. 단일 테스트 `TestF8Task6_GoldenPath_KindDeploy` 가 `discoverKindCluster` 로 `nullus-platform` 클러스터를 발견하면 Narwhal pin 기반 3 종 Golden Path 매트릭스 (`github-argocd-v1`, `gitlab-argocd-v1`, `gitlab-allinone-v1`) 를 순차 subtest 로 실행 — 각각 in-memory `StackRepository` / `TemplateRepository` + 실 `helm.Orchestrator` + `InstallStack.Execute` 경로로 배포 → 상태 폴링 → `completed` 도달 여부 검증. Kind 또는 helm CLI 미설치 시 graceful skip. 테스트별 namespace (`nullus-e2e-<template>-<ts>`), `toolOverrides` 로 monitoring/logging 등을 disabled 처리해 리소스 경합 감소. 실패 시 `dumpKindDiagnostics` 가 stack config / `kubectl get pods -o wide` / `kubectl get events --sort-by=.lastTimestamp` 덤프. 신규 `Makefile` 타겟 `test-golden-path` + `docs/20_아키텍처/F8_Task6_Kind_Runbook.md` 런북. 병렬 필수 조건이었던 `MemoryPipelineRepository.Delete` 누락 (F8 Task 7 precondition) 을 동시 해결 — e2e 빌드가 clean 하게 성공. **로컬 검증 결과**: `github-argocd-v1` 실제 통과 (4분 5초, cert-manager/metrics-server/PostgreSQL/MinIO/Argo CD 실 helm 설치), `gitlab-argocd-v1` 은 단일 노드 Kind + 15분 timeout 에서 미도달 (GitLab 풀스택 리소스 부족) — 런북에 "subtest 사이 Kind 재생성 필요 (cluster-scoped cert-manager CRD/ClusterRole leak)" 관측과 권장 운영 스크립트를 문서화. EKS/GKE 검증은 `F8-F6-Cloud` follow-up 으로 분리 유지.
- Deploy 단계 서버측 Pre-Deploy Gate (F8-F3): `POST /api/v1/stacks/:id/deploy` 가 `InstallStack.Execute` 호출 전에 `ValidateCompatibility` 를 persisted mode 로 재실행한다. `fail` → `DEPLOY_COMPAT_FAIL` 400 하드 블록, `warn` 은 body 의 `acknowledge_warnings=true` 가 없으면 `DEPLOY_COMPAT_WARN_UNACK` 400 으로 블록. 두 오류 모두 응답 body 에 `verdict.overall` / `verdict.issues` / `verdict.node_architectures` / `verdict.matrix` / `verdict.checkedAt` 를 포함해 프론트가 기존 Pre-Deploy Gate UI 로 그대로 렌더링할 수 있다. `ValidateCompatibilityInput.StackID` 필드 추가 + `WithStackRepository` 옵션: tools 가 비어 있고 StackID 만 주어지면 use case 가 stack 을 로드해 tools/clusterID 를 파생, 이를 통해 UI 를 우회한 직접 API 호출도 차단된다. `POST /stacks/:stackId/validate` 가 path 의 `:stackId` 를 body 가 생략했을 때 자동 보충해 persisted mode 로 동작. 프론트 Install Wizard 의 submit 플로우 재편 (`createStack → validateCompatibility → deployStack`): 서버 verdict 이 `fail` 이면 `DEPLOY_COMPAT_FAIL` UI, `warn` 이면 전용 ack 체크박스 (`server-warn-ack`) 를 띄우고 체크 후 재제출 시 `deployStack({ acknowledgeWarnings: true })` 로 진행. `useValidateCompatibility` 훅 재사용, `useDeployStack` 가 `{ stackId, acknowledgeWarnings }` 객체 입력 + legacy string 동시 지원. 신규 순수 util `shouldBlockOnServerVerdict` (`stack/utils/server-verdict.ts`) 가 verdict → `{ block, mode, acknowledgeWarnings? }` 결정을 내리며 3 단위 테스트로 정책 고정. 백엔드 테스트 `deploy_handler_compat_test.go` 5 케이스 (pass / fail / warn-unack / warn-ack / CLUSTER_ARCH_UNKNOWN ack 분기) + `validate_compatibility_test.go` persisted mode 4 케이스 (tools 로딩, stack 부재 에러, clusterID fallback 시 arch 체크, explicit tools override) 추가. `AcknowledgeWarnings` 는 opt-in 이므로 기존 클라이언트가 body 없이 호출하면 자동 `false` 로 해석되어 warn 조합이 기본 차단 — 의도된 보안 강화.
- Stack Install Wizard Auto Select + 노드 아키텍처 게이트 (F8 Task 5): `stack-install-page.tsx`의 Pre-Deploy Compatibility Gate 영역 위에 Golden Path 빠른 시작 카드 3종을 추가. 각 카드는 `compatibilityMatrixData`를 읽어 `isMatrixCompatibleWithCluster`로 현재 `draft.clusterId` 클러스터의 `nodeArchitectures`와 교차 검증한다 — incompatible 시 버튼 disabled + 툴팁으로 누락 아키텍처 명시, unknown 시 경고색 강조, 클러스터 미선택 시 안내 메시지. 클릭 시 `loadFromTemplate(matrix.id)`로 draft 주입. Gate의 기존 verified/untested/unsupported verdict 위에 `TOOL_ARCH_UNSUPPORTED` / `CLUSTER_ARCH_UNKNOWN` 이슈를 레이어링 (verified+arch miss → `fail`, untested+arch miss → `warn` 유지). 신규 순수 함수 `isMatrixCompatibleWithCluster` / `matrixArchMismatches` (+10 단위 테스트) 를 `stack/utils/compatibility-arch.ts`에 배치. i18n `stackInstall.compatibility.autoSelect.*` / `issue.*` 네임스페이스 신설 (ko/en).
- Admin Stack Version Management 페이지 (F8 Task 4): `/admin/stack-versions` (admin 전용) 신규 페이지 추가. 좌측 Golden Path 3종 목록(verified/untested/unsupported 배지), 우측 상세 — Kubernetes 범위 + tools 테이블 (arch/tier badges 포함, F8 Task 1 필드 노출) + clusters 섹션에 각 클러스터의 `node_architectures` 표시 + 매트릭스 교차 평가 (✓/✗/Unknown) + 행별 Refresh Discovery 버튼 (`useRefreshDiscovery` 호출 후 `useClusters` 캐시 invalidate). 공유 타입 확장: `CompatibilityTool.archSupport` / `minK8sVersion` / `tier`, `Cluster.nodeArchitectures`, `CompatibilityValidationResult.nodeArchitectures` / `matrix` / `message`. `stack-api.ts`의 `normalizeCompatibilityTool` 이 snake/Camel/Pascal 세 키 모두 수용. `admin-api.ts` 에 `refreshClusterDiscovery` + `useRefreshDiscovery`. `stack-api.ts validateCompatibility` 가 `{ clusterId, nodeArchitectures, tools }` 입력과 레거시 string stackId 모두 지원. 사이드바 `데브섹옵스 스택` 그룹에 `stackVersionsAdmin` 항목 추가 (role=admin).
- Compatibility Matrix 클러스터 노드 아키텍처 검증 (F8 Task 3): 마이그레이션 `000043_cluster_node_architectures`에서 `clusters.node_architectures TEXT[]` 컬럼을 추가하고, admin 모듈의 `kube.DiscoverCluster`가 `node.status.nodeInfo.architecture`를 수집해 sorted+deduped 셋으로 저장. `ClusterUseCase.RefreshDiscovery` 경로(신규 등록/업데이트/`/clusters/:id/refresh-discovery`)가 클러스터 실제 상태를 기록하며, 실패 시 `connection_status=connection_failed` + 빈 슬라이스로 축약. Stack 모듈의 Pre-Deploy Gate(`ValidateCompatibility`)는 신규 `port.ClusterReader`를 통해 `cluster_id` 또는 explicit `node_architectures` 입력을 받아 `ToolVersion.SupportsArch`를 교차 검증 — verified 매트릭스에서 아키 miss 시 `fail`(하드 블록), untested 매트릭스에서는 `warn` 유지, 아키 미상 시 `CLUSTER_ARCH_UNKNOWN` 경고. 신규 단위/통합 테스트 6종(kube fake clientset, usecase discovery 성공/실패, memory cluster round-trip, Pre-Deploy Gate 4종 시나리오, postgres node_architectures round-trip) 추가. Admin↔Stack 간 바운디드 컨텍스트 경계는 CI/CD의 `StackReader` 패턴과 동일하게 `ClusterReader` 인터페이스로 격리.
- Compatibility Matrix Narwhal baseline 재확정 (F8 Task 2): 마이그레이션 `000042_seed_narwhal_compat_refresh`에서 Golden Path 3종 조합(`gitlab-allinone-v1`, `gitlab-argocd-v1`, `github-argocd-v1`)의 `compatibility_matrices.tools` 및 `golden_path_templates.tools`를 Narwhal VERSIONS.md 기반 canonical baseline v1으로 재확정. GitLab CE/CI/Registry `9.5.1/18.5.1`, Harbor `1.15.0/2.11.0`, MinIO `5.2.0/2024-08-03`, Argo CD `6.8.0/v2.8.3`, Prometheus `67.0.0/v2.54.1`, Grafana `8.5.0/11.1.0`로 pin. `MemoryCompatibilityRepository`는 `narwhal*` 상수 블록으로 동일 값을 공유하고, `docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md`에 각 버전의 출처와 업데이트 규칙을 문서화. `TestMemoryCompatibilityRepository_NarwhalBaselineVersions`가 세 계층의 drift를 차단.
- Compatibility Matrix 스키마 세분화 (F8 Task 1): `ToolVersion`에 `MinK8sVersion`, `ArchSupport`, `Tier` 필드 추가. 마이그레이션 `000041_compat_tool_fields`에서 기존 3종 시드(gitlab-allinone-v1, gitlab-argocd-v1, github-argocd-v1)의 tools JSONB에 idempotent 하게 값을 패치 (Harbor/GitLab 계열은 amd64-only, 그 외 amd64+arm64). `ToolVersion.SupportsArch()` / `EffectiveMinK8sVersion()` 헬퍼로 Pre-Deploy Gate ARM64 체크 및 per-tool K8s 버전 검증의 기반을 제공.
- Cluster 모니터링 실집계 API 추가: `GET /api/v1/admin/clusters/:id/monitoring-summary` (kubeconfig 기반 전체 Pod/Ready Pod 및 CPU/Memory request/limit 요약)
- CI/CD 파이프라인 삭제 API 및 UI 추가: `DELETE /api/v1/cicd/pipelines/:id`, CI/CD List 상세 패널 `Delete` 버튼
- Stack Template → Install 오버라이드 공통 유틸 추가 (`web/src/features/stack/utils/template-overrides.ts`)
- CI/CD History 샘플 데이터 보강 마이그레이션 추가: `000040_seed_ml_service_history` (ML Prediction Service 배포 이력 10건)
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

- Monitoring Dashboard Cluster 뷰를 선택 클러스터 기준으로 재구성: Stack 모니터링 합산 + Stack 매핑이 없을 때 클러스터 실집계 자동 fallback
- Monitoring Dashboard의 CI/CD 탭 표시 정책 변경: 탭은 항상 노출하고, 클러스터 타입이 `target`이 아닐 때 비활성화
- CI/CD List 레이아웃을 Stack List 패턴으로 통일: 하단 확장 행 제거, 좌측 목록 + 우측 상세 패널(모바일은 하단)
- CI/CD List 상세 탭 구조 단순화: `Actions` 탭 제거, 주요 액션은 상세 헤더 버튼으로 통합
- CI/CD Pipeline Setup/Developer Deploy의 클러스터 선택을 Target Cluster 타입으로 제한
- Developer Deploy 네임스페이스 UX 개선: `default` 기본 제공 + `New Namespace` 직접 입력 지원
- Pipeline Logs 화면 상태 문구 개선: 배포 종료 후 로그 없음 상태를 명확히 안내
- Kind 로컬 클러스터 구성 조정: `scripts/kind-cluster.yaml`에 worker 노드 1대 추가
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

- 스택 재시도 안정화: cert-manager 네임스페이스/CRD ownership 감지, startupapicheck job optional 처리, GitLab rollout timeout 확장, rollback 잔존 리소스 정리를 반영해 rolled_back 재배포 경로를 안정화했습니다.
- 설치 단계 정합성 수정: `installing_openbao` 단계가 Orchestrator/UseCase 순서와 일치하도록 정렬해 `integration_check out-of-order` 실패를 수정했습니다.
- Stack 상세의 Gateway PF Copy 개선: 스택별 `GATEWAY_NAME`을 포함한 명령으로 잘못된 gateway service 선택을 방지하고, 접속정보 복사본에 Primary URLs + Gateway Port-Forward 섹션을 추가했습니다.
- 포트포워딩 스크립트 안전성 보강: 컨텍스트 폴백 메시지 정정 및 선택된 gateway service에 443 포트가 없을 때 HTTP-only로 자동 폴백하도록 수정했습니다.

- Stack 모니터링 OSS 상태 계산에서 one-shot 완료 Job Pod(`*-migrations*`, `*-job*`)를 제외해 GitLab migration 완료 Pod로 인한 `warning` 오탐을 방지
- Developer Deploy 진행 화면의 Created Resources 중복 노출 이슈를 리소스 dedupe 로직으로 보정
- CI/CD Deployment 상세 파싱 보강: step 로그가 없는 응답 형식도 fallback 로그로 표시
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

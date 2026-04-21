# 스택 버젼 호환성 매트릭스(Compatibility Matrix) 고도화 계획

**작성일시**: 2026-04-12
**기능 ID**: F8 (DevSecOps Stack OSS 버전 호환성 관리)
**소속 메뉴**: 데브섹옵스 스택 > 스택 버전 관리

---

## 1. 달성 목표 (Goal)

Nullus 시스템의 핵심 지향점인 **"안전하고 검증된 배포"**를 달성하기 위해, 수십 종의 OSS 도구들이 서로 충돌 없이 안착할 수 있는 조합을 시드 데이터로 구축합니다.
사용자(DevOps, 개발자)가 임의의 버전을 조합 시 발생할 수 있는 에러(K8s 버전 미달, CRD 충돌 등)를 **배포 위저드(Pre-Deploy Gate) 단계에서 사전 차단(Hard Block)하거나 경고(Warn)** 함으로써, 배포 성공률 90% 이상을 확보하는 것을 목표로 합니다.

## 2. 주요 기능 명세 (Features)

1. **상태 머신 게이트 (Pre-Deploy Gate)**
   * `pass`: 검증이 완료된 완전한 Golden Path 조합 (Recommended)
   * `warn`: 검증되지 않은 임의의 버전 조합 (사용자 명시적 승인 필요)
   * `fail`: 호환성이 깨지는 것으로 알려진 치명적 조합 (배포 버튼 활성화 불가)
2. **이원화된 버전 체크**
   * Helm 차트 관점의 `helm_version`과 내부 컨테이너 애플리케이션 관점의 `app_version`을 분리 검증 (예: `bitnami/postgresql` 차트 버전과 `Postgres 16` 엔진 버전 동시 확인)
3. **Narwhal 시드 데이터베이스 주입**
   * 오픈소스 프로젝트 Narwhal의 `VERSIONS.md`에 명시된 하드 트레이닝 데이터(각 분기별 검증 버전 차트)를 매트릭스로 활용
4. **아키텍처 및 제약사항 예외 처리**
   * 배포 타겟 쿠버네티스의 워커 노드 아키텍처(ARM64 등)를 판독하고, 지원하지 않는 이미지(Harbor 일부 등) 선택 시 대안 이미지를 강제하거나 Fail 처리.

---

## 3. 사용자별 사용 시나리오 (Use Cases)

| 페르소나 | 시나리오 요약 |
| :--- | :--- |
| **DevOps Engineer** | **UC 1. 골든 패스(Golden Path) 템플릿 생성**<br>데브옵스 엔지니어는 사내 개발팀을 위한 표준 CI/CD 환경을 조합할 때, 호환성 매트릭스에 `pass` 처리된(Recommended) 버전만을 선택하여 '표준 스택 템플릿' 카탈로그를 작성한다. 배포 실패 리스크를 원천 제거한다. |
| **Developer** | **UC 2. 사전 배포 경고의 확인 및 동의**<br>개발자가 위저드를 통해 신규 최신 버전의 인하우스 앱용 DB를 배포하려 했으나, 매트릭스상 검증되지 않은 조합으로 판별된다. 게이트에서 `warn`이 발생하고, 개발자는 "위험을 감수하고 설치합니다"라는 서약 체크 박스를 누른 뒤에야 배포를 진행할 수 있다. |
| **Admin** | **UC 3. 사내 전용 매트릭스 관리**<br>`스택 버전 관리` 메뉴에 진입하여 시스템이 제공하는 기본 매트릭스 외에, 내부 보안팀이 결재한 버전을 신규 `pass` 매트릭스로 직접 등록/관리한다. |

---

## 4. 현재 구현 현황 

*   **프론트엔드 (UI Layer): [🟢 완성도 높음]**
    *   `stack-install-page.tsx`에 `PreDeployCompatibilityGate` 로직 구비 완료.
    *   사용자의 선택 폼(K8s, MinIO, Postgres 등)을 실시간으로 감지, `useCompatibilityMatrix()` API 훅을 통해 점수 판별.
    *   `fail` 시 에러 토스트 노출 및 하드 블록, `warn` 시 명시적 동의(Acknowledgment 체크) UI 적용 완료.
    *   다국어(`ko.json`, `en.json`) 번역 및 메시지 매핑 완료.

*   **백엔드 (API Layer) 및 DB: [🟡 진행률 약 30%]**
    *   데이터베이스 스키마 및 더미/하드코딩된 API 규격 구성 중.
    *   실제 Narwhal Seed 데이터베이스(수십 종의 도구 매핑 테이블) 미완성.

---

## 5. 잔여 개발 아이템 (Backlog 및 계획)

v1 GA 전까지 이 기능을 프로덕션 수준으로 끌어올리기 위한 남은 작업 목록입니다.

### 백엔드 & 인프라 (Backend & Infra)
- [x] **Task 1:** `compatibility_matrices` DB 테이블 고도화 (`helm_version`, `app_version`, `min_k8s_version` 등 스키마 세분화)
  - 마이그레이션 `000041_compat_tool_fields` (up/down) — tools JSONB에 `MinK8sVersion`, `ArchSupport`, `Tier` per-tool 필드 추가 (idempotent). Harbor / GitLab 계열은 amd64-only, 그 외 amd64+arm64. Tier 매핑: verified→stable, untested→beta, unsupported→deprecated.
  - `internal/stack/domain/compatibility.go`의 `ToolVersion`에 3개 필드 추가 + `SupportsArch()` / `EffectiveMinK8sVersion()` 헬퍼, `ToolTier*` / `ArchAMD64` / `ArchARM64` 상수 도입.
  - `memory_compatibility.go` defaultCompatibilityMatrices() 동기화, postgres 리포지토리는 기존 `json.Unmarshal`로 신규 필드 자동 수용.
  - 단위 테스트: `ToolVersion_V2Fields`, `ToolVersion_SupportsArch`, `ToolVersion_EffectiveMinK8sVersion`, `MemoryCompatibilityRepository_ToolV2Fields` 추가.
- [x] **Task 2:** `GET /api/v1/compatibility/matrix` API에 Narwhal 기반의 **Golden Path 3종 조합을 Seed Data로 영속화**하여 제공.
  - 마이그레이션 `000042_seed_narwhal_compat_refresh` (up/down) — `compatibility_matrices.tools` 및 `golden_path_templates.tools` 두 테이블에 대해 Narwhal VERSIONS.md 기반 canonical baseline v1을 idempotent 하게 재확정. `000008 → 000024 → 000026 → 000033 → 000041` 체인 이후의 결과를 단일 지점에서 수렴시킴.
  - `internal/stack/adapter/repository/memory_compatibility.go`에 `narwhal*` 버전 상수 블록을 도입해 인메모리 리포지토리를 DB seed와 동기화 (이전에는 pre-000024 버전으로 drift되어 있었음).
  - 버전 출처 및 업데이트 규칙을 `docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md`에 문서화. DB / 인메모리 / 문서 3계층이 동일한 pin을 공유하도록 운영 규칙을 명문화.
  - 단위 테스트 `TestMemoryCompatibilityRepository_NarwhalBaselineVersions` 추가 — Golden Path 3종 각각에 대해 (category, tool name, helm_version, app_version) 조합을 고정. 계층 간 drift 시 테스트가 차단.
- [x] **Task 3:** 쿠버네티스 클러스터 Discovery 로직 내 ARM64 (Node Architecture) 체크 및 파라미터 전달 로직.
  - 마이그레이션 `000043_cluster_node_architectures` (up/down) — `clusters.node_architectures TEXT[] NOT NULL DEFAULT '{}'` 컬럼을 idempotent 하게 추가 (`ADD COLUMN IF NOT EXISTS`).
  - `internal/admin/domain/cluster.go`에 `Cluster.NodeArchitectures []string` 필드, `ClusterDiscoveryInfo` 값 객체, `NormalizeNodeArchitectures()` 헬퍼 추가. 결정론적 정렬 + dedup + nil-safe empty 처리.
  - `internal/admin/adapter/kube/client.go`에 `DiscoverCluster(ctx, kubeconfig) (*ClusterDiscoveryInfo, error)` 추가. `clientsetBuilder` 인디렉션으로 fake clientset 주입 가능. 기존 `VerifyCluster`는 `DiscoverCluster`에 위임.
  - `internal/admin/port/repository.go`에 `ClusterDiscoverer` 포트 신설, `internal/admin/adapter/kube/discoverer.go`에 어댑터 구현. Use case는 `port.ClusterDiscoverer`만 의존.
  - `ClusterUseCase.RefreshDiscovery(ctx, id)` 추가 + `WithDiscoverer` / `WithKubeconfigDecryptor` 옵션. 실패 시 `ConnectionStatusConnectionFailed` + 빈 slice 로 축약 저장. 성공 시 `ConnectionStatusConnected` + 정렬된 arch 셋 저장.
  - 핸들러 확장: `RegisterCluster` / `UpdateCluster` 에서 kubeconfig 저장 직후 best-effort discovery 수행. `VerifyCluster` 가 `DiscoverCluster` + `RefreshDiscovery` 를 호출. 신규 `POST /clusters/:id/refresh-discovery` 엔드포인트 추가. 응답 DTO (`clusterResponse`) 에 `node_architectures` 필드 노출. `POST /clusters/verify` draft 응답에도 포함.
  - Stack 측에 `port.ClusterReader` + `ClusterSummary` 인터페이스 신설 (`internal/stack/port/cluster_reader.go`). CI/CD 의 `StackReader` 와 동일한 Bounded Context 경계 패턴. 구현: `PostgresClusterReader` (clusters 테이블 직접 읽기, 모듈러 모놀리스 한정).
  - `ValidateCompatibility.Execute` 에 `NodeArchitectures` / `ClusterID` 입력 수용. 검증 정책: cluster context 미제공 → 기존 동작 유지, 제공 + verified 매트릭스 + arch miss → `fail` + `TOOL_ARCH_UNSUPPORTED`, untested 매트릭스 + arch miss → `warn` (severity warning), arch 미상 → `warn` + `CLUSTER_ARCH_UNKNOWN`. 응답 body 에 `node_architectures` 포함. `POST /compatibility/validate` 핸들러가 `cluster_id` / `node_architectures` 입력 수용.
  - 신규 테스트 (모두 `go test ./internal/admin/... ./internal/stack/...` 통과):
    - `internal/admin/adapter/kube/client_test.go`: fake clientset 기반 single-arch / 멀티-arch / no-nodes / VerifyCluster 위임 4 케이스.
    - `internal/admin/usecase/cluster_usecase_test.go`: `RefreshDiscovery` 성공 → Cluster 반영, 실패 → `connection_failed` + 빈 slice, kubeconfig 없음 → `KUBECONFIG_NOT_REGISTERED` AppError 3 케이스.
    - `internal/admin/adapter/repository/memory_cluster_test.go`: round-trip 정규화, deep-copy (input/output 양방향), update 재반영, empty slice → nil 4 케이스.
    - `internal/stack/usecase/validate_compatibility_test.go`: single-amd64 pass, 혼합 + verified → fail, 혼합 + untested → warn, cluster_id 해석, arch 미상 → CLUSTER_ARCH_UNKNOWN 5 케이스.
    - `internal/admin/adapter/repository/postgres_integration_test.go`: `node_architectures` round-trip + 정규화 + empty → nil (integration 태그).

### 프론트엔드 (Frontend)
- [x] **Task 4:** `데브섹옵스 스택 > 스택 버전 관리` 메뉴를 위한 **어드민 뷰 페이지 (목록/상세 그리드)** 신규 제작.
  - 신규 admin 전용 라우트 `/admin/stack-versions` (`features/admin/pages/stack-versions-page.tsx`) + 사이드바 `데브섹옵스 스택` 그룹에 `stackVersionsAdmin` 항목 추가. `ProtectedRoute allowedRoles={['admin']}` 로 role gate.
  - `ListDetailPanel` 레이아웃: 좌측 매트릭스 목록(status 배지 + ID + K8s 범위 + tool count), 우측 상세 — K8s 범위, tools 테이블 (name / helm / app / **arch 뱃지** / **minK8s** / **tier 뱃지**), clusters 테이블 (cluster name, node_architectures 뱃지, 교차 평가 ✓/✗/Unknown, 행별 Refresh Discovery 버튼).
  - 공유 타입 확장: `CompatibilityTool.archSupport`/`minK8sVersion`/`tier` (F8 Task 1 서버 필드 노출), `Cluster.nodeArchitectures` (F8 Task 3 서버 필드 노출). `stack-api.ts normalizeCompatibilityTool` 이 snake/camel/Pascal 키 모두 수용. `admin-api.ts normalizeCluster` 동일.
  - 신규 API 훅 `useRefreshDiscovery` — `POST /admin/clusters/:id/refresh-discovery` 호출 후 `useClusters` / `useCluster` 캐시 invalidate. 실패 응답(`KUBECONFIG_NOT_REGISTERED` 등)은 mutation 상태로 버튼 옆에 노출.
  - 신규 순수 함수 `isMatrixCompatibleWithCluster` / `matrixArchMismatches` (`features/stack/utils/compatibility-arch.ts`) — 백엔드 `SupportsArch` 정책 동등. 10개 단위 테스트(`compatibility-arch.test.ts`) — single-amd64 pass, 혼합 + amd64-only 툴 → incompatible, 미상 → unknown, mismatches 세부 열거 포함.
  - i18n 신설 네임스페이스 `stackVersionsAdmin.*` (ko/en): `title`, `subtitle`, `listTitle/Subtitle`, `selectMatrix`, `breadcrumb.*`, `status.*`, `tier.*`, `archBadge.*`, `crossEval.*`, `col.*`, `refreshDiscovery.*`. `sidebar.stackVersionsAdmin` 사이드바 라벨 추가.
- [x] **Task 5:** Golden Path 조합(매트릭스 3종)을 유저가 단축 버튼 클릭으로 한 번에 `Auto Select` 할 수 있는 UI 적용.
  - `stack-install-page.tsx` Pre-Deploy Gate 영역 위에 "Golden Path Quick Start" 카드 섹션 추가. `compatibilityMatrixData.filter(status !== 'unsupported')` 를 id 기준 정렬해 렌더. 클릭 시 `loadFromTemplate(matrix.id)` 로 draft.tools + selectedTemplateId 일괄 주입 (매트릭스 ID = Golden Path template ID 를 000042 시드가 보장).
  - **회색 처리**: 선택된 `draft.clusterId` 의 `nodeArchitectures` 와 `isMatrixCompatibleWithCluster` 로 판정 — `incompatible` → disabled + 툴팁에 누락 아키텍처 명시, `unknown` → 경고색 + "Refresh Discovery 실행" 안내, 클러스터 미선택 → "클러스터 선택 필요" 안내, `compatible` → active.
  - **Gate 확장**: `compatibilityGate` useMemo 에서 `matched` 매트릭스가 결정된 시점에 `isMatrixCompatibleWithCluster(matched, clusterArchs)` 를 실행. verified+incompatible → `fail` (code `TOOL_ARCH_UNSUPPORTED`), untested+incompatible → warn 유지 + 경고 추가, unknown → warn + `CLUSTER_ARCH_UNKNOWN`. 기존 경고 ack 체크박스 UX 그대로, 아키 관련 메시지만 issues 배열에 합류.
  - `PreDeployCompatibilityIssue` 에 `code?: string` 필드 추가 — 서버 `/stacks/:stackId/validate` 응답의 issue code 와 동일 문법을 유지해 향후 서버 호출 경로 도입 시 호환.
  - `validateCompatibility` API + `useValidateCompatibility` 훅을 `{ stackId, clusterId, nodeArchitectures, tools }` 입력 + 레거시 문자열 stackId 동시 지원으로 확장. 서버 응답 `node_architectures` / `matrix` / `message` 정규화.
  - i18n 신설: `stackInstall.compatibility.autoSelect.{title,subtitle,selectCluster,archUnknown,archMismatch,compatible}` + `stackInstall.compatibility.issue.{toolArchUnsupported,clusterArchUnknown,kubeconfigNotRegistered}` + `stackInstall.compatibility.refreshDiscoveryCta` (ko/en).
  - TypeScript strict 통과 (`npx tsc --noEmit`). 기존 pure 단위 테스트 `compatibility-arch.test.ts` 가 회색 처리 로직을 직접 커버 (e.g. GitLab amd64-only + 혼합 클러스터 → incompatible + `{toolName:"GitLab CE", missingArchs:["arm64"]}`).

### DevOps / QA
- [x] **Task 6 (v1 GA 스코프):** 3종의 Golden Path 조합을 **로컬 Kind 클러스터(`nullus-platform`)** 에서 CI 파이프라인으로 배포 후 성공 여부 검증. (스코프 조정 2026-04-20: EKS/GKE 실환경 검증은 v1 GA 이후 follow-up 으로 분리. 본 Task 는 로컬 재현 가능성과 회귀 방지에 집중.)
  - `e2e/golden_path_kind_test.go` (`//go:build e2e`) 신규 — 단일 테스트 `TestF8Task6_GoldenPath_KindDeploy` 가 `discoverKindCluster` 로 `nullus-platform` 발견 시 3 subtest 순차 실행, 미발견 시 graceful skip. 각 subtest: in-memory `StackRepository` / `TemplateRepository` + 실 `helm.Orchestrator` → `InstallStack.Execute` → 상태 폴링 → `completed` 도달 확인. 실패 시 `dumpKindDiagnostics` 가 stack config / `kubectl get pods -o wide` / events 덤프.
  - `goldenPathCase.toolOverrides` 로 monitoring/logging 등을 disable 해 단일 노드 Kind 리소스 경합 감소. `github-argocd-v1` 은 source_repository / ci_platform (external SaaS) 도 disable.
  - Makefile `test-golden-path` 타겟 (`go test -tags e2e -run "^TestF8Task6_GoldenPath" -timeout 60m`) + `.PHONY` 등록. 기본 `go test ./...` 파이프라인에는 영향 없음.
  - `docs/20_아키텍처/F8_Task6_Kind_Runbook.md` 신규 — 선결 조건, Kind 기동, subtest 선택 실행, 실패 분류, 정리 절차.
  - **선행 precondition 해결**: `internal/cicd/adapter/repository/memory_pipeline.go` 의 `MemoryPipelineRepository.Delete` 누락을 추가 — e2e 빌드가 `go build -tags e2e ./...` 에서 clean 하게 성공.
  - **실제 로컬 검증 결과 (2026-04-20)**:
    - `github-argocd-v1`: ✅ **PASS (4분 5초)**. cert-manager, metrics-server, PostgreSQL, MinIO, Argo CD 실 helm 설치 + `stack.state=completed` 도달. 첫 Task 6 end-to-end 성공 사례 확보.
    - `gitlab-argocd-v1`: ❌ 미도달 (15분 timeout). Fresh Kind 에서 GitLab 풀스택 설치 중 pod Ready 까지 도달하지 못함 — 단일 노드 Kind 리소스 한계 추정.
    - `gitlab-allinone-v1`: ⏭ 미시도 (Task 6 런북 §권장 사양 확보 전 skip).
  - **관측된 subtest-order 이슈**: cert-manager 의 CRD / ClusterRole / ClusterRoleBinding 은 cluster-scoped 이므로 subtest 사이에서 ownership 충돌 ("ClusterRole \"cert-manager-cainjector\" ...cannot be imported"). `helm uninstall` best-effort 로는 완전 제거 불가. 런북에 **subtest 사이 `kind delete cluster && kind create cluster` 재생성 필수** 를 명문화 + 샘플 shell script 제공. CI 샤딩으로 각 runner 가 subtest 1개씩 처리하도록 권장.
  - **확정된 follow-up**: `gitlab-argocd-v1` / `gitlab-allinone-v1` 의 실 클러스터 배포 성공 재현은 `F8-F6-Cloud` (EKS/GKE) 에서 리소스 보장 하에 진행. Kind 쪽 실패는 단일 노드 리소스 한계 이슈이며 seed drift (Task 2 범위) 이슈는 아님 — helm chart pull 단계까지는 진입했음을 events 로그에서 확인 (imagePull OK, pod creation OK, readiness 도달 전 종료).
- [x] **Task 7:** `warn` 상태에서 강제 진행 후 배포 실패 시 복구되는(Retry/Rollback) 통합 E2E 테스트 추가.
  - `e2e/warn_forced_retry_rollback_test.go` (`//go:build e2e`) 신규 — 독립 `httptest` 서버 + in-memory 리포지토리 + `fakeStepExecutor` (`atomic.Bool` 기반 실패 주입) + `warnClusterReader` (arm64 cluster 시뮬레이션) 로 6 subtest 실행. 기존 `e2e/setup_test.go` 공유 서버는 건드리지 않음.
  - subtest A/B/C/D (warn verdict / ack 차단 / ack + 성공 / ack + 실패→`rolled_back`) + E (state rewind 기반 Retry 계약 검증) + F (rollback 엔드포인트 prior 버전 복원 + 새 history row append) 모두 통과 (`3.0s` 총 실행).
  - Playwright `web/e2e/stack-warn-forced-retry.spec.ts` (`@stack-critical`) 신규 UI 스모크 — `/admin/stack-versions` 가 untested 매트릭스를 노출 + `/stack/install` 의 Auto Select 카드가 동일 매트릭스를 렌더링. 실제 deploy→terminal-state 구동은 Kind 의존이라 Task 6 / F8-F6-Cloud 범위로 분리.
  - Retry 경로는 테스트 레벨 state rewind 로 계약 검증만 수행. 프로덕션용 `POST /stacks/:id/retry` API 는 follow-up 으로 §6 에 별도 항목화.

---

## 6. v1 GA 후 Follow-up

v1 GA 본 백로그(Task 1~7) 완료 이후 발견된 구조적 gap 보강 작업.

- [x] **F8-F3:** Deploy 단계 서버측 호환성 재검증 (Server-side Pre-Deploy Gate).
  - 문제 정의: draft 단계의 Install Wizard 는 stackId 가 없어서 서버 `/stacks/:stackId/validate` 를 호출할 수 없고, 또한 `POST /stacks/:id/deploy` 가 서버측 게이트 없이 `InstallStack.Execute` 를 곧장 호출 → UI 를 우회한 API 호출이 fail 조합을 강행 배포할 수 있었음.
  - **백엔드**: `ValidateCompatibilityInput.StackID` + `WithStackRepository` 옵션으로 persisted mode 도입. Tools 가 비어 있고 StackID 만 있으면 use case 가 stack 을 로드해 `stack.Tools` → `{category: name}`, `ClusterID` 가 비어 있으면 `stack.ClusterID` 로 fallback. Explicit tools 가 있으면 그것이 우선 (override). `POST /stacks/:stackId/validate` 는 path `:stackId` 를 body 가 생략했을 때 자동 보충.
  - **Deploy 핸들러**: `DeployHandler.WithOptions(WithValidateCompatibility(...))` 로 ValidateCompatibility 를 주입. request body `{"acknowledge_warnings": bool}` 를 파싱 (빈 body = false). 게이트 결과: `fail` → `DEPLOY_COMPAT_FAIL` 400, `warn` + ack 미확인 → `DEPLOY_COMPAT_WARN_UNACK` 400, 그 외 → 기존 `InstallStack.Execute` 흐름. 두 400 응답 모두 body 에 `error.verdict = {overall, issues, node_architectures, matrix, message, checkedAt}` 를 포함해 프론트가 기존 Pre-Deploy Gate UI 로 그대로 렌더링 가능. `audit.Log` 에 `compatibility_verdict` / `issue_codes` / `acknowledge_warnings` 기록.
  - **프론트엔드**: `useDeployStack` 가 `{ stackId, acknowledgeWarnings }` 객체 + legacy string 동시 지원으로 확장. Install Wizard submit 플로우 재편 — (1) 기존 client-side pre-check → (2) `createStack.mutateAsync(request)` → (3) `useValidateCompatibility({ stackId })` persisted mode 호출 → (4) `shouldBlockOnServerVerdict(verdict, ack)` 순수 함수로 분기 (pass / warn-ack / block) → (5) `deployStack.mutateAsync({ stackId, acknowledgeWarnings })`. server-verdict-panel 을 기존 gate 아래 렌더링, warn 시 전용 `server-warn-ack` 체크박스로 ack 수집. `pendingStackId` 로 재제출 시 `createStack` 중복 호출 방지.
  - **에러 처리**: `extractDeployCompatError(error)` 유틸로 `/deploy` 경로의 400 verdict body 를 파싱해 `toDeployErrorMessage` 가 `[TOOL_ARCH_UNSUPPORTED] ...` 형태로 issue 단위 상세 메시지를 노출.
  - **테스트 커버리지**: `deploy_handler_compat_test.go` 5 케이스 (pass / fail TOOL_ARCH_UNSUPPORTED / warn-unack / warn-ack / CLUSTER_ARCH_UNKNOWN ack 분기), `validate_compatibility_test.go` persisted mode 4 케이스 (tools 로딩, stack 부재 에러, clusterID fallback 시 arch check, explicit tools override), `stack/utils/server-verdict.test.ts` 3 케이스 (pass / fail / warn-ack 결정 정책). End-to-end DOM 시나리오는 Playwright e2e 로 이관 (기존 wizard 의 tool/cluster/namespace 입력 세팅을 duplicate 하지 않기 위함).
  - **i18n 신설**: `stackInstall.compatibility.issue.serverFail`, `serverWarnAck`, `serverVerdict.title`, `serverVerdict.ackLabel` (ko/en).
  - **하위 호환**: `/stacks/:stackId/validate` 가 tools 있는 body 로 호출되면 기존 동작 유지. `/stacks/:id/deploy` 가 body 없이 호출되면 `acknowledge_warnings=false` 로 해석 → warn 조합이면 블록 (의도된 보안 강화).
  - **관측 포인트**: createStack 이후 서버 fail 시 stack 이 persist 된 채로 남는다 (orphan draft 리스크). 현재는 `pendingStackId` 를 통해 재제출 시 동일 stack 에 재검증만 수행하지만, 조합 수정 후 재제출하려면 updateStack 경로가 필요 (별도 follow-up 이슈).
  - **follow-up 후보**: orphan stack 자동 정리 / updateStack 경로, 매트릭스 CRUD UI, 서버 verdict 캐시 (동일 stack state 반복 재검증 회피), nightly Refresh Discovery cron.

- [ ] **F8-F6-Cloud:** Task 6 확장 — AWS EKS / GCP GKE 실환경에서의 Golden Path 3종 CI 배포 검증. v1 GA 스코프는 로컬 Kind 클러스터 검증(Task 6)으로 대체했고, 본 항목은 클라우드 프로비저닝 비용·권한·시크릿 관리 전제가 확보되는 시점에 진행. 로컬 배포가 안정화된 이후에야 의미가 있으므로 Task 6 완료가 precondition.
- [x] **F8-Retry-API:** `POST /api/v1/stacks/:id/retry` 프로덕션 엔드포인트 완료 (Phase 3). DeployHandler 에서 `runPreDeployGate` 공통 헬퍼 추출해 Deploy/Retry 가 공유. `rolled_back`/`failed` → `pending` 되감기 후 InstallStack 재실행. 5 테스트 케이스 + `retry` audit action + 프론트 `useRetryStack` 훅.
- [x] **F8-Phase1:** `@stack-critical` pre-existing 회귀(login URL) 수정 — `beforeEach` 의 `waitForURL('**/stack/templates')` → `'**/'`, 개별 테스트가 `/stack/templates` 로 goto. 3/3 @stack-critical 그린.
- [x] **F8-Phase2:** `audit.Sink` 인터페이스 + `MemorySink` 도입 — DeployHandler/StackHandler/ClusterHandler/MemberHandler/OrgHandler 가 모두 `audit.Sink` 에 의존. `*AuditLogger` 는 인터페이스를 구현해 기존 프로덕션 동작 변경 없음. MemorySink 로 E2E 단에서 acknowledge_warnings/compatibility_verdict/issue_codes 검증 가능.
- [x] **F8-Phase4 (orphan stack / updateStack):** `UpdateStack` usecase + `PUT /stacks/:id` 엔드포인트. `{pending, failed}` 에서만 허용, prior config 를 history 에 자동 스냅샷. StackHandler 에 `WithOptions(WithUpdateStack)` 주입. 4 단위 테스트.
- [x] **F8-Phase6 (verdict 캐시):** `MemoryVerdictCache` (sync.Map + TTL) + `VerdictCacheKey` (SHA256 of sorted stack/cluster/arch/tools). `ValidateCompatibility.WithVerdictCache` 옵션. 5 테스트. `VERDICT_CACHE_TTL_SEC` env override.
- [x] **F8-Phase7 (nightly Refresh Discovery):** `RefreshDiscoveryScheduler` — interval 마다 모든 cluster 의 discovery sweep, in-flight guard, ctx cancel 시 graceful stop. main.go 에서 signal handler 연동. 4 fake-runner 테스트. `REFRESH_DISCOVERY_INTERVAL` env override.
- [x] **F8-Phase5 (매트릭스 CRUD UI) — 재개 완료 (2026-04-20):** `port.CompatibilityRepository` CRUD 3메서드 + sentinel errors + Memory/Postgres 구현 + `ManageCompatibility` usecase (입력 validation + verdict cache Clear) + `CompatibilityHandler` admin 라우트 3종 + `mapCompatibilityError` + audit 기록 + `MatrixEditModal` 컴포넌트 + `ConfirmDialog` 기반 삭제 확인 + `stack-versions-page` New/Edit/Delete 버튼 통합. 20 단위 테스트(repo 6 + usecase 8 + handler 6) 전부 green. i18n `stackVersionsAdmin.{actions,modal,deleteConfirm}.*` (ko/en).
- [x] **F8-Phase3-Retry-UI (Retry 버튼) — 완료 (2026-04-20):** `canRetry(status)` 순수 헬퍼 + `RetryStackButton` 컴포넌트 (warn-ack Modal + issue list 노출) + `extractDeployCompatError` 공유 유틸 추출. `stack-list-page` Info 탭 action row 에 자기검열 버튼 배치. i18n `stackList.retry.*`. 10 enum 매트릭스 테스트 통과.
- [x] **F8-Phase5-DOMTest — 완료 (2026-04-20):** `stack-versions-page.test.tsx` 5 DOM 스모크 (page render / New 모달 / Edit 모달 prefill + ID disabled / Delete confirm → mutate / Cancel → dismiss). `vi.mock` 으로 `useCompatibilityMatrix` / `useDeleteMatrix` 등 훅 계약 커버. `renderWithProviders` 재사용.
- [x] **F8-DeployError-Dedup — 완료 (2026-04-20):** `stack-install-page.tsx` inline `extractDeployCompatError` 38 라인 제거하고 `utils/deploy-error.ts` 의 `DeployCompatError`-반환 공유 유틸로 전환. `deploy-error.test.ts` 4 케이스 (FAIL 파싱 / WARN_UNACK 파싱 / 비-compat null / malformed null) 추가.
- [x] **F8-RetryUI-E2E — 완료 (2026-04-20):** `web/e2e/stack-retry-button.spec.ts` 신규 3 케이스 (`@stack-critical`): (1) failed 스택은 Retry 노출/completed 는 숨김, (2) Retry click → `POST /stacks/:id/retry` 1회 발생, (3) WARN_UNACK 응답 → warn modal + issue list + ack 체크박스 → `acknowledge_warnings: true` 로 재제출. `page.route` 스텁 + `sessionStorage` 시드로 backend 의존 제거. 선재 `stack-warn-forced-retry.spec.ts` / `stack-workflow.spec.ts` 는 라이브 Postgres 의존이라 backend up 일 때만 통과 — 본 Phase 는 수정하지 않음.
- [x] **F8-Retry-Toast — 완료 (2026-04-20):** `RetryStackButton` 의 inline `errorMessage` 상태를 제거하고 sonner `toast.success` / `toast.error` 로 전역 알림 전환. 200 → success, `DEPLOY_COMPAT_FAIL` → error + issue 목록, `DEPLOY_COMPAT_WARN_UNACK` → 기존 modal (토스트 미발화), 기타 에러 → error. i18n `stackList.retry.toasts.{success,failure}` 재사용. `retry-stack-button.test.tsx` 4 케이스 (success / FAIL / WARN_UNACK modal / ack → 재시도 200) 모두 green.
- [x] **F8-DeploymentLogs-Retry — 완료 (2026-04-20):** `stack-deployment-logs-page.tsx` 에 real-data 분기 추가. `deploymentId` 가 `DEPLOYMENT_DATA` key 에 매칭되면 기존 mock 렌더(하위호환 보존), 아니면 `useStacks()` 로 실제 stack 조회 → `RealStackView` + `RetryStackButton`(failed/rolled_back 만 자기검열 노출) 렌더. `stack-deployment-logs-page.test.tsx` 4 케이스. 부수 효과로 `stack-list-page.test.tsx` 의 Phase B 선재 `useRetryStack` 누락 regression 도 수정(4 pre-existing failures → green).
- [x] **F8-UIUX-ServerVerdictI18n — 완료 (2026-04-21):** server verdict 패널(`stack-install-page.tsx`)에서 `[TOOL_ARCH_UNSUPPORTED]` 같은 원시 코드 노출 제거. 신규 `utils/compat-issue-i18n.ts` (`COMPAT_ISSUE_I18N` 매퍼 + `getCompatIssueMessage(t, issue)`) 가 코드→i18n 키 바인딩을 담당. 원시 코드는 `<li data-code="...">` 로만 보존(E2E/debug용). 3 단위 테스트(known/unknown/no-code).
- [x] **F8-UIUX-DeployGateServerCheck — 완료 (2026-04-21):** Deploy 버튼 disable 조건에 server verdict 검사 추가. 신규 순수 함수 `utils/deploy-gate.ts::isDeployServerGateLocked(verdict, ack)` → `null/pass`→false, `fail`→true, `warn && !ack`→true. 4 단위 테스트. `serverFailHint` 배너 (i18n `stackInstall.compatibility.gate.serverFailHint`, ko/en) 가 기존 manifest 경고 아래에 조건부 렌더.
- [x] **F8-UIUX-WarnAckI18n — 완료 (2026-04-21):** `retry-stack-button.tsx` cross-feature cancel 키(`stackVersionsAdmin.modal.cancel`) 제거. 신규 `stackList.retry.confirmWarn.{cancel,confirm}` (ko/en — "취소" / "경고 확인 후 재시도"). `retry-stack-button.test.tsx` 의 warn-ack modal 케이스에 Cancel/Confirm 라벨 assertion 추가. 기존 4 케이스 green 유지.
- [x] **F8-UIUX-MatrixEditDirty — 완료 (2026-04-21):** `matrix-edit-modal.tsx` 에 dirty-drop 가드. edit 모드에서 카테고리가 빈 채로 name/helm/app 중 하나라도 입력된 row 를 `droppedRows` 로 집계 → 1차 Save 는 경고 배너(`data-testid="matrix-drop-warn"`) 만 노출하고 mutation 유보, 2차 Save 에서 실제 `useUpdateMatrix.mutate` 호출. 사용자가 category 를 다시 채우면 `confirmDrop` 자동 reset. 신규 i18n `stackVersionsAdmin.modal.dropWarn.{title,unnamed,hint}` (ko/en). `matrix-edit-modal.test.tsx` 3 케이스 green.

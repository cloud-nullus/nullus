# F8 Follow-up 일괄 작업지시 프롬프트 (F8-F6-Cloud 제외)

> **목적:** F8 v1 GA 종결 후 `docs/plans/compatibility_matrix_plan.md` §6 에 등재된 follow-up 들을 `F8-F6-Cloud` 를 제외하고 전부 수행한다.
> **대상 follow-up (7개, 실행 순서대로):**
> 1. **Phase 1** — Pre-existing `@stack-critical` 회귀 수정 (stack-monitoring / stack-workflow)
> 2. **Phase 2** — Audit 캡처 인터페이스 (`audit.Sink`) 도입
> 3. **Phase 3** — `F8-Retry-API`: `POST /api/v1/stacks/:id/retry`
> 4. **Phase 4** — Orphan stack 정리 + `PUT /stacks/:id` (updateStack)
> 5. **Phase 5** — 매트릭스 CRUD UI + 백엔드 엔드포인트
> 6. **Phase 6** — 서버 verdict 캐시
> 7. **Phase 7** — Nightly Refresh Discovery 스케줄러

> **중요:** 규모가 크므로 각 phase 를 **독립 commit 단위** 로 구분한다. Phase N 완료 시 `go test ./...` + `pnpm -C web test` 스모크를 돌려 그린 확인 후 다음 phase 로 진입. Phase 중간에 큰 회귀가 발견되면 해당 phase 를 드롭하고 원인 보고 → 나머지 phase 계속 진행.

---

## 공통 규칙 (모든 Phase 공통)

### 하지 말 것
- 마이그레이션 번호 재사용 금지 (000044 부터 순차).
- 기존 `AuditLogger` 의 pgxpool 동작 변경 금지 (Phase 2 는 확장만).
- `ValidateCompatibility.applyArchCheck` verdict 정책 변경 금지.
- 에러 코드 이름 변경 금지 (`DEPLOY_COMPAT_FAIL`, `DEPLOY_COMPAT_WARN_UNACK`, `TOOL_ARCH_UNSUPPORTED`, `CLUSTER_ARCH_UNKNOWN`, `KUBECONFIG_NOT_REGISTERED`). 새 코드는 추가만.
- `@stack-critical` 태그 제거 금지.
- 모듈 경계 위반 금지 (stack ↔ admin 은 `port.ClusterReader`, cicd ↔ stack 은 `port.StackReader` 패턴 유지).
- 커밋/푸시는 **phase 경계에서만**, 본문 합의된 commit message 로. Phase 중간의 즉흥적 commit 금지.

### 공통 산출물 (각 phase 마다)
- `CHANGELOG.md` `[Unreleased] > Added` 에 phase 단위 엔트리 추가.
- `docs/plans/compatibility_matrix_plan.md` §6 해당 항목을 `[x]` 로 + 3~5줄 요약.
- 새 에러 코드는 ko.json / en.json 양쪽에 i18n 키 추가.
- 테스트 태그:
  - 빠른 단위 테스트: 무태그 (`go test ./...`).
  - kind 의존 E2E: `//go:build e2e`.
  - DB 의존 통합: `//go:build integration`.

### 공통 보고 섹션 (phase별)
1. 변경 파일 목록 (백엔드 / 프론트엔드 / 테스트 / 문서 / 마이그레이션).
2. 새 테스트와 의도.
3. `go test` / `vitest` / `tsc` / `eslint` / `playwright` 실행 결과.
4. 의사결정 포인트.
5. 다음 phase 진입 가능 여부 (스모크 green / 차단 요인).

---

## Phase 1 — Pre-existing `@stack-critical` 회귀 수정

### 배경
`web/e2e/stack-monitoring.spec.ts` 및 `stack-workflow.spec.ts` 의 `beforeEach` 가 로그인 후 `page.waitForURL('**/stack/templates')` 를 기대하지만, 현재 `web/src/features/auth/pages/login-page.tsx:79` 가 `navigate('/')` 로 Home 으로 보냄. Task 7 에서 2개 pre-existing fail 로 관측됨.

### 조치
- **방향 결정**: 로그인 후 Home(`/`) 이동은 현재 의도된 동작이다 (Task 5 이후 Home 이 role-aware CTA 의 hub). 따라서 **테스트 기대치를 현행 리다이렉트와 일치시키는** 방향으로 수정한다.
- `stack-workflow.spec.ts` / `stack-monitoring.spec.ts` 의 `beforeEach` 에서 `await page.waitForURL('**/stack/templates')` → `await page.waitForURL('**/')` 로 변경.
- 로그인 이후 Stack Templates 페이지로 진입하는 케이스는 `await page.goto('/stack/templates')` 를 추가 명시 (이미 있는 케이스는 그대로 유지).
- 두 파일 모두 full run 수행 후 그린 확인.

### 검증
- `npx playwright test --grep @stack-critical` → 3/3 pass (새 Task 7 spec + 기존 2개).
- `npx playwright test web/e2e/stack-workflow.spec.ts web/e2e/stack-monitoring.spec.ts` 전체 pass.

### 체크리스트
- [ ] `web/e2e/stack-workflow.spec.ts` `beforeEach` 수정.
- [ ] `web/e2e/stack-monitoring.spec.ts` `beforeEach` 수정.
- [ ] 각 spec 의 케이스가 명시적으로 `/stack/*` 로 이동하는지 확인, 없으면 `await page.goto(...)` 추가.
- [ ] `@stack-critical` grep 기준 3/3 통과.
- [ ] CHANGELOG / plan §6 엔트리 갱신.

### Stop-and-verify
- `npx playwright test --grep @stack-critical` 그린 후 Phase 2 진입.

---

## Phase 2 — Audit 캡처 인터페이스 (`audit.Sink`) 도입

### 배경
Task 7 에서 `auditQuerier` 가 unexported 라서 E2E 에서 audit 엔트리 검증을 포기했음. 후속 phase (Phase 3 Retry API, Phase 4 updateStack) 모두 audit 기록을 포함하므로, 테스트 가능한 추상 레이어가 우선 필요.

### 조치
- `internal/shared/audit/sink.go` 신규:
  ```go
  type Sink interface {
      Log(ctx context.Context, entry AuditEntry) error
  }
  ```
- 기존 `*AuditLogger` 는 `Sink` 를 만족한다 (변경 없음, 컴파일 레벨 `var _ Sink = (*AuditLogger)(nil)` 만 추가).
- 테스트용 구현 `internal/shared/audit/memsink.go` 신규 (build constraint 없음, 테스트가 import 해서 쓸 수 있게):
  ```go
  type MemorySink struct { mu sync.Mutex; Entries []AuditEntry }
  func (m *MemorySink) Log(...) error { ... append ... }
  func (m *MemorySink) Snapshot() []AuditEntry { ... deep copy ... }
  ```
- 모든 핸들러/usecase 에서 `*AuditLogger` 받던 곳을 `audit.Sink` 로 교체. 타입 narrow 하게 유지. 영향 받는 파일:
  - `internal/stack/adapter/handler/deploy_handler.go` — `audit.Sink` 필드로.
  - `internal/admin/adapter/handler/cluster_handler.go` / `org_handler.go` — 있다면.
  - `cmd/api/main.go` — `audit.NewAuditLogger(pool)` 은 `*AuditLogger` 를 반환, Sink 인터페이스로 자연 수용 가능.
- **기존 `AuditLogger` 동작 변경 금지.** Export 대상은 `Sink` 인터페이스 하나뿐.

### 테스트
- `internal/shared/audit/sink_test.go` — MemorySink 기본 동작 (append, Snapshot isolation).
- `internal/stack/adapter/handler/deploy_handler_compat_test.go` 확장 — 기존 5 케이스에 `MemorySink` 를 주입해 `acknowledge_warnings=true`, `compatibility_verdict`, `issue_codes` 필드 검증 subtest 추가 (또는 인라인으로 확장).

### 체크리스트
- [ ] `audit.Sink` 인터페이스 + `MemorySink` 구현 + 단위 테스트.
- [ ] `DeployHandler` / 기타 audit 사용 핸들러가 `audit.Sink` 로 depend.
- [ ] `TestDeployHandler_Deploy_*` 기존 회귀 없음.
- [ ] `deploy_handler_compat_test.go` 에 audit 필드 검증 2~3 케이스 추가.
- [ ] CHANGELOG / plan §6 갱신.

### Stop-and-verify
- `go test ./...` + `go vet ./...` 그린 후 Phase 3 진입.

---

## Phase 3 — `F8-Retry-API`: `POST /api/v1/stacks/:id/retry`

### 배경
Task 7-E 가 검증한 `StateRolledBack` → `StatePending` 전이의 HTTP 노출. 프로덕션 UI 의 "Retry" 버튼이 직접 호출할 수 있어야 한다.

### 백엔드
- 라우트 등록: `v1.POST("/stacks/:id/retry", h.Retry)` on `DeployHandler`.
- `RetryHandler.Retry(c echo.Context)` 구현:
  1. Stack 로드. state 가 `failed` 또는 `rolled_back` 이 아니면 HTTP 409 `STACK_RETRY_INVALID_STATE`.
  2. Body `{"acknowledge_warnings": bool}` 파싱 (같은 `deployRequest` 구조 재사용).
  3. `ValidateCompatibility` persisted mode 재실행. 결과에 따라 `DEPLOY_COMPAT_FAIL` 400 / `DEPLOY_COMPAT_WARN_UNACK` 400 / 통과 분기 (deploy_handler 의 게이트와 동일 로직을 **재사용** 할 것, 중복 구현 금지 — 내부 헬퍼 `runPreDeployGate(ctx, stack, ack) (*verdictResp, *echo.HTTPError)` 로 추출해 Deploy/Retry 양쪽에서 호출).
  4. Stack state 를 `pending` 으로 전이 (`stack.TransitionTo(StatePending)`). 전이 실패 시 500.
  5. `InstallStack.Execute` 호출 (Deploy 와 동일).
  6. Audit: `operation=retry`, `compatibility_verdict`, `issue_codes`, `acknowledge_warnings`, `previous_state=failed|rolled_back`.
  7. 응답 202 + `{stackId, deploymentId, message}`.

### 프론트엔드
- `web/src/features/stack/api/stack-api.ts` — `retryStack(input: { stackId: string; acknowledgeWarnings?: boolean })` + `useRetryStack` 훅.
- `web/src/features/stack/pages/stack-deployment-logs-page.tsx` (또는 상세 패널) — state 가 `failed`/`rolled_back` 일 때만 "Retry" 버튼 렌더링. 클릭 시 서버 verdict 재검증 패턴(Install Wizard 와 동일) 재사용 — warn 발생 시 ack 체크박스 → 승인 후 재호출.
- 재사용 포인트: `shouldBlockOnServerVerdict` 순수 함수 + `extractDeployCompatError` 유틸 그대로 재활용.
- i18n: `stackList.retry.{label,warnAckLabel,confirm,success,failure}` (ko/en).

### 테스트
- `internal/stack/adapter/handler/retry_handler_test.go` 신규 — 5 케이스:
  1. failed → retry ack 없이 verified 조합 → 202, state pending → installing.
  2. rolled_back → retry ack 없이 verified → 202.
  3. completed state → 409 STACK_RETRY_INVALID_STATE.
  4. failed + warn 조합 + ack 없음 → 400 DEPLOY_COMPAT_WARN_UNACK.
  5. failed + warn + ack=true → 202 + audit `operation=retry`.
- `web/src/features/stack/api/stack-api.test.ts` 확장 — `retryStack` mutation shape.
- `web/src/features/stack/pages/stack-deployment-logs-page.test.tsx` (신규/확장) — Retry 버튼 가시성 state 매트릭스, 클릭 시 verdict 재검증 분기.

### 체크리스트
- [ ] `runPreDeployGate` 공통 헬퍼 추출 (Deploy/Retry 공유).
- [ ] Retry 핸들러 + 라우트 등록.
- [ ] Audit `operation=retry` 기록.
- [ ] Frontend `retryStack` + `useRetryStack` + Retry 버튼 UI.
- [ ] i18n ko/en.
- [ ] 5 handler 케이스 + 2 frontend 케이스 통과.
- [ ] CHANGELOG / plan §6 갱신 (F8-Retry-API → [x]).

### Stop-and-verify
- `go test ./...` + `pnpm -C web test --run features/stack` 그린 후 Phase 4 진입.

---

## Phase 4 — Orphan stack 정리 + `PUT /stacks/:id` (updateStack)

### 배경
F8-F3 에서 observe 된 구조적 gap: Install Wizard 가 `createStack → server validate` 순서이므로 server verdict `fail` 시 stack 이 persist 된 채로 남는다. 현재 `pendingStackId` 로 재시도 시 동일 stack 에 재검증만 하지만, 조합을 바꾼 재제출은 불가.

### 백엔드
- `PUT /api/v1/stacks/:id` 엔드포인트 신설.
- `UpdateStack` usecase 신규 (`internal/stack/usecase/update_stack.go`):
  - Input: `StackID, Name?, ClusterID?, Namespace?, Config?, Tools?, GoldenPathID?`.
  - State 제약: `state ∈ {pending, failed}` 에서만 허용. 그 외 409 `STACK_UPDATE_INVALID_STATE`.
  - Version 증가: `stack.Version += 1`, `UpdatedAt = now`.
  - History 기록: `ManageHistory.SaveVersion(prev config)` 으로 이전 config 를 백업.
- `StackHandler` 에 `Update(c echo.Context)` 핸들러 + `g.PUT("/:id", h.Update)`.
- Audit: `operation=update`, `before_hash`, `after_hash` (옵셔널).

### 프론트엔드
- `stack-api.ts` — `updateStack(input)` + `useUpdateStack`.
- `stack-install-page.tsx` submit 플로우 갱신:
  - `pendingStackId` 가 존재하면 `createStack` 대신 `updateStack` 호출.
  - 이후 validate → deploy 플로우는 기존과 동일.
  - Fresh submit 은 기존 경로 그대로.
- i18n: `stackInstall.update.{success,failure}` 최소한.

### 테스트
- `internal/stack/usecase/update_stack_test.go` 신규 — 3 케이스: pending 에서 업데이트 성공, failed 에서 성공 + history 기록, completed 에서 409.
- `internal/stack/adapter/handler/stack_handler_test.go` 에 Update 핸들러 3 케이스 추가.
- `web/src/features/stack/pages/stack-install-page.test.tsx` — pendingStackId 존재 시 updateStack 호출 경로 1 케이스.

### 체크리스트
- [ ] `UpdateStack` usecase + 단위 테스트.
- [ ] `PUT /stacks/:id` 핸들러 + 테스트.
- [ ] History 자동 백업.
- [ ] Install Wizard `pendingStackId` 재제출 시 updateStack 경로.
- [ ] Audit `operation=update`.
- [ ] CHANGELOG / plan §6 갱신 (orphan 정리 / updateStack → [x]).

### Stop-and-verify
- `go test ./...` + `pnpm -C web test --run features/stack` 그린 후 Phase 5 진입.

---

## Phase 5 — 매트릭스 CRUD UI + 백엔드 엔드포인트

### 배경
Admin 뷰(`/admin/stack-versions`) 는 현재 read 만 가능. 사내 전용 매트릭스 등록/수정/삭제는 UC 3 (§3) 의 명시 요구사항.

### 백엔드
- `CompatibilityRepository` 에 `Create` / `Update` / `Delete` 메서드가 이미 있는지 먼저 확인. 없으면 추가 (in-memory + postgres 양쪽, idempotent).
- 새 엔드포인트 (admin role 가드):
  - `POST /api/v1/admin/compatibility/matrices` → 신규 매트릭스 생성.
  - `PUT /api/v1/admin/compatibility/matrices/:id` → 업데이트.
  - `DELETE /api/v1/admin/compatibility/matrices/:id` → 삭제.
- 입력 검증:
  - `status ∈ {verified, untested, unsupported}`.
  - `tools[*].tier ∈ {stable, beta, deprecated}`.
  - `tools[*].archSupport` 배열 값이 `amd64|arm64` 중 하나 이상.
  - `minK8sVersion` semver 형식.
  - 중복 id 생성 → 409 `COMPAT_MATRIX_DUPLICATE`.
- 마이그레이션은 **불필요** (기존 `compatibility_matrices` 테이블 그대로).

### 프론트엔드
- `/admin/stack-versions` 페이지 확장:
  - 헤더 "Create Matrix" 버튼 → `MatrixEditModal` 오픈.
  - 각 row 행동 버튼: 연필(수정) / 휴지통(삭제 + 확인 다이얼로그).
- `MatrixEditModal` 컴포넌트 신규 (`features/admin/components/matrix-edit-modal.tsx`):
  - 필드: id (생성 시), k8sVersionRange, status, tools 배열 (동적 추가/삭제). 각 tool: name, category, helmVersion, appVersion, minK8sVersion, archSupport checkbox 그룹, tier select.
  - 저장 시 `useCreateMatrix` / `useUpdateMatrix` mutation.
- 삭제: `useDeleteMatrix` mutation + 확인 다이얼로그 ("삭제 시 기존에 이 매트릭스를 사용한 스택은 untested 로 간주됩니다" 경고).
- 기존 `useRefreshDiscovery` 와 동일 패턴으로 캐시 invalidate (`useCompatibilityMatrix`, `useCompatibilityMatrices`).
- i18n 네임스페이스 `stackVersionsAdmin.edit.*` 확장.

### 테스트
- Backend: `compatibility_handler_admin_test.go` 신규 — 6 케이스 (create 성공 / duplicate 409 / update 성공 / update not-found 404 / delete 성공 / non-admin 403).
- Frontend: `matrix-edit-modal.test.tsx` — 폼 validation + 제출 플로우 3~4 케이스.

### 체크리스트
- [ ] `CompatibilityRepository.Create/Update/Delete` (in-memory + postgres).
- [ ] Admin CRUD 핸들러 3종 + role 가드.
- [ ] 입력 검증 로직.
- [ ] `MatrixEditModal` + 페이지 통합.
- [ ] 삭제 확인 다이얼로그.
- [ ] i18n ko/en.
- [ ] Handler 6 케이스 + Frontend 4 케이스 통과.
- [ ] CHANGELOG / plan §6 갱신 (매트릭스 CRUD UI → [x]).

### Stop-and-verify
- `go test ./...` + `pnpm -C web test --run features/admin` 그린 후 Phase 6 진입.

---

## Phase 6 — 서버 verdict 캐시

### 배경
F8-F3 도입 후 Install Wizard 가 submit 마다 서버 validate 를 호출. 동일 stack state (tools + cluster) 는 짧은 시간 내 동일 verdict 를 만들 것이므로 반복 연산/DB 조회가 낭비.

### 조치
- `internal/stack/usecase/verdict_cache.go` 신규:
  ```go
  type VerdictCache interface {
      Get(key string) (*ValidateCompatibilityOutput, bool)
      Put(key string, out *ValidateCompatibilityOutput)
      Invalidate(prefix string)
  }
  ```
- 기본 구현 `MemoryVerdictCache` — `sync.Map` + per-entry `expiresAt time.Time`. TTL 30s (상수, env 오버라이드 지원 `VERDICT_CACHE_TTL_SEC`).
- Key 계산: `sha256(stackID || "|" || sortedToolsJSON || "|" || clusterID || "|" || sortedArchsCSV || "|" || matrixHash)` — matrix drift 도 invalidation 트리거.
- `ValidateCompatibility` 에 `WithVerdictCache(VerdictCache)` 옵션 추가. 주입 안 되면 캐시 비활성 (backward-compat).
- 무효화 트리거:
  - `UpdateStack` 성공 시 `cache.Invalidate("stack:"+id)`.
  - `ClusterUseCase.RefreshDiscovery` 성공 시 `cache.Invalidate("cluster:"+id)`.
  - `CompatibilityRepository.Update/Delete` 성공 시 `cache.Invalidate("matrix:"+id)`.
  - 단, 현재 key 구조는 prefix 매칭이 복잡하니 **처음에는 전역 clear** 로 단순화 (TTL 이 30s 이므로 drift 위험 작음). 개선은 후속.
- Metric: `nullus_verdict_cache_hits_total` / `_misses_total` (Prometheus counter 가 이미 있으면 재사용, 없으면 noop).

### 테스트
- `internal/stack/usecase/verdict_cache_test.go` — hit/miss, TTL 만료, concurrent access 3 케이스.
- `validate_compatibility_test.go` 확장 — 캐시 hit 시 두 번째 호출은 repository 를 치지 않음 (mock repo 호출 횟수 assert).

### 체크리스트
- [ ] `VerdictCache` 인터페이스 + `MemoryVerdictCache`.
- [ ] `ValidateCompatibility.WithVerdictCache` 옵션.
- [ ] 전역 invalidation hook on UpdateStack / RefreshDiscovery / matrix CRUD.
- [ ] TTL 30s + env override.
- [ ] 단위 테스트 3 케이스 + 통합 1 케이스.
- [ ] CHANGELOG / plan §6 갱신 (verdict 캐시 → [x]).

### Stop-and-verify
- `go test ./...` 그린 + benchmark 옵션 `go test -bench=BenchmarkValidateCompatibility -benchtime=1s ./internal/stack/usecase` 실행 결과를 보고에 포함. Phase 7 진입.

---

## Phase 7 — Nightly Refresh Discovery 스케줄러

### 배경
Cluster 의 node_architectures 는 노드 추가/삭제/업그레이드로 drift 한다. 수동 Refresh Discovery 만으로는 모든 stack 의 arch 판단이 stale 해질 수 있음.

### 조치
- `internal/admin/scheduler/refresh_discovery.go` 신규:
  ```go
  type RefreshDiscoveryScheduler struct {
      clusterUC *ClusterUseCase
      interval  time.Duration
      clock     func() time.Time
      logger    *slog.Logger
  }
  func NewRefreshDiscoveryScheduler(uc, interval, opts...) *RefreshDiscoveryScheduler
  func (s *RefreshDiscoveryScheduler) Start(ctx context.Context)
  ```
- 구동: `cmd/api/main.go` 에서 서버 시작 시 `go scheduler.Start(ctx)` 호출. `ctx` 는 signal handler 가 관리하는 서버 생명주기.
- 주기: 기본 24h, env `REFRESH_DISCOVERY_INTERVAL` 으로 오버라이드.
- 동작: 매 tick 마다 `clusterRepo.List` → 각 cluster 에 대해 `uc.RefreshDiscovery(ctx, id)` 호출. 실패는 로그만 남기고 계속. 한 iteration 전체 타임아웃 5분.
- 중복 실행 방지: `sync.Mutex` 혹은 `atomic.Bool` 로 이전 iteration 이 진행 중이면 skip.
- Graceful shutdown: `ctx.Done()` 수신 시 즉시 중단.

### 테스트
- `internal/admin/scheduler/refresh_discovery_test.go` — fake clock 기반 4 케이스:
  1. 첫 tick 에서 모든 cluster RefreshDiscovery 호출.
  2. 한 cluster 실패 시 나머지는 계속 진행.
  3. 이전 iteration 이 running 이면 다음 tick skip.
  4. ctx cancel 시 즉시 종료.
- 통합: `cmd/api/main.go` 에서 스케줄러가 boot-time panic 없이 goroutine 으로 기동되는지 smoke (현재 `main_test.go` 가 있다면 확장, 없다면 skip).

### 체크리스트
- [ ] `RefreshDiscoveryScheduler` 구현 + fake clock 테스트.
- [ ] `main.go` 에서 startup 시 기동 + graceful shutdown.
- [ ] 환경변수 오버라이드.
- [ ] 실패 로그 + 다음 iteration 영향 없음 검증.
- [ ] CHANGELOG / plan §6 갱신 (nightly refresh cron → [x]).

### Stop-and-verify
- `go test ./...` 그린. 최종 `go vet ./...` + `go vet -tags e2e ./...` + `pnpm -C web test` + `npx tsc --noEmit` 전체 그린 후 전체 프롬프트 완료 보고.

---

## 마지막 종합 보고 포맷 (Phase 1~7 누적)

1. **Phase 별 commit SHA 또는 요약** — 7개 phase 각각의 변경 파일 개수, 주요 산출물, 실행 결과.
2. **누적 테스트 결과** — Go 패키지 수 / 프론트엔드 테스트 수 / Playwright `@stack-critical` 개수 / 신규 마이그레이션(없음 예상) 여부.
3. **미완료 / 드롭된 phase** — 있다면 원인과 회복 계획. 없으면 `none`.
4. **남은 follow-up** — 본 프롬프트 범위 밖. 대표적으로 `F8-F6-Cloud`, 프로덕션 테스트 인프라 강화, verdict 캐시 per-prefix invalidation 세분화 등.
5. **이슈 발견 기록** — 진행 중 발견한 pre-existing 결함이 있으면 memory 보고용으로 목록화.

---

## 참고 포인터

- Audit 기존 구현: `internal/shared/audit/logger.go` — `auditQuerier` interface, `AuditEntry` struct, `Log(ctx, entry) error`.
- Deploy 게이트: `internal/stack/adapter/handler/deploy_handler.go` `runPreDeployGate` 로 추출할 코드가 현재 `Deploy()` 내 인라인.
- State machine: `internal/stack/domain/stack.go` `validTransitions`. `failed → pending`, `rolled_back → pending` 이미 허용됨.
- Install Wizard submit 플로우: `web/src/features/stack/pages/stack-install-page.tsx` — `pendingStackId` 는 이미 F8-F3 에서 도입.
- 매트릭스 시드: `internal/stack/adapter/repository/memory_compatibility.go` `defaultCompatibilityMatrices()` — Create/Update/Delete 추가 시 이 함수와 충돌하지 않게.
- Scheduler 레퍼런스: 저장소 내에 `time.NewTicker` 기반 패턴이 `internal/stack/adapter/helm/orchestrator.go` 등에 있음. 동일 패턴 재사용.
- Kind E2E 관련 주의사항: multi-subtest Kind 배포는 cluster-scoped resource leak 때문에 subtest 사이 `kind delete && kind create` 필수 (Task 6 런북 참조). **본 프롬프트 phase 들은 kind 를 요구하지 않는다** — 유의점 공유 차원.

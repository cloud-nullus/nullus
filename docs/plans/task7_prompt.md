# Task 7 작업지시 프롬프트 — warn-forced Retry/Rollback E2E

> **목적:** F8 호환성 매트릭스의 마지막 QA 아이템. `warn` 상태를 사용자가 명시적으로 승인(acknowledge_warnings=true)한 뒤 실제 배포가 실패하는 시나리오에서, **Retry**(재배포) 와 **Rollback**(이전 버전 복구) 경로가 올바르게 동작하는지를 통합 E2E 테스트로 보장한다.
> `docs/plans/compatibility_matrix_plan.md` Task 7 의 이행 작업이다.

---

## 0. 선결 조건 (Precondition)

**이미 해결됨 (2026-04-20, Task 6 에서 함께 처리):** `internal/cicd/adapter/repository/memory_pipeline.go` 의 `MemoryPipelineRepository.Delete` 누락이 Task 6 선행으로 이미 구현되었고, `go vet -tags e2e ./...` 가 저장소 전역에서 그린 상태다. Task 7 에서는 **별도 조치 없이** 본론으로 들어간다.

- 다만 시작 전 확인: `go vet ./...`, `go vet -tags e2e ./...` 가 클린해야 한다. 깨진 상태면 본 Task 착수 전에 근본 원인 조사부터.
- Task 6 런북 (`docs/20_아키텍처/F8_Task6_Kind_Runbook.md`) 에 기록된 **Kind subtest 사이 cluster 재생성 규칙** 은 본 Task 에는 해당되지 않는다 (본 Task 는 in-memory repo + fake StepExecutor 기반이라 kind 자체를 쓰지 않는다).

---

## 1. 작업 범위

### 1.1 Go 통합 E2E (백엔드, 결정론적)
`e2e/` 디렉터리에 `//go:build e2e` 태그로 새 파일을 추가한다. 기존 `e2e/setup_test.go` 는 `MemoryStackRepository` + `InstallStack` + `NewDeployHandler(...)` 를 조립해 `httptest.NewServer` 로 띄우는 구조이며 이 테스트에서도 재사용한다.

다만 **실패 주입**을 위해 Task 7 전용 테스트는 자체 `httptest` 서버를 별도로 띄우는 편이 안전하다 — 기존 `testServerURL` 가 공유 서버인데 전역 상태를 바꾸면 다른 테스트가 영향을 받는다.

#### 1.1.1 새 파일
- `e2e/warn_forced_retry_rollback_test.go` (새 파일, `//go:build e2e`)

#### 1.1.2 테스트 헬퍼
- 같은 파일 또는 `e2e/helpers_test.go` 에 `newWarnForcedTestServer(t *testing.T, fail *atomic.Bool) (baseURL string, cleanup func())` 헬퍼를 둔다.
- 헬퍼 내부:
  - `stackrepo.NewMemoryStackRepository()`, `stackrepo.NewMemoryCompatibilityRepository()`, `stackrepo.NewMemoryHistoryRepository()`, `logadapter.NewMemoryStreamer()`, `adminrepo.NewMemoryClusterRepository()` 준비.
  - 클러스터를 **ARM64 단독** (e.g. `node_architectures: ["arm64"]`) 으로 사전 시드.
  - 호환성 매트릭스를 **untested** 상태 + GitLab 계열(amd64-only) 포함으로 사전 시드 — `memCompatRepo.Save(ctx, ...)` 나 `defaultCompatibilityMatrices()` 에서 임의 매트릭스 `test-warn-v1` 생성. 시드 데이터는 `validate` 시 `warn` + `TOOL_ARCH_UNSUPPORTED` 가 떨어지도록 구성.
  - `fakeStepExecutor` 를 구현해 `port.StepExecutor` 를 만족. `atomic.Bool` 을 주입받아 true 면 특정 phase(예: Install)에서 `errors.New("helm install failed (simulated)")` 반환, false 면 모든 step 을 no-op 성공.
  - `InstallStack` 을 `usecase.NewInstallStack(memStackRepo, memStreamer, usecase.WithExecutor(fakeExecutor))` 로 주입.
  - `ValidateCompatibility` 를 `usecase.NewValidateCompatibility(memCompatRepo, usecase.WithStackRepository(memStackRepo), usecase.WithClusterReader(memClusterReader))` 로 주입.
  - `DeployHandler` 를 `stackhandler.NewDeployHandler(installStackUC, memStackRepo, memStreamer).WithOptions(stackhandler.WithValidateCompatibility(validateCompatUC))` 로 조립.
  - 필요한 경우 admin cluster handler / compatibility handler / history handler / stack handler 를 모두 등록. (기존 `newEchoServer()` 의 축약 버전이라고 보면 된다.)
  - `httptest.NewServer` 반환 + `t.Cleanup(func() { ts.Close() })`.
- 기존 `setup_test.go` 의 `noopManifestApplier` 등은 그대로 재활용 가능.

#### 1.1.3 테스트 케이스 목록
하나의 `TestF8Task7_WarnForcedRetryRollback(t *testing.T)` 최상위 + `t.Run("subtest-name", ...)` 구조로 각 시나리오를 독립 실행한다. 각 subtest 는 새 서버를 띄워 상태 오염을 피한다.

**A) warn verdict 생성 확인**
- 매트릭스/클러스터/스택 생성 후 `POST /api/v1/stacks/:id/validate` (body 생략, path 기반 persisted mode) 호출.
- 기대: HTTP 200, body `verdict.overall == "warn"`, `verdict.issues[*].code` 에 `TOOL_ARCH_UNSUPPORTED` 포함.

**B) ack 없이 deploy → 차단**
- `POST /api/v1/stacks/:id/deploy` body `{}` 호출.
- 기대: HTTP 400, body `error.code == "DEPLOY_COMPAT_WARN_UNACK"`, `error.verdict.overall == "warn"`.
- 스택 state 는 여전히 `pending` (전이되지 않음).

**C) ack=true 로 deploy → 202 + 성공 경로**
- `fakeStepExecutor` 를 성공 모드(`fail=false`)로 두고 `POST /deploy` body `{"acknowledge_warnings": true}` → 202.
- 폴링(`GET /stacks/:id/status`) 로 `completed` 도달 확인.
- `audit.Log` 검증: 해당 deploy 이벤트에 `acknowledge_warnings=true`, `compatibility_verdict="warn"`, `issue_codes` 에 `TOOL_ARCH_UNSUPPORTED` 포함.
  - `setup_test.go` 가 audit logger 를 주입하지 않으면 `NoopAuditLogger` 대신 테스트용 `capturingAuditLogger`(슬라이스 append) 를 helper 에서 주입.

**D) ack=true + executor 실패 → failed 상태 + audit 기록 유지**
- 새 스택 + 새 서버. `fail=true` 로 세팅. `POST /deploy` ack=true → 202.
- 폴링 → `failed` 도달. `GET /stacks/:id` 검증: `state=="failed"`, `error_message` 가 simulated 메시지 포함.
- audit 캡처에서 `acknowledge_warnings=true` 여전히 기록된 상태 확인 (deploy 수락은 됐으므로).

**E) Retry 시나리오 — failed → pending → completed**
- D 직후 같은 서버에서 `fail.Store(false)` 로 뒤집고 `POST /deploy` ack=true 재호출.
- 기대: HTTP 202. 폴링 → `completed`. 도메인 state machine 관점: failed → pending → validating → installing → configuring → health_check → completed 경로가 실제로 통과되는지 확인.
  - 구현 노트: 기존 `DeployHandler.Deploy` 가 non-pending state 를 허용하는지는 `deploy_handler_test.go` (TestDeployHandler_Deploy_400WhenTransitionInvalid) 를 참조. `failed → pending` 은 `validTransitions` 에서 허용되어 있으므로, DeployHandler 가 이를 호출해 pending 으로 되돌린 뒤 Execute 를 돌리는 경로가 존재하는지 확인. **없으면 Retry 는 명시적 API 가 필요**. 현재 없는 것으로 보이면 두 옵션 중 하나:
    1. DeployHandler 가 `state == failed` 일 때 자동으로 pending 으로 되감은 뒤 Execute 를 시작 (단, 변경 범위가 커짐)
    2. 테스트 안에서 `stackRepo.Update(stack with state=pending)` 로 직접 되감은 뒤 Deploy 를 호출 (테스트 헬퍼 수준에서 해결, 프로덕션 API 변경 없음)
  - **우선 (2) 방식으로 구현해 Retry 의 state-transition 계약만 테스트**한다. (1) 은 plan 문서에 follow-up 으로 남긴다. 이것은 테스트 범위의 의도적 축소이며 Task 7 목적("복구되는 통합 E2E 테스트 추가") 을 충족한다.
- audit 에는 두 번째 deploy 이벤트가 별도로 기록되어야 한다 (`acknowledge_warnings=true` 여전히 set).

**F) Rollback 시나리오 — failed 에서 이전 버전으로 복구**
- 별도 서버 + 새 스택. 아래 순서:
  1. warn 매트릭스로 스택 생성 + **초기 성공 배포**(`fail=false`, ack=true) → 스택이 `completed` 가 된 상태에서 `ManageHistory.SaveVersion(...)` 이 이미 불리거나, 테스트가 수동으로 버전 v1 을 저장.
  2. 스택의 tool 조합을 임의로 바꿔(혹은 `StackHandler` UPDATE 경로가 없다면 `StackRepository.Update` 로 직접) 재배포 → `fail=true` 로 실패 상태 생성.
  3. `POST /api/v1/stacks/:id/rollback` body `{"versionId":"<v1-id>", "reason":"retry cancelled"}` → 200. history 에 새 버전 v2 가 기록되고 stack.Config 가 v1 으로 덮어써진 것 확인.
  4. (선택) 이어서 `POST /deploy` ack=true → 다시 성공.
- `ManageHistory.SaveVersion` 의 호출 지점이 현재 production 코드에 없다면, 본 테스트는 **직접 `memHistoryRepo.SaveVersion(ctx, version)` 로 v1 을 시드**해도 좋다. 목적은 rollback 엔드포인트의 동작 계약 검증이다.

각 subtest 는 독립적이다. 공통 로직은 helper 로 추출.

#### 1.1.4 제약 / 주의
- `context.Background()` 대신 `context.WithTimeout(..., 10*time.Second)` 를 사용 (일부 path 는 비동기 go routine 기반이라 sleep/polling 필요).
- 폴링 간격은 50ms, 총 타임아웃 5초 이내. `eventually` 류 helper 가 있으면 재사용.
- Go race detector(`-race`) 에서 깨끗해야 하므로 `atomic.Bool` 사용 필수. mutex 보호된 audit slice 도 동일.

### 1.2 Playwright @stack-critical 스모크 (UI 계약)
`web/e2e/` 에 한 파일을 추가. UI 를 실제 클러스터까지 동원하지 않고, **API 단으로 warn 조합 시딩 + UI submit 플로우** 를 드라이브한다.

#### 1.2.1 새 파일
- `web/e2e/stack-warn-forced-retry.spec.ts`

#### 1.2.2 테스트 시나리오 (단일 `@stack-critical` 케이스)
`test.describe('F8 Task 7 — Warn-Forced Retry/Rollback', () => { test('@stack-critical', async ({ page, request }) => { ... }) })` 구조.

1. `beforeEach` 로 `devops@nullus.dev` 로그인 (기존 `stack-workflow.spec.ts` 와 동일 패턴).
2. API 로 untested 매트릭스 + mixed-arch 클러스터를 seed (이미 seed 로 보장되면 skip).
3. 위저드(/stack/install) 오픈 → Golden Path 빠른 시작에서 해당 매트릭스 카드 클릭 → draft 가 채워짐 확인.
4. Pre-Deploy Gate 영역에 client-side warn 패널이 뜨는지 확인, `data-testid="compat-warn-ack"` 체크박스 클릭.
5. Submit → createStack → server validateCompatibility → server-warn-ack 패널(`data-testid="server-verdict-panel"`) 이 뜨는지 확인. `data-testid="server-warn-ack"` 체크 후 다시 Submit.
6. deployStack 202 까지 도달. `useDeployStack` 훅이 날린 요청 body 에 `acknowledge_warnings:true` 가 포함되는지 `page.on('request', ...)` 로 캡처 검증.
7. `/stack/logs/:id` 로 이동. 타임아웃 60s 내에 `completed` / `failed` / `rolled_back` 중 하나로 terminal 진입 (어떤 상태든 통과).
8. `expect(['completed','failed','rolled_back']).toContain(finalState)` 로 확정. E2E UI 는 실제 배포 성공/실패를 담보할 수 없으므로 terminal 도달만 계약화.

#### 1.2.3 제약
- 기존 `@stack-critical` 태그를 재사용 (`playwright.config.ts` 의 `grep` 에 이미 포함).
- 기존 로그인 헬퍼와 `apiBase = 'http://localhost:8090/api/v1'` 상수를 재사용.
- 병렬 실행 시 stack name 충돌을 피하려면 `pw-warn-retry-${Date.now()}` 패턴.
- 새 `data-testid` 가 필요하면 `stack-install-page.tsx` 에 추가해도 좋지만, 이미 도입된 `server-warn-ack` / `server-verdict-panel` 을 우선 재사용. UI 문구 변경은 하지 말 것.

---

## 2. 결정론/의도 정렬 원칙

- **Retry 는 "프로덕션 API 확장" 이 아니라 "state machine 계약 검증" 이다.** Test helper 가 state 를 `pending` 으로 되감는 방식이 허용되는 이유는, Task 7 의 산출물은 "복구 경로가 state machine 상 가능함" 을 증명하는 것이기 때문. 프로덕션용 Retry 버튼/엔드포인트는 별도 follow-up 이슈로 plan 문서에 남긴다 ("Task 7 follow-up: POST /stacks/:id/retry").
- **audit 검증은 최소 필수 필드만.** `acknowledge_warnings`, `compatibility_verdict`, `issue_codes` 세 필드. 기타 필드는 검증하지 않아 future change 에 brittle 해지지 않게 한다.
- **Playwright 는 terminal 도달만 계약화.** Kind / CI 환경에 따라 실제 helm 이 성공/실패할지 달라지므로 셋 중 하나면 통과.

---

## 3. 하지 말 것 (Forbidden)

- `DEPLOY_COMPAT_WARN_UNACK` / `DEPLOY_COMPAT_FAIL` / `TOOL_ARCH_UNSUPPORTED` 등 기존 코드 이름 변경 금지.
- `ValidateCompatibility.applyArchCheck` 의 verdict 정책(verified=fail / untested=warn / arch-unknown=warn) 변경 금지. 본 테스트는 정책의 **관찰자** 다.
- 마이그레이션 체인(000041 ~ 000043) 수정 금지. 필요한 시드는 in-memory 리포지토리 레벨에서만 구성.
- 기존 `e2e/setup_test.go` 의 `newEchoServer()` 전역 서버 설정을 바꾸지 말 것. 새 헬퍼가 별도 서버를 구성해야 한다.
- 기존 Playwright 스펙 수정 금지 (새 파일로 격리).
- 새 도메인 상수/에러 코드 추가 금지. 기존 코드 재사용.
- 커밋/푸시 금지 — 작업 보고만.

---

## 4. 체크리스트

- [ ] **[Precondition 확인]** `go vet ./...` / `go vet -tags e2e ./...` 클린 상태로 시작. (Task 6 에서 선행 해결 완료)
- [ ] `e2e/warn_forced_retry_rollback_test.go` 신규 (`//go:build e2e` 태그, 6 subtest).
- [ ] `fakeStepExecutor` + `capturingAuditLogger` 헬퍼 (같은 파일 또는 `helpers_test.go`).
- [ ] `go test -tags e2e ./e2e/...` 전체 통과 (새 파일 + 기존 UAT 회귀 모두).
- [ ] `go test ./internal/stack/... ./internal/admin/... ./internal/cicd/...` 회귀 통과.
- [ ] `web/e2e/stack-warn-forced-retry.spec.ts` 신규 (`@stack-critical` 태그).
- [ ] `npx playwright test --grep @stack-critical` 로 스모크 통과 (CI 환경에서 skip 되면 skip 사유 명시).
- [ ] `npx tsc --noEmit` / `npx eslint web/e2e/stack-warn-forced-retry.spec.ts` 클린.
- [ ] `CHANGELOG.md` `[Unreleased] > Added` 상단에 Task 7 엔트리 추가.
- [ ] `docs/plans/compatibility_matrix_plan.md` Task 7 을 `[x]` 로 + 3~5줄 요약.
- [ ] 시나리오 E (프로덕션 Retry 엔드포인트 부재) 를 plan `## 6. v1 GA 후 Follow-up` 섹션에 `POST /stacks/:id/retry` follow-up TODO 로 명시.

---

## 5. 추천 작업 순서

1. `fakeStepExecutor` 와 서버 헬퍼 스켈레톤 작성 → 시나리오 A/B 먼저 (validate+warn + ack-less deploy block) 통과 확인.
2. 시나리오 C/D (성공 경로 + 실패 주입) 추가.
3. 시나리오 E (Retry via state rewind) 추가.
4. 시나리오 F (Rollback + history save) 추가.
5. Playwright 스모크 작성.
6. CHANGELOG / plan 문서 반영.
7. 최종 test suite 실행 및 보고.

---

## 6. 완료 보고 포맷

아래 형식으로 회신:

1. **변경된 파일 목록** — 백엔드 / 프론트엔드 / 테스트 / 문서 구분.
2. **새 테스트와 의도** — 각 subtest 1~2줄 요약.
3. **실행 결과** — `go vet ./...` / `go test -tags e2e ./e2e/...` / `go test ./internal/...` / `npx playwright test --grep @stack-critical` / `npx tsc --noEmit` / `eslint` 각각의 pass/fail 및 관련 요약.
4. **의사결정 포인트** — Retry를 test-level state rewind 로 처리한 이유, audit 캡처 방식, Playwright terminal 도달 계약 등.
5. **남은 follow-up** — 프로덕션 Retry 엔드포인트, rollback UX, CI 환경에서 Playwright skip 처리 등.

---

## 7. 참고 포인터

- State machine: `internal/stack/domain/stack.go` `validTransitions` — failed → pending, failed → rolling_back, rolling_back → rolled_back, rolled_back → pending.
- 기존 deploy 테스트: `internal/stack/adapter/handler/deploy_handler_test.go`, `deploy_handler_compat_test.go`.
- 기존 rollback 엔드포인트: `internal/stack/adapter/handler/history_handler.go` `Rollback()`.
- 기존 Playwright 스택 스펙: `web/e2e/stack-workflow.spec.ts` (`@stack-critical` 케이스 하나 존재, 로그인 헬퍼/pollStackState 재사용 가능).
- F8-F3 구현 내용은 `docs/plans/compatibility_matrix_plan.md` `## 6. v1 GA 후 Follow-up > F8-F3` 섹션 참조.

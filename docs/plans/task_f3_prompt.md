# F8-F3 — Deploy 단계 서버측 호환성 재검증 (Server-side Pre-Deploy Gate)

> Claude CLI 에 그대로 붙여넣을 수 있는 단일 프롬프트.
> 사용 시 `cd /path/to/cloud-nullus/draft` 이동 후 `claude` 실행 → 이 문서 전체 복사.

---

## 배경 (반드시 먼저 읽기)

Nullus Platform F8 (DevSecOps Stack OSS 버전 호환성 관리) 의 follow-up 으로, **Task 5 작업 보고에서 드러난 구조적 gap** 을 메우는 작업이다.

현재 Install Wizard 는 draft 단계에서는 stackId 가 존재하지 않아 서버 `/stacks/:stackId/validate` 를 호출하지 못하고, 클라이언트 측 `isMatrixCompatibleWithCluster` / `matrixArchMismatches` (in `web/src/features/stack/utils/compatibility-arch.ts`) 로 동등 로직만 수행한다. 또한 `POST /stacks/:id/deploy` 핸들러 (`internal/stack/adapter/handler/deploy_handler.go`) 는 **서버측 호환성 게이트가 전혀 없이** 곧장 `InstallStack.Execute` 를 부른다. 즉 UI 를 우회한 직접 API 호출 / 자동화 클라이언트 / 구버전 클라이언트가 fail 조합을 강행 배포할 수 있다.

F8-F3 의 목표는:

1. Deploy 직전 (stack 이 persist 된 이후) **서버가 stack.Tools + stack.ClusterID 기반으로 Pre-Deploy Gate 를 재실행**하고, `fail` 은 하드 블록, `warn` 은 명시적 ack 플래그가 없으면 블록.
2. Install Wizard 가 `createStack` 직후 `deployStack` 직전에 같은 엔드포인트를 호출해 **서버 verdict 을 한 번 더 사용자에게 제시**하고 (draft 단계 클라이언트 판단과 다를 경우 대비), 필요 시 ack 를 받아 deploy 에 전달.

배경 문서:

- `docs/plans/compatibility_matrix_plan.md` (Task 1~5 완료 상태)
- `docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md`
- `internal/stack/usecase/validate_compatibility.go` — `applyArchCheck` 정책 (verified+miss=fail / untested+miss=warn / unknown=warn).
- `internal/stack/adapter/handler/compatibility_handler.go` — `POST /stacks/:stackId/validate` 현재 핸들러. `:stackId` path 파라미터가 현재 사용되지 않음 (body 의 tools 만 사용). 본 작업에서 이 불일치를 수정.
- `internal/stack/adapter/handler/deploy_handler.go` — `POST /stacks/:id/deploy` 현재 게이트 없음.
- `internal/stack/domain/stack.go` — `Stack.Tools []ToolConfig`, `Stack.ClusterID string` 보유 (서버가 tools 을 load 할 수 있음).
- `web/src/features/stack/pages/stack-install-page.tsx` — Deploy 버튼 클릭 플로우 (약 line 3540~3560 부근 `createStack.mutateAsync` → `deployStack.mutateAsync`).

---

## 1. 백엔드 — Deploy Handler Pre-Deploy Gate

### 1.1 Use Case 확장: 본체에서 persisted stack 로딩 허용

`internal/stack/usecase/validate_compatibility.go` 의 `ValidateCompatibilityInput` 에 두 가지 모드를 공식화:

- **Explicit mode** (기존): `Tools map[string]string` 이 비어 있지 않으면 그 값으로 검증.
- **Persisted mode** (신규): `Tools` 가 비어 있고 `StackID` 가 제공되면 use case 가 `StackRepository.GetByID(ctx, StackID)` 로 stack 을 로드해 `stack.Tools` 를 `map[category]name` 으로 변환한 뒤 검증. 이때 `ClusterID` 가 비어 있으면 `stack.ClusterID` 를 기본값으로 사용.

변경점:

- `ValidateCompatibilityInput` 에 `StackID string` 필드 추가.
- 생성자 `NewValidateCompatibility` 에 `stackReader` port (기존 stack repo 읽기만 필요하므로 `port.StackRepository` 로 충분. 단 cyclic import 회피를 위해 사용중인 port 를 재사용) 를 DI.
- 비어 있는 tools + 비어 있는 stackID → 기존처럼 `"tools or stack_id required"` 오류 반환.
- tools 와 stackID 둘 다 제공된 경우: body 의 `tools` 가 우선 (explicit 이 persisted 를 override).
- unit test 확장: (a) stackID 기반 로딩 성공 경로, (b) stackID 불일치 시 에러, (c) stack.ClusterID fallback 시 arch 체크까지 구동.

### 1.2 Compatibility Handler: `:stackId` path 바인딩 정비

`compatibility_handler.go` `Validate` 함수:

- `c.Param("id")` (또는 현재 라우트 param 이름) 을 읽어 `req.StackID` 가 비어 있을 때 채워 넣는다 (path → body 보충).
- 라우트 주석을 실제 path (`/stacks/:id/validate`) 에 맞춰 수정. 기존 `POST /compatibility/validate` 표현은 제거 또는 "legacy" 명시.

### 1.3 Deploy Handler 에 게이트 삽입

`internal/stack/adapter/handler/deploy_handler.go`:

- `NewDeployHandler` 의존성에 `*usecase.ValidateCompatibility` 추가.
- `Deploy(c echo.Context)` 본문:
  1. request body 를 파싱해 `AcknowledgeWarnings bool` (`json:"acknowledge_warnings"`) 를 추출. body 가 완전히 비어 있으면 기본 false.
  2. `validateCompatibility.Execute(ctx, ValidateCompatibilityInput{StackID: id})` 호출. persisted mode 로 동작.
  3. verdict 분기:
     - `overall.state == "fail"` → `errorResponse(c, 400, "DEPLOY_COMPAT_FAIL", ...)` 반환. 이때 응답 body 에 verdict 세부 (`overall`, `issues`, `node_architectures`) 포함해 프론트가 동일 포맷으로 재활용 가능하게 한다.
     - `overall.state == "warn"` 이고 `!AcknowledgeWarnings` → `errorResponse(c, 400, "DEPLOY_COMPAT_WARN_UNACK", ...)`. body 에 verdict 포함. 프론트가 이 에러를 받으면 "warn 확인 → ack 체크 → 다시 submit" 루프로 진입.
     - `overall.state == "pass"` 또는 warn+ack → 그대로 `installStack.Execute` 진행.
  4. 기존 `audit.Log` 에 `"compatibility_verdict": out.Overall.State`, `"issue_codes": [...]` 를 details 에 추가 (관측 용이).
- Error response 헬퍼는 JSON body 에 `verdict` 객체 포함. 기존 `errorResponse` 가 구조 고정이면 전용 helper `deployGateErrorResponse` 를 새로 작성.
- wiring: `cmd/api/main.go` 의 DeployHandler 생성자에 ValidateCompatibility use case 주입 (이미 compat handler 쪽에 인스턴스 존재하므로 같은 것 공유).

### 1.4 테스트

`deploy_handler_test.go` 확장 (새 파일 `deploy_handler_compat_test.go` 로 분리해도 OK):

- **Pass path**: stack 이 verified 매트릭스 조합 + 호환 cluster arch → 202 Accepted 유지.
- **Fail path**: verified 매트릭스 + arm64 only cluster → 400 `DEPLOY_COMPAT_FAIL`. response body 에 `verdict.issues[0].code == "TOOL_ARCH_UNSUPPORTED"`.
- **Warn unack**: untested 매트릭스 + arm64 only cluster + `acknowledge_warnings` 미포함 → 400 `DEPLOY_COMPAT_WARN_UNACK`. `verdict.overall.state == "warn"`.
- **Warn ack**: 같은 조합 + `{"acknowledge_warnings": true}` → 202 Accepted.
- **Cluster arch unknown**: stack.ClusterID 의 cluster.NodeArchitectures 빈 슬라이스 → `CLUSTER_ARCH_UNKNOWN` warn. ack 없을 때 블록, 있을 때 통과.
- `validate_compatibility_test.go` 에 persisted mode 3 케이스 추가 (§1.1 요약 참조).

### 1.5 범위 밖 (하지 말 것)

- 매트릭스 스키마 (`compatibility_matrices`) 변경.
- `ValidateCompatibility.Execute` 의 verdict 계산 정책 수정 (arch miss 분기 그대로 유지).
- `InstallStack.Execute` 내부에서 한 번 더 gate 삽입 — 중복.
- 이미 `connection_failed` 로 persisted 된 cluster 에 대한 특별 처리 — 기존 `CLUSTER_ARCH_UNKNOWN` 로 자연스럽게 warn 이 떨어지므로 추가 코드 불필요.

---

## 2. 프론트엔드 — Install Wizard 확장

### 2.1 validateCompatibility 훅 수정

`web/src/features/stack/api/stack-api.ts`:

- `validateCompatibility` 시그니처 재조정: `validateCompatibility({ stackId, tools?, clusterId?, nodeArchitectures? })`. 이미 Task 5 에서 객체 입력으로 바꿨으니 `tools` 가 `undefined` 인 호출을 허용하면 된다. body 에 `tools` 키 자체를 생략.
- React-Query mutation 훅 `useValidateCompatibility` 는 유지하되 input type 에 `tools?` 를 optional 로 선언.
- 응답 타입: 기존 `CompatibilityValidationResult` 재사용 (`overall`, `issues`, `nodeArchitectures`, `message`, `matrix`).

### 2.2 Deploy Stack 훅 수정

- `deployStack(stackId, { acknowledgeWarnings?: boolean })` 시그니처로 변경. body 가 없는 경우 기존 동작 유지를 위해 `acknowledgeWarnings` 를 optional 로 처리 (서버가 비어있으면 false 로 해석).
- `useDeployStack` mutation 도 같은 파라미터 형태.

### 2.3 stack-install-page.tsx Submit 플로우

현재 구조 (line ~3540~3560 부근):

```
if (compatibilityGate.state === 'fail') { ...block }
if (compatibilityGate.state === 'warn' && !compatWarnAcknowledged) { ...block }
const createRes = await createStack.mutateAsync(request)
await deployStack.mutateAsync(stackId)
navigate(`/stack/deploy/${stackId}`)
```

변경 후 (개략):

```
// 1. 기존 클라이언트 사이드 선검증 (지금 그대로 유지)
if (clientFail) block
if (clientWarn && !ack) block

// 2. stack 생성
const { id: stackId } = await createStack.mutateAsync(request)

// 3. 서버 측 재검증 (persisted mode)
const serverVerdict = await validateCompatibility.mutateAsync({ stackId })
if (serverVerdict.overall.state === 'fail') {
  surface issues, block, do NOT call deploy
  return
}
if (serverVerdict.overall.state === 'warn') {
  show acknowledgement prompt (reuse existing UI or modal)
  if user acks → continue with acknowledgeWarnings=true
  else block
}

// 4. deploy
await deployStack.mutateAsync({ stackId, acknowledgeWarnings: serverVerdict.overall.state === 'warn' })
navigate(`/stack/deploy/${stackId}`)
```

디테일:

- 서버 verdict 의 `issues[].code` 를 기존 i18n 키 (`stackInstall.compatibility.issue.toolArchUnsupported` 등) 에 매핑해 노출.
- 서버가 fail 을 돌려줬는데 이미 createStack 으로 DB 에 persist 된 stack 을 어떻게 처리할지: 이번 Task 에서는 **삭제하지 않음** (사용자가 조합을 고치고 재시도 가능하도록 draft 상태로 남김). 다만 서버 verdict 을 `tabGuardError` 아래 영역에 surface 하고, 사용자가 form 을 수정해 재제출하면 `createStack` 대신 `updateStack` / `saveDraft` 경로를 타야 하는데 **기존 Wizard 가 createStack 만 지원**하면 이 범위는 추후 follow-up 으로 돌리고, 지금은 "submit 을 다시 누르면 동일 tools 로 createStack 을 또 호출" → 중복 stack 이 생길 수 있음을 한 줄 TODO 로 남길 것.
- 프론트에 서버 verdict 재검증 중 스피너 상태 (`deployStack.isPending` 와 분리된 `isReValidating`) 를 도입해 버튼 레이블을 "Validating..." → "Deploying..." 두 단계로 구분.

### 2.4 deploy_handler 에러 응답 소비

서버가 `DEPLOY_COMPAT_FAIL` / `DEPLOY_COMPAT_WARN_UNACK` 로 400 을 던진 경우에도 동일한 verdict JSON 구조를 파싱해 UI 에 반영. 기존 `toDeployErrorMessage` 헬퍼를 확장해 code 기반 메시지 + issues 배열 요약을 얻도록 한다 (별도 함수 `extractDeployCompatError(error)` 로 분리 권장).

### 2.5 테스트

`web/src/features/stack/pages/stack-install-page.test.tsx` 확장:

- **Server-fail 블록**: MSW 가 `/stacks/:id/validate` 에서 `overall.state=fail` + `TOOL_ARCH_UNSUPPORTED` 반환 → deployStack 미호출, 에러 UI 노출.
- **Server-warn + ack 제공**: validate 가 warn 반환 → UI 가 ack prompt → 사용자가 ack 체크 후 재제출 → deployStack 이 `acknowledgeWarnings=true` 와 함께 호출됨.
- **Server-pass**: validate 가 pass 반환 → deployStack 바로 호출.
- **서버-Deploy 단계 fallback**: `/stacks/:id/validate` 통과했는데 `/deploy` 가 race 로 `DEPLOY_COMPAT_FAIL` 반환하는 edge — 에러 메시지 그대로 노출 + deploy 미완료 처리. (재시도 가능.)

validator 분기 순수 로직이 필요하면 별도 util 로 분리 (`shouldBlockOnServerVerdict(verdict)` 등) → 단위 테스트 3 case.

---

## 3. 공통 제약사항

1. **기존 API 의 하위 호환 유지**: `/stacks/:id/validate` 가 `tools` 있는 body 로 호출되면 지금 동작 그대로. `tools` 없이 호출되는 경우만 신규 persisted 모드.
2. **`acknowledge_warnings` 는 opt-in**: 기존 클라이언트가 body 없이 `/stacks/:id/deploy` 호출하면 `acknowledge_warnings=false` 로 해석 → verified 조합 + 호환 cluster 면 그대로 pass. warn 조합이면 블록. 이는 **의도된** 보안 강화.
3. **Module boundary 유지**: `deploy_handler` 가 admin 의 cluster repository 를 직접 import 하지 않는다. `ValidateCompatibility` 가 이미 `ClusterReader` port 를 통해 참조함.
4. **결정적 에러 메시지**: 서버 verdict 응답의 `issues[]` 순서는 stable 해야 한다 (test 에서 `.code` 로 매칭하도록 작성).
5. **테스트 실행**:
   ```
   go test ./internal/stack/... ./internal/admin/...
   go vet ./internal/stack/... ./internal/admin/...
   pnpm -C web test -- --run src/features/stack/
   pnpm -C web lint --max-warnings=0 src/features/stack/api src/features/stack/pages/stack-install-page.tsx
   pnpm -C web tsc --noEmit
   ```
   프로젝트 사전 부채 (cicd/e2e Delete 미구현, observability 테스트 등) 는 건드리지 말고 변경 범위 바깥이면 그대로 둘 것.
6. **CHANGELOG / plan**:
   - `CHANGELOG.md` `[Unreleased] > ### Added` 맨 위에 "F8-F3: Deploy 단계 서버측 Pre-Deploy Gate" 엔트리.
   - `docs/plans/compatibility_matrix_plan.md` 의 "v1 GA 후 follow-up" 섹션 (없다면 신규 추가) 에 F8-F3 를 `[x]` + 구현 요약으로 기록. Task 1~7 리스트 자체는 그대로.
7. **커밋/푸시 금지**: 사용자 명시 승인 전에는 `git commit`/`git push` 수행하지 말 것.

---

## 4. 산출물 체크리스트

- [ ] `internal/stack/usecase/validate_compatibility.go` — persisted mode + `StackID` 필드
- [ ] `internal/stack/usecase/validate_compatibility_test.go` — persisted mode 3 케이스
- [ ] `internal/stack/adapter/handler/compatibility_handler.go` — path stackId 를 body 에 보충
- [ ] `internal/stack/adapter/handler/deploy_handler.go` — Pre-Deploy Gate + ack 플래그 처리
- [ ] `deploy_handler_test.go` (또는 신규 `_compat_test.go`) — pass / fail / warn-unack / warn-ack / arch-unknown 5 케이스
- [ ] `cmd/api/main.go` — DeployHandler 에 ValidateCompatibility 주입
- [ ] `web/src/features/stack/api/stack-api.ts` — `validateCompatibility` tools optional, `deployStack` body 지원
- [ ] `web/src/features/stack/pages/stack-install-page.tsx` — 서버 재검증 → ack prompt → deploy 3단 플로우
- [ ] `stack-install-page.test.tsx` — 서버 verdict 분기 4 케이스
- [ ] `shouldBlockOnServerVerdict` (또는 유사) 유틸 + 단위 테스트 3 케이스
- [ ] `CHANGELOG.md` F8-F3 엔트리
- [ ] `docs/plans/compatibility_matrix_plan.md` F8-F3 follow-up 요약
- [ ] go test / vet / pnpm test / lint / tsc 결과 요약 보고

---

## 5. 작업 순서 제안

1. `validate_compatibility_test.go` 에 persisted mode 실패 테스트 작성 (Red).
2. `ValidateCompatibility` 에 persisted mode 구현 (Green).
3. `compatibility_handler.go` path-보충 로직 + test.
4. `deploy_handler.go` 에 gate 삽입 + test 4 케이스 (Red → Green).
5. `cmd/api/main.go` wiring 점검.
6. 서버측 go test / vet 한 번 돌려서 clean.
7. `stack-api.ts` 타입 + 훅 조정.
8. `stack-install-page.tsx` submit 플로우 재구성 + ack prompt UI.
9. `stack-install-page.test.tsx` 추가 케이스.
10. 유틸 함수 분리 + 단위 테스트.
11. pnpm test / lint / tsc clean.
12. CHANGELOG / plan 갱신.

---

## 6. 하지 말아야 할 것

- 스키마 / 매트릭스 seed / domain `ToolVersion` 수정.
- `InstallStack.Execute` 내부에 또 한번의 gate 삽입.
- `POST /compatibility/validate` 같은 새 경로 신설 (현재 `/stacks/:id/validate` 를 재사용).
- draft 단계 client-side 검증 (`compatibility-arch.ts`) 로직 변경 — 그대로 유지.
- 실패한 createStack 결과 stack 자동 삭제 — 별도 follow-up.
- cicd/e2e 쪽 pre-existing Delete 미구현 수정 (F8-F3 범위 밖; 사용자 허가 없이 손대지 말 것).
- 커밋/푸시.

---

## 7. 완료 보고 형식 (한국어, 간결)

1. 변경된 파일 목록 (백엔드 / 프론트엔드 / 테스트 / 문서 카테고리별).
2. 새 테스트와 의도 (1~2 문장씩).
3. `go test ./internal/stack/... ./internal/admin/...` 결과 요약.
4. `pnpm -C web test` / `lint` / `tsc` 결과 요약 (사전 부채는 unchanged 로 표기).
5. 관측한 의사결정 포인트 1~2개 (예: createStack 이후 fail 시 orphan stack 처리 전략, warn ack UI 위치 선택 근거).
6. 남은 follow-up 제안 — 최소한 "orphan stack 정리", "매트릭스 CRUD" 를 기록.

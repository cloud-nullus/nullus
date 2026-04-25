# F8 Follow-up: Cleanup 5종 일괄 프롬프트

**목적**: v1 GA 블로커가 아닌 cleanup 성 follow-up 5건을 묶어서 처리한다. 각 항목은 작은 리팩토링·테스트 추가·UX 마감 성격이며 서로 약한 의존만 있다.
**범위 제외**: F8-F6-Cloud (cloud provisioning 필요). 본 프롬프트에서 건드리지 않는다.
**실행 환경**: `/sessions/sweet-serene-curie/mnt/cloudbro/draft`, Go 1.24 / PostgreSQL 18 / React + TS / Vite + Vitest + Playwright.
**이미 확인된 사전조건**:
- `sonner@^2.0.7` 설치됨 + `web/src/components/ui/toast-provider.tsx` 가 `<Toaster />` 렌더 + `App.tsx` 에 등록됨. 기존 페이지들이 `import { toast } from 'sonner'` 로 사용 중 → Phase 4 toast 도입은 API 추가 작업 없이 직접 호출.
- `web/src/features/stack/utils/deploy-error.ts` 공유 `extractDeployCompatError` 이미 존재 (Phase B 성과물). Phase 1 은 순수 import 교체만.
- `web/src/features/stack/components/retry-stack-button.tsx` + `useRetryStack` 이미 존재 (Phase B 성과물). Phase 5 는 해당 컴포넌트 재사용.

---

## §0 공통 제약 (모든 Phase 적용)

1. **Phase 경계 commit**: 각 Phase 완료 시점에 1회 — 총 최대 5 commits. Phase 중간 WIP commit 금지. commit 메시지는 각 Phase 섹션 말미에 명시.
2. **Phase drop 허용**: 실행 중 현재 Phase 가 비정상적으로 큰 회귀를 일으키거나 세션 budget 초과가 명백하면 해당 Phase 를 drop 하고 `compatibility_matrix_plan.md §6` 의 해당 항목을 `DROPPED (재시도 필요)` 상태로 갱신한 뒤 다음 Phase 로 진행한다. 완료 보고서에 drop 사유를 기재.
3. **Phase 독립성**: 5 Phase 모두 서로 독립이다. 순서는 Phase 1 → 2 → 3 → 4 → 5 를 권장(단순→복잡, 테스트 계층→실 컴포넌트 변경). 임의 Phase 만 뽑아 실행해도 나머지에 영향 없음.
4. **금지 사항**:
   - 백엔드 Go 코드, 마이그레이션, audit 시맨틱 변경 금지. 본 cleanup 은 **Frontend + E2E only** 이다.
   - Phase 5 에서 기존 `DEPLOYMENT_DATA` mock 완전 제거 금지 — stack ID 로 매칭되지 않을 때 fallback 으로 유지 (기존 E2E/dev 워크플로 보호). 새 real-data 경로는 **추가 렌더 분기** 로 구현.
   - Retry 정책(`canRetry(status)`) 변경 금지. `failed | rolled_back` 만 true.
   - Pre-Deploy Gate verdict 정책 및 에러 코드 rename 금지.
   - 새 npm 패키지 추가 금지 — sonner / @testing-library / vitest / msw / playwright 등 현재 의존성 내에서 해결.
5. **테스트 원칙**: 각 Phase 변경 범위에 대한 테스트가 green 이어야 Phase 종료. Phase 별 test 지침은 각 섹션 참조.
6. **보고 형식** (Phase 별 각각): 변경 파일 리스트 / 추가/삭제 LOC / 테스트 결과 (`npx vitest run <scope>`, `npx tsc --noEmit`, `npx eslint <touched>`, Phase 3 만 `npx playwright --grep @stack-critical`) / 관측된 결함 / 다음 Phase 에 미치는 영향.

---

## Phase 1 — F8-DeployError-Dedup (inline → shared util)

### 1.1 현재 상태

- `web/src/features/stack/pages/stack-install-page.tsx` 라인 57~94 에 `extractDeployCompatError` 함수가 **inline 정의** 되어 있음.
- `web/src/features/stack/utils/deploy-error.ts` 에 동일 시맨틱 공유 버전이 존재 (Phase B 에서 추출됨). 중복 상태.
- 두 구현 간 미묘한 차이: inline 은 `DeployCompatError` 타입 없이 인라인 object 반환, shared 는 `DeployCompatError` 인터페이스 반환. 기능적으로 동일.

### 1.2 구현

1. `stack-install-page.tsx` 상단 import 에 `import { extractDeployCompatError } from '../utils/deploy-error'` 추가.
2. 라인 57~94 의 inline `function extractDeployCompatError` 정의 완전 삭제.
3. `toDeployErrorMessage` 함수 본문 (라인 96~) 은 그대로 유지 — shared 버전은 동일한 필드(`code`, `issueLines`) 를 반환하므로 호출 계약 변경 없음.
4. 검색 `grep -n "extractDeployCompatError" web/src/features/stack/pages/stack-install-page.tsx` 가 1건 (import 라인) 만 남는지 확인.

### 1.3 테스트

- 새 테스트 파일 생성 **불필요**. 기존 `stack-install-page.test.tsx` 가 `toDeployErrorMessage` 경로를 간접 커버. 유지만 확인.
- `web/src/features/stack/utils/deploy-error.test.ts` 가 없으면 얇은 테스트 추가 (4 케이스): `DEPLOY_COMPAT_FAIL` 파싱 / `DEPLOY_COMPAT_WARN_UNACK` 파싱 / 비-compat 에러 null / malformed body null.

### 1.4 완료 체크리스트

- [ ] `stack-install-page.tsx` inline 정의 삭제 (라인 57~94 블록).
- [ ] shared util import 추가.
- [ ] `npx vitest run src/features/stack` 클린.
- [ ] `npx tsc --noEmit` 클린.
- [ ] `npx eslint web/src/features/stack/pages/stack-install-page.tsx` 0 errors (pre-existing lint error 는 무시 범위 외).
- [ ] `compatibility_matrix_plan.md §6 F8-DeployError-Dedup` 을 `[x]` + 완료 요약으로 갱신.

### 1.5 Stop & Verify

```bash
cd web && npx vitest run src/features/stack --reporter=dot && npx tsc --noEmit
grep -c "function extractDeployCompatError" web/src/features/stack/pages/stack-install-page.tsx
# 기대: 0
```

**Commit**: `refactor(stack): share extractDeployCompatError between install/retry flows (F8 cleanup)`

---

## Phase 2 — F8-Phase5-DOMTest (매트릭스 관리 DOM 스모크)

### 2.1 현재 상태

- `web/src/features/admin/pages/stack-versions-page.tsx` 에 New/Edit/Delete 버튼 + `MatrixEditModal` + Confirm Dialog 통합됨 (Phase A 성과물).
- 백엔드 handler 6 테스트 + vitest 훅 계약 커버로 API 경계는 검증됨.
- DOM-레벨 상호작용 테스트 미존재.

### 2.2 구현

`web/src/features/admin/pages/stack-versions-page.test.tsx` 신규. MSW 로 `/admin/compatibility/matrices` 3종 + `/compatibility` GET + `/admin/clusters` 스텁.

테스트 케이스 5종:

1. **Render baseline**: page 렌더 → "New matrix" 버튼 보임 + 좌측 매트릭스 리스트 3종 (Narwhal seed) 렌더.
2. **New → modal open**: "New matrix" click → role=dialog + title="New matrix" + ID 입력 필드 enabled.
3. **Edit → modal pre-filled**: 목록에서 첫 매트릭스 선택 → "Edit" click → dialog 노출 + ID 필드 disabled + Name/Status/Tools 필드가 선택된 매트릭스 값으로 prefill.
4. **Delete → confirm dialog → mutation**: "Delete" click → Confirm dialog ("정말 삭제하시겠습니까?" or i18n 키) → "Delete" 확정 → MSW 가 DELETE 요청 수신 확인 (handler 내에서 flag set 후 assertion).
5. **Cancel delete**: Confirm dialog 에서 "Cancel" → dialog 사라짐 + 삭제 요청 발생하지 않음.

**Test utilities**:
- 기존 `test/setup.tsx` 또는 유사 파일의 `renderWithProviders(ui)` helper 재사용 (QueryClientProvider + i18n + MemoryRouter).
- MSW 스텁은 `test/mocks/handlers.ts` 혹은 test 파일 내 `setupServer` 로 국지적 정의.
- `@testing-library/user-event` 로 click / type 시뮬레이션.

**주의**:
- `matrix-edit-modal.tsx` 의 동적 tool 행 추가·제거 세부 동작은 이 스모크에서 검증하지 않음 — 별도 단위 테스트 (`matrix-edit-modal.test.tsx`) 로 분리 가능하나 본 Phase 범위 밖.
- ConfirmDialog 컴포넌트가 portal 렌더면 `findByRole('dialog')` 사용.

### 2.3 완료 체크리스트

- [ ] `stack-versions-page.test.tsx` 신규 — 5 케이스 모두 green.
- [ ] MSW handler 추가 (필요 시).
- [ ] `npx vitest run src/features/admin` 클린.
- [ ] `npx tsc --noEmit` 클린.
- [ ] `compatibility_matrix_plan.md §6 F8-Phase5-DOMTest` 을 `[x]` + 완료 요약으로 갱신.

### 2.4 Stop & Verify

```bash
cd web && npx vitest run src/features/admin/pages/stack-versions-page.test.tsx --reporter=verbose
# 기대: 5 passed
```

**Commit**: `test(admin): DOM smoke for compatibility matrix CRUD UI (F8-Phase5-DOMTest)`

---

## Phase 3 — F8-RetryUI-E2E (Playwright @stack-critical 스모크)

### 3.1 현재 상태

- `web/src/features/stack/components/retry-stack-button.tsx` 가 `failed | rolled_back` 에서 자기 노출.
- Playwright `web/e2e/stack-warn-forced-retry.spec.ts` 가 `@stack-critical` 태그로 warn UI 플로우만 커버. Retry 버튼 자체 플로우는 미커버.

### 3.2 구현

`web/e2e/stack-retry-button.spec.ts` (`@stack-critical`) 신규. Kind 의존 없는 순수 UI 스모크.

**Fixture 전략** (택 1):

A. **MSW-per-test** (권장, Kind 불필요):
  - Playwright `page.route('**/api/v1/stacks*', ...)` 로 API 응답 스텁.
  - `GET /stacks` → 배열 2종 리턴: `{id: 'stack-ok', status: 'completed', ...}`, `{id: 'stack-fail', status: 'failed', ...}`.
  - `POST /stacks/stack-fail/retry` → 200 `{stack_id: 'stack-fail', status: 'pending'}`.

B. **Test DB seed** (불가 — CI 세션 budget 범위 밖):
  - Postgres seed 방식은 본 Phase 에서 제외.

Phase 3 은 **A 방식만** 사용.

**케이스 3종**:

1. **Retry button presence**: `/stack/list` 진입 → `stack-fail` 카드에 `button:has-text("Retry")` 존재. `stack-ok` 카드에는 부재.
2. **Retry success flow**: `stack-fail` 의 Retry 버튼 click → `POST /retry` 요청이 발생 (`page.waitForRequest`), 성공 토스트 (sonner) 노출 — Phase 4 완료 전이면 이 assertion 은 `skip` 처리 후 Phase 4 에서 enable.
3. **Retry warn-ack flow**: `POST /stacks/stack-fail/retry` 스텁을 400 `DEPLOY_COMPAT_WARN_UNACK` (verdict body 포함) 로 바꿔 Retry click → warn modal 노출 + issue list 렌더 + ack 체크박스 + "다시 시도" click → 두 번째 `POST` 가 `acknowledge_warnings: true` body 로 전송.

**Fixture 구조**:
```ts
test.describe('@stack-critical retry button', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/stacks', async (route) => { ... })
    // ... 필요 시 /clusters, /templates 도 stub
  })
  test('shows retry on failed stack', async ({ page }) => { ... })
  test('retry success triggers API + toast', async ({ page }) => { ... })
  test('retry warn surfaces ack modal and resubmits', async ({ page }) => { ... })
})
```

**Pre-existing `@stack-critical` 회귀 방지**:
- `stack-workflow.spec.ts` / `stack-monitoring.spec.ts` / `stack-warn-forced-retry.spec.ts` 수정 금지.
- 새 스펙 추가로 3/3 → 6/6 까지 확장. 단, 기존 3개의 pass 상태 유지 확인.

### 3.3 완료 체크리스트

- [ ] `stack-retry-button.spec.ts` 신규 — 3 케이스.
- [ ] `npx playwright test --grep @stack-critical` 전체 그린 (기존 3 + 신규 3 = 6 pass).
- [ ] `npx tsc --noEmit` 클린 (Playwright 코드 포함).
- [ ] `compatibility_matrix_plan.md §6 F8-RetryUI-E2E` 을 `[x]` + 완료 요약으로 갱신.

### 3.4 Stop & Verify

```bash
cd web && npx playwright test --grep @stack-critical --reporter=list
# 기대: 6 passed (Phase 4 미완료 시 "toast 검증" 부분만 test.skip 으로 유예)
```

**Commit**: `test(e2e): @stack-critical retry button smoke (F8-RetryUI-E2E)`

---

## Phase 4 — F8-Retry-Toast (sonner 통합)

### 4.1 현재 상태

- `RetryStackButton` 은 `onRetried` 콜백만 호출. 성공/실패 전역 피드백 없음.
- 프로젝트는 이미 `sonner` 사용 중. `stack-list-page.tsx` / `stack-add-tools-page.tsx` 에서 `import { toast } from 'sonner'` 로 `toast.success` / `toast.error` 호출 패턴 확인됨.

### 4.2 구현

`retry-stack-button.tsx` 에 sonner 연동:

```ts
import { toast } from 'sonner'

// 성공 분기 (기존 200 응답 분기 내부):
toast.success(t('stackList.retry.toasts.success', '재배포를 시작했습니다.'))
onRetried?.(stackId)

// 실패 — DEPLOY_COMPAT_FAIL:
toast.error(
  t('stackList.retry.toasts.fail', '배포 차단') +
    (gate.issueLines.length ? ' — ' + gate.issueLines.join('; ') : ''),
)

// 실패 — 일반 에러 (warn-ack modal 경로는 기존대로 modal 노출 → modal 내부에서 재시도):
toast.error(toDeployErrorMessage(error))
```

- `toDeployErrorMessage` 는 기존 `utils/deploy-error.ts` 확장 또는 `stack-install-page` 에서 공유. 없으면 로컬 정의 허용 — 단순 fallback 문자열로 충분.
- i18n 키 `stackList.retry.toasts.success` / `stackList.retry.toasts.fail` 추가 (ko/en).
- Warn-ack modal 확인 후 성공 시에도 동일 `toast.success` 발화.
- warn-ack modal 내부에서 "취소" 시 toast 발화하지 않음 (무동작).

### 4.3 테스트

`retry-stack-button.test.tsx` (신규 또는 기존 확장):

- sonner `toast` 를 mock — `vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))`.
- 4 케이스:
  1. 200 응답 → `toast.success` 1회 호출.
  2. 400 `DEPLOY_COMPAT_FAIL` → `toast.error` 1회 호출 + issueLines 포함.
  3. 400 `DEPLOY_COMPAT_WARN_UNACK` → toast 미발화 + warn modal 노출 (기존 동작).
  4. warn modal 내 ack + 재시도 200 → `toast.success` 1회 호출.

### 4.4 완료 체크리스트

- [ ] `retry-stack-button.tsx` sonner 통합 + i18n 추가.
- [ ] `retry-stack-button.test.tsx` 4 케이스 추가.
- [ ] `npx vitest run src/features/stack/components` 클린.
- [ ] `npx tsc --noEmit` 클린.
- [ ] Phase 3 Playwright 에서 `skip` 처리한 toast assertion 을 enable (있으면) → 재실행 그린.
- [ ] `compatibility_matrix_plan.md §6 F8-Retry-Toast` 을 `[x]` + 완료 요약으로 갱신.

### 4.5 Stop & Verify

```bash
cd web && npx vitest run src/features/stack/components --reporter=verbose
# 기대: retry-stack-button.test.tsx 4 passed
cd web && npx playwright test --grep @stack-critical --reporter=list
# 기대: 6 passed (toast assertion enable 후에도)
```

**Commit**: `feat(stack): retry success/failure toast via sonner (F8-Retry-Toast)`

---

## Phase 5 — F8-DeploymentLogs-Retry (mock→real 리팩토링 + Retry 통합)

### 5.1 현재 상태

- `web/src/features/stack/pages/stack-deployment-logs-page.tsx` 335 라인.
- 현재 구조: `useParams<{ deploymentId }>` + 상수 `DEPLOYMENT_DATA` 에서 hard-coded 조회. `meta.result` 는 `'success' | 'failed' | 'running'` (3-state).
- `DEPLOYMENT_DATA` key 는 `deploy-v1-20260220` 형식의 mock ID. 실제 stack-list-page → "View logs" 네비게이션 시 동일 ID 를 params 로 넘긴다.

### 5.2 접근 전략

**완전 교체 금지** — 기존 mock 경로는 `/stack/deploy/:deploymentId` 라우트의 dev 편의 및 e2e 기대로 당분간 유지하되, **real-data 추가 분기** 로 단계적 전환.

전략: `deploymentId` 를 두 가지로 해석:

1. `deploymentId` 가 `DEPLOYMENT_DATA` 의 key 와 일치하면 기존 mock 렌더 (backward compat).
2. 아니면 `deploymentId` 를 `stackId` 로 간주하고 `useStack(stackId)` / `useStackMonitoring(stackId)` 로 실제 상태 로드.

`useStack(stackId)` 훅이 없으면 `useStacks().data?.items.find(s => s.id === stackId)` 로 파생 조회 (추가 API 호출 없음).

**Retry 버튼 통합**:

- Real-data 분기에서만 `RetryStackButton` 렌더. mock 분기는 건드리지 않음.
- `stack.status` 를 `RetryStackButton` 의 `status` prop 으로 전달. 버튼 자체가 `canRetry` 자기검열.
- 성공 시 `onRetried(stackId)` 콜백 → `navigate(\`/stack/deploy/\${stackId}\`)` 혹은 현재 페이지 refetch.

### 5.3 구현

`stack-deployment-logs-page.tsx`:

```tsx
// 기존 import 유지 + 추가:
import { useStacks } from '../api/stack-api'
import { RetryStackButton } from '../components/retry-stack-button'

// 컴포넌트 내부:
const { deploymentId } = useParams<{ deploymentId: string }>()
const mockEntry = deploymentId ? DEPLOYMENT_DATA[deploymentId] : undefined
const { data: stacksData } = useStacks()
const realStack = useMemo(() => {
  if (mockEntry || !deploymentId) return undefined
  return stacksData?.items?.find((s) => s.id === deploymentId)
}, [stacksData, deploymentId, mockEntry])

// 렌더 분기:
if (mockEntry) {
  return <LegacyMockRender entry={mockEntry} />  // 기존 JSX 를 소형 컴포넌트로 추출해도 OK
}
if (realStack) {
  return (
    <RealStackRender
      stack={realStack}
      onRetry={() => {/* refetch or navigate */}}
    />
  )
}
// 둘 다 없으면 기존 "Deployment not found" 분기 유지.
```

**`RealStackRender` 내부**:
- Stack name, namespace, status (ko label 매핑은 `stack-list-page` 의 `STATUS_LABEL` 재사용 또는 import).
- `stack.status ∈ {failed, rolled_back}` → stage table 에 "Failed" 단계 표시 + `RetryStackButton` 노출.
- 실제 로그 스트리밍은 본 Phase 범위 밖 — placeholder "Live log streaming not yet connected. See deployment events." 렌더.

**주의**:
- 기존 mock 케이스가 이미 테스트/스크린샷에 있을 수 있음. `grep -rn "DEPLOYMENT_DATA\|deploy-v1-20260220" web/src web/e2e` 로 영향 범위 확인 후 진행.
- Legacy 렌더 컴포넌트 추출은 선택사항. 추출 시 동일 파일 내 private 함수 컴포넌트로 유지 (새 파일 분리 금지 — 본 Phase 스코프 제어).

### 5.4 테스트

1. `stack-deployment-logs-page.test.tsx` (신규 또는 기존 확장):
   - **Mock fallback**: `/stack/deploy/deploy-v1-20260220` 라우트 → 기존 mock 렌더 (회귀 테스트).
   - **Real stack — completed**: `/stack/deploy/<realStackId>` + MSW `GET /stacks` 스텁 → Stack 정보 렌더 + Retry 버튼 **미노출**.
   - **Real stack — failed**: 동일 경로 + failed status → Retry 버튼 노출.
   - **Real stack — not found**: 존재하지 않는 ID + MSW empty items → "Deployment not found" 렌더.

2. Playwright 회귀 방지: `npx playwright --grep @stack-critical` 6/6 유지. 만약 기존 E2E 가 `deploy-v1-20260220` mock ID 에 의존하면 그대로 통과 (mock fallback 분기가 보존).

### 5.5 완료 체크리스트

- [ ] `stack-deployment-logs-page.tsx` real-data 분기 + Retry 버튼 통합.
- [ ] `DEPLOYMENT_DATA` 유지 (backward compat).
- [ ] 신규 테스트 4 케이스.
- [ ] `npx vitest run src/features/stack/pages` 클린.
- [ ] `npx tsc --noEmit` 클린.
- [ ] `npx playwright test --grep @stack-critical` 6/6 pass 유지.
- [ ] `compatibility_matrix_plan.md §6 F8-DeploymentLogs-Retry` 을 `[x]` + 완료 요약으로 갱신.

### 5.6 Stop & Verify

```bash
cd web && npx vitest run src/features/stack/pages --reporter=dot
cd web && npx tsc --noEmit
cd web && npx playwright test --grep @stack-critical --reporter=list
```

**Commit**: `feat(stack): real-data view + retry button on deployment logs page (F8-DeploymentLogs-Retry)`

---

## 최종 보고 템플릿

```
F8 Cleanup 5종 완료 보고
========================

| Phase | 항목                          | 상태     | 커밋 메시지 (예상)                                                        |
|-------|-------------------------------|----------|---------------------------------------------------------------------------|
| 1     | F8-DeployError-Dedup          | ✅/DROP | refactor(stack): share extractDeployCompatError ...                       |
| 2     | F8-Phase5-DOMTest             | ✅/DROP | test(admin): DOM smoke for compatibility matrix CRUD UI                   |
| 3     | F8-RetryUI-E2E                | ✅/DROP | test(e2e): @stack-critical retry button smoke                             |
| 4     | F8-Retry-Toast                | ✅/DROP | feat(stack): retry success/failure toast via sonner                       |
| 5     | F8-DeploymentLogs-Retry       | ✅/DROP | feat(stack): real-data view + retry button on deployment logs page        |

누적 테스트:
- npx vitest run: <N> files, <M> tests passed
- npx tsc --noEmit: clean
- npx playwright --grep @stack-critical: <K> passed
- 관측된 pre-existing 이슈: ...
- Drop 된 Phase (있다면) + 사유:
```

---

## 부록: 각 Phase 별 파일 영향 범위 (사전 추정)

| Phase | 수정/신규 파일 | 예상 LOC delta |
|-------|---------------|----------------|
| 1 | stack-install-page.tsx (삭제) + utils/deploy-error.test.ts (신규) | −35 / +40 |
| 2 | stack-versions-page.test.tsx (신규) + 필요 시 test/mocks/* | +180 |
| 3 | e2e/stack-retry-button.spec.ts (신규) | +150 |
| 4 | retry-stack-button.tsx (수정) + retry-stack-button.test.tsx (신규/확장) + i18n ko/en | +80 / −5 |
| 5 | stack-deployment-logs-page.tsx (수정) + stack-deployment-logs-page.test.tsx (신규) | +120 / −15 |

총 예상: backend 0 LOC 변경, frontend 약 +550 / −55.

---

## 끝.

Phase 1 → 2 → 3 → 4 → 5 순서로 실행. 각 Phase 끝에 Stop & Verify 거친 뒤 독립 commit. drop 조항은 §0.2 를 따른다.

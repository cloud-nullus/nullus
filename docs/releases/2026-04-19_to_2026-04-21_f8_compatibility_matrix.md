# main 머지 기능 요약 (2026-04-19 → 2026-04-21)

대규모 F8 트랙(호환성 매트릭스)이 메인에 들어왔습니다. **5 PR + 1 feat branch merge**, UI·backend 전반에 걸쳐 변경이 있습니다.

포함 머지:

- `41f9dac` (2026-04-20) — feat(stack): F8 compatibility matrix with CRUD UI, retry button, and v1 GA follow-ups
- #41 (2026-04-20) — F8 cleanup 5종 일괄 마감 (DeployError-Dedup / DOMTest / RetryUI-E2E / Retry-Toast / DeploymentLogs-Retry)
- #42 (2026-04-21) — F8 UI/UX Tier 1 — 즉시 버그성 수정 4종 (ServerVerdictI18n / DeployGateServerCheck / WarnAckI18n / MatrixEditDirty)
- #43 (2026-04-21) — F8 UI/UX Tier 2 — UX 정교화 6 Phase (StatusBadgeColors / MatrixListOps / MatrixEditValidation / RealTimeline / RetryFeedback / WarnAckPersist)
- #44 (2026-04-21) — F8 UI/UX Tier 3 — 4 Phase (A11y / EmptyLoading / Polish / E2EDeepScenario)
- #45 (2026-04-21) — F8 UI/UX Backlog 완료 — RetryAuditSurface (BE+FE) + KeyboardHints

---

## 🆕 새 엔드포인트 (API Contract 변경)

| Method | Path | 기능 |
| --- | --- | --- |
| `POST` | `/api/v1/stacks/:id/retry` | `failed` / `rolled_back` 스택 재배포. `acknowledge_warnings` body 지원 |
| `PUT` | `/api/v1/stacks/:id` | 스택 업데이트 (draft config 교체) |
| `POST` | `/api/v1/admin/compatibility/matrices` | 매트릭스 생성 |
| `PUT` | `/api/v1/admin/compatibility/matrices/:id` | 매트릭스 업데이트 |
| `DELETE` | `/api/v1/admin/compatibility/matrices/:id` | 매트릭스 삭제 |
| `GET` | `/api/v1/stacks/:id/retry-history` | 재시도 audit 이력 조회 |

**신규 에러 코드** (클라이언트가 처리해야 함):

- `DEPLOY_COMPAT_FAIL`
- `DEPLOY_COMPAT_WARN_UNACK`
- `STACK_RETRY_INVALID_STATE`

---

## 🧩 Backend 주요 변경

- **Server-side Pre-Deploy Gate** — Deploy/Retry 시 서버가 verdict 재검증
  - `warn` → `acknowledge_warnings: true` 필요
  - `fail` → 무조건 차단
- **`audit.Sink` interface + `MemorySink`** — E2E 에서 audit 이벤트 검증용
- **`audit.Reader` interface** — per-resource 조회 (retry-history 엔드포인트 backbone)
- **`MemoryVerdictCache`** — sha256 key + TTL
  - env: `VERDICT_CACHE_TTL_SEC`
- **`RefreshDiscoveryScheduler`** — cluster 노드 아키텍처 주기 갱신
  - env: `REFRESH_DISCOVERY_INTERVAL`
- **`UpdateStack` usecase** — `{pending, failed}` 에서만 허용, history 자동 스냅샷

---

## 🎨 Frontend 주요 변경

### 신규 기능

- **매트릭스 CRUD UI** (`/admin/stack-versions`)
  - New / Edit / Delete + ConfirmDialog
  - `items > 5` 시 검색·상태 필터 노출
  - color dot legend
- **`RetryStackButton`** — 자기검열 (`failed` / `rolled_back` 만 렌더)
  - `stack-list-page` 카드
  - `deployment-logs RealStackView`
- **`RetryHistoryPanel`** — 재배포 audit 기록 테이블 (3행 + expand 토글)
- **글로벌 키보드 단축키**
  - `?` — 도움말 모달
  - `n` — stack-list → install wizard
  - 우하단 `shortcut-badge` 고정 버튼
- **Skeleton loading + Empty state** — 매트릭스 리스트 로딩 / 0건 UI

### 공유 Util (다른 페이지에서 재사용 가능)

| 경로 | 제공 |
| --- | --- |
| `utils/status-style.ts` | `STATUS_STYLES` + `getStatusStyle(status)`. **`rolled_back` 색상 grey → amber 변경** |
| `utils/retry-policy.ts` | `canRetry(status)` |
| `utils/deploy-error.ts` | `extractDeployCompatError` |
| `utils/deploy-gate.ts` | `isDeployServerGateLocked` |
| `utils/compat-issue-i18n.ts` | `getCompatIssueMessage` |
| `utils/warn-ack-storage.ts` | sessionStorage 영속화 (verdictHash 키) |
| `hooks/use-keyboard-shortcut.ts` | 단일 키 바인딩 |
| `components/ui/skeleton.tsx` | `animate-pulse` primitive |
| `components/shortcut-help-modal.tsx` | 단축키 도움말 모달 + `SHORTCUT_REGISTRY` |

### 눈에 띄는 UX 변경

- **Modal** focus trap + prev-focus restore (외부 라이브러리 없이 자체 구현)
- **Retry 토스트 3단계** — `loading → success | error` (sonner `id` 기반)
- **warn-ack 체크박스 영속화** — 새로고침 시 verdict 동일하면 ack 유지
- **`matrix-edit-modal`**
  - 인라인 validation (ID / Name / K8s)
  - `window.confirm` unsaved 가드
  - 중복 category 검사
  - dirty-drop 2-step Save
- **deployment-logs `RealStackView`** — 5-stage 파생 timeline
  - `Queued → Provisioning → Deploying → Validating → Completed`

---

## 🧪 테스트 추가

- **Go** — audit / stack handler 테스트 **20+ 신규**
  - MemorySink Reader
  - retry-history handler
  - matrix CRUD handler
  - Pre-Deploy Gate
  - verdict cache
  - refresh scheduler
- **Vitest** — **150+ 신규** 케이스
  - util 순수 함수
  - 페이지 DOM 스모크
- **Playwright** `@stack-critical` Tier B (backend-free) — 3 spec 신규
  - `stack-retry-button.spec.ts`
  - `stack-warn-forced-retry.spec.ts`
  - `stack-retry-scenario.spec.ts`

---

## ⚠️ 다른 PR 작업자가 주의해야 할 점

1. **`stack-list-page.tsx` inline `STATUS_STYLES` 제거됨** — 사용 중이었다면 `utils/status-style.ts` 에서 import
2. **`retry-stack-button.tsx` 가 sonner 토스트 발화** — 테스트에서 `vi.mock('sonner')` 로 `toast.loading` / `toast.dismiss` 까지 mock 필요
3. **`ko.json` / `en.json` 대규모 추가** — merge conflict 위험 있음. 신규 상위 키:
   - `stackInstall.compatibility.{issue,serverVerdict,gate}`
   - `stackVersionsAdmin.{actions,modal,deleteConfirm,filter,legend,empty}`
   - `stackList.retry.{toasts,confirmWarn}`
   - `stackDeployment.retryHistory`
   - `shortcuts.*`
4. **`Modal` 컴포넌트에 focus trap 추가됨** — 기존 Modal 테스트에서 focus 기대 동작이 바뀔 수 있음
5. **`Button` 에 `whitespace-nowrap`, `Modal h2` 에 `break-keep`** 전역 적용
6. **pre-existing TFunction TS 타입 mismatch 12건 유지** — 해당 패턴 `(t: (key, defaultValue?) => string)` 은 기존 codebase convention, 신규 작성 시에도 같은 패턴 권장

---

## 📌 잔여 백로그

- **`F8-F6-Cloud`** — EKS / GKE golden path 검증
  - 클라우드 계정·시크릿 선행 필요
  - 별도 트랙

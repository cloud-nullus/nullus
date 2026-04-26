# F8 Follow-up: 매트릭스 CRUD UI (Phase 5 재개) + Retry UI 버튼 통합 프롬프트

**목적**: (A) 이전 일괄에서 DROP 된 매트릭스 CRUD UI 를 단독 Phase 로 재개하고, (B) Phase 3 에서 API/훅만 넣어둔 Retry 경로에 실제 UI 버튼을 연결한다.
**범위 제외**: F8-F6-Cloud, Scheduler 관측 메타데이터, Verdict 캐시 per-prefix 정교화. 본 프롬프트에서 건드리지 않는다.
**실행 환경**: `/sessions/sweet-serene-curie/mnt/cloudbro/draft`, Go 1.24 / PostgreSQL 18 / React + TS / Vite + Vitest + Playwright.

---

## §0 공통 제약 (모든 Phase 적용)

1. **Phase 경계 commit**: Phase A 완료 시점 1회, Phase B 완료 시점 1회 — 총 2 commits. Phase 중간 WIP commit 금지. 단, 각 Phase 내부에서 test-only 분리 commit 은 가능.
2. **Phase drop 허용**: 실행 중 현재 Phase 가 비정상적으로 큰 회귀를 일으키거나 세션 budget 초과가 명백하면 해당 Phase 를 drop 하고 `compatibility_matrix_plan.md §6` 에 `DROPPED` 라벨로 분리 항목을 등록한 뒤 다음 Phase 로 진행한다. drop 사유(스코프/회귀 종류)는 완료 보고서에 기재.
3. **Phase 간 의존성**: Phase A 와 B 는 독립적이다. A 실패가 B 실행을 막지 않는다. 순서는 A → B 권장(매트릭스 편집이 verdict 에 영향을 주므로 cache invalidation 경로를 먼저 확정).
4. **금지 사항**:
   - 새 마이그레이션 재번호 금지. 스키마 변경이 필요한 경우 `000044_*.up.sql` / `.down.sql` 로 append 만 허용 — 기존 000001~000043 을 건드리지 말 것.
   - `AuditLogger` pgxpool 실 거동(INSERT 쿼리 / 컬럼 순서) 변경 금지. 새 `audit.Log` 호출은 허용.
   - Pre-Deploy Gate verdict 정책(verified+arch miss → fail, untested+arch miss → warn, arch unknown → warn) 변경 금지.
   - 기존 에러 코드 rename 금지 (`TOOL_ARCH_UNSUPPORTED` / `CLUSTER_ARCH_UNKNOWN` / `KUBECONFIG_NOT_REGISTERED` / `DEPLOY_COMPAT_FAIL` / `DEPLOY_COMPAT_WARN_UNACK` 유지). 신규 코드 추가는 허용.
   - `port.CompatibilityRepository.Validate` signature 변경 금지.
5. **테스트 원칙**: 변경된 파일 인근에 단위 테스트 먼저 추가 → 구현 → `go test ./...` 및 `npx vitest run` 이 Phase 종료 시점 green 이어야 함. E2E (Playwright) 는 Kind 의존 없는 스모크만 본 프롬프트 범위. Kind 클러스터 필요한 검증은 F8-F6-Cloud 로 이관.
6. **보고 형식** (Phase 별 각각): 변경 파일 리스트(Backend/Frontend/Docs/Tests 구분) / 신규 엔드포인트 및 UI 요소 요약 / 테스트 결과 (`go test ./... | tail -5`, `go vet ./...`, `npx vitest run ...`, `npx tsc --noEmit`, `npx eslint <touched>`) / 관측된 결함 / 다음 단계 제안.

---

## Phase A — 매트릭스 CRUD UI (F8-Phase5 재개)

### A.1 현재 상태 (2026-04-20)

- 백엔드 `CompatibilityHandler` 는 `GET /compatibility` + `POST /:stackId/validate` 만 제공. CRUD 엔드포인트 없음.
- `port.CompatibilityRepository` 인터페이스는 `GetAll / GetByID / Validate` 3개 메서드. Create/Update/Delete 없음.
- `MemoryCompatibilityRepository` 는 `defaultCompatibilityMatrices()` 로 시드, `PostgresCompatibilityRepository` 는 `compatibility_matrices` 테이블 read-only.
- 프론트 `features/admin/pages/stack-versions-page.tsx` (309 줄) 는 read-only 뷰 — 좌측 매트릭스 목록, 우측 상세 (K8s 범위, tools 테이블, clusters cross-eval). "New / Edit / Delete" 버튼 없음.
- 매트릭스 편집 시 invalidation 대상: `useCompatibilityMatrix()` 캐시 + `MemoryVerdictCache.Clear()` (Phase 6 캐시).

### A.2 구현 범위

#### A.2.1 백엔드: Repository 포트 확장

`internal/stack/port/compatibility.go`:

```go
type CompatibilityRepository interface {
    GetAll(ctx context.Context) ([]*domain.CompatibilityMatrix, error)
    GetByID(ctx context.Context, id string) (*domain.CompatibilityMatrix, error)
    Validate(ctx context.Context, tools map[string]string) (*domain.CompatibilityMatrix, error)

    // CRUD — F8-Phase5 재개.
    Create(ctx context.Context, m *domain.CompatibilityMatrix) error
    Update(ctx context.Context, m *domain.CompatibilityMatrix) error
    Delete(ctx context.Context, id string) error
}
```

- `Create`: 동일 ID 존재 시 `ErrCompatibilityMatrixExists` (새로 정의) 반환. 성공 시 포인터 내부 상태는 변경하지 않음.
- `Update`: 존재하지 않는 ID 면 `ErrCompatibilityMatrixNotFound` (기존 `not found` 문자열을 sentinel error 로 승격). partial update 아니라 full-replace 시맨틱 — 클라이언트가 보내는 전체 매트릭스로 교체.
- `Delete`: 멱등적(idempotent). 존재하지 않는 ID 여도 에러 반환하지 않고 nil — 핸들러 쪽에서 404 처리하려면 선행 `GetByID` 호출로 결정.
- 에러 센티넬은 `internal/stack/port/errors.go` 신규 또는 `port/compatibility.go` 말미에 `ErrCompatibilityMatrixNotFound = errors.New("compatibility matrix not found")` / `ErrCompatibilityMatrixExists = errors.New("compatibility matrix already exists")` 정의.

#### A.2.2 백엔드: Memory 구현

`internal/stack/adapter/repository/memory_compatibility.go` 말미:

```go
func (r *MemoryCompatibilityRepository) Create(_ context.Context, m *domain.CompatibilityMatrix) error {
    if m == nil || strings.TrimSpace(m.ID) == "" {
        return fmt.Errorf("compatibility matrix: id is required")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.matrices[m.ID]; ok {
        return port.ErrCompatibilityMatrixExists
    }
    cp := *m
    r.matrices[m.ID] = &cp
    return nil
}

func (r *MemoryCompatibilityRepository) Update(_ context.Context, m *domain.CompatibilityMatrix) error {
    if m == nil || strings.TrimSpace(m.ID) == "" {
        return fmt.Errorf("compatibility matrix: id is required")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.matrices[m.ID]; !ok {
        return port.ErrCompatibilityMatrixNotFound
    }
    cp := *m
    r.matrices[m.ID] = &cp
    return nil
}

func (r *MemoryCompatibilityRepository) Delete(_ context.Context, id string) error {
    if strings.TrimSpace(id) == "" {
        return fmt.Errorf("compatibility matrix: id is required")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.matrices, id)
    return nil
}
```

(`port` import 추가. Deep-copy 는 `*m` 값 복사로 충분하나, 호출자가 내부 map 레퍼런스를 공유할 수 있으니 `m.Tools` 는 새 map 으로 clone 후 저장하는 것을 권장. 기존 `GetAll/GetByID` 처럼 `cp := *m` 만으로 충분하다고 판단되면 그대로 사용.)

#### A.2.3 백엔드: Postgres 구현

`internal/stack/adapter/repository/postgres_compatibility.go`:

```go
func (r *PostgresCompatibilityRepository) Create(ctx context.Context, m *domain.CompatibilityMatrix) error {
    toolsJSON, err := json.Marshal(m.Tools)
    if err != nil {
        return fmt.Errorf("marshal tools: %w", err)
    }
    const q = `
        INSERT INTO compatibility_matrices
            (id, name, status, k8s_min, k8s_max, k8s_recommended, tools)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (id) DO NOTHING`
    tag, err := r.pool.Exec(ctx, q,
        m.ID, m.Name, m.Status,
        m.Kubernetes.Min, m.Kubernetes.Max, m.Kubernetes.Recommended,
        toolsJSON,
    )
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return port.ErrCompatibilityMatrixExists
    }
    return nil
}

func (r *PostgresCompatibilityRepository) Update(ctx context.Context, m *domain.CompatibilityMatrix) error {
    toolsJSON, err := json.Marshal(m.Tools)
    if err != nil {
        return fmt.Errorf("marshal tools: %w", err)
    }
    const q = `
        UPDATE compatibility_matrices
        SET name = $2, status = $3, k8s_min = $4, k8s_max = $5,
            k8s_recommended = $6, tools = $7
        WHERE id = $1`
    tag, err := r.pool.Exec(ctx, q,
        m.ID, m.Name, m.Status,
        m.Kubernetes.Min, m.Kubernetes.Max, m.Kubernetes.Recommended,
        toolsJSON,
    )
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return port.ErrCompatibilityMatrixNotFound
    }
    return nil
}

func (r *PostgresCompatibilityRepository) Delete(ctx context.Context, id string) error {
    _, err := r.pool.Exec(ctx, `DELETE FROM compatibility_matrices WHERE id = $1`, id)
    return err
}
```

#### A.2.4 백엔드: Validation Layer (use case 내부)

`internal/stack/usecase/manage_compatibility.go` 신규:

```go
type ManageCompatibility struct {
    repo        port.CompatibilityRepository
    cacheClearer VerdictCacheClearer // optional, nil 허용
}

type VerdictCacheClearer interface {
    Clear()
}

type ManageCompatibilityOption func(*ManageCompatibility)

func WithVerdictCacheClearer(c VerdictCacheClearer) ManageCompatibilityOption { ... }

func NewManageCompatibility(repo port.CompatibilityRepository, opts ...ManageCompatibilityOption) *ManageCompatibility { ... }

func (u *ManageCompatibility) Create(ctx context.Context, m *domain.CompatibilityMatrix) error
func (u *ManageCompatibility) Update(ctx context.Context, m *domain.CompatibilityMatrix) error
func (u *ManageCompatibility) Delete(ctx context.Context, id string) error
```

각 메서드에서 `validateMatrixPayload(m)` 로 입력 검증:

- `ID`: non-empty, `^[a-z0-9][a-z0-9-]{0,63}$` (kebab-case, URL-safe).
- `Name`: non-empty, trim 후 <= 120 chars.
- `Status`: ∈ `{"verified", "untested", "unsupported"}`.
- `Kubernetes.Min / Max / Recommended`: non-empty, SemVer-ish (`v?\d+\.\d+(\.\d+)?`).
- `Tools`: len > 0 && <= 32. 각 tool 값:
  - `Name`: non-empty, trim 후 <= 80.
  - `HelmVersion` / `AppVersion`: non-empty, <= 60.
  - `Tier` (optional, default "stable"): ∈ `{"stable", "beta", "deprecated"}`.
  - `ArchSupport` (optional, default `["amd64"]`): 비어있지 않으면 ⊆ `{"amd64", "arm64"}`, dedup + 정렬.
  - `MinK8sVersion` (optional): 빈 값 허용, 있으면 Kubernetes 버전 포맷.

검증 실패 시 `AppError{Code: "COMPATIBILITY_VALIDATION", HTTPStatus: 400, Message: ...}` 형태로 반환. `internal/shared/apperror` 또는 기존 패턴 따라감.

성공 시 `u.cacheClearer != nil` 이면 `u.cacheClearer.Clear()` 호출 — 매트릭스 변경이 verdict 를 무효화해야 하기 때문.

#### A.2.5 백엔드: Handler 확장

`internal/stack/adapter/handler/compatibility_handler.go` 에 필드 + 라우트 추가:

```go
type CompatibilityHandler struct {
    compatRepo            port.CompatibilityRepository
    validateCompatibility *usecase.ValidateCompatibility
    manageCompatibility   *usecase.ManageCompatibility // nullable; nil 이면 CRUD 비활성
    auditSink             audit.Sink                    // nullable
}

type CompatibilityHandlerOption func(*CompatibilityHandler)

func WithManageCompatibility(u *usecase.ManageCompatibility) CompatibilityHandlerOption { ... }
func WithCompatibilityAuditSink(s audit.Sink) CompatibilityHandlerOption { ... }

func (h *CompatibilityHandler) RegisterAdminRoutes(g *echo.Group) {
    if h.manageCompatibility == nil {
        return
    }
    g.POST("/compatibility/matrices", h.Create)
    g.PUT("/compatibility/matrices/:id", h.Update)
    g.DELETE("/compatibility/matrices/:id", h.Delete)
}

func (h *CompatibilityHandler) Create(c echo.Context) error {
    var req compatibilityMatrixRequest
    if err := c.Bind(&req); err != nil { ... 400 COMPATIBILITY_REQUEST_INVALID }
    m := req.toDomain()
    if err := h.manageCompatibility.Create(c.Request().Context(), m); err != nil {
        return mapCompatibilityError(c, err)
    }
    h.audit(c, "create", m.ID)
    return c.JSON(http.StatusCreated, m)
}

// Update: PUT /admin/compatibility/matrices/:id. Path id 가 body id 와 다르면 400.
// Delete: DELETE /admin/compatibility/matrices/:id. 204 No Content.
```

`mapCompatibilityError` 는:
- `port.ErrCompatibilityMatrixExists` → 409
- `port.ErrCompatibilityMatrixNotFound` → 404
- `AppError{Code:"COMPATIBILITY_VALIDATION"}` → 400
- 기타 → 500

`compatibilityMatrixRequest` 는 snake_case JSON → domain 변환. 숨겨진 `matrix.ID` (URL param) 와 body `id` 가 같은지 확인 (Update 만).

Audit log: `action = "compatibility_matrix_create|update|delete"`, `target = matrix.ID`, `actor = echo context 의 user claim` (기존 handler 패턴 모방), `metadata = {"status": m.Status, "tool_count": len(m.Tools)}`. `audit.Sink` 가 nil 이면 skip.

#### A.2.6 백엔드: main.go 와이어링

`cmd/api/main.go` 에서 `compatHandler` 구성 부분 수정:

```go
manageCompatUC := stackuc.NewManageCompatibility(pgCompatRepo,
    stackuc.WithVerdictCacheClearer(verdictCache),
)
compatHandler := stackhandler.NewCompatibilityHandler(
    pgCompatRepo,
    validateCompatUC,
    stackhandler.WithManageCompatibility(manageCompatUC),
    stackhandler.WithCompatibilityAuditSink(auditSink), // 기존 auditSink 변수 재사용
)

// ... 라우트 등록
compatHandler.RegisterRoutes(v1)                 // 기존 public
compatHandler.RegisterAdminRoutes(v1Admin)       // 신규, admin-gated 그룹
```

`v1Admin` 이 이미 있다면 재사용, 없으면 admin role guard 미들웨어가 걸린 그룹 (기존 `/admin/clusters`, `/admin/users` 등과 동일 경로 prefix).

`VerdictCacheClearer` 는 `MemoryVerdictCache` 가 `Clear()` 를 이미 노출하고 있는지 확인 (Phase 6 에서 추가됨). 없으면 Phase 6 와 경계 충돌이므로 그대로 Phase 5 에서 `Clear()` 메서드를 `verdict_cache.go` 에 보강 (sync.Map 전체 clear) — 단, Phase 6 의 TTL/key 시맨틱은 건드리지 말 것.

#### A.2.7 백엔드: 테스트 (최소 케이스)

1. `internal/stack/adapter/repository/memory_compatibility_test.go`:
   - `Create` 성공 → `GetByID` 로 round-trip.
   - `Create` 중복 ID → `ErrCompatibilityMatrixExists`.
   - `Update` 성공 → 다시 `GetByID` 로 변경 확인.
   - `Update` 존재하지 않는 ID → `ErrCompatibilityMatrixNotFound`.
   - `Delete` 존재 → 이후 `GetByID` 에러.
   - `Delete` 존재하지 않는 ID → nil (멱등성).

2. `internal/stack/adapter/repository/postgres_integration_test.go` (integration tag):
   - `Create` → `GetByID` round-trip.
   - `Update` → 값 반영 확인.
   - `Delete` → `GetByID` 에러.
   - 중복 `Create` 는 `ErrCompatibilityMatrixExists`.

3. `internal/stack/usecase/manage_compatibility_test.go`:
   - valid payload → repo Create 호출됨 + cacheClearer.Clear() 호출 확인 (fake 로 count).
   - invalid status ("bogus") → `AppError COMPATIBILITY_VALIDATION`.
   - invalid archSupport (`["s390x"]`) → 400.
   - invalid tools (0 개) → 400.
   - missing ID → 400.
   - cacheClearer nil 허용 → no-panic.

4. `internal/stack/adapter/handler/compatibility_crud_handler_test.go`:
   - `POST /compatibility/matrices` 201 + JSON body.
   - duplicate → 409.
   - invalid JSON → 400.
   - `PUT /compatibility/matrices/:id` path != body.id → 400.
   - `PUT /compatibility/matrices/:id` not found → 404.
   - `DELETE /compatibility/matrices/:id` 204.
   - 각 성공 케이스가 audit.Sink 에 한 레코드 남기는지 (MemorySink 로 assert).

#### A.2.8 프론트엔드: API 훅

`web/src/features/stack/api/stack-api.ts`:

```ts
export function useCreateMatrix() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: MatrixInput) =>
      api.post<CompatibilityMatrix>('/admin/compatibility/matrices', matrixToPayload(input))
        .then((r) => r.data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: queryKeys.compatibility() }) },
  })
}

export function useUpdateMatrix() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: MatrixInput & { id: string }) =>
      api.put<CompatibilityMatrix>(`/admin/compatibility/matrices/${input.id}`, matrixToPayload(input))
        .then((r) => r.data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: queryKeys.compatibility() }) },
  })
}

export function useDeleteMatrix() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      api.delete<void>(`/admin/compatibility/matrices/${id}`).then(() => undefined),
    onSuccess: () => { qc.invalidateQueries({ queryKey: queryKeys.compatibility() }) },
  })
}
```

`MatrixInput` / `matrixToPayload` 는 ISO 규칙을 따라 snake_case 변환. 기존 `normalizeCompatibilityTool` 의 역변환에 해당.

#### A.2.9 프론트엔드: MatrixEditModal 컴포넌트

`web/src/features/admin/components/matrix-edit-modal.tsx` 신규:

- Props: `{ mode: 'create' | 'edit', initial?: CompatibilityMatrix, open, onClose, onSaved }`.
- 상태: `draft` (로컬 state), mutation 오브젝트 (create 또는 update).
- 섹션:
  1. **Identity**: ID (create 모드에서만 편집 가능), Name, Status (select). 각 필드에 inline validation 경고.
  2. **Kubernetes**: Min / Max / Recommended (text, v1.27.0 형태).
  3. **Tools**: 동적 배열 에디터 — `tools[category] = {name, helmVersion, appVersion, tier, archSupport, minK8sVersion}`. 각 행에 "Remove" 버튼, 맨 아래 "Add tool" 버튼. category 는 text (postgres / redis / minio 등), free-form.
  4. **Actions**: Cancel / Save. Save 클릭 시 client-side validation (status/tier 값, archSupport ⊆ {amd64,arm64}, 최소 1 tool) → mutation → onSaved.

- i18n: `stackVersionsAdmin.modal.{title.create,title.edit,kubernetes.min,kubernetes.max,kubernetes.recommended,tools.addTool,tools.remove,actions.save,actions.cancel,validation.*}` (ko/en).

#### A.2.10 프론트엔드: Delete Confirm Dialog

기존 `components/ui/` 에 Confirm Dialog 가 있으면 재사용, 없으면 `components/shared/confirm-dialog.tsx` 신규 — Props `{ open, title, message, confirmLabel, onConfirm, onCancel, danger?: boolean }`. `danger` 면 confirm 버튼 색을 destructive (`#ef4444`) 로.

#### A.2.11 프론트엔드: stack-versions-page.tsx 통합

- 헤더 우측에 `<Button onClick={openCreate}>{t('stackVersionsAdmin.actions.new', 'New matrix')}</Button>` 추가.
- 상세 패널 상단에 Edit / Delete 버튼 row — `selectedMatrix` 존재할 때만. Delete 는 `danger` Confirm Dialog 게이트.
- Modal 컨트롤 useState: `modalState: {mode:'create' | 'edit'} | null`.
- 성공 시 modal close + 현재 선택 유지 (edit) 또는 새로 생성된 ID 로 selection 전환 (create).

#### A.2.12 프론트엔드: 테스트

`web/src/features/admin/pages/stack-versions-page.test.tsx` 신규 또는 기존 확장:

- New matrix 클릭 → modal open (role=dialog) + title "New matrix".
- Fill form + Save → `useCreateMatrix.mutate` 호출 확인 (MSW 로 스텁).
- Edit 클릭 → modal open with initial values pre-filled.
- Delete 클릭 → Confirm dialog 노출 → Confirm → `useDeleteMatrix.mutate` 호출.
- Validation: status 빈 값 → Save disabled or error.

`web/src/features/stack/api/stack-api.test.ts` (또는 신규 `stack-api-matrix.test.ts`):

- `useCreateMatrix` → 201 body 수신 + invalidate 호출.
- `useUpdateMatrix` → PUT URL + body 검증.
- `useDeleteMatrix` → DELETE URL.
- 에러 핸들링: 409 → mutation.error 에 노출.

### A.3 Phase A 완료 체크리스트

- [ ] `port.CompatibilityRepository` 에 Create/Update/Delete 3 메서드 + 2 sentinel error.
- [ ] Memory / Postgres repository 양쪽 구현.
- [ ] `ManageCompatibility` use case + 입력 validation + optional cacheClearer.
- [ ] 3 admin 엔드포인트 + `mapCompatibilityError` + audit 통합.
- [ ] `MatrixEditModal` + Confirm Dialog + stack-versions-page 통합 + 3 훅.
- [ ] i18n ko/en 추가.
- [ ] 백엔드 단위 테스트 4 파일 + 프론트 테스트 2 파일 (최소).
- [ ] `go test ./...`, `go vet ./...` 클린.
- [ ] `npx tsc --noEmit` 클린.
- [ ] `npx vitest run web/src/features/admin web/src/features/stack/api` 클린.
- [ ] `docs/plans/compatibility_matrix_plan.md §6 F8-Phase5` DROPPED → `[x]` 전환 + 간단한 완료 요약.
- [ ] CHANGELOG Unreleased 항목 추가.

### A.4 Phase A 중지 및 검증 (Stop & Verify)

Phase A 커밋 직전에:

1. `git diff --stat` 으로 변경 파일 개수 확인 — 20~25 파일 내외가 정상 범위.
2. `go test ./internal/stack/... ./cmd/... ./internal/shared/... -count=1 | tail -20` — 모두 PASS.
3. `go vet ./... && go vet -tags e2e ./...` — 클린.
4. `cd web && npx tsc --noEmit && npx vitest run --reporter=dot` — 클린.
5. `grep -rn "compatibility_matrix_create\|compatibility_matrix_update\|compatibility_matrix_delete" internal/ cmd/ | wc -l` — audit action 상수가 한 지점에서 관리되는지 점검.
6. cURL 스모크 (dev 서버 ↑ 시):
   ```bash
   curl -X POST localhost:8080/api/v1/admin/compatibility/matrices -H 'Content-Type: application/json' -d '{"id":"test-matrix","name":"Test","status":"untested","kubernetes":{"min":"v1.27","max":"v1.29","recommended":"v1.28"},"tools":{"db":{"name":"Postgres","helm_version":"12.0.0","app_version":"16.0","tier":"beta","arch_support":["amd64","arm64"]}}}'
   curl -X PUT  localhost:8080/api/v1/admin/compatibility/matrices/test-matrix -H '...' -d '{...}'
   curl -X DELETE localhost:8080/api/v1/admin/compatibility/matrices/test-matrix
   ```

위 전부 클린하면 Phase A commit — `git commit -m "feat(stack): compatibility matrix CRUD UI (F8-Phase5)"`.

---

## Phase B — Retry UI 버튼 (F8-Phase3 후속)

### B.1 현재 상태 (2026-04-20)

- 백엔드 `POST /stacks/:id/retry` 엔드포인트 존재 (Phase 3 성과물).
- 프론트 `useRetryStack` 훅 + `stackApiCalls.retryStack` 존재.
- UI 연결점 없음 — 버튼을 아직 어디에도 노출하지 않았다.
- `stack-list-page.tsx` 의 `degradedState = ["failed", "rolling_back", "rolled_back", "cancelled"]` 분기에서 retry-eligible 상태 (`failed`, `rolled_back`) 만 발췌 가능.
- Retry 서버 동작: Phase 3 설계 상 stack state 가 `failed` 또는 `rolled_back` 에서 `pending` 으로 되감은 뒤 `runPreDeployGate` 공통 헬퍼 경유 → `fail` 이면 `DEPLOY_COMPAT_FAIL` 400, `warn` + ack 미확인 이면 `DEPLOY_COMPAT_WARN_UNACK` 400, 그 외 install 재시작.

### B.2 구현 범위

#### B.2.1 Retry 정책 헬퍼

`web/src/features/stack/utils/retry-policy.ts` 신규:

```ts
export type StackStatus =
  | 'pending' | 'validating' | 'installing' | 'configuring' | 'health_check'
  | 'completed' | 'failed' | 'rolling_back' | 'rolled_back' | 'cancelled'

export function canRetry(status: StackStatus): boolean {
  return status === 'failed' || status === 'rolled_back'
}
```

단위 테스트 `retry-policy.test.ts`: 모든 enum 값에 대한 truthy matrix.

#### B.2.2 RetryStackButton 컴포넌트

`web/src/features/stack/components/retry-stack-button.tsx` 신규:

- Props: `{ stackId: string, status: StackStatus, onRetried?: (stackId: string) => void }`.
- `canRetry(status)` false → null 반환(렌더 안 함).
- `useRetryStack` 훅 사용. Click:
  1. 서버 warn verdict 처리를 위해 먼저 `acknowledgeWarnings: false` 로 시도.
  2. 응답 에러가 `DEPLOY_COMPAT_FAIL` → 토스트 + 상세 issue 리스트.
  3. 응답 에러가 `DEPLOY_COMPAT_WARN_UNACK` → modal 로 warn issues + ack 체크박스 + "다시 시도" 버튼 → 체크 후 재호출 시 `acknowledgeWarnings: true`.
  4. 성공 → 토스트 "재배포가 시작되었습니다" + `onRetried(stackId)` 콜백 (부모가 `navigate('/stack/deploy/' + id)` 호출 가능).
- i18n: `stackList.retry.{button,confirmWarn.title,confirmWarn.ackLabel,toasts.success,toasts.failure}`.

`toDeployErrorMessage` (기존 `utils/deploy-error.ts`) + `extractDeployCompatError` 재사용. 이미 F8-F3 에서 구현됨.

#### B.2.3 Stack List 통합

`web/src/features/stack/pages/stack-list-page.tsx`:

- 기존 stack 카드 또는 expanded panel 의 action area (`degradedState` 인접) 에서 `<RetryStackButton stackId={stack.id} status={stack.status} onRetried={(id) => navigate(`/stack/deploy/${id}`)} />` 렌더.
- Status === `failed | rolled_back` 이 아니면 컴포넌트가 자기검열하므로 상위 조건 분기 추가 불필요.
- 카드 footer 에 이미 "View logs" / "Delete" 같은 버튼이 있으면 동일 row 에 배치, 없으면 새 button row 추가.

#### B.2.4 Stack Deployment Logs 통합 (optional, 작성자 판단)

`stack-deployment-logs-page.tsx` 의 `meta.result === 'failed'` 분기에도 RetryStackButton 노출 가능. 다만 현재 파일이 mock data 기반이므로 실제 stack status prop 으로 교체되는 후속 리팩토링 필요 — 본 Phase 에서는 **Do NOT touch** 하고 후속 follow-up 으로 넘긴다.

#### B.2.5 Playwright 스모크

`web/e2e/stack-retry-button.spec.ts` (`@stack-critical`) 신규:

- 주입 데이터: `failed` 상태의 스택 1개 (테스트 fixture 또는 API 시드).
- `/stack/list` 접근 → 해당 스택 카드에 "Retry" 버튼 노출 확인.
- `completed` 상태 스택은 Retry 버튼 부재 확인.
- 실제 click → API call 확인 (MSW 로 401 대신 200 스텁하거나, Kind 의존 없이 가짜 응답 반환).

Kind 의존 없는 순수 UI 스모크로 유지. 실제 deploy→completed 재현은 F8-F6-Cloud 범위.

#### B.2.6 테스트

1. `web/src/features/stack/utils/retry-policy.test.ts`: `canRetry` 10개 enum 각각.
2. `web/src/features/stack/components/retry-stack-button.test.tsx`:
   - `status: 'completed'` → 렌더 안 됨.
   - `status: 'failed'` → Retry 버튼 렌더.
   - click + 200 응답 → `onRetried` 콜백.
   - click + 400 `DEPLOY_COMPAT_WARN_UNACK` → warn modal 노출 + ack 체크박스.
   - warn modal 에서 ack + retry → `acknowledgeWarnings: true` 로 재호출.
   - click + 400 `DEPLOY_COMPAT_FAIL` → 토스트 + 재시도 불가.
3. `web/src/features/stack/pages/stack-list-page.test.tsx`: Retry 버튼이 `failed` 스택에만 노출되는지 스냅샷 변경 또는 새 테스트.

### B.3 Phase B 완료 체크리스트

- [ ] `retry-policy.ts` + 테스트.
- [ ] `RetryStackButton` 컴포넌트 + 단위 테스트.
- [ ] `stack-list-page` 통합 (단, 기존 레이아웃 회귀 없음 — `@stack-critical` Playwright 3/3 유지).
- [ ] warn-ack modal UX 동작.
- [ ] i18n ko/en 추가.
- [ ] `npx vitest run` 클린.
- [ ] `npx tsc --noEmit && npx eslint web/src/features/stack/components/retry-stack-button.tsx` 클린.
- [ ] Playwright @stack-critical 3/3 그린 (회귀 없음 확인).
- [ ] `docs/plans/compatibility_matrix_plan.md §6` "Retry UI 버튼" 항목을 `[x]` 로 전환 + 간단한 완료 요약.
- [ ] CHANGELOG Unreleased 항목 추가.

### B.4 Phase B 중지 및 검증 (Stop & Verify)

1. `git diff --stat` — 8~12 파일 내외가 정상.
2. `cd web && npx vitest run src/features/stack --reporter=dot` — 클린.
3. `cd web && npx tsc --noEmit` — 클린.
4. `cd web && npx eslint src/features/stack/components/retry-stack-button.tsx src/features/stack/utils/retry-policy.ts` — 0 errors.
5. `cd web && npx playwright test --grep @stack-critical` — 3/3 + 새 retry 스모크 통과.
6. Dev 서버 기동 시 수동 스모크: `failed` 상태 스택에서 Retry 버튼 가시성, click → API call, warn modal 분기.

위 전부 클린하면 Phase B commit — `git commit -m "feat(stack): retry UI button + warn-ack modal (F8 follow-up)"`.

---

## 최종 보고 템플릿

두 Phase 모두 완료 시 종합 보고:

```
F8 Phase5 재개 + Retry UI 통합 보고
===================================

Phase A (매트릭스 CRUD UI):
  - 상태: ✅ / ❌ DROP
  - 변경 파일: Backend N개, Frontend M개, Docs K개, Tests L개
  - 신규 엔드포인트: POST/PUT/DELETE /admin/compatibility/matrices[/:id]
  - 신규 UI: MatrixEditModal, ConfirmDialog, stack-versions-page 액션 버튼
  - 테스트 결과: go test ... / vitest ... / tsc ...
  - 관측 결함: ...

Phase B (Retry UI):
  - 상태: ✅ / ❌ DROP
  - 변경 파일: ...
  - 신규 UI: RetryStackButton, warn-ack modal
  - 테스트 결과: ...
  - 관측 결함: ...

남은 follow-up:
  - F8-F6-Cloud (cloud)
  - (Phase A drop 이면) F8-Phase5-Retry
  - deployment-logs-page Retry 통합 (mock→real status 리팩토링 필요)
  - verdict cache per-prefix invalidation
  - scheduler Prometheus 메타
```

---

## 부록: 관련 파일 빠른 참조

| 목적 | 파일 |
|---|---|
| 호환성 port | `internal/stack/port/compatibility.go` |
| Memory repo | `internal/stack/adapter/repository/memory_compatibility.go` |
| Postgres repo | `internal/stack/adapter/repository/postgres_compatibility.go` |
| Integration test | `internal/stack/adapter/repository/postgres_integration_test.go` |
| 기존 handler | `internal/stack/adapter/handler/compatibility_handler.go` |
| Deploy handler (Retry 참고) | `internal/stack/adapter/handler/deploy_handler.go` |
| ValidateCompatibility use case | `internal/stack/usecase/validate_compatibility.go` |
| Verdict cache | `internal/stack/usecase/verdict_cache.go` (F8-Phase6) |
| Audit Sink 인터페이스 | `internal/shared/audit/sink.go` (F8-Phase2) |
| Admin matrix page | `web/src/features/admin/pages/stack-versions-page.tsx` |
| Stack list page | `web/src/features/stack/pages/stack-list-page.tsx` |
| Stack install page (Retry ack UX 참고) | `web/src/features/stack/pages/stack-install-page.tsx` |
| stack-api (훅 추가 지점) | `web/src/features/stack/api/stack-api.ts` |
| admin-api (clusters 훅 패턴) | `web/src/features/admin/api/admin-api.ts` |
| 공유 타입 | `web/src/types.ts` 또는 `web/src/features/stack/api/stack-api.ts` 내 정의 |
| 기존 deploy 에러 유틸 | `web/src/features/stack/utils/deploy-error.ts`, `server-verdict.ts` |
| i18n 리소스 | `web/src/locales/ko.json`, `web/src/locales/en.json` |

---

## 끝.

Phase A → B 순서로 실행. 각 Phase 끝에 Stop & Verify 거친 뒤 commit. drop 조항은 §0.2 를 따른다.

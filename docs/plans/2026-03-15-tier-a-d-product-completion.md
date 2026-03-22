# Nullus Platform Tier A~D 제품 완성 작업 지시서

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Nullus Platform의 프론트엔드 MOCK 데이터를 실제 API 데이터로 전환하고, 백엔드 critical 이슈 수정, 테스트 커버리지 향상, 배포 인프라 완성, UX 폴리시까지 순차적으로 완료한다.

**Architecture:** Clean Architecture (Handler → UseCase → Domain). 모듈별 독립 Bounded Context. 프론트엔드는 TanStack Query 훅이 이미 존재하며, MOCK fallback만 제거하면 실제 API 연동 완료. 백엔드는 pgx/v5 raw SQL + Echo v4.

**Tech Stack:** Go 1.24+ (Echo v4), pgx/v5, React 19, TypeScript, TanStack Query, Vite, Tailwind CSS 4, shadcn/ui, Vitest, Playwright

---

## 실행 전 확인사항

- 로컬 인프라 기동: `./scripts/runbook_local.sh up`
- DB 마이그레이션 완료: `make migrate-up`
- 시드 데이터 존재 확인: `psql -h localhost -p 5433 -U nullus -d nullus_dev -c "SELECT count(*) FROM golden_path_templates;"`

---

## Tier A — 프로덕트 완성도 (Critical)

### Prompt 1: AlertRuleRepository Update/Delete 구현

**Files:**
- Modify: `internal/observability/port/repository.go` — AlertRuleRepository 인터페이스에 Update, Delete 메서드 추가
- Modify: `internal/observability/adapter/repository/postgres_alert.go` — PostgreSQL Update, Delete 구현
- Modify: `internal/observability/adapter/handler/alert_handler.go` — in-memory patchedRules/deletedRules 제거, 실제 repo 호출로 교체
- Create: `internal/observability/adapter/handler/alert_handler_test.go` — Update/Delete 핸들러 테스트

**구현 상세:**

1. `port/repository.go`의 AlertRuleRepository 인터페이스에 추가:
```go
Update(ctx context.Context, rule *domain.AlertRule) error
Delete(ctx context.Context, id string) error
```

2. `postgres_alert.go`에 구현:
```go
func (r *PostgresAlertRuleRepository) Update(ctx context.Context, rule *domain.AlertRule) error {
    const q = `UPDATE alert_rules SET name=$1, condition=$2, threshold=$3, channel=$4, enabled=$5, updated_at=NOW() WHERE id=$6`
    _, err := r.pool.Exec(ctx, q, rule.Name, rule.Condition, rule.Threshold, rule.Channel, rule.Enabled, rule.ID)
    return err
}

func (r *PostgresAlertRuleRepository) Delete(ctx context.Context, id string) error {
    const q = `DELETE FROM alert_rules WHERE id = $1`
    _, err := r.pool.Exec(ctx, q, id)
    return err
}
```

3. `alert_handler.go` 리팩토링:
- `sync.RWMutex`, `patchedRules`, `deletedRules` 필드 완전 제거
- `UpdateRule`에서 `h.alertRuleRepo.Update(ctx, &updated)` 호출
- `DeleteRule`에서 `h.alertRuleRepo.Delete(ctx, id)` 호출
- `ListRules`에서 in-memory 필터링 로직 제거, 단순히 `h.alertRuleRepo.List()` 반환

**검증:**
```bash
go test ./internal/observability/... -v -count=1
go build ./...
```

---

### Prompt 2: Known Issues DB 마이그레이션 + Repository 구현

**Files:**
- Create: `db/migrations/000008_known_issues.up.sql`
- Create: `db/migrations/000008_known_issues.down.sql`
- Create: `internal/admin/port/known_issues_repository.go` — 인터페이스 정의
- Create: `internal/admin/adapter/repository/postgres_known_issues.go` — PostgreSQL 구현
- Modify: `internal/admin/adapter/handler/known_issues_handler.go` — DB repository 주입, 하드코딩 제거
- Modify: `cmd/api/main.go` — KnownIssuesHandler 생성 시 repository 주입
- Create: `internal/admin/adapter/repository/postgres_known_issues_test.go`

**마이그레이션 SQL:**
```sql
CREATE TABLE IF NOT EXISTS known_issues (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    severity    VARCHAR(20) NOT NULL DEFAULT 'medium',
    title       VARCHAR(500) NOT NULL,
    description TEXT NOT NULL,
    workaround  TEXT DEFAULT '',
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 기존 하드코딩 데이터를 시드로 삽입
INSERT INTO known_issues (severity, title, description, workaround, status) VALUES
('medium', 'Helm install requires cluster admin', 'Helm-based stack installation currently requires cluster-admin role to create CRDs and cluster-scoped resources.', 'Use a temporary cluster-admin service account during installation, then rotate to least-privilege RBAC.', 'open'),
('low', 'Dashboard metrics delay', 'Prometheus cache TTL is 10s', 'Refresh page', 'acknowledged'),
('high', 'No automatic certificate renewal', 'Automatic certificate rotation is not wired into the current stack lifecycle jobs.', 'Manual cert-manager renewal', 'planned');
```

**Handler 리팩토링:**
```go
type KnownIssuesHandler struct {
    repo port.KnownIssuesRepository
}

func NewKnownIssuesHandler(repo port.KnownIssuesRepository) *KnownIssuesHandler {
    return &KnownIssuesHandler{repo: repo}
}

func (h *KnownIssuesHandler) ListKnownIssues(c echo.Context) error {
    items, err := h.repo.List(c.Request().Context())
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    return c.JSON(http.StatusOK, map[string]any{"items": items})
}
```

**main.go 변경:**
```go
// 기존: knownIssuesHandler := &adminhandler.KnownIssuesHandler{}
// 변경:
knownIssuesRepo := adminrepo.NewPostgresKnownIssuesRepository(pool)
knownIssuesHandler := adminhandler.NewKnownIssuesHandler(knownIssuesRepo)
```

**검증:**
```bash
make migrate-up
go test ./internal/admin/... -v -count=1
go build ./...
curl http://localhost:8090/api/v1/admin/known-issues | jq '.items | length'
# 기대값: 3
```

---

### Prompt 3: Audit Logging CRUD 핸들러 연동

**Files:**
- Modify: `internal/admin/adapter/handler/org_handler.go` — Create/Update org 시 audit log 기록
- Modify: `internal/admin/adapter/handler/cluster_handler.go` — Create/Update/Delete/Verify cluster 시 audit log 기록
- Modify: `internal/admin/adapter/handler/member_handler.go` — Invite/Remove member 시 audit log 기록
- Modify: `internal/stack/adapter/handler/stack_handler.go` — Create stack 시 audit log 기록
- Modify: `internal/stack/adapter/handler/deploy_handler.go` — Deploy 시 audit log 기록
- 각 핸들러에 `*audit.AuditLogger` 필드 추가 + 생성자 수정

**패턴 (모든 핸들러에 동일하게 적용):**
```go
// 핸들러 구조체에 필드 추가
type OrgHandler struct {
    useCase *usecase.OrgUseCase
    audit   *audit.AuditLogger  // 추가
}

// 생성자 수정
func NewOrgHandler(uc *usecase.OrgUseCase, audit *audit.AuditLogger) *OrgHandler {
    return &OrgHandler{useCase: uc, audit: audit}
}

// CUD 핸들러에서 audit 로깅 (성공 후에만)
func (h *OrgHandler) CreateOrganization(c echo.Context) error {
    // ... 기존 로직 ...
    // 성공 후:
    if h.audit != nil {
        _ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
            UserID:       c.Request().Header.Get("X-User-ID"),
            Action:       "create",
            ResourceType: "organization",
            ResourceID:   created.ID,
            Details:      map[string]any{"name": req.Name},
            IPAddress:    c.RealIP(),
        })
    }
    return c.JSON(http.StatusCreated, created)
}
```

**main.go 변경 — 모든 핸들러 생성자에 auditLogger 전달:**
```go
orgHandler := adminhandler.NewOrgHandler(orgUC, auditLogger)
clusterHandler := adminhandler.NewClusterHandler(clusterUC, auditLogger)
memberHandler := adminhandler.NewMemberHandler(userUC, auditLogger)
stackHandler := stackhandler.NewStackHandler(createStackUC, listStacksUC, pgStackRepo, auditLogger)
deployHandler := stackhandler.NewDeployHandler(installStackUC, pgStackRepo, memStreamer, auditLogger)
```

**MUST NOT:**
- audit 로깅 실패가 핸들러 응답에 영향을 주면 안 됨 (fire-and-forget, 에러 무시)
- Read 요청(GET)에는 audit 로깅하지 않음 — CUD만

**검증:**
```bash
go build ./...
go test ./internal/admin/... ./internal/stack/... -v -count=1
# API 호출 후 audit_logs 테이블 확인
curl -X POST http://localhost:8090/api/v1/admin/orgs -H 'Content-Type: application/json' -d '{"name":"test","slug":"test"}'
psql -h localhost -p 5433 -U nullus -d nullus_dev -c "SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 5;"
```

---

### Prompt 4: 프론트엔드 Organization/Members 라우트 정합 + MOCK 제거

**Files:**
- Modify: `internal/admin/adapter/handler/org_handler.go` — 프론트엔드 API 계약에 맞는 라우트 추가
  - GET `/admin/organization` → 첫 번째 org 반환 (프론트엔드가 단일 org 가정)
  - PATCH `/admin/organization` → 첫 번째 org 업데이트
  - GET `/admin/organizations/:orgId/members` → 해당 org의 멤버 목록
  - POST `/admin/organizations/:orgId/members` → 멤버 초대
  - DELETE `/admin/organizations/:orgId/members/:memberId` → 멤버 제거
- Modify: `internal/admin/adapter/handler/member_handler.go` — org-scoped 멤버 라우트
- Modify: `web/src/features/admin/pages/organization-page.tsx` — MOCK_ORG, MOCK_MEMBERS, ALL_CLUSTERS 제거
- Modify: `web/src/features/admin/pages/organization-page.tsx` — useClusters() 훅으로 클러스터 목록 동적 로딩

**프론트엔드 API 계약 (admin-api.ts 기준):**
```
GET  /api/v1/admin/organization              → Organization
PATCH /api/v1/admin/organization             → Organization
GET  /api/v1/admin/organizations/:orgId/members → { items: Member[], total: number }
POST /api/v1/admin/organizations/:orgId/members → Member
DELETE /api/v1/admin/organizations/:orgId/members/:memberId → void
```

**organization-page.tsx 변경:**
```typescript
// 삭제: const MOCK_ORG = { ... }
// 삭제: const MOCK_MEMBERS = [ ... ]
// 삭제: const ALL_CLUSTERS = [ ... ]

// 변경:
const { data: orgData, isLoading: orgLoading } = useOrganization()
const org = orgData  // MOCK fallback 제거
if (!org || orgLoading) return <LoadingSpinner />

const { data: membersData } = useMembers(org.id)
const members = membersData?.items ?? []

const { data: clustersData } = useClusters()
const allClusters = (clustersData?.items ?? []).map(c => c.name)
```

**검증:**
```bash
go build ./...
# 백엔드 기동 후 프론트엔드에서 Organization 페이지 접근
curl http://localhost:8090/api/v1/admin/organization | jq
curl http://localhost:8090/api/v1/admin/organizations/org-1/members | jq
cd web && npx vitest run && npx vite build
```

---

### Prompt 5: 프론트엔드 Stack 페이지 MOCK 제거 (stack-list, stack-history, stack-version)

**Files:**
- Modify: `web/src/features/stack/pages/stack-list-page.tsx`
- Modify: `web/src/features/stack/pages/stack-history-page.tsx`
- Modify: `web/src/features/stack/pages/stack-version-page.tsx`

**stack-list-page.tsx:**
```typescript
// 삭제: const MOCK_STACKS: Stack[] = [ ... ]
// 변경:
const { data: apiData, isLoading } = useStacks({ search, status: statusFilter || undefined })
const stacks = apiData?.items ?? []
// isLoading 시 로딩 상태 표시, 빈 배열이면 emptyMessage 표시
```

**stack-history-page.tsx:**
```typescript
// 삭제: const MOCK_HISTORY: StackHistoryEntry[] = [ ... ]
// 추가: import { useStackHistory, useRollbackStack } from '../api/stack-api'
// 변경:
const stackId = useParams<{ id: string }>().id ?? ''  // URL 파라미터에서 stackId
const { data: history, isLoading } = useStackHistory(stackId)
const entries = Array.isArray(history) ? history : []
const rollbackMutation = useRollbackStack()

const handleRollbackConfirm = () => {
    if (!rollbackEntry || !stackId) return
    rollbackMutation.mutate(
        { stackId, version: rollbackEntry.version },
        { onSuccess: () => setRollbackEntry(null) }
    )
}
// setTimeout 제거, rollbackMutation.isPending을 loading으로 사용
```

**stack-version-page.tsx:**
```typescript
// 삭제: const MOCK_MATRIX: CompatibilityMatrix[] = [ ... ]
// 삭제: const MOCK_VALIDATION: CompatibilityValidationResult = { ... }
// 추가: import { useCompatibilityMatrix, useValidateCompatibility } from '../api/stack-api'
// 변경:
const { data: matrixData, isLoading } = useCompatibilityMatrix()
const matrix = Array.isArray(matrixData) ? matrixData : []

// Validate Current Stack 버튼:
const validateMutation = useValidateCompatibility(selectedStackId)
const handleValidate = () => {
    setValidationOpen(true)
    validateMutation.mutate(undefined, {
        onSuccess: (result) => setValidationResult(result),
    })
}
// setTimeout(1200) 제거
```

**MUST NOT:**
- 기존 UI 레이아웃, 스타일 변경 금지
- 새로운 npm 패키지 추가 금지
- TanStack Query 훅 시그니처 변경 금지

**검증:**
```bash
cd web && npx vitest run && npx vite build
# 브라우저에서 Stack List, Stack History, Compatibility Matrix 페이지 확인
```

---

### Prompt 6: 프론트엔드 Monitoring Dashboard + Developer Deploy MOCK 제거

**Files:**
- Modify: `web/src/features/observability/pages/monitoring-page.tsx`
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx`

**monitoring-page.tsx:**
```typescript
// 삭제: const MOCK_DASHBOARD: MonitoringDashboard = { ... }
// 변경:
const { data: apiData, isLoading } = useDashboard(5000)
// apiData가 없으면 로딩 표시, 있으면 그대로 사용
if (isLoading || !apiData) return <LoadingState />
const { kpi, pipeline, tools } = apiData
```
- 주의: 백엔드가 Prometheus 미설정 시 in-memory fallback 대시보드를 반환하므로 프론트엔드 fallback 불필요

**developer-deploy-page.tsx:**
```typescript
// 삭제: const APP_TEMPLATES: [...] = [ ... ]  (하드코딩 앱 템플릿 목록)
// 삭제: const CLUSTERS = [ ... ]  (하드코딩 클러스터 목록)
// 추가: import { useAppTemplates } from '../api/cicd-api'
// 추가: import { useClusters } from '../../admin/api/admin-api'
// 변경:
const { data: appTemplatesData } = useAppTemplates()
const appTemplates = appTemplatesData ?? []

const { data: clustersData } = useClusters()
const clusters = (clustersData?.items ?? []).map(c => ({
    id: c.id,
    name: c.name,
    namespaces: c.namespaces ?? ['default'],
}))

// 수정: onError에서 setDeployed(true) 제거
deployMutation.mutate(request, {
    onSuccess: () => setDeployed(true),
    // onError 제거 — 에러 시 toast 알림으로 처리
})
```

**검증:**
```bash
cd web && npx vitest run && npx vite build
```

---

## Tier B — 품질 & 테스트

### Prompt 7: CICD 모듈 핸들러/유스케이스 테스트

**Files:**
- Create: `internal/cicd/adapter/handler/pipeline_handler_test.go`
- Create: `internal/cicd/adapter/handler/cicd_template_handler_test.go`
- Create: `internal/cicd/usecase/create_pipeline_test.go`
- Create: `internal/cicd/usecase/list_pipelines_test.go`
- Create: `internal/cicd/usecase/deploy_pipeline_test.go`

**테스트 명명 규칙:**
```go
func TestPipelineHandler_Create_Success(t *testing.T) { ... }
func TestPipelineHandler_Create_InvalidBody(t *testing.T) { ... }
func TestPipelineHandler_List_WithFilters(t *testing.T) { ... }
func TestPipelineHandler_Deploy_PipelineNotFound(t *testing.T) { ... }
```

**테스트 패턴:**
- 핸들러 테스트: `httptest.NewRecorder` + `echo.NewContext` + mock 유스케이스
- 유스케이스 테스트: mock repository 주입, 성공/실패 시나리오
- 모든 테스트 파일은 대상 파일과 같은 디렉토리에 `_test.go`

**검증:**
```bash
go test ./internal/cicd/... -v -count=1 -cover
# 목표: cicd 모듈 커버리지 50%+
```

---

### Prompt 8: Observability 모듈 핸들러/유스케이스 테스트

**Files:**
- Create: `internal/observability/adapter/handler/alert_handler_test.go`
- Create: `internal/observability/adapter/handler/dashboard_handler_test.go`
- Create: `internal/observability/usecase/create_alert_rule_test.go`
- Create: `internal/observability/usecase/list_alerts_test.go`

**테스트 시나리오:**
- AlertHandler: Create, Update, Delete, List 각 성공/실패 케이스
- DashboardHandler: GetDashboard 성공, Prometheus 연결 실패 fallback
- CreateAlertRule: 정상 생성, 중복 이름, 잘못된 채널
- ListAlerts: 빈 목록, 다수 알림

**검증:**
```bash
go test ./internal/observability/... -v -count=1 -cover
# 목표: observability 모듈 커버리지 50%+
```

---

### Prompt 9: Auth 미들웨어 + Admin 핸들러 테스트 보강

**Files:**
- Create: `internal/auth/adapter/middleware/dual_auth_test.go`
- Create: `internal/auth/adapter/middleware/auth_middleware_test.go`
- Create: `internal/admin/adapter/handler/member_handler_test.go`
- Create: `internal/admin/adapter/repository/postgres_org_test.go` (testcontainers 사용)

**테스트 시나리오:**
- DualAuth: session 모드, OIDC 모드, 유효하지 않은 토큰
- AuthMiddleware: 세션 있음/없음, 만료된 세션
- MemberHandler: Invite, Remove, Role update, 존재하지 않는 멤버
- PostgresOrg: CRUD 통합 테스트 (testcontainers-go)

**검증:**
```bash
go test ./internal/auth/... ./internal/admin/... -v -count=1 -cover
# 전체 커버리지 확인:
go test ./... -cover -count=1 2>&1 | grep -E "^ok|FAIL"
# 목표: 전체 >70%
```

---

### Prompt 10: E2E Playwright CI 통합

**Files:**
- Modify: `.github/workflows/ci.yml` — Playwright E2E 테스트 job 추가
- Create: `web/e2e/smoke.spec.ts` — 주요 페이지 렌더링 + 네비게이션 E2E
- Modify: `web/package.json` — e2e 스크립트 추가 (아직 없다면)

**CI 구성:**
```yaml
  e2e:
    runs-on: ubuntu-latest
    needs: [backend, frontend]
    services:
      postgres:
        image: postgres:18
        env:
          POSTGRES_DB: nullus_dev
          POSTGRES_USER: nullus
          POSTGRES_PASSWORD: nullus_dev
        ports: ['5433:5432']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - run: go build -o bin/api ./cmd/api
      - run: make migrate-up
      - run: bin/api &
      - run: cd web && npm ci && npx playwright install --with-deps
      - run: cd web && npx playwright test
```

**E2E 테스트 범위:**
- 로그인 → 대시보드 → Stack List → Stack Install Wizard → Monitoring
- 각 역할별 접근 가능 페이지 확인 (admin, devops, developer)

**검증:**
```bash
cd web && npx playwright test --reporter=list
```

---

## Tier C — 배포 & 운영

### Prompt 11: Helm 차트 templates 완성

**Files:**
- Create: `deploy/helm/nullus/templates/deployment.yaml`
- Create: `deploy/helm/nullus/templates/service.yaml`
- Create: `deploy/helm/nullus/templates/ingress.yaml`
- Create: `deploy/helm/nullus/templates/configmap.yaml`
- Create: `deploy/helm/nullus/templates/secret.yaml`
- Create: `deploy/helm/nullus/templates/migration-job.yaml`
- Create: `deploy/helm/nullus/templates/_helpers.tpl`
- Create: `deploy/helm/nullus/templates/NOTES.txt`
- Modify: `deploy/helm/nullus/values.yaml` — 누락 필드 추가

**배포 구조:**
- API Deployment (Go 바이너리, liveness/readiness probe)
- Web Deployment (nginx + static SPA files)
- API Service (ClusterIP :8080)
- Web Service (ClusterIP :80)
- Ingress (optional, host-based routing)
- ConfigMap (config.yaml 내용)
- Secret (DB credentials, Keycloak client secret)
- Migration Job (마이그레이션 실행 후 api 시작 전)

**검증:**
```bash
helm lint deploy/helm/nullus/
helm template nullus deploy/helm/nullus/ --values deploy/helm/nullus/values.yaml
```

---

### Prompt 12: CHANGELOG.md + ROADMAP.md 작성

**Files:**
- Create: `CHANGELOG.md`
- Create: `ROADMAP.md`

**CHANGELOG 구조:**
```markdown
# Changelog

## [0.1.0-alpha] - 2026-03-15

### Added
- F0-F10 전체 기능 구현 (PRD v1.3 Phase 1)
- Clean Architecture + DDD 모듈 구조 (5 Bounded Context)
- Helm SDK 기반 스택 자동 설치 엔진 (3-Phase DAG)
- Keycloak OIDC 인증 + 3단계 RBAC (Admin/DevOps/Developer)
- ... (git log --oneline 기반 주요 변경사항)

### Infrastructure
- Docker multi-stage build + Helm chart
- GitHub Actions CI (Go test + Vite build + Vitest)
- testcontainers-go 통합 테스트 인프라
```

**ROADMAP 구조:**
- v0.1.0-alpha (현재): Phase 1 기능 완료
- v0.2.0-beta: 테스트 커버리지 >70%, E2E 자동화, 프로덕션 배포 가이드
- v1.0.0 GA: Keycloak SSO 연동 검증, 멀티 클러스터 지원, 성능 최적화

**검증:**
```bash
# 마크다운 린트 (선택)
cat CHANGELOG.md | head -20
cat ROADMAP.md | head -20
```

---

### Prompt 13: Production Keycloak CI 자동화

**Files:**
- Modify: `scripts/setup-keycloak.sh` — idempotent하게 수정 (이미 존재하면 skip)
- Modify: `.github/workflows/ci.yml` — Keycloak realm setup step 추가 (선택적, staging 환경)
- Create: `scripts/keycloak-realm-export.json` — 표준 realm 설정 파일 (테스트 사용자 포함)

**Keycloak Realm 설정:**
- Realm: `nullus`
- Client: `nullus-web` (public, PKCE)
- Roles: `admin`, `devops`, `developer`
- Test users: admin@nullus.io, devops@nullus.io, dev@nullus.io (비밀번호: nullus123!)

**검증:**
```bash
./scripts/setup-keycloak.sh
# 실패 시 재실행해도 안전한지 확인 (idempotent)
./scripts/setup-keycloak.sh
```

---

## Tier D — 폴리시

### Prompt 14: React Console Warnings + Accessibility 수정

**Files:**
- Modify: `web/src/features/stack/pages/stack-list-page.tsx` — duplicate key 수정
- Modify: `web/src/features/stack/pages/stack-version-page.tsx` — Fragment key 추가 (map 내 `<>` → `<Fragment key={...}>`)
- Modify: `web/src/features/stack/pages/stack-template-page.tsx` — button nesting 수정
- Modify: `web/src/features/cicd/pages/cicd-list-page.tsx` — filter pill `<span>` → `<button>` 교체
- Modify: `web/src/components/layout/admin-sidebar.tsx` — "Known Issues" 메뉴 링크 추가

**Known Issues 사이드바 추가:**
```typescript
// admin-sidebar.tsx에 메뉴 항목 추가:
{ label: 'Known Issues', path: '/admin/known-issues', icon: AlertTriangle }
```

**접근성 수정:**
```typescript
// filter pill: <span onClick={...}> → <button type="button" onClick={...}>
// 동일 스타일 유지, 시맨틱 요소만 변경
```

**검증:**
```bash
cd web && npx vitest run && npx vite build
# 브라우저 DevTools Console에서 경고 없음 확인
```

---

### Prompt 15: 계획 체크박스 업데이트 + 최종 정리

**Files:**
- Modify: `.sisyphus/plans/nullus-product-completion.md` — 15개 태스크 체크박스 모두 체크

**변경:**
```markdown
# 기존: - [ ] Prompt 1: ...
# 변경: - [x] Prompt 1: ...
```

모든 15개 프롬프트에 대해 `[ ]` → `[x]` 변경.

**최종 검증 (전체 빌드 + 테스트):**
```bash
go build ./...
go test ./... -count=1
cd web && npx vitest run && npx vite build
```

---

## 실행 순서 및 병렬화 가이드

### 병렬 실행 가능 그룹:

**Tier A — 순차 실행 권장 (의존성 있음)**
- Prompt 1 → 2 → 3 (백엔드 수정, 순차)
- Prompt 4 (독립 — 1,2,3과 병렬 가능)
- Prompt 5, 6 (프론트엔드, Prompt 1-4 완료 후)

**Tier B — 병렬 실행 가능**
- Prompt 7, 8, 9 (모듈별 독립 테스트, 동시 실행)
- Prompt 10 (E2E, 7-9 완료 후)

**Tier C — 병렬 실행 가능**
- Prompt 11, 12, 13 (완전 독립, 동시 실행)

**Tier D — 순차 실행**
- Prompt 14 → 15

### 전체 검증 체크리스트:

- [x] `go build ./...` — 에러 0
- [x] `go test ./... -count=1` — 전체 PASS
- [x] `cd web && npx vitest run` — 전체 PASS (150/150)
- [x] `cd web && npx vite build` — 빌드 성공
- [ ] 프론트엔드에서 MOCK_ 상수 grep 결과 0건 (observability/cicd 일부 잔존 — Prompt 6 미완)
- [ ] `curl http://localhost:8090/api/v1/admin/organization` — 200 OK
- [ ] `curl http://localhost:8090/api/v1/admin/known-issues` — DB 기반 데이터 반환
- [x] 브라우저 Console 경고 0건 (button nesting, MOCK fallback 수정 완료)

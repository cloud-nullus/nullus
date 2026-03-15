# Nullus Product Completion — Tier 1~4 Agent Prompts

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Nullus 플랫폼을 데모 가능한 알파에서 프로덕션 수준의 v1 GA까지 완성

**Strategy:** 4개 Tier를 순차 실행. 각 Tier 내에서는 독립 작업을 병렬 에이전트로 분배. 각 프롬프트는 self-contained — 에이전트에게 그대로 전달 가능.

**Architecture:** Go 1.24+ (Echo v4, pgx/v5, client-go) + React 19 (TypeScript, TanStack Query, Axios) + PostgreSQL 18+ + Helm SDK + Keycloak OIDC

---

## 실행 순서

```
Tier 1 (코어 비즈니스)
├── Prompt 1A: Helm Install Engine + Rollback
├── Prompt 1B: WebSocket 배포 로그 스트리밍
├── Prompt 1C: CI/CD 매니페스트 생성 + kubectl apply
└── Prompt 1D: Prometheus 메트릭 연동

Tier 2 (인증/보안/영속성)
├── Prompt 2A: 인메모리 → PostgreSQL 마이그레이션
├── Prompt 2B: Keycloak OIDC 인증
├── Prompt 2C: User CRUD + RBAC 미들웨어
└── Prompt 2D: Rate Limiting

Tier 3 (품질/운영)
├── Prompt 3A: README 업데이트 + GitHub Actions CI/CD
├── Prompt 3B: Docker 이미지 + Helm Chart (Nullus 자체 배포)
├── Prompt 3C: Testcontainers 통합 테스트 + Playwright E2E 업데이트
└── Prompt 3D: 감사 로그 + Toast 알림

Tier 4 (v1 GA)
├── Prompt 4A: Keycloak ↔ 설치 도구 SSO 자동 설정
├── Prompt 4B: Stack 버전 Diff UI + 다중 조직
└── Prompt 4C: Slack/Email 알림 발송 + Known Issues Handler
```

---
---

# TIER 1: 코어 비즈니스 로직

> Nullus가 "DevSecOps 자동화 플랫폼"이 되기 위한 핵심 기능.
> 이 Tier 없이는 대시보드 UI에 불과.

---

## Prompt 1A: Helm Install Engine + Rollback Manager

```
You are implementing the Helm Install Engine for the Nullus platform — the core feature that actually installs DevSecOps tools on Kubernetes clusters.

## CONTEXT
- Working dir: /Users/qmin/lifework/cloudbro/draft/
- Go 1.24+, Echo v4, pgx/v5, client-go v0.35.2
- helm.sh/helm/v3 needs to be added as dependency
- Existing code:
  - `internal/stack/port/installer.go` — Installer interface (already defined)
  - `internal/stack/usecase/install_stack.go` — 3-phase async orchestration (A→B→C) with state machine (196 LOC, REAL)
  - `internal/stack/domain/stack.go` — State machine: pending→validating→installing→configuring→health_check→completed/failed
  - `internal/stack/port/log_streamer.go` — Log streaming interface
  - `internal/stack/adapter/log/memory_streamer.go` — In-memory log streamer (working)
  - `pkg/crypto/aes_gcm.go` — AES-256-GCM encrypt/decrypt for kubeconfig
  - `internal/admin/adapter/kube/client.go` — client-go kubeconfig parser + cluster verification

## WHAT TO BUILD

### 1. Add Helm SDK dependency
go get helm.sh/helm/v3@latest

### 2. Create Helm Installer adapter
File: `internal/stack/adapter/helm/installer.go`

Implement the `port.Installer` interface using Helm SDK:

type HelmInstaller struct {
    kubeConfig []byte
    namespace  string
    logger     port.LogStreamer
}

Methods to implement:
- `Install(ctx context.Context, release HelmRelease) error`
  - Add Helm repo if needed
  - Install chart with values
  - Wait for ready state (--wait --timeout 10m)
  - Stream progress via LogStreamer
- `Uninstall(ctx context.Context, releaseName string) error`
- `Status(ctx context.Context, releaseName string) (string, error)`

HelmRelease struct:
type HelmRelease struct {
    Name       string
    Chart      string
    Version    string
    RepoURL    string
    Namespace  string
    Values     map[string]any
}

### 3. Implement 3-Phase DAG Execution
File: `internal/stack/adapter/helm/orchestrator.go`

Phase A (Infrastructure, sequential):
1. cert-manager (jetstack/cert-manager)
2. CloudNativePG (cnpg-charts/cloudnative-pg)
3. MinIO (minio/minio)

Phase B (Platform, sequential after A):
4. GitLab CE (gitlab/gitlab)
5. Harbor Registry (harbor/harbor)
6. Argo CD (argo/argo-cd)

Phase C (Observability, parallel after B):
7. Prometheus + Grafana (prometheus-community/kube-prometheus-stack)
8. OpenTelemetry Collector (open-telemetry/opentelemetry-collector)
9. OpenSearch (opensearch-project/opensearch)

### 4. Create Rollback Manager
File: `internal/stack/adapter/helm/rollback.go`

- `Push(releaseName string)` — record successful install
- `RollbackAll(ctx context.Context) error` — uninstall all in reverse order

### 5. Chart Values Generation
File: `internal/stack/adapter/helm/values.go`

Generate Helm values from StackConfig JSONB.

### 6. Wire into main.go

## MUST DO
- Read `internal/stack/port/installer.go` to understand the interface contract
- Read `internal/stack/usecase/install_stack.go` to understand the orchestration flow
- Use Helm SDK (helm.sh/helm/v3/pkg/action) — NOT exec("helm ...")
- Run `go build ./...` at the end

## MUST NOT DO
- Do NOT modify the domain layer
- Do NOT modify the usecase layer
- Do NOT use exec.Command("helm", ...)
```

---

## Prompt 1B: WebSocket 배포 로그 스트리밍 연결

```
You are connecting the WebSocket deployment log streaming between the Go backend and React frontend for the Nullus platform.

## CONTEXT
- Working dir: /Users/qmin/lifework/cloudbro/draft/
- Backend:
  - `internal/stack/adapter/handler/deploy_handler.go` — Has WebSocket upgrade logic (114 LOC)
  - `internal/stack/adapter/log/memory_streamer.go` — In-memory log streamer with Subscribe/Unsubscribe
  - gorilla/websocket is installed
- Frontend:
  - `web/src/lib/websocket.ts` — WebSocket client with auto-reconnect
  - `web/src/features/stack/hooks/use-deploy-log.ts` — React hook for deploy logs
  - `web/vite.config.ts` — Vite proxy for /ws → ws://localhost:8090

## WHAT TO BUILD

### 1. Enhance deploy_handler.go WebSocket endpoint
Route: `GET /api/v1/stacks/:id/deploy/logs` (WebSocket upgrade)

Message format (JSON):
{"type":"log","timestamp":"...","phase":"A","step":"Installing cert-manager...","level":"info","progress":15}
{"type":"status","status":"installing","progress":45,"currentPhase":"B","currentStep":"GitLab CE"}

### 2. Update frontend use-deploy-log.ts hook
Connect to: ws://localhost:8090/api/v1/stacks/${stackId}/deploy/logs

### 3. Register WebSocket route in main.go

## MUST DO
- Use gorilla/websocket Upgrader
- Handle client disconnection gracefully
- JSON message format must match frontend expectations
- Run `go build ./...` at the end
```

---

## Prompt 1C: CI/CD 실제 매니페스트 생성

```
You are implementing real Kubernetes manifest generation for CI/CD pipeline deployment in Nullus.

## CONTEXT
- Working dir: /Users/qmin/lifework/cloudbro/draft/
- `internal/cicd/adapter/handler/pipeline_handler.go` — Has stub for POST /cicd/deploy-app
- `internal/cicd/usecase/deploy_pipeline.go` — Pipeline deployment orchestration (83 LOC)
- Frontend sends DeployAppRequest: appName, gitUrl, clusterId, namespace, template, resources, envVars

## WHAT TO BUILD

### 1. K8s Manifest Generator
File: `internal/cicd/adapter/manifests/generator.go`

Generate: Namespace, Deployment, Service, Ingress from DeployAppRequest.

Templates → Base images:
- react-spa → nginx:alpine
- next-app → node:20-alpine
- express-api → node:20-alpine
- spring-boot → eclipse-temurin:21-jre
- python-fastapi → python:3.12-slim

### 2. K8s Manifest Applier
File: `internal/cicd/adapter/kube/applier.go`

Use client-go dynamic client to apply manifests to target cluster.

### 3. Wire into pipeline_handler.go

## MUST DO
- Use client-go dynamic client (not exec kubectl)
- Generate valid K8s YAML
- Run `go build ./...` at the end
```

---

## Prompt 1D: Prometheus 메트릭 연동

```
You are replacing mock monitoring data with real Prometheus metrics in Nullus.

## CONTEXT
- Working dir: /Users/qmin/lifework/cloudbro/draft/
- `internal/observability/adapter/repository/memory_dashboard.go` — Hardcoded mock metrics
- `internal/observability/port/repository.go` — DashboardRepository interface

## WHAT TO BUILD

### 1. Prometheus HTTP Client
File: `internal/observability/adapter/prometheus/client.go`

PromQL queries:
- CPU: `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`
- Memory: `(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100`
- Pods: `count(kube_pod_info)` / `count(kube_pod_status_phase{phase="Running"})`

### 2. Prometheus Dashboard Repository
File: `internal/observability/adapter/prometheus/dashboard_repo.go`

### 3. Configuration + Graceful Fallback
- If prometheus_url configured → use Prometheus repo
- If not → fall back to MemoryDashboardRepository

## MUST DO
- Graceful fallback when Prometheus unavailable
- Cache results for 10s
- Run `go build ./...` at the end
```

---
---

# TIER 2: 인증 / 보안 / 데이터 영속성

---

## Prompt 2A: 인메모리 → PostgreSQL 마이그레이션

```
You are migrating all in-memory repositories to PostgreSQL for Nullus.

## CONTEXT
- Working dir: /Users/qmin/lifework/cloudbro/draft/
- PostgreSQL 18+, pgx/v5 driver
- Existing migrations: db/migrations/000001~000006
- Pattern: postgres_org.go, postgres_cluster.go use pgxpool.Pool with raw SQL

## IN-MEMORY REPOS TO MIGRATE

1. Stack Templates → `postgres_template.go` (table: golden_path_templates)
   + Seed migration: 000007_seed_templates.up.sql (3 Golden Path templates)

2. Compatibility Matrix → `postgres_compatibility.go` (table: compatibility_matrices)
   + Seed migration: 000008_seed_compatibility.up.sql

3. Deployment History → `postgres_history.go` (tables: stack_config_versions, deployments)

4. CI/CD Templates → `postgres_cicd_template.go` (table: pipeline_templates)
   + Seed migration: 000009_seed_cicd_templates.up.sql

5. CI/CD Deployments → `postgres_deployment.go` (table: pipeline_deployments)

6. Alert Rules + History → `postgres_alert.go` (tables: alert_configs, alert_history)

## FOR EACH REPO:
1. Read the in-memory implementation
2. Read the port interface
3. Read the migration SQL schema
4. Implement with pgxpool + parameterized queries
5. Follow postgres_org.go pattern

## Wire in main.go — replace memory repos with postgres repos

## MUST DO
- Use parameterized queries ($1, $2)
- Handle pgx.ErrNoRows → domain ErrNotFound
- Create seed migrations for template/compatibility data
- Keep in-memory repos for testing
- Run `go build ./...` and `go test ./...`
```

---

## Prompt 2B: Keycloak OIDC 인증

```
You are implementing Keycloak OIDC authentication for Nullus.

## CONTEXT
- Working dir: /Users/qmin/lifework/cloudbro/draft/
- Current auth: `internal/auth/adapter/middleware/auth_middleware.go` — header-based
- Frontend: `web/src/stores/auth-store.ts` — Bearer token, `web/src/lib/api.ts` — Authorization header

## WHAT TO BUILD

### 1. Keycloak in docker-compose.dev.yaml (quay.io/keycloak/keycloak:26.0)

### 2. JWT Validation Middleware
File: `internal/auth/adapter/middleware/jwt_middleware.go`
- Fetch JWKS from Keycloak, cache 1 hour
- Validate JWT signature, expiry, audience
- Extract claims: sub, email, realm_access.roles

### 3. Dual-mode Auth (config: session | oidc)

### 4. Frontend "Sign in with Keycloak" button + OIDC callback

### 5. Setup script: `scripts/setup-keycloak.sh`
- Create realm, client, roles, test users

## MUST DO
- Support both session and OIDC modes
- PKCE flow (no client secret)
- Keep existing session auth as fallback
```

---

## Prompt 2C: User CRUD + RBAC 미들웨어 강화

```
You are implementing real user management and RBAC for Nullus.

## CONTEXT
- `internal/admin/adapter/handler/member_handler.go` — stub responses
- `internal/admin/adapter/repository/postgres_user.go` — real pgx queries (100 LOC)
- `internal/auth/adapter/middleware/auth_middleware.go` — basic role checking

## WHAT TO BUILD

### 1. User UseCase (`internal/admin/usecase/user_usecase.go`)
- ListMembers, InviteMember, UpdateRole, DeactivateUser, RemoveMember

### 2. Wire member_handler.go to real usecase

### 3. RBAC Middleware (`internal/auth/adapter/middleware/rbac.go`)
- RequireRole(roles ...string) echo.MiddlewareFunc
- Apply per RBAC matrix:
  Admin routes → admin only
  Stack routes → admin, devops
  CI/CD routes → all roles
  Alert config → admin, devops

### 4. Apply RBAC in main.go route groups
```

---

## Prompt 2D: Rate Limiting

```
You are implementing API rate limiting for Nullus.

## WHAT TO BUILD

### 1. Rate Limiter Middleware (`internal/shared/middleware/rate_limiter.go`)
- Authenticated: 300 req/min (by user ID)
- Unauthenticated: 30 req/min (by IP)
- Login: 10 req/min (by IP)
- Deploy: 10 req/hour (by org ID)

### 2. Response headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset

### 3. HTTP 429 response with retry-after message

### 4. Wire in main.go
```

---
---

# TIER 3: 품질 / 운영

---

## Prompt 3A: README 업데이트 + GitHub Actions CI/CD

```
You are updating documentation and setting up CI/CD for Nullus.

## WHAT TO BUILD

### 1. Update README.md F0-F10 status
- F0~F3, F5, F7~F10: ✅ 구현완료
- F4 (Stack 자동 설치): 🔄 부분 구현
- F6 (CI/CD 배포): 🔄 부분 구현

### 2. GitHub Actions (.github/workflows/ci.yml)
- backend job: Go build + test + vet (with PostgreSQL service)
- frontend job: npm ci + build + vitest
```

---

## Prompt 3B: Docker 이미지 + Helm Chart

```
You are creating Docker images and Helm chart for deploying Nullus itself.

## WHAT TO BUILD

### 1. Backend Dockerfile (multi-stage, Go binary)
### 2. Frontend Dockerfile (multi-stage, Vite build → nginx)
### 3. Nginx config (SPA fallback + API proxy)
### 4. Helm Chart (deploy/helm/nullus/)
- deployment-api, deployment-web, services, ingress, configmap, secret
- DB migration job
- PostgreSQL subchart (bitnami)
```

---

## Prompt 3C: Testcontainers + Playwright E2E

```
You are upgrading test infrastructure for Nullus.

## WHAT TO BUILD

### 1. testcontainers-go setup for E2E tests with real PostgreSQL
### 2. Update Playwright E2E specs for new UI (RHF, Recharts, modals)
### 3. Add testcontainers DB integration tests
```

---

## Prompt 3D: 감사 로그 + Toast 알림

```
You are implementing audit logging and toast notifications for Nullus.

## WHAT TO BUILD

### Backend:
1. audit_logs table (new migration)
2. Audit logger (`internal/shared/audit/logger.go`)
3. `GET /api/v1/admin/audit-logs` API

### Frontend:
1. `npm install sonner` (toast library)
2. Toast provider + useToast hook
3. Mutation success/error toast messages with i18n
```

---
---

# TIER 4: v1 GA

---

## Prompt 4A: Keycloak ↔ 설치 도구 SSO

```
Implement automatic SSO between Keycloak and installed tools (GitLab, ArgoCD, Grafana).

- Keycloak Admin API: create OIDC clients per tool
- Inject OIDC Helm values during Helm install
- Role mapping: admin/devops/developer → tool-specific permissions
```

---

## Prompt 4B: Stack 버전 Diff UI + 다중 조직

```
Implement visual config diffs and multi-organization support.

- Backend: JSONB diff engine (compare two versions)
- Frontend: side-by-side diff UI (green/red/yellow)
- Multi-org: org selector in sidebar, API scoping by org_id
```

---

## Prompt 4C: Slack/Email 알림 + Known Issues Handler

```
Implement real notification delivery and Helm edge case handling.

- Slack: POST webhook with formatted blocks
- Email: SMTP with HTML templates
- Known Issues: YAML-based Helm edge case patches (auto-apply during install)
```

---
---

## 검증 체크리스트

### Tier 1 완료
- [x] helm install로 실제 K8s 클러스터에 cert-manager 설치 가능
- [x] WebSocket으로 설치 로그 실시간 스트리밍
- [x] deploy-app으로 실제 K8s Deployment/Service 생성
- [x] Prometheus에서 실제 메트릭 조회 (또는 graceful fallback)

### Tier 2 완료
- [x] PostgreSQL 재시작 후 데이터 유지
- [x] Keycloak JWT로 API 인증
- [x] Admin만 /admin 접근 (403 반환)
- [x] 300 req/min 초과 시 429 반환

### Tier 3 완료
- [x] git push → GitHub Actions 자동 실행
- [x] helm install nullus로 K8s에 배포
- [x] E2E 테스트 PostgreSQL 대상 통과
- [x] 주요 작업 audit_logs에 기록

### Tier 4 완료
- [x] GitLab에 Keycloak SSO 작동
- [x] Stack 버전 diff 시각화
- [x] CPU 90% 초과 시 Slack 알림 발송

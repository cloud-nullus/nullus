# Nullus v0.2 상세 기능 명세 및 시스템 아키텍처

**작성일**: 2026-03-30
**버전**: 0.2
**기반 문서**: Nullus 상세 기능 명세 및 시스템 아키텍처 v0.1, 실제 구현 코드 분석
**대상 독자**: 엔지니어, 아키텍트, DevOps Engineer
**현재 버전**: v0.2.0-alpha

---

## 변경 이력

| 버전 | 일자 | 변경 내용 |
|------|------|-----------|
| v0.1 | 2026-03-03 | 초기 아키텍처 설계 (PRD 기반) |
| v0.2 | 2026-03-30 | 구현 기준 아키텍처 동기화, 설계-구현 차이 분석 반영 |

---

## v0.1 대비 주요 변경 사항 요약

### 아키텍처 변경

| 항목 | v0.1 (설계) | v0.2 (구현) | 변경 사유 |
|------|-------------|-------------|-----------|
| 코드 구조 | 레이어드 (`handler/service/repository`) | Clean Architecture + DDD (`domain/port/usecase/adapter`) | 모듈 독립성 및 테스트 용이성 강화 |
| 모듈 경계 | 단일 `internal/` 내 기능 폴더 | 5개 Bounded Context (admin/stack/cicd/observability/auth) | DDD 적용으로 도메인별 완전 분리 |
| 설치 엔진 위치 | 독립 `internal/engine/` | `internal/stack/adapter/helm/` | Stack 도메인 안에 어댑터로 통합 |
| 호환성 엔진 | 독립 `internal/compatibility/` | `internal/stack/` 도메인 내부 | Stack Bounded Context에 통합 |
| 인증 구현 | 세션 기반(gorilla/sessions) | Dual Auth(세션 헤더 + Keycloak OIDC/JWT) | 개발/운영 모드 동시 지원 |
| API 경로 | `/api/v1/orgs`, `/api/v1/clusters` 등 독립 경로 | `/api/v1/admin/` 접두사로 통합 | Admin 모듈 일관성 |
| Helm 차트 위치 | `charts/nullus/` | `deploy/helm/nullus/` | 배포 관련 아티팩트 그룹핑 |

### 기능 상태 변경

| 기능 | v0.1 계획 릴리스 | v0.2 실제 상태 | 비고 |
|------|------------------|----------------|------|
| F0: Organization 관리 | Alpha | ✅ 구현 완료 | 멤버 초대/역할 관리 포함 |
| F1: Cluster 등록 | Alpha | ✅ 구현 완료 | kubeconfig AES-256 암호화 |
| F2: Stack 설정 UI | Alpha (3단계) | ✅ 구현 완료 (5단계) | 전체 5단계 워크플로우 구현 |
| F3: Golden Path 템플릿 | Alpha | ✅ 구현 완료 (3 종) | DB 시드 + CRUD API |
| F4: Stack 설치/배포 | Alpha | ✅ 구현 완료 | 3-Phase DAG + Helm SDK |
| F5: CI/CD 템플릿 | Beta | ✅ 구현 완료 | web/backend/batch 3종 |
| F6: CI/CD 배포/이력 | Beta | ✅ 구현 완료 | manifest 생성 + K8s 적용 |
| F7: 모니터링/알림 | Beta | ✅ 구현 완료 | Prometheus 프록시 + AlertRule CRUD |
| F8: 호환성 관리 | Alpha | ✅ 구현 완료 | DB 기반 매트릭스 |
| F9: RBAC | v1 | ✅ 구현 완료 (앞당김) | Admin/DevOps/Developer 3역할 |
| F10: 리소스 계산 | Alpha | ✅ 구현 완료 | resource-defaults 테이블 기반 |
| F11: 감사 로그 | (미계획) | ✅ 신규 구현 | audit_logs 테이블 + API |
| F12: Known Issues | (미계획) | ✅ 신규 구현 | DB 기반 이슈 레지스트리 |

### 미구현 / 축소된 기능

| 기능 | v0.1 설계 | v0.2 상태 | 사유 |
|------|-----------|-----------|------|
| Nullus Operator (K8s) | 대상 클러스터에 에이전트 배포 | ❌ 미구현 | API 서버에서 직접 Helm/kubectl 실행으로 대체 |
| 네임스페이스 분리 배포 | nullus-artifacts/scm/cicd/monitoring/logging 분리 | 단일 namespace 중심 배포 | 복잡도 감소 |
| OpenSearch/Loki 로깅 | 설치 대상 포함 | ❌ 미구현 | Phase 2 예정 |
| Harbor 컨테이너 레지스트리 | 도구 옵션 | ❌ 미구현 | GitLab Registry로 대체 |
| YAML 에디터 (Monaco) | v1 계획 | ⚠️ Monaco 탑재, 스택 설정 편집 용 | 양방향 동기화는 미구현 |
| 비용 추정 (클라우드별) | v1 계획 | ❌ 미구현 | resource-defaults로 기본값만 관리 |
| 파이프라인 롤백/diff | v1 계획 | ❌ 미구현 | 기본 배포/이력만 구현 |
| Nullus Operator | K8s Operator | ❌ 미구현 | Direct API call 방식 채택 |
| OpenAPI 자동 생성 | swaggo/swag | ❌ 미적용 | 수동 API 문서 |
| gRPC | 향후 도입 | ❌ 미도입 | REST + WebSocket으로 충분 |

---

## Part 1: 시스템 아키텍처

---

### 1. 아키텍처 개요

Nullus는 **3개의 런타임 환경**에 걸쳐 동작합니다. v0.1 설계와 달리, 대상 클러스터에 별도 Operator를 배포하지 않고 컨트롤 플레인에서 직접 Helm SDK + client-go로 클러스터를 관리합니다.

```
┌─────────────────────────────────────────────────────────────────┐
│                        사용자 브라우저                            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │              Nullus Web UI (React 19 + TypeScript)         │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────┐  │  │
│  │  │ Admin    │ │ Stack    │ │ CI/CD    │ │ Observ-     │  │  │
│  │  │ 관리     │ │ 설치     │ │ 파이프    │ │ ability     │  │  │
│  │  │          │ │ /배포    │ │ 라인     │ │ 모니터링    │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────────┘  │  │
│  └─────────────────────────┬─────────────────────────────────┘  │
└────────────────────────────┼────────────────────────────────────┘
                             │ REST API + WebSocket
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Nullus 컨트롤 플레인 (단일 프로세스)              │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │               Nullus API Server (Go 1.26 + Echo v4)       │   │
│  │                                                           │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────────┐  │   │
│  │  │ Admin   │ │ Stack   │ │ CI/CD   │ │ Observability │  │   │
│  │  │ Module  │ │ Module  │ │ Module  │ │ Module        │  │   │
│  │  │         │ │         │ │         │ │               │  │   │
│  │  │ Org     │ │ Template│ │ Pipeline│ │ Dashboard     │  │   │
│  │  │ Cluster │ │ Deploy  │ │ Deploy  │ │ AlertRule     │  │   │
│  │  │ Member  │ │ History │ │ Template│ │ AlertHistory  │  │   │
│  │  │ Audit   │ │ Compat. │ │         │ │               │  │   │
│  │  └─────────┘ └────┬────┘ └────┬────┘ └───────┬───────┘  │   │
│  │                    │          │               │           │   │
│  │  ┌─── Auth Module (Dual: Session + OIDC/JWT) ────────┐   │   │
│  │  │  SessionAuth │ JWTAuth │ RBAC Middleware            │   │   │
│  │  └────────────────────────────────────────────────────┘   │   │
│  └───────────────────────────────────────────────────────────┘   │
│                                                                   │
│  ┌──────────┐  ┌───────────────────┐  ┌────────────────────┐    │
│  │PostgreSQL│  │ Helm Orchestrator │  │ Prometheus Client  │    │
│  │ (pgx v5) │  │ (Helm SDK v3.20)  │  │ (HTTP Proxy)       │    │
│  │          │  │  3-Phase DAG      │  │                    │    │
│  │ 15+ 테이블│  │  + Log Streamer   │  │ 대시보드/메트릭    │    │
│  └──────────┘  └────────┬──────────┘  └────────────────────┘    │
│                          │                                       │
└──────────────────────────┼───────────────────────────────────────┘
                           │ Helm Install / kubectl apply
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                 대상 Kubernetes 클러스터                          │
│                                                                  │
│  ┌─ nullus-stack NS (또는 사용자 지정) ──────────────────────┐  │
│  │                                                            │  │
│  │  Phase A: cert-manager, metrics-server, PostgreSQL, MinIO  │  │
│  │  Phase B: GitLab CE, ArgoCD, GitLab Runner                │  │
│  │  Phase C: Prometheus, Grafana, OpenTelemetry, Gateway API │  │
│  │                                                            │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌─ app-{name} NS ─────────────────────────────────────────┐   │
│  │  사용자 애플리케이션 (CI/CD 파이프라인으로 배포)           │   │
│  │  Deployment + Service + Ingress + Secret                  │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

> **v0.1 대비 변경**: Nullus Operator 제거, 네임스페이스 분리(nullus-artifacts/scm/cicd/monitoring/logging) 대신 단일 namespace 중심 배포로 단순화, Prometheus Client 추가.

---

### 2. 비기능 요구사항 (NFR)

| 지표 | P95 목표 | P99 목표 | 측정 방법 | v0.2 상태 |
|------|---------|---------|-----------|-----------|
| REST API 응답 | < 200ms | < 500ms | Prometheus histogram | 미측정 (인프라 미구성) |
| WebSocket 연결 지연 | < 100ms | < 300ms | 클라이언트 측정 | ✅ 구현됨 (메모리 기반) |
| 설치 로그 유실률 | < 0.1% | < 1% | 송신/수신 카운트 비교 | 메모리 기반 Pub/Sub |
| 대시보드 초기 로딩 | < 2s | < 5s | Lighthouse | Lazy loading 적용 |

---

### 3. 공통 API 규격

#### 3.1 표준 에러 응답 형식 (구현 기준)

```json
{
  "error": {
    "code": "STACK_CONFIG_INVALID",
    "http_status": 400,
    "message": "스택 설정이 유효하지 않습니다"
  }
}
```

> **v0.1 대비 변경**: `detail`, `retryable`, `trace_id` 필드는 미구현. 에러 코드 네이밍은 `{DOMAIN}_{REASON}` 패턴으로 단순화됨.

#### 3.2 구현된 에러 코드

| 에러 코드 | HTTP Status | 설명 |
|-----------|-------------|------|
| `STACK_CONFIG_INVALID` | 400 | 스택 설정 유효성 실패 |
| `STACK_NOT_FOUND` | 404 | 스택 미존재 |
| `DEPLOY_FAILED` | 500 | 배포 실패 |
| `PIPELINE_CONFIG_INVALID` | 400 | 파이프라인 설정 유효성 실패 |
| `PIPELINE_LIST_FAILED` | 500 | 파이프라인 목록 조회 실패 |
| `CLUSTER_IN_USE` | 409 | 사용 중인 클러스터 삭제 시도 |
| `CLUSTER_NOT_FOUND` | 404 | 클러스터 미존재 |
| `COMPATIBILITY_CHECK_FAILED` | 500 | 호환성 검증 실패 |

---

### 4. 기술 스택

| 계층 | v0.1 (설계) | v0.2 (구현) | 변경 사유 |
|---|---|---|---|
| **Frontend** | React 19 + TypeScript | React 19.2 + TypeScript 5.9 | 업데이트 |
| **빌드 도구** | (미명시) | Vite 8 | 빠른 HMR |
| **상태 관리** | Zustand | Zustand + TanStack Query (React Query) | 서버 상태 캐싱 추가 |
| **스타일링** | Tailwind CSS + shadcn/ui | Tailwind CSS 4 + shadcn/ui | 동일 |
| **YAML 에디터** | Monaco Editor | Monaco Editor | 동일 |
| **라우팅** | (미명시) | React Router v7 (Lazy Loading) | SPA 라우팅 |
| **폼 처리** | (미명시) | React Hook Form + Zod | 타입 안전 폼 검증 |
| **i18n** | (미명시) | i18next | 다국어 지원 준비 |
| **차트** | (미명시) | Chart.js + Recharts | 대시보드 시각화 |
| **Backend** | Go 1.24+ | Go 1.26.1 | 버전 업그레이드 |
| **웹 프레임워크** | Echo v4 | Echo v4.15.1 | 동일 |
| **실시간 통신** | gorilla/websocket | gorilla/websocket | 동일 |
| **Database** | PostgreSQL 18+ | PostgreSQL 17+ (Docker) | 안정 버전 |
| **DB 드라이버** | (미명시) | pgx v5 | 고성능 PostgreSQL 드라이버 |
| **마이그레이션** | golang-migrate | golang-migrate | 동일 |
| **인증 (Dev)** | 세션 기반(gorilla/sessions) | 헤더 기반 세션 (X-User-* 헤더) | 단순화 |
| **인증 (Prod)** | Keycloak OIDC | Keycloak OIDC + Authentik 지원 | 다중 IdP |
| **설치 엔진** | Helm Go SDK + client-go | Helm SDK v3.20.1 + client-go | 동일 |
| **암호화** | AES-256 | AES-256-GCM (pkg/crypto) | 인증 암호화 |
| **E2E 테스트** | (미명시) | Playwright | 브라우저 E2E |
| **API 문서** | OpenAPI 3.0 (swaggo/swag) | (미적용) | 자동 생성 미도입 |

---

### 5. 코드 아키텍처 (Clean Architecture + DDD)

v0.1에서 설계한 레이어드 구조(`handler/service/repository`)와 달리, 실제 구현은 **Clean Architecture + DDD Bounded Context** 패턴을 적용했습니다.

#### 5.1 디렉토리 구조 (구현 기준)

```
nullus/
├── cmd/
│   └── api/
│       └── main.go              # DI + 라우트 등록 + 서버 부트스트랩
│
├── internal/
│   ├── admin/                   # BC: Organization Management
│   │   ├── domain/              # Entity: Organization, User, Cluster
│   │   ├── port/                # Repository/Service 인터페이스
│   │   ├── usecase/             # CreateOrg, RegisterCluster, ManageMember 등
│   │   └── adapter/
│   │       ├── handler/         # HTTP Handler (Echo)
│   │       └── repository/      # PostgreSQL 구현체
│   │
│   ├── stack/                   # BC: Stack Management
│   │   ├── domain/              # Entity: Stack, Template, History, Compatibility
│   │   ├── port/                # StackRepo, TemplateRepo, HelmInstaller 등
│   │   ├── usecase/             # CreateStack, InstallStack, ManageHistory 등
│   │   └── adapter/
│   │       ├── handler/         # StackHandler, DeployHandler, TemplateHandler 등
│   │       ├── repository/      # PostgreSQL 구현체
│   │       ├── helm/            # Helm Orchestrator, Installer, LogStreamer
│   │       └── kube/            # KubeconfigProvider
│   │
│   ├── cicd/                    # BC: CI/CD Pipeline
│   │   ├── domain/              # Entity: Pipeline, PipelineTemplate, Deployment
│   │   ├── port/                # PipelineRepo, DeploymentRepo 인터페이스
│   │   ├── usecase/             # CreatePipeline, DeployPipeline
│   │   └── adapter/
│   │       ├── handler/         # PipelineHandler, CICDTemplateHandler
│   │       ├── repository/      # PostgreSQL 구현체
│   │       └── kube/            # ManifestApplier, StepTracker
│   │
│   ├── observability/           # BC: Observability
│   │   ├── domain/              # Entity: AlertRule, Alert, Dashboard
│   │   ├── port/                # AlertRuleRepo, DashboardRepo 인터페이스
│   │   ├── usecase/             # CRUD AlertRule, GetDashboard
│   │   └── adapter/
│   │       ├── handler/         # DashboardHandler, AlertHandler
│   │       └── repository/      # PostgreSQL + Prometheus 구현체
│   │
│   ├── auth/                    # BC: Authentication & Authorization
│   │   ├── middleware/          # SessionAuth, JWTAuth, DualAuth, RBAC
│   │   └── provider/           # OIDCProvider, KeycloakProvider, AuthentikProvider
│   │
│   └── shared/                  # 크로스커팅 관심사
│       ├── audit/               # AuditLogger
│       ├── config/              # Viper 기반 설정 로더
│       ├── domain/              # EventBus (정의됨, 미사용)
│       ├── middleware/          # Logging, ErrorHandler, RateLimiter, OrgContext
│       └── notification/        # Notifier (플레이스홀더)
│
├── pkg/
│   └── crypto/                  # AES-256-GCM 암/복호화 유틸리티
│
├── db/
│   └── migrations/              # 30+ SQL 마이그레이션 (Up/Down)
│
├── configs/
│   └── config.yaml              # 애플리케이션 설정
│
├── deploy/
│   └── helm/nullus/             # Nullus 자체 Helm 차트
│
├── templates/
│   └── compatibility/           # 호환성 매트릭스 (YAML 시드)
│
├── web/                         # React 프론트엔드
│   └── src/
│       ├── app/                 # 라우트 + 레이아웃
│       ├── features/            # 기능별 모듈 (admin/stack/cicd/observability/auth)
│       ├── components/          # 공통 UI 컴포넌트 (shadcn)
│       ├── lib/                 # API 클라이언트, Query 설정
│       ├── hooks/               # 커스텀 훅
│       ├── stores/              # Zustand 스토어
│       ├── types/               # TypeScript 타입 정의
│       └── i18n.ts              # 국제화 설정
│
├── e2e/                         # Playwright E2E 테스트
├── scripts/                     # 로컬 개발/운영 스크립트
├── Dockerfile                   # Go API 이미지
├── docker-compose.dev.yaml      # 개발 인프라 (PostgreSQL, MinIO, Redis, Keycloak)
├── docker-compose.auth.yaml     # 인증 테스트 환경
├── Makefile                     # 빌드/테스트/배포 자동화
├── go.mod / go.sum
└── README.md
```

> **v0.1 대비 변경**: `internal/handler/`, `internal/service/`, `internal/repository/` 레이어드 구조 → 각 Bounded Context 내에 `domain/port/usecase/adapter` 계층을 가진 Clean Architecture로 전환. `internal/engine/` → `internal/stack/adapter/helm/`으로 이동. `charts/` → `deploy/helm/`으로 이동.

#### 5.2 의존성 흐름

```
┌─────────────────────────────────────────┐
│  Handler (adapter/handler)              │  ← HTTP 진입점 (Echo)
│  - Request 파싱, 응답 변환              │
├─────────────────────────────────────────┤
│  UseCase (usecase)                      │  ← 애플리케이션 로직
│  - 비즈니스 흐름 조율                    │
│  - Port 인터페이스를 통해 인프라 호출     │
├─────────────────────────────────────────┤
│  Domain (domain)                        │  ← 순수 비즈니스 규칙
│  - Entity, Value Object                 │
│  - 외부 의존성 없음                      │
├─────────────────────────────────────────┤
│  Port (port)                            │  ← 인터페이스 정의
│  - Repository, Service 계약             │
├─────────────────────────────────────────┤
│  Adapter (adapter/repository, helm, kube)│  ← 인프라 구현
│  - PostgreSQL, Helm SDK, K8s client     │
└─────────────────────────────────────────┘

의존 방향: Handler → UseCase → Domain/Port ← Adapter (의존성 역전)
```

#### 5.3 Bounded Context 간 통신

현재 모듈 간 통신은 main.go에서의 DI를 통한 **직접 참조**로 구현됩니다. EventBus 인터페이스가 `internal/shared/domain/event.go`에 정의되어 있으나, 실제로는 사용되지 않습니다.

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────────┐
│  Admin   │     │  Stack   │     │  CI/CD   │     │ Observability│
│          │     │          │     │          │     │              │
│ Org      │────→│ Cluster  │←────│ Cluster  │     │ Prometheus   │
│ Cluster  │     │ Template │     │ Pipeline │     │ AlertRule    │
│ Member   │     │ Deploy   │     │ Deploy   │     │ Dashboard    │
└──────────┘     └──────────┘     └──────────┘     └──────────────┘
      │               │                │                   │
      └───────────────┴────────────────┴───────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │  Auth Module      │
                    │  Session/JWT/RBAC │
                    └───────────────────┘
```

> **향후 계획**: EventBus를 활용한 비동기 이벤트 기반 통신 도입 예정 (v1.0 목표).

---

### 6. 데이터 모델

#### 6.1 ERD (구현 기준)

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ organizations │────<│ org_members      │>────│ users            │
│              │     │                  │     │                  │
│ id (PK)      │     │ org_id (FK)      │     │ id (PK)          │
│ name         │     │ user_id (FK)     │     │ email            │
│ slug         │     │ role             │     │ name             │
│ domain       │     │ created_at       │     │ role             │
│ status       │     └──────────────────┘     │ org_id           │
│ created_at   │                               │ created_at       │
│ updated_at   │                               └──────────────────┘
└──────┬───────┘
       │ 1:N
       ▼
┌──────────────────┐     ┌──────────────────────────┐
│ clusters         │     │ cluster_kubeconfigs       │
│                  │     │                          │
│ id (PK)          │     │ cluster_id (FK, PK)      │
│ org_id (FK)      │     │ encrypted_data (BYTEA)   │
│ name             │     │ created_at               │
│ type (pipeline/  │     └──────────────────────────┘
│       target)    │
│ endpoint         │     ┌──────────────────────────┐
│ connection_status│     │ stacks                   │
│ access_scope     │     │                          │
│ created_at       │     │ id (PK)                  │
│ updated_at       │     │ org_id (FK)              │
└──────────────────┘     │ cluster_id (FK)          │
                          │ template_id              │
                          │ name                     │
                          │ namespace                │
                          │ tools (JSONB)            │
                          │ config (JSONB)           │
                          │ state                    │
                          │ created_at               │
                          │ updated_at               │
                          └────────────┬─────────────┘
                                       │ 1:N
                                       ▼
                          ┌──────────────────────────┐
                          │ stack_history             │
                          │                          │
                          │ id (PK)                  │
                          │ stack_id (FK)            │
                          │ version                  │
                          │ config (JSONB)           │
                          │ change_reason            │
                          │ changed_by               │
                          │ created_at               │
                          └──────────────────────────┘

┌──────────────────────────┐     ┌──────────────────────────┐
│ stack_templates          │     │ stack_template_versions   │
│                          │     │                          │
│ id (PK)                  │     │ id (PK)                  │
│ name                     │     │ template_id (FK)         │
│ description              │     │ version                  │
│ tools (JSONB)            │     │ tools (JSONB)            │
│ resource_baseline (JSONB)│     │ created_at               │
│ created_at               │     └──────────────────────────┘
└──────────────────────────┘

┌──────────────────────────┐     ┌──────────────────────────┐
│ ci_cd_pipelines          │     │ deployments              │
│                          │     │                          │
│ id (PK)                  │     │ id (PK)                  │
│ org_id (FK)              │     │ pipeline_id (FK)         │
│ template_id              │     │ status (enum)            │
│ cluster_id (FK)          │     │ manifest (JSONB)         │
│ name                     │     │ started_at               │
│ namespace                │     │ completed_at             │
│ app_type                 │     │ started_by               │
│ git_repo_url             │     │ error_message            │
│ config (JSONB)           │     └──────────────────────────┘
│ status                   │
│ created_at               │     ┌──────────────────────────┐
└──────────────────────────┘     │ cicd_templates           │
                                  │                          │
┌──────────────────────────┐     │ id (PK)                  │
│ alert_rules              │     │ name                     │
│                          │     │ app_type                 │
│ id (PK)                  │     │ description              │
│ org_id (FK)              │     │ stages (JSONB)           │
│ name                     │     │ created_at               │
│ metric_name              │     └──────────────────────────┘
│ condition                │
│ threshold                │     ┌──────────────────────────┐
│ severity                 │     │ compatibility            │
│ channel (slack/email)    │     │                          │
│ webhook_url              │     │ id (PK)                  │
│ enabled                  │     │ tool_name                │
│ created_at               │     │ tool_version             │
│ updated_at               │     │ compatible_with (JSONB)  │
└──────────────────────────┘     │ status                   │
                                  │ verified_at              │
┌──────────────────────────┐     └──────────────────────────┘
│ audit_logs               │
│                          │     ┌──────────────────────────┐
│ id (PK)                  │     │ resource_defaults        │
│ org_id (FK)              │     │                          │
│ user_id                  │     │ id (PK)                  │
│ action                   │     │ tool_key                 │
│ resource_type            │     │ display_name             │
│ resource_id              │     │ cpu_request              │
│ details (JSONB)          │     │ cpu_limit                │
│ created_at               │     │ memory_request_gi        │
└──────────────────────────┘     │ memory_limit_gi          │
                                  │ storage_request_gi       │
┌──────────────────────────┐     │ storage_limit_gi         │
│ notifications            │     │ is_default               │
│                          │     └──────────────────────────┘
│ id (PK)                  │
│ org_id (FK)              │     ┌──────────────────────────┐
│ type                     │     │ known_issues             │
│ config (JSONB)           │     │                          │
│ enabled                  │     │ id (PK)                  │
│ created_at               │     │ title                    │
└──────────────────────────┘     │ description              │
                                  │ severity                 │
                                  │ status                   │
                                  │ created_at               │
                                  └──────────────────────────┘
```

> **v0.1 대비 변경**: `cluster_kubeconfigs` 테이블 분리(kubeconfig 암호화 전용), `resource_defaults` 테이블 신설, `audit_logs` 테이블 신설, `known_issues` 테이블 신설, `notifications` 테이블 신설, `stack_template_versions` 테이블 신설. `users` 테이블에서 `password_hash` 제거(OIDC 기반으로 전환). `deployments` 테이블이 CI/CD 모듈 전용으로 변경.

#### 6.2 핵심 JSONB 구조

**stacks.tools** — 설치 도구 목록:

```json
[
  {
    "category": "source-control",
    "name": "GitLab",
    "tool": "gitlab",
    "helm_version": "9.5.1",
    "app_version": "18.5.1",
    "version": "18.5.1"
  },
  {
    "category": "cicd",
    "name": "ArgoCD",
    "tool": "argocd",
    "helm_version": "6.8.0",
    "app_version": "v2.8.3",
    "version": "v2.8.3"
  }
]
```

> **v0.1 대비 변경**: `config_json` 내 중첩 구조(artifacts/pipeline/monitoring/logging/resources) → `tools` JSONB 배열 + 별도 `config` JSONB로 분리. `helm_version`과 `app_version` 명시적 분리.

**stacks.config** — 스택 설정 (접근 도메인, 스토리지, 리소스 등):

```json
{
  "access_domain": "stack.internal",
  "access_domain_tls": {
    "enabled": true,
    "secret_name": "tls-cert",
    "secret_namespace": "nullus-stack"
  },
  "storage": {
    "plan_mode": "integrated-create",
    "database": { "provider": "cnpg", "size_gi": 10 },
    "object_storage": { "provider": "minio", "size_gi": 50 }
  },
  "resources": {
    "developers": 20,
    "concurrent_runners": 5
  }
}
```

#### 6.3 Enum 정의

| Enum | 값 | 설명 |
|------|----|------|
| `org_status` | active, inactive | 조직 상태 |
| `user_role` | admin, devops, developer | 사용자 역할 |
| `connection_status` | connected, pending, unreachable, auth_failed | 클러스터 연결 상태 |
| `cluster_type` | pipeline, target | 클러스터 용도 |
| `deployment_state` | pending, validating, installing, configuring, health_check, completed, failed, rolling_back, rolled_back | 배포 상태 머신 |
| `alert_severity` | critical, warning, info | 알림 심각도 |
| `app_type` | web, backend, batch | CI/CD 앱 유형 |

---

### 7. 배포 아키텍처

#### 7.1 Nullus 자체의 배포 구조 (구현 기준)

```
배포 옵션 A: Kubernetes (Helm Chart)
┌─ nullus-system NS ─────────────────────────┐
│                                             │
│  ┌─────────────────┐  ┌─────────────────┐  │
│  │ nullus-api      │  │ nullus-web      │  │
│  │ (Go, Deployment)│  │ (Nginx + React) │  │
│  │ Port: 8090      │  │ Port: 80/443    │  │
│  │ (ConfigMap 마운트)│  └────────┬────────┘  │
│  │ (wait-for-db    │           │            │
│  │  initContainer) │           │            │
│  └────────┬────────┘           │            │
│           │                    │            │
│  ┌────────▼────────┐          │            │
│  │ postgresql      │          │            │
│  │ (StatefulSet)   │          │            │
│  │ Port: 5432      │          │            │
│  └─────────────────┘          │            │
│                                │            │
│  ┌─────────────────────────────▼─────────┐ │
│  │ Ingress / Gateway                     │ │
│  │ nullus.example.com                    │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘

배포 옵션 B: Docker Compose (개발/소규모)
┌─────────────────────────────────────────────┐
│  docker-compose.dev.yaml                    │
│                                             │
│  nullus-api:    localhost:8090              │
│  nullus-web:    localhost:5173 (Vite dev)   │
│  postgresql:    localhost:5433              │
│  minio:         localhost:9000/9001         │
│  redis:         localhost:6380              │
│  keycloak:      localhost:8180              │
└─────────────────────────────────────────────┘
```

> **v0.1 대비 변경**: API 포트 8080→8090. Docker Compose에 MinIO, Redis, Keycloak 추가. Helm 차트에 ConfigMap 마운트, wait-for-db initContainer, liveness/readiness probes 추가.

#### 7.2 Nullus가 설치하는 스택의 3-Phase DAG

```
Phase A: Foundation (인프라 기반)
├── cert-manager (TLS 인증서 관리)
├── metrics-server (K8s 메트릭)
├── PostgreSQL / CNPG (데이터베이스)
├── MinIO (오브젝트 스토리지)
└── object-storage-secret (MinIO 인증 시크릿)

Phase B: CI/CD (핵심 서비스) — Phase A 완료 후
├── GitLab CE (소스 코드 관리)
├── ArgoCD (지속적 배포)
└── GitLab Runner (CI 실행)

Phase C: Observability (보조 서비스) — Phase B 완료 후
├── Prometheus (메트릭 수집)
├── Grafana (시각화)
├── Logging (OpenTelemetry)
├── Gateway API (네트워크 라우팅)
└── Integration Check (연동 검증)
```

> **v0.1 대비 변경**: 8단계 순차 DAG → 3-Phase 병렬 DAG로 변경. 각 Phase 내 도구들은 병렬 설치 가능. cert-manager, metrics-server가 Phase A에 추가. OpenSearch/Loki는 미포함 (Phase 2 예정). Integration Step이 Phase C로 이동.

---

### 8. 보안 아키텍처

```
┌─ 데이터 흐름 보안 (구현 기준) ──────────────────────────────────┐
│                                                                  │
│  브라우저 ──HTTP──→ Nullus API (개발 모드)                       │
│  브라우저 ──HTTPS──→ Nullus API (운영 모드, Ingress TLS)         │
│                        │                                        │
│                        ├─ Kubeconfig: AES-256-GCM 암호화         │
│                        │   cluster_kubeconfigs 테이블 저장        │
│                        │   복호화는 메모리에서만 수행              │
│                        │                                        │
│                        ├─ 인증 (Dev): X-User-* 헤더 기반         │
│                        │   (X-User-ID, X-User-Email, X-User-Role)│
│                        │                                        │
│                        ├─ 인증 (Prod): Keycloak JWT 검증         │
│                        │   OIDC Discovery + JWKS 검증            │
│                        │                                        │
│                        └─ RBAC: 미들웨어 체인 적용               │
│                            RequireRole(admin/devops/developer)   │
│                                                                  │
│  Nullus API ──kubeconfig──→ K8s API Server                      │
│                (메모리에서 복호화 후 Helm SDK에 전달)              │
└──────────────────────────────────────────────────────────────────┘
```

#### RBAC 매핑 (구현 기준)

| 기능 영역 | Admin | DevOps | Developer |
|---|---|---|---|
| Organization 관리 (CRUD) | ✅ | ❌ | ❌ |
| 사용자/멤버 관리 | ✅ | ❌ | ❌ |
| 클러스터 등록/삭제 | ✅ | ❌ | ❌ |
| 클러스터 조회 | ✅ | ✅ | ❌ |
| 스택 생성/설정/배포/삭제 | ✅ | ✅ | ❌ |
| 스택 조회/이력 | ✅ | ✅ | ❌ |
| CI/CD 파이프라인 전체 | ✅ | ✅ | ✅ (제한적) |
| CI/CD 배포 실행 | ✅ | ✅ | ✅ |
| 모니터링 조회 | ✅ | ✅ | ✅ |
| 알림 규칙 관리 | ✅ | ✅ | ❌ |
| 감사 로그 조회 | ✅ | ❌ | ❌ |
| Known Issues 조회 | ✅ | ✅ | ✅ |
| OSS 리소스 기본값 관리 | ✅ | ❌ | ❌ |

> **v0.1 대비 변경**: Developer 역할이 CI/CD 파이프라인 배포 실행 가능(셀프 서비스 배포). 감사 로그, Known Issues, 리소스 기본값 관리 추가.

#### 인증 모드 전환

```yaml
# configs/config.yaml
auth:
  mode: session      # session (개발) / oidc (운영)
  oidc:
    provider: keycloak  # keycloak / authentik
    issuer_url: "http://localhost:8180/realms/nullus"
    client_id: "nullus"
    client_secret: "..."
```

DualAuthMiddleware가 `auth.mode` 설정에 따라 세션/JWT 인증을 자동 전환합니다.

---

## Part 2: 상세 기능 명세

---

### 기능 0: Organization 설정 등록

**상태**: ✅ 구현 완료

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| POST | `/api/v1/admin/orgs` | Organization 생성 | 경로 변경 (/orgs → /admin/orgs) |
| GET | `/api/v1/admin/organization` | 현재 Organization 조회 | 신규 (현재 org 전용) |
| PATCH | `/api/v1/admin/organization` | Organization 수정 | PUT → PATCH 변경 |
| GET | `/api/v1/admin/organizations/:orgId/members` | 멤버 목록 조회 | 경로 변경 |
| POST | `/api/v1/admin/organizations/:orgId/members` | 멤버 초대 | 경로 변경 |
| DELETE | `/api/v1/admin/organizations/:orgId/members/:id` | 멤버 제거 | 경로 변경 |
| PATCH | `/api/v1/admin/organizations/:orgId/members/:id` | 멤버 역할 변경 | PUT → PATCH, 경로 변경 |
| GET | `/api/v1/admin/users/search?email=` | 사용자 검색 | 신규 |

> **v0.1 대비 변경**: 모든 Admin API가 `/api/v1/admin/` 접두사로 통합. HTTP 메서드가 부분 업데이트에 PATCH 사용. 활성/비활성 전환 API 미구현.

---

### 기능 1: K8S Cluster Configurations 등록

**상태**: ✅ 구현 완료

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| POST | `/api/v1/admin/clusters` | 클러스터 등록 | 경로 변경 |
| GET | `/api/v1/admin/clusters` | 클러스터 목록 조회 | 경로 변경 |
| DELETE | `/api/v1/admin/clusters/:id` | 클러스터 삭제 | 경로 변경 |
| POST | `/api/v1/admin/clusters/:id/verify` | 연결 검증 | 경로 변경 |
| GET | `/api/v1/admin/clusters/:id/namespaces` | 네임스페이스 목록 | 경로 변경 |

> **v0.1 대비 변경**: GET/PUT 개별 클러스터 조회/수정 API 미구현. kubeconfig 암호화 저장이 별도 `cluster_kubeconfigs` 테이블로 분리. `access_scope` 필드 추가 (클러스터 접근 범위 설정).

#### Kubeconfig 암호화 흐름 (구현 기준)

```
등록 흐름:
1. 사용자가 kubeconfig 파일/텍스트 업로드
2. pkg/crypto.Encrypt(key, plaintext) → AES-256-GCM 암호화
3. cluster_kubeconfigs 테이블에 encrypted_data 저장
4. clusters 테이블에 메타데이터 저장 (name, type, endpoint, status=pending)

검증 흐름:
1. POST /clusters/:id/verify 호출
2. cluster_kubeconfigs에서 encrypted_data 조회
3. pkg/crypto.Decrypt(key, ciphertext) → kubeconfig 복호화
4. client-go로 K8s API Server 연결 테스트
5. 성공: connection_status = connected, 실패: unreachable/auth_failed

Helm 배포 시:
1. KubeconfigProvider.GetKubeconfigForCluster(clusterId)
2. DB에서 암호화된 kubeconfig 조회 + 복호화
3. 메모리 상의 kubeconfig를 Helm SDK에 전달
4. 배포 완료 후 메모리에서 삭제
```

---

### 기능 2: 노코드 기반 DevSecOps Stack 설정 UI

**상태**: ✅ 구현 완료 (5단계 전체)

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| POST | `/api/v1/stacks` | 스택 생성 | 동일 |
| GET | `/api/v1/stacks` | 스택 목록 | 동일 |
| GET | `/api/v1/stacks/:id` | 스택 상세 | 동일 |
| DELETE | `/api/v1/stacks/:id` | 스택 삭제 (Helm uninstall) | 신규 |
| PATCH | `/api/v1/stacks/:id/tools` | 도구 추가 | PUT → PATCH |
| POST | `/api/v1/stacks/:id/config` | 설정 저장 | 신규 |

#### 5단계 설정 워크플로우 (구현 기준)

```
┌──────────────────────────────────────────────────────────────────┐
│  Step 1        Step 2       Step 3        Step 4       Step 5    │
│ [Template] → [Tools] → [Namespace] → [Resources] → [Deploy]    │
│  템플릿선택    도구설정    NS/클러스터   리소스설정    배포확인  │
└──────────────────────────────────────────────────────────────────┘
```

> **v0.1 대비 변경**: v0.1의 5단계(Artifacts→Pipeline→Monitoring→Logging→Resources) → 구현에서는 Template→Tools→Namespace→Resources→Deploy 흐름으로 변경. 도구별 카테고리 선택이 아닌 템플릿 기반 일괄 설정 + 개별 도구 추가 방식.

#### 프론트엔드 상태 관리

Zustand `StackConfigStore`가 5단계 폼 상태를 관리합니다:

```typescript
interface StackConfigState {
  templateId: string;
  stackName: string;
  clusterId: string;
  namespace: string;
  tools: ToolConfig[];
  config: Record<string, any>;
  currentStep: number;
}
```

---

### 기능 3: Golden Path 템플릿

**상태**: ✅ 구현 완료 (3종)

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| GET | `/api/v1/stacks/templates` | 템플릿 목록 | 경로 변경 |
| GET | `/api/v1/stacks/templates/:id` | 템플릿 상세 | 경로 변경 |
| POST | `/api/v1/stacks/templates` | 템플릿 생성 | 신규 (Admin) |
| PUT | `/api/v1/stacks/templates/:id` | 템플릿 수정 | 신규 (Admin) |
| DELETE | `/api/v1/stacks/templates/:id` | 템플릿 삭제 | 신규 (Admin) |

> **v0.1 대비 변경**: 경로가 `/api/v1/templates/golden-paths` → `/api/v1/stacks/templates`로 변경. 템플릿 CRUD API 추가. 템플릿 데이터가 파일 시스템이 아닌 DB(`stack_templates` + `stack_template_versions` 테이블)에 저장.

#### 템플릿 목록 (구현 기준)

DB 시드로 3개의 Golden Path가 등록되어 있으며, Admin이 추가 생성 가능합니다.

---

### 기능 4: DevSecOps Stack 자동 설치/배포/이력 관리

**상태**: ✅ 구현 완료

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| POST | `/api/v1/stacks/:id/deploy` | 배포 시작 (202 Accepted) | 경로 변경 |
| GET | `/api/v1/stacks/:id/status` | 배포 상태 조회 | 경로 변경 |
| WS | `/ws/deployments/:id/logs` | 실시간 로그 WebSocket | 동일 |
| GET | `/api/v1/stacks/:id/history` | 버전 이력 조회 | 경로 변경 |
| POST | `/api/v1/stacks/:id/history` | 버전 저장 | 신규 |
| GET | `/api/v1/stacks/:id/history/diff` | 버전 간 diff | 경로 변경 |
| GET | `/api/v1/stacks/:id/monitoring` | 스택 모니터링 | 신규 |

> **v0.1 대비 변경**: `/api/v1/installations` → `/api/v1/stacks/:id/deploy`로 통합. 설치 취소/재시도 API 미구현. Rollback API 미구현 (DELETE로 대체). 모니터링 API 신규.

#### Install Engine 아키텍처 (구현 기준)

```
┌─────────────────────────────────────────────────────────────┐
│                  Helm Orchestrator                            │
│                  (internal/stack/adapter/helm/)               │
│                                                              │
│  입력: Stack (도구 목록, 클러스터, 네임스페이스)                │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              State Machine (상태 머신)                │    │
│  │                                                      │    │
│  │  PENDING → VALIDATING → INSTALLING → CONFIGURING     │    │
│  │                                          │           │    │
│  │                                    HEALTH_CHECK      │    │
│  │                                          │           │    │
│  │                 FAILED ←───────── COMPLETED          │    │
│  │                   │                                  │    │
│  │             ROLLING_BACK → ROLLED_BACK               │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │        3-Phase DAG Executor (병렬 실행)               │    │
│  │                                                      │    │
│  │  Phase A (Foundation):                               │    │
│  │    cert-manager → metrics-server → postgresql        │    │
│  │    → minio → object-storage-secret                   │    │
│  │                                                      │    │
│  │  Phase B (CI/CD): [Phase A 완료 대기]                 │    │
│  │    gitlab → argocd → runner                          │    │
│  │                                                      │    │
│  │  Phase C (Observability): [Phase B 완료 대기]         │    │
│  │    prometheus → grafana → logging → opentelemetry    │    │
│  │    → gateway-api → integration-check                 │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │        Helm Installer (Helm SDK Wrapper)              │    │
│  │                                                      │    │
│  │  Install() / Upgrade() / Uninstall() / Status()      │    │
│  │  GetValues() — Helm 릴리스 관리                       │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │        Log Streamer (메모리 기반 Pub/Sub)              │    │
│  │                                                      │    │
│  │  Subscribe(deploymentId) → channel                   │    │
│  │  Publish(deploymentId, logEntry)                     │    │
│  │  WebSocket Handler가 Subscribe하여 클라이언트 전송    │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │        Kubeconfig Provider                            │    │
│  │                                                      │    │
│  │  GetKubeconfigForCluster(clusterId)                  │    │
│  │  → DB 조회 → AES-256-GCM 복호화 → REST Config 반환   │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

> **v0.1 대비 변경**:
> - Orchestrator가 `internal/engine/` → `internal/stack/adapter/helm/`로 이동
> - State Machine이 단순화됨 (CANCELLED, RETRYING, TIMEOUT, PARTIAL_SUCCESS 상태 제거)
> - Rollback Manager가 별도 컴포넌트가 아닌 Helm uninstall로 단순화
> - Log Streamer가 DB 영속화 없이 메모리 기반 Pub/Sub으로 구현
> - known-issues.yaml 패턴 엔진이 DB 기반 Known Issues로 대체

---

### 기능 5: CI/CD Pipeline 템플릿

**상태**: ✅ 구현 완료

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| GET | `/api/v1/cicd/templates` | 파이프라인 템플릿 목록 | 경로 변경 |
| GET | `/api/v1/cicd/app-templates` | 앱 템플릿 목록 (Developer 셀프서비스) | 신규 |

> **v0.1 대비 변경**: 경로가 `/api/v1/templates/pipelines` → `/api/v1/cicd/templates`로 변경. Developer 셀프서비스용 `app-templates` API 추가.

#### 구현된 템플릿 유형

| app_type | 이름 | 대상 | 상태 |
|---|---|---|---|
| `web` | Web Application | React, Vue, Next.js 등 | ✅ 구현 |
| `backend` | Backend Service | Spring Boot, Express 등 | ✅ 구현 |
| `batch` | Batch Job | 크론 작업, 데이터 처리 | ✅ 구현 |

---

### 기능 6: CI/CD Pipeline 배포/이력 관리

**상태**: ✅ 구현 완료 (기본 기능)

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| POST | `/api/v1/cicd/pipelines` | 파이프라인 생성 | 경로 변경 |
| GET | `/api/v1/cicd/pipelines` | 파이프라인 목록 | 경로 변경 |
| POST | `/api/v1/cicd/pipelines/:id/deploy` | 배포 실행 (202 Accepted) | 경로 변경 |
| GET | `/api/v1/cicd/deployments` | 배포 이력 목록 | 경로 변경 |
| GET | `/api/v1/cicd/deployments/:id` | 배포 상세 | 경로 변경 |
| POST | `/api/v1/cicd/deploy-app` | 앱 배포 (Developer 셀프서비스) | 신규 |
| WS | `/ws/cicd/deployments/:id/logs` | CI/CD 배포 로그 WebSocket | 신규 |

> **v0.1 대비 변경**: 경로가 `/api/v1/pipelines` → `/api/v1/cicd/pipelines`로 변경. 롤백/diff API 미구현. Developer 셀프서비스 배포 API 추가.

#### CI/CD 배포 흐름 (구현 기준)

```
1. CreatePipeline: 파이프라인 설정 저장 (name, template, cluster, namespace, git_repo_url)
2. DeployPipeline:
   a. 매니페스트 생성 (Deployment + Service + Ingress + Secret YAML)
   b. ManifestApplier가 kubectl apply 실행
   c. StepTracker가 각 리소스 적용 상태를 WebSocket으로 스트리밍
   d. 전체 완료/실패 시 deployments 테이블 업데이트
```

---

### 기능 7: 모니터링/알림 관리

**상태**: ✅ 구현 완료

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| GET | `/api/v1/observability/dashboard` | 대시보드 (Prometheus) | 경로 변경 |
| GET | `/api/v1/observability/alert-rules` | 알림 규칙 목록 | 신규 (CRUD 분리) |
| POST | `/api/v1/observability/alert-rules` | 알림 규칙 생성 | 신규 |
| GET | `/api/v1/observability/alert-rules/:id` | 알림 규칙 상세 | 신규 |
| PATCH | `/api/v1/observability/alert-rules/:id` | 알림 규칙 수정 | 신규 |
| DELETE | `/api/v1/observability/alert-rules/:id` | 알림 규칙 삭제 | 신규 |
| GET | `/api/v1/observability/alert-history` | 알림 발생 이력 | 신규 |

> **v0.1 대비 변경**: 경로가 `/api/v1/monitoring/` → `/api/v1/observability/`로 변경. 단일 alerts/config → CRUD 개별 API로 분리. Prometheus가 선택적 연동(미구성 시 메모리 fallback). 알림 발송(Slack/Email)은 플레이스홀더 상태.

#### Prometheus 통합 (구현 기준)

```
┌─ Observability Module ─────────────────────────────┐
│                                                      │
│  DashboardHandler                                   │
│       │                                             │
│       ▼                                             │
│  GetDashboard UseCase                               │
│       │                                             │
│       ├─ Prometheus 설정 있음 → PrometheusClient    │
│       │   HTTP GET /api/v1/query_range              │
│       │   → 메트릭 데이터 반환                       │
│       │                                             │
│       └─ Prometheus 미설정 → 빈 대시보드 반환        │
│                                                      │
└──────────────────────────────────────────────────────┘
```

---

### 기능 8: DevSecOps Stack OSS 버전 호환성 관리

**상태**: ✅ 구현 완료

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| GET | `/api/v1/stacks/compatibility` | 호환성 매트릭스 조회 | 경로 변경 |

> **v0.1 대비 변경**: 경로가 `/api/v1/compatibility/matrix` → `/api/v1/stacks/compatibility`로 통합. POST `/validate` 엔드포인트는 별도 노출 대신 스택 생성/배포 시 내부 검증으로 통합. DB 기반 호환성 데이터(`compatibility` 테이블) 사용.

---

### 기능 9: UI 권한 체계 (RBAC)

**상태**: ✅ 구현 완료 (v0.1에서 v1 계획이었으나 앞당겨 구현)

#### 구현 방식

```go
// 미들웨어 체인 (main.go)
adminGroup := v1.Group("/admin", authMiddleware, rbacMiddleware.RequireRole("admin"))
stackGroup := v1.Group("/stacks", authMiddleware, rbacMiddleware.RequireRole("admin", "devops"))
cicdGroup  := v1.Group("/cicd", authMiddleware, rbacMiddleware.RequireRole("admin", "devops", "developer"))
obsGroup   := v1.Group("/observability", authMiddleware)
```

#### 프론트엔드 역할 기반 라우팅

```typescript
// ProtectedRoute 컴포넌트로 역할 기반 접근 제어
<Route path="/admin/*" element={<ProtectedRoute roles={['admin']} />} />
<Route path="/stack/*" element={<ProtectedRoute roles={['admin', 'devops']} />} />
<Route path="/cicd/*" element={<ProtectedRoute roles={['admin', 'devops', 'developer']} />} />
```

#### Keycloak OIDC 연동 (구현 기준)

```
로그인 흐름:
Browser → Nullus Web → /api/v1/auth/login → Keycloak OIDC Authorization Endpoint
  → 사용자 인증 → Authorization Code → Token Exchange
  → ID Token + Access Token 발급
  → Nullus API: JWT 검증 (JWKS) → 역할 추출 → 세션 생성
```

> **v0.1 대비 변경**: Authentik OIDC Provider 추가. OSS별 권한 매핑(GitLab/ArgoCD/Grafana)은 미구현.

---

### 기능 10: 리소스 예상량 관리

**상태**: ✅ 구현 완료 (기본값 관리)

#### API 엔드포인트 (구현 기준)

| Method | Path | 설명 | v0.1 대비 |
|---|---|---|---|
| GET | `/api/v1/stacks/resource-defaults` | OSS 리소스 기본값 조회 | 신규 |
| POST | `/api/v1/stacks/resource-defaults` | OSS 리소스 기본값 저장 | 신규 |

> **v0.1 대비 변경**: `POST /resources/estimate` 동적 계산 API → `resource_defaults` 테이블 기반 정적 기본값 관리로 변경. 비용 추정 기능 미구현. 스케일링 팩터 계산 미구현.

#### resource_defaults 테이블 구조

```
resource_defaults
├── tool_key          VARCHAR  # e.g., "gitlab", "argocd", "prometheus"
├── display_name      VARCHAR  # e.g., "GitLab CE", "Argo CD"
├── cpu_request       DECIMAL  # e.g., 2.0
├── cpu_limit         DECIMAL  # e.g., 4.0
├── memory_request_gi DECIMAL  # e.g., 4.0
├── memory_limit_gi   DECIMAL  # e.g., 8.0
├── storage_request_gi DECIMAL # e.g., 30.0
├── storage_limit_gi  DECIMAL  # e.g., 50.0
├── is_default        BOOLEAN  # 기본값 여부
```

---

### 기능 11: 감사 로그 (신규)

**상태**: ✅ 구현 완료

v0.1에 없던 신규 기능입니다.

#### API 엔드포인트

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/admin/audit-logs` | 감사 로그 목록 조회 (Admin 전용) |

#### 구현 방식

```go
// AuditLogger (internal/shared/audit/)
type AuditLogger struct {
    db *pgxpool.Pool
}

func (l *AuditLogger) Log(ctx context.Context, entry AuditEntry) error
// AuditEntry: org_id, user_id, action, resource_type, resource_id, details(JSONB)
```

주요 감사 대상 액션: 클러스터 등록/삭제, 스택 생성/배포/삭제, 멤버 초대/제거/역할변경, 알림 규칙 변경.

---

### 기능 12: Known Issues 레지스트리 (신규)

**상태**: ✅ 구현 완료

v0.1에서 `known-issues.yaml` 파일 기반으로 설계되었던 Helm edge case 패턴을 DB 기반 레지스트리로 구현했습니다.

#### API 엔드포인트

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/admin/known-issues` | Known Issues 목록 조회 |

---

## Part 3: 프론트엔드 아키텍처

---

### 1. 페이지 구조 (구현 기준)

```
Web Application (React 19 + TypeScript 5.9)
│
├── /login                          # 로그인 (역할 선택 mock / OIDC)
│
├── /                               # 홈 대시보드
│
├── /stack/
│   ├── /templates                  # Golden Path 템플릿 브라우저
│   ├── /install                    # 5단계 설치 위자드
│   ├── /list                       # 설치된 스택 목록 (상태 표시)
│   ├── /deploy/:id                 # 배포 진행 (WebSocket 로그)
│   ├── /:id/add-tools              # 도구 추가
│   ├── /history/:stackId           # 버전 이력 + diff 뷰어
│   ├── /versions                   # 전체 버전 이력
│   └── /oss-resource-default       # OSS 리소스 기본값 관리 (Admin)
│
├── /cicd/
│   ├── /templates                  # 파이프라인 템플릿
│   ├── /create                     # 파이프라인 생성 위자드
│   ├── /developer-deploy           # Developer 셀프서비스 배포
│   ├── /list                       # 파이프라인 목록 (인라인 상세)
│   ├── /pipelines/:id/logs         # 배포 로그 뷰어
│   └── /history                    # 배포 이력
│
├── /observability/
│   ├── /monitoring                 # Prometheus 대시보드
│   ├── /alerts                     # 알림 규칙 관리
│   └── /alert-history              # 알림 발생 이력
│
└── /admin/                         # Admin 전용
    ├── /organization               # 조직 설정
    ├── /users                      # 멤버 관리
    ├── /clusters                   # 클러스터 관리
    └── /known-issues               # Known Issues
```

### 2. API 서비스 레이어

TanStack Query (React Query)를 사용한 서버 상태 관리:

```typescript
// 각 feature 모듈에 api/ 디렉토리
features/
  stack/api/stack-api.ts     // useTemplates(), useCreateStack(), useDeployLog(), ...
  admin/api/admin-api.ts     // useOrganization(), useClusters(), useMembers(), ...
  cicd/api/cicd-api.ts       // usePipelines(), useDeployPipeline(), ...
  observability/api/          // useDashboard(), useAlertRules(), ...
```

### 3. 실시간 통신

WebSocket 커스텀 훅으로 배포 로그 스트리밍:

```typescript
// useDeployLog(deploymentId) → WebSocket 연결
// useCicdDeployLog(deploymentId) → CI/CD 배포 WebSocket
//
// 연결: ws://localhost:8090/ws/deployments/:id/logs
// 메시지 형식: { timestamp, level, message, step }
```

---

## Part 4: 테스트 전략

---

### Go 테스트

| 유형 | 도구 | 대상 | 현재 상태 |
|------|------|------|-----------|
| 단위 테스트 | testify/assert | domain, usecase | 구현됨 |
| 통합 테스트 | testcontainers-go | repository (real DB) | 구현됨 |
| HTTP 테스트 | httptest | handler | 구현됨 |
| Mock | testify/mock, memory 구현체 | port 인터페이스 | 구현됨 |

테스트 명명 규칙: `TestService_Operation_Scenario`

### React 테스트

| 유형 | 도구 | 대상 | 현재 상태 |
|------|------|------|-----------|
| 단위 테스트 | Vitest + RTL | 컴포넌트, 훅 | 구현됨 |
| E2E 테스트 | Playwright | 전체 워크플로우 | 18+ 시나리오 |

### E2E 테스트 시나리오 (주요)

- 로그인 → 클러스터 등록 → 연결 검증
- 템플릿 선택 → 스택 생성 → 배포 → 로그 확인
- CI/CD 파이프라인 생성 → 배포
- 알림 규칙 CRUD
- Admin 멤버 관리

---

## Part 5: 운영 및 마이그레이션 전략

---

### DB 마이그레이션 (구현 기준)

- **도구**: golang-migrate
- **마이그레이션 파일**: 30+ (Up/Down 쌍)
- **실행**: `make migrate-up` / `make migrate-down`
- **시드 데이터**: 마이그레이션 내 INSERT (007-009번: 템플릿, 호환성, CI/CD 템플릿)

### 설정 관리

```yaml
# configs/config.yaml (구현 기준)
server:
  port: 8090
  mode: development  # development | production
database:
  host: localhost
  port: 5433
  name: nullus
  user: nullus
  password: nullus
  sslmode: disable
auth:
  mode: session      # session | oidc
  session:
    secret: "..."
    max_age: 86400
  oidc:
    provider: keycloak
    issuer_url: "http://localhost:8180/realms/nullus"
    client_id: "nullus"
    client_secret: "..."
keycloak:
  admin_url: "http://localhost:8180"
  realm: "nullus"
  admin_user: "admin"
  admin_password: "admin"
helm:
  timeout: 600s
  namespace_prefix: "nullus-"
prometheus:
  url: ""  # 비어있으면 Prometheus 비활성
log:
  level: debug
  format: text
```

환경 변수로 오버라이드 가능: `NULLUS_DB_HOST`, `NULLUS_DB_PASSWORD`, `ENCRYPTION_KEY` 등.

---

## Part 6: 기술 의사결정 기록 (ADR)

| ID | 결정 | v0.1 선택 | v0.2 실제 | 변경 사유 |
|---|---|---|---|---|
| ADR-001 | 웹 프레임워크 | React + TS | React 19 + TS 5.9 + Vite 8 | 동일 (도구 업그레이드) |
| ADR-002 | 백엔드 언어 | Go 1.24+ | Go 1.26.1 | 버전 업그레이드 |
| ADR-003 | DB | PostgreSQL 18+ | PostgreSQL 17 (Docker) | 안정 버전 선택 |
| ADR-004 | 실시간 통신 | WebSocket | WebSocket (동일) | 동일 |
| ADR-005 | 설치 엔진 | Helm Go SDK | Helm SDK v3.20.1 | 동일 |
| ADR-006 | 인증 (Dev) | gorilla/sessions | 헤더 기반 세션 | 단순화 |
| ADR-007 | 인증 (Prod) | Keycloak OIDC | Keycloak + Authentik | 다중 IdP |
| ADR-008 | 모노레포 | 단일 리포지토리 | 동일 | 동일 |
| ADR-009 | 상태 관리 | Zustand | Zustand + TanStack Query | 서버 상태 캐싱 |
| ADR-010 | 스택 설정 저장 | JSONB | JSONB (동일) | 동일 |
| ADR-011 | 코드 아키텍처 | 레이어드 | Clean Architecture + DDD | **신규**: 테스트 용이성, 모듈 독립성 |
| ADR-012 | 서버 상태 관리 | (미명시) | TanStack Query | **신규**: 캐싱, 자동 갱신 |
| ADR-013 | 폼 처리 | (미명시) | React Hook Form + Zod | **신규**: 타입 안전 검증 |
| ADR-014 | DB 드라이버 | (미명시) | pgx v5 | **신규**: 고성능 PostgreSQL |
| ADR-015 | E2E 테스트 | (미명시) | Playwright | **신규**: 크로스 브라우저 |
| ADR-016 | 암호화 | AES-256 | AES-256-GCM | **신규**: 인증 암호화 |

---

## Part 7: 로드맵

### 현재: v0.2.0-alpha

- ✅ 5개 Bounded Context 구현 (Admin, Stack, CI/CD, Observability, Auth)
- ✅ 3-Phase Helm DAG 오케스트레이터
- ✅ WebSocket 실시간 로그 스트리밍
- ✅ Keycloak OIDC + RBAC
- ✅ Playwright E2E 테스트
- ✅ Helm 차트 배포

### 다음: v0.2.0-beta

- 🎯 테스트 커버리지 70% 이상
- 🎯 E2E CI 자동화
- 🎯 프로덕션 배포 가이드
- 🎯 프론트엔드 접근성(a11y) 개선

### v1.0 GA

- 🔮 EventBus 기반 모듈 간 비동기 통신
- 🔮 멀티 클러스터 동시 배포
- 🔮 스택 업그레이드 (개별 도구 버전 변경)
- 🔮 OpenSearch/Loki 로깅 스택 설치
- 🔮 알림 발송 (Slack/Email) 실제 구현
- 🔮 감사 로그 검색/필터
- 🔮 OSS별 OIDC 권한 매핑 (GitLab/ArgoCD/Grafana)
- 🔮 비용 추정 엔진
- 🔮 YAML ↔ 노코드 양방향 동기화

### 장기 비전

- 🌟 플러그인 아키텍처
- 🌟 GitOps 기반 스택 관리
- 🌟 멀티 테넌시 강화
- 🌟 SaaS 모드
- 🌟 마켓플레이스

---

## 부록: 전체 API 엔드포인트 목록 (구현 기준)

### Admin (`/api/v1/admin`) — Admin 역할 필요

| Method | Path | 설명 |
|---|---|---|
| POST | `/admin/orgs` | Organization 생성 |
| GET | `/admin/organization` | 현재 Organization 조회 |
| PATCH | `/admin/organization` | Organization 수정 |
| GET | `/admin/clusters` | 클러스터 목록 |
| POST | `/admin/clusters` | 클러스터 등록 |
| DELETE | `/admin/clusters/:id` | 클러스터 삭제 |
| POST | `/admin/clusters/:id/verify` | 연결 검증 |
| GET | `/admin/clusters/:id/namespaces` | 네임스페이스 목록 |
| GET | `/admin/organizations/:orgId/members` | 멤버 목록 |
| POST | `/admin/organizations/:orgId/members` | 멤버 초대 |
| DELETE | `/admin/organizations/:orgId/members/:id` | 멤버 제거 |
| PATCH | `/admin/organizations/:orgId/members/:id` | 역할 변경 |
| GET | `/admin/users/search` | 사용자 검색 |
| GET | `/admin/known-issues` | Known Issues 목록 |
| GET | `/admin/audit-logs` | 감사 로그 |
| GET | `/admin/notifications/configs` | 알림 설정 조회 |
| POST | `/admin/notifications/configs` | 알림 설정 저장 |

### Stack (`/api/v1/stacks`) — Admin/DevOps 역할 필요

| Method | Path | 설명 |
|---|---|---|
| GET | `/stacks` | 스택 목록 |
| POST | `/stacks` | 스택 생성 |
| GET | `/stacks/:id` | 스택 상세 |
| DELETE | `/stacks/:id` | 스택 삭제 (Helm uninstall) |
| PATCH | `/stacks/:id/tools` | 도구 추가 |
| POST | `/stacks/:id/config` | 설정 저장 |
| POST | `/stacks/:id/deploy` | 배포 시작 (202) |
| GET | `/stacks/:id/status` | 배포 상태 |
| GET | `/stacks/:id/monitoring` | 스택 모니터링 |
| GET | `/stacks/templates` | 템플릿 목록 |
| GET | `/stacks/templates/:id` | 템플릿 상세 |
| POST | `/stacks/templates` | 템플릿 생성 |
| PUT | `/stacks/templates/:id` | 템플릿 수정 |
| DELETE | `/stacks/templates/:id` | 템플릿 삭제 |
| GET | `/stacks/compatibility` | 호환성 매트릭스 |
| GET | `/stacks/resource-defaults` | 리소스 기본값 |
| POST | `/stacks/resource-defaults` | 리소스 기본값 저장 |
| GET | `/stacks/:id/history` | 버전 이력 |
| POST | `/stacks/:id/history` | 버전 저장 |
| GET | `/stacks/:id/history/diff` | 버전 diff |

### CI/CD (`/api/v1/cicd`) — 전체 역할 접근 가능

| Method | Path | 설명 |
|---|---|---|
| GET | `/cicd/templates` | 파이프라인 템플릿 |
| GET | `/cicd/pipelines` | 파이프라인 목록 |
| POST | `/cicd/pipelines` | 파이프라인 생성 |
| POST | `/cicd/pipelines/:id/deploy` | 배포 실행 (202) |
| GET | `/cicd/deployments` | 배포 이력 |
| GET | `/cicd/deployments/:id` | 배포 상세 |
| GET | `/cicd/app-templates` | 앱 템플릿 (Developer) |
| POST | `/cicd/deploy-app` | 앱 배포 (Developer) |

### Observability (`/api/v1/observability`) — 전체 역할 접근 가능

| Method | Path | 설명 |
|---|---|---|
| GET | `/observability/dashboard` | 모니터링 대시보드 |
| GET | `/observability/alert-rules` | 알림 규칙 목록 |
| POST | `/observability/alert-rules` | 알림 규칙 생성 |
| GET | `/observability/alert-rules/:id` | 알림 규칙 상세 |
| PATCH | `/observability/alert-rules/:id` | 알림 규칙 수정 |
| DELETE | `/observability/alert-rules/:id` | 알림 규칙 삭제 |
| GET | `/observability/alert-history` | 알림 이력 |

### 기타

| Method | Path | 설명 |
|---|---|---|
| GET | `/health` | 헬스 체크 (서버 + DB) |
| WS | `/ws/deployments/:id/logs` | Stack 배포 로그 WebSocket |
| WS | `/ws/cicd/deployments/:id/logs` | CI/CD 배포 로그 WebSocket |

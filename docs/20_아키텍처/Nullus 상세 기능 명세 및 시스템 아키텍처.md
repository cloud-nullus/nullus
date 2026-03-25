# Nullus v1.0 상세 기능 명세 및 시스템 아키텍처

**작성일**: 2026-03-03  
**버전**: 1.0  
**기반 문서**: Nullus PRD v1.2, 12주 마스터 플랜, Narwhal 분석 기반 Nullus 적용 항목  
**대상 독자**: 엔지니어, 아키텍트, DevOps Engineer

---

## Part 1: 시스템 아키텍처

---

### 1. 아키텍처 개요

Nullus는 크게 **3개의 런타임 환경**에 걸쳐 동작합니다.

```
┌─────────────────────────────────────────────────────────────────┐
│                        사용자 브라우저                            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                   Nullus Web UI (React)                   │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────┐  │  │
│  │  │ 설정     │ │ 템플릿   │ │ 배포     │ │ 모니터링    │  │  │
│  │  │ 워크플로우│ │ 선택     │ │ 대시보드 │ │ 뷰         │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────────┘  │  │
│  └─────────────────────────┬─────────────────────────────────┘  │
└────────────────────────────┼────────────────────────────────────┘
                             │ REST API + WebSocket
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Nullus 컨트롤 플레인                          │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Nullus API Server (Go)                   │   │
│  │  ┌──────────┐ ┌───────────┐ ┌───────────┐ ┌──────────┐  │   │
│  │  │ Auth     │ │ Config    │ │ Installer │ │ Monitor  │  │   │
│  │  │ Handler  │ │ Handler   │ │ Handler   │ │ Handler  │  │   │
│  │  └──────────┘ └───────────┘ └─────┬─────┘ └──────────┘  │   │
│  └───────────────────────────────────┼──────────────────────┘   │
│                                      │                          │
│  ┌──────────┐  ┌───────────────────┐ │ ┌────────────────────┐   │
│  │PostgreSQL│  │ Install Engine    │◄┘ │ Compatibility      │   │
│  │          │  │ (Orchestrator)    │   │ Matrix Engine      │   │
│  │ - Orgs   │  │  ┌─────────────┐ │   │                    │   │
│  │ - Users  │  │  │ Step Runner │ │   │ matrix.yaml 기반   │   │
│  │ - Config │  │  │ State Mgr   │ │   │ 버전 조합 검증     │   │
│  │ - History│  │  │ Rollback    │ │   └────────────────────┘   │
│  └──────────┘  │  └─────────────┘ │                            │
│                └────────┬──────────┘                            │
└─────────────────────────┼──────────────────────────────────────┘
                          │ Helm / kubectl
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                 대상 Kubernetes 클러스터                          │
│                                                                  │
│  ┌─ nullus-system NS ────────────────────────────────────────┐  │
│  │  Nullus Operator (설치 상태 감시, 헬스체크)                │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌─ nullus-artifacts NS ──────┐  ┌─ nullus-pipeline NS ─────────────────┐    │
│  │  MinIO                      │  │  GitLab CE / Gitea                  │    │
│  │  Harbor (선택)              │  │  GitLab CI Runner                   │    │
│  └─────────────────────────────┘  │  Argo CD                            │    │
│                                    └─────────────────────────────────────┘    │
│  ┌─ nullus-monitoring NS ─────┐  ┌─ nullus-logging NS ──────────────────┐    │
│  │  Prometheus                 │  │  OpenTelemetry Collector             │    │
│  │  Grafana                    │  │  OpenSearch / Loki                   │    │
│  └─────────────────────────────┘  └──────────────────────────────────────┘    │
│                                                                  │
│  ┌─ app-{name} NS ─────────────────────────────────────────┐   │
│  │  사용자 애플리케이션 (Nullus CI/CD 파이프라인으로 배포)   │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

### 2. 비기능 요구사항 (NFR)

| 지표 | P95 목표 | P99 목표 | 측정 방법 |
|------|---------|---------|-----------|
| REST API 응답 | < 200ms | < 500ms | Prometheus histogram |
| WebSocket 연결 지연 | < 100ms | < 300ms | 클라이언트 측정 |
| 설치 로그 유실률 | < 0.1% | < 1% | 송신/수신 카운트 비교 |
| 대시보드 초기 로딩 | < 2s | < 5s | Lighthouse |

### 3. 공통 API 규격

#### 3.1 표준 에러 응답 형식

```json
{
  "error": {
    "code": "INSTALL_HELM_TIMEOUT",
    "http_status": 504,
    "message": "Helm 차트 설치 시간 초과",
    "detail": "cert-manager 설치 60초 초과",
    "retryable": true,
    "trace_id": "abc-123"
  }
}
```
- **에러 코드 네이밍**: `{DOMAIN}_{ACTION}_{REASON}`
- **retryable**: 클라이언트 자동 재시도 여부 결정
- **trace_id**: 서버 로그 추적용 고유 ID

### 4. 기술 스택

| 계층 | 기술 | 선택 이유 |
|---|---|---|
| **Frontend** | React 19 + TypeScript | 생태계 최대, 채용 용이, Backstage 플러그인 전환 가능 |
| **상태 관리** | Zustand | 경량, 보일러플레이트 최소 |
| **스타일링** | Tailwind CSS + shadcn/ui | 다크 테마 기본, 빠른 UI 개발 |
| **YAML 에디터** | Monaco Editor (v1) | VS Code와 동일 엔진, YAML 스키마 검증 |
| **Backend** | Go 1.24+ | K8s 클라이언트 라이브러리 네이티브, 단일 바이너리 배포 |
| **웹 프레임워크** | Echo v4 | 경량, 고성능 |
| **실시간 통신** | WebSocket (gorilla/websocket) | 설치 로그 스트리밍, 양방향 |
| **Database** | PostgreSQL 18+ | 확장성, JSON 지원, 향후 pgvector 활용 가능 |
| **마이그레이션** | golang-migrate | Go 표준, SQL 기반 마이그레이션 |
| **인증 (Alpha/Beta)** | 세션 기반 (gorilla/sessions) | 빠른 구현, 단순 |
| **인증 (v1)** | Keycloak OIDC | SSO, RBAC, OSS별 권한 매핑 |
| **설치 엔진** | Helm Go SDK + client-go | K8s 네이티브, Helm 차트 프로그래밍 제어 |
| **컨테이너** | Docker multi-stage build | 경량 이미지, 단일 빌드 파이프라인 |
| **CI/CD** | GitHub Actions | Nullus 자체의 빌드/테스트/릴리스 |
| **API 문서** | OpenAPI 3.0 (swaggo/swag) | Go 구조체에서 자동 생성 |

---

### 5. 컴포넌트 상세

#### 5.1 Nullus API Server

API Server는 모든 클라이언트 요청의 진입점이며, 내부 엔진들을 조율합니다.

```
Nullus API Server
├── /api/v1
│   ├── /auth             ← Auth Handler
│   │   ├── POST /login
│   │   ├── POST /logout
│   │   └── GET  /me
│   │
│   ├── /orgs             ← Org Handler
│   │   ├── POST /                    (Organization 생성)
│   │   ├── GET  /:orgId
│   │   ├── PUT  /:orgId
│   │   ├── POST /:orgId/members      (멤버 초대)
│   │   └── GET  /:orgId/members
│   │
│   ├── /clusters         ← Cluster Handler
│   │   ├── POST /                    (클러스터 등록)
│   │   ├── GET  /:clusterId
│   │   ├── PUT  /:clusterId
│   │   ├── DELETE /:clusterId
│   │   ├── POST /:clusterId/verify   (연결 검증)
│   │   └── GET  /:clusterId/namespaces
│   │
│   ├── /stacks           ← Stack Config Handler
│   │   ├── POST /                    (스택 설정 생성)
│   │   ├── GET  /:stackId
│   │   ├── PUT  /:stackId
│   │   ├── GET  /:stackId/history    (이력 조회)
│   │   ├── GET  /:stackId/history/:versionId/diff
│   │   └── POST /:stackId/rollback/:versionId
│   │
│   ├── /templates        ← Template Handler
│   │   ├── GET  /golden-paths        (Golden Path 목록)
│   │   ├── GET  /golden-paths/:id
│   │   ├── GET  /pipelines           (CI/CD 템플릿 목록)
│   │   └── GET  /pipelines/:id
│   │
│   ├── /deployments      ← Deployment Handler
│   │   ├── POST /stacks/:stackId/deploy   (스택 배포 시작)
│   │   ├── GET  /stacks/:stackId/status   (배포 상태)
│   │   ├── POST /stacks/:stackId/rollback (배포 롤백)
│   │   ├── POST /pipelines               (파이프라인 배포)
│   │   ├── GET  /pipelines/:pipelineId/status
│   │   └── GET  /pipelines/:pipelineId/history
│   │
│   ├── /monitoring       ← Monitoring Handler
│   │   ├── GET  /dashboards
│   │   ├── GET  /metrics/summary
│   │   └── POST /alerts/config
│   │
│   ├── /compatibility    ← Compatibility Handler
│   │   ├── GET  /matrix
│   │   └── POST /validate            (조합 검증)
│   │
│   ├── /resources        ← Resource Calculator Handler
│   │   └── POST /estimate            (리소스 예상량 계산)
│   │
│   └── /users            ← User/RBAC Handler (v1)
│       ├── GET  /
│       ├── PUT  /:userId/role
│       └── DELETE /:userId
│
└── /ws
    └── /deployments/:deploymentId/logs  ← 실시간 로그 WebSocket
```

#### 5.2 Install Engine (설치 엔진)

설치 엔진은 Nullus의 핵심 컴포넌트입니다. 스택 설정을 받아 Kubernetes 클러스터에 순서대로 도구들을 설치합니다.

```
Install Engine 내부 구조
┌─────────────────────────────────────────────────────────────┐
│                      Orchestrator                            │
│                                                              │
│  입력: StackConfig (도구 목록, 버전, 클러스터 정보)            │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              State Machine (상태 머신)                │    │
│  │                                                      │    │
│  │  PENDING → VALIDATING → INSTALLING → CONFIGURING     │    │
│  │      │          │             │             │          │    │
│  │      ▼          ▼             ▼             ▼          │    │
│  │  CANCELLED   FAILED ←──── HEALTHCHECK    COMPLETED     │    │
│  │               │  │            │                        │    │
│  │               │  └────────────┼──────────┐             │    │
│  │               ▼               ▼          ▼             │    │
│  │           RETRYING      ROLLING_BACK  TIMEOUT          │    │
│  │                               │                        │    │
│  │                               ▼                        │    │
│  │               PARTIAL_SUCCESS ← ROLLED_BACK            │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  **상태 전이 정의**                                           │
│  | 이벤트 | 소스 상태 | 타겟 상태 | 액션/가드 |               │
│  |---|---|---|---|                                          │
│  | START | PENDING | VALIDATING | 설정 유효성 검사 |          │
│  | FAIL | VALIDATING | FAILED | 에러 로그 기록 |              │
│  | CANCEL | ANY (active) | CANCELLED | 실행 중인 작업 중단 |   │
│  | TIMEOUT | ANY (active) | TIMEOUT | 타임아웃 처리 |         │
│  | RETRY | FAILED | RETRYING | 실패 단계부터 재시작 |         │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Step Runner (단계별 실행기)               │    │
│  │                                                      │    │
│  │  ** 3-Phase 프로비저닝 (PRD 기준) **                   │    │
│  │                                                      │    │
│  │  Phase A (인프라 기반):                                │    │
│  │     - cert-manager (TLS 인증서 관리)                   │    │
│  │     - CNPG/PostgreSQL (데이터베이스 엔진)              │    │
│  │     - Storage/MinIO (오브젝트 스토리지)                │    │
│  │     * 선행조건: K8s 클러스터 연결 및 권한 확인           │    │
│  │                                                      │    │
│  │  Phase B (핵심 서비스):                                │    │
│  │     - GitLab/Gitea (소스 코드 관리)                    │    │
│  │     - Harbor (컨테이너 레지스트리)                     │    │
│  │     - ArgoCD (지속적 배포)                             │    │
│  │     * 선행조건: Phase A 완료 및 Storage 가용성 확인      │    │
│  │                                                      │    │
│  │  Phase C (보조 서비스):                                │    │
│  │     - Prometheus + Grafana (모니터링)                  │    │
│  │     - Loki (로깅)                                      │    │
│  │     * 선행조건: Phase B 완료 및 서비스 엔드포인트 확보    │    │
│  │                                                      │    │
│  │  known-issues.yaml:                                   │    │
│  │  Narwhal 70+ Helm edge case 패턴 코드화               │    │
│  │  - CRD 262KB 초과 시 --server-side --force-conflicts  │    │
│  │  - 비핵심 앱 --wait 제거, --timeout만 사용             │    │
│  │  - ARM64 노드 감지 → 대체 이미지 자동 선택             │    │
│  │  - 레지스트리 우선순위: ghcr.io > registry.k8s.io      │    │
│  │    > quay.io > docker.io                              │    │
│  │                                                      │    │
│  │  각 Step은 독립 goroutine으로 실행,                    │    │
│  │  이전 Step 완료 대기 후 실행 (DAG 기반)                │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Rollback Manager (롤백 관리자)            │    │
│  │                                                      │    │
│  │  **롤백 모드**                                         │    │
│  │  1. safe (기본): Helm uninstall, PVC 보존 (데이터 보호) │    │
│  │  2. destructive: PVC 포함 모든 리소스 삭제 (명시적 확인) │    │
│  │                                                      │    │
│  │  각 Step 완료 시 롤백 함수를 스택에 push               │    │
│  │  실패 시 스택을 역순으로 pop하며 롤백 실행              │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Log Streamer (로그 스트리머)               │    │
│  │                                                      │    │
│  │  각 Step의 stdout/stderr를 캡처                       │    │
│  │  WebSocket을 통해 클라이언트에 실시간 전송              │    │
│  │  DB에 로그 영속화 (디버깅용)                           │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### 5.3 Compatibility Matrix Engine

호환성 매트릭스 엔진은 도구 간 버전 호환성을 검증합니다.

```yaml
# templates/compatibility/compatibility-matrix.yaml 구조 예시 (app_version과 helm_version 분리)
matrices:
    # Note: app_version과 helm_version을 분리하여 관리 (Narwhal VERSIONS.md 패턴 적용)
  - id: "gitlab-allinone-v1"
    name: "GitLab All-in-One"
    status: "verified"           # verified | experimental | deprecated
    tested_at: "2026-03-15"
    kubernetes:
      min: "1.26"
      max: "1.30"
      recommended: "1.28"
    tools:
      source_repository:
        name: "gitlab-ce"
        app_version: "17.7.x"
        helm_chart: "gitlab/gitlab"
        helm_version: "8.7.x"
      ci_platform:
        name: "gitlab-ci"
        version: "17.7.x"        # GitLab 내장
      cd_tool:
        name: "argocd"
        app_version: "2.13.x"
        helm_chart: "argo/argo-cd"
        helm_version: "7.7.x"
      monitoring_collection:
        name: "prometheus"
        app_version: "3.1.x"
        helm_chart: "prometheus-community/kube-prometheus-stack"
        helm_version: "67.x"
      monitoring_visualization:
        name: "grafana"
        version: "11.4.x"        # kube-prometheus-stack 내장
      storage_backend:
        name: "minio"
        app_version: "2024.x"
        helm_chart: "minio/minio"
        helm_version: "5.3.x"
    integration_tests:
      - "gitlab-argocd-webhook"
      - "prometheus-service-discovery"
      - "grafana-dashboard-provisioning"
    resource_baseline:
      cpu_cores: 8
      memory_gi: 16
      storage_gi: 100
```

---

### 6. 데이터 모델

#### 6.1 ERD 개요

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ organizations │────<│ org_members      │>────│ users            │
│              │     │                  │     │                  │
│ id (PK)      │     │ org_id (FK)      │     │ id (PK)          │
│ name         │     │ user_id (FK)     │     │ email            │
│ slug         │     │ role             │     │ password_hash    │
│ status       │     └──────────────────┘     │ display_name     │
│ created_at   │                               │ created_at       │
└──────┬───────┘                               └──────────────────┘
       │
       │ 1:N
       ▼
┌──────────────────┐     ┌──────────────────────────┐
│ clusters         │     │ stack_configs             │
│                  │     │                          │
│ id (PK)          │     │ id (PK)                  │
│ org_id (FK)      │     │ cluster_id (FK)          │
│ name             │     │ org_id (FK)              │
│ type             │     │ name                     │
│   (pipeline/     │     │ golden_path_id           │
│    target)       │     │ config_json (JSONB)      │
│ kubeconfig_enc   │     │ status                   │
│ endpoint         │     │ current_version          │
│ namespace        │     │ created_at               │
│ auth_method      │     │ updated_at               │
│ status           │     └────────────┬─────────────┘
│ last_verified_at │                  │
│ created_at       │                  │ 1:N
└──────────────────┘                  ▼
                           ┌──────────────────────────┐
                           │ stack_config_versions     │
                           │                          │
                           │ id (PK)                  │
                           │ stack_config_id (FK)     │
                           │ version_number           │
                           │ config_snapshot (JSONB)  │
                           │ change_reason            │
                           │ changed_by (FK → users)  │
                           │ created_at               │
                           └──────────────────────────┘

┌──────────────────────────┐     ┌──────────────────────────┐
│ deployments              │     │ deployment_logs          │
│                          │     │                          │
│ id (PK)                  │     │ id (PK)                  │
│ stack_config_id (FK)     │     │ deployment_id (FK)       │
│ type (stack/pipeline)    │     │ step_name                │
│ status                   │     │ level (info/warn/error)  │
│ started_at               │     │ message                  │
│ completed_at             │     │ timestamp                │
│ started_by (FK → users)  │     └──────────────────────────┘
│ error_message            │
│ helm_releases (JSONB)    │
│ rollback_stack (JSONB)   │
└──────────────────────────┘

┌──────────────────────────┐     ┌──────────────────────────┐
│ pipeline_configs         │     │ pipeline_deployments     │
│                          │     │                          │
│ id (PK)                  │     │ id (PK)                  │
│ stack_config_id (FK)     │     │ pipeline_config_id (FK)  │
│ template_id              │     │ version                  │
│ name                     │     │ status                   │
│ params_json (JSONB)      │     │ k8s_objects (JSONB)      │
│ created_at               │     │ deployed_at              │
│ updated_at               │     │ deployed_by (FK → users) │
└──────────────────────────┘     │ error_message            │
                                 └──────────────────────────┘

┌──────────────────────────┐     ┌──────────────────────────┐
│ alert_configs            │     │ compatibility_matrices   │
│                          │     │                          │
│ id (PK)                  │     │ id (PK)                  │
│ stack_config_id (FK)     │     │ tool_name                │
│ channel (slack/email)    │     │ tool_version             │
│ webhook_url_enc          │     │ compatible_with (JSONB)  │
│ enabled                  │     │ verified_at              │
│ created_at               │     │ status                   │
└──────────────────────────┘     └──────────────────────────┘

┌──────────────────────────┐     ┌──────────────────────────┐
│ golden_path_templates    │     │ rbac_policies            │
│                          │     │                          │
│ id (PK)                  │     │ id (PK)                  │
│ name                     │     │ role_name                │
│ description              │     │ resource_type            │
│ tools_config (JSONB)     │     │ action                   │
│ resource_baseline (JSONB)│     │ effect (allow/deny)      │
│ created_at               │     │ created_at               │
└──────────────────────────┘     └──────────────────────────┘
```

#### 6.2 핵심 JSONB 구조

**stack_configs.config_json** — 스택 설정의 전체 상태를 단일 JSON으로 저장:

```json
{
  "artifacts": {
    "package_registry": { "tool": "gitlab", "version": "17.7.2" },
    "source_repository": { "tool": "gitlab", "version": "17.7.2" },
    "container_registry": { "tool": "gitlab-registry", "version": "17.7.2" },
    "storage_backend": { "tool": "minio", "version": "2024.11.7" }
  },
  "pipeline": {
    "ci_platform": { "tool": "gitlab-ci", "version": "17.7.2" },
    "cd_tool": { "tool": "argocd", "version": "2.13.2" }
  },
  "monitoring": {
    "collection": { "tool": "prometheus", "version": "3.1.0" },
    "visualization": { "tool": "grafana", "version": "11.4.0" }
  },
  "logging": {
    "collection": { "tool": "opentelemetry", "version": "0.115.0" },
    "search": { "tool": "opensearch", "version": "2.18.0" }
  },
  "resources": {
    "developers": 20,
    "concurrent_runners": 5,
    "weekly_commits": 100,
    "build_frequency": "hourly"
  },
  "cluster_id": "cls_abc123",
  "golden_path_id": "gitlab-allinone-v1",
  "custom_overrides": {}
}
```

#### 6.3 데이터 소스 경계

| 데이터 | 소스 | 저장소 | 갱신 방식 |
|--------|------|--------|-----------|
| 클러스터 메타데이터 (이름, 버전, 상태) | K8s API | PostgreSQL (캐시) | 주기적 동기화 (30초) |
| 설치/배포 이력 | Nullus 내부 | PostgreSQL | 이벤트 기반 |
| Pod/Service 실시간 상태 | K8s API | 미저장 (실시간 조회) | 요청 시 |
| Helm 릴리스 목록 | K8s API (Helm SDK) | PostgreSQL (캐시) | 설치/변경 시 |
| 사용자/프로젝트/RBAC | Nullus 내부 | PostgreSQL | CRUD |
| 모니터링 메트릭 | Prometheus API | 미저장 (프록시) | 요청 시 |

---

### 7. 배포 아키텍처

#### 7.1 Nullus 자체의 배포 구조

Nullus 컨트롤 플레인 자체도 Kubernetes에 배포됩니다 (또는 Docker Compose로 단독 실행).

```
배포 옵션 A: Kubernetes (권장)
┌─ nullus-system NS ─────────────────────────┐
│                                             │
│  ┌─────────────────┐  ┌─────────────────┐  │
│  │ nullus-api      │  │ nullus-web      │  │
│  │ (Go, 2 replica) │  │ (Nginx + React) │  │
│  │ Port: 8080      │  │ Port: 80/443    │  │
│  └────────┬────────┘  └────────┬────────┘  │
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
│  docker-compose.yaml                        │
│                                             │
│  nullus-api:    localhost:8090               │
│  nullus-web:    localhost:3000               │
│  postgresql:    localhost:5432               │
└─────────────────────────────────────────────┘
```

#### 7.2 Nullus가 설치하는 스택의 네임스페이스 구조

표준 네이밍은 `nullus-{service}` 접두사로 통일합니다.

```
대상 K8s 클러스터
│
├── nullus-system/          Nullus 에이전트 (상태 감시, 헬스체크)
│
├── nullus-artifacts/       스토리지 + 레지스트리
│   ├── minio               (PVC: 50Gi 기본)
│   └── harbor              (선택, GitLab Registry 사용 시 불필요)
│
├── nullus-scm/             소스 코드 관리
│   ├── gitlab-ce           (PVC: 30Gi 기본)
│   └── gitlab-runner       (DaemonSet 또는 Deployment)
│
├── nullus-cicd/            지속적 배포 (ArgoCD 등)
│   └── argocd              (Server, Repo Server, App Controller)
│
├── nullus-monitoring/      모니터링
│   ├── prometheus           (PVC: 20Gi 기본)
│   └── grafana
│
├── nullus-logging/         로깅
│   ├── otel-collector
│   └── opensearch          (PVC: 30Gi 기본)
│
└── app-{name}/             사용자 애플리케이션 (파이프라인으로 생성)
    ├── deployment
    ├── service
    ├── ingress
    └── secret
```

---

### 8. 보안 아키텍처

```
┌─ 데이터 흐름 보안 ─────────────────────────────────────────┐
│                                                             │
│  브라우저 ──HTTPS/TLS──→ Nullus API                        │
│                            │                               │
│                            ├─ Kubeconfig: AES-256 암호화   │
│                            │   저장 (DB, 복호화는 메모리)   │
│                            │                               │
│                            ├─ API Token: K8s Secret 저장    │
│                            │                               │
│                            ├─ 세션: HttpOnly + Secure 쿠키  │
│                            │                               │
│                            └─ DB 연결: TLS (선택)           │
│                                                             │
│  Nullus API ──kubeconfig──→ K8s API Server                 │
│                 (메모리에서 복호화 후 사용)                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘

┌─ RBAC 모델 (v1) ──────────────────────────────────────────┐
│                                                             │
│  Admin                                                     │
│  ├── Organization 관리 (생성, 수정, 삭제)                   │
│  ├── 멤버 관리 (초대, 역할 변경, 비활성화)                   │
│  ├── 클러스터 관리 (등록, 삭제)                              │
│  ├── 스택 설정/배포 (전체)                                   │
│  └── 사용자 관리                                            │
│                                                             │
│  DevOps Engineer                                            │
│  ├── 클러스터 조회                                           │
│  ├── 스택 설정/배포 (생성, 수정)                             │
│  ├── 파이프라인 배포                                         │
│  └── 모니터링 조회                                           │
│                                                             │
│  Developer                                                  │
│  ├── 클러스터 조회 (읽기 전용)                               │
│  ├── 스택 설정 조회 (읽기 전용)                              │
│  ├── 배포 이력 조회                                          │
│  └── 모니터링 조회                                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘

### 보안 운영 정책

| 항목 | 정책 | 책임 |
|------|------|------|
| DB 암호화 키 회전 | 90일 주기 | DevOps |
| kubeconfig 암호화 키 회전 | 90일 주기 | DevOps |
| 감사로그 보존 기간 | 1년 (v1.0), 추후 확장 | BE-1 |
| DB 백업 | 일 1회 자동, RPO 24시간, RTO 4시간 | DevOps |
| 세션/토큰 만료 | 액세스 토큰 15분, 리프레시 토큰 7일 | BE-1 |
| 접근 감사 | 관리자 작업 전건 기록 | BE-2 |

---

### 9. 운영 및 마이그레이션 전략

#### 9.1 DB 마이그레이션
- **도구**: `golang-migrate` 사용
- **전략**: 
  - Alpha/Beta: 순방향(Up) 마이그레이션만 지원
  - v1.0 GA: 양방향(Up/Down) 마이그레이션 및 롤백 테스트 자동화
- **검증**: CI 파이프라인에서 매 PR마다 마이그레이션 스크립트 유효성 검사

#### 9.2 API 버전 관리 및 하위 호환성
- **버전 정책**: `/api/v1/` 경로 고정
- **Breaking Changes**: 하위 호환성을 깨는 변경이 필요한 경우 `/api/v2/` 신설
- **Deprecation Window**: 기능 제거 시 최소 2 스프린트 (4주) 전 공지 및 경고(Warning) 헤더 포함

---

## Part 2: 상세 기능 명세

---

### 기능 0: Organization 설정 등록

**목적**: Nullus를 팀 단위로 사용하기 위한 조직 관리

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/orgs` | Organization 생성 | Alpha |
| GET | `/api/v1/orgs/:orgId` | Organization 조회 | Alpha |
| PUT | `/api/v1/orgs/:orgId` | Organization 수정 | Beta |
| PUT | `/api/v1/orgs/:orgId/status` | 활성/비활성 전환 | v1 |
| POST | `/api/v1/orgs/:orgId/members` | 멤버 초대 (초대 링크) | Beta |
| GET | `/api/v1/orgs/:orgId/members` | 멤버 목록 조회 | Beta |
| PUT | `/api/v1/orgs/:orgId/members/:userId` | 멤버 역할 변경 | v1 |
| DELETE | `/api/v1/orgs/:orgId/members/:userId` | 멤버 제거 | v1 |

#### 데이터 모델 상세

```
organizations
├── id             UUID, PK, auto-generated
├── name           VARCHAR(100), NOT NULL
├── slug           VARCHAR(50), UNIQUE, URL-safe
├── domain         VARCHAR(255), NULLABLE (회사 도메인)
├── status         ENUM('active', 'inactive'), DEFAULT 'active'
├── created_by     UUID, FK → users.id
├── created_at     TIMESTAMP WITH TIME ZONE
└── updated_at     TIMESTAMP WITH TIME ZONE

org_members
├── id             UUID, PK
├── org_id         UUID, FK → organizations.id
├── user_id        UUID, FK → users.id
├── role           ENUM('admin', 'devops', 'developer')
├── invited_at     TIMESTAMP
├── accepted_at    TIMESTAMP, NULLABLE
└── UNIQUE(org_id, user_id)
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Alpha** | Org 생성 (name, slug), 단일 Admin 자동 지정, 세션 인증 | **Given** 유효한 이름과 슬러그, **When** 조직 생성 요청, **Then** DB에 저장되고 생성자가 Admin으로 지정됨 |
| **Beta** | 멤버 초대 (이메일 전송 없이 링크 생성), 기본 역할 부여 (admin/devops/developer) | **Given** 생성된 초대 링크, **When** 다른 사용자가 링크 접속, **Then** 해당 조직의 멤버로 등록됨 |
| **v1** | 활성/비활성 상태 전환, 클러스터 접근 범위 설정, 멤버 관리 UI 전체 | **Given** 비활성화된 조직, **When** 멤버가 API 요청, **Then** 403 Forbidden 반환 |

---

### 기능 1: K8S Cluster Configurations 등록

**목적**: Nullus가 도구를 설치할 대상 Kubernetes 클러스터를 등록하고 관리

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/clusters` | 클러스터 등록 (kubeconfig 업로드) | Alpha |
| GET | `/api/v1/clusters` | 클러스터 목록 조회 | Alpha |
| GET | `/api/v1/clusters/:id` | 클러스터 상세 조회 | Alpha |
| PUT | `/api/v1/clusters/:id` | 클러스터 정보 수정 | Alpha |
| DELETE | `/api/v1/clusters/:id` | 클러스터 삭제 | Alpha |
| POST | `/api/v1/clusters/:id/verify` | 연결 검증 (kubectl version) | Alpha |
| GET | `/api/v1/clusters/:id/namespaces` | 네임스페이스 목록 조회 | Alpha |

#### 상태값 Enum 및 UI 매핑

| Enum (Internal) | 한글 표시 (UI) | 설명 |
|---|---|---|
| `connected` | 연결됨 | 클러스터 API 서버와 통신 성공 |
| `pending` | 대기 중 | 등록 후 아직 검증되지 않음 |
| `unreachable` | 접근 불가 | 네트워크 오류 또는 엔드포인트 응답 없음 |
| `auth_failed` | 인증 실패 | Kubeconfig 인증 정보가 유효하지 않음 |

#### 등록 흐름

```
사용자 행위                        API 처리
───────────                        ────────
kubeconfig 파일 선택     →    파일 파싱, context 추출
                         →    AES-256으로 암호화 후 DB 저장
클러스터 이름 입력        →    유니크 검증
클러스터 타입 선택        →    pipeline / target 구분 저장
  (pipeline: 도구 설치용)
  (target: 앱 배포용)
"연결 테스트" 클릭        →    kubeconfig 복호화 → kubectl version 실행
                         →    K8s 버전 확인, API 접근 가능 여부 판단
                         →    결과: connected / unreachable / auth_failed
연결 성공 확인            →    상태를 'connected'로 업데이트
                         →    네임스페이스 목록 캐시
```

#### UI 구성

```
┌─ Cluster Management ────────────────────────────────────────┐
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ + 클러스터 추가                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─────────────────────────────────────────────────┐        │
│  │ 🟢 production-gke    pipeline   v1.28   연결됨   │        │
│  │    endpoint: https://35.x.x.x                   │        │
│  │    네임스페이스: 12개    마지막 확인: 5분 전       │        │
│  ├─────────────────────────────────────────────────┤        │
│  │ 🟡 staging-eks       target     v1.27   대기     │        │
│  │    endpoint: https://api.eks.amazonaws.com      │        │
│  │    연결 테스트 필요                               │        │
│  ├─────────────────────────────────────────────────┤        │
│  │ 🔴 dev-kind          pipeline   v1.29   미연결   │        │
│  │    endpoint: https://127.0.0.1:6443             │        │
│  └─────────────────────────────────────────────────┘        │
└──────────────────────────────────────────────────────────────┘
```

---

### 기능 2: 노코드 기반 DevSecOps Stack 설정 UI

**목적**: 웹 UI에서 도구를 선택하고, 스택 전체 구성을 노코드 방식으로 설정

#### API 엔드포인트

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| GET | `/api/v1/stacks` | 스택 설정 목록 조회 | `[]StackConfig` |
| POST | `/api/v1/stacks` | 스택 설정 생성 | `CreateStackRequest` / `StackConfig` |
| GET | `/api/v1/stacks/:id` | 스택 설정 상세 조회 | `StackConfig` |
| PUT | `/api/v1/stacks/:id` | 스택 설정 수정 | `UpdateStackRequest` / `StackConfig` |
| GET | `/api/v1/stacks/:id/history` | 설정 변경 이력 조회 | `[]StackVersion` |

#### 5단계 설정 워크플로우

```
┌──────────────────────────────────────────────────────────────────┐
│  Step 1        Step 2         Step 3        Step 4      Step 5   │
│ [Artifacts] → [Pipeline] → [Monitoring] → [Logging] → [Resources]│
│  ████████      ████████      ░░░░░░░░      ░░░░░░░░    ░░░░░░░░ │
│  (완료)        (현재)         (미완료)       (미완료)    (미완료)  │
└──────────────────────────────────────────────────────────────────┘
```

#### 탭별 상세 옵션

**Step 1: Artifacts**

| 카테고리 | 도구 옵션 | 기본값 | 비고 |
|---|---|---|---|
| Package Registry | GitLab, Nexus, JFrog Artifactory, Harbor | GitLab | GitLab 선택 시 내장 |
| Source Repository | GitLab, GitHub, Gitea | GitLab | |
| Container Registry | GitLab Registry, Harbor, Docker Hub | GitLab Registry | |
| Storage Backend | MinIO, AWS S3, GCS | MinIO | S3/GCS는 외부 연결 |

각 도구별로 버전 드롭다운 제공. 호환성 매트릭스에서 검증된 버전만 "Recommended" 뱃지 표시.

**Step 2: Pipeline Tools**

| 카테고리 | 도구 옵션 | 기본값 | 선택 방식 |
|---|---|---|---|
| CI/CD Platform | GitLab CI, GitHub Actions, Jenkins | GitLab CI | 라디오 버튼 (단일) |
| CD Tool | Argo CD, Flux | Argo CD | 라디오 버튼 (단일) |

**Step 3: Monitoring Tools**

| 카테고리 | 도구 옵션 | 기본값 |
|---|---|---|
| Collection | Prometheus, Thanos | Prometheus |
| Query & Visualization | Grafana | Grafana (고정) |

**Step 4: Logging Tools**

| 카테고리 | 도구 옵션 | 기본값 |
|---|---|---|
| Collection | OpenTelemetry, Loki | OpenTelemetry |
| Query & Search | OpenSearch, Elasticsearch | OpenSearch |

**Step 5: Resources**

| 입력 필드 | 타입 | 기본값 | 설명 |
|---|---|---|---|
| 개발자 수 | 숫자 입력 | 20 | 팀 전체 인원 |
| 동시 러너 수 | 숫자 입력 | 5 | CI 동시 빌드 수 |
| 주간 커밋 수 | 숫자 입력 | 100 | 예상 주간 커밋 |
| 빌드 빈도 | 드롭다운 | hourly | hourly / daily / on-push |
| 통화 | 드롭다운 | USD | USD / KRW / CNY |

→ 자동 계산 출력: CPU (cores), Memory (Gi), Storage (Gi), 예상 월 비용

#### 파이프라인 8단계 시각화

```
┌─────────────────────────────────────────────────────────────┐
│                                                              │
│  Develop → Build → Security → Test → Deploy → Ops → Mon → FinOps │
│  ████████  ████████  ░░░░░░░  ░░░░░  ████████ ░░░  ████  ░░░ │
│  (설정됨)  (설정됨)  (Phase2) (Ph2)  (설정됨) (-)  (설정)(Ph2)│
│                                                              │
│  설정된 단계: 파란색 글로우 + 아이콘                           │
│  미설정 단계: 회색 흐림 + "Phase 2에서 지원" 툴팁              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Configuration Summary 패널

화면 오른쪽에 상시 표시되며, 탭/선택 변경 시 실시간 갱신:

```
┌─ Configuration Summary ──────────┐
│                                   │
│  Golden Path: GitLab All-in-One   │
│                                   │
│  📦 Artifacts                     │
│    Source: GitLab CE 17.7.2       │
│    Registry: GitLab Registry      │
│    Storage: MinIO 2024.11.7       │
│                                   │
│  🔧 Pipeline                      │
│    CI: GitLab CI 17.7.2           │
│    CD: Argo CD 2.13.2             │
│                                   │
│  📊 Monitoring                    │
│    Prometheus 3.1.0               │
│    Grafana 11.4.0                 │
│                                   │
│  📝 Logging                       │
│    OpenTelemetry 0.115.0          │
│    OpenSearch 2.18.0              │
│                                   │
│  💻 Resources (예상)              │
│    CPU: 12 cores                  │
│    Memory: 24 Gi                  │
│    Storage: 180 Gi                │
│    비용: ~$150/월 (AWS 기준)      │
│                                   │
│  ✅ 호환성: 검증 완료              │
│                                   │
│  [Deploy Stack]                   │
│                                   │
└───────────────────────────────────┘
```

#### YAML 에디터 (v1)

노코드 UI와 YAML 에디터 간 모드 전환 지원:

```
┌─ 모드 전환 ─────────────────────────────────┐
│  [노코드 UI]  [YAML 에디터]                  │
│                                              │
│  # nullus-stack.yaml                         │
│  apiVersion: nullus.io/v1alpha1              │
│  kind: StackConfig                           │
│  metadata:                                   │
│    name: my-devops-stack                     │
│  spec:                                       │
│    goldenPath: gitlab-allinone-v1            │
│    artifacts:                                │
│      sourceRepository:                       │
│        tool: gitlab                          │
│        version: "17.7.2"                     │
│    ...                                       │
│                                              │
│  ⚠️ Line 15: 'version' 값이 호환성           │
│     매트릭스에 없습니다                       │
│                                              │
└──────────────────────────────────────────────┘
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Alpha** | 3단계 (Artifacts, Pipeline, Monitoring), 노코드 UI만, Summary 패널 | **Given** 3단계 설정 완료, **When** 요약 패널 확인, **Then** 선택한 도구와 버전이 정확히 표시됨 |
| **Beta** | 5단계 전체, 8단계 파이프라인 시각화, Resources 탭 통화 선택 | **Given** 5단계 설정 완료, **When** 리소스 탭 진입, **Then** 예상 비용이 선택한 통화로 계산되어 표시됨 |
| **v1** | YAML 에디터 (Monaco), 노코드 ↔ YAML 양방향 동기화, 스키마 검증 | **Given** YAML 에디터 모드, **When** 잘못된 스키마 입력, **Then** 에디터 하단에 검증 에러 메시지 표시 |

---

### 기능 3: Golden Path 템플릿

**목적**: 검증된 CI/CD 도구 조합을 사전 정의하여 빠른 선택 지원

#### API 엔드포인트

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| GET | `/api/v1/templates/golden-paths` | Golden Path 목록 조회 | `[]GoldenPath` |
| GET | `/api/v1/templates/golden-paths/:id` | Golden Path 상세 조회 | `GoldenPath` |

#### 템플릿 목록

| ID | 이름 | 도구 조합 | 대상 | 릴리스 |
|---|---|---|---|---|
| `gitlab-allinone-v1` | GitLab All-in-One | GitLab CE + GitLab CI + GitLab Registry + MinIO + Argo CD + Prometheus + Grafana | 중견기업, 단일 플랫폼 선호 | Alpha |
| `gitlab-argocd-v1` | GitLab + Argo CD | GitLab CE + GitLab CI + Harbor + MinIO + Argo CD + Prometheus + Grafana | GitOps 중심 조직 | Beta |
| `github-argocd-v1` | GitHub + Argo CD | GitHub (외부) + GitHub Actions (외부) + Harbor + MinIO + Argo CD + Prometheus + Grafana | GitHub 사용 조직 | v1 |

#### 템플릿 선택 UI

```
┌─ Golden Path 선택 ──────────────────────────────────────────────┐
│                                                                  │
│  "검증된 조합을 선택하면 모든 설정이 자동으로 채워집니다."         │
│                                                                  │
│  ┌─────────────────────┐  ┌─────────────────────┐               │
│  │ ⭐ GitLab All-in-One │  │ GitLab + Argo CD    │               │
│  │                     │  │                     │               │
│  │ GitLab CE 17.7      │  │ GitLab CE 17.7      │               │
│  │ GitLab CI           │  │ GitLab CI           │               │
│  │ Argo CD 2.13        │  │ Harbor 2.11         │               │
│  │ Prometheus 3.1      │  │ Argo CD 2.13        │               │
│  │ Grafana 11.4        │  │ Prometheus 3.1      │               │
│  │                     │  │                     │               │
│  │ 설치 시간: ~90분     │  │ 설치 시간: ~120분    │               │
│  │ CPU: 8코어 / 16Gi   │  │ CPU: 10코어 / 20Gi  │               │
│  │ Storage: 100Gi      │  │ Storage: 130Gi      │               │
│  │                     │  │                     │               │
│  │ ✅ 검증 완료         │  │ ✅ 검증 완료         │               │
│  │                     │  │                     │               │
│  │ [선택하기]           │  │ [선택하기]           │               │
│  └─────────────────────┘  └─────────────────────┘               │
│                                                                  │
│  또는 [커스텀 구성] — 직접 도구를 선택합니다                      │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

### 기능 4: DevSecOps Stack 자동 설치/배포/이력 관리

**목적**: 선택한 도구 조합을 Kubernetes 클러스터에 자동으로 설치

#### 설치 순서 (의존성 DAG)

```
Step 1: Storage (MinIO)
    │
    ├──→ Step 2: Source Repo (GitLab CE)
    │        │
    │        ├──→ Step 3: Container Registry (GitLab Registry)
    │        │
    │        └──→ Step 4: CI Platform (GitLab CI Runner)
    │                 │
    │                 └──→ Step 5: CD Tool (Argo CD)
    │
    ├──→ Step 6: Monitoring (Prometheus + Grafana)
    │
    └──→ Step 7: Logging (OpenTelemetry + OpenSearch)

Step 8: Integration (모든 Step 완료 후)
    ├── GitLab ↔ Argo CD Webhook 연동
    ├── Prometheus ServiceMonitor 등록
    ├── Grafana 대시보드 프로비저닝
    └── GitLab Runner ↔ Container Registry 인증
```

> **Narwhal 레퍼런스**: 위 DAG는 Narwhal의 실전 검증된 설치 순서(07-cnpg → 14-bootstrap)를 참고하여 설계. 
> cert-manager와 TLS가 Keycloak OIDC보다 반드시 선행되어야 하며, DB(CNPG)는 모든 DB 의존 앱보다 선행.
> `known-issues.yaml`에 70+ Helm edge case 패턴을 코드화하여 설치 성공률 ≥90% 목표 달성.

#### 배포 상태 UI

```
┌─ Stack Deployment ──────────────────────────────────────────┐
│                                                              │
│  상태: INSTALLING (3/8 단계 완료)          시작: 14:32       │
│  ████████████░░░░░░░░░░░░░░░░░░  37%                        │
│                                                              │
│  ✅ Step 1: MinIO              완료 (2분 30초)               │
│  ✅ Step 2: GitLab CE          완료 (12분 15초)              │
│  ✅ Step 3: GitLab Registry    완료 (1분 45초)               │
│  🔄 Step 4: GitLab CI Runner   설치 중... (3분 경과)         │
│  ⏳ Step 5: Argo CD            대기 중                       │
│  ⏳ Step 6: Prometheus+Grafana  대기 중                       │
│  ⏳ Step 7: OTel+OpenSearch     대기 중                       │
│  ⏳ Step 8: Integration         대기 중                       │
│                                                              │
│  ┌─ 실시간 로그 ──────────────────────────────────────────┐  │
│  │ [14:47:12] Installing gitlab-runner helm chart...      │  │
│  │ [14:47:13] Waiting for runner pod to be ready...       │  │
│  │ [14:47:25] Runner pod gitlab-runner-0 is Running       │  │
│  │ [14:47:26] Registering runner with GitLab instance...  │  │
│  │ ▊                                                      │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  [롤백] [중단]                                               │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### 이력 관리 (v1)

```
┌─ Stack History ─────────────────────────────────────────────┐
│                                                              │
│  Version  변경자   시간              변경 내용               │
│  ─────── ──────── ──────────────── ────────────────────────  │
│  v3      김민수   2026-05-20 14:30  Argo CD 2.13.2 → 2.14.0│
│          [diff] [롤백]                                      │
│                                                              │
│  v2      이미정   2026-05-15 09:15  러너 동시 실행 5 → 8    │
│          [diff] [롤백]                                      │
│                                                              │
│  v1      이미정   2026-05-10 11:00  최초 설치                │
│          [diff]                                              │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Alpha** | Deploy → 순차 설치 + 실시간 로그. 롤백/이력 없음 | **Given** 유효한 스택 설정, **When** 설치 시작, **Then** 각 단계의 로그가 실시간으로 UI에 출력됨 |
| **Beta** | 설치 실패 시 자동 롤백, 헬스체크, 연동 설정 자동화 | **Given** 설치 중 특정 단계 실패, **When** 자동 롤백 활성화 상태, **Then** 이전 단계까지 설치된 리소스가 `safe` 모드로 정리됨 |
| **v1** | 설정 변경 시 스냅샷 저장, 버전별 diff, 특정 버전 롤백 | **Given** 설치 완료된 스택, **When** 재시도 실행, **Then** 실패한 단계부터 설치가 재개됨 |

---

### 기능 5: CI/CD Pipeline 템플릿

**목적**: 애플리케이션 배포를 위한 표준 CI/CD 파이프라인 템플릿 제공

#### API 엔드포인트

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| GET | `/api/v1/templates/pipelines` | 파이프라인 템플릿 목록 조회 | `[]PipelineTemplate` |
| GET | `/api/v1/templates/pipelines/:id` | 파이프라인 템플릿 상세 조회 | `PipelineTemplate` |

#### 템플릿 목록

| ID | 이름 | 대상 | 포함 단계 | 릴리스 |
|---|---|---|---|---|
| `web-backend-v1` | Web Backend | Spring Boot, Express, Django 등 | Build → Test → Image Build → Deploy | Beta |
| `web-frontend-v1` | Web Frontend | React, Vue, Next.js 등 | Build → Test → Static Build → Deploy (Nginx) | v1 |
| `batch-job-v1` | Batch Job | 크론 작업, 데이터 처리 | Build → Image Build → CronJob Deploy | v1 |

#### 파이프라인 배포 시 필수 K8s Object 자동 생성

```yaml
# 파이프라인 배포 시 자동 생성되는 리소스
Namespace:     app-{pipeline-name}
Deployment:    {pipeline-name}-deployment
Service:       {pipeline-name}-service (ClusterIP)
Ingress:       {pipeline-name}-ingress (도메인 설정 시)
Secret:        {pipeline-name}-registry-secret (이미지 풀 인증)
PVC:           {pipeline-name}-data (필요 시)
ServiceAccount: {pipeline-name}-sa (최소 권한)
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Beta** | Web Backend 템플릿 제공, 기본 파라미터 설정 | **Given** Backend 템플릿 선택, **When** 파이프라인 생성, **Then** Build/Test/Deploy 단계가 포함된 설정 생성 |
| **v1** | Frontend, Batch Job 템플릿 추가, 커스텀 단계 정의 | **Given** Frontend 템플릿 선택, **When** 배포 실행, **Then** Nginx 기반 정적 파일 배포 설정 자동 생성 |

---

### 기능 6: CI/CD Pipeline 배포/이력 관리

**목적**: 파이프라인으로 애플리케이션을 배포하고 이력을 추적

#### API 엔드포인트

| Method | Path | 설명 | 릴리스 |
|---|---|---|---|
| POST | `/api/v1/pipelines` | 파이프라인 생성 (템플릿 + 파라미터) | Beta |
| GET | `/api/v1/pipelines` | 파이프라인 목록 | Beta |
| GET | `/api/v1/pipelines/:id` | 파이프라인 상세 | Beta |
| POST | `/api/v1/pipelines/:id/deploy` | 파이프라인 배포 실행 | Beta |
| GET | `/api/v1/pipelines/:id/deployments` | 배포 이력 조회 | Beta |
| GET | `/api/v1/pipelines/:id/deployments/:did` | 배포 상세 (생성된 K8s 오브젝트 목록) | v1 |
| POST | `/api/v1/pipelines/:id/rollback/:did` | 특정 버전으로 롤백 | v1 |
| GET | `/api/v1/pipelines/:id/deployments/:did/diff` | 이전 버전과 diff | v1 |

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Beta** | 파이프라인 생성/배포, 기본 이력 (버전/시간/결과/상태 필터링) | **Given** 생성된 파이프라인, **When** 배포 실행, **Then** 배포 이력에 새로운 버전이 추가되고 상태가 추적됨 |
| **v1** | 롤백, diff, 변경자/사유 기록, K8s 오브젝트 상세 조회 | **Given** 이전 배포 성공 이력, **When** 롤백 실행, **Then** 해당 버전의 K8s 오브젝트 상태로 복구됨 |

---

### 기능 7: 모니터링/알림 관리

**목적**: 설치된 스택과 사용자 애플리케이션의 상태를 모니터링 및 알림 송신

#### API 엔드포인트

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| GET | `/api/v1/monitoring/dashboards` | 대시보드 목록 조회 | `[]Dashboard` |
| GET | `/api/v1/monitoring/metrics/summary` | 핵심 지표 요약 조회 | `MetricSummary` |
| POST | `/api/v1/monitoring/alerts/config` | 알림 설정 저장 | `AlertConfig` |

#### 기본 대시보드 구성

```
┌─ Monitoring Overview ───────────────────────────────────────┐
│                                                              │
│  ┌─ Cluster Health ──────────┐  ┌─ Pipeline Status ───────┐ │
│  │ CPU: ████████░░  78%      │  │ 성공: 142  실패: 3      │ │
│  │ MEM: ██████░░░░  62%      │  │ 성공률: 97.9%           │ │
│  │ STO: ████░░░░░░  41%      │  │ 평균 빌드 시간: 4분 32초 │ │
│  └───────────────────────────┘  └─────────────────────────┘ │
│                                                              │
│  ┌─ Tool Health ─────────────────────────────────────────┐  │
│  │ 🟢 GitLab CE      Running    CPU: 2.1/4  MEM: 6.2/8  │  │
│  │ 🟢 Argo CD        Running    CPU: 0.5/2  MEM: 1.1/4  │  │
│  │ 🟢 Prometheus     Running    CPU: 0.8/2  MEM: 3.2/8  │  │
│  │ 🟢 Grafana        Running    CPU: 0.2/1  MEM: 0.5/2  │  │
│  │ 🟡 GitLab Runner  Warning    CPU: 3.8/4  MEM: 7.5/8  │  │
│  │ 🟢 MinIO          Running    CPU: 0.3/1  MEM: 0.8/2  │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  [Grafana에서 상세 보기 →]                                   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### 알림 연동 (Slack)

```json
// POST /api/v1/monitoring/alerts/config
{
  "channel": "slack",
  "webhook_url": "https://hooks.slack.com/services/...",
  "events": [
    "tool_down",           // 도구 비정상 종료
    "high_cpu",            // CPU 90% 이상 5분 지속
    "high_memory",         // Memory 90% 이상 5분 지속
    "storage_warning",     // Storage 80% 이상
    "pipeline_failure"     // 파이프라인 3회 연속 실패
  ],
  "enabled": true
}
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Beta** | 기본 대시보드 (클러스터+도구 상태), 핵심 지표 (CPU/MEM/Storage), Slack 알림 | **Given** Slack 웹훅 설정 완료, **When** 도구 상태가 `Down`으로 변경, **Then** 설정된 Slack 채널로 즉시 알림 전송 |
| **v1** | 파이프라인 성공률 메트릭, Grafana 대시보드 자동 프로비저닝 확장, Email 알림 | **Given** 파이프라인 배포 완료, **When** 모니터링 뷰 진입, **Then** 해당 파이프라인의 성공률 및 평균 소요 시간 그래프 표시 |

---

### 기능 8: DevSecOps Stack OSS 버전 호환성 관리

**목적**: 도구 간 버전 호환성을 사전 검증하여 설치 실패 방지

#### 검증 API

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| GET | `/api/v1/compatibility/matrix` | 호환성 매트릭스 전체 조회 | `CompatibilityMatrix` |
| POST | `/api/v1/compatibility/validate` | 특정 조합 유효성 검증 | `ValidateRequest` / `ValidateResponse` |

#### 검증 API 예시

```
POST /api/v1/compatibility/validate

Request:
{
  "tools": [
    { "name": "gitlab-ce", "version": "17.7.2" },
    { "name": "argocd", "version": "2.13.2" },
    { "name": "prometheus", "version": "3.1.0" }
  ],
  "kubernetes_version": "1.28"
}

Response:
{
  "compatible": true,
  "matrix_id": "gitlab-allinone-v1",
  "status": "verified",
  "warnings": [],
  "recommendations": [
    { "tool": "argocd", "current": "2.13.2", "recommended": "2.14.0", "reason": "보안 패치" }
  ]
}

// 비검증 조합인 경우:
{
  "compatible": false,
  "status": "untested",
  "warnings": [
    "GitLab CE 18.0 + Argo CD 2.13 조합은 테스트되지 않았습니다."
  ],
  "closest_verified": "gitlab-allinone-v1"
}
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Alpha** | 매트릭스 1개 (GitLab All-in-One), API 검증 | **Given** 도구 조합 입력, **When** 검증 API 호출, **Then** 매트릭스 일치 여부 및 권장 버전 반환 |
| **Beta** | 매트릭스 2개, 설정 UI에서 비검증 조합 선택 시 콘솔 경고 | **Given** 비검증 버전 선택, **When** 설정 저장 시도, **Then** UI 상단에 호환성 경고 팝업 표시 |
| **v1** | 매트릭스 3개, UI 경고 팝업, 권장 버전 자동 선택, "Recommended" 뱃지 | **Given** 특정 도구 선택, **When** 버전 드롭다운 오픈, **Then** 검증된 버전에 `Recommended` 뱃지 표시 |

---

### 기능 9: UI 권한 체계

**목적**: 역할 기반 접근 제어로 팀별 기능 통제 (v1에서 구현)

#### API 엔드포인트

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| GET | `/api/v1/users` | 사용자 목록 조회 | `[]User` |
| PUT | `/api/v1/users/:userId/role` | 사용자 역할 변경 | `UpdateRoleRequest` |
| DELETE | `/api/v1/users/:userId` | 사용자 제거 | - |
| GET | `/api/v1/auth/me` | 내 정보 및 권한 조회 | `UserContext` |

#### RBAC 매핑

| 기능 영역 | Admin | DevOps Engineer | Developer |
|---|---|---|---|
| Organization 관리 | ✅ 전체 | ❌ | ❌ |
| 사용자 관리 | ✅ 전체 | ❌ | ❌ |
| 클러스터 등록/삭제 | ✅ | ❌ | ❌ |
| 클러스터 조회 | ✅ | ✅ | ✅ |
| 스택 설정 생성/수정 | ✅ | ✅ | ❌ |
| 스택 설정 조회 | ✅ | ✅ | ✅ |
| 스택 배포 | ✅ | ✅ | ❌ |
| 파이프라인 배포 | ✅ | ✅ | ❌ |
| 배포 이력 조회 | ✅ | ✅ | ✅ |
| 모니터링 조회 | ✅ | ✅ | ✅ |
| 알림 설정 | ✅ | ✅ | ❌ |

#### Keycloak 연동 (v1)

```
사용자 로그인 흐름:
Browser → Nullus Web → Keycloak OIDC → ID Token 발급
         → Nullus API (ID Token 검증)
         → DB에서 org_members.role 조회
         → 권한에 따라 API 응답 필터링

OSS별 권한 매핑:
Keycloak Role "admin" → GitLab Admin + Argo CD Admin + Grafana Admin
Keycloak Role "devops" → GitLab Maintainer + Argo CD Read-only + Grafana Editor
Keycloak Role "developer" → GitLab Reporter + Argo CD Read-only + Grafana Viewer
```

#### Narwhal Keycloak OIDC 자동 설정 레퍼런스

Nullus의 Keycloak 자동 설정은 Narwhal의 7-app OIDC 연동 구현(`11-keycloak.sh`)을 Go 코드로 전환하여 구현합니다.

**구현 플로우**:
1. Keycloak realm 생성
2. `groups` client scope 생성 + group membership mapper 추가
3. 앱별 클라이언트 생성 (ArgoCD, Grafana, Gitea 등)
4. 전체 클라이언트에 `groups` scope를 default scope로 할당
5. 각 앱의 Helm values에 OIDC 설정 주입
6. K8s API Server OIDC 연동 (선택)

**알려진 SSO 이슈 (사전 처리)**:
- `groups` scope 미생성 시 `invalid_scope` 에러 → 자동 생성 로직 포함
- ArgoCD: `x509: certificate signed by unknown authority` → self-signed cert 자동 처리
- Headlamp: `oidc-skip-issuer-tls-verify` 미지원 → CA cert 직접 마운트

---

### 기능 10: 리소스 예상량 계산

**목적**: 설치 전 필요한 인프라 리소스를 사전에 파악

#### 계산 로직

```
입력:
  developers = 20
  concurrent_runners = 5
  weekly_commits = 100
  build_frequency = "hourly"

기본 리소스 (도구 자체):
  도구별 기본값 테이블에서 합산
  ┌──────────────────┬───────┬────────┬─────────┐
  │ 도구             │ CPU   │ Memory │ Storage │
  ├──────────────────┼───────┼────────┼─────────┤
  │ GitLab CE        │ 4     │ 8 Gi   │ 30 Gi   │
  │ GitLab Runner    │ 2     │ 4 Gi   │ 10 Gi   │
  │ Argo CD          │ 1     │ 2 Gi   │ 5 Gi    │
  │ Prometheus       │ 1     │ 4 Gi   │ 20 Gi   │
  │ Grafana          │ 0.5   │ 1 Gi   │ 5 Gi    │
  │ MinIO            │ 0.5   │ 1 Gi   │ 50 Gi   │
  │ OpenTelemetry    │ 0.5   │ 1 Gi   │ 0 Gi    │
  │ OpenSearch       │ 2     │ 4 Gi   │ 30 Gi   │
  └──────────────────┴───────┴────────┴─────────┘
  기본 합계:          11.5    25 Gi    150 Gi

스케일링 팩터 (v1 동적 계산):
  runner_scale = concurrent_runners / 5  (기본 5 대비)
  commit_scale = weekly_commits / 100    (기본 100 대비)

  추가 CPU = runner_scale * 2
  추가 Memory = runner_scale * 4 Gi
  추가 Storage = commit_scale * 20 Gi  (빌드 아티팩트)

비용 추정 (v1):
  AWS 기준: CPU $0.05/core/hr, Memory $0.005/Gi/hr, Storage $0.10/Gi/month
  GCP 기준: CPU $0.04/core/hr, Memory $0.004/Gi/hr, Storage $0.08/Gi/month
  통화 변환: KRW = USD * 1,350, CNY = USD * 7.2

계산 단위 및 유효성 규칙:
  통화 단위: USD (소수점 2자리, HALF_UP 반올림)
  리소스 단위:
    - vCPU: 0.001 단위 (예: 0.500, 1.250)
    - Memory: MiB 단위 (예: 512 MiB, 8192 MiB)
    - Storage: GiB 단위 (예: 30 GiB, 100 GiB)
  유효 입력 범위:
    - vCPU: 0.1 ~ 1000 (범위 초과 시 에러 반환)
    - Memory: 128 MiB ~ 1 TiB (범위 초과 시 에러 반환)
    - Storage: 1 GiB ~ 100 TiB (범위 초과 시 에러 반환)
  0 또는 음수 입력 시: HTTP 400 에러 반환 (RESOURCE_INVALID_VALUE)
```

#### 점진적 완성 계획 및 수용 기준

| 릴리스 | 구현 범위 | 수용 기준 (Acceptance Criteria) |
|---|---|---|
| **Alpha** | 매트릭스 1개 (GitLab All-in-One), API 조회 | **Given** 템플릿 선택 화면, **When** GitLab All-in-One 선택, **Then** 모든 설정 단계가 해당 템플릿 값으로 자동 채워짐 |
| **Beta** | 매트릭스 2개, 템플릿별 설치 예상 시간 표시 | **Given** 템플릿 상세 정보, **When** 조회 요청, **Then** 예상 설치 시간 및 필요 리소스 정보가 포함됨 |
| **v1** | 매트릭스 3개, 커스텀 템플릿 저장 기능 | **Given** 사용자 설정 스택, **When** 템플릿으로 저장, **Then** 이후 템플릿 목록에서 선택 가능 |

---

### 기능 4: DevSecOps Stack 자동 설치/배포/이력 관리

**목적**: 선택한 도구 조합을 Kubernetes 클러스터에 자동으로 설치

#### 서브시스템 구성

- **F4a: Helm 차트 관리**: 저장소 등록, 버전 관리, values.yaml 오버라이드 로직
- **F4b: 설치 DAG 엔진**: 도구 간 의존성 해석, 실행 순서 결정, 단계별 게이트 검증
- **F4c: 실시간 진행률**: WebSocket 기반 상태 전송, 설치 로그 스트리밍
- **F4d: 롤백/재시도**: 실패 시 자동/수동 롤백, 특정 단계부터 재시도(RETRY) 모드
- **F4e: 상태 관리**: 설치 이력 저장, 현재 클러스터의 도구 버전 추적
- **F4f: 호환성 검증**: 설치 전 K8s 버전 및 도구 조합 최종 검증
- **F4g: 배포 스크립트 생성**: (Phase 2 예정) 외부 배포용 스크립트 추출

#### API 엔드포인트

| Method | Path | 설명 | 요청/응답 형식 |
|---|---|---|---|
| POST | `/api/v1/installations` | 설치 시작 | `InstallRequest` / `Deployment` |
| DELETE | `/api/v1/installations/:id` | 설치 취소 | - |
| POST | `/api/v1/installations/:id/retry` | 실패 단계 재시도 | `Deployment` |
| POST | `/api/v1/installations/:id/rollback` | 롤백 실행 | `Deployment` |
| GET | `/api/v1/installations/:id/status` | 상태 조회 | `DeploymentStatus` |
| GET | `/api/v1/installations/:id/logs` | 로그 스트림 (HTTP/WS) | Stream |

#### 설치 순서 (의존성 DAG)

```
Phase A: 인프라 기반
Step 1: cert-manager (TLS)
    │
    └──→ Step 2: CNPG (PostgreSQL)
             │
             └──→ Step 3: Storage (MinIO)

Phase B: 핵심 서비스 (Phase A 완료 후)
Step 4: Source Repo (GitLab CE)
    │
    ├──→ Step 5: Container Registry (Harbor / GitLab Registry)
    │
    └──→ Step 6: CD Tool (Argo CD)

Phase C: 보조 서비스 (Phase B 완료 후)
Step 7: Monitoring (Prometheus + Grafana)
    │
    └──→ Step 8: Loki (로깅)

Step 9: Integration (모든 Step 완료 후)
    ├── GitLab ↔ Argo CD Webhook 연동
    ├── Prometheus ServiceMonitor 등록
    ├── Grafana 대시보드 프로비저닝
    └── GitLab Runner ↔ Container Registry 인증
```
nullus/
├── .github/
│   ├── workflows/
│   │   ├── ci.yaml              # 린트, 테스트, 빌드
│   │   ├── release.yaml         # 릴리스 자동화
│   │   └── compatibility.yaml   # 호환성 매트릭스 자동 테스트
│   └── ISSUE_TEMPLATE/
│       ├── bug_report.md
│       └── feature_request.md
│
├── api/
│   ├── openapi.yaml             # OpenAPI 3.0 스펙
│   └── proto/                   # (향후 gRPC 도입 시)
│
├── cmd/
│   └── nullus-server/
│       └── main.go              # API 서버 진입점
│
├── internal/
│   ├── handler/                 # HTTP 핸들러 (Gin/Echo)
│   │   ├── auth.go
│   │   ├── org.go
│   │   ├── cluster.go
│   │   ├── stack.go
│   │   ├── deployment.go
│   │   ├── template.go
│   │   ├── monitoring.go
│   │   ├── compatibility.go
│   │   └── resource.go
│   ├── service/                 # 비즈니스 로직
│   │   ├── org_service.go
│   │   ├── cluster_service.go
│   │   ├── stack_service.go
│   │   └── ...
│   ├── engine/                  # 설치 엔진 코어
│   │   ├── orchestrator.go      # 설치 오케스트레이터
│   │   ├── state_machine.go     # 상태 머신
│   │   ├── step_runner.go       # 단계별 실행기
│   │   ├── rollback.go          # 롤백 관리자
│   │   ├── log_streamer.go      # 로그 스트리머
│   │   └── steps/               # 개별 도구 설치 로직
│   │       ├── minio.go
│   │       ├── gitlab.go
│   │       ├── argocd.go
│   │       ├── prometheus.go
│   │       └── integration.go
│   ├── compatibility/           # 호환성 매트릭스 엔진
│   │   ├── engine.go
│   │   └── matrix_loader.go
│   ├── repository/              # 데이터 액세스 (PostgreSQL)
│   │   ├── org_repo.go
│   │   ├── cluster_repo.go
│   │   ├── stack_repo.go
│   │   └── ...
│   ├── middleware/               # 인증, 로깅, CORS
│   │   ├── auth.go
│   │   ├── rbac.go              # (v1)
│   │   └── logging.go
│   └── config/                  # 서버 설정
│       └── config.go
│
├── db/
│   ├── migrations/              # SQL 마이그레이션 파일
│   │   ├── 001_init.up.sql
│   │   ├── 001_init.down.sql
│   │   └── ...
│   └── seed/                    # 초기 데이터 (Golden Path, 매트릭스)
│       ├── golden_paths.sql
│       └── compatibility_matrix.sql
│
├── web/                         # React 프론트엔드
│   ├── src/
│   │   ├── components/          # 재사용 컴포넌트
│   │   ├── pages/               # 페이지 컴포넌트
│   │   │   ├── Login.tsx
│   │   │   ├── Dashboard.tsx
│   │   │   ├── ClusterManagement.tsx
│   │   │   ├── StackSetup.tsx   # 5단계 워크플로우
│   │   │   ├── GoldenPathSelector.tsx
│   │   │   ├── DeploymentView.tsx
│   │   │   ├── PipelineManagement.tsx
│   │   │   ├── MonitoringDashboard.tsx
│   │   │   └── UserManagement.tsx  # (v1)
│   │   ├── stores/              # Zustand 상태 관리
│   │   ├── hooks/               # 커스텀 훅
│   │   ├── api/                 # API 클라이언트
│   │   └── types/               # TypeScript 타입
│   ├── package.json
│   └── tsconfig.json
│
├── charts/                      # Nullus 자체 Helm 차트
│   └── nullus/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│
├── templates/                   # Golden Path / CI/CD 템플릿
│   ├── golden-paths/
│   │   ├── gitlab-allinone-v1/
│   │   │   ├── manifest.yaml    # 템플릿 메타데이터
│   │   │   ├── values/          # 도구별 Helm values
│   │   │   └── integration/     # 연동 설정 스크립트
│   │   ├── gitlab-argocd-v1/
│   │   └── github-argocd-v1/
│   ├── pipelines/
│   │   ├── web-backend-v1/
│   │   ├── web-frontend-v1/
│   │   └── batch-job-v1/
│   ├── compatibility/
│   │   └── matrix.yaml          # 호환성 매트릭스
│   └── known-issues/
│       └── known-issues.yaml     # Helm edge case 패턴 DB (Narwhal 70+ 패턴 시드)
│
├── docs/                        # 사용자 문서
│   ├── quickstart.md
│   ├── architecture.md
│   ├── user-guide/
│   └── contributing.md
│
├── scripts/                     # 유틸리티 스크립트
│   ├── dev-setup.sh
│   └── seed-data.sh
│
├── docker-compose.yaml          # 로컬 개발 환경
├── Dockerfile                   # API 서버 이미지
├── Dockerfile.web               # 웹 UI 이미지
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

### 기술 의사결정 기록 (ADR 요약)

| ID | 결정 | 선택 | 근거 |
|---|---|---|---|
| ADR-001 | 웹 프레임워크 | React + TypeScript | 생태계 최대, Backstage 플러그인 전환 가능성 |
| ADR-002 | 백엔드 언어 | Go 1.24+ | K8s client-go 네이티브, 단일 바이너리, 크로스 컴파일 |
| ADR-003 | 데이터베이스 | PostgreSQL 18+ | JSONB 지원, 확장성, 향후 pgvector 활용 |
| ADR-004 | 실시간 통신 | WebSocket | 설치 로그 양방향 스트리밍, 재연결 처리 용이 |
| ADR-005 | 설치 엔진 | Helm Go SDK | K8s 생태계 표준, 프로그래밍 제어, 롤백 내장 |
| ADR-006 | 인증 (Alpha~Beta) | 세션 기반 | 빠른 구현, 단일 Admin 모드 충분 |
| ADR-007 | 인증 (v1) | Keycloak OIDC | SSO, RBAC, OSS 권한 매핑 통합 |
| ADR-008 | 모노레포 | 단일 리포지토리 | FE/BE 동시 변경 용이, CI 단일 파이프라인 |
| ADR-009 | 상태 관리 | Zustand | Redux 대비 보일러플레이트 90% 감소, 작은 번들 |
| ADR-010 | 스택 설정 저장 | JSONB (PostgreSQL) | 스키마 유연성, 버전별 스냅샷 저장 용이 |

---

## Part 8: 구현 동기화 업데이트 (2026-03, Stack 범위)

DevSecOps Stack 구현과 문서의 싱크를 맞추기 위해 아래 원칙을 추가한다.

- Stack Install은 `YAML View → Preview Deploy Script → Dry Run` 흐름을 기본 검토 단계로 사용한다.
- Gateway는 `Ingress` 대신 `Gateway API(Gateway + HTTPRoute)`를 기본으로 하며,
  Access Domain TLS 옵션을 통해 HTTPS listener + `tls.certificateRefs`를 반영한다.
- OSS 버전은 app/chart를 분리 고정한다 (예: GitLab `18.5.1 / 9.5.1`, Argo CD `v2.8.3 / 6.8.0`).
- 템플릿 편집 시 `helm_version`, `app_version`가 유실되지 않아야 하며, 생성 YAML/스크립트는 고정 버전을 명시한다.

### Stack 네임스페이스 모델 보정

| 구분 | 논리 모델(문서) | 실행 기본값(구현) |
|---|---|---|
| 스택 배포 단위 | `nullus-*` 영역 분리 | `nullus-stack` 중심 + 도구별 매니페스트 |
| 접근 도메인 | `{OSS}.{stack}.internal` | 동일(게이트웨이 라우팅 규칙 반영) |

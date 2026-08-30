# Nullus 상세 기능 명세 및 시스템 아키텍처

**작성일**: 2026-03-30
**최종 갱신**: 2026-08-31 (코드 기준 재대조)
**기준**: `draft` 실제 구현 및 운영 흐름
**문서 성격**: 현재 구현 기준 문서 (As-Is Baseline)
**대상 독자**: 엔지니어, 아키텍트, DevOps Engineer

---

## 문서 업데이트 원칙

- 본 문서는 `draft` 코드베이스의 현재 구현을 기준으로 서술한다.
- 설계에 있었지만 아직 구현되지 않은 항목은 본문에서 현재 기능처럼 설명하지 않는다.
- 설계 대비 미구현 목록은 `Nullus_설계_대비_미구현_항목.md`를 별도 참조한다.
- As-Is 다이어그램 원본은 `Nullus_As-Is_아키텍처_다이어그램.md`를 참조한다.

---

## Part 1: 시스템 아키텍처

### 1. 아키텍처 개요

현재 `draft`의 Nullus는 다음 성격을 가진다.

- 프런트엔드는 React/Vite 기반 SPA이며 Stack, CI/CD, Observability, Admin 화면을 제공한다.
- 백엔드는 단일 Go 바이너리로 배포되지만 내부는 `admin`, `auth`, `stack`, `cicd`, `observability`, `shared`, `cli` 모듈로 분리된 Modular Monolith 구조다. CLI·MCP 가 함께 쓰는 공유층은 `internal/` 밖의 `pkg/nullusclient` 에 있다 — 외부에서 import 해야 하기 때문이다.
- Stack 설치는 Helm 오케스트레이터 중심으로 동작하며, PostgreSQL에 설정과 이력을 저장하고 WebSocket으로 배포 로그를 스트리밍한다.
- 템플릿, 호환성, Known Issues, 리소스 기본값은 파일 카탈로그보다 DB 중심으로 관리된다.
- 설계 문서의 다중 네임스페이스 논리 모델과 달리 현재 실행 기본값은 Stack 단위 네임스페이스 중심이다.

```mermaid
flowchart LR
  Browser["사용자 브라우저"]
  Web["Nullus Web<br/>React 19 + TypeScript + Vite"]
  API["Nullus API<br/>Go + Echo"]
  PG["PostgreSQL"]
  K8S["등록된 Kubernetes 클러스터"]
  PROM["Prometheus API<br/>(선택 구성)"]

  Browser --> Web
  Web <-->|REST API| API
  Web <-->|WebSocket 로그 스트리밍| API
  API --> PG
  API -->|Helm / kubectl / K8s API| K8S
  API -->|메트릭 조회| PROM
```

### 1.1 Architecture as Code

아키텍처는 문장 설명보다 코드형 다이어그램과 구조 선언으로 관리한다.

```mermaid
flowchart TB
  subgraph ControlPlane["Control Plane"]
    API["cmd/api/main.go"]
    Admin["internal/admin"]
    Stack["internal/stack"]
    CICD["internal/cicd"]
    Obs["internal/observability"]
    Auth["internal/auth"]
    API --> Admin
    API --> Stack
    API --> CICD
    API --> Obs
    API --> Auth
  end

  subgraph DataPlane["Target Cluster"]
    Helm["Helm Releases"]
    Workloads["Stack/Pipeline Workloads"]
    Helm --> Workloads
  end

  Stack -->|Helm SDK + kubectl| Helm
```

```text
internal/
  admin/
    domain/
    usecase/
    port/
    adapter/
  stack/
    domain/
    usecase/
    port/
    adapter/
      helm/
      handler/
      repository/
  cicd/
  observability/
  auth/
  shared/
  cli/            # 통합 nullus CLI 명령 트리 (표면만, 로직은 pkg/nullusclient)

pkg/
  nullusclient/   # CLI·MCP 공유 기반 — API 클라이언트, 설정·토큰, OIDC, 버전 스큐
  crypto/         # AES-256-GCM (kubeconfig)

cmd/
  api/            # 서버
  nullus/         # 통합 CLI
  nullus-bootstrap/
  token-source-sync/
```

#### 현재 런타임 경계

1. 사용자 브라우저
2. Nullus 컨트롤 플레인 (`web` + `api` + `postgresql`)
3. 등록된 대상 Kubernetes 클러스터

#### 설계안 대비 핵심 변경점

- `Auth/Config/Installer/Monitor Handler` 중심 구조에서 모듈별 `domain/usecase/port/adapter` 구조로 정리되었다.
- 파일 기반 `matrix.yaml`, `known-issues.yaml` 중심 설계에서 DB 기반 카탈로그 구조로 이동했다.
- 대상 클러스터 내부 `Nullus Operator` 전제 대신 API 서버가 Helm/kubectl을 직접 구동하는 구조가 현재 기준이다.

---

### 2. 시스템 구성 원칙

#### 2.1 현재 아키텍처 원칙

- **모듈러 모놀리스**: 서비스 분리보다 단일 배포 단위 내 모듈 분리를 우선한다.
- **클린 아키텍처 지향**: 각 모듈은 `domain`, `usecase`, `port`, `adapter` 레이어를 가진다.
- **DB 중심 카탈로그**: 템플릿, 호환성, 리소스 기본값, Known Issues는 PostgreSQL 기준으로 관리한다.
- **비동기 배포 + 실시간 관찰**: Stack/Pipeline 배포는 서버에서 비동기로 실행하고 클라이언트는 WebSocket으로 진행 상태를 구독한다.
- **실행 가능한 문서화**: API 경로, 테이블명, 상태값은 실제 코드와 마이그레이션 기준으로 맞춘다.

#### 2.2 현재 제약 (2026-08-31 재확인)

- `auth.mode=session` 의 인증은 `X-User-*` 헤더 기반 단순화 구현이며 **로컬 개발 전용**이다.
  OIDC 와 ID/PW 두 경로는 실제로 연결돼 있다(9.1).
- **Alert Rule 과 Notification Config 는 CRUD 만 된다.** 규칙을 평가하는 루프도, notifier 를
  호출하는 프로덕션 코드도 없어 **알림이 실제로 발송되지 않는다.** 화면상으로는 설정이
  저장되므로 동작하는 것처럼 보인다 — 이 항목의 위험은 거기에 있다.
- 호환성 매트릭스의 생성·수정·삭제 API 가 `main.go` 에 등록되지 않아, 관리자 화면의
  해당 버튼 3개가 404 로 끝난다. 매트릭스 추가는 마이그레이션 시드로만 가능하다.
- 한 클러스터에 스택은 하나만 선다. OpenBao 의 ClusterRoleBinding 이 클러스터 범위라
  두 번째 스택 설치는 소유권 충돌로 막힌다(의도된 차단).

> 2026-08-11 판의 제약 두 개는 해소됐다 — 프런트 OIDC 런타임 연결(2026-08-19),
> Stack 배포 로그 DB 영속화(2026-08-21, `stack_deploy_logs`).

---

### 3. 비기능 요구사항과 현재 운영 기준

NFR 목표는 유지하되, 현재 구현이 제공하는 운영 근거를 함께 적는다.

| 항목 | 목표/기준 | 현재 구현 근거 |
|---|---|---|
| REST API 응답 | 일반 조회 API는 수백 ms 내 응답 목표 | Echo + pgx 기반, `/health` 제공, production rate limit 적용 |
| Stack 로그 전달 | Near real-time + 재기동 후에도 조회 가능 | gorilla/websocket + 인메모리 fan-out 스트리머(실시간) + `stack_deploy_logs`(영속) |
| 대시보드 조회 | 요청 시 외부 메트릭 반영 | Prometheus API 프록시 방식, DB 장기 저장 없음 |
| 배포 복구 | 실패 시 최소 safe rollback | Stack install 실패 시 rollback 단계 지원 |
| 보안 민감정보 보호 | DB 평문 저장 지양 | kubeconfig AES-256-GCM 암호화 저장 |

#### 현재 문서 해석 기준

- 이 절은 측정 결과 리포트가 아니라 운영 설계 기준이다.
- 수치형 목표는 유지하되, 실제 관측 체계가 아직 완성되지 않은 항목은 구현 근거만 적는다.

---

### 4. 기술 스택

| 계층 | 현재 기술 | 비고 |
|---|---|---|
| Frontend | React 19.2, TypeScript 5.9, Vite 8 | SPA |
| 라우팅 | React Router 7 | 역할 기반 라우팅 |
| 클라이언트 상태 | Zustand 5 | 인증/초안 상태 |
| 서버 상태 | TanStack Query 5 | API 캐시/동기화 |
| 폼/검증 | React Hook Form + Zod | 입력 검증 |
| 컴포넌트 | 자체 프리미티브(`components/ui/*`), 내부는 MUI 9 + Emotion | 공개 API 는 자체라 내부 교체가 화면에 새지 않는다 |
| 스타일/토큰 | Tailwind CSS 4, `web/DESIGN.md` → `theme/tokens.generated.*` | 색·간격 단일 출처. 색 리터럴은 ESLint 가 막는다 |
| 편집/시각화 | Monaco, `monaco-yaml`, Recharts 3 | YAML View, 차트. **Chart.js 는 2026-08-12 에 제거**됐다 — 두 라이브러리가 섞여 같은 차트가 화면마다 다르게 보였다 |
| Backend | Go 1.26, Echo v4 | 단일 API 바이너리 |
| 설정 | Viper | 런타임 설정 |
| DB 접근 | pgx v5 | PostgreSQL 연결 |
| 실시간 통신 | gorilla/websocket | Stack/Pipeline 로그 |
| K8s 연동 | Helm Go SDK, client-go, kubectl fallback | 설치/정리 |
| Database | PostgreSQL | dev compose는 PostgreSQL 17 |
| 인증 | Session-like middleware + OIDC JWT middleware | Keycloak/Authentik provider abstraction |
| API 문서 | `api/openapi.yaml` 정적 산출물 | 현재 swag 자동 생성 연결 미확인 |
| 테스트 | `testify`, Vitest, Testing Library, Playwright | FE/BE 혼합 |

#### 4.1 모듈 구성 (2026-08-31)

모듈마다 Clean Architecture 4계층(`domain` / `usecase` / `port` / `adapter`)을 유지한다.
모듈 간 호출은 포트 인터페이스를 통해서만 한다.

| 모듈 | 바운디드 컨텍스트 | 구현/테스트 파일 | 특이 계층 |
|------|------------------|------------------|-----------|
| `internal/stack` | Stack, Template, Compatibility | 116 / 130 | — |
| `internal/cicd` | Pipeline, Deployment | 69 / 56 | — |
| `internal/admin` | Organization, User, Cluster, TokenSource | 43 / 23 | `rotation`, `scheduler` |
| `internal/shared` | 공용 | 24 / 11 | `audit`, `config`, `domain`, `externalsecret`, `middleware`, `notification`, `secrets` |
| `internal/observability` | Dashboard, Alert | 23 / 12 | — |
| `internal/auth` | Session, Token | 21 / 21 | `adapter/keycloak`, `adapter/authentik`, `adapter/token` |
| `internal/cli` | (표면) | 2 / 1 | 통합 `nullus` CLI 명령 트리 |

프로덕션 Go 코드는 `cmd/`·`internal/`·`pkg/` 합쳐 약 52,900 라인이다(2026-08-11 약 38,700).
`internal/stack` 은 테스트 파일이 구현 파일보다 많다 — 설치·삭제 경로의 회귀가 곧 클러스터
사고라서, 고쳐 놓은 동작마다 테스트로 못을 박아 왔다.

**모듈이 아닌 공유 패키지**가 두 개 있다. `internal/` 밖에 있는 이유는 외부(CLI·MCP)가
import 해야 하기 때문이다.

| 패키지 | 구현/테스트 | 역할 |
|--------|------------|------|
| `pkg/nullusclient` | 6 / 5 | CLI(트랙 A)와 MCP 서버(트랙 B)가 함께 쓰는 유일한 공유층 — API 클라이언트, 설정·토큰 해석, OIDC 토큰 획득·갱신, 서버 버전 스큐 판정 |
| `pkg/crypto` | 1 / 1 | AES-256-GCM (kubeconfig 암호화) |

바이너리는 4개다 — `cmd/api`(서버), `cmd/nullus`(통합 CLI), `cmd/nullus-bootstrap`,
`cmd/token-source-sync`.

#### 4.2 외부 연동 매트릭스 (2026-08-31)

| 영역 | 지원 | 비고 |
|------|------|------|
| 소스 저장소 | GitLab CE(스택 내 설치), **Gitea(스택 내 설치)**, GitHub(외부 SaaS) | GitHub 은 Organization 을 API 로 만들 수 없어 존재 확인만 한다. Gitea 는 조직·저장소·팀을 다 만들고, 파이프라인을 만든 사람을 write 팀(`developers`)에 넣는다 |
| CI | GitLab CI, **Jenkins(스택 내 설치)**, GitHub Actions | Jenkins 는 플러그인을 이미지에 구워 런타임 설치를 끈다(부팅 중 플러그인 다운로드가 설치를 통째로 막았다). job 은 multibranch 로 프로비저닝한다. GitHub 은 호스티드 러너를 쓰므로 클러스터에 설치할 러너가 없다 |
| 컨테이너 레지스트리 | Harbor, Nexus, GitLab Container Registry, GHCR | GHCR 은 자격증명 등록이 없다 — 워크플로의 내장 토큰으로 push. Harbor 는 프로젝트 생성(`provisioning_harbor`)과 저장소 삭제까지, Nexus 는 컴포넌트 단위 삭제(단일 저장소 삭제 API 가 없다)까지 지원한다 |
| CD | Argo CD | Application 은 **CD 도구가 사는 네임스페이스**에 생성한다. 포트 이름에서 도구 이름을 걷어냈다 — `CDApplicationDeleter`, `SCMBundle.CDNamespace` |
| 오브젝트 스토리지 | MinIO | 스택 내 설치. 버킷 생성까지 |
| 시크릿 | OpenBao + External Secrets Operator | **스택마다 배포**된다. API 는 Kubernetes API server proxy 로 접근. 한 클러스터에 스택은 하나만 — OpenBao ClusterRoleBinding 이 클러스터 범위라 두 번째 설치는 소유권 충돌로 막힌다 |
| 인증(플랫폼) | Keycloak, Authentik (OIDC) + ID/PW | 차트 기본값은 `oidc`. ID/PW 는 IdP 장애 대비 우회로다 |
| 인증(스택 OSS) | Argo CD, Harbor, GitLab, Gitea, Jenkins, Grafana, MinIO → Keycloak | 스택 설치 중 `provisioning_sso` 스텝이 도구마다 OIDC 클라이언트를 만든다. **도구마다 비밀번호 로그인 경로를 남긴다** — IdP 가 죽으면 도구까지 잠기기 때문이다 |
| 모니터링 | Prometheus, Grafana, Loki / OpenSearch, Tempo / Jaeger, **OpenTelemetry Collector·Agent** | 스택이 설치한 OSS 가 자기 메트릭을 Prometheus 에 내주고, 배포되는 앱에는 스택 수집기 주소를 넣어 준다 |
| 게이트웨이 | Envoy Gateway (Gateway API) | 스택 네임스페이스에 선다. 8.3 참고 |

`internal/cicd/port/SCMPlatform` 이 GitLab/GitHub/Gitea 분기의 경계다. 저장소 생성·파이프라인
정의 형식·토큰 획득 경로가 플랫폼마다 달라서, 어댑터를 고른 뒤에도 스캐폴딩 렌더러까지
이 값이 따라간다. 렌더러는 **SCM 축과 CI 축을 분리**한다 — Gitea + Jenkins 조합에서는
`.gitlab-ci.yml` 이 아니라 `Jenkinsfile` 이 나가야 하는데, 두 축이 한 값에 묶여 있으면
표현할 수 없다.

**어떤 OSS 가 모니터링·워크로드 화면에 뜨는지는 `domain.InstalledToolWorkloads()` 하나가
정한다.** 여기에 등록되지 않은 도구는 파드가 멀쩡히 떠 있어도 어느 화면에도 나오지 않는다
— 과거 소스 저장소가 `gitlab` 으로 하드코딩돼 있어 Gitea 를 고르면 "0 파드" 경고만 났다.

---

### 5. 컴포넌트 상세

#### 5.1 Web UI

프런트엔드는 단일 SPA이며 라우트는 `web/src/app/routes.tsx` 하나에 모여 있다. 화면
컴포넌트는 28개, 라우트 항목은 32개다(별칭 3쌍 + 404 캐치올 포함). 모두 lazy 로드된다.

역할 가드는 라우트에 걸린다 — `ProtectedRoute` 가 `allowedRoles` 로 한 번, 사이드바가
같은 역할값으로 한 번 더 거른다.

| 가드 | 라우트 |
|------|--------|
| 없음 | `/login`, `*`(404) |
| 인증만 | `/`, `/stack/templates`, `/stack/list`, `/stack/logs/:deploymentId`, `/stack/deployments/:deploymentId/retry-history`, `/stack/history/:stackId?`, `/stack/version`(≡`/stack/versions`), `/observability/monitoring`, `/observability/alerts`(≡`/observability/alert-rules`), `/observability/alert-history` |
| admin·devops | `/stack/install`, `/stack/:id/add-tools`, `/stack/deploy/:id`, `/stack/oss-resource-default` |
| admin·devops·developer | `/cicd/developer-deploy`, `/cicd/templates`, `/cicd/create`, `/cicd/golden-paths`, `/cicd/list`, `/cicd/history`, `/cicd/pipelines/:id/logs` |
| admin | `/admin/organization`(≡`/admin/organizations`), `/admin/users`, `/admin/clusters`, `/admin/known-issues`, `/admin/token-management`, `/admin/stack-versions` |

로그인 후 이동할 곳은 `web/src/features/auth/role-landing.ts` 한 곳에서 나온다 —
admin `/admin/organization`, devops `/stack/templates`, developer `/cicd/developer-deploy`.
로그인 리다이렉트와 홈의 시작 버튼이 각자 목록을 들고 있어 developer 만 서로 다른 곳을
가리키던 문제를 그렇게 없앴다.

메뉴 노출과 역할 대조는 `docs/10_제품기획/Nullus_메뉴체계.md` 0장이 단일 출처다.

#### 5.2 API Server

현재 API는 `/api/v1` 아래 4개 그룹과 2개의 WebSocket 엔드포인트로 정리된다.

```text
Nullus API Server
├── GET  /health
├── /api/v1/admin
│   ├── GET    /organization
│   ├── PATCH  /organization
│   ├── POST   /orgs
│   ├── GET    /users/search
│   ├── GET    /organizations/:orgId/members
│   ├── POST   /organizations/:orgId/members
│   ├── PATCH  /organizations/:orgId/members/:memberId
│   ├── DELETE /organizations/:orgId/members/:memberId
│   ├── POST   /organizations/:orgId/members/:memberId/deactivate
│   ├── GET    /organizations/:orgId/invites
│   ├── POST   /organizations/:orgId/invites
│   ├── DELETE /organizations/:orgId/invites/:token
│   ├── POST   /clusters
│   ├── GET    /clusters
│   ├── GET    /clusters/:id
│   ├── GET    /clusters/:id/namespaces
│   ├── PATCH  /clusters/:id
│   ├── DELETE /clusters/:id
│   ├── POST   /clusters/:id/verify
│   ├── GET    /known-issues
│   ├── GET    /audit-logs
│   ├── GET    /notifications/configs
│   ├── POST   /notifications/configs
│   ├── DELETE /notifications/configs/:id
│   └── GET    /notifications/history
├── /api/v1/stacks
│   ├── POST   /
│   ├── GET    /
│   ├── GET    /:stackId
│   ├── DELETE /:stackId
│   ├── PATCH  /:stackId/tools
│   ├── POST   /:stackId/config
│   ├── POST   /draft
│   ├── GET    /templates
│   ├── GET    /templates/:id
│   ├── POST   /templates
│   ├── PUT    /templates/:id
│   ├── DELETE /templates/:id
│   ├── GET    /compatibility
│   ├── POST   /:stackId/validate
│   ├── POST   /estimate
│   ├── GET    /resource-defaults
│   ├── POST   /resource-defaults
│   ├── GET    /:stackId/history
│   ├── GET    /:id/history/diff
│   ├── GET    /:stackId/diff
│   ├── POST   /:stackId/rollback
│   ├── GET    /:stackId/monitoring
│   ├── POST   /:id/deploy
│   ├── GET    /:id/status
│   ├── GET    /:id/deploy/logs
│   └── GET    /:id/export?format=json|yaml
├── /api/v1/cicd
│   ├── GET    /templates
│   ├── GET    /templates/:id
│   ├── POST   /templates
│   ├── PUT    /templates/:id
│   ├── DELETE /templates/:id
│   ├── GET    /pipelines
│   ├── POST   /pipelines
│   ├── POST   /pipelines/:id/deploy
│   ├── GET    /deployments
│   ├── GET    /deployments/:id
│   ├── GET    /app-templates
│   └── POST   /deploy-app
├── /api/v1/observability
│   ├── GET    /dashboard
│   ├── GET    /alert-rules
│   ├── GET    /alert-rules/:id
│   ├── POST   /alert-rules
│   ├── PATCH  /alert-rules/:id
│   ├── DELETE /alert-rules/:id
│   └── GET    /alert-history
├── GET /ws/deployments/:id/logs
└── GET /ws/cicd/deployments/:id/logs
```

#### 5.3 인증 모드별 라우팅 특성

- development 모드: 인증 미들웨어를 끄고 모든 그룹을 바로 연다.
- production 모드:
  - `admin`: `admin` 전용
  - `stacks`: `admin`, `devops`
  - `cicd`: `admin`, `devops`, `developer`
  - `observability`: 인증된 사용자 전체

`/api/v1/auth/*`는 현재 별도 REST 그룹으로 존재하지 않는다.

#### 5.4 백엔드 모듈 구조

```mermaid
flowchart TB
  Main["cmd/api/main.go<br/>DI 조립 + Echo 서버"]

  subgraph Modules["내부 모듈"]
    direction LR
    Admin["admin"]
    Auth["auth"]
    Stack["stack"]
    CICD["cicd"]
    Obs["observability"]
    Shared["shared"]
  end

  subgraph Layers["모듈 내부 레이어"]
    direction LR
    Handler["adapter/handler"]
    Usecase["usecase"]
    Domain["domain"]
    Port["port"]
    Infra["adapter/repository + adapter/external"]
  end

  Main --> Admin
  Main --> Auth
  Main --> Stack
  Main --> CICD
  Main --> Obs
  Main --> Shared

  Handler --> Usecase
  Usecase --> Domain
  Usecase --> Port
  Infra --> Port
```

---

### 6. Install Engine

#### 6.1 현재 상태 머신

현재 Stack 배포 상태는 아래 값을 사용한다.

- `pending`
- `validating`
- `installing`
- `configuring`
- `health_check`
- `completed`
- `cancelled`
- `failed`
- `rolling_back`
- `rolled_back`

```mermaid
flowchart TB
  Start["POST /api/v1/stacks/:id/deploy"]
  Validate["validating"]
  Install["installing"]
  Config["configuring"]
  Health["health_check"]
  Done["completed"]
  Fail["failed"]
  Roll["rolling_back"]
  Rolled["rolled_back"]

  Start --> Validate --> Install --> Config --> Health --> Done
  Install --> Fail
  Config --> Fail
  Health --> Fail
  Fail --> Roll --> Rolled
```

#### 6.2 현재 설치 단계 (2026-08-31)

현재 구현은 설계 문서의 DAG 엔진보다 고정된 step order에 가깝다. **순서의 단일 출처는
`internal/stack/domain/deploy_steps.go` 의 `InstallStepOrder` 이며, 진행률도 여기서 나온다.**
예전에는 화면(단계 5개)과 서버(손으로 적은 스텝→퍼센트 표)가 각자 값을 들고 있어, 표에
없는 스텝에서는 진행률이 0 으로 나오고 화면이 다른 근거로 값을 지어냈다.

스텝은 **31개**이며 순서대로 다음과 같다.

| 구간 | 스텝 | 비고 |
|------|------|------|
| 기반 | `installing_cert_manager`, `installing_prometheus_crds`, `installing_metrics_server`, `installing_openbao`, `installing_external_secrets`, `provisioning_secrets` | Prometheus Operator CRD 를 먼저 깔지 않으면 뒤의 설치가 깨진다. metrics-server 는 스택 네임스페이스가 아니라 클러스터에 깐다 |
| 데이터 | `installing_postgresql`, `installing_minio`, `installing_object_storage_secret`, `installing_object_storage_buckets`, `installing_database_connection_check` | |
| SSO | `provisioning_sso` | 도구별 Keycloak OIDC 클라이언트를 만든다. 이 스텝이 없던 시절에는 도구는 SSO 로 설정되는데 클라이언트를 아무도 만들지 않았다 |
| SCM·레지스트리 | `installing_gitlab`, `installing_gitea`, `provisioning_gitea`, `installing_harbor`, `provisioning_harbor`, `installing_nexus`, `provisioning_nexus` | `provisioning_*` 은 설치 뒤 조직·프로젝트·토큰을 만드는 단계다 |
| CI/CD | `installing_argocd`, `installing_runner`, `installing_jenkins` | |
| 관측 | `installing_prometheus`, `installing_grafana`, `installing_logging`, `installing_log_search`, `installing_opentelemetry`, `installing_otel_collector`, `installing_otel_agent` | |
| 마무리 | `installing_gateway`, `integration_check` | |

설치 스텝 바깥의 구간은 진행률이 고정값이다 — `validate`/`continue` 5%, 설치 구간
5~90%(스텝들이 균등하게 나눠 갖는다), `configuring` 93%, `health_check` 96%,
`completed` 100%. 삭제도 같은 방식이다(`deleting_started` 5% → `deleting_release` 45%
→ `deleting_manifest` 75% → `deleted` 100%).

선택하지 않은 도구의 스텝은 건너뛴다 — 목록은 가능한 최대 경로다.

#### 6.3 현재 동작 방식

- 배포 시작은 HTTP 요청으로 받고 내부 goroutine에서 비동기로 진행한다.
- 진행 이벤트는 WebSocket과 HTTP 로그 스트림으로 전달된다.
- Compatibility는 `compatibility_matrices`를 조회한다.
- Known Issues는 `known_issues` 테이블을 조회한다.
- 배포 실패 시 rollback 단계가 실행된다.
- **로그는 DB(`stack_deploy_logs`)에 남는다.** `PersistentStreamer` 가 메모리 스트리머를
  감싸고 그 뒤에 저장소를 둔다 — 실시간 구독은 여전히 메모리 계층의 몫이고, DB 는
  재기동을 견디는 몫만 맡는다. 설치가 20~30분짜리라 그 사이 파드가 재시작되면 예전에는
  로그가 통째로 사라졌다(2026-08-21 운영에서 실제로 그렇게 됐다).
- **서버 재시작으로 끊긴 설치는 `installing` 에 갇히지 않는다.** 기동 시 고아 배포를
  회수한다.
- 설치 규모는 서버가 계산한다. 템플릿마다 `planning_profile`(`local`/`startup`/
  `standard`/`enterprise`)이 있어 "무엇을 깔지" 와 "얼마나 크게 깔지" 가 함께 정해진다.

#### 6.4 현재 Stack 설치 UX 보강 요소

프런트 Stack 설치 화면은 설계 초기안보다 다음 요소가 강화되어 있다.

- YAML View
- Preview Deploy Script
- Dry Run 스타일 체크리스트
- `access_domain` 및 TLS 입력
- `Gateway API` 기반 Gateway/HTTPRoute 미리보기
- `storage.plan_mode` 기반 스토리지 생성/연결 입력
- `resource-defaults` 기반 OSS 리소스 기본값 조정

---

### 7. 데이터 모델

#### 7.1 핵심 ERD

```mermaid
erDiagram
  organizations ||--o{ org_members : has
  users ||--o{ org_members : joins
  organizations ||--o{ clusters : owns
  organizations ||--o{ stacks : owns
  clusters ||--o{ stacks : targets
  stacks ||--o{ stack_config_versions : versions
  golden_path_templates ||--o{ stacks : seeds
  pipelines ||--o{ pipeline_deployments : deployments
  pipeline_templates ||--o{ pipelines : defines
  alert_rules ||--o{ alerts : fires

  organizations {
    uuid id
    string name
    string slug
    string status
  }
  users {
    uuid id
    string email
    string role
    bool is_active
  }
  clusters {
    uuid id
    string org_id
    string type
    string connection_status
  }
  stacks {
    string id
    string template_id
    string org_id
    string cluster_id
    string state
    jsonb config
    timestamptz deleted_at
  }
  stack_config_versions {
    string id
    string stack_id
    int version
    jsonb config
  }
  golden_path_templates {
    string id
    string name
    jsonb tools
  }
  compatibility_matrices {
    string id
    string status
    jsonb tools
  }
  stack_resource_defaults {
    string tool_key
    decimal cpu_request
    decimal memory_request_gi
    decimal storage_request_gi
  }
  pipelines {
    string id
    string template_id
    string namespace
    string status
  }
  pipeline_deployments {
    string id
    string pipeline_id
    string status
    string version
  }
  pipeline_templates {
    string id
    string app_type
    jsonb stages
  }
  alert_rules {
    string id
    string name
    string channel
    bool enabled
  }
  alerts {
    string id
    string rule_id
    string severity
  }
```

#### 7.2 현재 저장 구조 요약

| 영역 | 테이블 | 비고 |
|---|---|---|
| 조직/사용자 | `organizations`, `users`, `org_members` | 조직 및 멤버 관리 |
| 클러스터 | `clusters` | 등록 대상 클러스터 |
| Stack | `stacks`(soft delete), `stack_config_versions` | 본문 설정 + 버전 이력(삭제 후 보존) |
| Stack 카탈로그 | `golden_path_templates`, `compatibility_matrices`, `stack_resource_defaults`, `stack_helm_step_configs`, `known_issues` | DB 중심 카탈로그 |
| CI/CD | `pipeline_templates`, `pipelines`, `pipeline_deployments` | 파이프라인 정의/실행 |
| Observability | `alert_rules`, `alerts` | 규칙/이력 |
| 운영 로그 | `audit_logs` | 감사 로그 |
| 알림 | `notification_configs`, `notification_history` | 알림 채널 및 발송 이력 |

#### 7.3 JSONB 중심 필드

- `stacks.config`
- `stack_config_versions.config`
- `golden_path_templates.tools`
- `compatibility_matrices.tools`
- `stack_helm_step_configs`의 `version`, `repo_url`, `namespace`
- `notification_configs.config`

#### 7.4 StackConfig 현재 구조

```json
{
  "access_domain": "platform.example.internal",
  "access_domain_tls": {
    "enabled": true,
    "secret_name": "wildcard-platform-tls",
    "secret_namespace": "nullus"
  },
  "yaml_overrides": {},
  "artifacts": {},
  "pipeline": {},
  "monitoring": {},
  "logging": {},
  "resources": {
    "developers": 20,
    "concurrent_runners": 5,
    "weekly_commits": 100,
    "build_frequency": "hourly"
  },
  "storage": {
    "plan_mode": "integrated-create",
    "database": {},
    "object_storage": {}
  }
}
```

---

### 8. 배포 아키텍처

#### 8.1 Nullus 컨트롤 플레인 배포

현재 Helm chart는 `nullus-api`, `nullus-web`, 선택적 `postgresql` subchart를 중심으로 배포된다.

```mermaid
flowchart TB
  subgraph HelmRelease["Helm release: nullus"]
    API["Deployment: nullus-api<br/>2 replicas"]
    WEB["Deployment: nullus-web<br/>2 replicas"]
    CFG["ConfigMap / Secret"]
    SVC["Service(api/web)"]
    ING["Ingress(optional)"]
    PG["PostgreSQL subchart(optional)"]
  end

  CFG --> API
  CFG --> WEB
  API --> SVC
  WEB --> SVC
  ING --> SVC
  API --> PG
```

#### 8.2 로컬 개발 배포

```mermaid
flowchart LR
  API["go run ./cmd/api"]
  WEB["npm run dev"]
  PG["postgres:17"]
  MINIO["minio"]
  REDIS["redis"]
  KC["keycloak 26"]

  WEB --> API
  API --> PG
  API --> MINIO
  API --> KC
```

#### 8.3 대상 클러스터 배포 기준 (2026-08-31 개정)

> **2026-08-20 개정.** 예전 기본 namespace 는 `nullus` 하나였고, 그것은 **플랫폼 자신이
> 사는 namespace 와 같았다.** 두 가지가 동시에 깨졌다 — 설치는 스택의
> `nullus-postgresql` 릴리스가 플랫폼 차트 소유의 같은 이름 리소스와 부딪혀 Helm 이
> 거부했고, 삭제는 스택 정리가 같은 namespace 를 훑으며 플랫폼 리소스를 지웠다.
> 2026-08-20 에 실제로 운영 도메인이 통째로 내려갔다.

- Stack 은 **자기 namespace 를 갖는다** — 스택 이름에서 파생한 `nullus-<slug>`
  (`domain.DefaultStackNamespaceFor`). RFC1123 라벨로 정규화하고 63자에서 자른다.
- 스택 삭제는 자기 namespace 안에서만 동작한다. 플랫폼 namespace 는 훑지 않는다.
- 삭제 시 PVC·Gateway·Envoy 파드·Keycloak OIDC 클라이언트를 함께 회수한다. 남으면 다음
  설치가 옛 비밀번호를 가진 볼륨을 만나 깨진다.
- 프런트는 신규 namespace 생성과 기존 namespace 선택을 모두 지원한다.

**게이트웨이는 스택 namespace 에 선다.** 2026-08-20 에 클러스터 공용(`nullus-gateway`)
으로 옮겼다가 같은 날 되돌렸다 — 설치 마법사가 자기 Gateway 매니페스트를
`yamlOverrides` 로 보내고 서버는 override 가 있으면 자기 번들을 만들지 않으므로,
UI 설치 경로에서는 공용 게이트웨이가 한 번도 적용되지 않았다. 스택별로 정리한 근거는
세 가지다.

- namespace 가 경계 그대로다 — 삭제가 namespace 통째로 끝난다.
- 인증서가 스택별이다 — 공용일 때는 마지막 설치의 것이 남는 한계가 있었다.
- 마법사와 서버가 같은 이야기를 한다.

밖에서 들어오는 배선은 **브리지 Ingress**(`nullus-gateway-bridge`)가 맡고, 이것도 스택
namespace 에 둔다(Ingress 백엔드는 같은 namespace 여야 한다는 제약이 저절로 풀린다).
Envoy Gateway 가 만드는 데이터플레인 Service 이름에는 해시가 붙어 차트가 미리 적을 수
없으므로, 설치기가 실제로 생긴 것을 조회해서 가리킨다.

즉 현재 As-Is는 "서비스별 다중 namespace 설치"가 아니라 **"Stack 하나 = namespace 하나"**다.

---

### 9. 보안 아키텍처

```mermaid
flowchart LR
  FE["Web SPA"]
  API["API Server"]
  Session["Header-based session-like auth"]
  OIDC["OIDC JWT middleware<br/>Keycloak / Authentik"]
  DB["PostgreSQL kubeconfig(bytea)"]
  Crypto["AES-256-GCM"]

  FE --> API
  Session --> API
  OIDC --> API
  API --> DB
  Crypto --> API
```

#### 9.1 현재 인증 구조 (2026-08-31 개정)

- backend
  - `DualAuthMiddleware` 가 **OIDC JWT 와 로컬 발급 토큰을 함께** 받는다
  - OIDC 모드: JWT middleware + provider abstraction. 지원 provider 는 Keycloak 기본, Authentik 선택
  - **ID/PW 로그인**: `POST /api/v1/auth/login` → bcrypt 검증(`users.password_hash`) →
    HS256 로컬 토큰 발급. `auth.session.secret` 이 비면 라우트 자체가 등록되지 않는다.
    쿠키를 쓰지 않는다 — 토큰을 응답 본문으로 주고 클라이언트가 `Authorization` 헤더로 싣는다
  - session 모드: `X-User-*` 헤더를 그대로 믿는 단순화 구현. **로컬 개발 전용**이며,
    클라이언트가 `X-User-Role: admin` 을 붙이면 관리자가 된다
  - `server.mode=development` 에서는 인증 미들웨어가 아예 붙지 않는다(설계이지 우회가 아니다)
  - WebSocket 은 서브프로토콜로 온 토큰을 헤더로 옮겨 평소 인증을 태운다.
    `auth.mode=oidc` 일 때만 보호된다 — session 모드는 자격을 실을 방법이 없어 켜면 기능만 죽는다
- frontend
  - OIDC 리디렉션·콜백이 실제로 연결돼 있고(`oidc-client-ts`, `react-oidc-context`),
    로그인 화면은 **OIDC 와 ID/PW 두 경로를 나란히** 제공한다
  - 로그인 후 랜딩은 `features/auth/role-landing.ts` 한 곳에서 나온다
  - 로컬 기본값은 `VITE_AUTH_MODE=mock`(`web/.env.example`), 배포 차트 기본값은 `oidc` 다
- **스택이 설치한 OSS 도 같은 IdP 를 본다.** `provisioning_sso` 스텝이 Argo CD·Harbor·
  GitLab·Gitea·Jenkins·Grafana·MinIO 의 OIDC 클라이언트를 만든다. 도구마다 비밀번호
  로그인 경로(escape hatch)를 남긴다 — 남기지 않으면 IdP 장애가 도구까지 잠근다.
  MinIO 는 policy 클레임을 요구하므로 토큰에 함께 싣는다

#### 9.2 권한 모델

| 그룹 | 현재 접근 범위 |
|---|---|
| `admin` | 모든 그룹 접근 |
| `devops` | `stacks`, `cicd`, `observability` 중심 |
| `developer` | `cicd`, `observability` 중심 |

#### 9.3 민감정보 보호

- kubeconfig는 AES-256-GCM으로 암호화해 DB에 저장한다.
- 복호화는 실제 K8s 호출 시점에 메모리에서 수행한다.
- production 모드에서는 rate limiter가 활성화된다.

---

### 10. 운영 및 마이그레이션 전략

#### 10.1 DB 마이그레이션

- `golang-migrate` 기반 SQL 마이그레이션
- 초기 상태 enum 보정, 리소스 기본값, 템플릿 버전 정렬 등 후속 마이그레이션이 누적되어 있다
- Stack 상태 enum은 후속 마이그레이션으로 `healthcheck` → `health_check`, `cancelled` 추가가 반영되었다

#### 10.2 API 버전 정책

- 현재 공용 API prefix는 `/api/v1`
- 실시간 로그 채널은 `/ws/*` — `/api/v1` 아래가 아니라 **루트에 붙는다**
- 설계 문서에 있던 `/monitoring`, `/compatibility` 독립 그룹은 각각 `observability`,
  `stacks` 그룹 내부로 재배치되었다
- `/auth` 는 **다시 생겼다** — `POST /api/v1/auth/login` 하나. 인증 미들웨어 앞에 붙고
  로그인 전용 레이트리밋을 단다. `logout`·`me`·`token/refresh` 는 만들지 않았다
  (사유는 `Nullus_설계_대비_미구현_항목.md` 3.1 A)
- 서비스되는 라우트는 **116개**다. 전체 목록은 `Nullus_API_설계.md` 6.0 이 단일 출처다

#### 10.3 운영 진단 포인트

- `/health` 제공 — DB ping 결과와 버전을 함께 돌려준다. CLI·MCP 는 이 버전으로 서버
  버전 스큐를 판정한다(`pkg/nullusclient/version.go`)
- 배포 로그: HTTP 스트림 + WebSocket + **DB 영속화**(`stack_deploy_logs`)
- 감사 로그: `audit_logs`
- 알림 설정 및 이력: `notification_configs`, `notification_history`.
  **다만 발송 경로는 아직 배선되지 않았다** — 규칙을 평가하는 루프도, notifier 를 만드는
  프로덕션 코드도 없다. 상세는 `Nullus_설계_대비_미구현_항목.md` 3.5 B
- `/debug/pprof/*` 는 `server.mode=development` 에서만 등록된다

---

## Part 2: 상세 기능 명세

### 기능 0: Organization 및 Admin 영역

**목적**: 조직 메타데이터, 멤버, 클러스터, Known Issues, 알림 설정, 감사 로그를 관리한다.

#### 현재 구현 범위

- 단일 organization 조회/수정
- organization 생성 호환 엔드포인트
- 조직 멤버 CRUD 및 비활성화
- 사용자 이메일 검색
- 클러스터 등록/수정/삭제/검증/namespace 조회
- Known Issues 조회
- Notification Config CRUD 및 발송 이력 조회
- Audit Log 조회

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/admin/organization` | 조직 조회 |
| PATCH | `/api/v1/admin/organization` | 조직 수정 |
| POST | `/api/v1/admin/orgs` | 조직 생성 호환 엔드포인트 |
| GET | `/api/v1/admin/users/search` | 이메일 기반 사용자 검색 |
| GET | `/api/v1/admin/organizations/:orgId/members` | 멤버 목록 |
| POST | `/api/v1/admin/organizations/:orgId/members` | 멤버 생성/초대 |
| PATCH | `/api/v1/admin/organizations/:orgId/members/:memberId` | 멤버 수정 |
| DELETE | `/api/v1/admin/organizations/:orgId/members/:memberId` | 멤버 제거 |
| POST | `/api/v1/admin/organizations/:orgId/members/:memberId/deactivate` | 멤버 비활성화 |
| POST | `/api/v1/admin/clusters/:id/verify` | 클러스터 연결 검증 |
| GET | `/api/v1/admin/known-issues` | Known Issues 조회 |
| GET | `/api/v1/admin/audit-logs` | 감사 로그 조회 |
| GET | `/api/v1/admin/notifications/configs` | 알림 설정 목록 |
| POST | `/api/v1/admin/notifications/configs` | 알림 설정 생성 |
| GET | `/api/v1/admin/notifications/history` | 알림 이력 조회 |

#### 현재 데이터 모델

- `organizations`
- `users`
- `org_members`
- `clusters`
- `audit_logs`
- `notification_configs`
- `notification_history`
- `known_issues`

#### 향후 확장

- 상위 `/api/v1/users` 전역 RBAC 관리 API
- 실제 초대 토큰 발급/만료/수락 플로우
- 조직 상태 전환과 접근 제어 강화

---

### 기능 1: Kubernetes Cluster 등록 및 관리

**목적**: Stack 및 Pipeline이 사용할 대상 Kubernetes 클러스터를 등록하고 검증한다.

#### 현재 구현 범위

- cluster 등록
- 목록/상세 조회
- 수정/삭제
- kubeconfig 기반 연결 검증
- namespace 목록 조회

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| POST | `/api/v1/admin/clusters` | 클러스터 등록 |
| GET | `/api/v1/admin/clusters` | 클러스터 목록 |
| GET | `/api/v1/admin/clusters/:id` | 클러스터 상세 |
| PATCH | `/api/v1/admin/clusters/:id` | 클러스터 수정 |
| DELETE | `/api/v1/admin/clusters/:id` | 클러스터 삭제 |
| POST | `/api/v1/admin/clusters/:id/verify` | 연결 검증 |
| GET | `/api/v1/admin/clusters/:id/namespaces` | namespace 목록 |

#### 현재 구현 포인트

- cluster type은 `pipeline`, `target`
- connection status는 `connected`, `pending`, `unreachable`, `auth_failed`
- kubeconfig는 암호화 저장 후 검증 시 복호화한다

#### 향후 확장

- cluster 분류별 정책 템플릿
- 검증 결과 캐시/주기 동기화 고도화

---

### 기능 2: Stack 설계 및 노코드 구성 UI

**목적**: 사용자가 DevSecOps Stack을 코드 없이 설계하고 검토할 수 있게 한다.

#### 현재 구현 범위

- Stack 생성 및 목록/상세 조회
- 드래프트 저장
- 도구 추가
- 설정 저장
- YAML View
- Preview Deploy Script
- Dry Run 스타일 검토 체크리스트
- Access Domain 및 TLS 설정
- Storage plan mode 설정
- 리소스 예상량 표시

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| POST | `/api/v1/stacks` | Stack 생성 |
| GET | `/api/v1/stacks` | Stack 목록 |
| GET | `/api/v1/stacks/:stackId` | Stack 상세 |
| DELETE | `/api/v1/stacks/:stackId` | Stack 삭제 |
| PATCH | `/api/v1/stacks/:stackId/tools` | 도구 추가 |
| POST | `/api/v1/stacks/:stackId/config` | 설정 저장 |
| POST | `/api/v1/stacks/draft` | 드래프트 저장 |

#### 현재 설정 모델 핵심 필드

- `access_domain`
- `access_domain_tls.enabled`
- `access_domain_tls.secret_name`
- `access_domain_tls.secret_namespace`
- `yaml_overrides`
- `resources.developers`
- `resources.concurrent_runners`
- `resources.weekly_commits`
- `resources.build_frequency`
- `storage.plan_mode`
- `storage.database`
- `storage.object_storage`

#### 향후 확장

- 서버 측 정식 dry-run endpoint
- YAML 편집 정책과 배포 승인 플로우의 분리

---

### 기능 3: Golden Path 템플릿 및 호환성 관리

**목적**: 검증된 Stack 조합과 버전 호환성 정보를 제공한다.

#### 현재 구현 범위

- Golden Path 템플릿 CRUD
- Compatibility matrix 조회
- Stack 기준 조합 검증
- DB 기반 버전 메타데이터 관리

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/stacks/templates` | 템플릿 목록 |
| GET | `/api/v1/stacks/templates/:id` | 템플릿 상세 |
| POST | `/api/v1/stacks/templates` | 템플릿 생성 |
| PUT | `/api/v1/stacks/templates/:id` | 템플릿 수정 |
| DELETE | `/api/v1/stacks/templates/:id` | 템플릿 삭제 |
| GET | `/api/v1/stacks/compatibility` | 호환성 매트릭스 조회 |
| POST | `/api/v1/stacks/:stackId/validate` | Stack 조합 검증 |

#### 현재 데이터 모델

- `golden_path_templates`
- `compatibility_matrices`

#### 현재 구현 포인트

- 템플릿의 `tools`는 `category`, `name`, `helm_version`, `app_version`를 가진다
- 호환성 정보는 `compatibility_matrices.tools JSONB`에 저장된다
- 설계 초기안과 달리 YAML 카탈로그 파일이 아니라 DB를 기준으로 운영한다

#### 향후 확장

- 검증 결과의 추천 버전 자동 반영
- Known Issues와 호환성 경고의 통합 UI

---

### 기능 4: DevSecOps Stack 설치, 배포, 이력 관리

**목적**: 선택한 Stack 구성을 대상 클러스터에 설치하고 진행 상태와 이력을 관리한다.

#### 현재 구현 범위

- Stack 배포 시작
- 상태 조회
- 로그 스트리밍
- 버전 이력 조회
- 버전 diff 조회
- 설정 rollback
- Stack monitoring 조회
- 설정 export
- 삭제 시 best-effort 정리

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| POST | `/api/v1/stacks/:id/deploy` | 배포 시작 |
| GET | `/api/v1/stacks/:id/status` | 현재 상태 조회 |
| GET | `/api/v1/stacks/:id/deploy/logs` | HTTP 로그 스트림 |
| GET | `/ws/deployments/:id/logs` | WebSocket 로그 스트림 |
| GET | `/api/v1/stacks/:stackId/history` | 버전 이력 조회 |
| GET | `/api/v1/stacks/:id/history/diff` | 버전 간 diff |
| GET | `/api/v1/stacks/:stackId/diff` | 현재 대비 diff |
| POST | `/api/v1/stacks/:stackId/rollback` | 버전 rollback |
| GET | `/api/v1/stacks/:stackId/monitoring` | Stack 모니터링 정보 |
| GET | `/api/v1/stacks/:id/export?format=json|yaml` | 설정 내보내기 |

#### 현재 이력 모델

- 현재 설정: `stacks`
- 버전 이력: `stack_config_versions`
- 배포 로그: 인메모리 스트리머

#### 삭제 정책

- `DELETE /api/v1/stacks/:id`는 `stacks.deleted_at`을 설정하는 soft delete로 동작한다.
- soft delete된 스택은 목록/조회에서 제외되며, `stack_config_versions` 이력은 보존된다.

#### 현재 제약

- `deployments`, `deployment_logs` 테이블은 없다
- 재시도 전용 endpoint는 없다
- `partial_success`, `retrying`, `timeout` 상태는 없다

#### 향후 확장

- 배포 실행 이력 분리 저장
- 실패 단계 재시도 API
- 로그 영속화

---

### 기능 5: CI/CD 템플릿 카탈로그

**목적**: 애플리케이션 배포용 파이프라인 템플릿을 관리한다.

#### 현재 구현 범위

- CI/CD 템플릿 CRUD
- 앱 템플릿 목록 제공

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/cicd/templates` | 템플릿 목록 |
| GET | `/api/v1/cicd/templates/:id` | 템플릿 상세 |
| POST | `/api/v1/cicd/templates` | 템플릿 생성 |
| PUT | `/api/v1/cicd/templates/:id` | 템플릿 수정 |
| DELETE | `/api/v1/cicd/templates/:id` | 템플릿 삭제 |
| GET | `/api/v1/cicd/app-templates` | 앱 템플릿 목록 |

#### 현재 데이터 모델

- `pipeline_templates`

#### 향후 확장

- 템플릿 검증 정책 강화
- 언어/프레임워크별 더 세밀한 기본값 제공

---

### 기능 6: CI/CD Pipeline 배포 및 이력 관리

**목적**: 앱 배포용 파이프라인을 생성하고 실행 이력을 추적한다.

#### 현재 구현 범위

- pipeline 생성/목록 조회
- pipeline deploy 실행
- deployment 목록/상세 조회
- app deploy endpoint 제공
- pipeline 로그 WebSocket 스트림

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/cicd/pipelines` | 파이프라인 목록 |
| POST | `/api/v1/cicd/pipelines` | 파이프라인 생성 |
| POST | `/api/v1/cicd/pipelines/:id/deploy` | 파이프라인 배포 |
| GET | `/api/v1/cicd/deployments` | 배포 목록 |
| GET | `/api/v1/cicd/deployments/:id` | 배포 상세 |
| POST | `/api/v1/cicd/deploy-app` | 앱 배포 도우미 endpoint |
| GET | `/ws/cicd/deployments/:id/logs` | 파이프라인 로그 |

#### 현재 데이터 모델

- `pipelines`
- `pipeline_deployments`

#### 현재 구현 포인트

- app type은 `web`, `backend`, `batch`
- deployment status는 `pending`, `running`, `success`, `failed`, `rolled_back`
- 기본 namespace는 `default`

#### 향후 확장

- pipeline rollback/diff
- 생성된 K8s 오브젝트 상세 비교

---

### 기능 7: Observability 및 Alert Rule 관리

**목적**: 대시보드, Alert Rule, Alert History를 관리한다.

#### 현재 구현 범위

- Observability dashboard 조회
- Alert Rule CRUD
- Alert History 조회

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/v1/observability/dashboard` | 대시보드 조회 |
| GET | `/api/v1/observability/alert-rules` | Alert Rule 목록 |
| GET | `/api/v1/observability/alert-rules/:id` | Alert Rule 상세 |
| POST | `/api/v1/observability/alert-rules` | Alert Rule 생성 |
| PATCH | `/api/v1/observability/alert-rules/:id` | Alert Rule 수정 |
| DELETE | `/api/v1/observability/alert-rules/:id` | Alert Rule 삭제 |
| GET | `/api/v1/observability/alert-history` | Alert History 조회 |

#### 현재 데이터 모델

- `alert_rules`
- `alerts`

#### 현재 구현 포인트

- 메트릭은 Prometheus API 조회 기반이다
- 대시보드는 저장형 BI보다 요청 시 계산/조합에 가깝다
- 알림 채널 설정 테이블은 admin 영역에 분리되어 있다

#### 현재 제약

- `metrics/summary` 독립 endpoint는 없다
- Alert Rule에서 실제 Notification Config로 이어지는 자동 발송 경로는 제한적이다

#### 향후 확장

- Slack/Email notifier 운영 wiring
- 파이프라인/Stack 요약 메트릭 endpoint

---

### 기능 8: 리소스 예상량 계산 및 리소스 기본값 관리

**목적**: 설치 전에 필요한 리소스를 계산하고 OSS별 기본 요청량을 관리한다.

#### 현재 구현 범위

- Stack resource estimation
- OSS resource default 조회/수정

#### 현재 API

| Method | Path | 설명 |
|---|---|---|
| POST | `/api/v1/stacks/estimate` | 리소스 예상량 계산 |
| GET | `/api/v1/stacks/resource-defaults` | 기본값 목록 |
| POST | `/api/v1/stacks/resource-defaults` | 기본값 저장 |

#### 현재 데이터 모델

- `stack_resource_defaults`
- `stack_helm_step_configs`

#### 현재 구현 포인트

- 기본값은 CPU, Memory, Storage request/limit을 모두 가진다
- Stack 설치 화면은 이 기본값을 이용해 총량을 계산한다

#### 향후 확장

- 비용 추정 모델 정교화
- 템플릿별 baseline 추천 자동화

---

### 기능 9: 인증 및 역할 기반 접근 제어

**목적**: 사용자의 로그인 상태와 역할에 따라 UI/API 접근을 제어한다.

#### 현재 구현 범위

- mock 로그인
- header-based session-like auth
- OIDC JWT backend middleware
- 역할 기반 API 그룹 접근 제어
- 프런트 route guard

#### 현재 구현 포인트

- 테스트 계정
  - `admin@nullus.dev / admin123`
  - `devops@nullus.dev / devops123`
  - `developer@nullus.dev / developer123`
- OIDC provider abstraction은 Keycloak/Authentik을 지원한다
- 프런트 `OIDCWrapper`는 현재 no-op wrapper다

#### 현재 제약

- `/api/v1/auth/login`, `/logout`, `/me` REST 세트 미구현
- 실제 쿠키 세션 저장소 미연결
- 프런트 OIDC redirect/logout callback 미연결

#### 향후 확장

- 완전한 세션 쿠키 모드
- OIDC end-to-end 로그인/로그아웃/토큰 갱신
- 사용자 전역 관리 API

---

## Part 3: 현재 기준 ADR 요약

| ID | 결정 | 현재 선택 | 비고 |
|---|---|---|---|
| ADR-001 | 웹 앱 구조 | React SPA + feature route | 단일 Web UI |
| ADR-002 | 백엔드 구조 | Modular Monolith + Clean Architecture | 모듈별 레이어 분리 |
| ADR-003 | 설치 방식 | Helm 오케스트레이터 + kubectl fallback | Operator 미도입 |
| ADR-004 | Stack 설정 저장 | PostgreSQL JSONB + 버전 스냅샷 | `stacks`, `stack_config_versions` |
| ADR-005 | 카탈로그 저장소 | DB 중심 | 템플릿/호환성/Known Issues |
| ADR-006 | 실시간 배포 관찰 | WebSocket + HTTP stream | 로그는 인메모리 유지 |
| ADR-007 | 인증 | 단순 세션 헤더 + OIDC JWT 병행 | 프런트 OIDC는 과도기 |
| ADR-008 | 네트워크 진입 | Stack 영역은 Gateway API 중심 | Ingress보다 Gateway/HTTPRoute 비중 증가 |
| ADR-009 | 문서 기준 | As-Is 기준 문서화 | 설계안은 별도 문서로 분리 관리 |

---

## Part 4: 참조 문서

- `Nullus_시스템_아키텍처.md`
- `Nullus_설계_대비_미구현_항목.md`
- `Nullus_As-Is_아키텍처_다이어그램.md`
- `docs/20_개발가이드/Nullus_백엔드_모듈_개발_가이드.md`
- `docs/20_개발가이드/Nullus_프론트엔드_아키텍처_가이드.md`

---

## 결론

본 문서는 더 이상 "예정 아키텍처"를 기술하지 않는다.
현재 구현의 기준선은 다음과 같다.

- React SPA + Go API + PostgreSQL 컨트롤 플레인
- Modular Monolith + DB 중심 카탈로그
- Helm 오케스트레이터 기반 Stack 설치
- WebSocket 기반 배포 로그 스트리밍
- 과도기적 인증 구조와 단계적 RBAC

향후 설계 확장 논의는 반드시 이 기준선 위에서 진행한다.

# Nullus Platform

Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼

## 개요

Nullus는 DevOps 엔지니어가 검증된 CI/CD 베스트 프랙티스 조합(Golden Path)을 선택하고, 웹 UI에서 노코드로 설정한 후 한 번의 버튼 클릭으로 Kubernetes 클러스터에 전체 DevSecOps 스택을 자동 설치할 수 있도록 하는 오픈소스 플랫폼입니다.

### 핵심 가치

- **Golden Path 템플릿**: 검증된 CI/CD 도구 조합으로 선택의 어려움 제거
- **노코드 설정**: 웹 UI의 체크박스/드롭다운으로 5단계 설정 워크플로우
- **자동 설치**: 한 번의 Deploy로 전체 스택 자동 설치 및 연동
- **버전 호환성 보장**: 테스트 완료된 도구 버전 조합만 제공

## 기술 스택

- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- **Backend**: Go 1.24+ (Echo v4) + PostgreSQL 18+
- **Auth**: Keycloak OIDC + JWT + 3단계 RBAC (Admin/DevOps/Developer)
- **Infrastructure**: Docker, Docker Compose, Helm v3, Kubernetes 1.26+, kind (로컬 테스트)

## Quick Start

### 요구사항

- Docker + Docker Compose
- Go 1.24+
- Node.js 22+
- kind (K8s 로컬 테스트 시)

### 1. 인프라 기동

```bash
./scripts/runbook_local.sh up
```

Docker Compose로 PostgreSQL(:5433), Redis(:6380), MinIO(:9000/:9001), Keycloak(:8180)을 기동하고 DB 마이그레이션을 실행합니다.

### 환경변수

```bash
cp .env.example .env.dev
# .env.dev는 make run 시 자동 로드 (ENCRYPTION_KEY 포함)
```

### 샘플데이터 마이그레이션 (DevSecOps Stack 목업)

`000022_seed_devsecops_stack_mock` 마이그레이션으로 스택 목록/이력 화면 검증용 샘플 데이터를 추가할 수 있습니다.

```bash
# 1) 로컬 인프라 실행 (PostgreSQL 포함)
make dev-up

# 2) 최신 마이그레이션 전체 적용 (000022 포함)
make migrate-up

# 3) 샘플 데이터 확인
docker compose -f docker-compose.dev.yaml exec postgres \
  psql -U nullus -d nullus \
  -c "SELECT id, name, state, namespace FROM stacks WHERE id LIKE 'mock-devsecops-%' ORDER BY id;"
```

롤백이 필요하면 마지막 마이그레이션 1개를 되돌립니다.

```bash
make migrate-down
```

### 2. 백엔드 실행

```bash
make run
```

`.env.dev`에서 환경변수(`ENCRYPTION_KEY` 포함)를 자동 로드합니다.
수동 실행 시:

```bash
ENCRYPTION_KEY="nullus-dev-key-32bytes-padding!!" go run ./cmd/api
```

API 서버: `http://localhost:8090`. `ENCRYPTION_KEY`는 kubeconfig 암호화에 사용되며 32바이트 필수.

설정 파일: `configs/config.yaml`

### 3. 프론트엔드 개발 서버

```bash
cd web && npm run dev
```

Vite 개발 서버가 `http://localhost:5173`에서 실행됩니다.

### 4. K8s 테스트 클러스터 (선택)

```bash
kind create cluster --config scripts/kind-cluster.yaml
```

상세 가이드: [kind E2E 테스트 가이드](./docs/guides/kind-e2e-testing-guide.md)

## 테스트 계정

### 프론트엔드 (Mock Auth, development 모드)

| 역할 | 이메일 | 비밀번호 | 홈 페이지 |
|------|--------|----------|-----------|
| Admin | admin@nullus.dev | admin123 | /admin/organization |
| DevOps | devops@nullus.dev | devops123 | /stack/templates |
| Developer | developer@nullus.dev | developer123 | /cicd/developer-deploy |

### Keycloak OIDC (production 모드)

| 역할 | 이메일 | 비밀번호 |
|------|--------|----------|
| Admin | admin@nullus.io | nullus123! |
| DevOps | devops@nullus.io | nullus123! |
| Developer | dev@nullus.io | nullus123! |

### 인프라 서비스

| 서비스 | URL | 사용자 | 비밀번호 |
|--------|-----|--------|----------|
| PostgreSQL | localhost:5433 | nullus | nullus_dev |
| Keycloak Admin | localhost:8180 | admin | admin |
| MinIO Console | localhost:9001 | nullus | nullus_dev |
| Redis | localhost:6380 | - | - |

## 테스트

### Go 단위/통합 테스트

```bash
go test ./... -count=1
```

### React 단위 테스트 (Vitest)

```bash
cd web && npx vitest run
```

### E2E 테스트 (Playwright)

프론트엔드 개발 서버(`localhost:5173`)와 API 서버(`localhost:8090`)가 실행 중이어야 합니다.

```bash
cd web && npx playwright test --reporter=list
```

### API Smoke Test

```bash
./scripts/runbook_local.sh smoke
```

## API 엔드포인트

API 서버: `http://localhost:8090`

### Admin (`/api/v1/admin`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/admin/organization` | 현재 Organization 조회 |
| PATCH | `/admin/organization` | Organization 수정 |
| POST | `/admin/orgs` | Organization 생성 |
| GET/POST | `/admin/clusters` | 클러스터 목록 / 등록 |
| GET/PATCH/DELETE | `/admin/clusters/:id` | 클러스터 상세 / 수정 / 삭제 |
| POST | `/admin/clusters/:id/verify` | 클러스터 연결 검증 (K8s API 실연동) |
| GET/POST | `/admin/organizations/:orgId/members` | 멤버 목록 / 초대 (기존 사용자 추가 포함) |
| DELETE/PATCH | `/admin/organizations/:orgId/members/:id` | 멤버 제거 / 역할 변경 |
| GET | `/admin/users/search?email=` | 기존 사용자 검색 |
| GET | `/admin/clusters/:id/namespaces` | 클러스터 네임스페이스 목록 (K8s API) |
| GET | `/admin/known-issues` | Known Issues 목록 |
| GET | `/admin/audit-logs` | 감사 로그 |
| GET/POST | `/admin/notifications/configs` | 알림 설정 |
| GET | `/admin/notifications/history` | 알림 이력 |

### Stack (`/api/v1/stacks`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET/POST | `/stacks` | 스택 목록 / 생성 (namespace 지정 가능) |
| DELETE | `/stacks/:id` | 스택 삭제 (Helm uninstall 포함) |
| GET | `/stacks/templates` | Golden Path 템플릿 (3개) |
| GET | `/stacks/compatibility` | 도구 호환성 매트릭스 |
| POST | `/stacks/:id/deploy` | 스택 배포 (Helm SDK) |
| GET | `/stacks/:id/status` | 배포 상태 조회 |

### CI/CD (`/api/v1/cicd`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/cicd/templates` | CI/CD 파이프라인 템플릿 |
| GET/POST | `/cicd/pipelines` | 파이프라인 목록 / 생성 |
| POST | `/cicd/pipelines/:id/deploy` | 파이프라인 배포 |
| GET | `/cicd/deployments` | 배포 이력 |
| GET | `/cicd/app-templates` | Developer Self-Service 앱 템플릿 |
| POST | `/cicd/deploy-app` | 앱 배포 |

### Observability (`/api/v1/observability`)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/observability/dashboard` | 모니터링 대시보드 |
| GET/POST | `/observability/alert-rules` | 알림 규칙 목록 / 생성 |
| PATCH/DELETE | `/observability/alert-rules/:id` | 알림 규칙 수정 / 삭제 |
| GET | `/observability/alert-history` | 알림 이력 |

### 기타

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/health` | 서버 + DB 상태 |
| WS | `/ws/deployments/:id/logs` | 배포 로그 실시간 스트리밍 |

## 기능 구현 현황 (PRD v1.3 Phase 1)

| 기능 | 설명 | 상태 |
|------|------|------|
| F0 | Organization 설정 등록 | [x] Postgres Repository, CRUD API |
| F1 | K8S Cluster 등록/검증 | [x] Postgres Repository, client-go, Kubeconfig 암호화 |
| F2 | 노코드 DevSecOps Stack 설정 UI | [x] React 5단계 Wizard, RHF+Zod |
| F3 | Golden Path 템플릿 | [x] 3개 템플릿, Postgres Seed |
| F4 | 스택 자동 설치/배포/이력 | [x] Helm SDK 3-Phase DAG, Rollback, WebSocket 로그 |
| F5 | CI/CD Pipeline 템플릿 | [x] Pipeline 템플릿, K8s Manifest Generator |
| F6 | CI/CD Pipeline 배포/이력 | [x] Manifest Applier, 배포 추적 |
| F7 | 모니터링/알림 관리 | [x] Prometheus HTTP Client, Dashboard, Alert CRUD |
| F8 | 버전 호환성 관리 | [x] 호환성 매트릭스, JSONB Diff, 검증 API |
| F9 | UI 권한 체계 | [x] Keycloak OIDC, JWT, 라우트별 RBAC |
| F10 | 리소스 예상량 계산 | [x] 리소스 계산기, 비용 추정 |
| F11 | 기존 사용자 추가 | [x] org_members 멀티 조직, 이메일 검색, 즉시 활성 |
| F12 | 네임스페이스 선택/생성 | [x] K8s API 조회, 스택별 namespace 지정 |

전체 기능 명세: [PRD v1.3](./docs/10_제품기획/nullus_PRD_1.3.md)

## 프로젝트 구조

```
nullus/
├── cmd/api/                # API 서버 진입점
├── configs/                # 설정 파일 (config.yaml)
├── db/migrations/          # DB 마이그레이션 (18개)
├── deploy/helm/nullus/     # Helm 차트
├── internal/               # 내부 모듈 (Clean Architecture)
│   ├── admin/              # 조직/클러스터/사용자 관리
│   ├── auth/               # Keycloak OIDC/JWT/RBAC
│   ├── cicd/               # CI/CD 파이프라인
│   ├── observability/      # 모니터링/알림
│   ├── stack/              # DevSecOps 스택 설치
│   └── shared/             # 공유 미들웨어, audit, 알림
├── pkg/crypto/             # AES-256 암호화
├── scripts/                # 운영 스크립트 (runbook, keycloak, kind)
├── web/                    # React 프론트엔드
│   ├── src/features/       # 기능별 모듈 (admin, auth, cicd, observability, stack)
│   └── e2e/                # Playwright E2E 테스트 (41개)
├── e2e/                    # Go E2E 테스트
├── CHANGELOG.md            # 변경 이력
├── ROADMAP.md              # 로드맵
└── Makefile                # 개발 명령어
```

## 아키텍처

### 설계 원칙

**Modular Monolith**: 모놀리스로 시작하되, 모듈 경계를 명확히 하여 향후 마이크로서비스로 전환 가능하도록 설계합니다.

**Clean Architecture**: 의존성은 항상 안쪽(도메인)을 향합니다.

```
[Handler/Controller] → [UseCase/Service] → [Domain/Entity]
        ↓                      ↓
  [Repository Interface]  [Domain Logic]
        ↓
  [Repository Impl (DB/API)]
```

**Domain-Driven Design (DDD)**: 5개 Bounded Context로 구성됩니다.

| Context | 모듈 | 핵심 기능 |
|---------|------|----------|
| Stack Management | `internal/stack/` | DevSecOps 스택 설치/관리 (Helm SDK) |
| CI/CD Pipeline | `internal/cicd/` | 파이프라인 템플릿/배포 |
| Observability | `internal/observability/` | Prometheus 대시보드, 알림 |
| Organization | `internal/admin/` | 조직/사용자/클러스터 관리, 감사 로그 |
| Auth | `internal/auth/` | Keycloak OIDC, JWT, RBAC, SSO 프로비저닝 |

## 역할 체계

| 역할 | API 접근 | 주요 기능 |
|------|---------|----------|
| Admin | 전체 | 조직 설정, 사용자 관리, 클러스터 등록/검증, Known Issues |
| DevOps | stacks, cicd, observability | 스택 설치/배포, 파이프라인 관리, 알림 규칙 CRUD |
| Developer | cicd, observability (읽기) | 파이프라인 배포, 모니터링 대시보드 조회 |

## Helm 차트

```bash
# 린트
helm lint deploy/helm/nullus/

# 템플릿 확인
helm template nullus deploy/helm/nullus/ --values deploy/helm/nullus/values.yaml
```

## 라이선스

Apache License 2.0

## 커뮤니티

- **GitHub**: [cloud-nullus/draft](https://github.com/cloud-nullus/draft)
- **Issues**: 기능 요청 및 버그 리포트는 GitHub Issues를 이용
- **Discussions**: 아이디어 및 질문은 GitHub Discussions에서 논의

## 참고 문서

- [CHANGELOG.md](./CHANGELOG.md) — 변경 이력
- [ROADMAP.md](./ROADMAP.md) — 개발 로드맵
- [CLAUDE.md](./CLAUDE.md) — 아키텍처 원칙 및 개발 규칙
- [kind E2E 테스트 가이드](./docs/guides/kind-e2e-testing-guide.md) — K8s 클러스터 대상 시나리오 테스트
- [PRD v1.3](./docs/10_제품기획/nullus_PRD_1.3.md) — 제품 요구사항 명세
- [API 설계](./docs/20_아키텍처/Nullus_API_설계.md) — API 상세 설계
- [로컬 개발환경 세팅](./docs/50_운영/Nullus%20로컬%20개발환경%20세팅%20가이드.md) — 개발 환경 설정

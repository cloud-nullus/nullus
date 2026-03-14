# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Nullus Platform — Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼 (github.com/cloud-nullus/draft)

- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- **Backend**: Go 1.24+ (Gin/Echo) + PostgreSQL 18+
- **Infra**: Helm, Kubernetes 1.26+, Keycloak OIDC

## Repository State

- `docs/` 폴더에 기획 문서 구조화 완료 (PRD v1.3, 아키텍처, UI/UX 등)
- 애플리케이션 코드는 아직 없음 — 구현 착수 시 아래 원칙을 따를 것

---

## Architecture Principles

### Modular Monolith

모놀리스로 시작하되, 모듈 경계를 명확히 한다. 마이크로서비스 전환이 필요해질 때 모듈 단위로 분리할 수 있도록 설계한다.

- 각 모듈은 독립된 디렉토리를 가진다 (예: `internal/stack/`, `internal/cicd/`, `internal/admin/`)
- 모듈 간 통신은 반드시 공개 인터페이스(exported interface)를 통해서만 한다
- 모듈 간 직접 import 금지 — 다른 모듈의 internal 패키지를 참조하지 않는다
- 공유가 필요한 타입은 `internal/shared/` 또는 `pkg/` 에 둔다
- 데이터베이스 테이블은 모듈별로 소유한다 — 다른 모듈의 테이블을 직접 조회하지 않는다

### Clean Architecture

의존성 방향은 항상 안쪽(도메인)을 향한다. 외부 프레임워크, DB, UI는 바깥 레이어에 둔다.

```
[Handler/Controller] → [UseCase/Service] → [Domain/Entity]
        ↓                      ↓
  [Repository Interface]  [Domain Logic]
        ↓
  [Repository Impl (DB/API)]
```

**레이어 규칙:**
- **Domain (Entity)**: 비즈니스 규칙, 외부 의존 없음. 프레임워크 import 금지
- **UseCase (Service)**: 애플리케이션 로직, Repository 인터페이스에 의존
- **Interface (Handler/Controller)**: HTTP/gRPC/CLI 어댑터, UseCase를 호출
- **Infrastructure (Repository Impl)**: DB, 외부 API 구현체. 인터페이스를 구현

**Go 프로젝트 구조 예시:**
```
internal/
  stack/                    # DevSecOps 스택 모듈
    domain/                 # Entity, Value Object, Domain Service
      stack.go
      template.go
      errors.go
    usecase/                # Application Service (비즈니스 유스케이스)
      install_stack.go
      list_stacks.go
    port/                   # Input/Output 포트 (인터페이스)
      repository.go
      installer.go
    adapter/
      handler/              # HTTP Handler (Gin/Echo)
        stack_handler.go
      repository/           # DB 구현체
        postgres_stack.go
      helm/                 # Helm 설치 구현체
        helm_installer.go
```

**Frontend 구조 예시:**
```
src/
  features/
    stack/                  # 스택 모듈
      components/           # UI 컴포넌트
      hooks/                # 커스텀 훅 (useCase 역할)
      types/                # 도메인 타입
      api/                  # API 호출 (infrastructure)
```

### Domain-Driven Design (DDD)

도메인 모델이 코드의 중심이다. 기술적 관심사가 아닌 비즈니스 도메인을 기준으로 코드를 조직한다.

**Bounded Context (모듈 = 바운디드 컨텍스트):**

| Context | 모듈 | 핵심 Aggregate |
|---------|------|---------------|
| Stack Management | `internal/stack/` | Stack, Template, Compatibility |
| CI/CD Pipeline | `internal/cicd/` | Pipeline, Deployment |
| Observability | `internal/observability/` | Dashboard, Alert |
| Organization | `internal/admin/` | Organization, User, Cluster |
| Auth | `internal/auth/` | Session, Token |

**DDD 규칙:**
- Entity는 식별자(ID)를 가진다. Value Object는 불변이며 동등성으로 비교한다
- Aggregate Root를 통해서만 하위 Entity에 접근한다
- Repository는 Aggregate Root 단위로 정의한다
- 도메인 이벤트로 모듈 간 느슨한 결합을 유지한다 (예: `StackDeployed`, `PipelineCreated`)
- Ubiquitous Language를 사용한다 — PRD/기능분해도의 용어를 코드에 그대로 반영

---

## TDD (Test-Driven Development)

테스트를 먼저 작성하고, 실패를 확인한 뒤, 최소한의 구현으로 통과시킨다.

### TDD 사이클

```
1. RED    — 실패하는 테스트를 작성한다
2. GREEN  — 테스트를 통과시키는 최소한의 코드를 작성한다
3. REFACTOR — 중복을 제거하고 코드를 정리한다 (테스트는 계속 통과)
```

### 테스트 전략

| 레이어 | 테스트 유형 | 도구 | 범위 |
|--------|-----------|------|------|
| Domain | 단위 테스트 | Go: `testing`, FE: `vitest` | 비즈니스 규칙, 순수 함수 |
| UseCase | 단위 테스트 + 모킹 | Go: `testify/mock`, FE: `msw` | Repository 인터페이스 모킹 |
| Handler | 통합 테스트 | Go: `httptest`, FE: `testing-library` | HTTP 요청/응답 검증 |
| Repository | 통합 테스트 | Go: `testcontainers` | 실제 DB 연동 (테스트 컨테이너) |
| E2E | E2E 테스트 | Playwright | 전체 흐름 검증 |

### 테스트 규칙

- 새 기능 구현 시 반드시 테스트부터 작성한다
- 버그 수정 시 버그를 재현하는 테스트를 먼저 작성한다
- Domain 레이어는 100% 단위 테스트, UseCase는 핵심 시나리오 커버
- 테스트 커버리지 목표: >70% (v1 GA 기준)
- 테스트 파일은 테스트 대상과 같은 디렉토리에 `_test.go` / `.test.ts` 로 둔다
- 테스트에서 외부 의존(DB, API)은 인터페이스를 통해 모킹한다 — Domain/UseCase 테스트는 DB 없이 실행 가능해야 한다

### Go 테스트 명명 규칙

```go
func TestStackService_Install_Success(t *testing.T) { ... }
func TestStackService_Install_ClusterNotFound(t *testing.T) { ... }
func TestStackService_Install_IncompatibleVersion(t *testing.T) { ... }
```

### Frontend 테스트 명명 규칙

```typescript
describe('useInstallStack', () => {
  it('should deploy stack when cluster is connected', () => { ... })
  it('should show error when cluster is unreachable', () => { ... })
})
```

---

## Development Workflow

### 브랜치 전략

```
main (프로덕션)
  └── feat/<module>/<description>   # 기능 브랜치
  └── fix/<module>/<description>    # 버그 수정
  └── refactor/<module>/<description>
```

### 커밋 메시지

```
<type>(<module>): <description>

feat(stack): 스택 설치 워크플로우 5단계 구현
fix(cicd): 파이프라인 배포 롤백 시 PVC 보존
test(admin): 클러스터 등록 통합 테스트 추가
refactor(stack): Install Engine 상태 머신 정리
docs: PRD v1.3 업데이트
```

### 코드 리뷰 기준

- Clean Architecture 레이어 위반 없는지 확인
- 모듈 간 직접 의존 없는지 확인
- 테스트가 함께 포함되어 있는지 확인
- 도메인 용어가 코드에 일관되게 사용되는지 확인

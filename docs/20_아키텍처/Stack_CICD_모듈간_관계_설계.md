# Stack ↔ CI/CD 모듈 간 관계 설계

> 작성일: 2026-04-02
> 상태: 구현 완료 (v0.2-alpha)

---

## 1. 배경

Nullus 플랫폼에서 **Stack**(인프라 프로비저닝)과 **CI/CD**(애플리케이션 파이프라인)는 각각 독립된 Bounded Context로 설계되어 있다. 사용자 워크플로우 상으로는 "Stack을 먼저 배포한 뒤, 그 위에서 CI/CD 파이프라인을 생성"하는 순차 관계가 있지만, 코드 레벨에서 이 관계가 미완성 상태였다.

### 발견된 문제

| 항목 | 설계 | 구현 (수정 전) |
|------|------|--------------|
| `pipelines.stack_id` DB 컬럼 | migration #20에서 추가 | ✅ FK 존재 |
| `Pipeline.StackID` 도메인 필드 | 정의됨 | ✅ 필드 존재 |
| Repository SELECT에 `stack_id` 포함 | 필요 | ❌ 누락 |
| Repository INSERT에 `stack_id` 포함 | 필요 | ❌ 누락 |
| Handler → UseCase `stack_id` 전달 | 필요 | ❌ 누락 |
| `CreatePipelineInput.StackID` | 필요 | ❌ 필드 없음 |
| Stack 존재/Org 검증 | 권장 | ❌ 없음 |
| `GET /stacks/:id/pipelines` | 편의 API | ❌ 없음 |

---

## 2. 설계 원칙

### 2.1 Bounded Context 분리 유지

Stack 모듈과 CI/CD 모듈은 서로의 `domain` 패키지를 직접 import하지 않는다.

```
internal/stack/domain/   ←  Stack Context 소유
internal/cicd/domain/    ←  CI/CD Context 소유

❌ 금지: import "github.com/cloud-nullus/draft/internal/stack/domain"
         (CI/CD 모듈 내부에서)
```

### 2.2 Port 인터페이스를 통한 느슨한 결합

CI/CD 모듈이 Stack 정보를 필요로 할 때, 자신의 Port 레이어에 **최소 인터페이스**를 정의하고 Adapter에서 구현한다.

```
cicd/port/stack_reader.go    ← 인터페이스 정의 (CI/CD가 소유)
cicd/adapter/repository/     ← 구현체 (같은 DB를 직접 조회)
```

### 2.3 선택적 참조 (Optional Reference)

Pipeline은 Stack 없이도 독립적으로 존재할 수 있다. `stack_id`는 nullable이며, FK는 `ON DELETE SET NULL`로 설정되어 Stack 삭제 시 Pipeline이 고아가 되지만 유지된다.

---

## 3. 아키텍처

### 3.1 모듈 간 의존 관계

```
┌──────────────────────────────────────────────────────┐
│                   CI/CD Context                       │
│                                                       │
│  ┌─────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │ Handler │───▶│   UseCase    │───▶│    Domain     │ │
│  │         │    │ CreatePipeline│   │   Pipeline    │ │
│  └─────────┘    └──────┬───────┘    └──────────────┘ │
│                        │                              │
│                 ┌──────▼───────┐                      │
│                 │     Port     │                      │
│                 │ StackReader  │  ← 인터페이스        │
│                 │ (interface)  │                      │
│                 └──────┬───────┘                      │
│                        │                              │
│                 ┌──────▼───────────────┐              │
│                 │      Adapter         │              │
│                 │ PostgresStackReader   │              │
│                 └──────┬───────────────┘              │
└────────────────────────┼─────────────────────────────┘
                         │ SQL (SELECT id, org_id, state FROM stacks)
                         ▼
                 ┌───────────────┐
                 │  PostgreSQL   │
                 │  stacks 테이블 │  ← Stack Context가 소유
                 └───────────────┘
```

### 3.2 데이터 흐름: Pipeline 생성

```
Client
  │ POST /api/v1/cicd/pipelines { "stack_id": "stk_xxx", ... }
  ▼
PipelineHandler
  │ req.StackID → CreatePipelineInput.StackID
  ▼
CreatePipeline UseCase
  │ if StackID != "" && stackReader != nil:
  │   summary := stackReader.GetStackSummary(stackID)
  │   if summary == nil → ErrStackNotFound
  │   if summary.OrgID != input.OrgID → ErrStackOrgMismatch
  │   if summary.State != "completed" → warning (허용은 함)
  │
  │ pipeline.StackID = input.StackID
  ▼
PipelineRepository.Create
  │ INSERT INTO pipelines (..., stack_id, ...) VALUES (..., $6, ...)
  ▼
Response: { "pipeline": {...}, "warning": "stack is in state pending..." }
```

---

## 4. 구현 상세

### 4.1 새로 추가된 파일

#### `internal/cicd/port/stack_reader.go`

CI/CD 모듈이 Stack Context에서 필요한 최소 정보만 정의한다.

```go
type StackSummary struct {
    ID        string
    OrgID     string
    ClusterID string
    State     string  // "completed", "failed", etc.
}

type StackReader interface {
    GetStackSummary(ctx context.Context, stackID string) (*StackSummary, error)
}
```

**설계 의도**: Stack의 전체 도메인 모델(Config, Tools, Template 등)을 노출하지 않고, Pipeline 생성에 필요한 검증 정보만 제공한다.

#### `internal/cicd/adapter/repository/postgres_stack_reader.go`

Modular Monolith 단계에서는 같은 DB의 `stacks` 테이블을 직접 조회한다.

```go
func (r *PostgresStackReader) GetStackSummary(ctx context.Context, stackID string) (*port.StackSummary, error) {
    const q = `SELECT id, org_id, cluster_id, state FROM stacks WHERE id = $1`
    // ...
}
```

**마이크로서비스 전환 시**: 이 구현체를 HTTP/gRPC 클라이언트로 교체하면 된다. Port 인터페이스는 변경 없음.

### 4.2 수정된 파일

#### `internal/cicd/port/repository.go`

`PipelineRepository` 인터페이스에 메서드 추가:

```go
type PipelineRepository interface {
    // ... 기존 메서드 ...
    List(ctx context.Context, orgID string, stackID ...string) ([]*domain.Pipeline, error)
    ListByStackID(ctx context.Context, stackID string) ([]*domain.Pipeline, error)
}
```

- `List`의 `stackID` 파라미터는 variadic으로 하위 호환성 유지
- `ListByStackID`는 Org 필터 없이 Stack ID로만 조회 (Stack 상세 화면용)

#### `internal/cicd/adapter/repository/postgres_pipeline.go`

| 메서드 | 변경 내용 |
|--------|----------|
| `Create` | `stack_id` 컬럼 추가 (nullable) |
| `GetByID` | SELECT에 `COALESCE(stack_id, '')` 추가 |
| `List` | `stack_id` 필터 지원 + SELECT에 `stack_id` 추가 |
| `ListByStackID` | 신규 — `WHERE stack_id = $1` |
| `Update` | `stack_id` 컬럼 업데이트 추가 |
| `scanPipeline` | `&p.StackID` scan 추가 |

#### `internal/cicd/usecase/create_pipeline.go`

| 변경 | 상세 |
|------|------|
| `CreatePipelineInput.StackID` | 신규 필드 |
| `CreatePipelineOutput.StackWarning` | 신규 필드 — Stack이 미완료 상태일 때 경고 메시지 |
| `NewCreatePipeline` | `stackReader ...port.StackReader` variadic 파라미터 (하위 호환) |
| `Execute` 검증 로직 | Stack 존재 확인 + Org 일치 확인 + 상태 경고 |
| Sentinel errors | `ErrStackNotFound`, `ErrStackOrgMismatch` |

#### `internal/cicd/usecase/list_pipelines.go`

| 변경 | 상세 |
|------|------|
| `ListPipelinesInput.StackID` | 신규 필드 — 선택적 필터 |

#### `internal/cicd/adapter/handler/pipeline_handler.go`

| 변경 | 상세 |
|------|------|
| `createPipelineRequest.StackID` | 신규 JSON 필드 `"stack_id"` |
| `CreatePipeline` 핸들러 | `StackID` 전달 + 에러 분기 + warning 응답 |
| `ListPipelines` 핸들러 | `?stack_id=` 쿼리 파라미터 지원 |
| `ListPipelinesByStack` | 신규 — `GET /stacks/:stackId/pipelines` |
| `RegisterStackRoutes` | 신규 — Stack 라우트 그룹에 등록하는 메서드 |

#### `internal/cicd/adapter/repository/memory_pipeline.go`

In-memory 구현체도 동일하게 `List` 시그니처 변경 및 `ListByStackID` 추가.

---

## 5. API 변경

### 5.1 Pipeline 생성 (변경)

```
POST /api/v1/cicd/pipelines

Request:
{
  "name": "user-service",
  "template_id": "web-backend-v1",
  "cluster_id": "cls_abc123",
  "stack_id": "stk_m1n2o3",     ← 신규 (선택)
  "namespace": "app-user",
  "app_type": "backend",
  "git_repo_url": "https://github.com/org/user-service"
}

Response (201):
{
  "pipeline": { ... "stack_id": "stk_m1n2o3" ... },
  "warning": "stack \"stk_m1n2o3\" is in state \"installing\" — CI/CD tools may not be available yet"
}

Error (400): STACK_NOT_FOUND — stack_id가 존재하지 않을 때
Error (403): STACK_ORG_MISMATCH — stack이 다른 Org 소속일 때
```

### 5.2 Pipeline 목록 (변경)

```
GET /api/v1/cicd/pipelines?stack_id=stk_m1n2o3    ← 신규 필터

Response (200):
{
  "items": [...],
  "total": 3
}
```

### 5.3 Stack별 Pipeline 목록 (신규)

```
GET /api/v1/stacks/:stackId/pipelines

Response (200):
{
  "items": [...],
  "total": 3
}
```

---

## 6. DB 스키마 (기존 유지)

migration #20에서 이미 추가된 스키마를 그대로 활용한다. 추가 마이그레이션 불필요.

```sql
-- migration 000020_pipeline_stack_relation.up.sql (기존)
ALTER TABLE pipelines
  ADD COLUMN stack_id VARCHAR(100) REFERENCES stacks(id) ON DELETE SET NULL;

CREATE INDEX idx_pipelines_stack_id ON pipelines(stack_id);
```

| 속성 | 값 | 설명 |
|------|---|------|
| Nullable | ✅ | Pipeline은 Stack 없이도 존재 가능 |
| FK | `stacks.id` | 참조 무결성 보장 |
| ON DELETE | SET NULL | Stack 삭제 시 Pipeline 유지, `stack_id`만 NULL |
| Index | `idx_pipelines_stack_id` | `ListByStackID` 쿼리 성능 보장 |

---

## 7. 검증 규칙

Pipeline 생성 시 `stack_id`가 제공되면 다음 검증을 수행한다:

| # | 검증 | 실패 시 |
|---|------|--------|
| 1 | Stack이 존재하는가? | `400 STACK_NOT_FOUND` 반환 |
| 2 | Stack의 `org_id`가 Pipeline의 `org_id`와 같은가? | `403 STACK_ORG_MISMATCH` 반환 |
| 3 | Stack의 `state`가 `"completed"`인가? | **경고만** — 생성은 허용 |

3번은 차단하지 않는다. Stack 배포와 Pipeline 생성을 병렬로 준비할 수 있는 유연성을 제공하기 위함이다. 실제 Pipeline 배포(`/deploy`) 시점에는 Stack의 CI/CD 도구가 설치되어 있어야 하므로, 그 시점에서 추가 검증을 수행하는 것이 적절하다.

---

## 8. 마이크로서비스 전환 경로

현재 Modular Monolith에서는 `PostgresStackReader`가 동일 DB의 `stacks` 테이블을 직접 조회한다. 마이크로서비스로 분리할 때:

1. `StackReader` Port 인터페이스는 **변경 없음**
2. `PostgresStackReader` → `HTTPStackReader` (또는 gRPC)로 구현체만 교체
3. Stack 서비스에 `GET /internal/stacks/:id/summary` 내부 API 추가
4. DI(Dependency Injection) 설정에서 구현체 교체

```go
// 마이크로서비스 전환 시 구현체 예시
type HTTPStackReader struct {
    baseURL    string
    httpClient *http.Client
}

func (r *HTTPStackReader) GetStackSummary(ctx context.Context, stackID string) (*port.StackSummary, error) {
    resp, err := r.httpClient.Get(r.baseURL + "/internal/stacks/" + stackID + "/summary")
    // ...
}
```

---

## 9. 향후 확장 (Phase 2+)

### 도메인 이벤트 도입

v1.x 이후 Stack 배포 완료 시 `StackDeployed` 이벤트를 발행하고, CI/CD 모듈이 구독하여 사용 가능한 도구 목록을 캐싱하는 방식을 고려한다.

```go
// 향후 구현 예시
type StackDeployed struct {
    StackID   string
    OrgID     string
    ClusterID string
    Tools     []string  // ["gitlab", "argocd", "prometheus", ...]
}
```

이벤트 기반으로 전환하면, Pipeline 배포 시 Stack의 실시간 상태를 매번 조회하지 않고 로컬 캐시로 검증할 수 있다.

### Pipeline → Stack 역참조

Stack 상세 화면에서 "이 인프라에서 실행 중인 애플리케이션" 목록을 보여주는 기능을 위해, 이미 `GET /api/v1/stacks/:stackId/pipelines` 엔드포인트를 마련했다. 프론트엔드에서 이 API를 호출하면 된다.

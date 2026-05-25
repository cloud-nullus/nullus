# Stack를 통한 CICD 긴급모드 설계

> 작성일: 2026-04-02
> 수정일: 2026-05-25
> 상태: 기존 직접 배포 경로 구현 / 일반 운영 경로 아님
> 정상 운영 설계: [Stack_CICD_통합모드_설계.md](./Stack_CICD_통합모드_설계.md)

---

## 1. 문서 목적

이 문서는 CI/CD 파이프라인이 배포된 Stack의 Repository 및 Registry를 사용하지 못하는 상황에서 동작하는 **긴급 직접 배포 모드**(`emergency_direct`)를 설명한다.

신규 CI/CD 개발의 기본 경로는 `stack_integrated` 모드이며, 배포 완료 Stack의 Code Repository, Package Registry, Image Registry, CI Platform, CD Tool을 활용한다. 해당 설계는 별도 문서인 [Stack_CICD_통합모드_설계.md](./Stack_CICD_통합모드_설계.md)를 기준으로 한다.

`emergency_direct` 모드는 다음 경우에만 사용한다.

- Stack 컴포넌트 연계가 불가능하거나 아직 준비되지 않은 경우
- 통합모드 장애 시 운영자가 명시적으로 긴급 실행을 선택한 경우
- 로컬 Kind 환경에서 앱 배포 동작을 빠르게 검증하는 경우
- 장애 복구 또는 제한된 임시 배포가 필요한 경우


## 2. 실행 흐름

긴급모드에서는 Nullus가 애플리케이션 소스를 직접 가져와 이미지를 만들고 Kubernetes 대상 클러스터에 적용한다.

```text
Git Clone
  -> Docker Build
  -> kind load docker-image
  -> K8s Manifest Generate
  -> K8s Apply
  -> Deployment Status / Log Tracking
```

이 흐름은 배포된 Stack의 `code_repository`, `package_registry`, `image_registry`를 정상적인 pipeline runtime으로 사용하는 구조가 아니다. 따라서 새로운 일반 운영 기능을 이 흐름 위에 확장하지 않는다.

## 3. 모듈 관계

### 3.1 Bounded Context 분리

Stack 모듈과 CI/CD 모듈은 서로의 `domain` 패키지를 직접 import하지 않는다.

```
internal/stack/domain/   ←  Stack Context 소유
internal/cicd/domain/    ←  CI/CD Context 소유

금지: CI/CD 모듈에서 internal/stack/domain 직접 import
```

### 3.2 선택적 Stack 참조

긴급모드 및 기존 데이터 하위 호환을 위해 Pipeline은 Stack 없이 존재할 수 있다.

- `pipelines.stack_id`는 nullable이다.
- Stack이 삭제되면 FK는 `ON DELETE SET NULL`로 Pipeline을 유지한다.
- `stack_id`가 존재하더라도 긴급모드 실행 자체는 Stack Registry를 사용한다는 의미가 아니다.

```text
Pipeline (emergency_direct)
  -> optional stack_id: 목록 연결, 이력 문맥, 향후 통합 전환 식별자
  -> deployment runtime: Nullus 직접 build/apply
```

---

## 3. 아키텍처

### 3.1 모듈 간 의존 관계

```
┌───────────────────────────────────────────────────────┐
│                   CI/CD Context                       │
│                                                       │
│  ┌─────────┐    ┌───────────────┐    ┌──────────────┐ │
│  │ Handler │───▶│   UseCase     │───▶│    Domain    │ │
│  │         │    │ CreatePipeline│    │   Pipeline   │ │
│  └─────────┘    └──────┬────────┘    └──────────────┘ │
│                        │                              │
│                 ┌──────▼───────┐                      │
│                 │     Port     │                      │
│                 │ StackReader  │  ← 인터페이스           │
│                 │ (interface)  │                      │
│                 └──────┬───────┘                      │
│                        │                              │
│                 ┌──────▼───────────────┐              │
│                 │      Adapter         │              │
│                 │ PostgresStackReader  │              │
│                 └──────┬───────────────┘              │
└────────────────────────┼──────────────────────────────┘
                         │ SQL (SELECT id, org_id, state FROM stacks)
                         ▼
                 ┌───────────────┐
                 │  PostgreSQL   │
                 │  stacks 테이블  │  ← Stack Context가 소유
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

## 실행 정책

| 조건 | 처리 |
|---|---|
| 사용자가 `emergency_direct`를 명시 선택 | 기존 직접 배포 흐름 실행 |
| Stack 통합 연결이 장애 상태이고 긴급 실행 승인 | 기존 직접 배포 흐름 실행 |
| 정상 운영 신규 Pipeline 구성 | 통합모드 문서를 따름 |
| Stack Registry를 활용한 이미지 publish/deploy 필요 | 긴급모드 범위 아님 |

긴급모드에서는 Compatibility Matrix를 CI/CD 실행 차단 규칙으로 추가하지 않는다. 다만 기존 Stack 자체의 설치 전 호환성 검증 기능은 별도 기능으로 유지된다.

## 일반모드로의 전환 경계

다음 요구가 발생하면 이 문서의 직접 배포 경로를 확장하지 않고 통합모드 구현으로 처리한다.

- Stack의 Code Repository에 프로젝트를 만들거나 workflow 파일을 저장해야 하는 경우
- Stack의 Package Registry에 artifact, SBOM, test report를 publish해야 하는 경우
- Stack의 Image Registry에 이미지를 push하고 해당 digest를 배포해야 하는 경우
- Stack의 CI Platform 또는 CD Tool에 pipeline/application을 프로비저닝해야 하는 경우
- 외부 provider run 결과와 CD sync 결과를 Nullus 이력으로 수집해야 하는 경우

---

본 문서는 기존 직접 배포 코드를 유지하기 위한 긴급모드 기준이다. 정상 운영 CI/CD는 [Stack_CICD_통합모드_설계.md](./Stack_CICD_통합모드_설계.md)를 따른다.

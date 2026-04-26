# Nullus 백엔드 모듈 개발 가이드

**작성일**: 2026-03-22
**기술 스택**: Go 1.24+ · Echo v4 · PostgreSQL 18+ · Helm SDK

---

## 1. 아키텍처 원칙

Nullus 백엔드는 **Modular Monolith + Clean Architecture + DDD**를 따른다.

```
의존성 방향 (항상 안쪽으로):

[Handler/Controller] → [UseCase/Service] → [Domain/Entity]
        ↓                      ↓
  [Repository Interface]  [Domain Logic]
        ↓
  [Repository Impl (DB/API)]
```

**핵심 규칙**:
- Domain 레이어는 외부 의존 없음 (프레임워크 import 금지)
- 모듈 간 직접 import 금지 — `internal/shared/`의 공유 인터페이스만 사용
- Repository는 인터페이스(Port)로 정의, 구현체(Adapter)는 별도

---

## 2. 모듈 디렉토리 구조

```
internal/{module}/
├── domain/                    # Entity, Value Object, Domain Service, Errors
│   ├── {entity}.go            # Stack, Pipeline, Alert, Organization 등
│   ├── {entity}_test.go       # 도메인 로직 단위 테스트
│   └── errors.go              # 도메인 에러 정의
│
├── usecase/                   # Application Service
│   ├── {action}_{entity}.go   # create_stack.go, install_stack.go, add_tools.go
│   ├── {action}_{entity}_test.go
│   └── id.go                  # ID 생성 유틸리티
│
├── port/                      # 인터페이스 정의만
│   ├── repository.go          # StackRepository, TemplateRepository 등
│   └── {external}.go          # installer.go, log_streamer.go 등
│
└── adapter/
    ├── handler/               # HTTP Handler (Echo v4)
    │   ├── {entity}_handler.go
    │   └── {entity}_handler_test.go
    ├── repository/            # 데이터 접근 구현체
    │   ├── postgres_{entity}.go   # PostgreSQL 구현
    │   └── memory_{entity}.go     # In-Memory 구현 (테스트용)
    └── {external}/            # 외부 시스템 어댑터
        ├── helm/              # Helm SDK 래퍼
        ├── kube/              # K8s client-go 래퍼
        └── prometheus/        # Prometheus HTTP 클라이언트
```

---

## 3. 레이어별 규칙과 예시

### Domain Layer

비즈니스 규칙의 핵심. 외부 의존 없이 순수 Go로 작성.

```go
// internal/stack/domain/stack.go
package domain

type ToolConfig struct {
    Category string `json:"category"`
    Tool     string `json:"tool"`
    Version  string `json:"version"`
}

type Stack struct {
    ID        string       `json:"id"`
    Name      string       `json:"name"`
    Tools     []ToolConfig `json:"tools"`
    Status    string       `json:"status"`
    UpdatedAt time.Time    `json:"updatedAt"`
}

// 도메인 메서드 — 비즈니스 규칙 포함
func (s *Stack) AddTools(newTools []ToolConfig) error {
    existing := make(map[string]bool)
    for _, t := range s.Tools {
        existing[t.Category+":"+t.Tool] = true
    }
    for _, t := range newTools {
        if existing[t.Category+":"+t.Tool] {
            return fmt.Errorf("tool %s already exists in category %s", t.Tool, t.Category)
        }
        s.Tools = append(s.Tools, t)
    }
    s.UpdatedAt = time.Now()
    return nil
}
```

### Port Layer

인터페이스만 정의. 구현은 Adapter에서.

```go
// internal/stack/port/repository.go
package port

type StackRepository interface {
    Create(ctx context.Context, stack *domain.Stack) error
    GetByID(ctx context.Context, id string) (*domain.Stack, error)
    List(ctx context.Context, orgID string) ([]*domain.Stack, error)
    Update(ctx context.Context, stack *domain.Stack) error
    Delete(ctx context.Context, id string) error
    FindByID(ctx context.Context, id string) (*domain.Stack, error)
    UpdateTools(ctx context.Context, stack *domain.Stack) error
}
```

### UseCase Layer

하나의 유스케이스 = 하나의 파일. Port 인터페이스에만 의존.

```go
// internal/stack/usecase/add_tools.go
package usecase

type AddToolsInput struct {
    StackID string
    Tools   []domain.ToolConfig
}

type AddToolsUseCase struct {
    repo port.StackRepository
}

func NewAddToolsUseCase(repo port.StackRepository) *AddToolsUseCase {
    return &AddToolsUseCase{repo: repo}
}

func (uc *AddToolsUseCase) Execute(ctx context.Context, input AddToolsInput) (*domain.Stack, error) {
    stack, err := uc.repo.FindByID(ctx, input.StackID)
    if err != nil {
        return nil, fmt.Errorf("stack not found: %w", err)
    }
    if err := stack.AddTools(input.Tools); err != nil {
        return nil, err
    }
    if err := uc.repo.UpdateTools(ctx, stack); err != nil {
        return nil, fmt.Errorf("failed to update tools: %w", err)
    }
    return stack, nil
}
```

### Handler (Adapter)

Echo context → UseCase 호출 → HTTP 응답.

```go
// internal/stack/adapter/handler/stack_handler.go
func (h *StackHandler) AddTools(c echo.Context) error {
    stackID := c.Param("stackId")
    var req struct {
        Tools []domain.ToolConfig `json:"tools" validate:"required,min=1"`
    }
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
    }
    result, err := h.addToolsUC.Execute(c.Request().Context(), usecase.AddToolsInput{
        StackID: stackID,
        Tools:   req.Tools,
    })
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    return c.JSON(http.StatusOK, result)
}
```

---

## 4. 현재 모듈 구성

| 모듈 | 경로 | 핵심 Entity | API 엔드포인트 |
|------|------|------------|---------------|
| **admin** | `internal/admin/` | Organization, Cluster, User | `/api/v1/admin/*` |
| **auth** | `internal/auth/` | (Adapter 중심) | 미들웨어 |
| **stack** | `internal/stack/` | Stack, Template, Compatibility, History | `/api/v1/stacks/*` |
| **cicd** | `internal/cicd/` | Pipeline | `/api/v1/cicd/*` |
| **observability** | `internal/observability/` | Alert, Dashboard | `/api/v1/observability/*` |
| **shared** | `internal/shared/` | Error, Event | 미들웨어, 감사 로그, 알림 |

---

## 5. TDD 워크플로우

```
1. RED    — 실패하는 테스트를 먼저 작성한다
2. GREEN  — 테스트를 통과시키는 최소한의 코드를 작성한다
3. REFACTOR — 중복을 제거하고 코드를 정리한다
```

### 테스트 명명 규칙

```go
func TestStack_AddTools_AppendsNewToolsAndUpdatesTimestamp(t *testing.T) { ... }
func TestStack_AddTools_DuplicateReturnsErrorAndNoMutation(t *testing.T) { ... }
func TestAddToolsUseCase_Execute_Success(t *testing.T) { ... }
func TestAddToolsUseCase_Execute_StackNotFound(t *testing.T) { ... }
```

### 테스트 실행

```bash
go test ./internal/stack/...               # 모듈별
go test ./internal/stack/domain/...        # 레이어별
go test -run TestStack_AddTools ./...      # 특정 테스트
go test -v -count=1 ./...                  # 캐시 무시
```

---

## 6. 미들웨어 체인

```
Request → Recover → RequestID → Logger → CORS → RateLimiter
  → DualAuth(Session | JWT) → RequireRole → OrgContext → Handler
```

| 미들웨어 | 파일 | 역할 |
|---------|------|------|
| `DualAuthMiddleware` | `middleware/dual_auth.go` | 세션 또는 JWT 검증 |
| `JWTAuthMiddleware` | `middleware/jwt_middleware.go` | JWT 서명 검증 + OIDCProvider.ExtractRoles |
| `RequireRole` | `middleware/rbac.go` | 역할 기반 접근 제어 |
| `OrgContext` | `shared/middleware/org_context.go` | 요청에서 Organization ID 추출 |
| `RateLimiter` | `shared/middleware/rate_limiter.go` | 인증 300/min, 미인증 30/min |

---

## 7. 새 모듈 추가 Step-by-Step

예시: `notification` 모듈 추가

### Step 1: Domain 정의

```go
// internal/notification/domain/notification.go
type Notification struct {
    ID        string    `json:"id"`
    Channel   string    `json:"channel"` // "slack" | "email"
    Message   string    `json:"message"`
    SentAt    time.Time `json:"sentAt"`
}
```

### Step 2: Port 정의

```go
// internal/notification/port/repository.go
type NotificationRepository interface {
    Create(ctx context.Context, n *domain.Notification) error
    ListByOrg(ctx context.Context, orgID string) ([]*domain.Notification, error)
}
```

### Step 3: UseCase 구현

```go
// internal/notification/usecase/send_notification.go
type SendNotificationUseCase struct { repo port.NotificationRepository }
func (uc *SendNotificationUseCase) Execute(ctx context.Context, input SendInput) error { ... }
```

### Step 4: Handler 구현

```go
// internal/notification/adapter/handler/notification_handler.go
func (h *Handler) Send(c echo.Context) error { ... }
func (h *Handler) RegisterRoutes(g *echo.Group) {
    g.POST("/notifications", h.Send)
    g.GET("/notifications", h.List)
}
```

### Step 5: Repository 구현

```go
// internal/notification/adapter/repository/postgres_notification.go
// internal/notification/adapter/repository/memory_notification.go (테스트용)
```

### Step 6: DI + 라우트 등록 (`cmd/api/main.go`)

```go
notifRepo := repository.NewPostgresNotification(db)
sendUC := usecase.NewSendNotificationUseCase(notifRepo)
notifHandler := handler.NewHandler(sendUC)
notifHandler.RegisterRoutes(api.Group("/notifications"))
```

---

## 8. 에러 처리

```go
// internal/shared/domain/errors.go
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrConflict     = errors.New("resource conflict")
)

// Handler에서 매핑
func mapError(err error) (int, string) {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        return http.StatusNotFound, err.Error()
    case errors.Is(err, domain.ErrConflict):
        return http.StatusConflict, err.Error()
    default:
        return http.StatusInternalServerError, "internal server error"
    }
}
```

---

## 9. 설정 관리

```yaml
# configs/config.yaml
server:
  port: 8090
  mode: development
database:
  host: localhost
  port: 5433
  name: nullus
  user: nullus
  password: nullus_dev
auth:
  mode: session           # session | oidc
oidc:
  provider: keycloak      # keycloak | authentik
  issuer_url: http://localhost:8180/realms/nullus
  audience: nullus-app
helm:
  namespace: nullus-system
prometheus:
  url: http://localhost:9090
```

---

## 10. 참고 자료

| 자료 | 경로 |
|------|------|
| API 서버 진입점 | `cmd/api/main.go` |
| 설정 파일 | `configs/config.yaml` |
| 백엔드 상세설계 | `docs/20_아키텍처/Nullus_백엔드_상세설계.md` |
| API 설계 | `docs/20_아키텍처/Nullus_API_설계.md` |
| DB 스키마 | `docs/20_아키텍처/Nullus_DB_스키마.md` |
| OIDC Provider 가이드 | `docs/20_개발가이드/Nullus_OIDC_Provider_가이드.md` |

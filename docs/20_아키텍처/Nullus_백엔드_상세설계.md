# Nullus Platform Go 백엔드 상세 설계 — Part 1

**작성일**: 2026-03-14
**버전**: 1.0
**기반 문서**: nullus_PRD_1.3.md, Nullus_기능목록.md, 상세 기능 명세 및 시스템 아키텍처 v1.0
**대상 독자**: Go 백엔드 엔지니어, 아키텍트

---

## 목차

1. [개요](#1-개요)
2. [Go 프로젝트 구조](#2-go-프로젝트-구조)
3. [DDD Bounded Context별 모듈 설계](#3-ddd-bounded-context별-모듈-설계)
   - 3.1 stack 모듈
   - 3.2 admin 모듈
4. [Install Engine 상세 설계](#4-install-engine-상세-설계)

---

## 1. 개요

### 1.1 기술 스택

| 분류 | 기술 | 버전 | 선택 이유 |
|------|------|------|-----------|
| 언어 | Go | 1.24+ | Kubernetes 클라이언트 라이브러리 네이티브, 단일 바이너리 배포, 고성능 동시성 |
| 웹 프레임워크 | Echo | v4 | 미들웨어 생태계 풍부, 컨텍스트 기반 라우팅, Gin 대비 표준 라이브러리 친화적 |
| DB 드라이버 | pgx/v5 | v5 | PostgreSQL 전용 고성능 드라이버, JSONB 직접 지원 |
| ORM/쿼리 | sqlc | latest | SQL 기반 타입 안전 쿼리 생성, 런타임 리플렉션 없음 |
| 마이그레이션 | golang-migrate | v4 | SQL 파일 기반, 롤백 지원 |
| Helm SDK | helm.sh/helm/v3 | v3.16+ | Helm 차트 프로그래밍 제어, K8s 네이티브 |
| K8s 클라이언트 | k8s.io/client-go | v0.32+ | K8s API 직접 통신 |
| WebSocket | gorilla/websocket | v1.5 | 설치 로그 실시간 스트리밍 |
| 인증 (Alpha~Beta) | gorilla/sessions | v1.3 | 세션 기반 단순 인증 |
| 인증 (v1) | Keycloak OIDC | — | SSO, RBAC, OSS별 권한 매핑 |
| 로깅 | slog (표준) | Go 1.21+ | 구조화 로그, 외부 의존 없음 |
| 설정 | viper | v2 | YAML/환경변수 설정 통합 |
| API 문서 | swaggo/swag | v2 | Go 구조체에서 OpenAPI 3.0 자동 생성 |
| 테스트 | testify + gomock | — | 단언 라이브러리 + mock 생성 |

### 1.2 설계 원칙

#### Modular Monolith

초기에는 단일 Go 바이너리로 배포하되, 내부는 **Bounded Context 단위 모듈**로 명확히 분리한다. 모듈 간 통신은 직접 함수 호출 대신 **인터페이스(Port)를 통한 의존성 주입**으로만 허용하여, 향후 마이크로서비스 분리 시 비용을 최소화한다.

- **현재**: `cmd/api` 단일 바이너리, 모든 모듈 in-process 호출
- **미래 분리 경계**: Install Engine은 별도 Worker 프로세스로 가장 먼저 분리 가능
- **모듈 간 금지 패턴**: `internal/stack`이 `internal/admin`의 concrete struct를 직접 import하는 것 금지 — 반드시 interface를 통해 접근

#### Clean Architecture

각 모듈 내부를 4개 레이어로 구성한다:

```
┌──────────────────────────────────────┐
│  Interface Adapters (HTTP Handler)   │  ← Echo 핸들러, 요청/응답 변환
├──────────────────────────────────────┤
│  Application (UseCase)               │  ← 비즈니스 흐름 조율, 트랜잭션 경계
├──────────────────────────────────────┤
│  Domain (Entity, VO, Domain Service) │  ← 핵심 비즈니스 규칙, 상태 머신
├──────────────────────────────────────┤
│  Infrastructure (Adapter)            │  ← DB, Helm, K8s API 구현체
└──────────────────────────────────────┘
```

의존성 방향은 **바깥 → 안쪽 단방향**. Domain 레이어는 어떤 외부 패키지도 import하지 않는다.

#### DDD (Domain-Driven Design)

식별된 Bounded Context:

| Bounded Context | 핵심 책임 | 주요 Aggregate |
|-----------------|-----------|----------------|
| **admin** | 조직, 사용자, 클러스터 등록 관리 | Organization, User, Cluster |
| **stack** | 스택 설정, 배포, 이력 관리 | Stack, Deployment |
| **template** | Golden Path / CI/CD 템플릿 카탈로그 | GoldenPathTemplate, PipelineTemplate |
| **compatibility** | 버전 호환성 매트릭스 | CompatibilityMatrix |
| **monitoring** | 대시보드, 알림 설정 | AlertConfig |
| **resource** | 리소스 예상량 계산 | ResourceEstimate |

Part 1에서는 **stack**, **admin** 모듈을 상세 설계한다.

---

## 2. Go 프로젝트 구조

```
nullus/
├── cmd/
│   └── api/
│       └── main.go                  # 진입점: DI 컨테이너 조립, 서버 기동
│
├── internal/                        # 외부 노출 금지 패키지
│   ├── admin/                       # Bounded Context: 조직·사용자·클러스터
│   │   ├── domain/
│   │   │   ├── organization.go      # Organization Aggregate Root
│   │   │   ├── user.go              # User Entity
│   │   │   ├── cluster.go           # Cluster Entity
│   │   │   ├── invite.go            # Invite Value Object
│   │   │   └── errors.go            # 도메인 에러 정의
│   │   ├── usecase/
│   │   │   ├── register_cluster.go  # RegisterCluster UseCase
│   │   │   ├── invite_member.go     # InviteMember UseCase
│   │   │   ├── create_org.go        # CreateOrganization UseCase
│   │   │   └── verify_cluster.go    # VerifyClusterConnection UseCase
│   │   ├── port/
│   │   │   ├── repository.go        # OrgRepository, UserRepository, ClusterRepository
│   │   │   └── k8s_client.go        # K8sConnector Port
│   │   ├── adapter/
│   │   │   ├── postgres/
│   │   │   │   ├── org_repository.go
│   │   │   │   ├── user_repository.go
│   │   │   │   └── cluster_repository.go
│   │   │   └── k8s/
│   │   │       └── connector.go     # client-go 기반 K8s 연결 검증
│   │   └── handler/
│   │       ├── org_handler.go       # Echo HTTP 핸들러
│   │       ├── cluster_handler.go
│   │       └── user_handler.go
│   │
│   ├── stack/                       # Bounded Context: 스택 설정·배포·이력
│   │   ├── domain/
│   │   │   ├── stack.go             # Stack Aggregate Root
│   │   │   ├── template.go          # Template Value Object
│   │   │   ├── stack_config.go      # StackConfig Value Object
│   │   │   ├── deployment.go        # Deployment Entity
│   │   │   ├── state_machine.go     # 설치 상태 머신
│   │   │   └── errors.go
│   │   ├── usecase/
│   │   │   ├── install_stack.go     # InstallStack UseCase
│   │   │   ├── list_stacks.go       # ListStacks UseCase
│   │   │   └── rollback_stack.go    # RollbackStack UseCase
│   │   ├── port/
│   │   │   ├── repository.go        # StackRepository, DeploymentRepository
│   │   │   └── installer.go         # HelmInstaller, LogStreamer Port
│   │   ├── adapter/
│   │   │   ├── postgres/
│   │   │   │   ├── stack_repository.go
│   │   │   │   └── deployment_repository.go
│   │   │   └── helm/
│   │   │       ├── installer.go     # Helm Go SDK 기반 설치기
│   │   │       ├── known_issues.go  # known-issues.yaml 패턴 처리기
│   │   │       └── log_streamer.go  # WebSocket 로그 스트리머
│   │   └── handler/
│   │       ├── stack_handler.go
│   │       └── deployment_handler.go
│   │
│   ├── template/                    # Bounded Context: 템플릿 카탈로그
│   │   ├── domain/
│   │   │   └── golden_path.go
│   │   ├── usecase/
│   │   │   └── list_templates.go
│   │   ├── port/
│   │   │   └── repository.go
│   │   ├── adapter/
│   │   │   └── yaml/
│   │   │       └── template_loader.go  # YAML 파일 기반 템플릿 로드
│   │   └── handler/
│   │       └── template_handler.go
│   │
│   ├── compatibility/               # Bounded Context: 버전 호환성
│   │   ├── domain/
│   │   │   └── matrix.go
│   │   ├── usecase/
│   │   │   └── validate_combination.go
│   │   ├── port/
│   │   │   └── repository.go
│   │   ├── adapter/
│   │   │   └── yaml/
│   │   │       └── matrix_loader.go
│   │   └── handler/
│   │       └── compatibility_handler.go
│   │
│   ├── monitoring/                  # Bounded Context: 모니터링·알림
│   │   ├── domain/
│   │   │   └── alert_config.go
│   │   ├── usecase/
│   │   │   ├── get_dashboard.go
│   │   │   └── configure_alert.go
│   │   ├── port/
│   │   │   ├── prometheus_client.go
│   │   │   └── repository.go
│   │   ├── adapter/
│   │   │   ├── prometheus/
│   │   │   │   └── client.go
│   │   │   └── postgres/
│   │   │       └── alert_repository.go
│   │   └── handler/
│   │       └── monitoring_handler.go
│   │
│   ├── resource/                    # Bounded Context: 리소스 계산
│   │   ├── domain/
│   │   │   └── estimate.go
│   │   ├── usecase/
│   │   │   └── calculate_estimate.go
│   │   └── handler/
│   │       └── resource_handler.go
│   │
│   └── shared/                      # 모듈 공유 커널 (최소화 원칙)
│       ├── domain/
│       │   ├── errors.go            # 공통 도메인 에러 타입
│       │   └── pagination.go        # 페이지네이션 VO
│       ├── crypto/
│       │   └── aes.go               # AES-256-GCM 암호화 (kubeconfig 등)
│       ├── middleware/
│       │   ├── auth.go              # 세션/OIDC 인증 미들웨어
│       │   ├── rbac.go              # 역할 기반 접근 제어
│       │   └── trace.go             # trace_id 주입
│       └── apperror/
│           └── codes.go             # HTTP 상태코드 ↔ 도메인 에러 매핑
│
├── pkg/                             # 외부 노출 가능 패키지 (재사용성 높은 것만)
│   ├── helmutil/
│   │   ├── client.go                # Helm Action Config 팩토리
│   │   ├── install.go               # Install/Upgrade 래퍼
│   │   └── uninstall.go             # Uninstall 래퍼
│   ├── k8sutil/
│   │   ├── client.go                # kubeconfig 기반 client-go 팩토리
│   │   └── health.go                # Pod/Deployment 헬스체크 유틸
│   └── wsutil/
│       └── hub.go                   # WebSocket 연결 허브
│
├── api/
│   ├── openapi/
│   │   └── openapi.yaml             # swaggo 자동 생성 OpenAPI 3.0 스펙
│   └── proto/                       # (미래) gRPC 정의 (현재 미사용)
│
├── configs/
│   ├── config.yaml                  # 기본 설정
│   └── config.dev.yaml              # 개발 환경 오버라이드
│
├── templates/
│   ├── compatibility/
│   │   └── compatibility-matrix.yaml
│   ├── golden-paths/
│   │   ├── gitlab-allinone-v1.yaml
│   │   ├── gitlab-argocd-v1.yaml
│   │   └── github-argocd-v1.yaml
│   └── known-issues/
│       └── known-issues.yaml        # Helm edge case 패턴 DB
│
├── migrations/
│   ├── 000001_init_schema.up.sql
│   ├── 000001_init_schema.down.sql
│   └── ...
│
├── Dockerfile
├── docker-compose.yaml
├── go.mod
└── go.sum
```

---

## 3. DDD Bounded Context별 모듈 설계

### 3.1 stack 모듈

stack 모듈은 Nullus의 핵심 Bounded Context다. DevSecOps Stack 설정 생성·수정, 배포 실행, 이력 관리를 담당한다.

#### Domain Layer

**Stack Aggregate Root**

Stack은 StackConfig(도구 선택 상태)와 일련의 Deployment를 포함하는 Aggregate Root다.

```go
// internal/stack/domain/stack.go

package domain

import (
    "errors"
    "time"

    "github.com/google/uuid"
)

// StackID는 Stack의 식별자 Value Object다.
type StackID string

func NewStackID() StackID { return StackID(uuid.NewString()) }

// StackStatus는 Stack의 생명주기 상태를 나타낸다.
type StackStatus string

const (
    StackStatusDraft    StackStatus = "draft"    // 설정 중, 배포 전
    StackStatusDeployed StackStatus = "deployed" // 정상 배포 완료
    StackStatusFailed   StackStatus = "failed"   // 마지막 배포 실패
    StackStatusRolledBack StackStatus = "rolled_back"
)

// Stack은 DevSecOps Stack 설정과 배포 이력을 관리하는 Aggregate Root다.
type Stack struct {
    id            StackID
    orgID         string
    clusterID     string
    name          string
    config        StackConfig
    goldenPathID  string
    status        StackStatus
    currentVersion int
    createdAt     time.Time
    updatedAt     time.Time
}

func NewStack(orgID, clusterID, name string, config StackConfig) (*Stack, error) {
    if name == "" {
        return nil, ErrStackNameRequired
    }
    if err := config.Validate(); err != nil {
        return nil, err
    }
    return &Stack{
        id:             NewStackID(),
        orgID:          orgID,
        clusterID:      clusterID,
        name:           name,
        config:         config,
        status:         StackStatusDraft,
        currentVersion: 1,
        createdAt:      time.Now(),
        updatedAt:      time.Now(),
    }, nil
}

// UpdateConfig는 스택 설정을 변경하고 버전을 증가시킨다.
// 배포 중인 상태에서는 변경을 허용하지 않는다.
func (s *Stack) UpdateConfig(newConfig StackConfig, reason string) error {
    if err := newConfig.Validate(); err != nil {
        return err
    }
    s.config = newConfig
    s.currentVersion++
    s.updatedAt = time.Now()
    return nil
}

func (s *Stack) MarkDeployed() {
    s.status = StackStatusDeployed
    s.updatedAt = time.Now()
}

func (s *Stack) MarkFailed() {
    s.status = StackStatusFailed
    s.updatedAt = time.Now()
}

func (s *Stack) MarkRolledBack() {
    s.status = StackStatusRolledBack
    s.updatedAt = time.Now()
}

// Getters — Domain 객체의 필드는 외부에서 직접 수정 불가
func (s *Stack) ID() StackID          { return s.id }
func (s *Stack) OrgID() string        { return s.orgID }
func (s *Stack) ClusterID() string    { return s.clusterID }
func (s *Stack) Name() string         { return s.name }
func (s *Stack) Config() StackConfig  { return s.config }
func (s *Stack) Status() StackStatus  { return s.status }
func (s *Stack) Version() int         { return s.currentVersion }
```

**StackConfig Value Object**

StackConfig는 도구 선택과 리소스 설정의 불변 스냅샷이다.

```go
// internal/stack/domain/stack_config.go

package domain

import "fmt"

// ToolSelection은 단일 도구의 선택 정보다.
type ToolSelection struct {
    Tool    string `json:"tool"`    // 도구 식별자 (예: "gitlab", "argocd")
    Version string `json:"version"` // app version (예: "17.7.2")
}

// StackConfig는 스택 전체 도구 구성의 불변 Value Object다.
// 변경 시 새 StackConfig 인스턴스를 생성한다.
type StackConfig struct {
    Artifacts   ArtifactsConfig   `json:"artifacts"`
    Pipeline    PipelineConfig    `json:"pipeline"`
    Monitoring  MonitoringConfig  `json:"monitoring"`
    Logging     LoggingConfig     `json:"logging"`
    Resources   ResourcesConfig   `json:"resources"`
    ClusterID   string            `json:"cluster_id"`
    GoldenPathID string           `json:"golden_path_id,omitempty"`
}

type ArtifactsConfig struct {
    PackageRegistry    ToolSelection `json:"package_registry"`
    SourceRepository   ToolSelection `json:"source_repository"`
    ContainerRegistry  ToolSelection `json:"container_registry"`
    StorageBackend     ToolSelection `json:"storage_backend"`
}

type PipelineConfig struct {
    CIPlatform ToolSelection `json:"ci_platform"`
    CDTool     ToolSelection `json:"cd_tool"`
}

type MonitoringConfig struct {
    Collection    ToolSelection `json:"collection"`
    Visualization ToolSelection `json:"visualization"`
}

type LoggingConfig struct {
    Collection ToolSelection `json:"collection"`
    Search     ToolSelection `json:"search"`
}

type ResourcesConfig struct {
    Developers        int    `json:"developers"`
    ConcurrentRunners int    `json:"concurrent_runners"`
    WeeklyCommits     int    `json:"weekly_commits"`
    BuildFrequency    string `json:"build_frequency"` // "hourly"|"daily"|"weekly"
}

// Validate는 StackConfig의 필수 필드를 검증한다.
func (c StackConfig) Validate() error {
    if c.ClusterID == "" {
        return fmt.Errorf("%w: cluster_id is required", ErrInvalidStackConfig)
    }
    if c.Artifacts.SourceRepository.Tool == "" {
        return fmt.Errorf("%w: source_repository tool is required", ErrInvalidStackConfig)
    }
    if c.Pipeline.CDTool.Tool == "" {
        return fmt.Errorf("%w: cd_tool is required", ErrInvalidStackConfig)
    }
    return nil
}
```

**Template Value Object**

```go
// internal/stack/domain/template.go

package domain

// GoldenPathTemplate은 사전 검증된 도구 조합 템플릿의 불변 표현이다.
// 파일 시스템의 YAML에서 로드되며, 런타임에 변경되지 않는다.
type GoldenPathTemplate struct {
    ID          string      `yaml:"id"`
    Name        string      `yaml:"name"`
    Description string      `yaml:"description"`
    Status      string      `yaml:"status"` // "verified"|"experimental"|"deprecated"
    Config      StackConfig `yaml:"-"`      // YAML 파싱 후 변환
    ResourceBaseline ResourceBaseline `yaml:"resource_baseline"`
    InstallTimeMinutes int           `yaml:"install_time_minutes"`
}

type ResourceBaseline struct {
    CPUCores  float64 `yaml:"cpu_cores"`
    MemoryGi  float64 `yaml:"memory_gi"`
    StorageGi float64 `yaml:"storage_gi"`
}

// ApplyTo는 템플릿을 기반으로 새 StackConfig를 생성한다.
// 커스터마이징 오버라이드를 병합한다.
func (t GoldenPathTemplate) ApplyTo(override StackConfig) StackConfig {
    base := t.Config
    // ClusterID 등 사용자 입력 값은 override에서 가져온다
    if override.ClusterID != "" {
        base.ClusterID = override.ClusterID
    }
    base.GoldenPathID = t.ID
    return base
}
```

**설치 상태 머신**

```go
// internal/stack/domain/state_machine.go

package domain

import (
    "fmt"
    "time"
)

// DeploymentStatus는 설치 작업의 상태를 나타낸다.
type DeploymentStatus string

const (
    DeploymentStatusPending       DeploymentStatus = "PENDING"
    DeploymentStatusValidating    DeploymentStatus = "VALIDATING"
    DeploymentStatusInstalling    DeploymentStatus = "INSTALLING"
    DeploymentStatusConfiguring   DeploymentStatus = "CONFIGURING"  // Phase C: 연동 설정
    DeploymentStatusHealthCheck   DeploymentStatus = "HEALTHCHECK"
    DeploymentStatusCompleted     DeploymentStatus = "COMPLETED"
    DeploymentStatusFailed        DeploymentStatus = "FAILED"
    DeploymentStatusRollingBack   DeploymentStatus = "ROLLING_BACK"
    DeploymentStatusRolledBack    DeploymentStatus = "ROLLED_BACK"
    DeploymentStatusCancelled     DeploymentStatus = "CANCELLED"
    DeploymentStatusTimeout       DeploymentStatus = "TIMEOUT"
)

// DeploymentEvent는 상태 전이를 유발하는 이벤트다.
type DeploymentEvent string

const (
    EventStart      DeploymentEvent = "START"
    EventValidated  DeploymentEvent = "VALIDATED"
    EventInstalled  DeploymentEvent = "INSTALLED"
    EventConfigured DeploymentEvent = "CONFIGURED"
    EventHealthOK   DeploymentEvent = "HEALTH_OK"
    EventFail       DeploymentEvent = "FAIL"
    EventRollback   DeploymentEvent = "ROLLBACK"
    EventRolledBack DeploymentEvent = "ROLLED_BACK"
    EventCancel     DeploymentEvent = "CANCEL"
    EventTimeout    DeploymentEvent = "TIMEOUT"
)

// 상태 전이 테이블
// (source status, event) → target status
var transitions = map[DeploymentStatus]map[DeploymentEvent]DeploymentStatus{
    DeploymentStatusPending: {
        EventStart:  DeploymentStatusValidating,
        EventCancel: DeploymentStatusCancelled,
    },
    DeploymentStatusValidating: {
        EventValidated: DeploymentStatusInstalling,
        EventFail:      DeploymentStatusFailed,
        EventCancel:    DeploymentStatusCancelled,
        EventTimeout:   DeploymentStatusTimeout,
    },
    DeploymentStatusInstalling: {
        EventInstalled: DeploymentStatusConfiguring,
        EventFail:      DeploymentStatusFailed,
        EventCancel:    DeploymentStatusCancelled,
        EventTimeout:   DeploymentStatusTimeout,
    },
    DeploymentStatusConfiguring: {
        EventConfigured: DeploymentStatusHealthCheck,
        EventFail:       DeploymentStatusFailed,
        EventTimeout:    DeploymentStatusTimeout,
    },
    DeploymentStatusHealthCheck: {
        EventHealthOK: DeploymentStatusCompleted,
        EventFail:     DeploymentStatusFailed,
        EventTimeout:  DeploymentStatusTimeout,
    },
    DeploymentStatusFailed: {
        EventRollback: DeploymentStatusRollingBack,
    },
    DeploymentStatusTimeout: {
        EventRollback: DeploymentStatusRollingBack,
    },
    DeploymentStatusRollingBack: {
        EventRolledBack: DeploymentStatusRolledBack,
        EventFail:       DeploymentStatusFailed, // 롤백도 실패 가능
    },
}

// DeploymentStateMachine은 Deployment의 상태 전이를 관리한다.
type DeploymentStateMachine struct {
    current     DeploymentStatus
    transitions []StatusTransition
}

type StatusTransition struct {
    From      DeploymentStatus `json:"from"`
    To        DeploymentStatus `json:"to"`
    Event     DeploymentEvent  `json:"event"`
    Timestamp time.Time        `json:"timestamp"`
    Message   string           `json:"message,omitempty"`
}

func NewDeploymentStateMachine() *DeploymentStateMachine {
    return &DeploymentStateMachine{
        current: DeploymentStatusPending,
    }
}

// Transition은 이벤트를 수신하여 상태를 전이한다.
// 유효하지 않은 전이는 에러를 반환한다.
func (sm *DeploymentStateMachine) Transition(event DeploymentEvent, msg string) error {
    targets, ok := transitions[sm.current]
    if !ok {
        return fmt.Errorf("%w: no transitions from %s", ErrInvalidTransition, sm.current)
    }
    next, ok := targets[event]
    if !ok {
        return fmt.Errorf("%w: event %s not allowed in state %s",
            ErrInvalidTransition, event, sm.current)
    }
    sm.transitions = append(sm.transitions, StatusTransition{
        From:      sm.current,
        To:        next,
        Event:     event,
        Timestamp: time.Now(),
        Message:   msg,
    })
    sm.current = next
    return nil
}

func (sm *DeploymentStateMachine) Current() DeploymentStatus { return sm.current }

// IsTerminal은 현재 상태가 종료 상태인지 확인한다.
func (sm *DeploymentStateMachine) IsTerminal() bool {
    switch sm.current {
    case DeploymentStatusCompleted,
        DeploymentStatusRolledBack,
        DeploymentStatusCancelled,
        DeploymentStatusFailed:
        return true
    }
    return false
}

// IsActive는 현재 상태가 진행 중인지 확인한다 (취소 가능 여부 판단용).
func (sm *DeploymentStateMachine) IsActive() bool {
    switch sm.current {
    case DeploymentStatusPending,
        DeploymentStatusValidating,
        DeploymentStatusInstalling,
        DeploymentStatusConfiguring,
        DeploymentStatusHealthCheck:
        return true
    }
    return false
}
```

**도메인 에러**

```go
// internal/stack/domain/errors.go

package domain

import "errors"

var (
    ErrStackNotFound      = errors.New("stack not found")
    ErrStackNameRequired  = errors.New("stack name is required")
    ErrInvalidStackConfig = errors.New("invalid stack config")
    ErrInvalidTransition  = errors.New("invalid state transition")
    ErrDeploymentNotFound = errors.New("deployment not found")
    ErrDeploymentActive   = errors.New("deployment is already active")
)
```

---

#### Port Layer

```go
// internal/stack/port/repository.go

package port

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
)

// StackRepository는 Stack Aggregate의 영속성 포트다.
type StackRepository interface {
    Save(ctx context.Context, stack *domain.Stack) error
    FindByID(ctx context.Context, id domain.StackID) (*domain.Stack, error)
    FindByOrgID(ctx context.Context, orgID string) ([]*domain.Stack, error)
    Update(ctx context.Context, stack *domain.Stack) error
    Delete(ctx context.Context, id domain.StackID) error

    // 버전 스냅샷 관리
    SaveVersion(ctx context.Context, snapshot StackVersionSnapshot) error
    FindVersions(ctx context.Context, stackID domain.StackID) ([]StackVersionSnapshot, error)
    FindVersionByNumber(ctx context.Context, stackID domain.StackID, version int) (*StackVersionSnapshot, error)
}

// StackVersionSnapshot은 특정 시점의 스택 설정 스냅샷이다.
type StackVersionSnapshot struct {
    StackID       domain.StackID
    VersionNumber int
    ConfigSnapshot domain.StackConfig
    ChangeReason  string
    ChangedBy     string
    CreatedAt     interface{} // time.Time
}

// DeploymentRepository는 Deployment의 영속성 포트다.
type DeploymentRepository interface {
    Save(ctx context.Context, deployment *domain.Deployment) error
    FindByID(ctx context.Context, id string) (*domain.Deployment, error)
    FindByStackID(ctx context.Context, stackID domain.StackID) ([]*domain.Deployment, error)
    UpdateStatus(ctx context.Context, id string, status domain.DeploymentStatus, msg string) error
    AppendLog(ctx context.Context, deploymentID, stepName, level, message string) error
}
```

```go
// internal/stack/port/installer.go

package port

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
)

// HelmInstaller는 Helm 기반 설치 작업의 포트다.
// Install Engine의 핵심 포트로, 구현체는 adapter/helm에 위치한다.
type HelmInstaller interface {
    // Install은 단일 Helm 차트를 설치한다.
    // KnownIssueHandler를 통해 edge case를 자동 처리한다.
    Install(ctx context.Context, req HelmInstallRequest) error

    // Uninstall은 Helm 릴리스를 제거한다 (롤백용).
    Uninstall(ctx context.Context, req HelmUninstallRequest) error

    // GetReleaseStatus는 설치된 릴리스의 상태를 조회한다.
    GetReleaseStatus(ctx context.Context, namespace, releaseName string) (*HelmReleaseStatus, error)
}

type HelmInstallRequest struct {
    ReleaseName string
    ChartRepo   string
    ChartName   string
    ChartVersion string
    Namespace   string
    Values      map[string]interface{}
    // known-issues 처리 플래그
    ServerSideApply bool // CRD 262KB 초과 시 true
    SkipWait        bool // 비핵심 앱은 --wait 제거
    TimeoutSeconds  int  // 기본 600
}

type HelmUninstallRequest struct {
    ReleaseName    string
    Namespace      string
    DeletePVCs     bool // destructive 모드에서만 true
}

type HelmReleaseStatus struct {
    Name      string
    Namespace string
    Status    string // "deployed"|"failed"|"pending-install" 등
    Revision  int
}

// LogStreamer는 설치 로그를 클라이언트로 스트리밍하는 포트다.
type LogStreamer interface {
    // Stream은 deploymentID에 대한 로그 채널을 반환한다.
    // 로그 메시지는 JSON 직렬화되어 WebSocket으로 전송된다.
    Stream(ctx context.Context, deploymentID string) (<-chan LogEntry, error)

    // Publish는 새 로그 항목을 발행한다.
    Publish(ctx context.Context, deploymentID string, entry LogEntry) error
}

type LogEntry struct {
    DeploymentID string `json:"deployment_id"`
    StepName     string `json:"step_name"`
    Phase        string `json:"phase"` // "A"|"B"|"C"
    Level        string `json:"level"` // "info"|"warn"|"error"
    Message      string `json:"message"`
    Timestamp    int64  `json:"timestamp"` // Unix milliseconds
}
```

---

#### UseCase Layer

```go
// internal/stack/usecase/install_stack.go

package usecase

import (
    "context"
    "fmt"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
    "github.com/cloud-nullus/nullus/internal/stack/port"
)

// InstallStackUseCase는 스택 배포 실행의 유스케이스다.
// Orchestrator에게 비동기 설치를 위임하고 Deployment ID를 즉시 반환한다.
type InstallStackUseCase struct {
    stackRepo   port.StackRepository
    deployRepo  port.DeploymentRepository
    orchestrator InstallOrchestrator
}

// InstallOrchestrator는 설치 엔진의 진입점 포트다.
// 실제 구현은 Install Engine(internal/engine)에 위치한다.
type InstallOrchestrator interface {
    RunAsync(ctx context.Context, deployment *domain.Deployment, config domain.StackConfig) error
}

type InstallStackInput struct {
    StackID   domain.StackID
    RequestBy string // 사용자 ID
}

type InstallStackOutput struct {
    DeploymentID string
    Status       domain.DeploymentStatus
}

func NewInstallStackUseCase(
    stackRepo port.StackRepository,
    deployRepo port.DeploymentRepository,
    orchestrator InstallOrchestrator,
) *InstallStackUseCase {
    return &InstallStackUseCase{
        stackRepo:    stackRepo,
        deployRepo:   deployRepo,
        orchestrator: orchestrator,
    }
}

func (uc *InstallStackUseCase) Execute(ctx context.Context, input InstallStackInput) (*InstallStackOutput, error) {
    stack, err := uc.stackRepo.FindByID(ctx, input.StackID)
    if err != nil {
        return nil, fmt.Errorf("find stack: %w", err)
    }

    // 이미 진행 중인 배포가 있는지 확인
    existing, err := uc.deployRepo.FindByStackID(ctx, input.StackID)
    if err != nil {
        return nil, fmt.Errorf("find deployments: %w", err)
    }
    for _, d := range existing {
        if d.IsActive() {
            return nil, domain.ErrDeploymentActive
        }
    }

    deployment := domain.NewDeployment(string(input.StackID), input.RequestBy)
    if err := uc.deployRepo.Save(ctx, deployment); err != nil {
        return nil, fmt.Errorf("save deployment: %w", err)
    }

    // 비동기 실행: Deploy 버튼 클릭 후 10초 내 시작 보장
    if err := uc.orchestrator.RunAsync(ctx, deployment, stack.Config()); err != nil {
        return nil, fmt.Errorf("start orchestrator: %w", err)
    }

    return &InstallStackOutput{
        DeploymentID: deployment.ID(),
        Status:       deployment.Status(),
    }, nil
}
```

```go
// internal/stack/usecase/list_stacks.go

package usecase

import (
    "context"
    "fmt"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
    "github.com/cloud-nullus/nullus/internal/stack/port"
)

type ListStacksUseCase struct {
    stackRepo port.StackRepository
}

type ListStacksInput struct {
    OrgID string
}

type ListStacksOutput struct {
    Stacks []*domain.Stack
}

func NewListStacksUseCase(stackRepo port.StackRepository) *ListStacksUseCase {
    return &ListStacksUseCase{stackRepo: stackRepo}
}

func (uc *ListStacksUseCase) Execute(ctx context.Context, input ListStacksInput) (*ListStacksOutput, error) {
    stacks, err := uc.stackRepo.FindByOrgID(ctx, input.OrgID)
    if err != nil {
        return nil, fmt.Errorf("list stacks: %w", err)
    }
    return &ListStacksOutput{Stacks: stacks}, nil
}
```

```go
// internal/stack/usecase/rollback_stack.go

package usecase

import (
    "context"
    "fmt"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
    "github.com/cloud-nullus/nullus/internal/stack/port"
)

// RollbackStackUseCase는 특정 버전으로 스택을 롤백하는 유스케이스다.
// Alpha: 전체 롤백(FULL)만 지원. Beta+: 부분 롤백 추가.
type RollbackStackUseCase struct {
    stackRepo    port.StackRepository
    deployRepo   port.DeploymentRepository
    orchestrator InstallOrchestrator
}

type RollbackStackInput struct {
    StackID       domain.StackID
    TargetVersion int    // 0이면 마지막 성공 버전으로 롤백
    RequestBy     string
    Mode          RollbackMode
}

// RollbackMode는 롤백 방식을 결정한다 (PRD 롤백 전략 참조).
type RollbackMode string

const (
    RollbackModeFull    RollbackMode = "FULL"    // 전체 롤백 (Alpha)
    RollbackModePartial RollbackMode = "PARTIAL" // 부분 롤백 (v1.0)
)

type RollbackStackOutput struct {
    DeploymentID string
    Status       domain.DeploymentStatus
}

func (uc *RollbackStackUseCase) Execute(ctx context.Context, input RollbackStackInput) (*RollbackStackOutput, error) {
    stack, err := uc.stackRepo.FindByID(ctx, input.StackID)
    if err != nil {
        return nil, fmt.Errorf("find stack: %w", err)
    }

    targetVersion := input.TargetVersion
    if targetVersion == 0 {
        targetVersion = stack.Version() - 1
    }
    if targetVersion < 1 {
        return nil, fmt.Errorf("no previous version to roll back to")
    }

    snapshot, err := uc.stackRepo.FindVersionByNumber(ctx, input.StackID, targetVersion)
    if err != nil {
        return nil, fmt.Errorf("find version %d: %w", targetVersion, err)
    }

    // 롤백도 새 Deployment로 추적한다
    deployment := domain.NewRollbackDeployment(string(input.StackID), input.RequestBy, targetVersion)
    if err := uc.deployRepo.Save(ctx, deployment); err != nil {
        return nil, fmt.Errorf("save rollback deployment: %w", err)
    }

    if err := uc.orchestrator.RunAsync(ctx, deployment, snapshot.ConfigSnapshot); err != nil {
        return nil, fmt.Errorf("start rollback orchestrator: %w", err)
    }

    return &RollbackStackOutput{
        DeploymentID: deployment.ID(),
        Status:       deployment.Status(),
    }, nil
}
```

---

#### Adapter Layer (핵심 부분)

```go
// internal/stack/adapter/helm/installer.go

package helm

import (
    "context"
    "fmt"
    "os"

    "helm.sh/helm/v3/pkg/action"
    "helm.sh/helm/v3/pkg/chart/loader"
    "helm.sh/helm/v3/pkg/cli"
    "helm.sh/helm/v3/pkg/repo"
    "k8s.io/client-go/rest"

    "github.com/cloud-nullus/nullus/internal/stack/port"
    "github.com/cloud-nullus/nullus/internal/stack/adapter/helm/knownissues"
)

// HelmInstallerAdapter는 Helm Go SDK를 사용하는 HelmInstaller 구현체다.
type HelmInstallerAdapter struct {
    restConfig     *rest.Config
    knownIssueProc *knownissues.Processor
    settings       *cli.EnvSettings
}

func NewHelmInstallerAdapter(restConfig *rest.Config, knownIssuePath string) (*HelmInstallerAdapter, error) {
    proc, err := knownissues.NewProcessor(knownIssuePath)
    if err != nil {
        return nil, fmt.Errorf("load known-issues: %w", err)
    }
    return &HelmInstallerAdapter{
        restConfig:     restConfig,
        knownIssueProc: proc,
        settings:       cli.New(),
    }, nil
}

func (a *HelmInstallerAdapter) Install(ctx context.Context, req port.HelmInstallRequest) error {
    // known-issues.yaml에서 이 차트에 대한 패치를 적용한다
    req = a.knownIssueProc.Apply(req)

    cfg, err := a.newActionConfig(req.Namespace)
    if err != nil {
        return fmt.Errorf("create action config: %w", err)
    }

    // 이미 설치되어 있으면 Upgrade, 없으면 Install
    histClient := action.NewHistory(cfg)
    histClient.Max = 1
    if _, err := histClient.Run(req.ReleaseName); err == nil {
        return a.upgrade(ctx, cfg, req)
    }
    return a.install(ctx, cfg, req)
}

func (a *HelmInstallerAdapter) install(ctx context.Context, cfg *action.Configuration, req port.HelmInstallRequest) error {
    client := action.NewInstall(cfg)
    client.ReleaseName = req.ReleaseName
    client.Namespace = req.Namespace
    client.Version = req.ChartVersion
    client.Wait = !req.SkipWait
    client.Timeout = helmTimeout(req.TimeoutSeconds)

    // CRD 크기 초과 시 server-side apply 사용
    if req.ServerSideApply {
        client.UseReleaseName = true // server-side apply 플래그 활성화
    }

    chart, err := a.loadChart(req)
    if err != nil {
        return err
    }

    if _, err := client.RunWithContext(ctx, chart, req.Values); err != nil {
        return fmt.Errorf("helm install %s: %w", req.ReleaseName, err)
    }
    return nil
}

func (a *HelmInstallerAdapter) Uninstall(ctx context.Context, req port.HelmUninstallRequest) error {
    cfg, err := a.newActionConfig(req.Namespace)
    if err != nil {
        return fmt.Errorf("create action config: %w", err)
    }
    client := action.NewUninstall(cfg)
    client.KeepHistory = false

    if _, err := client.Run(req.ReleaseName); err != nil {
        return fmt.Errorf("helm uninstall %s: %w", req.ReleaseName, err)
    }

    if req.DeletePVCs {
        // destructive 모드: PVC 삭제 (별도 K8s API 호출)
        return a.deletePVCs(ctx, req.Namespace, req.ReleaseName)
    }
    return nil
}

func (a *HelmInstallerAdapter) GetReleaseStatus(ctx context.Context, namespace, releaseName string) (*port.HelmReleaseStatus, error) {
    cfg, err := a.newActionConfig(namespace)
    if err != nil {
        return nil, err
    }
    client := action.NewStatus(cfg)
    rel, err := client.Run(releaseName)
    if err != nil {
        return nil, err
    }
    return &port.HelmReleaseStatus{
        Name:      rel.Name,
        Namespace: rel.Namespace,
        Status:    rel.Info.Status.String(),
        Revision:  rel.Version,
    }, nil
}

func (a *HelmInstallerAdapter) newActionConfig(namespace string) (*action.Configuration, error) {
    cfg := new(action.Configuration)
    if err := cfg.Init(
        // restClientGetter는 kubeconfig 기반 팩토리
        newRESTClientGetter(a.restConfig, namespace),
        namespace,
        os.Getenv("HELM_DRIVER"), // 기본값: "secret"
        func(format string, v ...interface{}) {
            // Helm 내부 로그를 slog로 라우팅
        },
    ); err != nil {
        return nil, fmt.Errorf("helm action config init: %w", err)
    }
    return cfg, nil
}
```

---

### 3.2 admin 모듈

admin 모듈은 Organization, User, Cluster의 등록과 관리를 담당한다.

#### Domain Layer

**Organization Aggregate Root**

```go
// internal/admin/domain/organization.go

package domain

import (
    "errors"
    "regexp"
    "time"

    "github.com/google/uuid"
)

// OrgStatus는 Organization의 활성 상태다.
type OrgStatus string

const (
    OrgStatusActive   OrgStatus = "active"
    OrgStatusInactive OrgStatus = "inactive"
)

// Organization은 Nullus의 최상위 테넌트 단위다.
// 모든 클러스터, 스택, 사용자는 Organization에 속한다.
type Organization struct {
    id        string
    name      string
    slug      string  // URL 안전 식별자, 고유
    domain    string  // 이메일 도메인 (선택)
    status    OrgStatus
    adminID   string
    createdAt time.Time
    updatedAt time.Time
}

var slugRegex = regexp.MustCompile(`^[a-z0-9-]{3,50}$`)

func NewOrganization(name, slug, adminID string) (*Organization, error) {
    if name == "" {
        return nil, ErrOrgNameRequired
    }
    if !slugRegex.MatchString(slug) {
        return nil, ErrInvalidOrgSlug
    }
    if adminID == "" {
        return nil, ErrAdminIDRequired
    }
    return &Organization{
        id:        uuid.NewString(),
        name:      name,
        slug:      slug,
        adminID:   adminID,
        status:    OrgStatusActive,
        createdAt: time.Now(),
        updatedAt: time.Now(),
    }, nil
}

func (o *Organization) Deactivate() error {
    if o.status == OrgStatusInactive {
        return errors.New("organization is already inactive")
    }
    o.status = OrgStatusInactive
    o.updatedAt = time.Now()
    return nil
}

func (o *Organization) Activate() {
    o.status = OrgStatusActive
    o.updatedAt = time.Now()
}

func (o *Organization) ChangeAdmin(newAdminID string) error {
    if newAdminID == "" {
        return ErrAdminIDRequired
    }
    o.adminID = newAdminID
    o.updatedAt = time.Now()
    return nil
}

func (o *Organization) ID() string       { return o.id }
func (o *Organization) Name() string     { return o.name }
func (o *Organization) Slug() string     { return o.slug }
func (o *Organization) Status() OrgStatus { return o.status }
func (o *Organization) AdminID() string  { return o.adminID }
```

**User Entity**

```go
// internal/admin/domain/user.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// Role은 Nullus의 3역할 체계 (PRD v1.3 proto4 확정).
type Role string

const (
    RoleAdmin          Role = "admin"           // Organization 관리 전체 권한
    RoleDevOpsEngineer Role = "devops_engineer" // 스택/클러스터 설정·배포
    RoleDeveloper      Role = "developer"       // CI/CD, 관측성 접근
)

// User는 Nullus 플랫폼 사용자 Entity다.
type User struct {
    id           string
    email        string
    passwordHash string
    displayName  string
    isActive     bool
    createdAt    time.Time
    updatedAt    time.Time
}

func NewUser(email, passwordHash, displayName string) (*User, error) {
    if email == "" {
        return nil, ErrEmailRequired
    }
    return &User{
        id:           uuid.NewString(),
        email:        email,
        passwordHash: passwordHash,
        displayName:  displayName,
        isActive:     true,
        createdAt:    time.Now(),
        updatedAt:    time.Now(),
    }, nil
}

func (u *User) Deactivate() { u.isActive = false; u.updatedAt = time.Now() }
func (u *User) Activate()   { u.isActive = true; u.updatedAt = time.Now() }

func (u *User) ID() string           { return u.id }
func (u *User) Email() string        { return u.email }
func (u *User) PasswordHash() string { return u.passwordHash }
func (u *User) DisplayName() string  { return u.displayName }
func (u *User) IsActive() bool       { return u.isActive }

// OrgMembership은 사용자의 Organization 내 역할을 나타내는 Value Object다.
type OrgMembership struct {
    OrgID     string
    UserID    string
    Role      Role
    JoinedAt  time.Time
}
```

**Cluster Entity**

```go
// internal/admin/domain/cluster.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// ClusterType은 클러스터의 용도를 구분한다.
type ClusterType string

const (
    ClusterTypePipeline ClusterType = "pipeline" // CI/CD 도구가 실행되는 클러스터
    ClusterTypeTarget   ClusterType = "target"   // 애플리케이션이 배포되는 클러스터
)

// ClusterStatus는 클러스터 연결 상태다.
type ClusterStatus string

const (
    ClusterStatusConnected   ClusterStatus = "connected"
    ClusterStatusPending     ClusterStatus = "pending"      // 검증 전
    ClusterStatusUnreachable ClusterStatus = "unreachable"
    ClusterStatusAuthFailed  ClusterStatus = "auth_failed"
)

// Cluster는 Nullus가 관리하는 Kubernetes 클러스터 Entity다.
// kubeconfig는 AES-256-GCM으로 암호화하여 DB에 저장한다.
type Cluster struct {
    id               string
    orgID            string
    name             string
    clusterType      ClusterType
    kubeconfigEnc    []byte       // AES-256-GCM 암호화된 kubeconfig
    endpoint         string       // K8s API Server 주소
    authMethod       string       // "kubeconfig"|"serviceaccount"
    namespace        string       // Nullus 시스템 네임스페이스
    status           ClusterStatus
    k8sVersion       string
    lastVerifiedAt   *time.Time
    createdAt        time.Time
    updatedAt        time.Time
}

func NewCluster(orgID, name string, clusterType ClusterType, kubeconfigEnc []byte) (*Cluster, error) {
    if name == "" {
        return nil, ErrClusterNameRequired
    }
    if len(kubeconfigEnc) == 0 {
        return nil, ErrKubeconfigRequired
    }
    return &Cluster{
        id:            uuid.NewString(),
        orgID:         orgID,
        name:          name,
        clusterType:   clusterType,
        kubeconfigEnc: kubeconfigEnc,
        namespace:     "nullus-system",
        status:        ClusterStatusPending,
        authMethod:    "kubeconfig",
        createdAt:     time.Now(),
        updatedAt:     time.Now(),
    }, nil
}

// MarkConnected는 연결 검증 성공 후 상태를 업데이트한다.
func (c *Cluster) MarkConnected(k8sVersion string) {
    now := time.Now()
    c.status = ClusterStatusConnected
    c.k8sVersion = k8sVersion
    c.lastVerifiedAt = &now
    c.updatedAt = now
}

func (c *Cluster) MarkUnreachable() {
    c.status = ClusterStatusUnreachable
    c.updatedAt = time.Now()
}

func (c *Cluster) MarkAuthFailed() {
    c.status = ClusterStatusAuthFailed
    c.updatedAt = time.Now()
}

func (c *Cluster) ID() string               { return c.id }
func (c *Cluster) OrgID() string            { return c.orgID }
func (c *Cluster) Name() string             { return c.name }
func (c *Cluster) Type() ClusterType        { return c.clusterType }
func (c *Cluster) KubeconfigEnc() []byte    { return c.kubeconfigEnc }
func (c *Cluster) Endpoint() string         { return c.endpoint }
func (c *Cluster) Status() ClusterStatus    { return c.status }
func (c *Cluster) K8sVersion() string       { return c.k8sVersion }
func (c *Cluster) LastVerifiedAt() *time.Time { return c.lastVerifiedAt }
```

**Invite Value Object**

```go
// internal/admin/domain/invite.go

package domain

import (
    "crypto/rand"
    "encoding/hex"
    "time"
)

// Invite는 Organization 멤버 초대 토큰 Value Object다.
// 토큰은 32바이트 랜덤 hex 문자열이다.
type Invite struct {
    Token     string
    OrgID     string
    Role      Role
    CreatedBy string
    ExpiresAt time.Time
    UsedAt    *time.Time
}

func NewInvite(orgID string, role Role, createdBy string, ttl time.Duration) (*Invite, error) {
    token, err := generateToken()
    if err != nil {
        return nil, err
    }
    return &Invite{
        Token:     token,
        OrgID:     orgID,
        Role:      role,
        CreatedBy: createdBy,
        ExpiresAt: time.Now().Add(ttl),
    }, nil
}

func (i *Invite) IsExpired() bool {
    return time.Now().After(i.ExpiresAt)
}

func (i *Invite) IsUsed() bool {
    return i.UsedAt != nil
}

func (i *Invite) Accept(userID string) error {
    if i.IsExpired() {
        return ErrInviteExpired
    }
    if i.IsUsed() {
        return ErrInviteAlreadyUsed
    }
    now := time.Now()
    i.UsedAt = &now
    return nil
}

func generateToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return hex.EncodeToString(b), nil
}
```

---

#### Port Layer (admin)

```go
// internal/admin/port/repository.go

package port

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/admin/domain"
)

type OrgRepository interface {
    Save(ctx context.Context, org *domain.Organization) error
    FindByID(ctx context.Context, id string) (*domain.Organization, error)
    FindBySlug(ctx context.Context, slug string) (*domain.Organization, error)
    Update(ctx context.Context, org *domain.Organization) error
    ExistsBySlug(ctx context.Context, slug string) (bool, error)
}

type UserRepository interface {
    Save(ctx context.Context, user *domain.User) error
    FindByID(ctx context.Context, id string) (*domain.User, error)
    FindByEmail(ctx context.Context, email string) (*domain.User, error)
    Update(ctx context.Context, user *domain.User) error
    ExistsByEmail(ctx context.Context, email string) (bool, error)

    // Membership 관리
    SaveMembership(ctx context.Context, m domain.OrgMembership) error
    FindMemberships(ctx context.Context, orgID string) ([]domain.OrgMembership, error)
    FindMembership(ctx context.Context, orgID, userID string) (*domain.OrgMembership, error)
    UpdateMembershipRole(ctx context.Context, orgID, userID string, role domain.Role) error
    DeleteMembership(ctx context.Context, orgID, userID string) error
}

type ClusterRepository interface {
    Save(ctx context.Context, cluster *domain.Cluster) error
    FindByID(ctx context.Context, id string) (*domain.Cluster, error)
    FindByOrgID(ctx context.Context, orgID string) ([]*domain.Cluster, error)
    Update(ctx context.Context, cluster *domain.Cluster) error
    Delete(ctx context.Context, id string) error
    ExistsByNameInOrg(ctx context.Context, orgID, name string) (bool, error)
}

type InviteRepository interface {
    Save(ctx context.Context, invite *domain.Invite) error
    FindByToken(ctx context.Context, token string) (*domain.Invite, error)
    MarkUsed(ctx context.Context, token string) error
}
```

```go
// internal/admin/port/k8s_client.go

package port

import (
    "context"
)

// K8sConnector는 kubeconfig 기반 Kubernetes 클러스터 연결 검증 포트다.
type K8sConnector interface {
    // VerifyConnection은 kubeconfig(복호화된)로 K8s API Server에 접속을 시도한다.
    // 성공 시 K8s 버전 문자열을 반환한다.
    VerifyConnection(ctx context.Context, kubeconfigBytes []byte) (k8sVersion string, err error)

    // ListNamespaces는 연결 검증 후 네임스페이스 목록을 반환한다.
    ListNamespaces(ctx context.Context, kubeconfigBytes []byte) ([]string, error)
}
```

---

#### UseCase Layer (admin)

```go
// internal/admin/usecase/register_cluster.go

package usecase

import (
    "context"
    "fmt"

    "github.com/cloud-nullus/nullus/internal/admin/domain"
    "github.com/cloud-nullus/nullus/internal/admin/port"
    "github.com/cloud-nullus/nullus/internal/shared/crypto"
)

// RegisterClusterUseCase는 클러스터 등록 유스케이스다.
// kubeconfig를 AES-256-GCM으로 암호화하여 저장한다.
type RegisterClusterUseCase struct {
    clusterRepo port.ClusterRepository
    connector   port.K8sConnector
    crypto      crypto.AESEncryptor
}

type RegisterClusterInput struct {
    OrgID          string
    Name           string
    ClusterType    domain.ClusterType
    KubeconfigRaw  []byte // 평문 kubeconfig (업로드된 파일)
    AutoVerify     bool   // 등록과 동시에 연결 검증 수행 여부
}

type RegisterClusterOutput struct {
    ClusterID  string
    Status     domain.ClusterStatus
    K8sVersion string // AutoVerify=true인 경우에만 채워짐
}

func NewRegisterClusterUseCase(
    clusterRepo port.ClusterRepository,
    connector port.K8sConnector,
    enc crypto.AESEncryptor,
) *RegisterClusterUseCase {
    return &RegisterClusterUseCase{
        clusterRepo: clusterRepo,
        connector:   connector,
        crypto:      enc,
    }
}

func (uc *RegisterClusterUseCase) Execute(ctx context.Context, input RegisterClusterInput) (*RegisterClusterOutput, error) {
    // 중복 이름 검증
    exists, err := uc.clusterRepo.ExistsByNameInOrg(ctx, input.OrgID, input.Name)
    if err != nil {
        return nil, fmt.Errorf("check duplicate: %w", err)
    }
    if exists {
        return nil, domain.ErrClusterNameDuplicate
    }

    // kubeconfig AES-256-GCM 암호화
    encrypted, err := uc.crypto.Encrypt(input.KubeconfigRaw)
    if err != nil {
        return nil, fmt.Errorf("encrypt kubeconfig: %w", err)
    }

    cluster, err := domain.NewCluster(input.OrgID, input.Name, input.ClusterType, encrypted)
    if err != nil {
        return nil, err
    }

    out := &RegisterClusterOutput{}

    if input.AutoVerify {
        // 연결 검증: 평문 kubeconfig는 메모리에서만 사용, DB에 저장하지 않음
        k8sVer, verifyErr := uc.connector.VerifyConnection(ctx, input.KubeconfigRaw)
        if verifyErr != nil {
            cluster.MarkUnreachable()
            out.Status = cluster.Status()
        } else {
            cluster.MarkConnected(k8sVer)
            out.K8sVersion = k8sVer
            out.Status = cluster.Status()
        }
    }

    if err := uc.clusterRepo.Save(ctx, cluster); err != nil {
        return nil, fmt.Errorf("save cluster: %w", err)
    }

    out.ClusterID = cluster.ID()
    if out.Status == "" {
        out.Status = cluster.Status()
    }
    return out, nil
}
```

```go
// internal/admin/usecase/invite_member.go

package usecase

import (
    "context"
    "fmt"
    "time"

    "github.com/cloud-nullus/nullus/internal/admin/domain"
    "github.com/cloud-nullus/nullus/internal/admin/port"
)

// InviteMemberUseCase는 Organization 멤버 초대 유스케이스다.
type InviteMemberUseCase struct {
    orgRepo    port.OrgRepository
    inviteRepo port.InviteRepository
    userRepo   port.UserRepository
}

const defaultInviteTTL = 7 * 24 * time.Hour

type InviteMemberInput struct {
    OrgID     string
    Role      domain.Role
    CreatedBy string      // 초대를 생성한 Admin 사용자 ID
    TTL       time.Duration // 0이면 기본값 7일 적용
}

type InviteMemberOutput struct {
    Token     string
    ExpiresAt time.Time
    InviteURL string // 프론트엔드가 구성할 URL 접두사는 설정에서 주입
}

func NewInviteMemberUseCase(
    orgRepo port.OrgRepository,
    inviteRepo port.InviteRepository,
    userRepo port.UserRepository,
) *InviteMemberUseCase {
    return &InviteMemberUseCase{
        orgRepo:    orgRepo,
        inviteRepo: inviteRepo,
        userRepo:   userRepo,
    }
}

func (uc *InviteMemberUseCase) Execute(ctx context.Context, input InviteMemberInput) (*InviteMemberOutput, error) {
    // Organization 존재 확인
    org, err := uc.orgRepo.FindByID(ctx, input.OrgID)
    if err != nil {
        return nil, fmt.Errorf("find org: %w", err)
    }
    if org.Status() != domain.OrgStatusActive {
        return nil, domain.ErrOrgInactive
    }

    ttl := input.TTL
    if ttl == 0 {
        ttl = defaultInviteTTL
    }

    invite, err := domain.NewInvite(input.OrgID, input.Role, input.CreatedBy, ttl)
    if err != nil {
        return nil, fmt.Errorf("create invite: %w", err)
    }

    if err := uc.inviteRepo.Save(ctx, invite); err != nil {
        return nil, fmt.Errorf("save invite: %w", err)
    }

    return &InviteMemberOutput{
        Token:     invite.Token,
        ExpiresAt: invite.ExpiresAt,
        InviteURL: fmt.Sprintf("/invites/%s", invite.Token),
    }, nil
}
```

---

## 4. Install Engine 상세 설계

Install Engine은 stack 모듈의 `InstallOrchestrator` 포트를 구현하는 핵심 컴포넌트다. `internal/engine` 패키지에 위치하며, goroutine 기반 비동기 실행과 DAG 기반 설치 순서를 제공한다.

### 4.1 상태 머신 전체 전이도

```
                     ┌─────────┐
                     │ PENDING │
                     └────┬────┘
               START ─────┘
                          │
                     ┌────▼────────┐
                     │ VALIDATING  │ ← 설정 유효성, 클러스터 연결, 호환성 매트릭스 검증
                     └────┬────────┘
          VALIDATED ──────┘   └── FAIL ──────┐
                                              │
                     ┌────▼────────┐         │
                     │ INSTALLING  │ ← Phase A, B, C 순차 실행
                     └────┬────────┘         │
         INSTALLED ───────┘   └── FAIL ──────┤
                                              │
                     ┌────▼────────┐         │
                     │ CONFIGURING │ ← Phase C: 연동 설정 (OIDC, Webhook, ServiceMonitor)
                     └────┬────────┘         │
        CONFIGURED ───────┘   └── FAIL ──────┤
                                              │
                     ┌────▼────────┐         │
                     │ HEALTHCHECK │ ← 120+ 항목 헬스체크 (Narwhal 패턴)
                     └────┬────────┘         │
         HEALTH_OK ───────┘   └── FAIL ──────┤
                                              │
                     ┌────▼────────┐         │
                     │  COMPLETED  │◄─────────┘──── (최종 상태)
                     └─────────────┘         │
                                             │
                              ┌──────────────▼──────┐
                              │       FAILED        │ ← 에러 기록, 롤백 여부 결정
                              └──────┬──────────────┘
                      ROLLBACK ──────┘
                                     │
                     ┌───────────────▼─────┐
                     │    ROLLING_BACK      │ ← 역순 Helm uninstall
                     └──────┬──────────────┘
           ROLLED_BACK ─────┘   └── FAIL
                     │
                     ▼
              ┌──────────────┐
              │  ROLLED_BACK │ (최종 상태)
              └──────────────┘

  CANCEL은 PENDING, VALIDATING, INSTALLING, CONFIGURING, HEALTHCHECK 상태에서 가능
  TIMEOUT은 모든 active 상태에서 발생 가능 → ROLLING_BACK 전이
```

### 4.2 DAG 기반 설치 순서

설치 단계는 방향 비순환 그래프(DAG)로 정의된다. 각 노드는 독립 goroutine으로 실행되며, 선행 노드 완료 후 다음 노드가 실행된다.

```
Phase A: 기반 인프라 (병렬 가능한 것은 동시 실행)
─────────────────────────────────────────────
  [A1] cert-manager ──────────────────────────────────────────┐
  [A2] MetalLB (LoadBalancer) ────────────────────────────────┤
  [A3] Traefik (Ingress) ← depends on A1 (TLS), A2 (LB IP)  ─┤
  [A4] CNPG Operator ─────────────────────────────────────────┤
  [A5] MinIO ─────────────────────────────────────────────────┘
                                 ↓ Phase A Gate Check

Phase B: 핵심 서비스 (Phase A 완료 필수)
────────────────────────────────────────
  [B1] GitLab CE ← depends on A3, A4(CNPG), A5(MinIO)
  [B2] Harbor    ← depends on A3, A5(MinIO)   (optional, GitLab Registry 사용 시 skip)
  [B3] Argo CD   ← depends on A3
  [B4] GitLab Runner ← depends on B1
                                 ↓ Phase B Gate Check

Phase C: 보조 서비스 + 연동 (Phase B 완료 필수)
───────────────────────────────────────────────
  [C1] kube-prometheus-stack (Prometheus + Grafana) ← depends on A3
  [C2] Loki ← depends on A5(MinIO), A3
  [C3] OpenTelemetry Collector ← depends on C1, C2
  [C4] OpenSearch ← depends on A3                  (alternative to Loki/OTel)
  [C5] Keycloak OIDC 자동 설정 ← depends on B1, B2, B3, C1
       └─ realm 생성 → groups scope → 7-app 클라이언트 → K8s API OIDC 연동
  [C6] GitLab-ArgoCD Webhook 연결 ← depends on B1, B3
  [C7] Prometheus ServiceMonitor 등록 ← depends on B1, B2, B3, C1
```

### 4.3 Orchestrator 구현

```go
// internal/engine/orchestrator.go

package engine

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
    "github.com/cloud-nullus/nullus/internal/stack/port"
    "github.com/cloud-nullus/nullus/internal/engine/dag"
    "github.com/cloud-nullus/nullus/internal/engine/phase"
)

// Orchestrator는 InstallOrchestrator 포트의 구현체다.
// 고루틴을 통한 비동기 실행과 상태 머신 관리를 담당한다.
type Orchestrator struct {
    deployRepo  port.DeploymentRepository
    installer   port.HelmInstaller
    logStreamer  port.LogStreamer
    dagBuilder  *dag.Builder
    timeout     time.Duration
}

func NewOrchestrator(
    deployRepo port.DeploymentRepository,
    installer port.HelmInstaller,
    logStreamer port.LogStreamer,
) *Orchestrator {
    return &Orchestrator{
        deployRepo: deployRepo,
        installer:  installer,
        logStreamer: logStreamer,
        dagBuilder: dag.NewBuilder(),
        timeout:    2 * time.Hour, // PRD 요구사항: < 2시간
    }
}

// RunAsync는 비동기로 설치를 시작한다.
// PRD 요구사항: Deploy 버튼 클릭 후 10초 내 배포 시작 보장.
func (o *Orchestrator) RunAsync(ctx context.Context, deployment *domain.Deployment, config domain.StackConfig) error {
    // 별도 goroutine으로 실행 — 컨텍스트는 새로 생성 (HTTP 요청 컨텍스트와 분리)
    installCtx, cancel := context.WithTimeout(context.Background(), o.timeout)
    go func() {
        defer cancel()
        if err := o.run(installCtx, deployment, config); err != nil {
            slog.Error("install engine error", "deployment_id", deployment.ID(), "error", err)
        }
    }()
    return nil
}

func (o *Orchestrator) run(ctx context.Context, deployment *domain.Deployment, config domain.StackConfig) error {
    sm := domain.NewDeploymentStateMachine()
    deployID := deployment.ID()

    log := func(step, level, msg string) {
        _ = o.logStreamer.Publish(ctx, deployID, port.LogEntry{
            DeploymentID: deployID,
            StepName:     step,
            Level:        level,
            Message:      msg,
            Timestamp:    time.Now().UnixMilli(),
        })
        _ = o.deployRepo.AppendLog(ctx, deployID, step, level, msg)
    }

    updateStatus := func(status domain.DeploymentStatus, msg string) {
        _ = o.deployRepo.UpdateStatus(ctx, deployID, status, msg)
    }

    // 1. VALIDATING
    if err := sm.Transition(domain.EventStart, "starting validation"); err != nil {
        return err
    }
    updateStatus(sm.Current(), "")

    if err := o.validate(ctx, config, log); err != nil {
        _ = sm.Transition(domain.EventFail, err.Error())
        updateStatus(sm.Current(), err.Error())
        o.triggerRollback(ctx, deployID, sm, log, updateStatus)
        return err
    }
    _ = sm.Transition(domain.EventValidated, "validation passed")
    updateStatus(sm.Current(), "")

    // 2. INSTALLING — Phase A → B 실행
    phaseRunner := phase.NewRunner(o.installer, o.logStreamer, deployID)

    _ = sm.Transition(domain.EventInstalled, "")
    updateStatus(sm.Current(), "starting phase A")
    log("orchestrator", "info", "Phase A 시작: 기반 인프라 설치")

    if err := phaseRunner.RunPhaseA(ctx, config); err != nil {
        _ = sm.Transition(domain.EventFail, err.Error())
        updateStatus(sm.Current(), err.Error())
        o.triggerRollback(ctx, deployID, sm, log, updateStatus)
        return err
    }
    log("orchestrator", "info", "Phase A 완료. Phase B 시작: 핵심 서비스 설치")

    if err := phaseRunner.RunPhaseB(ctx, config); err != nil {
        _ = sm.Transition(domain.EventFail, err.Error())
        updateStatus(sm.Current(), err.Error())
        o.triggerRollback(ctx, deployID, sm, log, updateStatus)
        return err
    }
    _ = sm.Transition(domain.EventInstalled, "phases A+B complete")

    // 3. CONFIGURING — Phase C (연동 설정)
    updateStatus(sm.Current(), "starting phase C")
    log("orchestrator", "info", "Phase C 시작: 연동 설정")

    if err := phaseRunner.RunPhaseC(ctx, config); err != nil {
        _ = sm.Transition(domain.EventFail, err.Error())
        updateStatus(sm.Current(), err.Error())
        o.triggerRollback(ctx, deployID, sm, log, updateStatus)
        return err
    }
    _ = sm.Transition(domain.EventConfigured, "integration configured")

    // 4. HEALTHCHECK
    updateStatus(sm.Current(), "running health checks")
    if err := o.healthCheck(ctx, config, log); err != nil {
        _ = sm.Transition(domain.EventFail, err.Error())
        updateStatus(sm.Current(), err.Error())
        return err // 헬스체크 실패는 롤백하지 않음 (설치는 완료됨)
    }
    _ = sm.Transition(domain.EventHealthOK, "all checks passed")
    updateStatus(sm.Current(), "installation completed successfully")
    log("orchestrator", "info", "설치 완료!")
    return nil
}

func (o *Orchestrator) triggerRollback(
    ctx context.Context,
    deployID string,
    sm *domain.DeploymentStateMachine,
    log func(step, level, msg string),
    updateStatus func(status domain.DeploymentStatus, msg string),
) {
    log("orchestrator", "warn", "설치 실패. 롤백을 시작합니다...")
    _ = sm.Transition(domain.EventRollback, "auto rollback on failure")
    updateStatus(sm.Current(), "rolling back")
    // 실제 롤백 로직은 RollbackManager에 위임
    // (생략: phase.Runner가 push한 rollback stack을 역순 실행)
    _ = sm.Transition(domain.EventRolledBack, "rollback completed")
    updateStatus(sm.Current(), "rolled back")
}
```

### 4.4 Phase Runner — 3-Phase 프로비저닝

```go
// internal/engine/phase/runner.go

package phase

import (
    "context"
    "sync"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
    "github.com/cloud-nullus/nullus/internal/stack/port"
)

// Runner는 3-Phase 프로비저닝을 실행한다.
// Phase 내 독립 스텝은 goroutine으로 병렬 실행, 의존 스텝은 순차 실행한다.
type Runner struct {
    installer   port.HelmInstaller
    logStreamer  port.LogStreamer
    deployID    string
    rollbackStack []rollbackFn // LIFO 스택: 실패 시 역순 실행
    mu          sync.Mutex
}

type rollbackFn func(ctx context.Context) error

func NewRunner(installer port.HelmInstaller, ls port.LogStreamer, deployID string) *Runner {
    return &Runner{
        installer:  installer,
        logStreamer: ls,
        deployID:   deployID,
    }
}

// RunPhaseA는 기반 인프라 (cert-manager, Traefik, MinIO 등)를 설치한다.
// cert-manager, MetalLB, CNPG, MinIO는 병렬 실행.
// Traefik은 cert-manager + MetalLB 완료 후 실행.
func (r *Runner) RunPhaseA(ctx context.Context, config domain.StackConfig) error {
    // 병렬 그룹 1: cert-manager, MetalLB, CNPG Operator, MinIO
    group1 := []stepFn{
        r.installCertManager,
        r.installMetalLB,
        r.installCNPGOperator,
        r.installMinIO(config),
    }
    if err := r.runParallel(ctx, "phase-A-group1", group1); err != nil {
        return err
    }

    // Traefik은 cert-manager + MetalLB 이후
    return r.installTraefik(ctx, config)
}

// RunPhaseB는 핵심 서비스 (GitLab, ArgoCD 등)를 설치한다.
func (r *Runner) RunPhaseB(ctx context.Context, config domain.StackConfig) error {
    // ArgoCD와 Harbor는 병렬 실행 가능
    group1 := []stepFn{
        r.installArgoCD,
    }
    if config.Artifacts.ContainerRegistry.Tool == "harbor" {
        group1 = append(group1, r.installHarbor(config))
    }
    if err := r.runParallel(ctx, "phase-B-group1", group1); err != nil {
        return err
    }

    // GitLab은 CNPG/MinIO 의존성 때문에 Phase A 이후 순차 실행
    if err := r.installGitLab(ctx, config); err != nil {
        return err
    }

    // GitLab Runner는 GitLab 이후
    return r.installGitLabRunner(ctx, config)
}

// RunPhaseC는 보조 서비스 + 연동 설정을 수행한다.
func (r *Runner) RunPhaseC(ctx context.Context, config domain.StackConfig) error {
    // Prometheus+Grafana, Loki는 병렬 실행
    monGroup := []stepFn{
        r.installPrometheusStack,
    }
    if config.Logging.Collection.Tool == "loki" {
        monGroup = append(monGroup, r.installLoki(config))
    } else {
        monGroup = append(monGroup, r.installOpenSearch(config))
    }
    if err := r.runParallel(ctx, "phase-C-monitoring", monGroup); err != nil {
        return err
    }

    // OTel Collector는 Prometheus + 로그 백엔드 이후
    if err := r.installOTelCollector(ctx, config); err != nil {
        return err
    }

    // 연동 설정: Keycloak OIDC, Webhook, ServiceMonitor
    return r.runIntegrations(ctx, config)
}

// runParallel은 스텝 목록을 goroutine으로 병렬 실행한다.
// 하나라도 실패하면 나머지를 취소하고 첫 번째 에러를 반환한다.
func (r *Runner) runParallel(ctx context.Context, groupName string, steps []stepFn) error {
    errCh := make(chan error, len(steps))
    var wg sync.WaitGroup
    for _, step := range steps {
        wg.Add(1)
        go func(s stepFn) {
            defer wg.Done()
            if err := s(ctx); err != nil {
                errCh <- err
            }
        }(step)
    }
    wg.Wait()
    close(errCh)
    if err, ok := <-errCh; ok {
        return fmt.Errorf("parallel step in %s failed: %w", groupName, err)
    }
    return nil
}

type stepFn func(ctx context.Context) error

// pushRollback은 완료된 스텝의 롤백 함수를 스택에 추가한다.
func (r *Runner) pushRollback(fn rollbackFn) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.rollbackStack = append(r.rollbackStack, fn)
}

// Rollback은 스택을 역순으로 실행하여 설치된 컴포넌트를 제거한다.
func (r *Runner) Rollback(ctx context.Context) error {
    r.mu.Lock()
    stack := make([]rollbackFn, len(r.rollbackStack))
    copy(stack, r.rollbackStack)
    r.mu.Unlock()

    // LIFO 순서로 롤백
    for i := len(stack) - 1; i >= 0; i-- {
        if err := stack[i](ctx); err != nil {
            // 롤백 실패는 경고로 처리하고 계속 진행
            r.logStreamer.Publish(ctx, r.deployID, port.LogEntry{
                StepName: "rollback",
                Level:    "warn",
                Message:  fmt.Sprintf("rollback step failed: %v", err),
            })
        }
    }
    return nil
}
```

### 4.5 Known-Issues 패턴 처리기

`known-issues.yaml`은 Narwhal의 70+ Helm edge case 패턴을 코드화한 파일이다.

```yaml
# templates/known-issues/known-issues.yaml

version: "1.0"
description: "Narwhal 레퍼런스 기반 Helm edge case 패턴 DB"

issues:
  # CRD 262KB 초과 → server-side apply 자동 전환
  - id: "crd-size-limit"
    description: "CRD 크기가 262KB를 초과하면 kubectl apply가 실패함"
    pattern:
      type: "crd_size_exceeded"
      affected_charts:
        - "cert-manager"
        - "kube-prometheus-stack"
    fix:
      action: "use_server_side_apply"
      helm_flags:
        server_side_apply: true
        force_conflicts: true

  # 비핵심 앱 --wait 제거
  - id: "skip-wait-non-critical"
    description: "비핵심 앱에 --wait 사용 시 타임아웃으로 설치 실패 빈발"
    pattern:
      type: "non_critical_app"
      affected_charts:
        - "loki"
        - "opentelemetry-collector"
        - "grafana"
    fix:
      action: "skip_wait"
      use_timeout_only: true
      timeout_seconds: 600

  # ARM64 노드 대체 이미지
  - id: "arm64-image-override"
    description: "Harbor 등 일부 차트는 ARM64 이미지 미지원"
    pattern:
      type: "arch_incompatible"
      node_arch: "arm64"
      affected_charts:
        - "harbor"
    fix:
      action: "override_image"
      image_overrides:
        harbor:
          registry: "ghcr.io"
          repository: "goharbor/harbor-core"

  # 레지스트리 우선순위
  - id: "registry-priority"
    description: "Docker Hub rate limit 및 Bitnami 상용화 대응"
    pattern:
      type: "registry_selection"
    fix:
      action: "prefer_registry"
      priority:
        - "ghcr.io"
        - "registry.k8s.io"
        - "quay.io"
        - "docker.io"

  # Loki bucketNames 필수 필드
  - id: "loki-bucket-names"
    description: "Loki helm values에 bucketNames 미설정 시 설치 실패"
    pattern:
      type: "missing_required_value"
      affected_charts:
        - "loki"
      required_fields:
        - "loki.storage.bucketNames.chunks"
        - "loki.storage.bucketNames.ruler"
    fix:
      action: "inject_default_values"
      defaults:
        "loki.storage.bucketNames.chunks": "loki-chunks"
        "loki.storage.bucketNames.ruler": "loki-ruler"

  # Grafana assertNoLeakedSecrets
  - id: "grafana-assert-no-leaked-secrets"
    description: "Grafana 11.x에서 assertNoLeakedSecrets 기본 true → 커스텀 values 사용 불가"
    pattern:
      type: "helm_value_conflict"
      affected_charts:
        - "kube-prometheus-stack"
    fix:
      action: "inject_default_values"
      defaults:
        "grafana.assertNoLeakedSecrets": false

  # Keycloak groups scope
  - id: "keycloak-groups-scope"
    description: "groups scope 미생성 시 OIDC invalid_scope 에러"
    pattern:
      type: "oidc_setup_order"
      affected_apps:
        - "keycloak"
    fix:
      action: "enforce_setup_order"
      steps:
        - "create_realm"
        - "create_groups_scope"
        - "add_groups_mapper"
        - "assign_scope_to_all_clients"
```

```go
// internal/stack/adapter/helm/knownissues/processor.go

package knownissues

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"

    "github.com/cloud-nullus/nullus/internal/stack/port"
)

// Processor는 known-issues.yaml을 로드하여 HelmInstallRequest에 패치를 적용한다.
type Processor struct {
    issues []Issue
}

type Issue struct {
    ID          string  `yaml:"id"`
    Description string  `yaml:"description"`
    Pattern     Pattern `yaml:"pattern"`
    Fix         Fix     `yaml:"fix"`
}

type Pattern struct {
    Type           string   `yaml:"type"`
    AffectedCharts []string `yaml:"affected_charts"`
    NodeArch       string   `yaml:"node_arch,omitempty"`
}

type Fix struct {
    Action          string                 `yaml:"action"`
    UseTimeoutOnly  bool                   `yaml:"use_timeout_only"`
    TimeoutSeconds  int                    `yaml:"timeout_seconds"`
    Defaults        map[string]interface{} `yaml:"defaults"`
}

func NewProcessor(yamlPath string) (*Processor, error) {
    data, err := os.ReadFile(yamlPath)
    if err != nil {
        return nil, fmt.Errorf("read known-issues.yaml: %w", err)
    }
    var parsed struct {
        Issues []Issue `yaml:"issues"`
    }
    if err := yaml.Unmarshal(data, &parsed); err != nil {
        return nil, fmt.Errorf("parse known-issues.yaml: %w", err)
    }
    return &Processor{issues: parsed.Issues}, nil
}

// Apply는 HelmInstallRequest에 매칭되는 known-issue 패치를 적용한다.
func (p *Processor) Apply(req port.HelmInstallRequest) port.HelmInstallRequest {
    for _, issue := range p.issues {
        if !p.matches(issue.Pattern, req) {
            continue
        }
        req = p.applyFix(issue.Fix, req)
    }
    return req
}

func (p *Processor) matches(pattern Pattern, req port.HelmInstallRequest) bool {
    for _, chart := range pattern.AffectedCharts {
        if chart == req.ChartName {
            return true
        }
    }
    return false
}

func (p *Processor) applyFix(fix Fix, req port.HelmInstallRequest) port.HelmInstallRequest {
    switch fix.Action {
    case "use_server_side_apply":
        req.ServerSideApply = true
    case "skip_wait":
        req.SkipWait = true
        if fix.TimeoutSeconds > 0 {
            req.TimeoutSeconds = fix.TimeoutSeconds
        }
    case "inject_default_values":
        if req.Values == nil {
            req.Values = make(map[string]interface{})
        }
        for k, v := range fix.Defaults {
            // 사용자가 명시적으로 설정하지 않은 경우에만 기본값 주입
            if _, exists := req.Values[k]; !exists {
                req.Values[k] = v
            }
        }
    }
    return req
}
```

### 4.6 Post-Install 헬스체크

```go
// internal/engine/healthcheck/checker.go

package healthcheck

import (
    "context"
    "fmt"
    "time"

    "k8s.io/client-go/kubernetes"

    "github.com/cloud-nullus/nullus/internal/stack/domain"
    "github.com/cloud-nullus/nullus/internal/stack/port"
)

// Checker는 설치 완료 후 헬스체크를 수행한다.
// Narwhal verify-cluster.sh의 120+ 항목을 참고하여 구현한다.
type Checker struct {
    k8sClient  kubernetes.Interface
    logStreamer port.LogStreamer
    deployID   string
}

// CheckResult는 개별 체크 항목의 결과다.
type CheckResult struct {
    Name    string
    Passed  bool
    Message string
}

// RunAll은 설정된 스택에 따라 해당하는 헬스체크를 실행한다.
func (c *Checker) RunAll(ctx context.Context, config domain.StackConfig) error {
    checks := c.buildChecks(config)
    var failures []string

    for _, check := range checks {
        result := check(ctx)
        level := "info"
        if !result.Passed {
            level = "error"
            failures = append(failures, result.Name)
        }
        c.logStreamer.Publish(ctx, c.deployID, port.LogEntry{
            StepName:  "healthcheck",
            Phase:     "post-install",
            Level:     level,
            Message:   fmt.Sprintf("[%s] %s: %s", statusIcon(result.Passed), result.Name, result.Message),
            Timestamp: time.Now().UnixMilli(),
        })
    }

    if len(failures) > 0 {
        return fmt.Errorf("헬스체크 실패 항목: %v", failures)
    }
    return nil
}

func (c *Checker) buildChecks(config domain.StackConfig) []func(context.Context) CheckResult {
    checks := []func(context.Context) CheckResult{
        c.checkNamespacesExist,
        c.checkCertManagerWebhook,
        c.checkTraefikIngress,
    }

    if config.Artifacts.SourceRepository.Tool == "gitlab" {
        checks = append(checks,
            c.checkGitLabWebservice,
            c.checkGitLabRegistry,
            c.checkGitLabRunner,
        )
    }
    if config.Pipeline.CDTool.Tool == "argocd" {
        checks = append(checks, c.checkArgoCDServer)
    }
    if config.Monitoring.Collection.Tool == "prometheus" {
        checks = append(checks,
            c.checkPrometheusTargets,
            c.checkGrafanaDatasource,
        )
    }
    if config.Artifacts.StorageBackend.Tool == "minio" {
        checks = append(checks, c.checkMinIOBuckets)
    }
    return checks
}

func (c *Checker) checkNamespacesExist(ctx context.Context) CheckResult {
    namespaces := []string{
        "nullus-system", "nullus-artifacts", "nullus-scm",
        "nullus-cicd", "nullus-monitoring", "nullus-logging",
    }
    for _, ns := range namespaces {
        _, err := c.k8sClient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
        if err != nil {
            return CheckResult{
                Name:    "namespaces-exist",
                Passed:  false,
                Message: fmt.Sprintf("namespace %s not found: %v", ns, err),
            }
        }
    }
    return CheckResult{Name: "namespaces-exist", Passed: true, Message: "모든 필수 네임스페이스 존재"}
}

func statusIcon(passed bool) string {
    if passed {
        return "PASS"
    }
    return "FAIL"
}
```

---

## 변경 이력

| 버전 | 날짜 | 내용 | 작성자 |
|------|------|------|--------|
| 1.0 | 2026-03-14 | Part 1 초안 작성 (개요, 프로젝트 구조, stack/admin 모듈, Install Engine) | Nullus 팀 |
# Nullus Platform Go 백엔드 상세 설계 — Part 2

**작성일**: 2026-03-14
**버전**: 1.0
**기반 문서**: nullus_PRD_1.3.md, Nullus_기능목록.md, Nullus 상세 기능 명세 및 시스템 아키텍처.md
**대상 독자**: Backend 엔지니어

---

## 목차

1. [cicd 모듈](#1-cicd-모듈)
2. [observability 모듈](#2-observability-모듈)
3. [auth 모듈](#3-auth-모듈)
4. [도메인 이벤트 설계](#4-도메인-이벤트-설계)
5. [에러 처리 전략](#5-에러-처리-전략)
6. [의존성 주입](#6-의존성-주입)
7. [테스트 전략](#7-테스트-전략)
8. [설정 관리](#8-설정-관리)
9. [로깅/메트릭](#9-로깅메트릭)

---

## 1. cicd 모듈

CI/CD 파이프라인의 생성·배포·롤백을 담당하는 모듈입니다. PRD 기능 F5(템플릿 제공), F6(배포/이력 관리)에 대응합니다.

### 1.1 디렉토리 구조

```
internal/cicd/
├── domain/
│   ├── pipeline.go          # Pipeline, PipelineTemplate Entity
│   ├── deployment.go        # Deployment Entity
│   └── errors.go            # 도메인 에러 정의
├── usecase/
│   ├── create_pipeline.go
│   ├── deploy_pipeline.go
│   └── rollback_deployment.go
├── port/
│   ├── pipeline_repository.go
│   ├── deployment_repository.go
│   └── k8s_deployer.go
└── adapter/
    ├── postgres/
    │   ├── pipeline_repo.go
    │   └── deployment_repo.go
    └── k8s/
        └── k8s_deployer.go
```

### 1.2 Domain — Entity

```go
// internal/cicd/domain/pipeline.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// PipelineType 은 파이프라인 유형을 나타냅니다.
type PipelineType string

const (
    PipelineTypeWebBackend  PipelineType = "web-backend"
    PipelineTypeWebFrontend PipelineType = "web-frontend"
    PipelineTypeBatchJob    PipelineType = "batch-job"
)

// PipelineTemplate 은 사전 정의된 CI/CD 파이프라인 청사진입니다.
// 변경 불가(읽기 전용) 도메인 객체입니다.
type PipelineTemplate struct {
    ID          string
    Name        string
    Type        PipelineType
    Description string
    // Steps 는 파이프라인 단계 정의 목록(JSON 직렬화 대상)입니다.
    Steps       []PipelineStep
    Parameters  []ParameterDef
    Version     string
    CreatedAt   time.Time
}

// PipelineStep 은 파이프라인의 단일 실행 단계입니다.
type PipelineStep struct {
    Name     string
    Image    string
    Commands []string
    EnvVars  map[string]string
}

// ParameterDef 는 템플릿 인스턴스화 시 사용자가 입력해야 하는 파라미터 명세입니다.
type ParameterDef struct {
    Key         string
    Description string
    Required    bool
    Default     string
}

// Pipeline 은 템플릿에서 생성된 실제 파이프라인 인스턴스입니다.
type Pipeline struct {
    ID         uuid.UUID
    OrgID      uuid.UUID
    TemplateID string
    Name       string
    Type       PipelineType
    // Parameters 는 사용자가 입력한 파라미터 값입니다.
    Parameters map[string]string
    // GitRepoURL 은 소스 저장소 URL입니다.
    GitRepoURL string
    // ClusterID 는 배포 대상 클러스터입니다.
    ClusterID  uuid.UUID
    Namespace  string
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// NewPipeline 은 Pipeline 도메인 객체를 생성하고 필수 필드를 검증합니다.
func NewPipeline(orgID uuid.UUID, name, gitRepoURL string, clusterID uuid.UUID) (*Pipeline, error) {
    if name == "" {
        return nil, ErrPipelineNameRequired
    }
    if gitRepoURL == "" {
        return nil, ErrGitRepoURLRequired
    }
    return &Pipeline{
        ID:         uuid.New(),
        OrgID:      orgID,
        Name:       name,
        GitRepoURL: gitRepoURL,
        ClusterID:  clusterID,
        Parameters: make(map[string]string),
        CreatedAt:  time.Now().UTC(),
        UpdatedAt:  time.Now().UTC(),
    }, nil
}
```

```go
// internal/cicd/domain/deployment.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// DeploymentStatus 는 배포 진행 상태입니다.
type DeploymentStatus string

const (
    DeploymentStatusPending    DeploymentStatus = "pending"
    DeploymentStatusRunning    DeploymentStatus = "running"
    DeploymentStatusSucceeded  DeploymentStatus = "succeeded"
    DeploymentStatusFailed     DeploymentStatus = "failed"
    DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
)

// Deployment 는 특정 Pipeline의 단일 배포 실행 기록입니다.
type Deployment struct {
    ID         uuid.UUID
    PipelineID uuid.UUID
    // Revision 은 이 배포의 순번입니다 (1부터 증가).
    Revision   int
    Status     DeploymentStatus
    // ManifestSnapshot 은 배포 시점의 Kubernetes 매니페스트 YAML 스냅샷입니다.
    ManifestSnapshot string
    // DeployedBy 는 배포를 실행한 사용자 ID입니다.
    DeployedBy uuid.UUID
    Reason     string
    StartedAt  time.Time
    FinishedAt *time.Time
    FailReason string
}

// Succeed 는 배포 성공 상태로 전이합니다.
func (d *Deployment) Succeed() {
    now := time.Now().UTC()
    d.Status = DeploymentStatusSucceeded
    d.FinishedAt = &now
}

// Fail 은 배포 실패 상태로 전이합니다.
func (d *Deployment) Fail(reason string) {
    now := time.Now().UTC()
    d.Status = DeploymentStatusFailed
    d.FailReason = reason
    d.FinishedAt = &now
}

// MarkRolledBack 은 배포를 롤백 완료 상태로 전이합니다.
func (d *Deployment) MarkRolledBack() {
    now := time.Now().UTC()
    d.Status = DeploymentStatusRolledBack
    d.FinishedAt = &now
}

// IsTerminal 은 더 이상 상태 전이가 없는 최종 상태인지 반환합니다.
func (d *Deployment) IsTerminal() bool {
    switch d.Status {
    case DeploymentStatusSucceeded, DeploymentStatusFailed, DeploymentStatusRolledBack:
        return true
    }
    return false
}
```

### 1.3 UseCase

```go
// internal/cicd/usecase/create_pipeline.go

package usecase

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/cicd/domain"
    "github.com/cloud-nullus/nullus/internal/cicd/port"
    "github.com/google/uuid"
)

// CreatePipelineInput 은 파이프라인 생성 요청 DTO입니다.
type CreatePipelineInput struct {
    OrgID      uuid.UUID
    TemplateID string
    Name       string
    GitRepoURL string
    ClusterID  uuid.UUID
    Namespace  string
    Parameters map[string]string
}

// CreatePipelineOutput 은 파이프라인 생성 결과 DTO입니다.
type CreatePipelineOutput struct {
    Pipeline *domain.Pipeline
}

// CreatePipelineUseCase 는 파이프라인 생성 유스케이스입니다.
type CreatePipelineUseCase struct {
    pipelineRepo     port.PipelineRepository
    templateRepo     port.PipelineTemplateRepository
    eventBus         port.EventBus
}

func NewCreatePipelineUseCase(
    pipelineRepo port.PipelineRepository,
    templateRepo port.PipelineTemplateRepository,
    eventBus port.EventBus,
) *CreatePipelineUseCase {
    return &CreatePipelineUseCase{
        pipelineRepo: pipelineRepo,
        templateRepo: templateRepo,
        eventBus:     eventBus,
    }
}

func (uc *CreatePipelineUseCase) Execute(ctx context.Context, in CreatePipelineInput) (*CreatePipelineOutput, error) {
    // 1. 템플릿 존재 여부 확인
    tmpl, err := uc.templateRepo.FindByID(ctx, in.TemplateID)
    if err != nil {
        return nil, err
    }
    if tmpl == nil {
        return nil, domain.ErrTemplateNotFound
    }

    // 2. 필수 파라미터 검증
    if err := validateParameters(tmpl, in.Parameters); err != nil {
        return nil, err
    }

    // 3. Pipeline 엔티티 생성
    p, err := domain.NewPipeline(in.OrgID, in.Name, in.GitRepoURL, in.ClusterID)
    if err != nil {
        return nil, err
    }
    p.TemplateID = in.TemplateID
    p.Namespace = in.Namespace
    p.Parameters = in.Parameters

    // 4. 저장
    if err := uc.pipelineRepo.Save(ctx, p); err != nil {
        return nil, err
    }

    // 5. 도메인 이벤트 발행
    _ = uc.eventBus.Publish(ctx, domain.NewPipelineCreatedEvent(p))

    return &CreatePipelineOutput{Pipeline: p}, nil
}

func validateParameters(tmpl *domain.PipelineTemplate, provided map[string]string) error {
    for _, def := range tmpl.Parameters {
        if def.Required {
            if v, ok := provided[def.Key]; !ok || v == "" {
                return domain.NewErrMissingParameter(def.Key)
            }
        }
    }
    return nil
}
```

```go
// internal/cicd/usecase/deploy_pipeline.go

package usecase

import (
    "context"
    "time"

    "github.com/cloud-nullus/nullus/internal/cicd/domain"
    "github.com/cloud-nullus/nullus/internal/cicd/port"
    "github.com/google/uuid"
)

type DeployPipelineInput struct {
    PipelineID uuid.UUID
    DeployedBy uuid.UUID
    Reason     string
}

type DeployPipelineOutput struct {
    Deployment *domain.Deployment
}

type DeployPipelineUseCase struct {
    pipelineRepo    port.PipelineRepository
    deploymentRepo  port.DeploymentRepository
    k8sDeployer     port.K8sDeployer
    eventBus        port.EventBus
}

func NewDeployPipelineUseCase(
    pipelineRepo port.PipelineRepository,
    deploymentRepo port.DeploymentRepository,
    k8sDeployer port.K8sDeployer,
    eventBus port.EventBus,
) *DeployPipelineUseCase {
    return &DeployPipelineUseCase{
        pipelineRepo:   pipelineRepo,
        deploymentRepo: deploymentRepo,
        k8sDeployer:    k8sDeployer,
        eventBus:       eventBus,
    }
}

func (uc *DeployPipelineUseCase) Execute(ctx context.Context, in DeployPipelineInput) (*DeployPipelineOutput, error) {
    // 1. 파이프라인 조회
    p, err := uc.pipelineRepo.FindByID(ctx, in.PipelineID)
    if err != nil {
        return nil, err
    }
    if p == nil {
        return nil, domain.ErrPipelineNotFound
    }

    // 2. 이전 배포 리비전 계산
    lastRevision, err := uc.deploymentRepo.LatestRevision(ctx, in.PipelineID)
    if err != nil {
        return nil, err
    }

    // 3. Deployment 엔티티 생성
    dep := &domain.Deployment{
        ID:         uuid.New(),
        PipelineID: in.PipelineID,
        Revision:   lastRevision + 1,
        Status:     domain.DeploymentStatusRunning,
        DeployedBy: in.DeployedBy,
        Reason:     in.Reason,
        StartedAt:  time.Now().UTC(),
    }
    if err := uc.deploymentRepo.Save(ctx, dep); err != nil {
        return nil, err
    }

    // 4. Kubernetes에 실제 배포 (비동기 실행을 원하면 goroutine으로 분리)
    manifest, err := uc.k8sDeployer.Deploy(ctx, p)
    if err != nil {
        dep.Fail(err.Error())
        _ = uc.deploymentRepo.Update(ctx, dep)
        _ = uc.eventBus.Publish(ctx, domain.NewDeploymentFailedEvent(dep, err))
        return nil, err
    }

    dep.ManifestSnapshot = manifest
    dep.Succeed()
    if err := uc.deploymentRepo.Update(ctx, dep); err != nil {
        return nil, err
    }

    _ = uc.eventBus.Publish(ctx, domain.NewPipelineDeployedEvent(p, dep))

    return &DeployPipelineOutput{Deployment: dep}, nil
}
```

```go
// internal/cicd/usecase/rollback_deployment.go

package usecase

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/cicd/domain"
    "github.com/cloud-nullus/nullus/internal/cicd/port"
    "github.com/google/uuid"
)

type RollbackDeploymentInput struct {
    PipelineID   uuid.UUID
    // TargetRevision 은 롤백할 대상 리비전 번호입니다.
    TargetRevision int
    RequestedBy  uuid.UUID
}

type RollbackDeploymentUseCase struct {
    pipelineRepo   port.PipelineRepository
    deploymentRepo port.DeploymentRepository
    k8sDeployer    port.K8sDeployer
    eventBus       port.EventBus
}

func NewRollbackDeploymentUseCase(
    pipelineRepo port.PipelineRepository,
    deploymentRepo port.DeploymentRepository,
    k8sDeployer port.K8sDeployer,
    eventBus port.EventBus,
) *RollbackDeploymentUseCase {
    return &RollbackDeploymentUseCase{
        pipelineRepo:   pipelineRepo,
        deploymentRepo: deploymentRepo,
        k8sDeployer:    k8sDeployer,
        eventBus:       eventBus,
    }
}

func (uc *RollbackDeploymentUseCase) Execute(ctx context.Context, in RollbackDeploymentInput) error {
    // 1. 롤백 대상 리비전의 매니페스트 스냅샷 조회
    target, err := uc.deploymentRepo.FindByRevision(ctx, in.PipelineID, in.TargetRevision)
    if err != nil {
        return err
    }
    if target == nil {
        return domain.ErrDeploymentNotFound
    }
    if target.ManifestSnapshot == "" {
        return domain.ErrManifestSnapshotMissing
    }

    // 2. 스냅샷 매니페스트를 클러스터에 재적용
    if err := uc.k8sDeployer.ApplyManifest(ctx, target.ManifestSnapshot); err != nil {
        return err
    }

    // 3. 롤백 이력 기록
    target.MarkRolledBack()
    if err := uc.deploymentRepo.Update(ctx, target); err != nil {
        return err
    }

    _ = uc.eventBus.Publish(ctx, domain.NewDeploymentRolledBackEvent(target))
    return nil
}
```

### 1.4 Port (인터페이스)

```go
// internal/cicd/port/repositories.go

package port

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/cicd/domain"
    "github.com/google/uuid"
)

// PipelineRepository 는 Pipeline 영속성 인터페이스입니다.
type PipelineRepository interface {
    Save(ctx context.Context, p *domain.Pipeline) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Pipeline, error)
    FindByOrgID(ctx context.Context, orgID uuid.UUID) ([]*domain.Pipeline, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

// PipelineTemplateRepository 는 PipelineTemplate 조회 인터페이스입니다.
type PipelineTemplateRepository interface {
    FindByID(ctx context.Context, id string) (*domain.PipelineTemplate, error)
    FindAll(ctx context.Context) ([]*domain.PipelineTemplate, error)
}

// DeploymentRepository 는 Deployment 영속성 인터페이스입니다.
type DeploymentRepository interface {
    Save(ctx context.Context, d *domain.Deployment) error
    Update(ctx context.Context, d *domain.Deployment) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Deployment, error)
    FindByPipelineID(ctx context.Context, pipelineID uuid.UUID) ([]*domain.Deployment, error)
    FindByRevision(ctx context.Context, pipelineID uuid.UUID, revision int) (*domain.Deployment, error)
    LatestRevision(ctx context.Context, pipelineID uuid.UUID) (int, error)
}
```

```go
// internal/cicd/port/k8s_deployer.go

package port

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/cicd/domain"
)

// K8sDeployer 는 Kubernetes 배포 실행 인터페이스입니다.
type K8sDeployer interface {
    // Deploy 는 파이프라인 설정을 기반으로 K8s 오브젝트를 생성하고 적용된 매니페스트 YAML을 반환합니다.
    Deploy(ctx context.Context, p *domain.Pipeline) (manifestYAML string, err error)
    // ApplyManifest 는 이미 렌더링된 매니페스트 YAML을 클러스터에 적용합니다 (롤백 용도).
    ApplyManifest(ctx context.Context, manifestYAML string) error
    // Delete 는 파이프라인 관련 모든 K8s 리소스를 삭제합니다.
    Delete(ctx context.Context, namespace string) error
}

// EventBus 는 도메인 이벤트 발행 인터페이스입니다 (공유 타입, 별도 패키지에서 정의 가능).
type EventBus interface {
    Publish(ctx context.Context, event interface{}) error
}
```

---

## 2. observability 모듈

모니터링 대시보드, 알림 규칙, 알림 이력을 관리하는 모듈입니다. PRD 기능 F7에 대응합니다.

### 2.1 디렉토리 구조

```
internal/observability/
├── domain/
│   ├── dashboard.go
│   ├── alert.go
│   └── errors.go
├── usecase/
│   ├── get_dashboard.go
│   ├── create_alert_rule.go
│   └── evaluate_alerts.go
├── port/
│   ├── metrics_provider.go
│   ├── alert_repository.go
│   └── notifier.go
└── adapter/
    ├── prometheus/
    │   └── metrics_provider.go
    ├── postgres/
    │   └── alert_repo.go
    └── notifier/
        ├── slack_notifier.go
        └── email_notifier.go
```

### 2.2 Domain — Entity

```go
// internal/observability/domain/dashboard.go

package domain

import "time"

// MetricPoint 는 단일 시계열 데이터 포인트입니다.
type MetricPoint struct {
    Timestamp time.Time
    Value     float64
}

// ClusterMetrics 는 클러스터 수준의 집계 지표입니다.
type ClusterMetrics struct {
    CPUUsagePct    float64
    MemoryUsagePct float64
    StorageUsagePct float64
    // Series 는 시계열 데이터(그래프용)입니다.
    CPUSeries    []MetricPoint
    MemorySeries []MetricPoint
}

// PipelineMetrics 는 CI/CD 파이프라인 집계 지표입니다.
type PipelineMetrics struct {
    TotalRuns      int
    SuccessCount   int
    FailureCount   int
    SuccessRate    float64
    AvgDurationSec float64
}

// ToolHealthStatus 는 개별 도구의 상태를 나타냅니다.
type ToolHealthStatus string

const (
    ToolHealthRunning ToolHealthStatus = "running"
    ToolHealthWarning ToolHealthStatus = "warning"
    ToolHealthError   ToolHealthStatus = "error"
)

// ToolHealth 는 스택에 설치된 개별 도구의 상태입니다.
type ToolHealth struct {
    Name      string
    Namespace string
    Status    ToolHealthStatus
    Message   string
    CheckedAt time.Time
}

// Dashboard 는 모니터링 대시보드 집계 뷰입니다.
// 도메인 서비스가 여러 포트에서 수집한 데이터를 조합하여 생성합니다.
type Dashboard struct {
    Cluster   ClusterMetrics
    Pipelines PipelineMetrics
    Tools     []ToolHealth
    GeneratedAt time.Time
}
```

```go
// internal/observability/domain/alert.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// AlertSeverity 는 알림 심각도입니다.
type AlertSeverity string

const (
    AlertSeverityCritical AlertSeverity = "critical"
    AlertSeverityWarning  AlertSeverity = "warning"
    AlertSeverityInfo     AlertSeverity = "info"
)

// AlertRuleCondition 은 알림 발동 조건입니다.
type AlertRuleCondition string

const (
    ConditionToolDown       AlertRuleCondition = "tool_down"
    ConditionHighCPU        AlertRuleCondition = "high_cpu"
    ConditionHighMemory     AlertRuleCondition = "high_memory"
    ConditionStorageWarning AlertRuleCondition = "storage_warning"
    ConditionPipelineFailure AlertRuleCondition = "pipeline_failure"
)

// AlertRule 은 알림 발동 규칙을 정의합니다.
type AlertRule struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Name        string
    Condition   AlertRuleCondition
    // Threshold 는 수치형 조건의 임계값(예: CPU 85%)입니다.
    Threshold   float64
    Severity    AlertSeverity
    // NotifyChannels 는 발동 시 알릴 채널 목록입니다 (slack, email).
    NotifyChannels []string
    Enabled     bool
    CreatedAt   time.Time
}

// Alert 는 실제 발동된 알림 이력입니다.
type Alert struct {
    ID         uuid.UUID
    RuleID     uuid.UUID
    OrgID      uuid.UUID
    Severity   AlertSeverity
    Message    string
    // Resolved 는 알림이 해소되었는지 나타냅니다.
    Resolved   bool
    FiredAt    time.Time
    ResolvedAt *time.Time
}

// Resolve 는 알림을 해소 상태로 전이합니다.
func (a *Alert) Resolve() {
    now := time.Now().UTC()
    a.Resolved = true
    a.ResolvedAt = &now
}

// NewAlertRule 은 AlertRule 엔티티를 생성하고 기본값을 설정합니다.
func NewAlertRule(orgID uuid.UUID, name string, condition AlertRuleCondition, severity AlertSeverity) (*AlertRule, error) {
    if name == "" {
        return nil, ErrAlertRuleNameRequired
    }
    return &AlertRule{
        ID:        uuid.New(),
        OrgID:     orgID,
        Name:      name,
        Condition: condition,
        Severity:  severity,
        Enabled:   true,
        CreatedAt: time.Now().UTC(),
    }, nil
}
```

### 2.3 UseCase

```go
// internal/observability/usecase/get_dashboard.go

package usecase

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/observability/domain"
    "github.com/cloud-nullus/nullus/internal/observability/port"
    "github.com/google/uuid"
)

// GetDashboardUseCase 는 모니터링 대시보드 데이터를 조합하여 반환합니다.
type GetDashboardUseCase struct {
    metricsProvider port.MetricsProvider
    alertRepo       port.AlertRepository
}

func NewGetDashboardUseCase(
    metricsProvider port.MetricsProvider,
    alertRepo port.AlertRepository,
) *GetDashboardUseCase {
    return &GetDashboardUseCase{
        metricsProvider: metricsProvider,
        alertRepo:       alertRepo,
    }
}

func (uc *GetDashboardUseCase) Execute(ctx context.Context, orgID uuid.UUID) (*domain.Dashboard, error) {
    clusterMetrics, err := uc.metricsProvider.GetClusterMetrics(ctx, orgID)
    if err != nil {
        return nil, err
    }

    pipelineMetrics, err := uc.metricsProvider.GetPipelineMetrics(ctx, orgID)
    if err != nil {
        return nil, err
    }

    toolHealths, err := uc.metricsProvider.GetToolHealths(ctx, orgID)
    if err != nil {
        return nil, err
    }

    return &domain.Dashboard{
        Cluster:   *clusterMetrics,
        Pipelines: *pipelineMetrics,
        Tools:     toolHealths,
    }, nil
}
```

```go
// internal/observability/usecase/create_alert_rule.go

package usecase

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/observability/domain"
    "github.com/cloud-nullus/nullus/internal/observability/port"
    "github.com/google/uuid"
)

type CreateAlertRuleInput struct {
    OrgID          uuid.UUID
    Name           string
    Condition      domain.AlertRuleCondition
    Threshold      float64
    Severity       domain.AlertSeverity
    NotifyChannels []string
}

type CreateAlertRuleUseCase struct {
    alertRepo port.AlertRepository
    eventBus  port.EventBus
}

func NewCreateAlertRuleUseCase(
    alertRepo port.AlertRepository,
    eventBus port.EventBus,
) *CreateAlertRuleUseCase {
    return &CreateAlertRuleUseCase{alertRepo: alertRepo, eventBus: eventBus}
}

func (uc *CreateAlertRuleUseCase) Execute(ctx context.Context, in CreateAlertRuleInput) (*domain.AlertRule, error) {
    rule, err := domain.NewAlertRule(in.OrgID, in.Name, in.Condition, in.Severity)
    if err != nil {
        return nil, err
    }
    rule.Threshold = in.Threshold
    rule.NotifyChannels = in.NotifyChannels

    if err := uc.alertRepo.SaveRule(ctx, rule); err != nil {
        return nil, err
    }

    _ = uc.eventBus.Publish(ctx, domain.NewAlertRuleCreatedEvent(rule))
    return rule, nil
}
```

```go
// internal/observability/usecase/evaluate_alerts.go

package usecase

import (
    "context"
    "fmt"
    "time"

    "github.com/cloud-nullus/nullus/internal/observability/domain"
    "github.com/cloud-nullus/nullus/internal/observability/port"
    "github.com/google/uuid"
)

// EvaluateAlertsUseCase 는 활성 AlertRule을 평가하고 조건 충족 시 Alert를 발행합니다.
// 주기적인 스케줄러(예: 1분 간격)에 의해 호출됩니다.
type EvaluateAlertsUseCase struct {
    alertRepo       port.AlertRepository
    metricsProvider port.MetricsProvider
    notifier        port.Notifier
    eventBus        port.EventBus
}

func NewEvaluateAlertsUseCase(
    alertRepo port.AlertRepository,
    metricsProvider port.MetricsProvider,
    notifier port.Notifier,
    eventBus port.EventBus,
) *EvaluateAlertsUseCase {
    return &EvaluateAlertsUseCase{
        alertRepo:       alertRepo,
        metricsProvider: metricsProvider,
        notifier:        notifier,
        eventBus:        eventBus,
    }
}

func (uc *EvaluateAlertsUseCase) Execute(ctx context.Context, orgID uuid.UUID) error {
    rules, err := uc.alertRepo.FindEnabledRules(ctx, orgID)
    if err != nil {
        return err
    }

    for _, rule := range rules {
        fired, msg, err := uc.evaluateRule(ctx, orgID, rule)
        if err != nil {
            // 단일 규칙 평가 실패는 로그만 남기고 계속 진행
            continue
        }
        if !fired {
            continue
        }

        alert := &domain.Alert{
            ID:       uuid.New(),
            RuleID:   rule.ID,
            OrgID:    orgID,
            Severity: rule.Severity,
            Message:  msg,
            FiredAt:  time.Now().UTC(),
        }
        if err := uc.alertRepo.SaveAlert(ctx, alert); err != nil {
            continue
        }

        // 알림 채널로 발송
        for _, ch := range rule.NotifyChannels {
            _ = uc.notifier.Send(ctx, ch, alert)
        }

        _ = uc.eventBus.Publish(ctx, domain.NewAlertTriggeredEvent(alert, rule))
    }
    return nil
}

func (uc *EvaluateAlertsUseCase) evaluateRule(
    ctx context.Context,
    orgID uuid.UUID,
    rule *domain.AlertRule,
) (fired bool, message string, err error) {
    switch rule.Condition {
    case domain.ConditionHighCPU:
        metrics, err := uc.metricsProvider.GetClusterMetrics(ctx, orgID)
        if err != nil {
            return false, "", err
        }
        if metrics.CPUUsagePct >= rule.Threshold {
            return true, fmt.Sprintf("CPU 사용률 %.1f%% (임계값: %.1f%%)", metrics.CPUUsagePct, rule.Threshold), nil
        }
    case domain.ConditionHighMemory:
        metrics, err := uc.metricsProvider.GetClusterMetrics(ctx, orgID)
        if err != nil {
            return false, "", err
        }
        if metrics.MemoryUsagePct >= rule.Threshold {
            return true, fmt.Sprintf("메모리 사용률 %.1f%% (임계값: %.1f%%)", metrics.MemoryUsagePct, rule.Threshold), nil
        }
    case domain.ConditionPipelineFailure:
        pm, err := uc.metricsProvider.GetPipelineMetrics(ctx, orgID)
        if err != nil {
            return false, "", err
        }
        // 성공률이 임계값 이하이면 발동
        if pm.SuccessRate <= rule.Threshold {
            return true, fmt.Sprintf("파이프라인 성공률 %.1f%% (임계값: %.1f%%)", pm.SuccessRate, rule.Threshold), nil
        }
    }
    return false, "", nil
}
```

### 2.4 Port (인터페이스)

```go
// internal/observability/port/ports.go

package port

import (
    "context"

    "github.com/cloud-nullus/nullus/internal/observability/domain"
    "github.com/google/uuid"
)

// MetricsProvider 는 외부 메트릭 수집 시스템과의 인터페이스입니다 (Prometheus 등).
type MetricsProvider interface {
    GetClusterMetrics(ctx context.Context, orgID uuid.UUID) (*domain.ClusterMetrics, error)
    GetPipelineMetrics(ctx context.Context, orgID uuid.UUID) (*domain.PipelineMetrics, error)
    GetToolHealths(ctx context.Context, orgID uuid.UUID) ([]domain.ToolHealth, error)
}

// AlertRepository 는 AlertRule, Alert 영속성 인터페이스입니다.
type AlertRepository interface {
    SaveRule(ctx context.Context, rule *domain.AlertRule) error
    UpdateRule(ctx context.Context, rule *domain.AlertRule) error
    FindRuleByID(ctx context.Context, id uuid.UUID) (*domain.AlertRule, error)
    FindEnabledRules(ctx context.Context, orgID uuid.UUID) ([]*domain.AlertRule, error)
    SaveAlert(ctx context.Context, alert *domain.Alert) error
    FindAlertsByOrgID(ctx context.Context, orgID uuid.UUID) ([]*domain.Alert, error)
}

// Notifier 는 외부 알림 발송 인터페이스입니다.
type Notifier interface {
    // Send 는 지정 채널("slack", "email")로 Alert를 발송합니다.
    Send(ctx context.Context, channel string, alert *domain.Alert) error
}

// EventBus 는 도메인 이벤트 발행 인터페이스입니다.
type EventBus interface {
    Publish(ctx context.Context, event interface{}) error
}
```

---

## 3. auth 모듈

### 3.1 인증 전환 전략 개요

PRD와 기능목록에 명시된 대로 인증 방식은 단계적으로 전환됩니다.

| 릴리스 | 방식 | 구현 |
|--------|------|------|
| Alpha | 세션 기반 인증 | gorilla/sessions + PostgreSQL 세션 저장소 |
| Beta | 세션 기반 인증 (유지) | 멤버 초대/역할 부여 기능 추가 |
| v1 GA | Keycloak OIDC | OAuth2/OIDC 흐름, RBAC 매핑 |

이 전환을 유연하게 지원하기 위해 `AuthProvider` 인터페이스를 먼저 정의하고, 구현체만 교체하는 전략을 사용합니다.

### 3.2 디렉토리 구조

```
internal/auth/
├── domain/
│   ├── user.go           # User, Role, Session Entity
│   └── errors.go
├── usecase/
│   ├── login.go          # Alpha/Beta: 세션 로그인
│   ├── logout.go
│   └── get_current_user.go
├── port/
│   ├── auth_provider.go  # 핵심 인터페이스
│   ├── user_repository.go
│   └── session_store.go
└── adapter/
    ├── session/          # Alpha/Beta 구현
    │   ├── session_auth_provider.go
    │   └── postgres_session_store.go
    └── oidc/             # v1 구현
        └── keycloak_auth_provider.go
```

### 3.3 Domain — Entity

```go
// internal/auth/domain/user.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// Role 은 사용자의 역할입니다. PRD proto4에서 3역할 체계 확정.
type Role string

const (
    RoleAdmin    Role = "admin"
    RoleDevOps   Role = "devops_engineer"
    RoleDeveloper Role = "developer"
)

// User 는 Nullus 플랫폼 사용자입니다.
type User struct {
    ID        uuid.UUID
    OrgID     uuid.UUID
    Email     string
    Name      string
    Role      Role
    // ExternalID 는 Keycloak subject (v1 전환 후 사용)입니다.
    ExternalID string
    Active    bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

// CanAccess 는 역할 기반 접근 권한을 도메인 레이어에서 검증합니다.
// PRD F9의 RBAC 매핑을 코드로 표현합니다.
func (u *User) CanAccess(resource Resource, action Action) bool {
    switch u.Role {
    case RoleAdmin:
        // Admin은 모든 리소스에 접근 가능
        return true
    case RoleDevOps:
        return devOpsPermissions[resource][action]
    case RoleDeveloper:
        return developerPermissions[resource][action]
    }
    return false
}

// Resource 는 RBAC에서 보호 대상 리소스입니다.
type Resource string

const (
    ResourceOrg        Resource = "org"
    ResourceUser       Resource = "user"
    ResourceCluster    Resource = "cluster"
    ResourceStack      Resource = "stack"
    ResourcePipeline   Resource = "pipeline"
    ResourceMonitoring Resource = "monitoring"
    ResourceAlert      Resource = "alert"
)

// Action 은 리소스에 대한 작업 종류입니다.
type Action string

const (
    ActionRead   Action = "read"
    ActionWrite  Action = "write"
    ActionDeploy Action = "deploy"
    ActionDelete Action = "delete"
)

// devOpsPermissions 는 DevOps Engineer 역할의 권한 매핑입니다.
var devOpsPermissions = map[Resource]map[Action]bool{
    ResourceCluster:    {ActionRead: true},
    ResourceStack:      {ActionRead: true, ActionWrite: true, ActionDeploy: true},
    ResourcePipeline:   {ActionRead: true, ActionWrite: true, ActionDeploy: true},
    ResourceMonitoring: {ActionRead: true},
    ResourceAlert:      {ActionRead: true, ActionWrite: true},
}

// developerPermissions 는 Developer 역할의 권한 매핑입니다.
var developerPermissions = map[Resource]map[Action]bool{
    ResourcePipeline:   {ActionRead: true, ActionDeploy: true},
    ResourceMonitoring: {ActionRead: true},
}
```

### 3.4 Port — AuthProvider 인터페이스

```go
// internal/auth/port/auth_provider.go

package port

import (
    "context"
    "net/http"

    "github.com/cloud-nullus/nullus/internal/auth/domain"
)

// AuthProvider 는 인증 방식에 대한 추상화 인터페이스입니다.
// 세션 기반(Alpha/Beta)과 OIDC(v1) 모두 이 인터페이스를 구현합니다.
type AuthProvider interface {
    // Login 은 자격증명을 검증하고 세션을 생성합니다 (세션 방식) 또는
    // OIDC 리다이렉트 URL을 반환합니다 (OIDC 방식).
    Login(ctx context.Context, r *http.Request, w http.ResponseWriter, credentials Credentials) (*domain.User, error)
    // Logout 은 세션을 무효화합니다.
    Logout(ctx context.Context, r *http.Request, w http.ResponseWriter) error
    // Authenticate 는 요청에서 인증된 사용자를 추출합니다.
    // 미들웨어에서 호출합니다.
    Authenticate(ctx context.Context, r *http.Request) (*domain.User, error)
}

// Credentials 는 로그인 요청 자격증명입니다.
type Credentials struct {
    Email    string
    Password string
}
```

### 3.5 Adapter — Alpha/Beta 세션 기반 구현

```go
// internal/auth/adapter/session/session_auth_provider.go

package session

import (
    "context"
    "net/http"

    "github.com/cloud-nullus/nullus/internal/auth/domain"
    "github.com/cloud-nullus/nullus/internal/auth/port"
    "github.com/gorilla/sessions"
    "golang.org/x/crypto/bcrypt"
)

const sessionName = "nullus_session"
const sessionUserKey = "user_id"

// SessionAuthProvider 는 gorilla/sessions 기반 세션 인증 구현입니다.
type SessionAuthProvider struct {
    store    sessions.Store
    userRepo port.UserRepository
}

func NewSessionAuthProvider(store sessions.Store, userRepo port.UserRepository) *SessionAuthProvider {
    return &SessionAuthProvider{store: store, userRepo: userRepo}
}

func (p *SessionAuthProvider) Login(ctx context.Context, r *http.Request, w http.ResponseWriter, creds port.Credentials) (*domain.User, error) {
    user, err := p.userRepo.FindByEmail(ctx, creds.Email)
    if err != nil || user == nil {
        return nil, domain.ErrInvalidCredentials
    }
    if !user.Active {
        return nil, domain.ErrUserInactive
    }
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password)); err != nil {
        return nil, domain.ErrInvalidCredentials
    }

    sess, _ := p.store.Get(r, sessionName)
    sess.Values[sessionUserKey] = user.ID.String()
    if err := sess.Save(r, w); err != nil {
        return nil, domain.ErrSessionSave
    }
    return user, nil
}

func (p *SessionAuthProvider) Logout(ctx context.Context, r *http.Request, w http.ResponseWriter) error {
    sess, _ := p.store.Get(r, sessionName)
    sess.Options.MaxAge = -1
    return sess.Save(r, w)
}

func (p *SessionAuthProvider) Authenticate(ctx context.Context, r *http.Request) (*domain.User, error) {
    sess, err := p.store.Get(r, sessionName)
    if err != nil {
        return nil, domain.ErrUnauthenticated
    }
    rawID, ok := sess.Values[sessionUserKey]
    if !ok {
        return nil, domain.ErrUnauthenticated
    }
    userID, ok := rawID.(string)
    if !ok {
        return nil, domain.ErrUnauthenticated
    }
    user, err := p.userRepo.FindByStringID(ctx, userID)
    if err != nil || user == nil {
        return nil, domain.ErrUnauthenticated
    }
    return user, nil
}
```

### 3.6 Adapter — v1 Keycloak OIDC 구현

```go
// internal/auth/adapter/oidc/keycloak_auth_provider.go

package oidc

import (
    "context"
    "fmt"
    "net/http"
    "strings"

    "github.com/cloud-nullus/nullus/internal/auth/domain"
    "github.com/cloud-nullus/nullus/internal/auth/port"
    "github.com/coreos/go-oidc/v3/oidc"
    "golang.org/x/oauth2"
)

// KeycloakAuthProvider 는 Keycloak OIDC 기반 인증 구현입니다.
// PRD F9의 Keycloak OIDC 자동 설정 요구사항에 대응합니다.
type KeycloakAuthProvider struct {
    provider *oidc.Provider
    verifier *oidc.IDTokenVerifier
    oauth2Cfg oauth2.Config
    userRepo port.UserRepository
}

func NewKeycloakAuthProvider(ctx context.Context, issuerURL, clientID, clientSecret, redirectURL string, userRepo port.UserRepository) (*KeycloakAuthProvider, error) {
    provider, err := oidc.NewProvider(ctx, issuerURL)
    if err != nil {
        return nil, fmt.Errorf("OIDC provider 초기화 실패: %w", err)
    }

    verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

    cfg := oauth2.Config{
        ClientID:     clientID,
        ClientSecret: clientSecret,
        RedirectURL:  redirectURL,
        Endpoint:     provider.Endpoint(),
        // "groups" scope는 Narwhal 패턴에 따라 반드시 포함해야 합니다.
        // 미포함 시 Keycloak에서 invalid_scope 에러가 발생합니다.
        Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"},
    }

    return &KeycloakAuthProvider{
        provider:  provider,
        verifier:  verifier,
        oauth2Cfg: cfg,
        userRepo:  userRepo,
    }, nil
}

// Authenticate 는 Authorization 헤더의 Bearer 토큰을 검증합니다.
func (p *KeycloakAuthProvider) Authenticate(ctx context.Context, r *http.Request) (*domain.User, error) {
    authHeader := r.Header.Get("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return nil, domain.ErrUnauthenticated
    }
    rawToken := strings.TrimPrefix(authHeader, "Bearer ")

    idToken, err := p.verifier.Verify(ctx, rawToken)
    if err != nil {
        return nil, domain.ErrInvalidToken
    }

    var claims struct {
        Subject string   `json:"sub"`
        Email   string   `json:"email"`
        Name    string   `json:"name"`
        Groups  []string `json:"groups"`
    }
    if err := idToken.Claims(&claims); err != nil {
        return nil, domain.ErrInvalidToken
    }

    // Keycloak groups → Nullus 역할 매핑
    role := mapGroupsToRole(claims.Groups)

    // DB에서 사용자 조회 또는 JIT(Just-In-Time) 프로비저닝
    user, err := p.userRepo.FindByExternalID(ctx, claims.Subject)
    if err != nil {
        return nil, err
    }
    if user == nil {
        user = &domain.User{
            Email:      claims.Email,
            Name:       claims.Name,
            ExternalID: claims.Subject,
            Role:       role,
            Active:     true,
        }
        if err := p.userRepo.Save(ctx, user); err != nil {
            return nil, err
        }
    }
    return user, nil
}

func (p *KeycloakAuthProvider) Login(_ context.Context, _ *http.Request, w http.ResponseWriter, _ port.Credentials) (*domain.User, error) {
    // OIDC 방식에서 Login은 브라우저를 Keycloak으로 리다이렉트합니다.
    http.Redirect(w, nil, p.oauth2Cfg.AuthCodeURL("state"), http.StatusFound)
    return nil, nil
}

func (p *KeycloakAuthProvider) Logout(_ context.Context, _ *http.Request, w http.ResponseWriter) error {
    // Keycloak logout endpoint로 리다이렉트
    return nil
}

// mapGroupsToRole 은 Keycloak groups 클레임을 Nullus Role로 변환합니다.
func mapGroupsToRole(groups []string) domain.Role {
    for _, g := range groups {
        switch g {
        case "/nullus-admins", "nullus-admins":
            return domain.RoleAdmin
        case "/nullus-devops", "nullus-devops":
            return domain.RoleDevOps
        }
    }
    return domain.RoleDeveloper
}
```

### 3.7 미들웨어

```go
// internal/middleware/auth.go

package middleware

import (
    "context"
    "net/http"

    "github.com/cloud-nullus/nullus/internal/auth/domain"
    "github.com/cloud-nullus/nullus/internal/auth/port"
)

// contextKey 는 컨텍스트 키 타입 충돌을 방지하기 위한 private 타입입니다.
type contextKey string

const contextKeyUser contextKey = "current_user"

// AuthMiddleware 는 모든 보호된 경로에서 인증을 강제합니다.
func AuthMiddleware(provider port.AuthProvider) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, err := provider.Authenticate(r.Context(), r)
            if err != nil {
                http.Error(w, `{"error":{"code":"UNAUTHENTICATED","message":"인증이 필요합니다"}}`, http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), contextKeyUser, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// RequireRole 은 특정 역할 이상의 권한을 가진 사용자만 접근을 허용합니다.
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
    allowed := make(map[domain.Role]bool, len(roles))
    for _, r := range roles {
        allowed[r] = true
    }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := CurrentUser(r.Context())
            if user == nil || !allowed[user.Role] {
                http.Error(w, `{"error":{"code":"FORBIDDEN","message":"권한이 없습니다"}}`, http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// CurrentUser 는 컨텍스트에서 현재 사용자를 꺼냅니다.
func CurrentUser(ctx context.Context) *domain.User {
    u, _ := ctx.Value(contextKeyUser).(*domain.User)
    return u
}
```

---

## 4. 도메인 이벤트 설계

도메인 이벤트는 모듈 간 결합도를 낮추고 감사(audit) 로그 생성, 알림 트리거 등 부수 효과를 선언적으로 표현하기 위해 사용합니다.

### 4.1 이벤트 목록

| 이벤트 이름 | 발행 모듈 | 주요 구독자 |
|-------------|-----------|-------------|
| `PipelineCreated` | cicd | observability (감사 로그) |
| `PipelineDeployed` | cicd | observability (메트릭 업데이트) |
| `DeploymentFailed` | cicd | observability (알림 트리거) |
| `DeploymentRolledBack` | cicd | observability (감사 로그) |
| `StackDeployed` | install | observability, cicd |
| `StackRolledBack` | install | observability |
| `AlertTriggered` | observability | (알림 발송 — 이벤트 핸들러 내 처리) |
| `AlertRuleCreated` | observability | (감사 로그) |
| `UserInvited` | auth | (이메일 발송) |
| `UserRoleChanged` | auth | (감사 로그) |

### 4.2 공용 이벤트 타입

```go
// internal/events/events.go

package events

import (
    "time"

    "github.com/google/uuid"
)

// DomainEvent 는 모든 도메인 이벤트의 공통 인터페이스입니다.
type DomainEvent interface {
    EventName() string
    EventID() uuid.UUID
    OccurredAt() time.Time
}

// BaseEvent 는 공통 필드를 포함하는 기반 구조체입니다.
type BaseEvent struct {
    ID         uuid.UUID `json:"event_id"`
    Name       string    `json:"event_name"`
    OccurredAt_ time.Time `json:"occurred_at"`
}

func (e BaseEvent) EventName() string     { return e.Name }
func (e BaseEvent) EventID() uuid.UUID    { return e.ID }
func (e BaseEvent) OccurredAt() time.Time { return e.OccurredAt_ }

func newBase(name string) BaseEvent {
    return BaseEvent{
        ID:         uuid.New(),
        Name:       name,
        OccurredAt_: time.Now().UTC(),
    }
}

// --- cicd 이벤트 ---

// PipelineCreatedEvent 는 파이프라인이 생성되었을 때 발행됩니다.
type PipelineCreatedEvent struct {
    BaseEvent
    PipelineID uuid.UUID `json:"pipeline_id"`
    OrgID      uuid.UUID `json:"org_id"`
    Name       string    `json:"name"`
}

func NewPipelineCreatedEvent(pipelineID, orgID uuid.UUID, name string) PipelineCreatedEvent {
    return PipelineCreatedEvent{
        BaseEvent:  newBase("PipelineCreated"),
        PipelineID: pipelineID,
        OrgID:      orgID,
        Name:       name,
    }
}

// PipelineDeployedEvent 는 파이프라인 배포가 성공했을 때 발행됩니다.
type PipelineDeployedEvent struct {
    BaseEvent
    PipelineID   uuid.UUID `json:"pipeline_id"`
    DeploymentID uuid.UUID `json:"deployment_id"`
    Revision     int       `json:"revision"`
    DeployedBy   uuid.UUID `json:"deployed_by"`
}

// DeploymentFailedEvent 는 배포 실패 시 발행됩니다.
type DeploymentFailedEvent struct {
    BaseEvent
    PipelineID   uuid.UUID `json:"pipeline_id"`
    DeploymentID uuid.UUID `json:"deployment_id"`
    Reason       string    `json:"reason"`
}

// DeploymentRolledBackEvent 는 롤백 완료 시 발행됩니다.
type DeploymentRolledBackEvent struct {
    BaseEvent
    PipelineID   uuid.UUID `json:"pipeline_id"`
    DeploymentID uuid.UUID `json:"deployment_id"`
    ToRevision   int       `json:"to_revision"`
}

// StackDeployedEvent 는 DevSecOps 스택 전체 배포 완료 시 발행됩니다.
type StackDeployedEvent struct {
    BaseEvent
    StackID   uuid.UUID `json:"stack_id"`
    OrgID     uuid.UUID `json:"org_id"`
    ClusterID uuid.UUID `json:"cluster_id"`
}

// --- observability 이벤트 ---

// AlertTriggeredEvent 는 알림 규칙이 발동되었을 때 발행됩니다.
type AlertTriggeredEvent struct {
    BaseEvent
    AlertID  uuid.UUID `json:"alert_id"`
    RuleID   uuid.UUID `json:"rule_id"`
    OrgID    uuid.UUID `json:"org_id"`
    Severity string    `json:"severity"`
    Message  string    `json:"message"`
}
```

### 4.3 EventBus 인터페이스 및 인메모리 구현

```go
// internal/events/bus.go

package events

import (
    "context"
    "log/slog"
    "sync"
)

// Handler 는 이벤트 핸들러 함수 타입입니다.
type Handler func(ctx context.Context, event DomainEvent) error

// EventBus 는 동기 인메모리 이벤트 버스입니다.
// Phase 1에서는 동기 처리로 충분합니다.
// 향후 Kafka/NATS로 교체 시 이 인터페이스만 재구현합니다.
type EventBus struct {
    mu       sync.RWMutex
    handlers map[string][]Handler
}

func NewEventBus() *EventBus {
    return &EventBus{handlers: make(map[string][]Handler)}
}

// Subscribe 는 이벤트 이름에 핸들러를 등록합니다.
func (b *EventBus) Subscribe(eventName string, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[eventName] = append(b.handlers[eventName], handler)
}

// Publish 는 이벤트를 동기적으로 모든 핸들러에 전달합니다.
// 핸들러 에러는 로그만 남기고 다른 핸들러 실행을 중단하지 않습니다.
func (b *EventBus) Publish(ctx context.Context, event DomainEvent) error {
    b.mu.RLock()
    handlers := b.handlers[event.EventName()]
    b.mu.RUnlock()

    for _, h := range handlers {
        if err := h(ctx, event); err != nil {
            slog.Error("이벤트 핸들러 오류",
                "event", event.EventName(),
                "event_id", event.EventID(),
                "error", err,
            )
        }
    }
    return nil
}
```

### 4.4 이벤트 핸들러 등록 패턴

```go
// internal/app/event_handlers.go

package app

import (
    "context"
    "log/slog"

    "github.com/cloud-nullus/nullus/internal/events"
)

// RegisterEventHandlers 는 애플리케이션 시작 시 이벤트 핸들러를 한 곳에서 등록합니다.
func RegisterEventHandlers(bus *events.EventBus) {
    // 파이프라인 배포 실패 → 알림 평가 트리거
    bus.Subscribe("DeploymentFailed", func(ctx context.Context, e events.DomainEvent) error {
        ev, ok := e.(events.DeploymentFailedEvent)
        if !ok {
            return nil
        }
        slog.Info("배포 실패 이벤트 수신",
            "pipeline_id", ev.PipelineID,
            "reason", ev.Reason,
        )
        // 알림 평가 유스케이스 호출 (별도 주입 필요)
        return nil
    })

    // 스택 배포 완료 → 감사 로그 기록
    bus.Subscribe("StackDeployed", func(ctx context.Context, e events.DomainEvent) error {
        ev, ok := e.(events.StackDeployedEvent)
        if !ok {
            return nil
        }
        slog.Info("스택 배포 완료",
            "stack_id", ev.StackID,
            "org_id", ev.OrgID,
            "cluster_id", ev.ClusterID,
        )
        return nil
    })
}
```

---

## 5. 에러 처리 전략

### 5.1 에러 분류 원칙

Nullus는 에러를 두 계층으로 분리합니다.

- **도메인 에러**: 비즈니스 규칙 위반. 클라이언트에 명확한 메시지를 전달해야 합니다.
- **인프라 에러**: DB 연결 실패, 네트워크 오류 등. 내부 정보를 숨기고 재시도 가능 여부를 표시합니다.

### 5.2 에러 코드 체계

```
{DOMAIN}_{ENTITY}_{REASON}

예시:
  CICD_PIPELINE_NOT_FOUND
  CICD_DEPLOYMENT_MANIFEST_MISSING
  AUTH_USER_INVALID_CREDENTIALS
  AUTH_SESSION_SAVE_FAILED
  CLUSTER_KUBECONFIG_INVALID
  INSTALL_HELM_TIMEOUT
  INSTALL_PHASE_GATE_FAILED
```

### 5.3 도메인 에러 정의

```go
// internal/apperrors/errors.go

package apperrors

import (
    "errors"
    "fmt"
    "net/http"
)

// AppError 는 에러 코드, HTTP 상태코드, 재시도 가능 여부를 포함하는 구조체 에러입니다.
type AppError struct {
    Code       string
    HTTPStatus int
    Message    string
    Detail     string
    Retryable  bool
    // Unwrap 가능하도록 원인 에러를 보존합니다.
    Cause      error
}

func (e *AppError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

// New 는 AppError를 생성합니다.
func New(code string, httpStatus int, message string) *AppError {
    return &AppError{Code: code, HTTPStatus: httpStatus, Message: message}
}

// Wrap 은 인프라 에러를 AppError로 감쌉니다.
func Wrap(code string, httpStatus int, message string, cause error) *AppError {
    return &AppError{Code: code, HTTPStatus: httpStatus, Message: message, Cause: cause}
}

// --- 미리 정의된 도메인 에러들 ---

var (
    // cicd
    ErrPipelineNotFound       = New("CICD_PIPELINE_NOT_FOUND", http.StatusNotFound, "파이프라인을 찾을 수 없습니다")
    ErrPipelineNameRequired   = New("CICD_PIPELINE_NAME_REQUIRED", http.StatusBadRequest, "파이프라인 이름은 필수입니다")
    ErrGitRepoURLRequired     = New("CICD_GIT_REPO_URL_REQUIRED", http.StatusBadRequest, "Git 저장소 URL은 필수입니다")
    ErrTemplateNotFound       = New("CICD_TEMPLATE_NOT_FOUND", http.StatusNotFound, "파이프라인 템플릿을 찾을 수 없습니다")
    ErrDeploymentNotFound     = New("CICD_DEPLOYMENT_NOT_FOUND", http.StatusNotFound, "배포 기록을 찾을 수 없습니다")
    ErrManifestSnapshotMissing = New("CICD_DEPLOYMENT_MANIFEST_MISSING", http.StatusUnprocessableEntity, "롤백에 필요한 매니페스트 스냅샷이 없습니다")

    // observability
    ErrAlertRuleNameRequired  = New("OBS_ALERT_RULE_NAME_REQUIRED", http.StatusBadRequest, "알림 규칙 이름은 필수입니다")
    ErrAlertRuleNotFound      = New("OBS_ALERT_RULE_NOT_FOUND", http.StatusNotFound, "알림 규칙을 찾을 수 없습니다")

    // auth
    ErrInvalidCredentials     = New("AUTH_USER_INVALID_CREDENTIALS", http.StatusUnauthorized, "이메일 또는 비밀번호가 잘못되었습니다")
    ErrUserInactive           = New("AUTH_USER_INACTIVE", http.StatusForbidden, "비활성화된 계정입니다")
    ErrUnauthenticated        = New("AUTH_UNAUTHENTICATED", http.StatusUnauthorized, "인증이 필요합니다")
    ErrForbidden              = New("AUTH_FORBIDDEN", http.StatusForbidden, "권한이 없습니다")
    ErrInvalidToken           = New("AUTH_INVALID_TOKEN", http.StatusUnauthorized, "유효하지 않은 토큰입니다")
    ErrSessionSave            = New("AUTH_SESSION_SAVE_FAILED", http.StatusInternalServerError, "세션 저장에 실패했습니다")

    // install
    ErrHelmTimeout            = &AppError{Code: "INSTALL_HELM_TIMEOUT", HTTPStatus: http.StatusGatewayTimeout, Message: "Helm 차트 설치 시간 초과", Retryable: true}
    ErrPhaseGateFailed        = &AppError{Code: "INSTALL_PHASE_GATE_FAILED", HTTPStatus: http.StatusUnprocessableEntity, Message: "이전 설치 단계 완료 확인 실패", Retryable: true}
)

// IsNotFound 는 에러가 404 Not Found인지 확인합니다.
func IsNotFound(err error) bool {
    var ae *AppError
    return errors.As(err, &ae) && ae.HTTPStatus == http.StatusNotFound
}
```

### 5.4 HTTP 핸들러에서의 에러 변환

```go
// internal/handler/error_response.go

package handler

import (
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"

    "github.com/cloud-nullus/nullus/internal/apperrors"
    "github.com/google/uuid"
)

// ErrorResponse 는 표준 에러 응답 형식입니다 (PRD 3.1 표준 에러 응답 형식 준수).
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code       string `json:"code"`
    HTTPStatus int    `json:"http_status"`
    Message    string `json:"message"`
    Detail     string `json:"detail,omitempty"`
    Retryable  bool   `json:"retryable"`
    TraceID    string `json:"trace_id"`
}

// RespondError 는 에러를 표준 JSON 응답으로 변환합니다.
// 인프라 에러는 내부 정보를 숨기고 500을 반환합니다.
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
    traceID := uuid.NewString()

    var ae *apperrors.AppError
    if errors.As(err, &ae) {
        writeJSON(w, ae.HTTPStatus, ErrorResponse{
            Error: ErrorDetail{
                Code:       ae.Code,
                HTTPStatus: ae.HTTPStatus,
                Message:    ae.Message,
                Detail:     ae.Detail,
                Retryable:  ae.Retryable,
                TraceID:    traceID,
            },
        })
        return
    }

    // 미분류 에러는 500으로 처리하고 서버 로그에만 기록
    slog.Error("미처리 에러",
        "trace_id", traceID,
        "path", r.URL.Path,
        "error", err,
    )
    writeJSON(w, http.StatusInternalServerError, ErrorResponse{
        Error: ErrorDetail{
            Code:       "INTERNAL_SERVER_ERROR",
            HTTPStatus: http.StatusInternalServerError,
            Message:    "서버 내부 오류가 발생했습니다",
            Retryable:  false,
            TraceID:    traceID,
        },
    })
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(body)
}
```

---

## 6. 의존성 주입

Nullus는 외부 코드 생성 도구(wire) 없이 **수동 DI**를 사용합니다. 이유는 다음과 같습니다.

- 코드베이스가 아직 작고 wire 학습 비용 대비 효과가 낮음
- 빌드 타임 코드 생성 없이 디버깅이 단순함
- 향후 규모 확장 시 google/wire 전환 가능하도록 인터페이스 구조 유지

### 6.1 모듈별 Provider 패턴

```go
// internal/cicd/provider.go

package cicd

import (
    "database/sql"

    "github.com/cloud-nullus/nullus/internal/cicd/adapter/k8s"
    postgresadapter "github.com/cloud-nullus/nullus/internal/cicd/adapter/postgres"
    "github.com/cloud-nullus/nullus/internal/cicd/port"
    "github.com/cloud-nullus/nullus/internal/cicd/usecase"
    "github.com/cloud-nullus/nullus/internal/events"
    "k8s.io/client-go/rest"
)

// Dependencies 는 cicd 모듈이 외부에서 주입받는 의존성 묶음입니다.
type Dependencies struct {
    DB          *sql.DB
    K8sConfig   *rest.Config
    EventBus    *events.EventBus
}

// Module 은 cicd 모듈의 유스케이스와 어댑터를 보유합니다.
type Module struct {
    CreatePipeline    *usecase.CreatePipelineUseCase
    DeployPipeline    *usecase.DeployPipelineUseCase
    RollbackDeployment *usecase.RollbackDeploymentUseCase

    // 핸들러 레이어에서 직접 접근이 필요한 repository
    PipelineRepo   port.PipelineRepository
    DeploymentRepo port.DeploymentRepository
}

// NewModule 은 cicd 모듈의 모든 의존성을 조립합니다.
func NewModule(deps Dependencies) *Module {
    // Adapter 생성
    pipelineRepo := postgresadapter.NewPipelineRepository(deps.DB)
    templateRepo := postgresadapter.NewPipelineTemplateRepository(deps.DB)
    deploymentRepo := postgresadapter.NewDeploymentRepository(deps.DB)
    k8sDeployer := k8s.NewK8sDeployer(deps.K8sConfig)

    // UseCase 조립
    return &Module{
        CreatePipeline: usecase.NewCreatePipelineUseCase(
            pipelineRepo, templateRepo, deps.EventBus,
        ),
        DeployPipeline: usecase.NewDeployPipelineUseCase(
            pipelineRepo, deploymentRepo, k8sDeployer, deps.EventBus,
        ),
        RollbackDeployment: usecase.NewRollbackDeploymentUseCase(
            pipelineRepo, deploymentRepo, k8sDeployer, deps.EventBus,
        ),
        PipelineRepo:   pipelineRepo,
        DeploymentRepo: deploymentRepo,
    }
}
```

```go
// internal/observability/provider.go

package observability

import (
    "database/sql"

    "github.com/cloud-nullus/nullus/internal/events"
    prometheusadapter "github.com/cloud-nullus/nullus/internal/observability/adapter/prometheus"
    postgresadapter "github.com/cloud-nullus/nullus/internal/observability/adapter/postgres"
    "github.com/cloud-nullus/nullus/internal/observability/adapter/notifier"
    "github.com/cloud-nullus/nullus/internal/observability/usecase"
    "github.com/cloud-nullus/nullus/internal/config"
)

type Dependencies struct {
    DB          *sql.DB
    EventBus    *events.EventBus
    Config      *config.Config
}

type Module struct {
    GetDashboard      *usecase.GetDashboardUseCase
    CreateAlertRule   *usecase.CreateAlertRuleUseCase
    EvaluateAlerts    *usecase.EvaluateAlertsUseCase
}

func NewModule(deps Dependencies) *Module {
    metricsProvider := prometheusadapter.NewMetricsProvider(deps.Config.Prometheus.URL)
    alertRepo := postgresadapter.NewAlertRepository(deps.DB)
    multiNotifier := notifier.NewMultiNotifier(
        notifier.NewSlackNotifier(deps.Config.Slack.WebhookURL),
    )

    return &Module{
        GetDashboard:    usecase.NewGetDashboardUseCase(metricsProvider, alertRepo),
        CreateAlertRule: usecase.NewCreateAlertRuleUseCase(alertRepo, deps.EventBus),
        EvaluateAlerts:  usecase.NewEvaluateAlertsUseCase(alertRepo, metricsProvider, multiNotifier, deps.EventBus),
    }
}
```

### 6.2 애플리케이션 전체 조립

```go
// cmd/server/main.go

package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/cloud-nullus/nullus/internal/app"
    "github.com/cloud-nullus/nullus/internal/auth"
    "github.com/cloud-nullus/nullus/internal/cicd"
    "github.com/cloud-nullus/nullus/internal/config"
    "github.com/cloud-nullus/nullus/internal/events"
    "github.com/cloud-nullus/nullus/internal/observability"
    "github.com/cloud-nullus/nullus/internal/store"
)

func main() {
    cfg := config.MustLoad()

    db := store.MustConnect(cfg.Database)
    defer db.Close()

    k8sCfg := store.MustK8sConfig(cfg.Kubernetes)

    bus := events.NewEventBus()

    // 모듈별 조립
    authModule := auth.NewModule(auth.Dependencies{
        DB:     db,
        Config: cfg,
    })
    cicdModule := cicd.NewModule(cicd.Dependencies{
        DB:        db,
        K8sConfig: k8sCfg,
        EventBus:  bus,
    })
    obsModule := observability.NewModule(observability.Dependencies{
        DB:       db,
        EventBus: bus,
        Config:   cfg,
    })

    // 이벤트 핸들러 등록
    app.RegisterEventHandlers(bus)

    // HTTP 라우터 조립
    router := app.NewRouter(authModule, cicdModule, obsModule)

    srv := &http.Server{
        Addr:         cfg.Server.Addr,
        Handler:      router,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 30 * time.Second,
    }

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        slog.Info("서버 시작", "addr", cfg.Server.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("서버 시작 실패", "error", err)
            os.Exit(1)
        }
    }()

    <-quit
    slog.Info("서버 종료 시작")
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        slog.Error("서버 강제 종료", "error", err)
    }
    slog.Info("서버 종료 완료")
}
```

---

## 7. 테스트 전략

### 7.1 테스트 레이어 분류

| 레이어 | 목적 | 도구 | 커버리지 목표 |
|--------|------|------|--------------|
| 단위 테스트 | 도메인 엔티티, 유스케이스 순수 로직 | 표준 `testing`, `testify` | >80% |
| 통합 테스트 | Repository 어댑터 (실제 DB 대상) | `testcontainers-go` | 주요 경로 전체 |
| E2E 테스트 | HTTP 핸들러 + 라우터 + DB | `httptest` + testcontainers | Happy Path |

### 7.2 단위 테스트 — 유스케이스

유스케이스 테스트는 포트 인터페이스를 모킹하여 외부 의존성 없이 실행합니다.

```go
// internal/cicd/usecase/create_pipeline_test.go

package usecase_test

import (
    "context"
    "errors"
    "testing"

    "github.com/cloud-nullus/nullus/internal/cicd/domain"
    "github.com/cloud-nullus/nullus/internal/cicd/usecase"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// --- Mock 구현 ---

type mockPipelineRepo struct {
    saved *domain.Pipeline
}

func (m *mockPipelineRepo) Save(_ context.Context, p *domain.Pipeline) error {
    m.saved = p
    return nil
}
func (m *mockPipelineRepo) FindByID(_ context.Context, _ uuid.UUID) (*domain.Pipeline, error) {
    return nil, nil
}
func (m *mockPipelineRepo) FindByOrgID(_ context.Context, _ uuid.UUID) ([]*domain.Pipeline, error) {
    return nil, nil
}
func (m *mockPipelineRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

type mockTemplateRepo struct {
    template *domain.PipelineTemplate
}

func (m *mockTemplateRepo) FindByID(_ context.Context, _ string) (*domain.PipelineTemplate, error) {
    return m.template, nil
}
func (m *mockTemplateRepo) FindAll(_ context.Context) ([]*domain.PipelineTemplate, error) {
    return nil, nil
}

type mockEventBus struct {
    published []interface{}
}

func (m *mockEventBus) Publish(_ context.Context, event interface{}) error {
    m.published = append(m.published, event)
    return nil
}

// --- 테스트 ---

func TestCreatePipelineUseCase_Execute_Success(t *testing.T) {
    pipelineRepo := &mockPipelineRepo{}
    templateRepo := &mockTemplateRepo{
        template: &domain.PipelineTemplate{
            ID:   "web-backend-v1",
            Name: "Web Backend",
            Parameters: []domain.ParameterDef{
                {Key: "image_name", Required: true},
            },
        },
    }
    bus := &mockEventBus{}

    uc := usecase.NewCreatePipelineUseCase(pipelineRepo, templateRepo, bus)

    orgID := uuid.New()
    clusterID := uuid.New()
    out, err := uc.Execute(context.Background(), usecase.CreatePipelineInput{
        OrgID:      orgID,
        TemplateID: "web-backend-v1",
        Name:       "my-api",
        GitRepoURL: "https://github.com/org/repo",
        ClusterID:  clusterID,
        Parameters: map[string]string{"image_name": "my-api:latest"},
    })

    require.NoError(t, err)
    assert.NotNil(t, out.Pipeline)
    assert.Equal(t, "my-api", out.Pipeline.Name)
    assert.NotNil(t, pipelineRepo.saved)
    assert.Len(t, bus.published, 1, "PipelineCreated 이벤트가 발행되어야 합니다")
}

func TestCreatePipelineUseCase_Execute_MissingRequiredParam(t *testing.T) {
    templateRepo := &mockTemplateRepo{
        template: &domain.PipelineTemplate{
            ID: "web-backend-v1",
            Parameters: []domain.ParameterDef{
                {Key: "image_name", Required: true},
            },
        },
    }
    uc := usecase.NewCreatePipelineUseCase(&mockPipelineRepo{}, templateRepo, &mockEventBus{})

    _, err := uc.Execute(context.Background(), usecase.CreatePipelineInput{
        OrgID:      uuid.New(),
        TemplateID: "web-backend-v1",
        Name:       "my-api",
        GitRepoURL: "https://github.com/org/repo",
        ClusterID:  uuid.New(),
        Parameters: map[string]string{}, // image_name 누락
    })

    require.Error(t, err)
    assert.True(t, errors.Is(err, domain.ErrMissingParameter("image_name")))
}
```

### 7.3 통합 테스트 — Repository (testcontainers)

```go
// internal/cicd/adapter/postgres/pipeline_repo_integration_test.go

package postgres_test

import (
    "context"
    "database/sql"
    "testing"

    postgresadapter "github.com/cloud-nullus/nullus/internal/cicd/adapter/postgres"
    "github.com/cloud-nullus/nullus/internal/cicd/domain"
    "github.com/cloud-nullus/nullus/internal/testhelper"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    _ "github.com/lib/pq"
)

func TestPipelineRepository_SaveAndFind(t *testing.T) {
    // testcontainers로 PostgreSQL 컨테이너 실행
    ctx := context.Background()
    db := testhelper.SetupPostgres(t, ctx)
    testhelper.RunMigrations(t, db)

    repo := postgresadapter.NewPipelineRepository(db)

    orgID := uuid.New()
    clusterID := uuid.New()
    p := &domain.Pipeline{
        ID:         uuid.New(),
        OrgID:      orgID,
        Name:       "test-pipeline",
        GitRepoURL: "https://github.com/test/repo",
        ClusterID:  clusterID,
        Namespace:  "test-ns",
        Parameters: map[string]string{"key": "value"},
    }

    // Save
    err := repo.Save(ctx, p)
    require.NoError(t, err)

    // FindByID
    found, err := repo.FindByID(ctx, p.ID)
    require.NoError(t, err)
    require.NotNil(t, found)
    assert.Equal(t, p.Name, found.Name)
    assert.Equal(t, p.GitRepoURL, found.GitRepoURL)
    assert.Equal(t, "value", found.Parameters["key"])
}
```

### 7.4 테스트 헬퍼

```go
// internal/testhelper/postgres.go

package testhelper

import (
    "context"
    "database/sql"
    "fmt"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

// SetupPostgres 는 테스트용 PostgreSQL 컨테이너를 시작하고 *sql.DB를 반환합니다.
// 테스트 종료 시 컨테이너를 자동으로 정리합니다(t.Cleanup).
func SetupPostgres(t *testing.T, ctx context.Context) *sql.DB {
    t.Helper()

    container, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:18-alpine"),
        postgres.WithDatabase("nullus_test"),
        postgres.WithUsername("nullus"),
        postgres.WithPassword("nullus_secret"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
        ),
    )
    if err != nil {
        t.Fatalf("PostgreSQL 컨테이너 시작 실패: %v", err)
    }
    t.Cleanup(func() {
        if err := container.Terminate(ctx); err != nil {
            t.Logf("컨테이너 종료 실패: %v", err)
        }
    })

    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("연결 문자열 조회 실패: %v", err)
    }

    db, err := sql.Open("postgres", connStr)
    if err != nil {
        t.Fatalf("DB 연결 실패: %v", err)
    }
    t.Cleanup(func() { db.Close() })

    return db
}

// RunMigrations 는 테스트 DB에 마이그레이션을 적용합니다.
func RunMigrations(t *testing.T, db *sql.DB) {
    t.Helper()
    // golang-migrate 사용
    // m, err := migrate.New("file://../../migrations", connURL)
    // 간략화: SQL 직접 실행으로 대체 가능
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS pipelines (
            id UUID PRIMARY KEY,
            org_id UUID NOT NULL,
            name TEXT NOT NULL,
            git_repo_url TEXT NOT NULL,
            cluster_id UUID NOT NULL,
            namespace TEXT NOT NULL,
            parameters JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
    `)
    if err != nil {
        t.Fatalf("마이그레이션 실패: %v", err)
    }
}
```

---

## 8. 설정 관리

### 8.1 환경 변수 및 Config 구조체

Nullus는 **Viper**를 사용하여 환경 변수, YAML 파일, 기본값을 우선순위 순으로 병합합니다.

```
환경 변수 > config.yaml > 기본값
```

```go
// internal/config/config.go

package config

import (
    "fmt"
    "strings"
    "time"

    "github.com/spf13/viper"
)

// Config 는 애플리케이션 전체 설정입니다.
type Config struct {
    Server     ServerConfig
    Database   DatabaseConfig
    Auth       AuthConfig
    Kubernetes KubernetesConfig
    Prometheus PrometheusConfig
    Slack      SlackConfig
    Log        LogConfig
}

type ServerConfig struct {
    Addr            string        `mapstructure:"addr"`
    ReadTimeout     time.Duration `mapstructure:"read_timeout"`
    WriteTimeout    time.Duration `mapstructure:"write_timeout"`
    ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
    DSN             string        `mapstructure:"dsn"`
    MaxOpenConns    int           `mapstructure:"max_open_conns"`
    MaxIdleConns    int           `mapstructure:"max_idle_conns"`
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type AuthConfig struct {
    // Mode 는 "session" (Alpha/Beta) 또는 "oidc" (v1)입니다.
    Mode           string `mapstructure:"mode"`
    SessionSecret  string `mapstructure:"session_secret"`

    // OIDC 설정 (Mode == "oidc"일 때 사용)
    OIDCIssuerURL  string `mapstructure:"oidc_issuer_url"`
    OIDCClientID   string `mapstructure:"oidc_client_id"`
    OIDCClientSecret string `mapstructure:"oidc_client_secret"`
    OIDCRedirectURL string `mapstructure:"oidc_redirect_url"`
}

type KubernetesConfig struct {
    // InCluster 가 true이면 Pod ServiceAccount로 인증합니다.
    InCluster  bool   `mapstructure:"in_cluster"`
    KubeConfig string `mapstructure:"kubeconfig_path"`
}

type PrometheusConfig struct {
    URL string `mapstructure:"url"`
}

type SlackConfig struct {
    WebhookURL string `mapstructure:"webhook_url"`
}

type LogConfig struct {
    // Level 은 "debug", "info", "warn", "error" 중 하나입니다.
    Level  string `mapstructure:"level"`
    Format string `mapstructure:"format"` // "json" | "text"
}

// MustLoad 는 설정을 로드합니다. 필수 설정 누락 시 panic합니다.
func MustLoad() *Config {
    cfg, err := Load()
    if err != nil {
        panic(fmt.Sprintf("설정 로드 실패: %v", err))
    }
    return cfg
}

// Load 는 Viper를 통해 설정을 로드합니다.
func Load() (*Config, error) {
    v := viper.New()

    // 환경 변수 매핑 (NULLUS_ 접두사)
    v.SetEnvPrefix("NULLUS")
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    v.AutomaticEnv()

    // config.yaml 파일 탐색
    v.SetConfigName("config")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    v.AddConfigPath("/etc/nullus")

    // 기본값 설정
    v.SetDefault("server.addr", ":8080")
    v.SetDefault("server.read_timeout", "10s")
    v.SetDefault("server.write_timeout", "30s")
    v.SetDefault("server.shutdown_timeout", "15s")
    v.SetDefault("database.max_open_conns", 25)
    v.SetDefault("database.max_idle_conns", 5)
    v.SetDefault("database.conn_max_lifetime", "5m")
    v.SetDefault("auth.mode", "session")
    v.SetDefault("log.level", "info")
    v.SetDefault("log.format", "json")

    // 파일이 없어도 에러 무시 (환경 변수만으로도 동작)
    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("설정 파일 읽기 실패: %w", err)
        }
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("설정 파싱 실패: %w", err)
    }

    if err := validate(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

// validate 는 필수 설정 필드를 검증합니다.
func validate(cfg *Config) error {
    if cfg.Database.DSN == "" {
        return fmt.Errorf("NULLUS_DATABASE_DSN 환경 변수가 필요합니다")
    }
    if cfg.Auth.Mode == "session" && cfg.Auth.SessionSecret == "" {
        return fmt.Errorf("NULLUS_AUTH_SESSION_SECRET 환경 변수가 필요합니다")
    }
    if cfg.Auth.Mode == "oidc" {
        if cfg.Auth.OIDCIssuerURL == "" || cfg.Auth.OIDCClientID == "" {
            return fmt.Errorf("OIDC 모드에서 OIDC_ISSUER_URL, OIDC_CLIENT_ID가 필요합니다")
        }
    }
    return nil
}
```

### 8.2 환경 변수 목록

```
# 서버
NULLUS_SERVER_ADDR=:8080

# 데이터베이스
NULLUS_DATABASE_DSN=postgres://nullus:secret@localhost:5432/nullus?sslmode=disable

# 인증 (Alpha/Beta)
NULLUS_AUTH_MODE=session
NULLUS_AUTH_SESSION_SECRET=<최소 32바이트 랜덤 문자열>

# 인증 (v1 GA — OIDC)
NULLUS_AUTH_MODE=oidc
NULLUS_AUTH_OIDC_ISSUER_URL=https://keycloak.example.com/realms/nullus
NULLUS_AUTH_OIDC_CLIENT_ID=nullus-backend
NULLUS_AUTH_OIDC_CLIENT_SECRET=<client_secret>
NULLUS_AUTH_OIDC_REDIRECT_URL=https://nullus.example.com/auth/callback

# Prometheus (observability 어댑터)
NULLUS_PROMETHEUS_URL=http://prometheus.nullus-monitoring.svc.cluster.local:9090

# Slack 알림
NULLUS_SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...

# 로그
NULLUS_LOG_LEVEL=info
NULLUS_LOG_FORMAT=json
```

---

## 9. 로깅/메트릭

### 9.1 Structured Logging (slog)

Go 1.21+의 표준 `log/slog` 패키지를 사용합니다. 별도 서드파티 없이 JSON/텍스트 포맷을 지원합니다.

```go
// internal/logger/logger.go

package logger

import (
    "log/slog"
    "os"
)

// Setup 은 애플리케이션 전역 slog 핸들러를 초기화합니다.
func Setup(level, format string) {
    var lvl slog.Level
    switch level {
    case "debug":
        lvl = slog.LevelDebug
    case "warn":
        lvl = slog.LevelWarn
    case "error":
        lvl = slog.LevelError
    default:
        lvl = slog.LevelInfo
    }

    opts := &slog.HandlerOptions{
        Level:     lvl,
        AddSource: lvl == slog.LevelDebug,
    }

    var handler slog.Handler
    if format == "json" {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    } else {
        handler = slog.NewTextHandler(os.Stdout, opts)
    }

    slog.SetDefault(slog.New(handler))
}
```

```go
// 사용 예시 — usecase 내에서의 구조화된 로깅

slog.Info("파이프라인 생성 완료",
    "pipeline_id", p.ID,
    "org_id", p.OrgID,
    "name", p.Name,
    "template_id", p.TemplateID,
)

slog.Error("Kubernetes 배포 실패",
    "pipeline_id", in.PipelineID,
    "deployment_id", dep.ID,
    "error", err,
)
```

### 9.2 요청 로깅 미들웨어

```go
// internal/middleware/logging.go

package middleware

import (
    "log/slog"
    "net/http"
    "time"

    "github.com/google/uuid"
)

// RequestLogging 은 모든 HTTP 요청에 대해 구조화된 액세스 로그를 기록합니다.
func RequestLogging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        traceID := uuid.NewString()

        // trace_id를 컨텍스트에 저장 (에러 응답에서 재사용)
        rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

        next.ServeHTTP(rw, r)

        duration := time.Since(start)
        slog.Info("HTTP 요청",
            "trace_id", traceID,
            "method", r.Method,
            "path", r.URL.Path,
            "status", rw.status,
            "duration_ms", duration.Milliseconds(),
            "remote_addr", r.RemoteAddr,
        )
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.status = code
    rw.ResponseWriter.WriteHeader(code)
}
```

### 9.3 Prometheus 커스텀 메트릭

```go
// internal/metrics/metrics.go

package metrics

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// NullusMetrics 는 Nullus 플랫폼 전용 Prometheus 메트릭 묶음입니다.
type NullusMetrics struct {
    // HTTP 레이어
    HTTPRequestsTotal    *prometheus.CounterVec
    HTTPRequestDuration  *prometheus.HistogramVec

    // 설치 엔진
    StackDeployTotal     *prometheus.CounterVec
    StackDeployDuration  *prometheus.HistogramVec

    // CI/CD 파이프라인
    PipelineDeployTotal  *prometheus.CounterVec
    PipelineDeployDuration *prometheus.HistogramVec

    // 알림
    AlertsFiredTotal     *prometheus.CounterVec
}

// NewNullusMetrics 는 메트릭을 등록하고 반환합니다.
// promauto를 사용하므로 별도 Register 호출이 불필요합니다.
func NewNullusMetrics() *NullusMetrics {
    return &NullusMetrics{
        HTTPRequestsTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: "nullus",
                Name:      "http_requests_total",
                Help:      "HTTP 요청 총 수",
            },
            []string{"method", "path", "status"},
        ),
        HTTPRequestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: "nullus",
                Name:      "http_request_duration_seconds",
                Help:      "HTTP 요청 처리 시간",
                Buckets:   prometheus.DefBuckets,
            },
            []string{"method", "path"},
        ),
        StackDeployTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: "nullus",
                Name:      "stack_deploy_total",
                Help:      "DevSecOps 스택 배포 총 수",
            },
            []string{"status", "template_id"},
        ),
        StackDeployDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: "nullus",
                Name:      "stack_deploy_duration_seconds",
                Help:      "DevSecOps 스택 배포 소요 시간",
                // 설치는 수분~수십분이므로 넓은 버킷 설정
                Buckets: []float64{30, 60, 120, 300, 600, 1200, 3600},
            },
            []string{"template_id"},
        ),
        PipelineDeployTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: "nullus",
                Name:      "pipeline_deploy_total",
                Help:      "CI/CD 파이프라인 배포 총 수",
            },
            []string{"status", "pipeline_type"},
        ),
        PipelineDeployDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: "nullus",
                Name:      "pipeline_deploy_duration_seconds",
                Help:      "CI/CD 파이프라인 배포 소요 시간",
                Buckets:   []float64{1, 5, 10, 30, 60, 120},
            },
            []string{"pipeline_type"},
        ),
        AlertsFiredTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: "nullus",
                Name:      "alerts_fired_total",
                Help:      "발동된 알림 총 수",
            },
            []string{"severity", "condition"},
        ),
    }
}

// Handler 는 /metrics 엔드포인트 핸들러를 반환합니다.
func Handler() http.Handler {
    return promhttp.Handler()
}
```

### 9.4 메트릭 미들웨어 적용

```go
// internal/middleware/metrics.go

package middleware

import (
    "net/http"
    "strconv"
    "time"

    "github.com/cloud-nullus/nullus/internal/metrics"
)

// MetricsMiddleware 는 HTTP 요청마다 Prometheus 메트릭을 기록합니다.
func MetricsMiddleware(m *metrics.NullusMetrics) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

            next.ServeHTTP(rw, r)

            duration := time.Since(start).Seconds()
            statusStr := strconv.Itoa(rw.status)

            m.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
            m.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
        })
    }
}
```

### 9.5 유스케이스에서의 메트릭 기록 패턴

유스케이스가 메트릭 라이브러리를 직접 참조하면 도메인 순수성이 깨집니다. 이를 방지하기 위해 **데코레이터 패턴**을 사용합니다.

```go
// internal/cicd/usecase/deploy_pipeline_metrics.go

package usecase

import (
    "context"
    "time"

    "github.com/cloud-nullus/nullus/internal/metrics"
)

// DeployPipelineMetricsDecorator 는 DeployPipeline 유스케이스에 메트릭을 추가합니다.
// 원본 유스케이스는 메트릭 코드를 전혀 모릅니다.
type DeployPipelineMetricsDecorator struct {
    inner   *DeployPipelineUseCase
    metrics *metrics.NullusMetrics
}

func NewDeployPipelineMetricsDecorator(inner *DeployPipelineUseCase, m *metrics.NullusMetrics) *DeployPipelineMetricsDecorator {
    return &DeployPipelineMetricsDecorator{inner: inner, metrics: m}
}

func (d *DeployPipelineMetricsDecorator) Execute(ctx context.Context, in DeployPipelineInput) (*DeployPipelineOutput, error) {
    start := time.Now()

    out, err := d.inner.Execute(ctx, in)

    duration := time.Since(start).Seconds()
    pipelineType := "unknown"
    if out != nil && out.Deployment != nil {
        // 파이프라인 타입은 별도 조회가 필요하므로 단순화
        pipelineType = "generic"
    }

    status := "success"
    if err != nil {
        status = "failure"
    }

    d.metrics.PipelineDeployTotal.WithLabelValues(status, pipelineType).Inc()
    d.metrics.PipelineDeployDuration.WithLabelValues(pipelineType).Observe(duration)

    return out, err
}
```

---

## 부록: 모듈 의존 관계 다이어그램

```
cmd/server
    └── internal/app
            ├── internal/auth
            │       ├── domain/
            │       ├── usecase/
            │       ├── port/
            │       └── adapter/
            │               ├── session/   ← Alpha/Beta
            │               └── oidc/      ← v1 GA
            ├── internal/cicd
            │       ├── domain/
            │       ├── usecase/
            │       ├── port/
            │       └── adapter/
            │               ├── postgres/
            │               └── k8s/
            ├── internal/observability
            │       ├── domain/
            │       ├── usecase/
            │       ├── port/
            │       └── adapter/
            │               ├── prometheus/
            │               ├── postgres/
            │               └── notifier/
            ├── internal/events           ← 공유 이벤트 버스
            ├── internal/apperrors        ← 공유 에러 타입
            ├── internal/config           ← Viper 기반 설정
            ├── internal/logger           ← slog 초기화
            ├── internal/metrics          ← Prometheus 메트릭
            └── internal/middleware       ← auth, logging, metrics

규칙:
- domain/ 은 외부 패키지를 import하지 않습니다 (stdlib 제외)
- usecase/ 는 port/ 인터페이스만 알고 adapter/ 를 import하지 않습니다
- adapter/ 는 domain/ 과 외부 라이브러리를 모두 알 수 있습니다
- events/, apperrors/ 는 모든 모듈에서 공유합니다
```

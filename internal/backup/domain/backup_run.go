package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// BackupRun 은 이 컨텍스트의 Aggregate Root 다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §7.1 (nullus-plan#75)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	// StatusPartial 은 일부 컴포넌트만 성공한 상태다.
	//
	// failed 로 뭉뚱그리면 "부분적으로 쓸 수 있는 백업" 과 "아무것도 없는
	// 상태" 가 구분되지 않는다. 복구 시점에 그 차이가 결정적이다.
	StatusPartial Status = "partial"
	StatusFailed  Status = "failed"
)

type Mode string

const (
	ModeFull         Mode = "full"
	ModePlatformOnly Mode = "platform_only"
	ModeStackOnly    Mode = "stack_only"
)

type Trigger string

const (
	TriggerManual   Trigger = "manual"
	TriggerSchedule Trigger = "schedule"
)

type Component string

const (
	ComponentPlatformDB         Component = "platform_db"
	ComponentKeycloakDB         Component = "keycloak_db"
	ComponentOpenBaoKV          Component = "openbao_kv"
	ComponentNamespaceResources Component = "ns_resources"
	ComponentVolume             Component = "volume"
)

// ParseComponents 는 사용자가 고른 백업 대상을 검증해 도메인 값으로 바꾼다.
//
// 모르는 이름은 **거부한다.** 조용히 버리면 사용자가 고른 것과 실제로 백업된
// 것이 달라지고, 그 사실은 복구할 때에야 드러난다 — 그때는 늦다.
//
// 빈 입력은 nil 을 돌려준다. 호출부가 "고르지 않았다"(모드에서 파생) 와
// "아무것도 고르지 않았다" 를 구분할 수 있어야 하기 때문이다.
func ParseComponents(raw []string) ([]Component, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	known := map[Component]bool{
		ComponentPlatformDB:         true,
		ComponentKeycloakDB:         true,
		ComponentOpenBaoKV:          true,
		ComponentNamespaceResources: true,
		ComponentVolume:             true,
	}
	out := make([]Component, 0, len(raw))
	seen := make(map[Component]bool, len(raw))
	for _, r := range raw {
		c := Component(strings.TrimSpace(r))
		if !known[c] {
			return nil, ErrInvalidComponent(r)
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out, nil
}

// ModeComponents 는 모드별 대상 컴포넌트를 돌려준다 (설계 §6.5).
func ModeComponents(mode Mode) []Component {
	platform := []Component{ComponentPlatformDB, ComponentKeycloakDB, ComponentOpenBaoKV}
	stack := []Component{ComponentNamespaceResources, ComponentVolume}
	switch mode {
	case ModePlatformOnly:
		return platform
	case ModeStackOnly:
		return stack
	default:
		return append(platform, stack...)
	}
}

type BackupRun struct {
	ID      string
	OrgID   string
	StackID string
	Trigger Trigger
	Mode    Mode
	Scope   []Component
	Status  Status

	SchemaVersion int

	QuiesceStartedAt *time.Time
	QuiesceEndedAt   *time.Time

	Manifest   map[string]any
	TotalBytes int64
	Error      string

	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

func NewBackupRun(orgID string, mode Mode, trigger Trigger, scope []Component) *BackupRun {
	return &BackupRun{
		OrgID:     orgID,
		Mode:      mode,
		Trigger:   trigger,
		Scope:     scope,
		Status:    StatusPending,
		Manifest:  map[string]any{},
		CreatedAt: time.Now().UTC(),
	}
}

// RequiresQuiesce 는 정지 창이 필요한지 알려준다.
//
// 볼륨을 뜨지 않는 백업은 무중단이어야 한다 — 이유 없이 서비스를 멈추면
// 사용자가 백업을 피하게 되고, 피해진 백업은 없는 백업이다.
func (r *BackupRun) RequiresQuiesce() bool {
	for _, c := range r.Scope {
		if c == ComponentVolume {
			return true
		}
	}
	return false
}

func (r *BackupRun) Start() error {
	if r.Status != StatusPending {
		return fmt.Errorf("백업 실행을 시작할 수 없습니다: 현재 상태 %s", r.Status)
	}
	now := time.Now().UTC()
	r.Status = StatusRunning
	r.StartedAt = &now
	return nil
}

func (r *BackupRun) BeginQuiesce() {
	now := time.Now().UTC()
	r.QuiesceStartedAt = &now
}

func (r *BackupRun) EndQuiesce() {
	now := time.Now().UTC()
	r.QuiesceEndedAt = &now
}

// Complete 는 컴포넌트별 결과로 최종 상태를 정한다.
//
// 결과가 비어 있으면 failed 다 — 아무것도 시도하지 못한 것은 성공이 아니다.
func (r *BackupRun) Complete(results map[Component]error) {
	now := time.Now().UTC()
	r.FinishedAt = &now

	if len(results) == 0 {
		r.Status = StatusFailed
		r.Error = "실행된 컴포넌트가 없습니다"
		return
	}

	failed := make([]string, 0, len(results))
	succeeded := 0
	for comp, err := range results {
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", comp, err))
			continue
		}
		succeeded++
	}
	sort.Strings(failed)

	switch {
	case len(failed) == 0:
		r.Status = StatusSucceeded
	case succeeded == 0:
		r.Status = StatusFailed
		r.Error = strings.Join(failed, "; ")
	default:
		r.Status = StatusPartial
		r.Error = strings.Join(failed, "; ")
	}
}

// Fail 은 컴포넌트 실행 전에 중단된 경우를 기록한다 (예: 목적지 검사 실패).
func (r *BackupRun) Fail(reason string) {
	now := time.Now().UTC()
	r.FinishedAt = &now
	r.Status = StatusFailed
	r.Error = reason
}

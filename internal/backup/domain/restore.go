package domain

import (
	"fmt"
	"strings"
	"time"
)

// 복구 전 검사. 설계 §6.1 0~2단계 (nullus-plan#75)

type RestoreMode = Mode

type RestoreStatus = Status

// SchemaState 는 복구 대상 DB 의 현재 마이그레이션 상태다 (golang-migrate).
type SchemaState struct {
	Version int
	Dirty   bool
}

type SchemaCheckResult struct {
	Allowed bool `json:"allowed"`
	// MigrateAfterRestore 는 복원 뒤에 migrate up 이 필요한지 알려준다.
	// 순서를 뒤집으면 실패한다 — 복원이 먼저다.
	MigrateAfterRestore bool   `json:"migrate_after_restore"`
	BackupVersion       int    `json:"backup_version"`
	CurrentVersion      int    `json:"current_version"`
	Reason              string `json:"reason,omitempty"`
}

// CheckSchemaVersion 은 백업본을 이 코드에 복원해도 되는지 판정한다.
func CheckSchemaVersion(backupVersion int, current SchemaState) SchemaCheckResult {
	res := SchemaCheckResult{BackupVersion: backupVersion, CurrentVersion: current.Version}

	switch {
	case current.Dirty:
		// 마이그레이션이 중단된 상태다. 무엇이 적용됐는지 알 수 없다.
		res.Reason = "현재 DB 가 dirty 상태입니다. 마이그레이션을 먼저 정리해야 합니다"
	case backupVersion <= 0:
		res.Reason = "백업본에 스키마 버전이 기록되어 있지 않습니다"
	case backupVersion > current.Version:
		// 구버전 코드가 신버전 스키마를 읽는 것은 정의되지 않은 동작이다.
		res.Reason = fmt.Sprintf("백업본 스키마 버전(%d)이 현재 코드(%d)보다 높습니다", backupVersion, current.Version)
	default:
		res.Allowed = true
		res.MigrateAfterRestore = backupVersion < current.Version
	}
	return res
}

// Prerequisites 는 복구 착수 전에 확보돼야 하는 것들이다 (설계 §6.1 0단계).
type Prerequisites struct {
	// EncryptionKeyPresent — clusters.kubeconfig 복호화 키. 없으면 복구해도
	// 등록된 어떤 클러스터도 조작할 수 없다 (설계 §1.3).
	EncryptionKeyPresent bool
	// BackupSealKeyPresent — 산출물 복호화 키.
	BackupSealKeyPresent bool
	// DestinationCredsPresent — 목적지 자격증명. 스택 OpenBao 가 아니라
	// 컨트롤 플레인에서 온다 (설계 §4.2.1).
	DestinationCredsPresent bool
	DestinationReachable    bool
}

// CheckPrerequisites 는 빠진 것을 모두 모아 한 번에 알려준다.
//
// 하나씩 알려주면 사용자가 고치고 다시 시도하기를 반복하게 되고, 복구는
// 그만큼 늦어진다. 재해 상황에서 그 시간이 비싸다.
func CheckPrerequisites(p Prerequisites) error {
	missing := make([]string, 0, 4)
	if !p.EncryptionKeyPresent {
		missing = append(missing, "ENCRYPTION_KEY (클러스터 kubeconfig 복호화 키)")
	}
	if !p.BackupSealKeyPresent {
		missing = append(missing, "백업 암호화 키")
	}
	if !p.DestinationCredsPresent {
		missing = append(missing, "백업 목적지 자격증명")
	}
	if !p.DestinationReachable {
		missing = append(missing, "백업 목적지 네트워크 도달성")
	}
	if len(missing) == 0 {
		return nil
	}
	return ErrPrerequisitesMissing(strings.Join(missing, ", "))
}

// DanglingReference 는 DB 가 가리키는데 금고에 없는 시크릿 경로다 (설계 §6.4).
type DanglingReference struct {
	TokenSourceID string `json:"token_source_id"`
	Path          string `json:"path"`
}

// IntegrityReport 는 복구 후 참조 정합성 검사 결과다.
//
// dangling 은 에러 없이 지나갔다가 파이프라인 실행 시점에 인증 실패로
// 드러난다. 조용히 넘기지 않는 것이 이 보고서의 존재 이유다.
type IntegrityReport struct {
	CheckedPaths int                 `json:"checked_paths"`
	Dangling     []DanglingReference `json:"dangling"`
	CheckedAt    time.Time           `json:"checked_at"`
}

func (r IntegrityReport) HasIssues() bool { return len(r.Dangling) > 0 }

// RestoreRun 은 한 번의 복구 실행이다.
type RestoreRun struct {
	ID              string
	BackupRunID     string
	Mode            RestoreMode
	Status          RestoreStatus
	SchemaCheck     SchemaCheckResult
	IntegrityReport IntegrityReport
	Error           string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
}

func NewRestoreRun(backupRunID string, mode RestoreMode) *RestoreRun {
	return &RestoreRun{
		BackupRunID: backupRunID,
		Mode:        mode,
		Status:      StatusPending,
		CreatedAt:   time.Now().UTC(),
	}
}

func (r *RestoreRun) Start() error {
	if r.Status != StatusPending {
		return fmt.Errorf("복구를 시작할 수 없습니다: 현재 상태 %s", r.Status)
	}
	now := time.Now().UTC()
	r.Status = StatusRunning
	r.StartedAt = &now
	return nil
}

func (r *RestoreRun) Succeed() {
	now := time.Now().UTC()
	r.Status = StatusSucceeded
	r.FinishedAt = &now
}

func (r *RestoreRun) Fail(reason string) {
	now := time.Now().UTC()
	r.Status = StatusFailed
	r.Error = reason
	r.FinishedAt = &now
}

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
	// GatewayAliases 는 복구 뒤 러너·Argo CD hostAliases 를 게이트웨이의 지금 주소로 맞춘 결과다.
	// 스택을 되돌리지 않았으면 없다.
	GatewayAliases *GatewayAliasesOutcome `json:"gateway_aliases,omitempty"`
	// SharedComponents 는 공용 자원을 되살리기 전에 cert-manager·metrics-server 를 다시 세운 결과다.
	// 스택을 되돌리지 않았거나 설치기가 배선되지 않았으면 없다.
	SharedComponents *SharedComponentsOutcome `json:"shared_components,omitempty"`
	// VaultImport 는 스택 금고로 KV 를 되돌린 결과다. 스택을 되돌리지 않았거나 백업에 금고 산출물이 없으면
	// 없다. 실패해도 복구는 성공이다 — 데이터는 되돌아왔고, 금고만 손으로(또는 다시 복구해서) 채울 수 있다.
	VaultImport *VaultImportOutcome `json:"vault_import,omitempty"`
}

// VaultImportOutcome 은 스택 금고 KV import 결과다. 로그로만 남기면 "안 돌았다" 와 "실패했다" 와
// "성공했다" 를 응답에서 가를 수 없다 — 2026-10-12 두 스택 리허설에서 한 스택의 import 가 끝내 실패했는데
// 복구 응답은 succeeded 였다. Error 가 있으면 그 스택의 도구들은 금고의 자격증명을 못 찾는다.
type VaultImportOutcome struct {
	Imported bool   `json:"imported"`
	Attempts int    `json:"attempts,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SharedComponentsOutcome 은 공용 컨트롤러 재설치 결과다. Steps 는 끝까지 돌린 설치 단계, Warnings 는
// 복구를 멈추지 않고 넘긴 실패다(metrics-server — HPA·자원 지표에만 쓰인다). cert-manager 실패는
// 여기 남지 않는다 — 복구가 그 자리에서 실패한다.
type SharedComponentsOutcome struct {
	Steps    []string `json:"steps,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// GatewayAliasesOutcome 은 게이트웨이 주소 재설정 결과다. Updated 가 비어 있으면 바꿀 것이 없었다 —
// 주소가 그대로였거나, 박아 둔 주소가 없었다(접근 도메인 없음·러너/Argo CD 릴리스 없음·설치 형식이
// 아닌 러너 설정). Error 가 있으면 일부 또는 전부를 맞추지 못했다 — 파이프라인이 "Could not resolve
// host" 로 멈출 수 있으니 손으로 맞춰야 한다(복구 자체는 성공이다).
type GatewayAliasesOutcome struct {
	GatewayIP string   `json:"gateway_ip,omitempty"`
	Updated   []string `json:"updated,omitempty"`
	Error     string   `json:"error,omitempty"`
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

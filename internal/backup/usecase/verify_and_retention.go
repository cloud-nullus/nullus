package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// VerifyUseCase 는 복원 없이 "이 백업이 열리기는 하는가" 를 확인한다.
//
// 복구 리허설을 상시로 돌릴 수는 없으므로, 팀이 겪은 "백업은 되는데 복원이
// 안 됐다" 에 대한 최소한의 상시 방어선이다 (설계 §7.2).
type VerifyUseCase struct {
	repo   port.BackupRepository
	store  port.ArtifactStore
	dumper port.DBDumper
	target port.DBTarget
	logger *slog.Logger
}

func NewVerifyUseCase(repo port.BackupRepository, store port.ArtifactStore, dumper port.DBDumper, target port.DBTarget, logger *slog.Logger) *VerifyUseCase {
	if logger == nil {
		logger = slog.Default()
	}
	return &VerifyUseCase{repo: repo, store: store, dumper: dumper, target: target, logger: logger}
}

type VerifyResult struct {
	BackupRunID    string                   `json:"backup_run_id"`
	OK             bool                     `json:"ok"`
	CheckedCount   int                      `json:"checked_count"`
	SchemaCheck    domain.SchemaCheckResult `json:"schema_check"`
	MissingObjects []string                 `json:"missing_objects,omitempty"`
	ChecksumFails  []string                 `json:"checksum_fails,omitempty"`
}

func (uc *VerifyUseCase) Verify(ctx context.Context, backupRunID string) (VerifyResult, error) {
	run, err := uc.repo.GetRun(ctx, backupRunID)
	if err != nil {
		return VerifyResult{}, err
	}
	arts, err := uc.repo.ListArtifacts(ctx, backupRunID)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("산출물 목록 조회: %w", err)
	}

	res := VerifyResult{BackupRunID: backupRunID, OK: true}
	for _, a := range arts {
		res.CheckedCount++
		_, sum, err := uc.store.Stat(ctx, locationKey(a.Location))
		if err != nil {
			res.MissingObjects = append(res.MissingObjects, a.Location)
			res.OK = false
			continue
		}
		if a.ChecksumSHA256 != "" && sum != a.ChecksumSHA256 {
			res.ChecksumFails = append(res.ChecksumFails, a.Location)
			res.OK = false
		}
	}

	// 지금 이 코드로 복원할 수 있는 백업인지도 함께 본다. 산출물이 멀쩡해도
	// 스키마가 안 맞으면 복원할 수 없다.
	if uc.dumper != nil {
		if current, err := uc.dumper.SchemaState(ctx, uc.target); err == nil {
			res.SchemaCheck = domain.CheckSchemaVersion(run.SchemaVersion, current)
			if !res.SchemaCheck.Allowed {
				res.OK = false
			}
		}
	}

	if len(arts) == 0 {
		res.OK = false
	}
	return res, nil
}

// RetentionUseCase 는 보존 정책을 적용한다.
//
// 산출물만 지우고 이력 행은 남긴다 — "언제 백업이 끊겼나" 의 근거가
// 사라지면 안 된다 (설계 §4.5).
type RetentionUseCase struct {
	repo   port.BackupRepository
	store  port.ArtifactStore
	policy domain.RetentionPolicy
	logger *slog.Logger
}

func NewRetentionUseCase(repo port.BackupRepository, store port.ArtifactStore, policy domain.RetentionPolicy, logger *slog.Logger) *RetentionUseCase {
	if logger == nil {
		logger = slog.Default()
	}
	return &RetentionUseCase{repo: repo, store: store, policy: policy, logger: logger}
}

func (uc *RetentionUseCase) Apply(ctx context.Context, orgID string) ([]string, error) {
	summaries, err := uc.repo.ListSummaries(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("백업 요약 조회: %w", err)
	}

	toDelete := uc.policy.SelectForDeletion(summaries)
	deleted := make([]string, 0, len(toDelete))
	for _, id := range toDelete {
		if err := uc.store.Delete(ctx, "backup-"+id+"/"); err != nil {
			// 오브젝트를 못 지웠는데 DB 기록만 지우면 고아 오브젝트가 남고
			// 용량은 계속 찬다. 여기서 멈추고 다음 주기에 다시 시도한다.
			uc.logger.Error("백업 산출물 삭제 실패", "backup_run_id", id, "error", err)
			continue
		}
		if err := uc.repo.DeleteArtifacts(ctx, id); err != nil {
			uc.logger.Error("산출물 기록 삭제 실패", "backup_run_id", id, "error", err)
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, nil
}

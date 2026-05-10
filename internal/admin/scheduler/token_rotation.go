package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/admin/rotation"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

type TokenRotationScheduler struct {
	pool       *pgxpool.Pool
	secret     *secrets.Router
	interval   time.Duration
	iterTimout time.Duration
	logger     *slog.Logger
	inFlight   atomic.Bool
	reissuer   rotation.Reissuer
}

func NewTokenRotationScheduler(pool *pgxpool.Pool, secret *secrets.Router, interval, iterTimeout time.Duration, logger *slog.Logger, reissuer rotation.Reissuer) *TokenRotationScheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if iterTimeout <= 0 {
		iterTimeout = 2 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	if reissuer == nil {
		reissuer = rotation.NewNoopReissuer()
	}
	return &TokenRotationScheduler{pool: pool, secret: secret, interval: interval, iterTimout: iterTimeout, logger: logger, reissuer: reissuer}
}

func (s *TokenRotationScheduler) Start(ctx context.Context) {
	s.runIteration(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("token rotation scheduler stopping")
			return
		case <-ticker.C:
			s.runIteration(ctx)
		}
	}
}

type dueTokenSource struct {
	ID               string
	Provider         string
	Path             string
	Type             string
	Status           string
	RequiresApproval bool
	ExpiresAt        *time.Time
	Metadata         map[string]any
}

func (s *TokenRotationScheduler) runIteration(ctx context.Context) {
	if s.pool == nil || s.secret == nil || !s.secret.Has("openbao") {
		return
	}
	if !s.inFlight.CompareAndSwap(false, true) {
		return
	}
	defer s.inFlight.Store(false)

	iterCtx, cancel := context.WithTimeout(ctx, s.iterTimout)
	defer cancel()

	rows, err := s.pool.Query(iterCtx, `
		SELECT id::text, provider, path, token_type, status, requires_approval, expires_at, metadata
		FROM token_sources
		WHERE deleted_at IS NULL
		  AND COALESCE((metadata->>'secret_manager'),'openbao')='openbao'
		  AND (next_check_at IS NULL OR next_check_at <= now())
		LIMIT 100`)
	if err != nil {
		s.logger.Warn("token rotation list failed", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		if iterCtx.Err() != nil {
			return
		}
		var item dueTokenSource
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Provider, &item.Path, &item.Type, &item.Status, &item.RequiresApproval, &item.ExpiresAt, &raw); err != nil {
			continue
		}
		_ = json.Unmarshal(raw, &item.Metadata)
		if item.ExpiresAt != nil && time.Now().UTC().After(item.ExpiresAt.UTC()) {
			s.markExpired(iterCtx, item, "TOKEN_ROTATE_EXPIRED")
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Type), "manual") || item.RequiresApproval {
			s.markApprovalRequired(iterCtx, item)
			continue
		}
		if err := s.rotateOne(iterCtx, item); err != nil {
			s.logger.Warn("token rotation failed", "token_source_id", item.ID, "error", err)
			retryDelay, retryCount := nextRetryDelay(item.Metadata)
			s.updateRetryMetadata(iterCtx, item.ID, item.Metadata, retryCount)
			_, _ = s.pool.Exec(iterCtx, `UPDATE token_sources SET status='failed_manual', next_check_at=now()+$2::interval, updated_at=now() WHERE id=$1::uuid`, item.ID, retryDelay)
			_, _ = s.pool.Exec(iterCtx, `INSERT INTO token_rotation_events (token_source_id, event_type, result, reason_code, detail_json) VALUES ($1::uuid,'rotate','failed',$2,$3::jsonb)`, item.ID, "ROTATION_FAILED", fmt.Sprintf(`{"retry_in":"%s","retry_count":%d}`, retryDelay, retryCount))
		}
	}
}

func (s *TokenRotationScheduler) rotateOne(ctx context.Context, item dueTokenSource) error {
	currentToken, _ := s.secret.GetToken(ctx, "openbao", item.Path)
	token, err := s.issueToken(ctx, item, currentToken)
	if err != nil {
		return err
	}
	if err := s.secret.PutToken(ctx, "openbao", item.Path, token); err != nil {
		return err
	}
	status := "rotated"
	if strings.TrimSpace(strings.ToLower(item.Type)) == "manual" {
		status = "renew_due"
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE token_sources
		SET status=$2,
		    last_rotated_at=now(),
		    next_check_at=now()+interval '24 hours',
		    metadata = COALESCE(metadata,'{}'::jsonb) - 'retry_count',
		    updated_at=now()
		WHERE id=$1::uuid`, item.ID, status)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO token_rotation_events (token_source_id, event_type, result, reason_code, detail_json)
		VALUES ($1::uuid,'rotate','success',$2,$3::jsonb)`, item.ID, "ROTATION_SUCCESS", fmt.Sprintf(`{"provider":"%s"}`, item.Provider))
	return err
}

func (s *TokenRotationScheduler) issueToken(ctx context.Context, item dueTokenSource, currentToken string) (string, error) {
	typeName := strings.TrimSpace(strings.ToLower(item.Type))
	if typeName == "reissue" && s.reissuer != nil {
		next, err := s.reissuer.Reissue(ctx, rotation.ReissueInput{
			Provider:     item.Provider,
			CurrentToken: currentToken,
			Metadata:     item.Metadata,
		})
		if err == nil && strings.TrimSpace(next) != "" {
			return next, nil
		}
		if err != nil && err != rotation.ErrReissueUnsupported {
			return "", err
		}
	}
	return randomToken()
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "nls_" + hex.EncodeToString(b), nil
}

func (s *TokenRotationScheduler) markApprovalRequired(ctx context.Context, item dueTokenSource) {
	if strings.EqualFold(strings.TrimSpace(item.Status), "failed_manual") {
		return
	}
	_, _ = s.pool.Exec(ctx, `UPDATE token_sources SET status='failed_manual', updated_at=now(), next_check_at=now()+interval '24 hours' WHERE id=$1::uuid`, item.ID)
	_, _ = s.pool.Exec(ctx, `INSERT INTO token_rotation_events (token_source_id, event_type, result, reason_code, detail_json) VALUES ($1::uuid,'rotate','failed',$2,$3::jsonb)`, item.ID, "TOKEN_ROTATE_APPROVAL_REQUIRED", `{}`)
}

func (s *TokenRotationScheduler) markExpired(ctx context.Context, item dueTokenSource, reason string) {
	_, _ = s.pool.Exec(ctx, `UPDATE token_sources SET status='expired', updated_at=now(), next_check_at=now()+interval '24 hours' WHERE id=$1::uuid`, item.ID)
	_, _ = s.pool.Exec(ctx, `INSERT INTO token_rotation_events (token_source_id, event_type, result, reason_code, detail_json) VALUES ($1::uuid,'rotate','failed',$2,$3::jsonb)`, item.ID, reason, `{}`)
}

func nextRetryDelay(metadata map[string]any) (string, int) {
	count := 0
	if metadata != nil {
		if raw, ok := metadata["retry_count"]; ok {
			switch v := raw.(type) {
			case float64:
				count = int(v)
			case int:
				count = v
			}
		}
	}
	count++
	switch {
	case count <= 1:
		return "1 minute", count
	case count == 2:
		return "5 minutes", count
	case count == 3:
		return "15 minutes", count
	default:
		return "1 hour", count
	}
}

func (s *TokenRotationScheduler) updateRetryMetadata(ctx context.Context, id string, metadata map[string]any, retryCount int) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["retry_count"] = retryCount
	raw, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	_, _ = s.pool.Exec(ctx, `UPDATE token_sources SET metadata=$2::jsonb WHERE id=$1::uuid`, id, string(raw))
}

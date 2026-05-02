package usecase

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

type TokenSourceUseCase struct {
	repo port.TokenSourceRepository
}

func NewTokenSourceUseCase(repo port.TokenSourceRepository) *TokenSourceUseCase {
	return &TokenSourceUseCase{repo: repo}
}

func (uc *TokenSourceUseCase) ListSources(ctx context.Context, orgID string) ([]*domain.TokenSource, error) {
	items, err := uc.repo.ListSources(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing token sources: %w", err)
	}
	return items, nil
}

func (uc *TokenSourceUseCase) ListEvents(ctx context.Context, tokenSourceID string) ([]*domain.TokenRotationEvent, error) {
	items, err := uc.repo.ListEvents(ctx, tokenSourceID)
	if err != nil {
		return nil, fmt.Errorf("listing token source events: %w", err)
	}
	return items, nil
}

func (uc *TokenSourceUseCase) ApplyAction(ctx context.Context, tokenSourceID, action, reason string, details map[string]any) error {
	if tokenSourceID == "" {
		return &shareddomain.AppError{Code: "TOKEN_SOURCE_ID_REQUIRED", HTTPStatus: http.StatusBadRequest, Message: "Token source id is required", Retryable: false}
	}
	status := mapActionToStatus(action)
	if err := uc.repo.UpdateSourceStatus(ctx, tokenSourceID, status); err != nil {
		return fmt.Errorf("updating token source status: %w", err)
	}
	if err := uc.repo.InsertEvent(ctx, &domain.TokenRotationEvent{
		TokenSourceID: tokenSourceID,
		EventType:     action,
		Result:        "success",
		ReasonCode:    reason,
		DetailJSON:    details,
	}); err != nil {
		return fmt.Errorf("inserting token rotation event: %w", err)
	}
	return nil
}

func (uc *TokenSourceUseCase) RevealMeta(ctx context.Context, tokenSourceID string) (map[string]any, error) {
	item, err := uc.repo.GetSource(ctx, tokenSourceID)
	if err != nil {
		return nil, fmt.Errorf("getting token source: %w", err)
	}
	if item == nil {
		return nil, &shareddomain.AppError{Code: "TOKEN_SOURCE_NOT_FOUND", HTTPStatus: http.StatusNotFound, Message: "Token source not found", Retryable: false}
	}
	return map[string]any{
		"token_source_id": tokenSourceID,
		"provider":        item.Provider,
		"path":            item.Path,
		"status":          item.Status,
		"expires_at":      item.ExpiresAt,
		"token_value":     "stored-in-openbao",
	}, nil
}

func mapActionToStatus(action string) string {
	switch action {
	case "rotate":
		return "rotated"
	case "approve", "resume":
		return "renewing"
	case "pause":
		return "failed_manual"
	default:
		return "healthy"
	}
}

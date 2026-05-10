package usecase

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

type TokenSourceUseCase struct {
	repo         port.TokenSourceRepository
	secretRouter *secrets.Router
}

type TokenSourceUseCaseOption func(*TokenSourceUseCase)

func WithSecretRouter(router *secrets.Router) TokenSourceUseCaseOption {
	return func(uc *TokenSourceUseCase) {
		uc.secretRouter = router
	}
}

func NewTokenSourceUseCase(repo port.TokenSourceRepository, opts ...TokenSourceUseCaseOption) *TokenSourceUseCase {
	uc := &TokenSourceUseCase{repo: repo}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
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
	provider := "openbao"
	if item.Metadata != nil {
		if raw, ok := item.Metadata["secret_manager"]; ok {
			if typed, ok := raw.(string); ok && strings.TrimSpace(typed) != "" {
				provider = strings.TrimSpace(typed)
			}
		}
	}
	tokenValue := "stored-in-openbao"
	if uc.secretRouter != nil {
		if v, readErr := uc.secretRouter.GetToken(ctx, provider, item.Path); readErr == nil && strings.TrimSpace(v) != "" {
			tokenValue = v
		}
	}

	return map[string]any{
		"token_source_id": tokenSourceID,
		"provider":        item.Provider,
		"path":            item.Path,
		"status":          item.Status,
		"expires_at":      item.ExpiresAt,
		"token_value":     tokenValue,
		"secret_manager":  provider,
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

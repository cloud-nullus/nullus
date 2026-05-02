package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

type mockTokenSourceRepo struct {
	listSourcesFn       func(ctx context.Context, orgID string) ([]*domain.TokenSource, error)
	listEventsFn        func(ctx context.Context, tokenSourceID string) ([]*domain.TokenRotationEvent, error)
	getSourceFn         func(ctx context.Context, tokenSourceID string) (*domain.TokenSource, error)
	updateSourceStatusFn func(ctx context.Context, tokenSourceID, status string) error
	insertEventFn       func(ctx context.Context, event *domain.TokenRotationEvent) error
}

func (m *mockTokenSourceRepo) ListSources(ctx context.Context, orgID string) ([]*domain.TokenSource, error) {
	return m.listSourcesFn(ctx, orgID)
}
func (m *mockTokenSourceRepo) ListEvents(ctx context.Context, tokenSourceID string) ([]*domain.TokenRotationEvent, error) {
	return m.listEventsFn(ctx, tokenSourceID)
}
func (m *mockTokenSourceRepo) GetSource(ctx context.Context, tokenSourceID string) (*domain.TokenSource, error) {
	return m.getSourceFn(ctx, tokenSourceID)
}
func (m *mockTokenSourceRepo) UpdateSourceStatus(ctx context.Context, tokenSourceID, status string) error {
	return m.updateSourceStatusFn(ctx, tokenSourceID, status)
}
func (m *mockTokenSourceRepo) InsertEvent(ctx context.Context, event *domain.TokenRotationEvent) error {
	return m.insertEventFn(ctx, event)
}

func TestTokenSourceUseCase_ApplyAction_Success(t *testing.T) {
	t.Parallel()
	repo := &mockTokenSourceRepo{}
	repo.updateSourceStatusFn = func(_ context.Context, tokenSourceID, status string) error {
		assert.Equal(t, "ts-1", tokenSourceID)
		assert.Equal(t, "rotated", status)
		return nil
	}
	repo.insertEventFn = func(_ context.Context, event *domain.TokenRotationEvent) error {
		assert.Equal(t, "rotate", event.EventType)
		assert.Equal(t, "manual", event.ReasonCode)
		return nil
	}
	uc := NewTokenSourceUseCase(repo)
	require.NoError(t, uc.ApplyAction(context.Background(), "ts-1", "rotate", "manual", map[string]any{"x": 1}))
}

func TestTokenSourceUseCase_RevealMeta_Success(t *testing.T) {
	t.Parallel()
	repo := &mockTokenSourceRepo{}
	expires := time.Now().UTC().Add(time.Hour)
	repo.getSourceFn = func(_ context.Context, tokenSourceID string) (*domain.TokenSource, error) {
		assert.Equal(t, "ts-1", tokenSourceID)
		return &domain.TokenSource{ID: tokenSourceID, Provider: "github", Path: "kv/nullus/dev", Status: "healthy", ExpiresAt: &expires}, nil
	}
	uc := NewTokenSourceUseCase(repo)
	out, err := uc.RevealMeta(context.Background(), "ts-1")
	require.NoError(t, err)
	assert.Equal(t, "github", out["provider"])
}

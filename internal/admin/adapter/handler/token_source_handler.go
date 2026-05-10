package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
)

type TokenSourceHandler struct {
	tokenUC *usecase.TokenSourceUseCase

	listSourcesFn func(ctx context.Context, orgID string) ([]tokenSource, error)
	listEventsFn  func(ctx context.Context, tokenSourceID string) ([]tokenRotationEvent, error)
	actionFn      func(ctx context.Context, tokenSourceID, action, reason string, details map[string]any) error
	revealFn      func(ctx context.Context, tokenSourceID string) (map[string]any, error)

	stepUpTTL   time.Duration
	stepUpStore sync.Map
}

type tokenSource struct {
	ID               string         `json:"id"`
	OrgID            string         `json:"org_id"`
	Module           string         `json:"module"`
	Provider         string         `json:"provider"`
	Path             string         `json:"path"`
	TokenType        string         `json:"token_type"`
	Status           string         `json:"status"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	LastRotatedAt    *time.Time     `json:"last_rotated_at,omitempty"`
	NextCheckAt      *time.Time     `json:"next_check_at,omitempty"`
	RequiresApproval bool           `json:"requires_approval"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type tokenRotationEvent struct {
	ID            string         `json:"id"`
	TokenSourceID string         `json:"token_source_id"`
	EventType     string         `json:"event_type"`
	Result        string         `json:"result"`
	ReasonCode    string         `json:"reason_code,omitempty"`
	DetailJSON    map[string]any `json:"detail_json,omitempty"`
	TraceID       string         `json:"trace_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

type tokenSourceActionRequest struct {
	Reason  string         `json:"reason"`
	Details map[string]any `json:"details"`
}

type tokenSourceReAuthRequest struct {
	Reason string `json:"reason"`
}

type tokenSourceRevealRequest struct {
	StepUpToken string `json:"step_up_token"`
}

type stepUpGrant struct {
	UserID    string
	ExpiresAt time.Time
}

func NewTokenSourceHandler(tokenUC *usecase.TokenSourceUseCase) *TokenSourceHandler {
	h := &TokenSourceHandler{tokenUC: tokenUC, stepUpTTL: 5 * time.Minute}
	h.listSourcesFn = h.listSources
	h.listEventsFn = h.listEvents
	h.actionFn = h.applyAction
	h.revealFn = h.reveal
	return h
}

func (h *TokenSourceHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/token-sources", h.ListSources)
	g.GET("/token-sources/:id/events", h.ListEvents)
	g.POST("/token-sources/:id/rotate", h.Rotate)
	g.POST("/token-sources/:id/approve", h.Approve)
	g.POST("/token-sources/:id/pause", h.Pause)
	g.POST("/token-sources/:id/resume", h.Resume)
	g.POST("/token-sources/:id/re-auth", h.ReAuth)
	g.POST("/token-sources/:id/reveal", h.Reveal)
}

func (h *TokenSourceHandler) ListSources(c echo.Context) error {
	items, err := h.listSourcesFn(c.Request().Context(), resolveOrgID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *TokenSourceHandler) ListEvents(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	items, err := h.listEventsFn(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *TokenSourceHandler) Rotate(c echo.Context) error  { return h.handleAction(c, "rotate") }
func (h *TokenSourceHandler) Approve(c echo.Context) error { return h.handleAction(c, "approve") }
func (h *TokenSourceHandler) Pause(c echo.Context) error   { return h.handleAction(c, "pause") }
func (h *TokenSourceHandler) Resume(c echo.Context) error  { return h.handleAction(c, "resume") }

func (h *TokenSourceHandler) ReAuth(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "X-User-ID is required")
	}
	var req tokenSourceReAuthRequest
	_ = c.Bind(&req)
	token, err := newStepUpToken()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create step-up token")
	}
	h.stepUpStore.Store(token, stepUpGrant{UserID: userID, ExpiresAt: time.Now().UTC().Add(h.stepUpTTL)})
	_ = h.actionFn(c.Request().Context(), id, "re-auth", req.Reason, map[string]any{"user_id": userID})
	return c.JSON(http.StatusOK, map[string]any{"step_up_token": token, "expires_in_seconds": int(h.stepUpTTL.Seconds())})
}

func (h *TokenSourceHandler) Reveal(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	var req tokenSourceRevealRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.StepUpToken == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "step_up_token is required")
	}
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "X-User-ID is required")
	}
	grantRaw, ok := h.stepUpStore.Load(req.StepUpToken)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid step_up_token")
	}
	grant, ok := grantRaw.(stepUpGrant)
	if !ok || grant.UserID != userID || time.Now().UTC().After(grant.ExpiresAt) {
		h.stepUpStore.Delete(req.StepUpToken)
		return echo.NewHTTPError(http.StatusUnauthorized, "expired or mismatched step_up_token")
	}
	h.stepUpStore.Delete(req.StepUpToken)

	result, err := h.revealFn(c.Request().Context(), id)
	if err != nil {
		return err
	}
	_ = h.actionFn(c.Request().Context(), id, "reveal", "", map[string]any{"user_id": userID})
	return c.JSON(http.StatusOK, result)
}

func (h *TokenSourceHandler) handleAction(c echo.Context, action string) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	var req tokenSourceActionRequest
	_ = c.Bind(&req)
	if err := h.actionFn(c.Request().Context(), id, action, req.Reason, req.Details); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "ok", "action": action})
}

func (h *TokenSourceHandler) listSources(ctx context.Context, orgID string) ([]tokenSource, error) {
	items, err := h.tokenUC.ListSources(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]tokenSource, 0, len(items))
	for _, item := range items {
		out = append(out, mapTokenSource(item))
	}
	return out, nil
}

func (h *TokenSourceHandler) listEvents(ctx context.Context, tokenSourceID string) ([]tokenRotationEvent, error) {
	items, err := h.tokenUC.ListEvents(ctx, tokenSourceID)
	if err != nil {
		return nil, err
	}
	out := make([]tokenRotationEvent, 0, len(items))
	for _, item := range items {
		out = append(out, mapTokenRotationEvent(item))
	}
	return out, nil
}

func (h *TokenSourceHandler) applyAction(ctx context.Context, tokenSourceID, action, reason string, details map[string]any) error {
	return h.tokenUC.ApplyAction(ctx, tokenSourceID, action, reason, details)
}

func (h *TokenSourceHandler) reveal(ctx context.Context, tokenSourceID string) (map[string]any, error) {
	return h.tokenUC.RevealMeta(ctx, tokenSourceID)
}

func mapTokenSource(item *domain.TokenSource) tokenSource {
	return tokenSource{
		ID:               item.ID,
		OrgID:            item.OrgID,
		Module:           item.Module,
		Provider:         item.Provider,
		Path:             item.Path,
		TokenType:        item.TokenType,
		Status:           item.Status,
		ExpiresAt:        item.ExpiresAt,
		LastRotatedAt:    item.LastRotatedAt,
		NextCheckAt:      item.NextCheckAt,
		RequiresApproval: item.RequiresApproval,
		Metadata:         item.Metadata,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func mapTokenRotationEvent(item *domain.TokenRotationEvent) tokenRotationEvent {
	return tokenRotationEvent{
		ID:            item.ID,
		TokenSourceID: item.TokenSourceID,
		EventType:     item.EventType,
		Result:        item.Result,
		ReasonCode:    item.ReasonCode,
		DetailJSON:    item.DetailJSON,
		TraceID:       item.TraceID,
		CreatedAt:     item.CreatedAt,
	}
}

func newStepUpToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

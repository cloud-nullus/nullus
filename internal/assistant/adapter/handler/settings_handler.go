package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	sharedmw "github.com/cloud-nullus/draft/internal/shared/middleware"
)

type settingsService interface {
	Get(ctx context.Context, orgID string) (usecase.SettingsView, error)
	Update(ctx context.Context, orgID, actor string, in usecase.UpdateSettings) (usecase.SettingsView, error)
	Delete(ctx context.Context, orgID string) error
	Test(ctx context.Context, orgID string, in usecase.UpdateSettings) (usecase.TestResult, error)
}

// AuditSink 는 설정 변경을 남기는 감사 로그다. 없어도 된다.
type AuditSink interface {
	Log(ctx context.Context, entry audit.AuditEntry) error
}

// SettingsHandler 는 관리자가 조직의 모델 공급자 · 모델 · API 키를 등록하는 API 다. admin 그룹에 붙는다.
type SettingsHandler struct {
	svc   settingsService
	audit AuditSink
}

func NewSettingsHandler(svc settingsService, auditSink AuditSink) *SettingsHandler {
	return &SettingsHandler{svc: svc, audit: auditSink}
}

func (h *SettingsHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/assistant", h.Get)
	g.PUT("/assistant", h.Put)
	g.DELETE("/assistant", h.Delete)
	g.POST("/assistant/test", h.Test)
}

// settingsRequest 의 APIKey 는 포인터다 — 빠지면(nil) 저장된 키를 그대로 쓰고, 빈 문자열이면 지운다.
type settingsRequest struct {
	Enabled  bool    `json:"enabled"`
	Provider string  `json:"provider"`
	BaseURL  string  `json:"base_url"`
	Model    string  `json:"model"`
	APIKey   *string `json:"api_key"`
}

func (r settingsRequest) toInput() usecase.UpdateSettings {
	return usecase.UpdateSettings{Enabled: r.Enabled, Provider: r.Provider, BaseURL: r.BaseURL, Model: r.Model, APIKey: r.APIKey}
}

type settingsResponse struct {
	Enabled          bool              `json:"enabled"`
	Provider         string            `json:"provider"`
	BaseURL          string            `json:"base_url"`
	Model            string            `json:"model"`
	HasKey           bool              `json:"has_key"`
	KeyHint          string            `json:"key_hint"`
	KeyUnreadable    bool              `json:"key_unreadable"`
	EffectiveBaseURL string            `json:"effective_base_url"`
	EffectiveModel   string            `json:"effective_model"`
	UpdatedAt        *time.Time        `json:"updated_at,omitempty"`
	UpdatedBy        string            `json:"updated_by,omitempty"`
	Providers        []domain.Provider `json:"providers"`
}

func toResponse(v usecase.SettingsView) settingsResponse {
	return settingsResponse{
		Enabled: v.Enabled, Provider: v.Provider, BaseURL: v.BaseURL, Model: v.Model,
		HasKey: v.HasKey, KeyHint: v.KeyHint, KeyUnreadable: v.KeyUnreadable, EffectiveBaseURL: v.EffectiveBaseURL, EffectiveModel: v.EffectiveModel,
		UpdatedAt: v.UpdatedAt, UpdatedBy: v.UpdatedBy, Providers: v.Providers,
	}
}

func (h *SettingsHandler) Get(c echo.Context) error {
	v, err := h.svc.Get(c.Request().Context(), resolveOrgID(c))
	if err != nil {
		return settingsError(c, err)
	}
	return c.JSON(http.StatusOK, toResponse(v))
}

func (h *SettingsHandler) Put(c echo.Context) error {
	var req settingsRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ASSISTANT_SETTINGS_INVALID", "요청 본문을 해석하지 못했습니다")
	}
	actor := sharedmw.ActorFromContext(c)
	orgID := resolveOrgID(c)
	v, err := h.svc.Update(c.Request().Context(), orgID, actor.Label(), req.toInput())
	if err != nil {
		return settingsError(c, err)
	}
	// 키 원문은 남기지 않는다 — 바꿨는지만.
	h.log(c, actor, "update_assistant_settings", orgID, map[string]any{
		"enabled": req.Enabled, "provider": req.Provider, "model": req.Model, "base_url": req.BaseURL,
		"key_changed": req.APIKey != nil,
	})
	return c.JSON(http.StatusOK, toResponse(v))
}

func (h *SettingsHandler) Delete(c echo.Context) error {
	actor := sharedmw.ActorFromContext(c)
	orgID := resolveOrgID(c)
	if err := h.svc.Delete(c.Request().Context(), orgID); err != nil {
		return settingsError(c, err)
	}
	h.log(c, actor, "delete_assistant_settings", orgID, map[string]any{})
	return c.NoContent(http.StatusNoContent)
}

// Test 는 연결을 시험한다. 모델이 실패해도 200 이고 결과는 본문의 ok · error 다(알림 채널 테스트와 같다).
func (h *SettingsHandler) Test(c echo.Context) error {
	var req settingsRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ASSISTANT_SETTINGS_INVALID", "요청 본문을 해석하지 못했습니다")
	}
	orgID := resolveOrgID(c)
	res, err := h.svc.Test(c.Request().Context(), orgID, req.toInput())
	if err != nil {
		return settingsError(c, err)
	}
	// 바꾸는 것은 없어도 키를 바깥으로 보내는 일이라 남긴다 — 키는 빼고.
	h.log(c, sharedmw.ActorFromContext(c), "test_assistant_settings", orgID, map[string]any{
		"provider": req.Provider, "model": req.Model, "base_url": req.BaseURL, "key_sent": req.APIKey != nil, "ok": res.OK,
	})
	return c.JSON(http.StatusOK, map[string]any{"ok": res.OK, "model": res.Model, "error": res.Error, "latency_ms": res.LatencyMs})
}

func (h *SettingsHandler) log(c echo.Context, actor sharedmw.Actor, action, orgID string, details map[string]any) {
	if h.audit == nil {
		return
	}
	details["actor"] = actor.Label()
	_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
		UserID: actor.ID, Action: action, ResourceType: "assistant_settings", ResourceID: orgID,
		Details: details, IPAddress: c.RealIP(),
	})
}

func settingsError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidSettings):
		return errorResponse(c, http.StatusBadRequest, "ASSISTANT_SETTINGS_INVALID", err.Error())
	case errors.Is(err, domain.ErrKeyStorageUnavailable):
		return errorResponse(c, http.StatusServiceUnavailable, "ASSISTANT_KEY_STORAGE_UNAVAILABLE",
			"서버에 ENCRYPTION_KEY 가 없어 API 키를 저장할 수 없습니다")
	default:
		return errorResponse(c, http.StatusInternalServerError, "ASSISTANT_SETTINGS_FAILED", "어시스턴트 설정을 처리하지 못했습니다")
	}
}

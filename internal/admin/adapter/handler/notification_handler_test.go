package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNotificationEcho(h *NotificationHandler) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	return e
}

func TestNotificationHandler_ListConfigs_200(t *testing.T) {
	t.Parallel()

	h := &NotificationHandler{}
	h.listConfigsFn = func(_ context.Context) ([]notificationConfig, error) {
		return []notificationConfig{{
			ID:        "cfg-1",
			OrgID:     "org-1",
			Channel:   "slack",
			Config:    map[string]any{"webhook_url": "https://hooks.slack.com/services/T000/B000/XXX"},
			Events:    []string{"stack.deployed"},
			IsActive:  true,
			CreatedAt: time.Now().UTC(),
		}}, nil
	}

	e := newNotificationEcho(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/notifications/configs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.EqualValues(t, 1, resp["total"])
}

func TestNotificationHandler_CreateConfig_201(t *testing.T) {
	t.Parallel()

	h := &NotificationHandler{}
	h.createConfigFn = func(_ context.Context, input createNotificationConfigInput) (*notificationConfig, error) {
		assert.Equal(t, "email", input.Channel)
		assert.Equal(t, []string{"pipeline.failed"}, input.Events)
		assert.Equal(t, "alerts@nullus.io", input.Config["to"])
		return &notificationConfig{
			ID:        "cfg-2",
			OrgID:     "org-1",
			Channel:   input.Channel,
			Config:    input.Config,
			Events:    input.Events,
			IsActive:  true,
			CreatedAt: time.Now().UTC(),
		}, nil
	}

	e := newNotificationEcho(h)
	body := `{"channel":"email","config":{"to":"alerts@nullus.io"},"events":["pipeline.failed"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications/configs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "email", resp["channel"])
}

func TestNotificationHandler_DeleteConfig_204(t *testing.T) {
	t.Parallel()

	h := &NotificationHandler{}
	h.deleteConfigFn = func(_ context.Context, id string) error {
		assert.Equal(t, "cfg-1", id)
		return nil
	}

	e := newNotificationEcho(h)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/notifications/configs/cfg-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestNotificationHandler_ListHistory_200(t *testing.T) {
	t.Parallel()

	h := &NotificationHandler{}
	h.listHistoryFn = func(_ context.Context) ([]notificationHistoryEntry, error) {
		return []notificationHistoryEntry{{
			ID:        "h-1",
			OrgID:     "org-1",
			Channel:   "slack",
			Event:     "stack.deployed",
			Status:    "sent",
			Payload:   map[string]any{"stack_id": "stack-1"},
			Error:     "",
			CreatedAt: time.Now().UTC(),
		}}, nil
	}

	e := newNotificationEcho(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/notifications/history", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.EqualValues(t, 1, resp["total"])
}

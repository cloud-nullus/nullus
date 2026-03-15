package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type NotificationHandler struct {
	pool *pgxpool.Pool

	listConfigsFn  func(ctx context.Context) ([]notificationConfig, error)
	createConfigFn func(ctx context.Context, input createNotificationConfigInput) (*notificationConfig, error)
	deleteConfigFn func(ctx context.Context, id string) error
	listHistoryFn  func(ctx context.Context) ([]notificationHistoryEntry, error)
}

type createNotificationConfigRequest struct {
	Channel string         `json:"channel"`
	Config  map[string]any `json:"config"`
	Events  []string       `json:"events"`
}

type createNotificationConfigInput struct {
	OrgID   string
	Channel string
	Config  map[string]any
	Events  []string
}

type notificationConfig struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"org_id"`
	Channel   string         `json:"channel"`
	Config    map[string]any `json:"config"`
	Events    []string       `json:"events"`
	IsActive  bool           `json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
}

type notificationHistoryEntry struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"org_id"`
	Channel   string         `json:"channel"`
	Event     string         `json:"event"`
	Status    string         `json:"status"`
	Payload   map[string]any `json:"payload,omitempty"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func NewNotificationHandler(pool *pgxpool.Pool) *NotificationHandler {
	h := &NotificationHandler{pool: pool}
	h.listConfigsFn = h.listConfigs
	h.createConfigFn = h.createConfig
	h.deleteConfigFn = h.deleteConfig
	h.listHistoryFn = h.listHistory
	return h
}

func (h *NotificationHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/notifications/configs", h.ListConfigs)
	g.POST("/notifications/configs", h.CreateConfig)
	g.DELETE("/notifications/configs/:id", h.DeleteConfig)
	g.GET("/notifications/history", h.ListHistory)
}

func (h *NotificationHandler) ListConfigs(c echo.Context) error {
	if h.listConfigsFn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification service is not configured")
	}

	items, err := h.listConfigsFn(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *NotificationHandler) CreateConfig(c echo.Context) error {
	var req createNotificationConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Channel == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "channel is required")
	}

	if h.createConfigFn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification service is not configured")
	}

	created, err := h.createConfigFn(c.Request().Context(), createNotificationConfigInput{
		OrgID:   resolveOrgID(c),
		Channel: req.Channel,
		Config:  req.Config,
		Events:  req.Events,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, created)
}

func (h *NotificationHandler) DeleteConfig(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	if h.deleteConfigFn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification service is not configured")
	}

	if err := h.deleteConfigFn(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *NotificationHandler) ListHistory(c echo.Context) error {
	if h.listHistoryFn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification service is not configured")
	}

	items, err := h.listHistoryFn(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *NotificationHandler) listConfigs(ctx context.Context) ([]notificationConfig, error) {
	if h.pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	const q = `
		SELECT id, org_id, channel, config, events, is_active, created_at
		FROM notification_configs
		ORDER BY created_at DESC`

	rows, err := h.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]notificationConfig, 0)
	for rows.Next() {
		var (
			item      notificationConfig
			configRaw []byte
		)

		if err := rows.Scan(&item.ID, &item.OrgID, &item.Channel, &configRaw, &item.Events, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(configRaw) > 0 {
			if err := json.Unmarshal(configRaw, &item.Config); err != nil {
				return nil, err
			}
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func (h *NotificationHandler) createConfig(ctx context.Context, input createNotificationConfigInput) (*notificationConfig, error) {
	if h.pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	configJSON, err := json.Marshal(input.Config)
	if err != nil {
		return nil, err
	}

	const q = `
		INSERT INTO notification_configs (org_id, channel, config, events)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, channel, config, events, is_active, created_at`

	item := &notificationConfig{}
	var configRaw []byte
	if err := h.pool.QueryRow(ctx, q, input.OrgID, input.Channel, configJSON, input.Events).Scan(
		&item.ID,
		&item.OrgID,
		&item.Channel,
		&configRaw,
		&item.Events,
		&item.IsActive,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}

	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &item.Config); err != nil {
			return nil, err
		}
	}

	return item, nil
}

func (h *NotificationHandler) deleteConfig(ctx context.Context, id string) error {
	if h.pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	const q = `DELETE FROM notification_configs WHERE id = $1`
	_, err := h.pool.Exec(ctx, q, id)
	return err
}

func (h *NotificationHandler) listHistory(ctx context.Context) ([]notificationHistoryEntry, error) {
	if h.pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	const q = `
		SELECT id, org_id, channel, event, status, payload, error, created_at
		FROM notification_history
		ORDER BY created_at DESC`

	rows, err := h.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]notificationHistoryEntry, 0)
	for rows.Next() {
		var (
			item       notificationHistoryEntry
			payloadRaw []byte
		)

		if err := rows.Scan(&item.ID, &item.OrgID, &item.Channel, &item.Event, &item.Status, &payloadRaw, &item.Error, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(payloadRaw) > 0 {
			if err := json.Unmarshal(payloadRaw, &item.Payload); err != nil {
				return nil, err
			}
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func resolveOrgID(c echo.Context) string {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = c.QueryParam("orgId")
	}
	if orgID == "" {
		orgID = "default-org"
	}
	return orgID
}

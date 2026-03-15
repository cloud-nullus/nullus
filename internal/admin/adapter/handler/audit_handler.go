package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/labstack/echo/v4"
)

type AuditHandler struct {
	logger *audit.AuditLogger
	listFn func(ctx context.Context, limit, offset int) ([]audit.AuditEntry, int, error)
}

func NewAuditHandler(logger *audit.AuditLogger) *AuditHandler {
	return &AuditHandler{
		logger: logger,
		listFn: logger.List,
	}
}

func (h *AuditHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/audit-logs", h.ListAuditLogs)
}

func (h *AuditHandler) ListAuditLogs(c echo.Context) error {
	if h.listFn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "audit logger is not configured")
	}

	limit := 50
	offset := 0

	if raw := c.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if raw := c.QueryParam("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}

	items, total, err := h.listFn(c.Request().Context(), limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": total})
}

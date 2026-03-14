package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/labstack/echo/v4"
)

// AlertHandler handles HTTP requests for alert rule and history operations.
type AlertHandler struct {
	createAlertRule *usecase.CreateAlertRule
	listAlerts      *usecase.ListAlerts
	alertRuleRepo   port.AlertRuleRepository
}

// NewAlertHandler constructs an AlertHandler.
func NewAlertHandler(
	createAlertRule *usecase.CreateAlertRule,
	listAlerts *usecase.ListAlerts,
	alertRuleRepo port.AlertRuleRepository,
) *AlertHandler {
	return &AlertHandler{
		createAlertRule: createAlertRule,
		listAlerts:      listAlerts,
		alertRuleRepo:   alertRuleRepo,
	}
}

// RegisterRoutes registers alert routes on the given Echo group.
func (h *AlertHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/alerts/rules", h.ListRules)
	g.POST("/alerts/rules", h.CreateRule)
	g.GET("/alerts/history", h.ListHistory)
}

// createAlertRuleRequest is the request body for POST /alerts/rules.
type createAlertRuleRequest struct {
	Name      string  `json:"name"`
	Condition string  `json:"condition"`
	Threshold float64 `json:"threshold"`
	Channel   string  `json:"channel"`
	Enabled   bool    `json:"enabled"`
}

// ListRules handles GET /api/v1/alerts/rules.
func (h *AlertHandler) ListRules(c echo.Context) error {
	rules, err := h.alertRuleRepo.List(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "ALERT_RULE_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rules})
}

// CreateRule handles POST /api/v1/alerts/rules.
func (h *AlertHandler) CreateRule(c echo.Context) error {
	var req createAlertRuleRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ALERT_RULE_INVALID", err.Error())
	}

	out, err := h.createAlertRule.Execute(c.Request().Context(), usecase.CreateAlertRuleInput{
		Name:      req.Name,
		Condition: req.Condition,
		Threshold: req.Threshold,
		Channel:   domain.AlertChannel(req.Channel),
		Enabled:   req.Enabled,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "ALERT_RULE_CREATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]any{"data": out.Rule})
}

// ListHistory handles GET /api/v1/alerts/history.
func (h *AlertHandler) ListHistory(c echo.Context) error {
	out, err := h.listAlerts.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "ALERT_HISTORY_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": out.Alerts})
}

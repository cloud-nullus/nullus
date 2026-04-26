package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
)

// AlertHandler handles HTTP requests for alert rule and history operations.
type AlertHandler struct {
	createAlertRule *usecase.CreateAlertRule
	getAlertRule    *usecase.GetAlertRule
	listAlertRules  *usecase.ListAlertRules
	updateAlertRule *usecase.UpdateAlertRule
	deleteAlertRule *usecase.DeleteAlertRule
	listAlerts      *usecase.ListAlerts
}

// NewAlertHandler constructs an AlertHandler.
func NewAlertHandler(
	createAlertRule *usecase.CreateAlertRule,
	getAlertRule *usecase.GetAlertRule,
	listAlertRules *usecase.ListAlertRules,
	updateAlertRule *usecase.UpdateAlertRule,
	deleteAlertRule *usecase.DeleteAlertRule,
	listAlerts *usecase.ListAlerts,
) *AlertHandler {
	return &AlertHandler{
		createAlertRule: createAlertRule,
		getAlertRule:    getAlertRule,
		listAlertRules:  listAlertRules,
		updateAlertRule: updateAlertRule,
		deleteAlertRule: deleteAlertRule,
		listAlerts:      listAlerts,
	}
}

// RegisterRoutes registers alert routes on the given Echo group.
func (h *AlertHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/alert-rules", h.ListRules)
	g.GET("/alert-rules/:id", h.GetRule)
	g.POST("/alert-rules", h.CreateRule)
	g.PATCH("/alert-rules/:id", h.UpdateRule)
	g.DELETE("/alert-rules/:id", h.DeleteRule)
	g.GET("/alert-history", h.ListHistory)
}

// createAlertRuleRequest is the request body for POST /alerts/rules.
type createAlertRuleRequest struct {
	Name              string  `json:"name"`
	MetricName        string  `json:"metric_name"`
	Condition         string  `json:"condition"`
	Threshold         float64 `json:"threshold"`
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
	Channel           string  `json:"channel"`
	Enabled           bool    `json:"enabled"`
}

// ListRules handles GET /api/v1/alerts/rules.
// Query params:
//
//	scope: "stack" | "cicd" — filters alert rules to the specified domain.
//	        TODO: pass scope to repository filter when scoped rules are implemented.
func (h *AlertHandler) ListRules(c echo.Context) error {
	_ = c.QueryParam("scope")

	out, err := h.listAlertRules.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "ALERT_RULE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": out.Rules, "total": len(out.Rules)})
}

func (h *AlertHandler) GetRule(c echo.Context) error {
	id := c.Param("id")

	out, err := h.getAlertRule.Execute(c.Request().Context(), usecase.GetAlertRuleInput{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrAlertRuleNotFound) {
			return errorResponse(c, http.StatusNotFound, "ALERT_RULE_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "ALERT_RULE_GET_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, out.Rule)
}

// CreateRule handles POST /api/v1/alerts/rules.
func (h *AlertHandler) CreateRule(c echo.Context) error {
	var req createAlertRuleRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ALERT_RULE_INVALID", err.Error())
	}

	metricName := strings.TrimSpace(req.MetricName)
	if metricName == "" {
		metricName = strings.TrimSpace(req.Condition)
	}
	warningThreshold := req.WarningThreshold
	criticalThreshold := req.CriticalThreshold
	if warningThreshold == 0 && req.Threshold > 0 {
		warningThreshold = req.Threshold
	}
	if criticalThreshold == 0 && req.Threshold > 0 {
		criticalThreshold = req.Threshold
	}

	out, err := h.createAlertRule.Execute(c.Request().Context(), usecase.CreateAlertRuleInput{
		Name:              req.Name,
		MetricName:        metricName,
		WarningThreshold:  warningThreshold,
		CriticalThreshold: criticalThreshold,
		Channel:           domain.AlertChannel(req.Channel),
		Enabled:           req.Enabled,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "ALERT_RULE_CREATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusCreated, out.Rule)
}

type updateAlertRuleRequest struct {
	Name              *string  `json:"name"`
	MetricName        *string  `json:"metric_name"`
	Condition         *string  `json:"condition"`
	Threshold         *float64 `json:"threshold"`
	WarningThreshold  *float64 `json:"warning_threshold"`
	CriticalThreshold *float64 `json:"critical_threshold"`
	Channel           *string  `json:"channel"`
	Enabled           *bool    `json:"enabled"`
}

func (h *AlertHandler) UpdateRule(c echo.Context) error {
	id := c.Param("id")

	var req updateAlertRuleRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ALERT_RULE_INVALID", err.Error())
	}

	var channel *domain.AlertChannel
	if req.Channel != nil {
		v := domain.AlertChannel(*req.Channel)
		channel = &v
	}

	metricName := req.MetricName
	if metricName == nil {
		metricName = req.Condition
	}
	warningThreshold := req.WarningThreshold
	criticalThreshold := req.CriticalThreshold
	if warningThreshold == nil && req.Threshold != nil {
		warningThreshold = req.Threshold
	}
	if criticalThreshold == nil && req.Threshold != nil {
		criticalThreshold = req.Threshold
	}

	out, err := h.updateAlertRule.Execute(c.Request().Context(), usecase.UpdateAlertRuleInput{
		ID:                id,
		Name:              req.Name,
		MetricName:        metricName,
		WarningThreshold:  warningThreshold,
		CriticalThreshold: criticalThreshold,
		Channel:           channel,
		Enabled:           req.Enabled,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAlertRuleNotFound) {
			return errorResponse(c, http.StatusNotFound, "ALERT_RULE_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "ALERT_RULE_UPDATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, out.Rule)
}

func (h *AlertHandler) DeleteRule(c echo.Context) error {
	id := c.Param("id")

	err := h.deleteAlertRule.Execute(c.Request().Context(), usecase.DeleteAlertRuleInput{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrAlertRuleNotFound) {
			return errorResponse(c, http.StatusNotFound, "ALERT_RULE_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "ALERT_RULE_DELETE_FAILED", err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// ListHistory handles GET /api/v1/alerts/history.
func (h *AlertHandler) ListHistory(c echo.Context) error {
	out, err := h.listAlerts.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "ALERT_HISTORY_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out.Alerts, "total": len(out.Alerts)})
}

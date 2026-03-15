package handler

import (
	"net/http"
	"sync"

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
	mu              sync.RWMutex
	patchedRules    map[string]*domain.AlertRule
	deletedRules    map[string]bool
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
		patchedRules:    make(map[string]*domain.AlertRule),
		deletedRules:    make(map[string]bool),
	}
}

// RegisterRoutes registers alert routes on the given Echo group.
func (h *AlertHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/alert-rules", h.ListRules)
	g.POST("/alert-rules", h.CreateRule)
	g.PATCH("/alert-rules/:id", h.UpdateRule)
	g.DELETE("/alert-rules/:id", h.DeleteRule)
	g.GET("/alert-history", h.ListHistory)
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

	h.mu.RLock()
	items := make([]*domain.AlertRule, 0, len(rules))
	for _, rule := range rules {
		if h.deletedRules[rule.ID] {
			continue
		}
		if patched, ok := h.patchedRules[rule.ID]; ok {
			items = append(items, patched)
			continue
		}
		items = append(items, rule)
	}
	h.mu.RUnlock()

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
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

	return c.JSON(http.StatusCreated, out.Rule)
}

type updateAlertRuleRequest struct {
	Name      *string  `json:"name"`
	Condition *string  `json:"condition"`
	Threshold *float64 `json:"threshold"`
	Channel   *string  `json:"channel"`
	Enabled   *bool    `json:"enabled"`
}

func (h *AlertHandler) UpdateRule(c echo.Context) error {
	id := c.Param("id")

	var req updateAlertRuleRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ALERT_RULE_INVALID", err.Error())
	}

	rule, err := h.alertRuleRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "ALERT_RULE_NOT_FOUND", err.Error())
	}

	updated := *rule
	if req.Name != nil {
		updated.Name = *req.Name
	}
	if req.Condition != nil {
		updated.Condition = *req.Condition
	}
	if req.Threshold != nil {
		updated.Threshold = *req.Threshold
	}
	if req.Channel != nil {
		updated.Channel = domain.AlertChannel(*req.Channel)
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}

	h.mu.Lock()
	h.patchedRules[id] = &updated
	h.deletedRules[id] = false
	h.mu.Unlock()

	return c.JSON(http.StatusOK, &updated)
}

func (h *AlertHandler) DeleteRule(c echo.Context) error {
	id := c.Param("id")

	h.mu.Lock()
	h.deletedRules[id] = true
	delete(h.patchedRules, id)
	h.mu.Unlock()

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

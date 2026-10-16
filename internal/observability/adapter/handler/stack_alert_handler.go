package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharedmw "github.com/cloud-nullus/draft/internal/shared/middleware"
)

// StackAlertHandler 는 스택 심각 경보 목록이다(nullus-plan#87) — 알림 이력 화면이 쓴다.
type StackAlertHandler struct {
	list *usecase.ListStackAlerts
}

func NewStackAlertHandler(list *usecase.ListStackAlerts) *StackAlertHandler {
	return &StackAlertHandler{list: list}
}

func (h *StackAlertHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stack-alerts", h.List)
}

type stackAlertResponse struct {
	ID             string            `json:"id"`
	StackID        string            `json:"stack_id"`
	StackName      string            `json:"stack_name"`
	AlertName      string            `json:"alertname"`
	Severity       string            `json:"severity"`
	Summary        string            `json:"summary"`
	Description    string            `json:"description,omitempty"`
	RunbookURL     string            `json:"runbook_url,omitempty"`
	Target         string            `json:"target,omitempty"`
	Labels         map[string]string `json:"labels"`
	StartedAt      time.Time         `json:"started_at"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty"`
	LastNotifiedAt *time.Time        `json:"last_notified_at,omitempty"`
	Active         bool              `json:"active"`
}

// List 는 GET /observability/stack-alerts?state=open|all&limit= 다. 조직은 로그인한 사용자의 것이다.
func (h *StackAlertHandler) List(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	openOnly := strings.EqualFold(c.QueryParam("state"), "open")
	items, err := h.list.Execute(c.Request().Context(), principalOrgID(c), openOnly, limit)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_ALERTS_FAILED", err.Error())
	}
	out := make([]stackAlertResponse, 0, len(items))
	for _, a := range items {
		out = append(out, toStackAlertResponse(a))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func toStackAlertResponse(a *domain.StackAlert) stackAlertResponse {
	labels := a.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	return stackAlertResponse{
		ID: a.ID, StackID: a.StackID, StackName: a.StackName, AlertName: a.AlertName, Severity: a.Severity,
		Summary: a.Summary, Description: a.Description, RunbookURL: a.RunbookURL, Target: a.Target(),
		Labels: labels, StartedAt: a.StartedAt, ResolvedAt: a.ResolvedAt, LastNotifiedAt: a.LastNotifiedAt,
		Active: a.ResolvedAt == nil,
	}
}

// principalOrgID 는 로그인한 사용자의 조직, 없으면 X-Org-ID, 그것도 없으면 기본 조직이다(백업 · 알림 API 와 같다).
func principalOrgID(c echo.Context) string {
	if orgID := sharedmw.OrgIDFromEchoContext(c); orgID != "" {
		return orgID
	}
	if orgID := strings.TrimSpace(c.Request().Header.Get("X-Org-ID")); orgID != "" {
		return orgID
	}
	return shareddomain.DefaultOrgID()
}

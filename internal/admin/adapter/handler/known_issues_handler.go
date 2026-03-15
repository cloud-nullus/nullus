package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type KnownIssuesHandler struct{}

type knownIssueItem struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Workaround  string `json:"workaround"`
	Status      string `json:"status"`
}

func (h *KnownIssuesHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/known-issues", h.ListKnownIssues)
}

func (h *KnownIssuesHandler) ListKnownIssues(c echo.Context) error {
	items := []knownIssueItem{
		{
			ID:          "KI-001",
			Severity:    "medium",
			Title:       "Helm install requires cluster admin",
			Description: "Helm-based stack installation currently requires cluster-admin role to create CRDs and cluster-scoped resources.",
			Workaround:  "Use a temporary cluster-admin service account during installation, then rotate to least-privilege RBAC.",
			Status:      "open",
		},
		{
			ID:          "KI-002",
			Severity:    "low",
			Title:       "Dashboard metrics delay",
			Description: "Prometheus cache TTL is 10s",
			Workaround:  "Refresh page",
			Status:      "acknowledged",
		},
		{
			ID:          "KI-003",
			Severity:    "high",
			Title:       "No automatic certificate renewal",
			Description: "Automatic certificate rotation is not wired into the current stack lifecycle jobs.",
			Workaround:  "Manual cert-manager renewal",
			Status:      "planned",
		},
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

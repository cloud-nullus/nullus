package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/admin/port"
)

type KnownIssuesHandler struct {
	repo port.KnownIssuesRepository
}

func NewKnownIssuesHandler(repo port.KnownIssuesRepository) *KnownIssuesHandler {
	return &KnownIssuesHandler{repo: repo}
}

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
	if h.repo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "known issues service is not configured")
	}

	repoItems, err := h.repo.List(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	items := make([]knownIssueItem, 0, len(repoItems))
	for _, item := range repoItems {
		items = append(items, knownIssueItem{
			ID:          item.ID,
			Severity:    item.Severity,
			Title:       item.Title,
			Description: item.Description,
			Workaround:  item.Workaround,
			Status:      item.Status,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

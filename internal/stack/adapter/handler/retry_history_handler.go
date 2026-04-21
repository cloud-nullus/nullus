package handler

import (
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/labstack/echo/v4"
)

// RetryHistoryHandler serves GET /api/v1/stacks/:id/retry-history. It reads
// audit events via audit.Reader (F8-UIUX-RetryAuditSurface) and returns
// only the ones emitted by the Retry handler, mapping the payload fields
// that the frontend panel consumes.
type RetryHistoryHandler struct {
	reader audit.Reader
}

// NewRetryHistoryHandler wires the audit Reader that backs the endpoint.
func NewRetryHistoryHandler(reader audit.Reader) *RetryHistoryHandler {
	return &RetryHistoryHandler{reader: reader}
}

// RegisterRoutes attaches the retry-history endpoint to the stacks group.
// Mirrors the convention of every other stack handler in this package.
func (h *RetryHistoryHandler) RegisterRoutes(stacks *echo.Group) {
	stacks.GET("/:id/retry-history", h.GetRetryHistory)
}

type retryHistoryItem struct {
	ID                  string    `json:"id"`
	Timestamp           time.Time `json:"timestamp"`
	Actor               string    `json:"actor"`
	PreviousState       string    `json:"previousState,omitempty"`
	AcknowledgeWarnings bool      `json:"acknowledgeWarnings"`
	Verdict             string    `json:"verdict,omitempty"`
	IssueCodes          []string  `json:"issueCodes,omitempty"`
}

type retryHistoryResponse struct {
	Items []retryHistoryItem `json:"items"`
}

// GetRetryHistory returns the retry audit entries for the given stack id.
// Missing stacks yield a 200 with an empty items slice — consistent with
// the rest of the stack list surface and friendlier to eventual-consistency
// UI flows than a 404.
func (h *RetryHistoryHandler) GetRetryHistory(c echo.Context) error {
	stackID := c.Param("id")
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack id is required")
	}
	entries, err := h.reader.ListByResource(c.Request().Context(), "stack", stackID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "RETRY_HISTORY_FAILED", err.Error())
	}
	items := make([]retryHistoryItem, 0, len(entries))
	for _, e := range entries {
		if e.Entry.Action != "retry" {
			continue
		}
		items = append(items, mapAuditEntryToRetryItem(e))
	}
	return c.JSON(http.StatusOK, retryHistoryResponse{Items: items})
}

// mapAuditEntryToRetryItem extracts the payload fields the Retry handler
// records on emit. Unknown keys are ignored; missing keys yield zero values
// so the response shape stays stable regardless of server version.
func mapAuditEntryToRetryItem(e audit.TimedEntry) retryHistoryItem {
	item := retryHistoryItem{
		ID:        e.ID,
		Timestamp: e.Timestamp,
		Actor:     e.Entry.UserID,
	}
	details := e.Entry.Details
	if details == nil {
		return item
	}
	if v, ok := details["previous_state"].(string); ok {
		item.PreviousState = v
	}
	if v, ok := details["acknowledge_warnings"].(bool); ok {
		item.AcknowledgeWarnings = v
	}
	if v, ok := details["compatibility_verdict"].(string); ok {
		item.Verdict = v
	}
	switch codes := details["issue_codes"].(type) {
	case []string:
		item.IssueCodes = append(item.IssueCodes, codes...)
	case []any:
		for _, raw := range codes {
			if s, ok := raw.(string); ok {
				item.IssueCodes = append(item.IssueCodes, s)
			}
		}
	}
	return item
}

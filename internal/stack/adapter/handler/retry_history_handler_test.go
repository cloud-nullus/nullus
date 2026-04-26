package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/audit"
)

func newRetryHistoryEcho(t *testing.T, sink *audit.MemorySink, stackID string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/"+stackID+"/retry-history", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(stackID)
	h := NewRetryHistoryHandler(sink)
	require.NoError(t, h.GetRetryHistory(ctx))
	return rec
}

func TestRetryHistoryHandler_FiltersNonRetryActions(t *testing.T) {
	sink := audit.NewMemorySink()
	ctx := context.Background()
	require.NoError(t, sink.Log(ctx, audit.AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1", UserID: "u-1",
		Details: map[string]any{"previous_state": "failed", "compatibility_verdict": "pass"}}))
	require.NoError(t, sink.Log(ctx, audit.AuditEntry{Action: "retry", ResourceType: "stack", ResourceID: "s1", UserID: "u-1",
		Details: map[string]any{"previous_state": "rolled_back", "acknowledge_warnings": true, "compatibility_verdict": "warn", "issue_codes": []any{"TOOL_ARCH_UNSUPPORTED"}}}))
	require.NoError(t, sink.Log(ctx, audit.AuditEntry{Action: "deploy", ResourceType: "stack", ResourceID: "s1", UserID: "u-1"}))

	rec := newRetryHistoryEcho(t, sink, "s1")
	assert.Equal(t, http.StatusOK, rec.Code)
	var body retryHistoryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 2, "deploy action must be filtered out by the handler")
	for _, it := range body.Items {
		assert.NotEmpty(t, it.ID)
		assert.False(t, it.Timestamp.IsZero())
		assert.Equal(t, "u-1", it.Actor)
	}
}

func TestRetryHistoryHandler_NormalisesIssueCodes(t *testing.T) {
	sink := audit.NewMemorySink()
	// issue_codes as []any is what json-unmarshalling would produce.
	require.NoError(t, sink.Log(context.Background(), audit.AuditEntry{
		Action: "retry", ResourceType: "stack", ResourceID: "s1", UserID: "u-1",
		Details: map[string]any{
			"compatibility_verdict": "warn",
			"acknowledge_warnings":  true,
			"issue_codes":           []any{"TOOL_ARCH_UNSUPPORTED", "CLUSTER_ARCH_UNKNOWN"},
		},
	}))

	rec := newRetryHistoryEcho(t, sink, "s1")
	var body retryHistoryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, []string{"TOOL_ARCH_UNSUPPORTED", "CLUSTER_ARCH_UNKNOWN"}, body.Items[0].IssueCodes)
	assert.True(t, body.Items[0].AcknowledgeWarnings)
	assert.Equal(t, "warn", body.Items[0].Verdict)
}

func TestRetryHistoryHandler_UnknownStack_EmptyItems200(t *testing.T) {
	sink := audit.NewMemorySink()
	// Seed an unrelated stack to prove the handler scopes to the requested id.
	require.NoError(t, sink.Log(context.Background(), audit.AuditEntry{
		Action: "retry", ResourceType: "stack", ResourceID: "other", UserID: "u-1",
	}))

	rec := newRetryHistoryEcho(t, sink, "missing-stack")
	assert.Equal(t, http.StatusOK, rec.Code)
	var body retryHistoryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Empty(t, body.Items)
}

// Compile-time check that the handler accepts any audit.Reader (both
// MemorySink and AuditLogger satisfy the interface).
var _ = func() *RetryHistoryHandler {
	_ = time.Now
	return NewRetryHistoryHandler((audit.Reader)(nil))
}

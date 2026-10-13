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

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
)

type fakeStackAlertLister struct {
	gotOrg   string
	gotOpen  bool
	gotLimit int
	items    []*domain.StackAlert
}

func (f *fakeStackAlertLister) List(_ context.Context, orgID string, openOnly bool, limit int) ([]*domain.StackAlert, error) {
	f.gotOrg, f.gotOpen, f.gotLimit = orgID, openOnly, limit
	return f.items, nil
}

func TestStackAlertHandler_ListsOrgAlerts(t *testing.T) {
	started := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	resolved := started.Add(30 * time.Minute)
	repo := &fakeStackAlertLister{items: []*domain.StackAlert{
		{ID: "a1", StackID: "stk_1", StackName: "gitlab", AlertName: "KubePodNotReady", Severity: "critical",
			Summary: "pod not ready", RunbookURL: "https://rb", StartedAt: started,
			Labels: map[string]string{"namespace": "nullus-gitlab", "pod": "gitlab-0"}},
		{ID: "a2", StackID: "stk_1", StackName: "gitlab", AlertName: "NodeFilesystemAlmostOutOfSpace", Severity: "critical",
			StartedAt: started.Add(-time.Hour), ResolvedAt: &resolved, Labels: map[string]string{}},
	}}
	e := echo.New()
	NewStackAlertHandler(usecase.NewListStackAlerts(repo)).RegisterRoutes(e.Group("/api/v1/observability"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/stack-alerts?state=open&limit=20", nil)
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "org-1", repo.gotOrg)
	assert.True(t, repo.gotOpen)
	assert.Equal(t, 20, repo.gotLimit)

	var body struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 2)
	first := body.Items[0]
	assert.Equal(t, "KubePodNotReady", first["alertname"])
	assert.Equal(t, "gitlab", first["stack_name"])
	assert.Equal(t, "nullus-gitlab/gitlab-0", first["target"])
	assert.Equal(t, true, first["active"])
	assert.Equal(t, false, body.Items[1]["active"])
	assert.NotEmpty(t, body.Items[1]["resolved_at"])
}

func TestListStackAlerts_ClampsLimit(t *testing.T) {
	repo := &fakeStackAlertLister{}
	uc := usecase.NewListStackAlerts(repo)
	_, err := uc.Execute(context.Background(), "org-1", false, 0)
	require.NoError(t, err)
	assert.Equal(t, 100, repo.gotLimit)
	_, err = uc.Execute(context.Background(), "org-1", false, 10_000)
	require.NoError(t, err)
	assert.Equal(t, 500, repo.gotLimit)
}

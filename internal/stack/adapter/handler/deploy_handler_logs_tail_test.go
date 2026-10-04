package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stacklog "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

func newTailEcho(t *testing.T) (*echo.Echo, *stackrepo.MemoryStackRepository, *stacklog.MemoryStreamer) {
	t.Helper()

	e := echo.New()
	repo := stackrepo.NewMemoryStackRepository()
	streamer := stacklog.NewMemoryStreamer()
	install := usecase.NewInstallStack(repo, streamer)
	h := stackhandler.NewDeployHandler(install, repo, streamer)

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1.Group("/stacks"), e)

	return e, repo, streamer
}

func tailRequest(t *testing.T, e http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestTailLogs_ReturnsRecentLines(t *testing.T) {
	e, repo, streamer := newTailEcho(t)
	id := seedStack(t, repo, domain.StateInstalling)

	for _, msg := range []string{"one", "two", "three"} {
		streamer.Stream(context.Background(), id, port.LogEntry{Level: "info", Message: msg})
	}

	rec := tailRequest(t, e, "/api/v1/stacks/"+id+"/deploy/logs/tail?lines=2")

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []port.LogEntry `json:"items"`
		Total int             `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Total)
	assert.Equal(t, "two", resp.Items[0].Message)
	assert.Equal(t, "three", resp.Items[1].Message)
}

func TestTailLogs_DefaultsTo100Lines(t *testing.T) {
	e, repo, streamer := newTailEcho(t)
	id := seedStack(t, repo, domain.StateInstalling)

	for i := 0; i < 150; i++ {
		streamer.Stream(context.Background(), id, port.LogEntry{Level: "info", Message: "x"})
	}

	rec := tailRequest(t, e, "/api/v1/stacks/"+id+"/deploy/logs/tail")

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 100, resp.Total)
}

func TestTailLogs_NoLogsYet_EmptyItems(t *testing.T) {
	e, repo, _ := newTailEcho(t)
	id := seedStack(t, repo, domain.StateInstalling)

	rec := tailRequest(t, e, "/api/v1/stacks/"+id+"/deploy/logs/tail")

	require.Equal(t, http.StatusOK, rec.Code)
	// 로그 없음은 빈 배열이다 — null 은 클라이언트가 누락과 구분할 수 없다.
	assert.JSONEq(t, `{"items":[],"total":0}`, rec.Body.String())
}

func TestTailLogs_StackNotFound(t *testing.T) {
	e, _, _ := newTailEcho(t)

	rec := tailRequest(t, e, "/api/v1/stacks/no-such/deploy/logs/tail")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "STACK_NOT_FOUND")
}

func TestTailLogs_InvalidLines(t *testing.T) {
	e, repo, _ := newTailEcho(t)
	id := seedStack(t, repo, domain.StateInstalling)

	for _, bad := range []string{"abc", "0", "-5"} {
		rec := tailRequest(t, e, "/api/v1/stacks/"+id+"/deploy/logs/tail?lines="+bad)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "lines=%s", bad)
	}
}

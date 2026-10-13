package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/notification/adapter/repository"
	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	"github.com/cloud-nullus/draft/internal/notification/usecase"
	sharedmw "github.com/cloud-nullus/draft/internal/shared/middleware"
)

const (
	tgToken  = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"
	tgSecret = "AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"
)

type stubSender struct {
	err  error
	sent int
}

func (s *stubSender) Send(context.Context, *domain.Channel, domain.Message) error {
	s.sent++
	return s.err
}

type fixture struct {
	e      *echo.Echo
	sender *stubSender
}

func newFixture() *fixture {
	channels := repository.NewMemoryChannelRepository()
	deliveries := repository.NewMemoryDeliveryRepository()
	sender := &stubSender{}
	deliverer := usecase.NewDeliverer(map[domain.ChannelType]port.Sender{
		domain.ChannelTelegram: sender, domain.ChannelEmail: sender, domain.ChannelSlack: sender,
	}, deliveries)
	h := NewNotificationHandler(usecase.NewChannelService(channels), usecase.NewSendTest(channels, deliverer),
		usecase.NewListDeliveries(deliveries))
	e := echo.New()
	e.HTTPErrorHandler = sharedmw.AppErrorHandler
	h.RegisterRoutes(e.Group("/api/v1/admin"))
	return &fixture{e: e, sender: sender}
}

func (f *fixture) do(t *testing.T, method, path, org, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if org != "" {
		req.Header.Set("X-Org-ID", org)
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

const createTelegram = `{"name":"운영 텔레그램","type":"telegram","settings":{"chat_id":"-100123"},
  "secret":"` + tgToken + `","events":["backup_failed"]}`

func TestNotificationHandler_CreateNeverReturnsSecret(t *testing.T) {
	f := newFixture()
	rec, body := f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels", "org-1", createTelegram)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), tgSecret)
	assert.Equal(t, true, body["secret_set"])
	assert.Equal(t, "bot 123456789", body["secret_hint"])
	assert.Equal(t, true, body["active"])
	assert.Equal(t, "telegram", body["type"])
	assert.Equal(t, "-100123", body["settings"].(map[string]any)["chat_id"])
}

func TestNotificationHandler_CreateRejectsInvalidWithField(t *testing.T) {
	f := newFixture()
	rec, body := f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels", "org-1",
		`{"name":"x","type":"telegram","settings":{"chat_id":"-1"},"secret":"nope","events":["backup_failed"]}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "NOTIFICATION_CHANNEL_INVALID", errBody["code"])
	assert.Equal(t, "secret", errBody["detail"])
	assert.NotContains(t, rec.Body.String(), "nope", "잘못 넣은 비밀도 되돌려 보내지 않는다")
}

func TestNotificationHandler_ChannelsAreScopedToOrg(t *testing.T) {
	f := newFixture()
	_, created := f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels", "org-1", createTelegram)
	id := created["id"].(string)

	rec, body := f.do(t, http.MethodGet, "/api/v1/admin/notifications/channels", "org-2", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 0, body["total"])

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/notifications/channels/" + id, ""},
		{http.MethodPatch, "/api/v1/admin/notifications/channels/" + id, `{"name":"탈취"}`},
		{http.MethodDelete, "/api/v1/admin/notifications/channels/" + id, ""},
		{http.MethodPost, "/api/v1/admin/notifications/channels/" + id + "/test", ""},
	} {
		rec, _ := f.do(t, tc.method, tc.path, "org-2", tc.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, tc.method+" "+tc.path)
	}
	assert.Zero(t, f.sender.sent)
}

func TestNotificationHandler_PatchKeepsSecretAndDeleteRemoves(t *testing.T) {
	f := newFixture()
	_, created := f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels", "org-1", createTelegram)
	id := created["id"].(string)

	rec, body := f.do(t, http.MethodPatch, "/api/v1/admin/notifications/channels/"+id, "org-1",
		`{"name":"이름 바꿈","secret":"","events":["backup_failed","backup_succeeded"],"active":false}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "이름 바꿈", body["name"])
	assert.Equal(t, true, body["secret_set"], "빈 비밀은 기존 값을 지킨다")
	assert.Equal(t, false, body["active"])
	assert.Equal(t, []any{"backup_failed", "backup_succeeded"}, body["events"])

	rec, _ = f.do(t, http.MethodDelete, "/api/v1/admin/notifications/channels/"+id, "org-1", "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	rec, _ = f.do(t, http.MethodGet, "/api/v1/admin/notifications/channels/"+id, "org-1", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// 테스트 전송은 실패해도 200 이다 — 결과가 응답이다. 이력에도 남는다.
func TestNotificationHandler_TestSendReportsResultAndRecordsHistory(t *testing.T) {
	f := newFixture()
	_, created := f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels", "org-1", createTelegram)
	id := created["id"].(string)

	rec, body := f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels/"+id+"/test", "org-1", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "sent", body["status"])
	assert.Equal(t, "test", body["event"])

	f.sender.err = &port.SendError{Err: errors.New("HTTP 400: Bad Request: chat not found")}
	rec, body = f.do(t, http.MethodPost, "/api/v1/admin/notifications/channels/"+id+"/test", "org-1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "failed", body["status"])
	assert.Equal(t, "HTTP 400: Bad Request: chat not found", body["error"])

	rec, hist := f.do(t, http.MethodGet, "/api/v1/admin/notifications/history?limit=10", "org-1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	items := hist["items"].([]any)
	require.Len(t, items, 2)
	latest := items[0].(map[string]any)
	assert.Equal(t, "failed", latest["status"])
	assert.Equal(t, "운영 텔레그램", latest["channel_name"])
}

func TestNotificationHandler_ListEvents(t *testing.T) {
	f := newFixture()
	rec, body := f.do(t, http.MethodGet, "/api/v1/admin/notifications/events", "org-1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []any{"telegram", "email", "slack"}, body["channel_types"])
	items := body["items"].([]any)
	require.Len(t, items, 3)
	first := items[0].(map[string]any)
	assert.Equal(t, "backup_failed", first["type"])
	assert.Equal(t, true, first["default_on"])
}

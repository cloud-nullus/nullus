// Package handler 는 알림 채널 API 다(관리자 전용 — /api/v1/admin/notifications).
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/usecase"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharedmw "github.com/cloud-nullus/draft/internal/shared/middleware"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// NotificationHandler 는 채널 CRUD · 테스트 전송 · 발송 이력 · 이벤트 목록이다. 응답에는 채널의 비밀
// (봇 토큰 · 웹훅 URL · SMTP 비밀번호)을 싣지 않는다 — secret_set 과 secret_hint 만.
type NotificationHandler struct {
	channels   *usecase.ChannelService
	sendTest   *usecase.SendTest
	deliveries *usecase.ListDeliveries
}

func NewNotificationHandler(channels *usecase.ChannelService, sendTest *usecase.SendTest, deliveries *usecase.ListDeliveries) *NotificationHandler {
	return &NotificationHandler{channels: channels, sendTest: sendTest, deliveries: deliveries}
}

func (h *NotificationHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/notifications/events", h.ListEvents)
	g.GET("/notifications/channels", h.ListChannels)
	g.POST("/notifications/channels", h.CreateChannel)
	g.GET("/notifications/channels/:id", h.GetChannel)
	g.PATCH("/notifications/channels/:id", h.UpdateChannel)
	g.DELETE("/notifications/channels/:id", h.DeleteChannel)
	g.POST("/notifications/channels/:id/test", h.TestChannel)
	g.GET("/notifications/history", h.ListHistory)
}

type channelResponse struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Settings   domain.Settings `json:"settings"`
	SecretSet  bool            `json:"secret_set"`
	SecretHint string          `json:"secret_hint,omitempty"`
	Events     []string        `json:"events"`
	Active     bool            `json:"active"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func toChannelResponse(ch *domain.Channel) channelResponse {
	events := make([]string, 0, len(ch.Events))
	for _, e := range ch.Events {
		events = append(events, string(e))
	}
	settings := ch.Settings
	if settings.To == nil && ch.Type == domain.ChannelEmail {
		settings.To = []string{}
	}
	return channelResponse{
		ID: ch.ID, Name: ch.Name, Type: string(ch.Type), Settings: settings,
		SecretSet: ch.Secret != "", SecretHint: ch.SecretHint(), Events: events, Active: ch.Active,
		CreatedAt: ch.CreatedAt, UpdatedAt: ch.UpdatedAt,
	}
}

type deliveryResponse struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id,omitempty"`
	ChannelName string    `json:"channel_name"`
	ChannelType string    `json:"channel_type"`
	Event       string    `json:"event"`
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	Attempts    int       `json:"attempts"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func toDeliveryResponse(d *domain.Delivery) deliveryResponse {
	return deliveryResponse{
		ID: d.ID, ChannelID: d.ChannelID, ChannelName: d.ChannelName, ChannelType: string(d.ChannelType),
		Event: string(d.Event), Status: string(d.Status), Title: d.Title, Attempts: d.Attempts,
		Error: d.Error, CreatedAt: d.CreatedAt,
	}
}

type createChannelRequest struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Settings domain.Settings `json:"settings"`
	Secret   string          `json:"secret"`
	Events   []string        `json:"events"`
	Active   *bool           `json:"active"`
}

type updateChannelRequest struct {
	Name     *string          `json:"name"`
	Settings *domain.Settings `json:"settings"`
	Secret   *string          `json:"secret"`
	Events   *[]string        `json:"events"`
	Active   *bool            `json:"active"`
}

func (h *NotificationHandler) ListEvents(c echo.Context) error {
	types := make([]string, 0, 3)
	for _, t := range domain.ChannelTypes() {
		types = append(types, string(t))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": domain.EventCatalog(), "channel_types": types})
}

func (h *NotificationHandler) ListChannels(c echo.Context) error {
	items, err := h.channels.List(c.Request().Context(), resolveOrgID(c))
	if err != nil {
		return mapError(err)
	}
	out := make([]channelResponse, 0, len(items))
	for _, ch := range items {
		out = append(out, toChannelResponse(ch))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func (h *NotificationHandler) CreateChannel(c echo.Context) error {
	var req createChannelRequest
	if err := c.Bind(&req); err != nil {
		return badRequest("요청 본문을 읽을 수 없습니다")
	}
	ch, err := h.channels.Create(c.Request().Context(), usecase.CreateChannelInput{
		OrgID: resolveOrgID(c), Name: req.Name, Type: domain.ChannelType(strings.TrimSpace(req.Type)),
		Settings: req.Settings, Secret: req.Secret, Events: toEvents(req.Events), Active: req.Active,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, toChannelResponse(ch))
}

func (h *NotificationHandler) GetChannel(c echo.Context) error {
	ch, err := h.channels.Get(c.Request().Context(), resolveOrgID(c), c.Param("id"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, toChannelResponse(ch))
}

func (h *NotificationHandler) UpdateChannel(c echo.Context) error {
	var req updateChannelRequest
	if err := c.Bind(&req); err != nil {
		return badRequest("요청 본문을 읽을 수 없습니다")
	}
	u := domain.ChannelUpdate{Name: req.Name, Settings: req.Settings, Secret: req.Secret, Active: req.Active}
	if req.Events != nil {
		events := toEvents(*req.Events)
		u.Events = &events
	}
	ch, err := h.channels.Update(c.Request().Context(), resolveOrgID(c), c.Param("id"), u)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, toChannelResponse(ch))
}

func (h *NotificationHandler) DeleteChannel(c echo.Context) error {
	if err := h.channels.Delete(c.Request().Context(), resolveOrgID(c), c.Param("id")); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// TestChannel 은 테스트 메시지를 보낸다. 발송이 실패해도 200 이다 — 결과(status · error)가 응답이다.
func (h *NotificationHandler) TestChannel(c echo.Context) error {
	d, err := h.sendTest.Execute(c.Request().Context(), resolveOrgID(c), c.Param("id"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, toDeliveryResponse(d))
}

func (h *NotificationHandler) ListHistory(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	items, err := h.deliveries.Execute(c.Request().Context(), resolveOrgID(c), limit)
	if err != nil {
		return mapError(err)
	}
	out := make([]deliveryResponse, 0, len(items))
	for _, d := range items {
		out = append(out, toDeliveryResponse(d))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func toEvents(in []string) []sharednotify.EventType {
	out := make([]sharednotify.EventType, 0, len(in))
	for _, e := range in {
		out = append(out, sharednotify.EventType(strings.TrimSpace(e)))
	}
	return out
}

// resolveOrgID 는 로그인한 사용자의 조직, 없으면 X-Org-ID, 그것도 없으면 기본 조직이다(백업 API 와 같다).
func resolveOrgID(c echo.Context) string {
	if orgID := sharedmw.OrgIDFromEchoContext(c); orgID != "" {
		return orgID
	}
	if orgID := strings.TrimSpace(c.Request().Header.Get("X-Org-ID")); orgID != "" {
		return orgID
	}
	return shareddomain.DefaultOrgID()
}

func badRequest(msg string) error {
	return shareddomain.NewAppError("NOTIFICATION_BAD_REQUEST", http.StatusBadRequest, msg, "", false)
}

func mapError(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		return shareddomain.NewAppError("NOTIFICATION_CHANNEL_INVALID", http.StatusBadRequest, ve.Reason, ve.Field, false)
	case errors.Is(err, domain.ErrChannelNotFound):
		return shareddomain.NewAppError("NOTIFICATION_CHANNEL_NOT_FOUND", http.StatusNotFound, "알림 채널이 없습니다", "", false)
	case errors.Is(err, usecase.ErrOrgRequired):
		return shareddomain.NewAppError("ORG_REQUIRED", http.StatusBadRequest, "조직이 필요합니다", "", false)
	default:
		return shareddomain.NewAppError("NOTIFICATION_INTERNAL", http.StatusInternalServerError, err.Error(), "", false)
	}
}

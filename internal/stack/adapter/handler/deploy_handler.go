package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

// DeployHandler handles HTTP and WebSocket requests for stack deployments.
type DeployHandler struct {
	installStack *usecase.InstallStack
	stackRepo    port.StackRepository
	streamer     port.LogStreamer
}

// NewDeployHandler constructs a DeployHandler.
func NewDeployHandler(
	installStack *usecase.InstallStack,
	stackRepo port.StackRepository,
	streamer port.LogStreamer,
) *DeployHandler {
	return &DeployHandler{
		installStack: installStack,
		stackRepo:    stackRepo,
		streamer:     streamer,
	}
}

// RegisterRoutes registers deployment routes on the given Echo instance and group.
func (h *DeployHandler) RegisterRoutes(v1 *echo.Group, e *echo.Echo) {
	v1.POST("/stacks/:id/deploy", h.Deploy)
	v1.GET("/stacks/:id/status", h.Status)
	e.GET("/ws/deployments/:id/logs", h.StreamLogs)
}

// deployResponse is the response body for POST /stacks/:id/deploy.
type deployResponse struct {
	StackID string `json:"stack_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Deploy handles POST /api/v1/stacks/:id/deploy.
// It starts the installation asynchronously and returns 202 Accepted.
func (h *DeployHandler) Deploy(c echo.Context) error {
	id := c.Param("id")

	if err := h.installStack.Execute(c.Request().Context(), usecase.InstallStackInput{StackID: id}); err != nil {
		return errorResponse(c, http.StatusBadRequest, "DEPLOY_FAILED", err.Error())
	}

	return c.JSON(http.StatusAccepted, deployResponse{
		StackID: id,
		Status:  "accepted",
		Message: "deployment started; subscribe to /ws/deployments/" + id + "/logs for real-time logs",
	})
}

// statusResponse is the response body for GET /stacks/:id/status.
type statusResponse struct {
	StackID   string    `json:"stack_id"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Status handles GET /api/v1/stacks/:id/status.
func (h *DeployHandler) Status(c echo.Context) error {
	id := c.Param("id")

	stack, err := h.stackRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": statusResponse{
			StackID:   stack.ID,
			State:     string(stack.State),
			UpdatedAt: stack.UpdatedAt,
		},
	})
}

// StreamLogs handles GET /ws/deployments/:id/logs.
// It upgrades the connection to WebSocket and streams log entries until the
// deployment completes or the client disconnects.
func (h *DeployHandler) StreamLogs(c echo.Context) error {
	id := c.Param("id")

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		ch := h.streamer.Subscribe(id)
		defer h.streamer.Unsubscribe(id, ch)

		for entry := range ch {
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			if err := websocket.Message.Send(ws, string(data)); err != nil {
				// Client disconnected.
				return
			}
		}
	}).ServeHTTP(c.Response(), c.Request())

	return nil
}

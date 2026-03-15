package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// DeployHandler handles HTTP and WebSocket requests for stack deployments.
type DeployHandler struct {
	installStack *usecase.InstallStack
	stackRepo    port.StackRepository
	streamer     port.LogStreamer
	audit        *audit.AuditLogger
}

// NewDeployHandler constructs a DeployHandler.
func NewDeployHandler(
	installStack *usecase.InstallStack,
	stackRepo port.StackRepository,
	streamer port.LogStreamer,
	auditLogger ...*audit.AuditLogger,
) *DeployHandler {
	var logger *audit.AuditLogger
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &DeployHandler{
		installStack: installStack,
		stackRepo:    stackRepo,
		streamer:     streamer,
		audit:        logger,
	}
}

// RegisterRoutes registers deployment routes on the given Echo instance and group.
func (h *DeployHandler) RegisterRoutes(v1 *echo.Group, e *echo.Echo) {
	v1.POST("/stacks/:id/deploy", h.Deploy)
	v1.GET("/stacks/:id/status", h.Status)
	v1.GET("/stacks/:id/deploy/logs", h.StreamLogs)
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
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "deploy",
			ResourceType: "stack",
			ResourceID:   id,
			Details:      map[string]any{},
			IPAddress:    c.RealIP(),
		})
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

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
)

type wsLogMessage struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Phase     string `json:"phase,omitempty"`
	Step      string `json:"step,omitempty"`
	Level     string `json:"level,omitempty"`
	Message   string `json:"message,omitempty"`
	Progress  int    `json:"progress"`
	Status    string `json:"status,omitempty"`
}

var stepProgress = map[string]int{
	"validate":                5,
	"installing_cert_manager": 15,
	"installing_minio":        25,
	"installing_gitlab":       40,
	"installing_argocd":       55,
	"installing_runner":       65,
	"installing_prometheus":   75,
	"installing_grafana":      85,
	"integration_check":       90,
	"configuring":             93,
	"health_check":            96,
	"completed":               100,
}

func (h *DeployHandler) StreamLogs(c echo.Context) error {
	id := c.Param("id")

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return nil
	}
	defer conn.Close()

	ch := h.streamer.Subscribe(id)
	defer h.streamer.Unsubscribe(id, ch)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadLimit(512)
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(wsPongWait))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case entry, ok := <-ch:
			if !ok {
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return nil
			}

			progress := stepProgress[entry.Step]

			msg := wsLogMessage{
				Type:      "log",
				Timestamp: entry.Timestamp.Format(time.RFC3339),
				Phase:     entry.Phase,
				Step:      entry.Step,
				Level:     entry.Level,
				Message:   entry.Message,
				Progress:  progress,
			}

			if entry.Step == "completed" {
				msg.Type = "status"
				msg.Status = "success"
			} else if entry.Step == "failed" || entry.Step == "rolling_back" || entry.Step == "rolled_back" {
				msg.Type = "status"
				msg.Status = "failed"
			}

			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return nil
			}
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return nil
			}
		case <-done:
			return nil
		}
	}
}

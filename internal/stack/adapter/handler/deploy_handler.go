package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// DeployHandler handles HTTP and WebSocket requests for stack deployments.
type DeployHandler struct {
	installStack          *usecase.InstallStack
	stackRepo             port.StackRepository
	streamer              port.LogStreamer
	validateCompatibility *usecase.ValidateCompatibility
	audit                 audit.Sink
}

// DeployHandlerOption configures optional dependencies on DeployHandler.
type DeployHandlerOption func(*DeployHandler)

// WithValidateCompatibility wires the server-side Pre-Deploy Gate. When set,
// Deploy() runs ValidateCompatibility in persisted mode before kicking off
// install, blocking `fail` verdicts and `warn` without an explicit ack.
func WithValidateCompatibility(uc *usecase.ValidateCompatibility) DeployHandlerOption {
	return func(h *DeployHandler) { h.validateCompatibility = uc }
}

// NewDeployHandler constructs a DeployHandler. The audit logger is variadic
// for backward compatibility with existing call sites; any DeployHandlerOption
// values passed via the same variadic slot are applied.
//
// Extra callers passing *audit.AuditLogger or any audit.Sink keep the old
// positional behavior. Callers that also want to inject the compatibility
// gate should use WithValidateCompatibility via WithOptions.
func NewDeployHandler(
	installStack *usecase.InstallStack,
	stackRepo port.StackRepository,
	streamer port.LogStreamer,
	auditLogger ...audit.Sink,
) *DeployHandler {
	var logger audit.Sink
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

// WithOptions applies DeployHandlerOption values after construction. Kept as
// a separate step so the original NewDeployHandler signature stays
// backward-compatible (auditLogger positional variadic).
func (h *DeployHandler) WithOptions(opts ...DeployHandlerOption) *DeployHandler {
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers deployment routes on the given Echo instance and group.
func (h *DeployHandler) RegisterRoutes(v1 *echo.Group, e *echo.Echo) {
	stacks := v1.Group("/stacks")
	stacks.POST("/:id/deploy", h.Deploy)
	stacks.POST("/:id/retry", h.Retry)
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

// deployRequest captures the optional Deploy body. Backward-compatible: a
// completely empty body (older clients) still parses with default values.
type deployRequest struct {
	// AcknowledgeWarnings is the explicit opt-in for proceeding when the
	// server-side Pre-Deploy Gate returns overall.state == "warn". Missing
	// or false ⇒ treat as un-acknowledged and block with
	// DEPLOY_COMPAT_WARN_UNACK.
	AcknowledgeWarnings bool `json:"acknowledge_warnings"`
}

// deployGateErrorResponse writes a DEPLOY_COMPAT_* error with the full
// verdict object attached so the frontend can re-use its existing Pre-Deploy
// Gate UI to display issues and drive the ack flow.
func deployGateErrorResponse(
	c echo.Context,
	code string,
	message string,
	verdict *usecase.ValidateCompatibilityOutput,
) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"code":        code,
			"http_status": http.StatusBadRequest,
			"message":     message,
			"verdict": map[string]any{
				"overall":            verdict.Overall,
				"issues":             verdict.Issues,
				"node_architectures": verdict.NodeArchitectures,
				"matrix":             verdict.Matrix,
				"message":            verdict.Message,
				"checkedAt":          verdict.CheckedAt.Format(time.RFC3339),
			},
		},
	})
}

func issueCodes(issues []usecase.ValidationIssue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		if i.Code != "" {
			out = append(out, i.Code)
		}
	}
	return out
}

// preDeployGateResult is the runPreDeployGate outcome. `blocked` is true
// when the verdict fails the gate and the caller should return the bound
// echo error directly; otherwise the caller proceeds with install/retry.
type preDeployGateResult struct {
	verdict *usecase.ValidateCompatibilityOutput
	blocked bool
	handled bool
}

// runPreDeployGate re-executes ValidateCompatibility in persisted mode for
// the given stack and decides whether the deployment/retry may proceed.
// Used by both Deploy() and Retry() so the gate logic lives in one place
// (F8 follow-up Phase 3 refactor).
func (h *DeployHandler) runPreDeployGate(c echo.Context, stackID string, ack bool) *preDeployGateResult {
	if h.validateCompatibility == nil {
		return &preDeployGateResult{}
	}
	verdict, err := h.validateCompatibility.Execute(
		c.Request().Context(),
		usecase.ValidateCompatibilityInput{StackID: stackID},
	)
	if err != nil {
		_ = errorResponse(c, http.StatusBadRequest, "DEPLOY_COMPAT_VALIDATE_FAILED", err.Error())
		return &preDeployGateResult{blocked: true, handled: true}
	}
	switch verdict.Overall.State {
	case "fail":
		_ = deployGateErrorResponse(c, "DEPLOY_COMPAT_FAIL",
			"deployment blocked by compatibility gate (fail)", verdict)
		return &preDeployGateResult{
			verdict: verdict,
			blocked: true,
			handled: true,
		}
	case "warn":
		if !ack {
			_ = deployGateErrorResponse(c, "DEPLOY_COMPAT_WARN_UNACK",
				"deployment blocked: explicit acknowledgement required for warn verdict", verdict)
			return &preDeployGateResult{
				verdict: verdict,
				blocked: true,
				handled: true,
			}
		}
	}
	return &preDeployGateResult{verdict: verdict}
}

// Deploy handles POST /api/v1/stacks/:id/deploy.
// It starts the installation asynchronously and returns 202 Accepted.
//
// F8-F3 adds a server-side Pre-Deploy Gate: when the compatibility use case
// is wired, the handler re-runs validation in persisted mode against
// stack.Tools + stack.ClusterID before kicking off install. `fail` blocks
// with DEPLOY_COMPAT_FAIL; `warn` blocks with DEPLOY_COMPAT_WARN_UNACK unless
// the request body explicitly sets acknowledge_warnings=true.
func (h *DeployHandler) Deploy(c echo.Context) error {
	id := c.Param("id")

	// Body is optional. Older clients send empty or no body; treat as
	// acknowledge_warnings=false. Bind errors are non-fatal when the body
	// is empty but surface real parse errors.
	var req deployRequest
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return errorResponse(c, http.StatusBadRequest, "DEPLOY_REQUEST_INVALID", err.Error())
		}
	}

	auditDetails := map[string]any{
		"acknowledge_warnings": req.AcknowledgeWarnings,
	}

	gate := h.runPreDeployGate(c, id, req.AcknowledgeWarnings)
	if gate.handled {
		return nil
	}
	if gate.blocked {
		return nil // gate.err is nil here only if validate was never run
	}
	if gate.verdict != nil {
		auditDetails["compatibility_verdict"] = gate.verdict.Overall.State
		auditDetails["issue_codes"] = issueCodes(gate.verdict.Issues)
	}

	if err := h.installStack.Execute(c.Request().Context(), usecase.InstallStackInput{StackID: id}); err != nil {
		return errorResponse(c, http.StatusBadRequest, "DEPLOY_FAILED", err.Error())
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "deploy",
			ResourceType: "stack",
			ResourceID:   id,
			Details:      auditDetails,
			IPAddress:    c.RealIP(),
		})
	}

	return c.JSON(http.StatusAccepted, deployResponse{
		StackID: id,
		Status:  "accepted",
		Message: "deployment started; subscribe to /ws/deployments/" + id + "/logs for real-time logs",
	})
}

// Retry handles POST /api/v1/stacks/:id/retry. Phase 3 of the F8 follow-up:
// transitions a stack from `failed` / `rolled_back` back to `pending` and
// re-runs the Pre-Deploy Gate + InstallStack, mirroring the Deploy flow.
// Other states return 409 STACK_RETRY_INVALID_STATE.
func (h *DeployHandler) Retry(c echo.Context) error {
	id := c.Param("id")

	stack, err := h.stackRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}
	if stack == nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found")
	}
	if stack.State != domain.StateFailed && stack.State != domain.StateRolledBack {
		return errorResponse(c, http.StatusConflict, "STACK_RETRY_INVALID_STATE",
			"retry requires state failed or rolled_back, got "+string(stack.State))
	}

	var req deployRequest
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return errorResponse(c, http.StatusBadRequest, "DEPLOY_REQUEST_INVALID", err.Error())
		}
	}

	auditDetails := map[string]any{
		"acknowledge_warnings": req.AcknowledgeWarnings,
		"previous_state":       string(stack.State),
	}

	gate := h.runPreDeployGate(c, id, req.AcknowledgeWarnings)
	if gate.handled {
		return nil
	}
	if gate.blocked {
		return nil
	}
	if gate.verdict != nil {
		auditDetails["compatibility_verdict"] = gate.verdict.Overall.State
		auditDetails["issue_codes"] = issueCodes(gate.verdict.Issues)
	}

	// Wind the state back to pending so InstallStack can transition forward
	// normally (pending → validating → ...). validTransitions already allows
	// failed → pending and rolled_back → pending.
	if err := stack.TransitionTo(domain.StatePending); err != nil {
		return errorResponse(c, http.StatusConflict, "STACK_RETRY_INVALID_STATE", err.Error())
	}
	if err := h.stackRepo.Update(c.Request().Context(), stack); err != nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_UPDATE_FAILED", err.Error())
	}

	if err := h.installStack.Execute(c.Request().Context(), usecase.InstallStackInput{StackID: id}); err != nil {
		return errorResponse(c, http.StatusBadRequest, "DEPLOY_FAILED", err.Error())
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "retry",
			ResourceType: "stack",
			ResourceID:   id,
			Details:      auditDetails,
			IPAddress:    c.RealIP(),
		})
	}

	return c.JSON(http.StatusAccepted, deployResponse{
		StackID: id,
		Status:  "accepted",
		Message: "retry started; subscribe to /ws/deployments/" + id + "/logs for real-time logs",
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
	if stack == nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found")
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
	"validate":                 5,
	"installing_cert_manager":  15,
	"installing_minio":         25,
	"installing_gitlab":        40,
	"installing_argocd":        55,
	"installing_runner":        65,
	"installing_prometheus":    75,
	"installing_grafana":       85,
	"installing_logging":       87,
	"installing_log_search":    88,
	"installing_opentelemetry": 89,
	"installing_gateway":       90,
	"integration_check":        90,
	"configuring":              93,
	"health_check":             96,
	"completed":                100,
	"deleting_started":         5,
	"deleting_release":         45,
	"deleting_manifest":        75,
	"deleted":                  100,
	"delete_failed":            100,
}

func (h *DeployHandler) StreamLogs(c echo.Context) error {
	id := c.Param("id")

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return nil
	}
	defer conn.Close()

	ch := h.streamer.Subscribe(id)
	defer h.streamer.Unsubscribe(id, ch)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadLimit(512)
		if err := conn.SetReadDeadline(time.Now().Add(wsPongWait)); err != nil {
			log.Printf("websocket set read deadline error: %v", err)
			return
		}
		conn.SetPongHandler(func(string) error {
			if err := conn.SetReadDeadline(time.Now().Add(wsPongWait)); err != nil {
				log.Printf("websocket pong set read deadline error: %v", err)
				return err
			}
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				log.Printf("websocket read error: %v", err)
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
				if err := conn.SetWriteDeadline(time.Now().Add(wsWriteWait)); err != nil {
					log.Printf("websocket set write deadline error: %v", err)
					return nil
				}
				if err := conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				); err != nil {
					log.Printf("websocket close message error: %v", err)
				}
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

			if entry.Step == "completed" || entry.Step == "deleted" {
				msg.Type = "status"
				msg.Status = "success"
			} else if entry.Step == "failed" || entry.Step == "rolling_back" || entry.Step == "rolled_back" || entry.Step == "delete_failed" {
				msg.Type = "status"
				msg.Status = "failed"
			}

			if err := conn.SetWriteDeadline(time.Now().Add(wsWriteWait)); err != nil {
				log.Printf("websocket set write deadline error: %v", err)
				return nil
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("websocket message marshal error: %v", err)
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("websocket write error: %v", err)
				return nil
			}
		case <-pingTicker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(wsWriteWait)); err != nil {
				log.Printf("websocket set write deadline error: %v", err)
				return nil
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("websocket ping error: %v", err)
				return nil
			}
		case <-done:
			return nil
		}
	}
}

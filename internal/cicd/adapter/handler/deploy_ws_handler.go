package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var cicdWsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	cicdWsWriteWait  = 10 * time.Second
	cicdWsPongWait   = 60 * time.Second
	cicdWsPingPeriod = (cicdWsPongWait * 9) / 10
)

type cicdWsLogMessage struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level,omitempty"`
	Message   string `json:"message,omitempty"`
	Progress  int    `json:"progress"`
	Status    string `json:"status,omitempty"`
}

func StreamCicdLogs(c echo.Context, tracker *kube.StepTracker) error {
	if tracker == nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}

	deploymentID := c.Param("id")

	conn, err := cicdWsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("cicd websocket upgrade error: %v", err)
		return nil
	}
	defer conn.Close()

	ch := tracker.Subscribe(deploymentID)
	defer tracker.Unsubscribe(deploymentID, ch)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadLimit(512)
		conn.SetReadDeadline(time.Now().Add(cicdWsPongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(cicdWsPongWait))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				log.Printf("cicd websocket read error: %v", err)
				return
			}
		}
	}()

	pingTicker := time.NewTicker(cicdWsPingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				conn.SetWriteDeadline(time.Now().Add(cicdWsWriteWait))
				if err := conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				); err != nil {
					log.Printf("cicd websocket close message error: %v", err)
				}
				return nil
			}

			msg := cicdWsLogMessage{
				Type:      "log",
				Timestamp: event.Timestamp.Format(time.RFC3339),
				Level:     event.Level,
				Message:   event.Message,
				Progress:  event.Progress,
			}
			if event.Status != "" {
				msg.Type = "status"
				msg.Status = event.Status
			}

			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("cicd websocket message marshal error: %v", err)
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(cicdWsWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("cicd websocket write error: %v", err)
				return nil
			}
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(cicdWsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("cicd websocket ping error: %v", err)
				return nil
			}
		case <-done:
			return nil
		}
	}
}

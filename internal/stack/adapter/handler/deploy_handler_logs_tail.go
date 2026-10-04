package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// tailDefaultLines 는 lines 쿼리 파라미터 생략 시 기본값이다.
const tailDefaultLines = 100

// TailLogs handles GET /api/v1/stacks/:id/deploy/logs/tail.
//
// WS 스트림(/deploy/logs)과 달리 최근 N줄만 한 번의 응답으로 돌려준다 —
// 연결을 붙들 수 없는 자동화 클라이언트(CLI·MCP)용 읽기 경로다.
func (h *DeployHandler) TailLogs(c echo.Context) error {
	id := c.Param("id")

	stack, err := h.stackRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}
	if stack == nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found")
	}

	lines := tailDefaultLines
	if raw := c.QueryParam("lines"); raw != "" {
		lines, err = strconv.Atoi(raw)
		if err != nil || lines <= 0 {
			return errorResponse(c, http.StatusBadRequest, "LOGS_TAIL_REQUEST_INVALID",
				"lines must be a positive integer")
		}
	}

	entries, err := h.streamer.Tail(c.Request().Context(), id, lines)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "LOGS_TAIL_FAILED", err.Error())
	}
	if entries == nil {
		// 로그 없음은 빈 배열이다 — null 은 클라이언트가 누락과 구분할 수 없다.
		entries = []port.LogEntry{}
	}

	return c.JSON(http.StatusOK, map[string]any{"items": entries, "total": len(entries)})
}

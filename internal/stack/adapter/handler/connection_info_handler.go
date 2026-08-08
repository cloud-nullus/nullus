package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// GetConnectionInfo handles GET /api/v1/stacks/:stackId/connection-info.
//
// 접속에 필요한 이름(Secret, 키, 엔드포인트)을 서버가 확정해 내려준다.
// 화면이 같은 값을 다시 조립하면 설치 경로와 규칙이 갈리는 순간 조용히
// 어긋나고, 존재하지 않는 리소스를 안내하게 된다.
func (h *StackHandler) GetConnectionInfo(c echo.Context) error {
	stackID := strings.TrimSpace(c.Param("stackId"))
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id is required")
	}
	if h.connectionInfo == nil {
		return errorResponse(c, http.StatusInternalServerError,
			"CONNECTION_INFO_UNAVAILABLE", "connection info use case is not wired")
	}

	info, err := h.connectionInfo.Execute(c.Request().Context(), stackID)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, info)
}

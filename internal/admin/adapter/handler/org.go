package handler

import "github.com/labstack/echo/v4"

// resolveOrgID 는 X-Org-ID 헤더(없으면 orgId 쿼리, 그것도 없으면 default-org)의 조직이다.
// 알림 설정 핸들러가 알림 모듈로 옮겨 가며 이 파일로 남았다 — 토큰 소스 핸들러가 쓴다.
func resolveOrgID(c echo.Context) string {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = c.QueryParam("orgId")
	}
	if orgID == "" {
		orgID = "default-org"
	}
	return orgID
}

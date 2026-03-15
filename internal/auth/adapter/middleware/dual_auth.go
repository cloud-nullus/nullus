package middleware

import "github.com/labstack/echo/v4"

func DualAuthMiddleware(authMode string, sessionMW, oidcMW echo.MiddlewareFunc) echo.MiddlewareFunc {
	switch authMode {
	case "oidc":
		return oidcMW
	case "session":
		return sessionMW
	default:
		return sessionMW
	}
}

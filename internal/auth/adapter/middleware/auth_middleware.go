package middleware

import (
	"net/http"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/labstack/echo/v4"
)

const userContextKey = "current_user"

// AuthMiddleware checks the session for a user and stores it in the context.
// For the alpha/beta phase, this uses a simplified session-based approach.
func AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Retrieve user from session (simplified for alpha).
			// In production, this would validate a session cookie via gorilla/sessions.
			userID := c.Request().Header.Get("X-User-ID")
			if userID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
			}

			user := &admindomain.User{
				ID:       userID,
				Email:    c.Request().Header.Get("X-User-Email"),
				Name:     c.Request().Header.Get("X-User-Name"),
				Role:     admindomain.Role(c.Request().Header.Get("X-User-Role")),
				OrgID:    c.Request().Header.Get("X-User-OrgID"),
				IsActive: true,
			}

			c.Set(userContextKey, user)
			return next(c)
		}
	}
}

// RequireRole returns a middleware that checks if the current user has one of the required roles.
func RequireRole(roles ...admindomain.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get(userContextKey).(*admindomain.User)
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
			}

			if !user.CanAccess(roles...) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "insufficient permissions",
				})
			}

			return next(c)
		}
	}
}

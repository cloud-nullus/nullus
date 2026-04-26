package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
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

// roleAPIMatrix defines which URL path prefixes each role may access.
// Admin has full access; DevOps may access stacks, CI/CD, observability, and clusters;
// Developer may only access CI/CD and observability (read-only).
var roleAPIMatrix = map[admindomain.Role][]string{
	admindomain.RoleAdmin: {
		"/api/v1/",
	},
	admindomain.RoleDevOps: {
		"/api/v1/stacks",
		"/api/v1/compatibility",
		"/api/v1/pipelines",
		"/api/v1/templates",
		"/api/v1/dashboards",
		"/api/v1/alerts",
		"/api/v1/clusters",
	},
	admindomain.RoleDeveloper: {
		"/api/v1/pipelines",
		"/api/v1/dashboards",
		"/api/v1/alerts",
	},
}

// RBACMiddleware enforces the role-based API access matrix.
// Admin bypasses all checks. DevOps and Developer are limited to their allowed prefixes.
func RBACMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get(userContextKey).(*admindomain.User)
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
			}

			// Admin has unrestricted access.
			if user.Role == admindomain.RoleAdmin {
				return next(c)
			}

			path := c.Request().URL.Path
			allowedPrefixes, known := roleAPIMatrix[user.Role]
			if !known {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "insufficient permissions",
				})
			}

			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(path, prefix) {
					return next(c)
				}
			}

			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "insufficient permissions",
			})
		}
	}
}

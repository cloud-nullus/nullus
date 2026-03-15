package middleware

import (
	"net/http"
	"strings"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/labstack/echo/v4"
)

var (
	adminRouteRoles         = []admindomain.Role{admindomain.RoleAdmin}
	stackRouteRoles         = []admindomain.Role{admindomain.RoleAdmin, admindomain.RoleDevOps}
	cicdRouteRoles          = []admindomain.Role{admindomain.RoleAdmin, admindomain.RoleDevOps, admindomain.RoleDeveloper}
	observabilityRouteRoles = []admindomain.Role{admindomain.RoleAdmin, admindomain.RoleDevOps, admindomain.RoleDeveloper}
	alertConfigRoles        = []admindomain.Role{admindomain.RoleAdmin, admindomain.RoleDevOps}
	alertReadRoles          = []admindomain.Role{admindomain.RoleAdmin, admindomain.RoleDevOps, admindomain.RoleDeveloper}
)

func RequiredRolesForRoute(path, method string) []admindomain.Role {
	if strings.HasPrefix(path, "/api/v1/admin/") {
		return adminRouteRoles
	}
	if strings.HasPrefix(path, "/api/v1/stacks/") || path == "/api/v1/stacks" {
		return stackRouteRoles
	}
	if strings.HasPrefix(path, "/api/v1/cicd/") || path == "/api/v1/cicd" {
		return cicdRouteRoles
	}
	if strings.HasPrefix(path, "/api/v1/observability/") {
		if strings.Contains(path, "/alert-rules") {
			switch method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				return alertConfigRoles
			default:
				return alertReadRoles
			}
		}
		return observabilityRouteRoles
	}
	return adminRouteRoles
}

func RBACByRouteGroup() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get(userContextKey).(*admindomain.User)
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
			}

			requiredRoles := RequiredRolesForRoute(c.Request().URL.Path, c.Request().Method)
			if requiredRoles == nil {
				requiredRoles = adminRouteRoles
			}
			if len(requiredRoles) == 0 {
				return next(c)
			}

			if !user.CanAccess(requiredRoles...) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "insufficient permissions",
				})
			}

			return next(c)
		}
	}
}

package middleware

import (
	"context"
	"reflect"

	"github.com/labstack/echo/v4"
)

type orgIDContextKey struct{}

var requestOrgIDKey orgIDContextKey

func OrgContextMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if orgID := orgIDFromEchoContext(c); orgID != "" {
				ctx := context.WithValue(c.Request().Context(), requestOrgIDKey, orgID)
				c.SetRequest(c.Request().WithContext(ctx))
			}

			return next(c)
		}
	}
}

func OrgIDFromContext(ctx context.Context) (string, bool) {
	orgID, ok := ctx.Value(requestOrgIDKey).(string)
	if !ok || orgID == "" {
		return "", false
	}
	return orgID, true
}

func orgIDFromEchoContext(c echo.Context) string {
	if orgID := orgIDFromUser(c.Get("current_user")); orgID != "" {
		return orgID
	}
	return orgIDFromUser(c.Get("user"))
}

func orgIDFromUser(user any) string {
	if user == nil {
		return ""
	}

	switch u := user.(type) {
	case map[string]any:
		if orgID, ok := u["org_id"].(string); ok {
			return orgID
		}
		if orgID, ok := u["OrgID"].(string); ok {
			return orgID
		}
	case map[string]string:
		if orgID, ok := u["org_id"]; ok {
			return orgID
		}
		if orgID, ok := u["OrgID"]; ok {
			return orgID
		}
	}

	v := reflect.ValueOf(user)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return ""
	}

	field := v.FieldByName("OrgID")
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}

	return ""
}

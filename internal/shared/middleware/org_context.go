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

// UserEmailFromEchoContext 는 인증된 사용자의 이메일을 꺼낸다.
//
// 리플렉션으로 읽는 이유는 orgIDFromUser 와 같다 — 이 패키지는 인증 모듈의
// 사용자 타입을 알지 못하고, 알아서도 안 된다(모듈 간 직접 import 금지).
func UserEmailFromEchoContext(c echo.Context) string {
	if email := userEmail(c.Get("current_user")); email != "" {
		return email
	}
	return userEmail(c.Get("user"))
}

func userEmail(user any) string {
	if user == nil {
		return ""
	}

	switch u := user.(type) {
	case map[string]any:
		if email, ok := u["email"].(string); ok {
			return email
		}
	case map[string]string:
		if email, ok := u["email"]; ok {
			return email
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
	if field := v.FieldByName("Email"); field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

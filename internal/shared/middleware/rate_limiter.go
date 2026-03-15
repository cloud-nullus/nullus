package middleware

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type RateLimitConfig struct {
	Authenticated   int
	Unauthenticated int
	Login           int
	Deploy          int
}

type rateLimitCategory int

const (
	rateLimitCategoryGeneral rateLimitCategory = iota
	rateLimitCategoryLogin
	rateLimitCategoryDeploy
)

const (
	defaultAuthenticatedLimit   = 300
	defaultUnauthenticatedLimit = 30
	defaultLoginLimit           = 10
	defaultDeployLimit          = 10
	defaultCleanupInterval      = time.Minute
)

type rateLimitEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
	expiresAt  time.Time
}

type limiterState struct {
	entries sync.Map
	nowFn   func() time.Time
	window  time.Duration
}

func RateLimiter(cfg RateLimitConfig) echo.MiddlewareFunc {
	return newRateLimiter(rateLimitCategoryGeneral, cfg, time.Now, defaultCleanupInterval)
}

func DeployRateLimiter(cfg RateLimitConfig) echo.MiddlewareFunc {
	return newRateLimiter(rateLimitCategoryDeploy, cfg, time.Now, defaultCleanupInterval)
}

func LoginRateLimiter(cfg RateLimitConfig) echo.MiddlewareFunc {
	return newRateLimiter(rateLimitCategoryLogin, cfg, time.Now, defaultCleanupInterval)
}

func newRateLimiter(category rateLimitCategory, cfg RateLimitConfig, nowFn func() time.Time, cleanupInterval time.Duration) echo.MiddlewareFunc {
	window := windowForCategory(category)
	limit := limitForCategory(category, cfg)
	if nowFn == nil {
		nowFn = time.Now
	}
	if cleanupInterval <= 0 {
		cleanupInterval = defaultCleanupInterval
	}

	state := &limiterState{
		nowFn:  nowFn,
		window: window,
	}
	state.startCleanup(cleanupInterval)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key, requestLimit := keyAndLimitForCategory(c, category, cfg, limit)
			now := state.nowFn()
			allowed, remaining, resetAt, retryAfter := state.allow(key, requestLimit, now)

			setRateLimitHeaders(c, requestLimit, remaining, resetAt)
			if !allowed {
				c.Response().Header().Set(echo.HeaderRetryAfter, strconv.FormatInt(retryAfter, 10))
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"error": map[string]any{
						"code":        "RATE_LIMITED",
						"message":     fmt.Sprintf("rate limit exceeded, retry in %d seconds", retryAfter),
						"retry_after": retryAfter,
					},
				})
			}

			return next(c)
		}
	}
}

func (s *limiterState) allow(key string, limit int, now time.Time) (bool, int, time.Time, int64) {
	entryAny, _ := s.entries.LoadOrStore(key, &rateLimitEntry{})
	entry := entryAny.(*rateLimitEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	cutoff := now.Add(-s.window)
	filtered := entry.timestamps[:0]
	for _, ts := range entry.timestamps {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	entry.timestamps = filtered

	if len(entry.timestamps) >= limit {
		resetAt := entry.timestamps[0].Add(s.window)
		retryAfter := int64(resetAt.Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		entry.expiresAt = resetAt
		return false, 0, resetAt, retryAfter
	}

	entry.timestamps = append(entry.timestamps, now)
	remaining := limit - len(entry.timestamps)
	resetAt := entry.timestamps[0].Add(s.window)
	entry.expiresAt = resetAt
	return true, remaining, resetAt, 0
}

func (s *limiterState) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for tickTime := range ticker.C {
			s.entries.Range(func(key, value any) bool {
				entry, ok := value.(*rateLimitEntry)
				if !ok {
					s.entries.Delete(key)
					return true
				}

				entry.mu.Lock()
				cutoff := tickTime.Add(-s.window)
				filtered := entry.timestamps[:0]
				for _, ts := range entry.timestamps {
					if ts.After(cutoff) {
						filtered = append(filtered, ts)
					}
				}
				entry.timestamps = filtered
				shouldDelete := len(entry.timestamps) == 0 && !entry.expiresAt.After(tickTime)
				entry.mu.Unlock()

				if shouldDelete {
					s.entries.Delete(key)
				}
				return true
			})
		}
	}()
}

func setRateLimitHeaders(c echo.Context, limit, remaining int, resetAt time.Time) {
	headers := c.Response().Header()
	headers.Set("X-RateLimit-Limit", strconv.Itoa(limit))
	headers.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	headers.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
}

func windowForCategory(category rateLimitCategory) time.Duration {
	if category == rateLimitCategoryDeploy {
		return time.Hour
	}
	return time.Minute
}

func limitForCategory(category rateLimitCategory, cfg RateLimitConfig) int {
	switch category {
	case rateLimitCategoryLogin:
		if cfg.Login > 0 {
			return cfg.Login
		}
		return defaultLoginLimit
	case rateLimitCategoryDeploy:
		if cfg.Deploy > 0 {
			return cfg.Deploy
		}
		return defaultDeployLimit
	case rateLimitCategoryGeneral:
		return authLimit(cfg)
	}
	return defaultAuthenticatedLimit
}

func keyAndLimitForCategory(c echo.Context, category rateLimitCategory, cfg RateLimitConfig, fallbackLimit int) (string, int) {
	if category == rateLimitCategoryDeploy {
		if orgID := userField(c, "OrgID", "org_id"); orgID != "" {
			return "org:" + orgID, fallbackLimit
		}
		return "ip:" + c.RealIP(), fallbackLimit
	}

	if category == rateLimitCategoryLogin {
		return "ip:" + c.RealIP(), fallbackLimit
	}

	if userID := userField(c, "ID", "id"); userID != "" {
		return "user:" + userID, authLimit(cfg)
	}
	return "ip:" + c.RealIP(), unauthLimit(cfg)
}

func authLimit(cfg RateLimitConfig) int {
	if cfg.Authenticated > 0 {
		return cfg.Authenticated
	}
	return defaultAuthenticatedLimit
}

func unauthLimit(cfg RateLimitConfig) int {
	if cfg.Unauthenticated > 0 {
		return cfg.Unauthenticated
	}
	return defaultUnauthenticatedLimit
}

func userField(c echo.Context, fieldNames ...string) string {
	user := c.Get("user")
	if user == nil {
		user = c.Get("current_user")
	}
	if user == nil {
		return ""
	}

	v := reflect.ValueOf(user)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	for _, fieldName := range fieldNames {
		switch val := user.(type) {
		case map[string]any:
			if s := toString(val[fieldName]); s != "" {
				return s
			}
		case map[string]string:
			if s, ok := val[fieldName]; ok && s != "" {
				return s
			}
		}

		if v.Kind() == reflect.Struct {
			field := v.FieldByName(fieldName)
			if field.IsValid() && field.Kind() == reflect.String {
				if s := field.String(); s != "" {
					return s
				}
			}
		}
	}

	return ""
}

func toString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

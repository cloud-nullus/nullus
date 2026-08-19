package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/auth/domain"
	"github.com/cloud-nullus/draft/internal/auth/usecase"
)

type stubRepo struct {
	cred *domain.Credential
	err  error
}

func (s stubRepo) FindByEmail(context.Context, string) (*domain.Credential, error) {
	return s.cred, s.err
}

type stubIssuer struct{}

func (stubIssuer) Issue(domain.SessionClaims) (string, error) { return "signed-token", nil }

func postLogin(t *testing.T, repo stubRepo, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := NewLoginHandler(usecase.NewLogin(repo, stubIssuer{}))
	c := e.NewContext(req, rec)
	if err := h.Login(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

func credWithPassword(t *testing.T) *domain.Credential {
	t.Helper()
	hash, err := domain.HashPassword("correct-horse-battery")
	require.NoError(t, err)
	return &domain.Credential{
		UserID: "u-1", Email: "admin@nullus.dev", Name: "Admin",
		Role: "admin", OrgID: "org-1", PasswordHash: hash, IsActive: true,
	}
}

func TestLoginHandler_ReturnsTokenAndUser(t *testing.T) {
	rec := postLogin(t, stubRepo{cred: credWithPassword(t)},
		`{"email":"admin@nullus.dev","password":"correct-horse-battery"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "signed-token", body["token"])

	user, ok := body["user"].(map[string]any)
	require.True(t, ok, "user 가 응답에 없다")
	assert.Equal(t, "u-1", user["id"])
	assert.Equal(t, "admin", user["role"])
	assert.Equal(t, "org-1", user["orgId"])
}

// 해시가 응답에 실리면 오프라인 크래킹 대상이 된다.
func TestLoginHandler_NeverLeaksPasswordHash(t *testing.T) {
	rec := postLogin(t, stubRepo{cred: credWithPassword(t)},
		`{"email":"admin@nullus.dev","password":"correct-horse-battery"}`)

	assert.NotContains(t, rec.Body.String(), "$2a$", "bcrypt 해시가 응답에 노출됐다")
	assert.NotContains(t, strings.ToLower(rec.Body.String()), "passwordhash")
}

func TestLoginHandler_InvalidCredentialsReturn401(t *testing.T) {
	rec := postLogin(t, stubRepo{cred: credWithPassword(t)},
		`{"email":"admin@nullus.dev","password":"wrong"}`)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.NotContains(t, rec.Body.String(), "signed-token")
}

// 없는 계정과 틀린 비밀번호가 같은 응답이어야 이메일 존재 여부가 새지 않는다.
func TestLoginHandler_UnknownEmailLooksTheSameAsWrongPassword(t *testing.T) {
	unknown := postLogin(t, stubRepo{cred: nil}, `{"email":"nobody@nullus.dev","password":"x"}`)
	wrong := postLogin(t, stubRepo{cred: credWithPassword(t)}, `{"email":"admin@nullus.dev","password":"x"}`)

	assert.Equal(t, unknown.Code, wrong.Code)
	assert.JSONEq(t, unknown.Body.String(), wrong.Body.String())
}

func TestLoginHandler_MalformedBodyReturns400(t *testing.T) {
	rec := postLogin(t, stubRepo{cred: credWithPassword(t)}, `{"email":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// 저장소 장애는 401 이 아니라 500 이어야 한다 — 401 로 답하면 DB 가 죽었을 때
// 전 사용자가 "비밀번호가 틀렸다" 는 답을 받고 원인을 못 찾는다.
func TestLoginHandler_RepositoryFailureIsServerError(t *testing.T) {
	rec := postLogin(t, stubRepo{err: errors.New("db down")}, `{"email":"a@b.c","password":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

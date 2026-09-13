package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

type policyBundleFactory struct{ bundle *port.SCMBundle }

func (f *policyBundleFactory) For(context.Context, string) (*port.SCMBundle, error) {
	return f.bundle, nil
}

type okPublisher struct{ apps []string }

func (p *okPublisher) PublishScanPolicy(_ context.Context, apps []string, _ []port.ProjectVariable) map[string]error {
	p.apps = append(p.apps, apps...)
	return map[string]error{}
}

func newScanPolicyEcho(t *testing.T, pipelines ...*domain.Pipeline) (*echo.Echo, *okPublisher) {
	t.Helper()
	pub := &okPublisher{}
	svc := usecase.NewScanPolicyService(
		repository.NewMemoryScanPolicyRepository(),
		newMockPipelineRepository(pipelines...),
		&policyBundleFactory{bundle: &port.SCMBundle{ScanPolicy: pub}},
	)
	e := echo.New()
	cicdhandler.NewScanPolicyHandler(svc).RegisterStackRoutes(e.Group("/api/v1/stacks"))
	return e, pub
}

type policyResponse struct {
	StackID string `json:"stack_id"`
	Policy  struct {
		BlockSeverity        string `json:"block_severity"`
		IgnoreUnfixed        bool   `json:"ignore_unfixed"`
		OnScannerUnreachable string `json:"on_scanner_unreachable"`
	} `json:"policy"`
	IsDefault bool `json:"is_default"`
	Pushes    []struct {
		PipelineName string `json:"pipeline_name"`
		Status       string `json:"status"`
	} `json:"pushes"`
}

func doPolicyRequest(t *testing.T, e *echo.Echo, method, body string) (*httptest.ResponseRecorder, policyResponse) {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/stacks/stk_1/image-scan-policy", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var resp policyResponse
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	}
	return rec, resp
}

// 저장한 적 없는 스택은 기본 정책이고, 그것이 기본값이라는 사실도 내려준다.
func TestScanPolicyHandler_GetReturnsDefault(t *testing.T) {
	e, _ := newScanPolicyEcho(t)

	rec, resp := doPolicyRequest(t, e, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "stk_1", resp.StackID)
	assert.True(t, resp.IsDefault)
	assert.Equal(t, "CRITICAL", resp.Policy.BlockSeverity)
	assert.True(t, resp.Policy.IgnoreUnfixed)
	assert.Equal(t, "block", resp.Policy.OnScannerUnreachable)
}

// 저장하면 그 스택의 스캔 파이프라인에 싣고, 파이프라인별 결과를 돌려준다.
func TestScanPolicyHandler_PutStoresAndPushes(t *testing.T) {
	e, pub := newScanPolicyEcho(t, &domain.Pipeline{
		ID: "p1", Name: "shop", StackID: "stk_1", Stages: []string{"Build", "ImageScan", "Deploy"},
	})

	rec, resp := doPolicyRequest(t, e, http.MethodPut,
		`{"block_severity":"HIGH","ignore_unfixed":false,"on_scanner_unreachable":"allow"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, resp.IsDefault)
	assert.Equal(t, "HIGH", resp.Policy.BlockSeverity)
	require.Len(t, resp.Pushes, 1)
	assert.Equal(t, "applied", resp.Pushes[0].Status)
	assert.Equal(t, []string{"shop"}, pub.apps)

	_, after := doPolicyRequest(t, e, http.MethodGet, "")
	assert.False(t, after.IsDefault)
	assert.Equal(t, "HIGH", after.Policy.BlockSeverity)
	assert.Equal(t, "allow", after.Policy.OnScannerUnreachable)
}

func TestScanPolicyHandler_PutRejectsInvalidSeverity(t *testing.T) {
	e, _ := newScanPolicyEcho(t)

	rec, _ := doPolicyRequest(t, e, http.MethodPut,
		`{"block_severity":"HIGHT","ignore_unfixed":true,"on_scanner_unreachable":"block"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_SCAN_POLICY")
}

// 필드를 빠뜨리면 0 값으로 받지 않는다. ignore_unfixed 누락을 false 로 받으면
// 수정본 없는 CVE 가 차단 사유가 되어 운영자가 모르는 사이 배포가 막힌다.
func TestScanPolicyHandler_PutRequiresAllFields(t *testing.T) {
	e, _ := newScanPolicyEcho(t)

	rec, _ := doPolicyRequest(t, e, http.MethodPut, `{"block_severity":"HIGH"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	_, after := doPolicyRequest(t, e, http.MethodGet, "")
	assert.True(t, after.IsDefault, "잘못된 요청이 정책을 바꾸면 안 된다")
}

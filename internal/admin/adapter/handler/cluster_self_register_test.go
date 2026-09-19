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

	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

// 셀프 등록은 kubeconfig 만 저장하고 노드를 탐색하지 않았다. node_architectures 가 비어
// Pre-Deploy Gate 는 CLUSTER_ARCH_UNKNOWN warn 만 냈고, 에어갭 29 스크립트는 warn 에
// 동의하고 진행해 DGX Spark(arm64)에서 Harbor 가 installing_harbor 에서 멈췄다(#270).
type archDiscoverer struct {
	archs []string
	err   error
	calls int
}

func (d *archDiscoverer) Discover(_ context.Context, _ []byte) (*domain.ClusterDiscoveryInfo, error) {
	d.calls++
	if d.err != nil {
		return nil, d.err
	}
	return &domain.ClusterDiscoveryInfo{NodeArchitectures: d.archs}, nil
}

func newSelfRegisterEcho(t *testing.T, discoverer *archDiscoverer) (*echo.Echo, *adminrepo.MemoryClusterRepository) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("k", 32))
	original := inClusterKubeconfigFn
	inClusterKubeconfigFn = func() ([]byte, error) {
		return []byte("apiVersion: v1\nkind: Config\nclusters:\n- cluster:\n    server: https://kubernetes.default.svc\n  name: self\n"), nil
	}
	t.Cleanup(func() { inClusterKubeconfigFn = original })

	repo := adminrepo.NewMemoryClusterRepository()
	h := NewClusterHandler(usecase.NewClusterUseCase(repo, usecase.WithDiscoverer(discoverer)), nil)

	e := echo.New()
	e.HTTPErrorHandler = middleware.AppErrorHandler
	h.RegisterRoutes(e.Group("/api/v1").Group("/admin"))
	return e, repo
}

func postSelfRegister(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters/self-register", strings.NewReader(`{"org_id":"org-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestSelfRegisterCluster_DiscoversNodeArchitectures(t *testing.T) {
	discoverer := &archDiscoverer{archs: []string{"arm64"}}
	e, repo := newSelfRegisterEcho(t, discoverer)

	rec := postSelfRegister(e)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []any{"arm64"}, resp["node_architectures"])

	stored, err := repo.GetByID(context.Background(), resp["id"].(string))
	require.NoError(t, err)
	assert.Equal(t, []string{"arm64"}, stored.NodeArchitectures, "게이트는 저장된 아키텍처를 본다")
}

// 탐색이 실패해도 등록은 성공한다 — 업로드 경로와 같은 정책이다. 설치 직전 사전검사가
// 노드를 직접 읽으므로 여기서 막을 이유는 없다.
func TestSelfRegisterCluster_DiscoveryFailureDoesNotFailRegistration(t *testing.T) {
	e, _ := newSelfRegisterEcho(t, &archDiscoverer{err: errors.New("forbidden: nodes")})

	rec := postSelfRegister(e)
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

// 이 수정 전에 등록된 셀프 클러스터는 아키텍처가 비어 있다. 29 를 다시 돌리면 멱등 경로를
// 타므로 거기서도 채워야 한다.
func TestSelfRegisterCluster_ExistingClusterWithoutArchitecturesIsRediscovered(t *testing.T) {
	discoverer := &archDiscoverer{err: errors.New("temporarily unreachable")}
	e, repo := newSelfRegisterEcho(t, discoverer)
	require.Equal(t, http.StatusCreated, postSelfRegister(e).Code)

	discoverer.err = nil
	discoverer.archs = []string{"arm64"}
	rec := postSelfRegister(e)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []any{"arm64"}, resp["node_architectures"])
	stored, err := repo.GetByID(context.Background(), resp["id"].(string))
	require.NoError(t, err)
	assert.Equal(t, []string{"arm64"}, stored.NodeArchitectures)
}

// 이미 아키텍처를 아는 셀프 클러스터는 다시 탐색하지 않는다 — 재실행마다 노드를 긁을 이유가 없다.
func TestSelfRegisterCluster_ExistingClusterWithArchitecturesIsNotRediscovered(t *testing.T) {
	discoverer := &archDiscoverer{archs: []string{"arm64"}}
	e, _ := newSelfRegisterEcho(t, discoverer)
	require.Equal(t, http.StatusCreated, postSelfRegister(e).Code)
	require.Equal(t, 1, discoverer.calls)

	require.Equal(t, http.StatusOK, postSelfRegister(e).Code)
	assert.Equal(t, 1, discoverer.calls)
}

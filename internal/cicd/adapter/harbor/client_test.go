package harbor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func harborTarget(repository string) *port.ImageTarget {
	return &port.ImageTarget{
		Kind:       port.RegistryKindHarbor,
		Host:       "harbor.nullus.io",
		Repository: repository,
	}
}

// 플랫폼은 Harbor 프로젝트를 만들고 이미지를 밀어 넣으면서 정리는 못 했다.
// 파이프라인을 지워도 이미지가 계속 쌓이고, 디스크는 아무도 안 보는 사이에 찬다.
func TestDeleteImageRepository_DeletesProjectRepository(t *testing.T) {
	var gotMethod, gotPath, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotUser, _, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), harborTarget("harbor.nullus.io/nullus/sample-frontend"))

	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/api/v2.0/projects/nullus/repositories/sample-frontend", gotPath)
	assert.Equal(t, "admin", gotUser)
}

// 저장소 이름에 슬래시가 들어갈 수 있다(nullus/team/app). Harbor 는 그것을
// 경로가 아니라 이름으로 받으므로 인코딩해야 한다 — 그러지 않으면 404 가 난다.
func TestDeleteImageRepository_EncodesNestedRepositoryName(t *testing.T) {
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	require.NoError(t, NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), harborTarget("harbor.nullus.io/nullus/team/app")))

	assert.Equal(t, "/api/v2.0/projects/nullus/repositories/team%2Fapp", gotRaw)
}

// 이미 없으면 성공으로 본다. 삭제의 목표는 "없는 상태" 이고, 404 를 오류로
// 올리면 재시도가 영영 끝나지 않는다.
func TestDeleteImageRepository_TreatsMissingAsDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	assert.NoError(t, NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), harborTarget("harbor.nullus.io/nullus/gone")))
}

// 다른 레지스트리의 대상을 받으면 지우지 않는다. 조용히 성공으로 넘기면
// 사용자는 이미지가 사라진 줄 안다.
func TestDeleteImageRepository_RejectsForeignTarget(t *testing.T) {
	target := harborTarget("registry.example/acme/app")
	target.Kind = port.RegistryKindNexus

	err := NewClient("http://harbor.local", "admin", "pw").
		DeleteImageRepository(context.Background(), target)

	assert.ErrorIs(t, err, port.ErrImageDeletionUnsupported)
}

// 프로젝트를 못 가려내면 지우지 않는다. 잘못 가려내면 남의 저장소를 지운다.
func TestDeleteImageRepository_RequiresProjectAndRepository(t *testing.T) {
	client := NewClient("http://harbor.local", "admin", "pw")

	assert.Error(t, client.DeleteImageRepository(context.Background(), harborTarget("harbor.nullus.io")))
	assert.Error(t, client.DeleteImageRepository(context.Background(), harborTarget("harbor.nullus.io/onlyproject")))
}

// 자격증명이 없으면 Harbor 는 401 을 돌려준다. 그 사실을 그대로 올린다.
func TestDeleteImageRepository_ReportsAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "admin", "wrong").
		DeleteImageRepository(context.Background(), harborTarget("harbor.nullus.io/nullus/app"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

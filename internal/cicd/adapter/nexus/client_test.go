package nexus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func nexusTarget(repository string) *port.ImageTarget {
	return &port.ImageTarget{
		Kind:       port.RegistryKindNexus,
		Host:       "registry.nullus.io",
		Repository: repository,
	}
}

// Nexus 에는 "이미지 저장소를 통째로 지운다" 는 단일 API 가 없다. 이름으로 찾은
// 컴포넌트를 하나씩 지운다 — 태그마다 컴포넌트가 하나씩 생기므로 여러 개다.
func TestDeleteImageRepository_DeletesEveryComponent(t *testing.T) {
	var mu sync.Mutex
	deleted := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/search"):
			assert.Equal(t, "docker-hosted", r.URL.Query().Get("repository"))
			assert.Equal(t, "sample-frontend", r.URL.Query().Get("name"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": "c1"}, {"id": "c2"}},
			})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/components/"):
			mu.Lock()
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/service/rest/v1/components/"))
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), nexusTarget("registry.nullus.io/sample-frontend"))

	require.NoError(t, err)
	sort.Strings(deleted)
	assert.Equal(t, []string{"c1", "c2"}, deleted)
}

// 검색 결과는 쪽으로 나뉜다. 첫 쪽만 지우면 나머지 태그가 조용히 남는다.
func TestDeleteImageRepository_FollowsContinuationToken(t *testing.T) {
	var mu sync.Mutex
	deleted := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/search"):
			if r.URL.Query().Get("continuationToken") == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items":             []map[string]any{{"id": "c1"}},
					"continuationToken": "next",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": "c2"}},
			})
		case r.Method == http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/service/rest/v1/components/"))
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	require.NoError(t, NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), nexusTarget("registry.nullus.io/app")))

	sort.Strings(deleted)
	assert.Equal(t, []string{"c1", "c2"}, deleted)
}

// 이미 아무것도 없으면 성공이다. 삭제의 목표는 "없는 상태" 다.
func TestDeleteImageRepository_TreatsEmptyAsDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}})
	}))
	defer srv.Close()

	assert.NoError(t, NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), nexusTarget("registry.nullus.io/gone")))
}

// 다른 레지스트리의 대상은 지우지 않는다. 조용히 성공으로 넘기면 사용자는
// 이미지가 사라진 줄 안다.
func TestDeleteImageRepository_RejectsForeignTarget(t *testing.T) {
	target := nexusTarget("harbor.nullus.io/nullus/app")
	target.Kind = port.RegistryKindHarbor

	err := NewClient("http://nexus.local", "admin", "pw").
		DeleteImageRepository(context.Background(), target)

	assert.ErrorIs(t, err, port.ErrImageDeletionUnsupported)
}

// 이름을 가려내지 못하면 지우지 않는다. 빈 이름으로 검색하면 저장소의 모든
// 컴포넌트가 걸려 남의 이미지까지 지운다.
func TestDeleteImageRepository_RequiresImageName(t *testing.T) {
	err := NewClient("http://nexus.local", "admin", "pw").
		DeleteImageRepository(context.Background(), nexusTarget("registry.nullus.io"))

	assert.Error(t, err)
}

// 컴포넌트 하나가 실패하면 나머지도 시도하고, 실패 사실은 올린다. 첫 실패에서
// 멈추면 절반만 지워진 채 성공도 실패도 아닌 상태가 된다.
func TestDeleteImageRepository_ReportsFailureAfterTryingAll(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": "c1"}, {"id": "c2"}},
			})
			return
		}
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "admin", "pw").
		DeleteImageRepository(context.Background(), nexusTarget("registry.nullus.io/app"))

	require.Error(t, err)
	assert.Equal(t, 2, attempts, "하나가 실패해도 나머지를 시도한다")
}

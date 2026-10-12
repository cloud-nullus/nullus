package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ListKeys 가 404 외의 오류까지 "하위가 없다" 로 삼키면, 읽지 못한 서브트리가
// 통째로 백업에서 빠진다. 그 사실은 복구할 때에야 드러난다 —
// 이 설계가 가장 경계하는 실패 형태다.

func newTestStore(t *testing.T, handler http.HandlerFunc) *OpenBaoStore {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewOpenBaoStore(srv.URL, "test-token")
}

func TestListKeys_404_는_하위가_없다는_뜻이다(t *testing.T) {
	s := newTestStore(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	keys, err := s.ListKeys(context.Background(), "kv/nullus/dev")
	require.NoError(t, err, "404 는 오류가 아니라 잎 경로다")
	assert.Empty(t, keys)
}

func TestListKeys_403_은_오류로_올린다(t *testing.T) {
	// 권한이 막힌 서브트리를 "비었다" 로 넘기면 백업에서 조용히 빠진다.
	s := newTestStore(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := s.ListKeys(context.Background(), "kv/nullus/dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestListKeys_500_도_오류로_올린다(t *testing.T) {
	s := newTestStore(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := s.ListKeys(context.Background(), "kv/nullus/dev")
	require.Error(t, err)
}

func TestListKeys_정상_응답(t *testing.T) {
	s := newTestStore(t, func(w http.ResponseWriter, r *http.Request) {
		// HTTP LIST 가 아니라 GET ...?list=true 여야 한다.
		// 운영 경로(API server proxy)는 GET/POST 만 지원하므로, LIST 를 쓰면
		// 로컬에서는 되고 운영에서만 죽는다.
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/metadata/")
		assert.Equal(t, "true", r.URL.Query().Get("list"))
		_, _ = w.Write([]byte(`{"data":{"keys":["a","sub/"]}}`))
	})
	keys, err := s.ListKeys(context.Background(), "kv/nullus/dev")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "sub/"}, keys)
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, IsNotFound(&StatusError{Status: 404}))
	assert.False(t, IsNotFound(&StatusError{Status: 403}))
	assert.False(t, IsNotFound(errors.New("plain")))
	assert.False(t, IsNotFound(nil))
}

func TestStatusError_메시지_형식을_유지한다(t *testing.T) {
	// 기존 호출자들이 이 문자열을 로그로 남긴다. 형식을 바꾸면 운영 검색이 깨진다.
	e := &StatusError{Status: 403, Body: "permission denied"}
	assert.Equal(t, "openbao request failed: status=403 body=permission denied", e.Error())
}

// 운영 경로(API server proxy transport)가 GET/POST 만 지원하는데 ListKeys 가
// LIST 를 쓰고 있었다. 로컬 직결에서는 통과하고 운영에서만 죽는 부류라,
// transport 가 무엇을 받는지 직접 고정한다.
func TestListKeys_운영_transport_가_지원하는_메서드만_쓴다(t *testing.T) {
	var seen []string
	s := newTestStore(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method)
		_, _ = w.Write([]byte(`{"data":{"keys":[]}}`))
	})
	_, err := s.ListKeys(context.Background(), "kv/nullus/dev")
	require.NoError(t, err)

	for _, m := range seen {
		assert.Contains(t, []string{http.MethodGet, http.MethodPost}, m,
			"API server proxy transport 는 GET/POST 만 지원한다 (apiserver_proxy.go:65)")
	}
}

// Suffix() 는 인자를 경로 조각으로 다룬다. 쿼리가 붙은 채로 넘어가면 통째로
// 이스케이프돼 프록시 URL 이 깨진다 — KV 목록 조회가 실제로 그렇게 죽었다.
func TestSplitPathQuery(t *testing.T) {
	for _, tc := range []struct {
		in     string
		suffix string
		query  string
	}{
		{"/v1/kv/nullus/metadata/dev?list=true", "v1/kv/nullus/metadata/dev", "true"},
		{"v1/kv/nullus/metadata/dev?list=true", "v1/kv/nullus/metadata/dev", "true"},
		{"/v1/kv/nullus/data/dev", "v1/kv/nullus/data/dev", ""},
		{"/v1/sys/seal-status", "v1/sys/seal-status", ""},
	} {
		suffix, q := splitPathQuery(tc.in)
		assert.Equal(t, tc.suffix, suffix, "in=%s", tc.in)
		assert.Equal(t, tc.query, q.Get("list"), "in=%s", tc.in)
	}
}

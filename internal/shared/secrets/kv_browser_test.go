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
		assert.Equal(t, "LIST", r.Method)
		assert.Contains(t, r.URL.Path, "/metadata/")
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

package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeKV 는 KV v2 트리를 흉내 낸다. 키는 전체 경로다.
type fakeKV struct {
	leaves  map[string]bool
	deleted []string
	failOn  string
}

func (f *fakeKV) PutToken(context.Context, string, string) error   { return nil }
func (f *fakeKV) GetToken(context.Context, string) (string, error) { return "", nil }

func (f *fakeKV) ListKeys(_ context.Context, path string) ([]string, error) {
	prefix := strings.TrimSuffix(path, "/") + "/"
	seen := map[string]bool{}
	for leaf := range f.leaves {
		if !strings.HasPrefix(leaf, prefix) {
			continue
		}
		rest := strings.TrimPrefix(leaf, prefix)
		if i := strings.Index(rest, "/"); i >= 0 {
			seen[rest[:i+1]] = true
		} else {
			seen[rest] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (f *fakeKV) DeleteSecret(_ context.Context, path string) error {
	if path == f.failOn {
		return errors.New("boom")
	}
	delete(f.leaves, path)
	f.deleted = append(f.deleted, path)
	return nil
}

// 파이프라인을 지울 때 그 파이프라인 몫의 자격증명(…/cicd/pipelines/<앱>/*)을 모두 회수한다.
// 하위 디렉터리가 있어도 빠짐없이, 다른 앱의 값은 건드리지 않는다. (운영 정책은 앱 아래 한
// 단계만 지우게 두지만, 트리 삭제 자체는 깊이를 가리지 않는다.)
func TestRouter_DeleteTreeForStack_RemovesEverythingUnderPrefix(t *testing.T) {
	kv := &fakeKV{leaves: map[string]bool{
		"kv/nullus/dev/org/cicd/pipelines/api/harbor_password":  true,
		"kv/nullus/dev/org/cicd/pipelines/api/harbor_username":  true,
		"kv/nullus/dev/org/cicd/pipelines/api/nested/token":     true,
		"kv/nullus/dev/org/cicd/pipelines/api2/harbor_password": true,
	}}
	r := NewRouter()
	r.Register("openbao", kv)

	require.NoError(t, r.DeleteTreeForStack(context.Background(), "openbao", "", "kv/nullus/dev/org/cicd/pipelines/api/"))

	assert.ElementsMatch(t, []string{
		"kv/nullus/dev/org/cicd/pipelines/api/harbor_password",
		"kv/nullus/dev/org/cicd/pipelines/api/harbor_username",
		"kv/nullus/dev/org/cicd/pipelines/api/nested/token",
	}, kv.deleted)
	assert.True(t, kv.leaves["kv/nullus/dev/org/cicd/pipelines/api2/harbor_password"], "이름이 비슷한 다른 앱은 남는다")
}

// 이미 없으면 성공이다 — 다시 시도한 삭제가 같은 자리에서 막히면 안 된다.
func TestRouter_DeleteTreeForStack_EmptyIsSuccess(t *testing.T) {
	r := NewRouter()
	r.Register("openbao", &fakeKV{leaves: map[string]bool{}})

	assert.NoError(t, r.DeleteTreeForStack(context.Background(), "openbao", "", "kv/nullus/dev/org/cicd/pipelines/api/"))
}

// 하나를 못 지워도 나머지는 지우고, 못 지운 것을 알린다.
func TestRouter_DeleteTreeForStack_ContinuesPastFailure(t *testing.T) {
	kv := &fakeKV{leaves: map[string]bool{
		"kv/x/cicd/pipelines/api/a": true,
		"kv/x/cicd/pipelines/api/b": true,
	}, failOn: "kv/x/cicd/pipelines/api/a"}
	r := NewRouter()
	r.Register("openbao", kv)

	err := r.DeleteTreeForStack(context.Background(), "openbao", "", "kv/x/cicd/pipelines/api/")
	require.Error(t, err)
	assert.Equal(t, []string{"kv/x/cicd/pipelines/api/b"}, kv.deleted)
}

// 너무 넓은 접두사는 거부한다 — 실수로 금고를 통째로 비우지 않게.
func TestRouter_DeleteTreeForStack_RefusesShallowPrefix(t *testing.T) {
	kv := &fakeKV{leaves: map[string]bool{"kv/nullus/a": true}}
	r := NewRouter()
	r.Register("openbao", kv)

	assert.Error(t, r.DeleteTreeForStack(context.Background(), "openbao", "", "kv/"))
	assert.Error(t, r.DeleteTreeForStack(context.Background(), "openbao", "", "kv/nullus"))
	assert.Empty(t, kv.deleted)
}

// KV v2 에서 값을 완전히 지우려면 metadata 를 지워야 한다(data 삭제는 버전만 지운다).
func TestOpenBaoStore_DeleteSecret_DeletesMetadata(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/gone") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	store := NewOpenBaoStore(srv.URL, "root")

	require.NoError(t, store.DeleteSecret(context.Background(), "kv/nullus/dev/org/cicd/pipelines/api/harbor_password"))
	require.NoError(t, store.DeleteSecret(context.Background(), "kv/nullus/dev/org/cicd/pipelines/api/gone"), "없으면 성공")
	assert.Equal(t, []string{
		"DELETE /v1/kv/metadata/nullus/dev/org/cicd/pipelines/api/harbor_password",
		"DELETE /v1/kv/metadata/nullus/dev/org/cicd/pipelines/api/gone",
	}, got)
}

// API server proxy 를 거친 오류는 client-go 의 StatusError 로 온다. 그대로 두면 금고가 돌려준
// 상태 코드(403 · 404)를 가릴 수 없어 "unknown (delete services http:openbao:8200)" 만 보인다.
func TestAsStoreError_KeepsVaultStatusFromProxy(t *testing.T) {
	forbidden := apierrors.NewGenericServerResponse(http.StatusForbidden, "DELETE",
		schema.GroupResource{Resource: "services"}, "http:openbao:8200", "", 0, true)

	_, err := asStoreError([]byte(`{"errors":["permission denied"]}`), forbidden)

	require.Error(t, err)
	assert.True(t, IsForbidden(err))
	assert.Contains(t, err.Error(), "permission denied")

	notFound := apierrors.NewGenericServerResponse(http.StatusNotFound, "GET",
		schema.GroupResource{Resource: "services"}, "http:openbao:8200", "", 0, true)
	_, err = asStoreError(nil, notFound)
	assert.True(t, IsNotFound(err))

	raw, err := asStoreError([]byte("ok"), nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(raw))
}

// "." · ".." · 빈 세그먼트가 든 접두사는 거부한다. client-go 가 경로를 정리해 다른 곳을 지운다.
func TestRouter_DeleteTreeForStack_RejectsTraversalPrefix(t *testing.T) {
	kv := &fakeKV{leaves: map[string]bool{"kv/nullus/dev/org/cicd/pipelines/other/k": true}}
	r := NewRouter()
	r.Register("openbao", kv)

	for _, prefix := range []string{
		"kv/nullus/dev/org/cicd/pipelines/../pipelines/other/",
		"kv/nullus/dev/org/cicd/pipelines/./other/",
		"kv/nullus/dev/org/cicd//pipelines/other/",
	} {
		assert.Error(t, r.DeleteTreeForStack(context.Background(), "openbao", "", prefix), prefix)
	}
	assert.Empty(t, kv.deleted)
}

// 금고가 돌려준 키 이름도 한 세그먼트여야 한다 — 이상한 이름은 건너뛰고 알린다.
func TestRouter_DeleteTreeForStack_SkipsSuspiciousKeys(t *testing.T) {
	kv := &suspiciousKV{fakeKV: fakeKV{leaves: map[string]bool{"kv/x/cicd/pipelines/api/ok": true}}}
	r := NewRouter()
	r.Register("openbao", kv)

	err := r.DeleteTreeForStack(context.Background(), "openbao", "", "kv/x/cicd/pipelines/api/")
	require.Error(t, err)
	assert.Equal(t, []string{"kv/x/cicd/pipelines/api/ok"}, kv.deleted)
}

type suspiciousKV struct{ fakeKV }

func (s *suspiciousKV) ListKeys(ctx context.Context, path string) ([]string, error) {
	keys, err := s.fakeKV.ListKeys(ctx, path)
	return append(keys, "..", "../"), err
}

// API server 자체의 오류(metav1.Status 본문)는 금고 오류로 바꾸지 않는다 — openbao Service 가
// 사라진 404 를 "값이 없다" 로, RBAC 403 을 "금고 정책" 으로 읽게 된다.
func TestAsStoreError_LeavesAPIServerStatusAlone(t *testing.T) {
	apiErr := apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, "openbao")
	body := []byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`)

	_, err := asStoreError(body, apiErr)

	require.Error(t, err)
	assert.False(t, IsNotFound(err))
	assert.True(t, apierrors.IsNotFound(err))
}

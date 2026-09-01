package openbao

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

// fakeVault 는 KV 트리를 흉내낸다. 키는 전체 경로다.
type fakeVault struct {
	data map[string]map[string]any
}

func (f *fakeVault) ListKeys(_ context.Context, path string) ([]string, error) {
	prefix := strings.TrimSuffix(path, "/") + "/"
	seen := map[string]bool{}
	var out []string
	for full := range f.data {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		rest := strings.TrimPrefix(full, prefix)
		if i := strings.Index(rest, "/"); i >= 0 {
			dir := rest[:i+1]
			if !seen[dir] {
				seen[dir] = true
				out = append(out, dir)
			}
			continue
		}
		if !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out, nil
}

func (f *fakeVault) GetSecret(_ context.Context, path string) (map[string]any, error) {
	return f.data[path], nil
}

func (f *fakeVault) PutSecret(_ context.Context, path string, data map[string]any) error {
	f.data[path] = data
	return nil
}

type fakeResolver struct{ v *fakeVault }

func (r *fakeResolver) ForStack(context.Context, string, string) (secrets.Store, error) {
	return storeAdapter{r.v}, nil
}

// storeAdapter 는 fakeVault 를 secrets.Store 로 보이게 한다.
type storeAdapter struct{ *fakeVault }

func (storeAdapter) PutToken(context.Context, string, string) error   { return nil }
func (storeAdapter) GetToken(context.Context, string) (string, error) { return "", nil }

func newFixture() (*KVExporter, *fakeVault) {
	v := &fakeVault{data: map[string]map[string]any{
		"kv/nullus/dev/org-1/cicd/github/api-token": {"token": "gh-1"},
		"kv/nullus/dev/org-1/cicd/gitlab/api-token": {"token": "gl-1"},
		"kv/nullus/dev/org-2/stack/harbor/admin":    {"username": "admin", "password": "p"},
	}}
	return NewKVExporter(&fakeResolver{v: v}), v
}

func TestExport_트리를_전부_훑는다(t *testing.T) {
	e, _ := newFixture()
	var buf bytes.Buffer

	res, err := e.Export(context.Background(), "stack-1", &buf)
	require.NoError(t, err)

	assert.Equal(t, 3, res.PathCount, "중첩된 경로까지 재귀로 모은다")
	assert.Contains(t, buf.String(), "kv/nullus/dev/org-1/cicd/github/api-token")
}

func TestExportImport_왕복(t *testing.T) {
	e, src := newFixture()
	var buf bytes.Buffer
	_, err := e.Export(context.Background(), "stack-1", &buf)
	require.NoError(t, err)

	// 빈 금고에 복원한다.
	dst := &fakeVault{data: map[string]map[string]any{}}
	e2 := NewKVExporter(&fakeResolver{v: dst})
	require.NoError(t, e2.Import(context.Background(), "stack-1", &buf))

	assert.Equal(t, src.data, dst.data, "값이 그대로 복원돼야 한다")
}

func TestImport_형식_버전이_다르면_거부한다(t *testing.T) {
	// 옛 백업본을 새 코드가 잘못 해석해 조용히 망가뜨리는 것을 막는다.
	e, _ := newFixture()
	err := e.Import(context.Background(), "s", strings.NewReader(`{"version":99,"paths":{}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 99")
}

func TestImport_깨진_JSON(t *testing.T) {
	e, _ := newFixture()
	require.Error(t, e.Import(context.Background(), "s", strings.NewReader("not json")))
}

func TestPathExists(t *testing.T) {
	e, _ := newFixture()
	ok, err := e.PathExists(context.Background(), "s", "kv/nullus/dev/org-1/cicd/github/api-token")
	require.NoError(t, err)
	assert.True(t, ok)

	// 없는 경로는 false — 참조 정합성 검사가 이것으로 dangling 을 찾는다.
	ok, err = e.PathExists(context.Background(), "s", "kv/nullus/dev/org-1/없음")
	require.NoError(t, err)
	assert.False(t, ok)
}

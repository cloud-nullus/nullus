package openbao

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

// fakeVault 는 KV 트리를 흉내낸다. 키는 전체 경로다.
type fakeVault struct {
	putErr error
	data   map[string]map[string]any
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
	if f.putErr != nil {
		return f.putErr
	}
	f.data[path] = data
	return nil
}

// putErr 가 있으면 모든 PutSecret 이 그 오류로 실패한다 — 금고가 안 뜬·권한 없는·잘못 라우팅된 상황.
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

// 실패한 경로 목록만 남기면 "금고가 아직 안 떴다" 와 "권한이 없다" 와 "주소가 다른 금고를 가리킨다" 를
// 가를 수 없다 — 2026-10-12 두 스택 리허설에서 한 스택의 import 가 25 경로 실패로만 남아 원인을 로그로
// 알 수 없었다. 첫 오류의 원인을 함께 남긴다.
func TestImport_실패_원인을_남긴다(t *testing.T) {
	e, src := newFixture()
	var buf bytes.Buffer
	_, err := e.Export(context.Background(), "stack-1", &buf)
	require.NoError(t, err)
	_ = src

	dst := &fakeVault{data: map[string]map[string]any{}, putErr: errors.New("dial tcp 10.96.0.1:8200: connection refused")}
	err = NewKVExporter(&fakeResolver{v: dst}).Import(context.Background(), "stack-1", &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "복원하지 못했습니다")
	assert.Contains(t, err.Error(), "connection refused", "원인이 함께 남아야 한다")
}

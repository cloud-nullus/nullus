package registrycreds

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	values map[string]string
	err    error
}

func (f *fakeStore) GetTokenForStack(_ context.Context, _, _, path string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.values[path], nil
}

func resolver(store SecretStore) *Resolver {
	return New(store, "dev", "org-1", "stk-1")
}

// 스택이 설치한 Harbor 의 자격증명은 플랫폼이 이미 갖고 있다. 사용자에게 다시
// 받아 적게 할 이유가 없다 — 화면은 그 값을 묻지도 않아서, 받지 못하면
// HARBOR_USERNAME/HARBOR_PASSWORD 가 비고 CI 의 docker login 이 죽는다.
func TestResolve_HarborFromSecretStore(t *testing.T) {
	store := &fakeStore{values: map[string]string{
		"kv/nullus/dev/org-1/artifacts/harbor/admin-password": "harbor-secret",
	}}

	got, err := resolver(store).Resolve(context.Background(),
		[]string{"HARBOR_USERNAME", "HARBOR_PASSWORD"})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"HARBOR_USERNAME": "admin",
		"HARBOR_PASSWORD": "harbor-secret",
	}, got)
}

func TestResolve_NexusFromSecretStore(t *testing.T) {
	store := &fakeStore{values: map[string]string{
		"kv/nullus/dev/org-1/artifacts/nexus/admin-password": "nexus-secret",
	}}

	got, err := resolver(store).Resolve(context.Background(),
		[]string{"NEXUS_USERNAME", "NEXUS_PASSWORD"})

	require.NoError(t, err)
	assert.Equal(t, "admin", got["NEXUS_USERNAME"])
	assert.Equal(t, "nexus-secret", got["NEXUS_PASSWORD"])
}

// 플랫폼이 소유하지 않는 레지스트리는 풀 수 없다. 조용히 빈 값을 채우면
// CI 가 엉뚱한 자격증명으로 로그인을 시도한다.
func TestResolve_LeavesUnknownVariablesAlone(t *testing.T) {
	got, err := resolver(&fakeStore{}).Resolve(context.Background(),
		[]string{"REGISTRY_USERNAME", "REGISTRY_PASSWORD"})

	require.NoError(t, err)
	assert.Empty(t, got)
}

// 값이 비어 있으면 채우지 않는다 — 빈 문자열을 등록하면 set -u 는 통과하지만
// 로그인이 실패해, 원인이 한 겹 더 멀어진다.
func TestResolve_SkipsEmptySecret(t *testing.T) {
	got, err := resolver(&fakeStore{values: map[string]string{}}).Resolve(
		context.Background(), []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"})

	require.NoError(t, err)
	assert.Empty(t, got)
}

// 저장소를 못 읽는 것은 조용히 넘길 일이 아니다. 호출부가 경고로 옮겨 담는다.
func TestResolve_ReportsStoreFailure(t *testing.T) {
	_, err := resolver(&fakeStore{err: errors.New("openbao unreachable")}).Resolve(
		context.Background(), []string{"HARBOR_PASSWORD"})

	require.Error(t, err)
}

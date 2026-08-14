package rotation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func giteaMetadata() map[string]any {
	return map[string]any{
		"stack_id":   "stk_1",
		"cluster_id": "cluster-1",
		"namespace":  "nullus-demo",
		"org_id":     "org-1",
		"env":        "dev",
	}
}

func TestGiteaReissuer_IssuesNewToken(t *testing.T) {
	var got GiteaReissueSpec
	r := NewGiteaReissuer(func(_ context.Context, spec GiteaReissueSpec) (string, error) {
		got = spec
		return "new-token", nil
	})

	token, err := r.Reissue(context.Background(), ReissueInput{
		Provider: "gitea", Metadata: giteaMetadata(),
	})

	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
	assert.Equal(t, "stk_1", got.StackID)
	assert.Equal(t, "nullus-demo", got.Namespace)
}

// 다른 provider 를 가로채면 GitLab/GitHub 회전이 Gitea 경로로 흘러간다.
func TestGiteaReissuer_OtherProviderIsUnsupported(t *testing.T) {
	r := NewGiteaReissuer(func(context.Context, GiteaReissueSpec) (string, error) {
		t.Fatal("다른 provider 를 가로채면 안 된다")
		return "", nil
	})

	_, err := r.Reissue(context.Background(), ReissueInput{Provider: "gitlab"})
	assert.ErrorIs(t, err, ErrReissueUnsupported)
}

// 배선되지 않았는데 성공을 돌려주면 만료된 토큰이 그대로 남은 채 회전이 끝난
// 것으로 기록된다.
func TestGiteaReissuer_UnwiredIsUnsupported(t *testing.T) {
	_, err := NewGiteaReissuer(nil).Reissue(context.Background(), ReissueInput{
		Provider: "gitea", Metadata: giteaMetadata(),
	})
	assert.ErrorIs(t, err, ErrReissueUnsupported)
}

// 빈 좌표로 발급을 시도하면 엉뚱한 네임스페이스의 파드에 exec 하거나
// 다른 스택의 시크릿 경로에 쓰게 된다.
func TestGiteaReissuer_RequiresStackCoordinates(t *testing.T) {
	r := NewGiteaReissuer(func(context.Context, GiteaReissueSpec) (string, error) {
		t.Fatal("metadata 가 불완전한데 발급을 시도하면 안 된다")
		return "", nil
	})

	md := giteaMetadata()
	delete(md, "namespace")

	_, err := r.Reissue(context.Background(), ReissueInput{Provider: "gitea", Metadata: md})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace")
}

// 빈 토큰을 성공으로 다루면 이후 모든 호출이 401 로 죽는다.
func TestGiteaReissuer_EmptyTokenIsError(t *testing.T) {
	r := NewGiteaReissuer(func(context.Context, GiteaReissueSpec) (string, error) {
		return "  ", nil
	})

	_, err := r.Reissue(context.Background(), ReissueInput{
		Provider: "gitea", Metadata: giteaMetadata(),
	})
	require.Error(t, err)
}

func TestGiteaReissuer_IssueFailurePropagates(t *testing.T) {
	r := NewGiteaReissuer(func(context.Context, GiteaReissueSpec) (string, error) {
		return "", errors.New("pod not found")
	})

	_, err := r.Reissue(context.Background(), ReissueInput{
		Provider: "gitea", Metadata: giteaMetadata(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pod not found")
}

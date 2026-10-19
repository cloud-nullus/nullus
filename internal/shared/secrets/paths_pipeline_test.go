package secrets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineCredentialPrefix(t *testing.T) {
	prefix, err := PipelineCredentialPrefix("dev", "org-1", " api ")
	require.NoError(t, err)
	assert.Equal(t, "kv/nullus/dev/org-1/cicd/pipelines/api/", prefix)

	// 조직이 비면 예전부터 쓰던 "default" 다 — 이미 쓴 값과 경로가 갈라지면 회수하지 못한다.
	prefix, err = PipelineCredentialPrefix("", "", "api")
	require.NoError(t, err)
	assert.Equal(t, "kv/nullus/dev/default/cicd/pipelines/api/", prefix)
}

// 앱 이름은 경로의 한 세그먼트다. "..", "/" 가 들어가면 client-go 가 경로를 정리하면서 다른
// 파이프라인의 값을 가리키게 된다(검토에서 재현: pipelines/../pipelines/other/…).
func TestPipelineCredentialPrefix_RejectsNamesThatAreNotOneSegment(t *testing.T) {
	for _, app := range []string{"", " ", ".", "..", "a/b", "x/../victim", "../x", `a\b`} {
		_, err := PipelineCredentialPrefix("dev", "org-1", app)
		assert.Error(t, err, "app=%q", app)
	}
}

// 스택 모듈의 부트스트랩 정책은 이 경로 깊이에만 삭제를 허용한다. 값 경로와 정책 경로가
// 갈라지면 컴파일은 되고 운영에서 403 만 난다.
func TestPipelineCredentialPolicyPathMatchesValuePaths(t *testing.T) {
	prefix, err := PipelineCredentialPrefix("prod", "org-9", "api")
	require.NoError(t, err)
	valuePath := strings.TrimPrefix(prefix+"harbor_password", "kv/")

	policy := strings.Split(strings.TrimPrefix(PipelineCredentialsPolicyPath, "metadata/"), "/")
	got := strings.Split(valuePath, "/")
	require.Len(t, got, len(policy), "정책과 값 경로의 깊이가 같아야 한다")
	for i, seg := range policy {
		if seg != "+" {
			assert.Equal(t, seg, got[i], "세그먼트 %d", i)
		}
	}
	assert.False(t, strings.HasSuffix(PipelineCredentialsPolicyPath, "*"),
		"끝이 * 면 nullus/* 규칙이 이겨 delete 가 무시된다")
}

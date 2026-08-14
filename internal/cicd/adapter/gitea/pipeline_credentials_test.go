package gitea

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type stubWriter struct {
	written map[string]string
	err     error
}

func newStubWriter() *stubWriter { return &stubWriter{written: map[string]string{}} }

func (w *stubWriter) PutTokenForStack(_ context.Context, _, _, path, value string) error {
	if w.err != nil {
		return w.err
	}
	w.written[path] = value
	return nil
}

// Jenkinsfile 의 envFrom 이 참조하는 이름과 같아야 한다.
// 갈라지면 agent 파드가 없는 Secret 을 참조해 기동하지 못한다.
func TestCISecretName_MatchesScaffoldContract(t *testing.T) {
	assert.Equal(t, "nullus-ci-api", CISecretName("api"))
}

func TestProvision_WritesToOpenBaoAndRendersExternalSecret(t *testing.T) {
	w := newStubWriter()
	plane := NewCredentialPlane(w, "dev", "org-1", "stk_1", "nullus-demo")

	manifest, err := plane.Provision(context.Background(), "api", []port.PipelineVariable{
		{Key: "HARBOR_PASSWORD", Value: "pw"},
		{Key: "HARBOR_USERNAME", Value: "robot"},
	})
	require.NoError(t, err)

	// 값은 매니페스트가 아니라 OpenBao 에 간다 — 매니페스트에 평문이 실리면
	// 클러스터 리소스에 자격증명이 노출된다.
	assert.NotContains(t, manifest, "pw")
	assert.Equal(t, "pw", w.written["kv/nullus/dev/org-1/cicd/pipelines/api/harbor_password"])
	assert.Equal(t, "robot", w.written["kv/nullus/dev/org-1/cicd/pipelines/api/harbor_username"])

	assert.Contains(t, manifest, "name: nullus-ci-api")
	assert.Contains(t, manifest, "namespace: nullus-demo")
	assert.Contains(t, manifest, "name: "+ESOSecretStoreName)
	assert.Contains(t, manifest, "secretKey: HARBOR_USERNAME")
	assert.Contains(t, manifest, "secretKey: HARBOR_PASSWORD")
}

// 같은 입력이 같은 매니페스트를 내야 재적용이 의미 없는 diff 를 만들지 않는다.
func TestProvision_IsDeterministicRegardlessOfInputOrder(t *testing.T) {
	first, err := NewCredentialPlane(newStubWriter(), "dev", "o", "s", "ns").
		Provision(context.Background(), "api", []port.PipelineVariable{
			{Key: "B_VAR", Value: "2"}, {Key: "A_VAR", Value: "1"},
		})
	require.NoError(t, err)

	second, err := NewCredentialPlane(newStubWriter(), "dev", "o", "s", "ns").
		Provision(context.Background(), "api", []port.PipelineVariable{
			{Key: "A_VAR", Value: "1"}, {Key: "B_VAR", Value: "2"},
		})
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Less(t, strings.Index(first, "A_VAR"), strings.Index(first, "B_VAR"))
}

// 기록에 실패했는데 매니페스트를 돌려주면 ESO 가 없는 경로를 참조해
// Secret 이 영원히 만들어지지 않는다 — 원인이 멀리 떨어진 실패가 된다.
func TestProvision_WriteFailureReturnsError(t *testing.T) {
	w := newStubWriter()
	w.err = errors.New("openbao unreachable")

	_, err := NewCredentialPlane(w, "dev", "o", "s", "ns").
		Provision(context.Background(), "api", []port.PipelineVariable{{Key: "K", Value: "v"}})

	require.Error(t, err)
}

func TestProvision_NoVariablesIsNoOp(t *testing.T) {
	manifest, err := NewCredentialPlane(newStubWriter(), "dev", "o", "s", "ns").
		Provision(context.Background(), "api", nil)

	require.NoError(t, err)
	assert.Empty(t, manifest)
}

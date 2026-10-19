package gitea

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

type stubWriter struct {
	written      map[string]string
	deletedTrees []string
	err          error
}

func (w *stubWriter) DeleteTreeForStack(_ context.Context, provider, stackID, prefix string) error {
	if w.err != nil {
		return w.err
	}
	w.deletedTrees = append(w.deletedTrees, provider+" "+stackID+" "+prefix)
	return nil
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

// 파이프라인을 지우면 Provision 이 금고에 쓴 값을 회수한다 — 그 파이프라인 몫의 경로만.
func TestRevoke_DeletesPipelineTreeInOpenBao(t *testing.T) {
	w := newStubWriter()
	plane := NewCredentialPlane(w, "dev", "org-1", "stk_1", "nullus-demo")

	require.NoError(t, plane.Revoke(context.Background(), " api "))

	assert.Equal(t, []string{"openbao stk_1 kv/nullus/dev/org-1/cicd/pipelines/api/"}, w.deletedTrees)
}

func TestRevoke_RequiresAppName(t *testing.T) {
	plane := NewCredentialPlane(newStubWriter(), "dev", "org-1", "stk_1", "nullus-demo")
	assert.Error(t, plane.Revoke(context.Background(), " "))
}

// 클러스터에서 지울 객체는 Provision 이 만든 ExternalSecret 이다. 이름이 같아도 남의 것을
// 지우지 않도록, 그 객체가 이 파이프라인의 금고 경로만 가리키는지 볼 기준을 함께 준다.
func TestCredentialObject_PointsAtProvisionedExternalSecret(t *testing.T) {
	plane := NewCredentialPlane(newStubWriter(), "dev", "org-1", "stk_1", "nullus-demo")

	obj, err := plane.CredentialObject("api")
	require.NoError(t, err)

	assert.Equal(t, port.PipelineCredentialObject{
		Namespace:       "nullus-demo",
		Name:            "nullus-ci-api",
		RemoteKeyPrefix: "kv/nullus/dev/org-1/cicd/pipelines/api/",
	}, obj)
}

// 새로 만드는 ExternalSecret 에는 플랫폼 표시를 붙인다 — 지울 때 남의 것과 가린다.
func TestProvision_LabelsExternalSecretAsPlatformOwned(t *testing.T) {
	plane := NewCredentialPlane(newStubWriter(), "dev", "org-1", "stk_1", "nullus-demo")

	manifest, err := plane.Provision(context.Background(), "api", []port.PipelineVariable{{Key: "K", Value: "v"}})
	require.NoError(t, err)

	// 값은 따옴표로 감싼다 — "true" 나 숫자 같은 앱 이름이 YAML 에서 문자열이 아닌 값이 되지 않게.
	assert.Contains(t, manifest, `app.kubernetes.io/managed-by: "nullus-cicd"`)
	assert.Contains(t, manifest, `app.kubernetes.io/name: "api"`)
}

// 삭제 권한이 생기기 전에 설치한 스택은 금고 정책이 삭제를 막는다(root 토큰은 부트스트랩 뒤
// 폐기돼 정책을 고칠 수 없다). 무엇이 왜 남았는지 알아듣게 돌려준다.
func TestRevoke_ExplainsPolicyWithoutDeletePermission(t *testing.T) {
	w := newStubWriter()
	w.err = errors.Join(&secrets.StatusError{Status: 403, Body: `{"errors":["permission denied"]}`})
	plane := NewCredentialPlane(w, "dev", "org-1", "stk_1", "nullus-demo")

	err := plane.Revoke(context.Background(), "api")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "삭제 권한")
	assert.Contains(t, err.Error(), "직접 지워야")
	assert.True(t, secrets.IsForbidden(err))
}

// 경로 한 세그먼트가 아닌 이름은 쓰지도, 지우지도 않는다 — 다른 파이프라인의 값을 가리킨다.
func TestPlane_RejectsNamesThatAreNotOneSegment(t *testing.T) {
	w := newStubWriter()
	plane := NewCredentialPlane(w, "dev", "org-1", "stk_1", "nullus-demo")

	for _, app := range []string{"..", "x/../victim", "a/b"} {
		_, err := plane.Provision(context.Background(), app, []port.PipelineVariable{{Key: "K", Value: "v"}})
		assert.Error(t, err, app)
		assert.Error(t, plane.Revoke(context.Background(), app), app)
		_, err = plane.CredentialObject(app)
		assert.Error(t, err, app)
	}
	assert.Empty(t, w.written)
	assert.Empty(t, w.deletedTrees)
}

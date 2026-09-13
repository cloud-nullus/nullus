package provisioning

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type recordingVariableWriter struct {
	calls   []string
	failFor string
}

func (w *recordingVariableWriter) SetPipelineVariable(_ context.Context, projectID, key, value string) error {
	w.calls = append(w.calls, projectID+" "+key+"="+value)
	if projectID == w.failFor {
		return errors.New("403 forbidden")
	}
	return nil
}

func TestVariablePolicyPublisher_WritesEveryVariableToEachAppProject(t *testing.T) {
	w := &recordingVariableWriter{failFor: "acme/broken"}
	vars := port.ScanPolicyVariables(domain.DefaultScanPolicy())

	results := newVariablePolicyPublisher(w, "acme").
		PublishScanPolicy(context.Background(), []string{"shop", "broken"}, vars)

	assert.NoError(t, results["shop"])
	assert.Error(t, results["broken"], "일부 실패를 성공으로 덮으면 그 파이프라인이 옛 정책으로 도는 것을 모른다")
	for _, v := range vars {
		assert.Contains(t, w.calls, "acme/shop "+v.Key+"="+v.Value)
	}
}

type recordingApplier struct {
	kubeconfig []byte
	manifests  []string
	err        error
}

func (a *recordingApplier) Apply(_ context.Context, kubeconfig []byte, manifests []string) error {
	a.kubeconfig = kubeconfig
	a.manifests = append(a.manifests, manifests...)
	return a.err
}

func (a *recordingApplier) ApplyWithTracking(context.Context, []byte, []string, string, ...int) error {
	return nil
}

type recordingKubeconfig struct {
	clusters []string
	err      error
}

func (k *recordingKubeconfig) GetKubeconfig(_ context.Context, clusterID string) ([]byte, error) {
	k.clusters = append(k.clusters, clusterID)
	return []byte("kubeconfig-" + clusterID), k.err
}

// Jenkins 스택은 ConfigMap 하나를 스택 네임스페이스에 적용한다 — 앱 수만큼 적용하지 않는다.
func TestConfigMapPolicyPublisher_AppliesOneConfigMapForTheStack(t *testing.T) {
	applier := &recordingApplier{}
	kc := &recordingKubeconfig{}

	results := newConfigMapPolicyPublisher(applier, kc, "c1", "devsecops").
		PublishScanPolicy(context.Background(), []string{"shop", "cart"},
			port.ScanPolicyVariables(domain.DefaultScanPolicy()))

	require.Len(t, applier.manifests, 1)
	assert.Contains(t, applier.manifests[0], "name: "+port.ScanPolicyConfigMapName)
	assert.Contains(t, applier.manifests[0], "namespace: devsecops")
	assert.Equal(t, []string{"c1"}, kc.clusters)
	assert.Equal(t, "kubeconfig-c1", string(applier.kubeconfig))
	assert.NoError(t, results["shop"])
	assert.NoError(t, results["cart"])
}

// 적용이 실패하면 그 스택의 Jenkins 파이프라인 모두가 옛 정책으로 돈다.
func TestConfigMapPolicyPublisher_ApplyFailureFailsEveryApp(t *testing.T) {
	applier := &recordingApplier{err: errors.New("connection refused")}

	results := newConfigMapPolicyPublisher(applier, &recordingKubeconfig{}, "c1", "devsecops").
		PublishScanPolicy(context.Background(), []string{"shop", "cart"},
			port.ScanPolicyVariables(domain.DefaultScanPolicy()))

	for _, app := range []string{"shop", "cart"} {
		require.Error(t, results[app])
		assert.True(t, strings.Contains(results[app].Error(), "connection refused"))
	}
}

// GitLab·GitHub 스택 번들은 앱 프로젝트 변수로 정책을 싣는다.
func TestFor_GitLabBundlePublishesScanPolicyToProjectVariables(t *testing.T) {
	rec := &pathRecorder{}
	f := NewBundleFactory(&fakeStackReader{summary: gitlabStack()}, &fakeTokenIssuer{token: "t"}, Options{
		Env: "dev", GroupPath: "acme", GitLabBaseURLOverride: rec.server(t, map[string]any{}),
	})

	bundle, err := f.For(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, bundle.ScanPolicy, "GitLab 스택에 정책을 실을 경로가 없다")

	results := bundle.ScanPolicy.PublishScanPolicy(context.Background(), []string{"shop"},
		port.ScanPolicyVariables(domain.DefaultScanPolicy()))
	assert.NoError(t, results["shop"])
	assert.True(t, rec.has("/api/v4/projects/acme%2Fshop/variables"))
}

func TestFor_GitHubBundlePublishesScanPolicyToRepositoryVariables(t *testing.T) {
	rec := &pathRecorder{}
	conns := &fakeConnectionReader{conn: &port.SCMConnection{
		Platform: port.SCMPlatformGitHub, Owner: "acme", APIBaseURL: rec.server(t, map[string]any{}),
	}}
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{Env: "dev"}).
		WithGitHub(&fakeTokenIssuer{token: "ghp"}, conns)

	bundle, err := f.For(context.Background(), "stk_gh")
	require.NoError(t, err)
	require.NotNil(t, bundle.ScanPolicy, "GitHub 스택에 정책을 실을 경로가 없다")

	results := bundle.ScanPolicy.PublishScanPolicy(context.Background(), []string{"shop"},
		port.ScanPolicyVariables(domain.DefaultScanPolicy()))
	assert.NoError(t, results["shop"])
	assert.True(t, rec.has("/repos/acme/shop/actions/variables"))
}

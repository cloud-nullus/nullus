package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func baseSpec() ApplicationSpec {
	return ApplicationSpec{
		AppName:              "myapp",
		ArgoNamespace:        "devsecops",
		RepoURL:              "http://gitlab-webservice-default.devsecops.svc:8181/acme/myapp.git",
		Path:                 "deploy",
		TargetRevision:       "main",
		DestinationNamespace: "acme-prod",
	}
}

func parseApp(t *testing.T, manifest string) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &doc))
	return doc
}

func TestRenderApplication_ProducesArgoCDApplication(t *testing.T) {
	manifest, err := RenderApplication(baseSpec())
	require.NoError(t, err)

	doc := parseApp(t, manifest)
	assert.Equal(t, "argoproj.io/v1alpha1", doc["apiVersion"])
	assert.Equal(t, "Application", doc["kind"])

	meta := doc["metadata"].(map[string]any)
	assert.Equal(t, "myapp", meta["name"])
	// Application 은 Argo CD 가 설치된 네임스페이스에 있어야 인식된다.
	assert.Equal(t, "devsecops", meta["namespace"])
}

// 앱 저장소의 deploy/ 를 바라봐야 한다 — CI 가 태그를 갱신하는 곳이다.
func TestRenderApplication_PointsAtRepositoryDeployPath(t *testing.T) {
	manifest, err := RenderApplication(baseSpec())
	require.NoError(t, err)

	spec := parseApp(t, manifest)["spec"].(map[string]any)
	source := spec["source"].(map[string]any)

	assert.Equal(t, "http://gitlab-webservice-default.devsecops.svc:8181/acme/myapp.git", source["repoURL"])
	assert.Equal(t, "deploy", source["path"])
	assert.Equal(t, "main", source["targetRevision"])

	dest := spec["destination"].(map[string]any)
	assert.Equal(t, "acme-prod", dest["namespace"])
	assert.Equal(t, "https://kubernetes.default.svc", dest["server"])
}

// 자동 동기화가 없으면 CI 가 태그를 갱신해도 배포가 일어나지 않는다.
func TestRenderApplication_EnablesAutomatedSync(t *testing.T) {
	manifest, err := RenderApplication(baseSpec())
	require.NoError(t, err)

	spec := parseApp(t, manifest)["spec"].(map[string]any)
	syncPolicy := spec["syncPolicy"].(map[string]any)
	automated, ok := syncPolicy["automated"].(map[string]any)
	require.True(t, ok, "automated sync 가 없으면 태그 갱신이 배포로 이어지지 않는다")

	assert.Equal(t, true, automated["prune"])
	assert.Equal(t, true, automated["selfHeal"])

	// 대상 네임스페이스가 없으면 첫 동기화가 실패한다.
	options, ok := syncPolicy["syncOptions"].([]any)
	require.True(t, ok)
	assert.Contains(t, options, "CreateNamespace=true")
}

func TestRenderApplication_DefaultsRevisionAndPath(t *testing.T) {
	in := baseSpec()
	in.Path = ""
	in.TargetRevision = ""

	manifest, err := RenderApplication(in)
	require.NoError(t, err)

	source := parseApp(t, manifest)["spec"].(map[string]any)["source"].(map[string]any)
	assert.Equal(t, DefaultManifestPath, source["path"])
	assert.Equal(t, DefaultTargetRevision, source["targetRevision"])
}

func TestRenderApplication_UsesConfiguredArgoProject(t *testing.T) {
	in := baseSpec()
	in.ArgoProject = "platform"

	manifest, err := RenderApplication(in)
	require.NoError(t, err)

	spec := parseApp(t, manifest)["spec"].(map[string]any)
	assert.Equal(t, "platform", spec["project"])
}

func TestRenderApplication_DefaultsArgoProjectToDefault(t *testing.T) {
	manifest, err := RenderApplication(baseSpec())
	require.NoError(t, err)

	spec := parseApp(t, manifest)["spec"].(map[string]any)
	assert.Equal(t, "default", spec["project"])
}

// Nullus 가 만든 리소스임을 라벨로 남겨야 조회·정리가 가능하다.
func TestRenderApplication_LabelsManagedBy(t *testing.T) {
	manifest, err := RenderApplication(baseSpec())
	require.NoError(t, err)

	meta := parseApp(t, manifest)["metadata"].(map[string]any)
	labels := meta["labels"].(map[string]any)
	assert.Equal(t, "nullus-cicd", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "myapp", labels["app.kubernetes.io/name"])
}

func TestRenderApplication_RequiresAppNameAndRepo(t *testing.T) {
	in := baseSpec()
	in.AppName = ""
	_, err := RenderApplication(in)
	require.Error(t, err)

	in = baseSpec()
	in.RepoURL = ""
	_, err = RenderApplication(in)
	require.Error(t, err)

	in = baseSpec()
	in.ArgoNamespace = ""
	_, err = RenderApplication(in)
	require.Error(t, err)
}

func TestRenderApplication_IsDeterministic(t *testing.T) {
	first, err := RenderApplication(baseSpec())
	require.NoError(t, err)
	second, err := RenderApplication(baseSpec())
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

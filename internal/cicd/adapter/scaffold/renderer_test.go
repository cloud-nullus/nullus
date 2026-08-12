package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func gitlabTarget() *port.ImageTarget {
	return &port.ImageTarget{
		Kind:        port.RegistryKindSCMProject,
		Host:        "registry.nullus.local",
		Repository:  "registry.nullus.local/acme/myapp",
		UsernameVar: "CI_REGISTRY_USER",
		PasswordVar: "CI_REGISTRY_PASSWORD",
	}
}

func harborTarget() *port.ImageTarget {
	return &port.ImageTarget{
		Kind:              port.RegistryKindHarbor,
		Host:              "harbor.nullus.local",
		Repository:        "harbor.nullus.local/acme/myapp",
		UsernameVar:       "HARBOR_USERNAME",
		PasswordVar:       "HARBOR_PASSWORD",
		RequiredVariables: []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
	}
}

func baseInput() Input {
	return Input{
		AppName:     "myapp",
		Namespace:   "acme-prod",
		Port:        8080,
		Replicas:    2,
		ImageTarget: gitlabTarget(),
	}
}

func fileMap(t *testing.T, files []port.CommitFile) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range files {
		out[f.Path] = f.Content
	}
	return out
}

func TestRender_ProducesCiAndDeployManifests(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)

	m := fileMap(t, files)
	assert.Contains(t, m, ".gitlab-ci.yml")
	assert.Contains(t, m, "deploy/deployment.yaml")
	assert.Contains(t, m, "deploy/service.yaml")
	assert.Contains(t, m, "Dockerfile")
}

func TestRender_ManifestsAreValidYAML(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	m := fileMap(t, files)

	for _, path := range []string{"deploy/deployment.yaml", "deploy/service.yaml", ".gitlab-ci.yml"} {
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(m[path]), &doc), "%s 가 YAML 로 파싱되어야 한다", path)
	}
}

// 이미지 경로는 resolver 가 정한 값을 그대로 써야 한다.
// 여기서 GitLab 을 전제하면 Harbor 스택에서 엉뚱한 곳에 올린다.
func TestRender_UsesResolvedRepositoryNotHardcodedRegistry(t *testing.T) {
	in := baseInput()
	in.ImageTarget = harborTarget()

	files, err := Render(in)
	require.NoError(t, err)
	m := fileMap(t, files)

	assert.Contains(t, m[".gitlab-ci.yml"], "harbor.nullus.local/acme/myapp")
	assert.NotContains(t, m[".gitlab-ci.yml"], "registry.nullus.local")
	assert.Contains(t, m["deploy/deployment.yaml"], "harbor.nullus.local/acme/myapp")
}

func TestRender_LoginUsesTargetProvidedVariables(t *testing.T) {
	in := baseInput()
	in.ImageTarget = harborTarget()

	files, err := Render(in)
	require.NoError(t, err)
	ci := fileMap(t, files)[".gitlab-ci.yml"]

	assert.Contains(t, ci, "$HARBOR_USERNAME")
	assert.Contains(t, ci, "$HARBOR_PASSWORD")
	assert.Contains(t, ci, "harbor.nullus.local")
	assert.NotContains(t, ci, "CI_REGISTRY_USER", "대상이 요구한 변수만 써야 한다")
}

// 외부 레지스트리는 자격증명을 파이프라인에 등록해야 한다. 무엇을 등록해야
// 하는지 파일에 남기지 않으면 실패 원인을 사용자가 알 수 없다.
func TestRender_DocumentsRequiredVariables(t *testing.T) {
	in := baseInput()
	in.ImageTarget = harborTarget()

	files, err := Render(in)
	require.NoError(t, err)
	m := fileMap(t, files)

	assert.Contains(t, m[".gitlab-ci.yml"], "HARBOR_USERNAME")
	require.Contains(t, m, "README.md")
	assert.Contains(t, m["README.md"], "HARBOR_USERNAME")
}

func TestRender_GitLabTargetNeedsNoExtraVariableDocs(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	m := fileMap(t, files)

	// 내장 변수만 쓰므로 "등록이 필요하다" 는 안내가 붙으면 안 된다.
	assert.NotContains(t, m["README.md"], "CI/CD Variables 에 등록")
}

// CI 가 이미지를 만든 뒤 deploy/ 의 태그를 갱신해 커밋해야 ArgoCD 가 감지한다.
// 클러스터 내부 레지스트리는 보통 HTTP 로 노출된다. docker 는 기본적으로
// HTTPS 를 요구하므로 dind 에 대상 호스트를 명시하지 않으면 push 가 실패한다.
func TestRender_DindTrustsRegistryHost(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	ci := fileMap(t, files)[".gitlab-ci.yml"]

	assert.Contains(t, ci, "--insecure-registry=registry.nullus.local",
		"HTTP 레지스트리를 신뢰하지 않으면 push 가 TLS 오류로 죽는다")
}

// job 컨테이너에는 docker 데몬이 없다. dind 서비스를 TCP 로 가리키지 않으면
// docker CLI 가 유닉스 소켓을 찾다 실패하고, TLS 디렉터리를 비우지 않으면
// dind 가 2376(TLS)만 열어 2375 연결이 거부된다.
func TestRender_ConfiguresDindConnection(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	ci := fileMap(t, files)[".gitlab-ci.yml"]

	assert.Contains(t, ci, "DOCKER_HOST: tcp://docker:2375")
	assert.Contains(t, ci, `DOCKER_TLS_CERTDIR: ""`)
}

// dind 는 즉시 뜨지 않는다. 기다리지 않으면 첫 docker 명령이 데몬 없이 실행돼
// 레지스트리 문제처럼 보이는 엉뚱한 오류로 죽는다 (실제로 겪었다).
func TestRender_WaitsForDockerDaemonBeforeBuilding(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	ci := fileMap(t, files)[".gitlab-ci.yml"]

	waitIdx := strings.Index(ci, "docker info")
	loginIdx := strings.Index(ci, "docker login")
	require.Greater(t, waitIdx, 0, "데몬 준비를 기다려야 한다")
	assert.Less(t, waitIdx, loginIdx, "대기가 login 보다 앞서야 한다")
}

func TestRender_DeployStageUpdatesManifestTagAndPushes(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	ci := fileMap(t, files)[".gitlab-ci.yml"]

	assert.Contains(t, ci, "deploy/deployment.yaml", "매니페스트 태그를 갱신해야 한다")
	assert.Contains(t, ci, "git push")
	assert.Contains(t, ci, DeployTokenVar, "저장소에 되쓰려면 쓰기 권한 토큰이 필요하다")

	// 게이트웨이가 HTTP 로만 노출된 구성에서 https 를 하드코딩하면 push 가
	// 연결 자체를 못 한다 (실제로 겪었다). 서버가 알려준 값을 쓴다.
	assert.Contains(t, ci, "${CI_SERVER_PROTOCOL}://")
	assert.NotContains(t, ci, `"https://oauth2:`)
}

// 커밋 되쓰기가 다시 파이프라인을 돌리면 무한 루프가 된다.
func TestRender_DeployCommitSkipsCI(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	ci := fileMap(t, files)[".gitlab-ci.yml"]

	assert.Contains(t, ci, "[skip ci]", "되쓰기 커밋이 파이프라인을 다시 돌리면 무한 루프가 된다")
}

func TestRender_DeploymentReflectsReplicasAndPort(t *testing.T) {
	in := baseInput()
	in.Replicas = 3
	in.Port = 9090

	files, err := Render(in)
	require.NoError(t, err)

	var dep struct {
		Spec struct {
			Replicas int `yaml:"replicas"`
			Template struct {
				Spec struct {
					Containers []struct {
						Ports []struct {
							ContainerPort int `yaml:"containerPort"`
						} `yaml:"ports"`
					} `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(fileMap(t, files)["deploy/deployment.yaml"]), &dep))

	assert.Equal(t, 3, dep.Spec.Replicas)
	require.Len(t, dep.Spec.Template.Spec.Containers, 1)
	require.Len(t, dep.Spec.Template.Spec.Containers[0].Ports, 1)
	assert.Equal(t, 9090, dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
}

func TestRender_AppliesDefaultsForReplicasAndPort(t *testing.T) {
	in := baseInput()
	in.Replicas = 0
	in.Port = 0

	files, err := Render(in)
	require.NoError(t, err)
	dep := fileMap(t, files)["deploy/deployment.yaml"]

	assert.Contains(t, dep, "replicas: 1")
	assert.Contains(t, dep, "containerPort: 8080")
}

func TestRender_RequiresAppNameAndTarget(t *testing.T) {
	in := baseInput()
	in.AppName = ""
	_, err := Render(in)
	require.Error(t, err)

	in = baseInput()
	in.ImageTarget = nil
	_, err = Render(in)
	require.Error(t, err)
}

// 매니페스트의 이미지 태그는 CI 가 sed 로 갱신한다. 초기값이 예측 가능한
// 형태여야 갱신 스크립트가 매칭할 수 있다.
func TestRender_InitialImageTagIsReplaceable(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	m := fileMap(t, files)

	assert.Contains(t, m["deploy/deployment.yaml"],
		"image: registry.nullus.local/acme/myapp:"+InitialImageTag)

	// CI 는 저장소 경로를 IMAGE_REPOSITORY 변수로 선언하고, sed 가 그 변수로
	// 매니페스트의 `image: <repo>:<tag>` 줄을 치환한다. 둘이 어긋나면 태그가
	// 갱신되지 않아 Argo CD 가 영원히 bootstrap 이미지를 본다.
	ci := m[".gitlab-ci.yml"]
	assert.Contains(t, ci, `IMAGE_REPOSITORY: "registry.nullus.local/acme/myapp"`)
	assert.Contains(t, ci, "s#image: $IMAGE_REPOSITORY:.*#image: $IMAGE_REPOSITORY:$IMAGE_TAG#")
}

func TestRender_NamespaceIsAppliedToManifests(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	m := fileMap(t, files)

	assert.Contains(t, m["deploy/deployment.yaml"], "namespace: acme-prod")
	assert.Contains(t, m["deploy/service.yaml"], "namespace: acme-prod")
}

func TestRender_IsDeterministic(t *testing.T) {
	first, err := Render(baseInput())
	require.NoError(t, err)
	second, err := Render(baseInput())
	require.NoError(t, err)

	assert.Equal(t, first, second, "같은 입력은 같은 파일을 만들어야 재스캐폴딩이 잡음을 만들지 않는다")
}

func TestRender_FilePathsAreSorted(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	sorted := append([]string(nil), paths...)
	assert.Equal(t, sortedCopy(sorted), paths, "커밋 순서가 흔들리면 diff 가 지저분해진다")
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.Compare(out[j], out[j-1]) < 0; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Service 만으로는 클러스터 밖에서 앱에 닿을 수 없다.
// 게이트웨이 정보가 있으면 외부 접근 경로를 함께 만들어야 한다.
func TestRender_CreatesHTTPRouteWhenGatewayKnown(t *testing.T) {
	in := baseInput()
	in.AccessDomain = "nullus-devsecops-stack.internal"
	in.GatewayName = "nullus-devsecops-stack-gateway"
	in.GatewayNamespace = "devsecops"

	files, err := Render(in)
	require.NoError(t, err)
	m := fileMap(t, files)

	require.Contains(t, m, "deploy/httproute.yaml")
	route := m["deploy/httproute.yaml"]

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(route), &doc))
	assert.Equal(t, "HTTPRoute", doc["kind"])

	assert.Contains(t, route, "myapp.nullus-devsecops-stack.internal")
	// 게이트웨이는 스택 네임스페이스에 있으므로 parentRef 가 경계를 넘는다.
	assert.Contains(t, route, "name: nullus-devsecops-stack-gateway")
	assert.Contains(t, route, "namespace: devsecops")
}

// 게이트웨이를 모르면 라우트를 만들지 않는다 — 존재하지 않는 게이트웨이를
// 가리키는 라우트는 Argo CD 동기화만 어지럽힌다.
func TestRender_SkipsHTTPRouteWhenGatewayUnknown(t *testing.T) {
	files, err := Render(baseInput())
	require.NoError(t, err)
	assert.NotContains(t, fileMap(t, files), "deploy/httproute.yaml")
}

// 기본 Dockerfile 은 고른 포트에서 실제로 들어야 한다.
//
// nginx 기본 설정은 80 에서 듣는다. EXPOSE 는 문서일 뿐 바인딩을 바꾸지 않으므로,
// 포트만 바꿔 놓으면 Service·Deployment·HTTPRoute 가 가리키는 곳에 아무도 없다.
// readinessProbe 도 없어서 파드는 Running 으로 보이고 Argo CD 도 Healthy 라고
// 보고한다 — 첫 배포가 조용히 응답 없는 상태로 끝난다.
func TestRender_DockerfileListensOnConfiguredPort(t *testing.T) {
	in := baseInput()
	in.Port = 8080
	files, err := Render(in)
	require.NoError(t, err)

	dockerfile := fileMap(t, files)["Dockerfile"]
	require.NotEmpty(t, dockerfile)

	assert.Contains(t, dockerfile, "EXPOSE 8080")
	// EXPOSE 만으로는 바인딩이 바뀌지 않는다. 이미지 안의 nginx 설정이 그 포트를
	// 듣도록 실제로 바뀌어야 한다.
	assert.Contains(t, dockerfile, "listen 8080",
		"nginx 가 8080 을 듣도록 설정을 바꿔야 한다 — EXPOSE 는 문서일 뿐이다")
}

// 포트가 80 이면 기본 설정 그대로여야 한다 — 불필요한 치환을 넣지 않는다.
func TestRender_DockerfileKeepsDefaultForPort80(t *testing.T) {
	in := baseInput()
	in.Port = 80
	files, err := Render(in)
	require.NoError(t, err)

	dockerfile := fileMap(t, files)["Dockerfile"]
	assert.Contains(t, dockerfile, "EXPOSE 80")
	assert.NotContains(t, dockerfile, "sed -i")
}

// 배포된 파드가 어느 스택 소속인지 클러스터만 보고도 알 수 있어야 한다.
//
// 네임스페이스로는 안 된다 — 파이프라인이 default 에 깔 수도 있고, 여러 스택이
// 한 네임스페이스를 공유할 수도 있다. 스택 도구 파드가 이미 nullus.io/stack-name
// 을 달고 있으므로(stack/adapter/helm/manifest-builders.go) 같은 접두사를 쓴다.
func TestRender_LabelsWorkloadsWithStackID(t *testing.T) {
	files, err := Render(Input{
		AppName:   "demo-app",
		Namespace: "apps",
		Port:      8080,
		Replicas:  2,
		StackID:   "stk_abc123",
		ImageTarget: &port.ImageTarget{
			Repository: "registry.example.com/demo-app",
		},
	})
	require.NoError(t, err)

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	for _, path := range []string{"deploy/deployment.yaml", "deploy/service.yaml"} {
		content, ok := byPath[path]
		require.True(t, ok, "%s 가 있어야 한다", path)
		assert.Contains(t, content, "nullus.io/stack-id: stk_abc123",
			"%s 에 스택 라벨이 있어야 워크로드 조회가 스택 단위로 걸린다", path)
	}

	// 파드 템플릿에도 붙어야 한다. Deployment 에만 붙으면 파드를 라벨로 못 찾는다.
	deployment := byPath["deploy/deployment.yaml"]
	assert.GreaterOrEqual(t, strings.Count(deployment, "nullus.io/stack-id: stk_abc123"), 2,
		"metadata 와 pod template 양쪽에 있어야 한다")
}

// 어느 CI/CD 템플릿으로 만든 앱인지도 클러스터에서 알아볼 수 있어야 한다.
//
// 스택 라벨만으로는 "이 스택의 앱들" 까지만 묶인다. 템플릿별로 자원 사용을
// 비교하거나(백엔드 템플릿이 프론트보다 메모리를 두 배 쓴다) 템플릿을 고친 뒤
// 그 템플릿으로 만든 앱들만 골라 보려면 별도 라벨이 필요하다.
//
// 라벨 값은 템플릿 id 다. 이름("Nullus Sample App — Backend")은 공백과 em dash 가
// 들어가 쿠버네티스 라벨 값으로 유효하지 않고, 바뀌면 과거 파드와 안 맞는다.
func TestRender_LabelsWorkloadsWithTemplateID(t *testing.T) {
	files, err := Render(Input{
		AppName:    "demo-app",
		Namespace:  "apps",
		Port:       8080,
		Replicas:   2,
		StackID:    "stk_abc123",
		TemplateID: "nullus-sample-backend-v1",
		ImageTarget: &port.ImageTarget{
			Repository: "registry.example.com/demo-app",
		},
	})
	require.NoError(t, err)

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	for _, path := range []string{"deploy/deployment.yaml", "deploy/service.yaml"} {
		content, ok := byPath[path]
		require.True(t, ok, "%s 가 있어야 한다", path)
		assert.Contains(t, content, "nullus.io/cicd-template-id: nullus-sample-backend-v1",
			"%s 에 템플릿 라벨이 있어야 템플릿 단위로 묶을 수 있다", path)
	}

	deployment := byPath["deploy/deployment.yaml"]
	assert.GreaterOrEqual(t, strings.Count(deployment, "nullus.io/cicd-template-id: nullus-sample-backend-v1"), 2,
		"metadata 와 pod template 양쪽에 있어야 파드를 라벨로 찾을 수 있다")
}

// 템플릿 없이 만든 파이프라인도 있다. 그때는 라벨을 붙이지 않는다 —
// 스택 라벨과 같은 이유로, 빈 값은 "템플릿 없음" 과 구분되지 않는다.
func TestRender_OmitsTemplateLabelWhenNoTemplate(t *testing.T) {
	files, err := Render(Input{
		AppName:   "demo-app",
		Namespace: "apps",
		Port:      8080,
		Replicas:  1,
		StackID:   "stk_abc123",
		ImageTarget: &port.ImageTarget{
			Repository: "registry.example.com/demo-app",
		},
	})
	require.NoError(t, err)

	for _, f := range files {
		assert.NotContains(t, f.Content, "nullus.io/cicd-template-id", "%s", f.Path)
	}
}

// 스택에 속하지 않는 파이프라인도 있다. 그때는 라벨을 붙이지 않는다 —
// 빈 값 라벨은 쿠버네티스에서 유효하지만 "스택 없음" 과 구분되지 않는다.
func TestRender_OmitsStackLabelWhenNoStack(t *testing.T) {
	files, err := Render(Input{
		AppName:   "demo-app",
		Namespace: "apps",
		Port:      8080,
		Replicas:  1,
		ImageTarget: &port.ImageTarget{
			Repository: "registry.example.com/demo-app",
		},
	})
	require.NoError(t, err)

	for _, f := range files {
		assert.NotContains(t, f.Content, "nullus.io/stack-id", "%s", f.Path)
	}
}

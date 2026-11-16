package scaffold

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func webInput() Input {
	in := baseInput()
	in.AppType = domain.AppTypeWeb
	return in
}

// 웹 앱 템플릿은 실제로 도는 React 앱을 준다.
//
// 예전에는 앱 타입과 무관하게 nginx 기본 페이지를 담은 Dockerfile 하나만
// 만들었다. 개발자는 배포가 끝난 뒤에야 그 안에 앱이 없다는 것을 알게 된다.
func TestScaffold_WebAppGetsReactSources(t *testing.T) {
	files, err := Render(webInput())
	require.NoError(t, err)
	byPath := fileMap(t, files)

	for _, want := range []string{
		"package.json", "vite.config.ts", "tsconfig.json",
		"index.html", "src/main.tsx", "src/App.tsx", ".dockerignore",
	} {
		assert.Contains(t, byPath, want)
	}

	// 버전은 플랫폼 자신이 쓰는 조합에 맞춘다. 함께 도는 것이 이미 검증됐고,
	// 팀이 아는 스택과 어긋나지 않는다.
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	require.NoError(t, json.Unmarshal([]byte(byPath["package.json"]), &pkg),
		"package.json 이 유효한 JSON 이어야 한다 — 깨지면 빌드가 첫 줄에서 죽는다")
	assert.True(t, strings.HasPrefix(pkg.Dependencies["react"], "^19."),
		"react 19: got %q", pkg.Dependencies["react"])
	assert.Equal(t, pkg.Dependencies["react"], pkg.Dependencies["react-dom"],
		"react 와 react-dom 은 같은 버전이어야 한다")
	assert.Contains(t, pkg.DevDependencies, "vite")
	assert.Contains(t, pkg.DevDependencies, "typescript")
}

// 빌드 결과를 nginx 가 서빙하는 2단계 이미지여야 한다. node 이미지를 그대로
// 배포하면 개발 서버가 프로덕션에 뜬다.
func TestScaffold_WebDockerfileBuildsThenServes(t *testing.T) {
	files, err := Render(webInput())
	require.NoError(t, err)
	dockerfile := fileMap(t, files)["Dockerfile"]

	assert.Contains(t, dockerfile, "AS build")
	assert.Contains(t, dockerfile, "npm run build")
	assert.Contains(t, dockerfile, "COPY --from=build /app/dist /usr/share/nginx/html")

	// SPA 는 새로고침하면 서버가 그 경로를 찾는다. try_files 로 index.html 에
	// 되돌리지 않으면 첫 화면 말고는 전부 404 가 된다.
	assert.Contains(t, dockerfile, "try_files")

	// 매니페스트가 가리키는 포트로 듣게 한다. 기본 80 으로 두면 Service 의
	// targetPort 와 어긋나 endpoints 가 비고, 원인이 멀리 떨어진 503 이 된다.
	assert.Contains(t, dockerfile, "listen 8080")
	assert.Contains(t, dockerfile, "EXPOSE 8080")
}

// 웹이 아닌 앱은 예전 그대로다. 백엔드 리포에 React 소스를 넣으면 안 된다.
func TestScaffold_NonWebKeepsPlaceholder(t *testing.T) {
	in := baseInput()
	in.AppType = domain.AppTypeBackend

	files, err := Render(in)
	require.NoError(t, err)
	byPath := fileMap(t, files)

	assert.NotContains(t, byPath, "package.json")
	assert.NotContains(t, byPath, "src/App.tsx")
	assert.Contains(t, byPath["Dockerfile"], "FROM nginx:alpine")
}

// 웹 앱의 최종(nginx) 단계도 같다 — 빌드 단계가 아니라 배포되는 이미지에
// OS 보안 패치가 들어가야 이미지 검사 게이트를 통과한다.
func TestScaffold_WebDockerfileAppliesOSSecurityUpdates(t *testing.T) {
	files, err := Render(webInput())
	require.NoError(t, err)
	dockerfile := fileMap(t, files)["Dockerfile"]

	runtime := strings.Index(dockerfile, "FROM nginx:alpine")
	upgrade := strings.Index(dockerfile, "RUN apk upgrade --no-cache")
	require.NotEqual(t, -1, runtime)
	require.Greater(t, upgrade, runtime,
		"최종 nginx 단계의 FROM 뒤에 OS 보안 패치(apk upgrade)가 있어야 한다")
	// 연속된 RUN 은 SonarQube(docker:S7031)가 새 이슈로 잡는다.
	assert.Equal(t, 1, strings.Count(dockerfile[runtime:], "\nRUN "), "nginx 단계의 RUN 은 하나여야 한다")
	assert.Contains(t, dockerfile, "RUN apk upgrade --no-cache \\\n && printf", "줄 이음은 백슬래시 하나")
}

package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	bootSampleURL    = "https://github.com/eGovFramework/egovframe-boot-sample-java-config"
	bootSampleCommit = "c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb"
	sourceDockerfile = "FROM maven:3.9-eclipse-temurin-17 AS build\n"
	sourceFont       = "wOFF\x00\x01\xff\xfe"
)

// importedInput 은 eGov 부트 샘플 모양의 원본을 담은 입력이다. 원본에 든 CI 정의·deploy/ 와
// 플랫폼이 소유하는 파일 이름을 일부러 섞었다.
func importedInput(profile domain.BuildProfile) Input {
	in := baseInput()
	in.Source = &ImportedSource{
		Origin: domain.TemplateSource{
			RepoURL:      bootSampleURL,
			Commit:       bootSampleCommit,
			Label:        "v5.0.0",
			BuildProfile: profile,
		},
		Files: []port.CommitFile{
			{Path: "pom.xml", Content: "<project/>"},
			{Path: "README.md", Content: "# 부트 기반 심플 게시판\n"},
			{Path: "Dockerfile", Content: sourceDockerfile},
			{Path: "src/main/resources/static/fonts/a.woff", Content: sourceFont},
			{Path: ".github/workflows/maven.yml", Content: "name: build-test\n"},
			{Path: ".github/ISSUE_TEMPLATE/bug_report.yml", Content: "name: bug\n"},
			{Path: ".gitlab-ci.yml", Content: "stages: [upstream]\n"},
			{Path: "Jenkinsfile", Content: "pipeline {}\n"},
			{Path: "deploy/old.yaml", Content: "kind: ConfigMap\n"},
			{Path: GitleaksConfigPath, Content: "# upstream gitleaks\n"},
		},
	}
	return in
}

func renderedFiles(t *testing.T, in Input) map[string]string {
	t.Helper()
	files, err := Render(in)
	require.NoError(t, err)
	seen := map[string]bool{}
	for i, f := range files {
		require.False(t, seen[f.Path], "경로가 두 번 나왔다: %s", f.Path)
		seen[f.Path] = true
		if i > 0 {
			require.Less(t, files[i-1].Path, f.Path, "같은 입력이면 같은 결과가 나오도록 정렬한다")
		}
	}
	return fileMap(t, files)
}

func TestRender_ImportedSource_KeepsSourceFiles(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileSpringBoot))

	assert.Equal(t, "<project/>", m["pom.xml"])
	assert.Equal(t, "# 부트 기반 심플 게시판\n", m["README.md"], "원본 README 는 원본 것이다")
	assert.Equal(t, sourceDockerfile, m["Dockerfile"], "spring-boot 는 원본 Dockerfile 을 그대로 쓴다")
	assert.Equal(t, sourceFont, m["src/main/resources/static/fonts/a.woff"], "바이너리는 바이트 그대로다")
	assert.Contains(t, m, ".github/ISSUE_TEMPLATE/bug_report.yml", "CI 정의가 아닌 .github 파일은 남긴다")
}

func TestRender_ImportedSource_PlatformOwnsPipelineAndDeploy(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileSpringBoot))

	assert.Contains(t, m[GitLabPipelinePath], "Nullus 가 생성한 파이프라인", "CI 정의는 플랫폼 것으로 바꾼다")
	assert.NotContains(t, m, ".github/workflows/maven.yml", "원본 워크플로를 두면 GitHub 스택에서 우리 CI 와 함께 돈다")
	assert.NotContains(t, m, JenkinsfilePath, "다른 CI 의 정의도 남기지 않는다")
	assert.NotContains(t, m, "deploy/old.yaml", "Argo CD 는 deploy/ 전체를 적용한다 — 원본 매니페스트가 함께 배포된다")
	assert.Contains(t, m, "deploy/deployment.yaml")
	assert.Contains(t, m, "deploy/service.yaml")
	assert.NotContains(t, m[GitleaksConfigPath], "upstream", "시크릿 탐지 설정은 플랫폼 것이다")
}

func TestRender_ImportedSource_NoAppScaffolding(t *testing.T) {
	in := importedInput(domain.BuildProfileSpringBoot)
	// web 이어도 원본이 앱이다. React 파일을 깔면 원본 소스를 덮는다.
	in.AppType = domain.AppTypeWeb
	m := renderedFiles(t, in)

	assert.NotContains(t, m, "package.json")
	assert.NotContains(t, m, "src/App.tsx")
	assert.Equal(t, sourceDockerfile, m["Dockerfile"])
}

func TestRender_ImportedSource_WritesPlatformGuide(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileSpringBoot))

	guide, ok := m[PlatformGuidePath]
	require.True(t, ok, "원본 README 를 덮지 않는 대신 플랫폼 안내를 따로 둔다")
	for _, want := range []string{bootSampleURL, bootSampleCommit, "v5.0.0", "spring-boot",
		".github/workflows/maven.yml", JenkinsfilePath, "deploy/old.yaml", GitleaksConfigPath} {
		assert.Contains(t, guide, want)
	}
	assert.Contains(t, guide, "따라가지 않습니다", "원본 업데이트 정책을 저장소 안에서도 알린다")
	assert.Contains(t, guide, "## 흐름", "흐름·변수 안내는 스캐폴딩 README 와 같다")
	assert.NotContains(t, guide, "Nullus 가 생성한 애플리케이션 프로젝트입니다", "원본을 가져온 저장소는 Nullus 가 만든 앱이 아니다")
}

func TestRender_ImportedSource_MissingDockerfile(t *testing.T) {
	in := importedInput(domain.BuildProfileRepository)
	files := in.Source.Files[:0]
	for _, f := range importedInput(domain.BuildProfileRepository).Source.Files {
		if f.Path != "Dockerfile" {
			files = append(files, f)
		}
	}
	in.Source.Files = files

	_, err := Render(in)
	require.Error(t, err, "CI 는 docker build . 만 한다 — Dockerfile 없이 만들면 첫 빌드가 실패한다")
	assert.Contains(t, err.Error(), "Dockerfile")
}

func TestRender_ImportedSource_MavenWARGeneratesDockerfile(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileMavenWAR))

	df := m["Dockerfile"]
	assert.NotEqual(t, sourceDockerfile, df, "원본 Dockerfile 은 미리 빌드한 WAR 를 복사해 우리 CI 에서 돌지 않는다")
	for _, want := range []string{mavenBuilderImage, tomcatRuntimeImage, "mvn ", "ROOT.war", "EXPOSE 8080"} {
		assert.Contains(t, df, want)
	}
	assert.NotContains(t, df, "server.xml", "Tomcat 기본 포트면 설정을 건드리지 않는다")
	assert.Contains(t, m[PlatformGuidePath], "Dockerfile", "원본 파일을 바꿨으면 안내에 적는다")
}

func TestRender_ImportedSource_MavenWARFollowsPort(t *testing.T) {
	in := importedInput(domain.BuildProfileMavenWAR)
	in.Port = 9090
	m := renderedFiles(t, in)

	assert.Contains(t, m["Dockerfile"], `port="9090"`, "Tomcat 이 매니페스트가 가리키는 포트에서 들어야 한다")
	assert.Contains(t, m["Dockerfile"], "EXPOSE 9090")
}

type probe struct {
	HTTPGet *struct {
		Path string `json:"path"`
	} `json:"httpGet"`
	TCPSocket *struct {
		Port string `json:"port"`
	} `json:"tcpSocket"`
	FailureThreshold int `json:"failureThreshold"`
}

type container struct {
	ReadinessProbe *probe `json:"readinessProbe"`
	Resources      struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	} `json:"resources"`
}

func deploymentContainer(t *testing.T, manifest string) container {
	t.Helper()
	var d struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []container `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &d))
	require.Len(t, d.Spec.Template.Spec.Containers, 1)
	return d.Spec.Template.Spec.Containers[0]
}

func TestRender_ImportedSource_SpringBootDeployment(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileSpringBoot))
	c := deploymentContainer(t, m["deploy/deployment.yaml"])

	require.NotNil(t, c.ReadinessProbe, "JVM 이 뜨는 동안 Argo CD 가 Healthy 로 보고하지 않게 준비를 본다")
	// actuator 를 전제하지 않는다 — eGov 부트 샘플 v5.0.0 에는 actuator 가 없어 그 경로가 404 다.
	// Spring Boot 는 컨텍스트가 다 뜬 뒤에 포트를 연다.
	require.NotNil(t, c.ReadinessProbe.TCPSocket)
	assert.Nil(t, c.ReadinessProbe.HTTPGet)
	assert.Equal(t, "http", c.ReadinessProbe.TCPSocket.Port)
	assert.Equal(t, "768Mi", c.Resources.Limits["memory"], "JVM 에 512Mi 는 빠듯하다")
}

func TestRender_ImportedSource_MavenWARDeployment(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileMavenWAR))
	c := deploymentContainer(t, m["deploy/deployment.yaml"])

	require.NotNil(t, c.ReadinessProbe)
	// Tomcat 은 웹앱을 배포한 뒤에 커넥터를 연다. WAR 마다 / 를 연다는 보장은 없다.
	require.NotNil(t, c.ReadinessProbe.TCPSocket)
	assert.Equal(t, "1Gi", c.Resources.Limits["memory"])
}

func TestRender_WithoutSource_DeploymentUnchanged(t *testing.T) {
	m := renderedFiles(t, baseInput())
	c := deploymentContainer(t, m["deploy/deployment.yaml"])

	assert.Nil(t, c.ReadinessProbe, "원본이 없는 스캐폴딩의 매니페스트는 그대로다")
	assert.Equal(t, "512Mi", c.Resources.Limits["memory"])
	assert.Contains(t, m, "README.md")
	assert.NotContains(t, m, PlatformGuidePath)
}

func TestRender_ImportedSource_GitHubDropsOnlyUpstreamWorkflows(t *testing.T) {
	in := importedInput(domain.BuildProfileSpringBoot)
	in.Platform = port.SCMPlatformGitHub
	m := renderedFiles(t, in)

	assert.Contains(t, m, GitHubWorkflowPath, "플랫폼 워크플로는 남는다")
	assert.NotContains(t, m, ".github/workflows/maven.yml")
	assert.NotContains(t, m, GitLabPipelinePath)
	var workflows []string
	for path := range m {
		if strings.HasPrefix(path, ".github/workflows/") {
			workflows = append(workflows, path)
		}
	}
	assert.Equal(t, []string{GitHubWorkflowPath}, workflows)
}

func withoutFile(files []port.CommitFile, path string) []port.CommitFile {
	out := make([]port.CommitFile, 0, len(files))
	for _, f := range files {
		if f.Path != path {
			out = append(out, f)
		}
	}
	return out
}

// eGov 부트 샘플 v5.0.0 에는 Dockerfile 이 없다(그 뒤 main 에 생겼다). spring-boot 프로필은 그때
// Maven 으로 실행 jar 를 만들어 JRE 위에 올리는 Dockerfile 을 쓴다.
func TestRender_ImportedSource_SpringBootGeneratesMissingDockerfile(t *testing.T) {
	in := importedInput(domain.BuildProfileSpringBoot)
	in.Source.Files = withoutFile(in.Source.Files, "Dockerfile")
	m := renderedFiles(t, in)

	df := m["Dockerfile"]
	for _, want := range []string{mavenBuilderImage, jreRuntimeImage, "mvn ", "app.jar", "EXPOSE 8080"} {
		assert.Contains(t, df, want)
	}
	assert.NotContains(t, df, "SERVER_PORT", "기본 포트면 설정을 건드리지 않는다")
	assert.Contains(t, m[PlatformGuidePath], "`Dockerfile`", "플랫폼이 만든 파일은 안내에 적는다")
}

func TestRender_ImportedSource_SpringBootDockerfileFollowsPort(t *testing.T) {
	in := importedInput(domain.BuildProfileSpringBoot)
	in.Source.Files = withoutFile(in.Source.Files, "Dockerfile")
	in.Port = 9090
	m := renderedFiles(t, in)

	assert.Contains(t, m["Dockerfile"], "SERVER_PORT=9090", "Spring Boot 는 SERVER_PORT 로 듣는 포트를 바꾼다")
	assert.Contains(t, m["Dockerfile"], "EXPOSE 9090")
}

// 원본 Dockerfile 을 쓴 경우는 플랫폼이 Dockerfile 을 관리한다고 적지 않는다.
func TestRender_ImportedSource_GuideDoesNotClaimSourceDockerfile(t *testing.T) {
	m := renderedFiles(t, importedInput(domain.BuildProfileSpringBoot))
	assert.NotContains(t, m[PlatformGuidePath], "- `Dockerfile`")
}

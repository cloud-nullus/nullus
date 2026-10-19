package scaffold

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 외부 원본을 가져온 저장소 — 템플릿에 원본(domain.TemplateSource)이 있을 때의 스캐폴딩.
//
// 파일 소유 규칙이 핵심이다. 소스·README·원본 Dockerfile 은 원본 것을 그대로 쓰고, CI 정의와
// deploy/ 매니페스트, 시크릿 탐지 설정은 플랫폼이 소유한다. 플랫폼의 "정의 갱신"이 CI 정의만
// 다시 쓰는 구조(IsPipelineDefinitionPath)가 이 규칙 위에 서 있다.

// PlatformGuidePath 는 원본을 가져온 저장소에 두는 플랫폼 안내다.
//
// 원본 README 를 덮지 않는다 — 그 프로젝트를 설명하는 문서는 원본 것이다. 흐름·변수·원본
// 출처는 이 파일에 따로 적는다.
const PlatformGuidePath = "NULLUS.md"

// 빌드 프로필이 만드는 Dockerfile 의 이미지. eGov 샘플 원본의 Dockerfile·CI 와 같은 조합이다
// (JDK 17, Tomcat 10.1 = Jakarta EE 10). 인터넷에서 받는다 — 원본을 가져오는 템플릿은
// 오프라인 설치를 지원하지 않으므로 에어갭 번들(RuntimeImages)에 싣지 않는다.
const (
	mavenBuilderImage  = "maven:3.9-eclipse-temurin-17"
	tomcatRuntimeImage = "tomcat:10.1-jdk17-temurin"
	jreRuntimeImage    = "eclipse-temurin:17-jre"
	tomcatDefaultPort  = 8080
	springDefaultPort  = 8080
)

// ImportedSource 는 템플릿이 가져온 원본이다.
type ImportedSource struct {
	// Origin 은 어디서 무엇을 가져왔는지다. 안내 문서와 빌드 프로필 판단에 쓴다.
	Origin domain.TemplateSource
	// Files 는 그 커밋의 파일이다(port.SourceArchive). 경로는 저장소 루트 기준이다.
	Files []port.CommitFile
}

// CheckImportable 은 원본을 이 저장소의 첫 커밋으로 올릴 수 있는지 본다.
//
// 저장소를 만들기 전에 부른다. 만든 뒤에 실패하면 README 만 든 저장소가 남고, 같은 이름으로 다시
// 만들면 "이미 있던 저장소" 라 스캐폴딩을 건너뛴다.
func CheckImportable(src *ImportedSource) error {
	if src == nil {
		return nil
	}
	if src.Origin.BuildProfile.GeneratesDockerfile() || hasFile(src.Files, dockerfilePath) {
		return nil
	}
	return fmt.Errorf("원본 %s@%s 의 저장소 루트에 Dockerfile 이 없습니다 — CI 는 루트의 Dockerfile 로 이미지를 만듭니다(빌드 프로필 %q)",
		src.Origin.RepoURL, src.Origin.ShortCommit(), src.Origin.BuildProfile)
}

// importOutcome 은 원본 파일 중 무엇을 빼고 무엇을 플랫폼 파일로 바꿨는지다. 안내에 적는다.
type importOutcome struct {
	dropped  []string
	replaced []string
	// generatedDockerfile 은 플랫폼이 Dockerfile 을 썼는지다(원본 것을 바꿨거나 원본에 없었다).
	generatedDockerfile bool
}

// composeImported 는 원본 파일에 플랫폼 소유 파일을 얹는다.
//
// 같은 경로면 플랫폼 파일이 이긴다. 원본의 다른 CI 정의와 deploy/ 는 가져오지 않는다 —
// 원본 워크플로를 두면 GitHub 스택에서 push 마다 우리 CI 와 함께 돌고, Argo CD 는 deploy/
// 전체를 적용하므로 원본 매니페스트가 함께 배포된다.
func composeImported(in Input, appPort int32, platform []port.CommitFile, pipelinePath string) ([]port.CommitFile, error) {
	src := in.Source
	profile := src.Origin.BuildProfile

	owned := make(map[string]bool, len(platform)+2)
	for _, f := range platform {
		owned[f.Path] = true
	}
	owned[PlatformGuidePath] = true
	if profile.ReplacesDockerfile() {
		owned[dockerfilePath] = true
	}

	if err := CheckImportable(src); err != nil {
		return nil, err
	}

	var (
		files   = make([]port.CommitFile, 0, len(src.Files)+len(platform)+2)
		outcome importOutcome
	)
	for _, f := range src.Files {
		switch {
		case owned[f.Path]:
			outcome.replaced = append(outcome.replaced, f.Path)
		case isUpstreamPipelineDefinition(f.Path) || strings.HasPrefix(f.Path, deployDir):
			outcome.dropped = append(outcome.dropped, f.Path)
		default:
			files = append(files, f)
		}
	}
	if profile.ReplacesDockerfile() || !hasFile(src.Files, dockerfilePath) {
		files = append(files, port.CommitFile{Path: dockerfilePath, Content: profileDockerfile(profile, appPort)})
		outcome.generatedDockerfile = true
	}

	sort.Strings(outcome.dropped)
	sort.Strings(outcome.replaced)
	files = append(files, platform...)
	files = append(files, port.CommitFile{
		Path:    PlatformGuidePath,
		Content: renderPlatformGuide(in, pipelinePath, outcome),
	})
	return files, nil
}

const (
	dockerfilePath = "Dockerfile"
	deployDir      = "deploy/"
)

func hasFile(files []port.CommitFile, path string) bool {
	for _, f := range files {
		if f.Path == path {
			return true
		}
	}
	return false
}

// profileDockerfile 은 빌드 프로필이 쓰는 Dockerfile 이다. CheckImportable 이 만들 수 없는 프로필을
// 먼저 걸러 낸다.
func profileDockerfile(profile domain.BuildProfile, appPort int32) string {
	if profile == domain.BuildProfileMavenWAR {
		return mavenWARDockerfile(appPort)
	}
	return springBootDockerfile(appPort)
}

// springBootDockerfile 은 Maven 으로 실행 jar 를 만들어 JRE 위에 올리는 Dockerfile 이다.
func springBootDockerfile(appPort int32) string {
	var b strings.Builder
	b.WriteString("# Nullus 가 생성한 Dockerfile 입니다(빌드 프로필 spring-boot).\n")
	b.WriteString("# 원본에 Dockerfile 이 없어 Maven 으로 실행 jar 를 만든 뒤 JRE 위에 올립니다.\n\n")
	fmt.Fprintf(&b, "FROM %s AS build\n", mavenBuilderImage)
	b.WriteString("WORKDIR /workspace\n")
	b.WriteString("COPY . .\n")
	b.WriteString("# 테스트는 건너뜁니다 — 원본 테스트에는 브라우저가 있어야 도는 Selenium 테스트가 있습니다.\n")
	b.WriteString("RUN mvn -B -ntp -Dmaven.test.skip=true package && cp target/*.jar /workspace/app.jar\n\n")
	fmt.Fprintf(&b, "FROM %s\n", jreRuntimeImage)
	b.WriteString("WORKDIR /app\n")
	b.WriteString("COPY --from=build /workspace/app.jar /app/app.jar\n")
	b.WriteString("ENV JAVA_OPTS=\"-XX:MaxRAMPercentage=75 -XX:+ExitOnOutOfMemoryError\"\n")
	if appPort != springDefaultPort {
		// Service·HTTPRoute 는 매니페스트의 포트를 가리킨다. Spring Boot 는 SERVER_PORT 를 읽는다.
		fmt.Fprintf(&b, "ENV SERVER_PORT=%d\n", appPort)
	}
	fmt.Fprintf(&b, "EXPOSE %d\n", appPort)
	b.WriteString("ENTRYPOINT [\"sh\", \"-c\", \"exec java $JAVA_OPTS -jar /app/app.jar\"]\n")
	return b.String()
}

// isUpstreamPipelineDefinition 은 원본에 든 CI 정의인지 본다. 어느 CI 의 것이든 우리 CI 와
// 겹쳐 돌거나 읽히지 않을 파일이다.
func isUpstreamPipelineDefinition(path string) bool {
	return path == GitLabPipelinePath || path == JenkinsfilePath || strings.HasPrefix(path, ".github/workflows/")
}

// mavenWARDockerfile 은 Maven 으로 WAR 를 만들어 Tomcat 의 ROOT 로 올리는 Dockerfile 이다.
//
// ROOT.war 로 올려야 / 에서 열린다. 원본은 app.war 로 올려 /app/ 에서 열렸는데, HTTPRoute 는
// <앱>.<도메인>/ 을 이 서비스로 보낸다.
func mavenWARDockerfile(appPort int32) string {
	var b strings.Builder
	b.WriteString("# Nullus 가 생성한 Dockerfile 입니다(빌드 프로필 maven-war).\n")
	b.WriteString("# 원본 Dockerfile 은 미리 빌드한 WAR 를 복사하는 구조라 CI 의 `docker build .` 로는 돌지 않습니다.\n")
	b.WriteString("# Maven 으로 WAR 를 만든 뒤 Tomcat 의 ROOT 로 올려 / 에서 열리게 합니다.\n\n")
	fmt.Fprintf(&b, "FROM %s AS build\n", mavenBuilderImage)
	b.WriteString("WORKDIR /workspace\n")
	b.WriteString("COPY . .\n")
	b.WriteString("# 테스트는 원본 CI 처럼 건너뜁니다 — Selenium 테스트는 브라우저가 있어야 돕니다.\n")
	b.WriteString("RUN mvn -B -ntp -Dmaven.test.skip=true package && cp target/*.war /workspace/app.war\n\n")
	fmt.Fprintf(&b, "FROM %s\n", tomcatRuntimeImage)
	b.WriteString("RUN rm -rf /usr/local/tomcat/webapps/*\n")
	b.WriteString("COPY --from=build /workspace/app.war /usr/local/tomcat/webapps/ROOT.war\n")
	if appPort != tomcatDefaultPort {
		// Service·HTTPRoute 는 매니페스트의 포트를 가리킨다. EXPOSE 는 바인딩을 바꾸지 않는다.
		fmt.Fprintf(&b, "RUN sed -i 's/port=\"%d\"/port=\"%d\"/' /usr/local/tomcat/conf/server.xml\n",
			tomcatDefaultPort, appPort)
	}
	fmt.Fprintf(&b, "EXPOSE %d\n", appPort)
	b.WriteString("CMD [\"catalina.sh\", \"run\"]\n")
	return b.String()
}

// renderPlatformGuide 는 원본을 가져온 저장소의 플랫폼 안내다.
func renderPlatformGuide(in Input, pipelinePath string, outcome importOutcome) string {
	origin := in.Source.Origin
	app := strings.TrimSpace(in.AppName)
	namespace := strings.TrimSpace(in.Namespace)
	if namespace == "" {
		namespace = "default"
	}

	var b strings.Builder
	b.WriteString("# Nullus 안내 — " + app + "\n\n")
	b.WriteString("이 저장소는 CI/CD 템플릿의 원본을 가져와 만들었습니다.\n\n")

	b.WriteString("## 원본\n\n")
	fmt.Fprintf(&b, "- 저장소: %s\n", origin.RepoURL)
	if label := strings.TrimSpace(origin.Label); label != "" {
		fmt.Fprintf(&b, "- 커밋: `%s` (%s)\n", origin.Commit, label)
	} else {
		fmt.Fprintf(&b, "- 커밋: `%s`\n", origin.Commit)
	}
	fmt.Fprintf(&b, "- 빌드 프로필: %s\n\n", buildProfileLabel(origin.BuildProfile))
	b.WriteString("이 저장소는 원본을 따라가지 않습니다. 원본에 새 버전이 나와도 여기는 바뀌지 않으며,\n")
	b.WriteString("지금부터 이 저장소의 코드는 여러분 것입니다. 프레임워크 보안 패치·버전업은 `pom.xml` 같은\n")
	b.WriteString("의존성 선언의 버전을 올려 받으세요.\n\n")

	b.WriteString("## 플랫폼이 관리하는 파일\n\n")
	fmt.Fprintf(&b, "- `%s` — CI 정의입니다. 플랫폼의 정의 갱신이 이 파일을 다시 씁니다.\n", pipelinePath)
	b.WriteString("- `deploy/` — 배포 매니페스트입니다. CI 의 deploy 단계가 이미지 태그를 갱신하고 Argo CD 가 동기화합니다.\n")
	switch {
	case outcome.generatedDockerfile && origin.BuildProfile.ReplacesDockerfile():
		b.WriteString("- `Dockerfile` — 빌드 프로필이 만든 파일입니다. 원본 Dockerfile 은 CI 에서 돌지 않아 대신 씁니다.\n")
	case outcome.generatedDockerfile:
		b.WriteString("- `Dockerfile` — 원본에 없어 빌드 프로필이 만든 파일입니다.\n")
	}
	fmt.Fprintf(&b, "- `%s`, `%s` — 시크릿 탐지 설정입니다.\n\n", PreCommitConfigPath, GitleaksConfigPath)

	if len(outcome.dropped) > 0 {
		b.WriteString("## 가져오지 않은 원본 파일\n\n")
		b.WriteString("원본의 다른 CI 정의는 우리 CI 와 겹쳐 돌고, `deploy/` 는 Argo CD 가 통째로 적용하므로 가져오지 않았습니다.\n\n")
		for _, p := range outcome.dropped {
			fmt.Fprintf(&b, "- `%s`\n", p)
		}
		b.WriteString("\n")
	}
	if len(outcome.replaced) > 0 {
		b.WriteString("## 플랫폼 파일로 바꾼 원본 파일\n\n")
		for _, p := range outcome.replaced {
			fmt.Fprintf(&b, "- `%s`\n", p)
		}
		b.WriteString("\n")
	}

	// 흐름·레지스트리·시크릿 탐지 안내는 스캐폴딩 README 와 같다. 그 README 의 도입부("Nullus 가
	// 생성한 애플리케이션 프로젝트입니다")는 원본을 가져온 저장소에 맞지 않아 뺀다.
	readme := renderReadme(in.Platform, app, namespace, in.ImageTarget)
	if i := strings.Index(readme, "## "); i >= 0 {
		readme = readme[i:]
	}
	b.WriteString(readme)
	return b.String()
}

func buildProfileLabel(p domain.BuildProfile) string {
	if p == domain.BuildProfileRepository {
		return "원본 Dockerfile 그대로"
	}
	return "`" + string(p) + "`"
}

// runtimeProfile 은 배포 매니페스트의 헬스 체크와 자원이다.
type runtimeProfile struct {
	// readiness 가 거짓이면 readinessProbe 를 두지 않는다.
	readiness           bool
	initialDelaySeconds int32
	requestCPU          string
	requestMemory       string
	limitCPU            string
	limitMemory         string
}

// defaultRuntime 은 원본이 없는 스캐폴딩의 값이다. 자리표시자 이미지에는 헬스 체크할 경로가 없다.
var defaultRuntime = runtimeProfile{
	requestCPU: "100m", requestMemory: "128Mi",
	limitCPU: "500m", limitMemory: "512Mi",
}

// runtimeProfileFor 는 빌드 프로필에 맞춘 값이다.
//
// JVM 은 뜨는 데 수십 초가 걸린다. probe 가 없으면 그동안 Argo CD 가 Healthy 로 보고하고,
// 512Mi 는 힙과 메타스페이스를 다 담기에 빠듯하다. 자원은 eGov 샘플 저장소의 k8s 매니페스트를 따른다.
func runtimeProfileFor(in Input) runtimeProfile {
	if in.Source == nil {
		return defaultRuntime
	}
	switch in.Source.Origin.BuildProfile {
	case domain.BuildProfileSpringBoot:
		return runtimeProfile{
			readiness: true, initialDelaySeconds: 20,
			requestCPU: "200m", requestMemory: "384Mi",
			limitCPU: "1000m", limitMemory: "768Mi",
		}
	case domain.BuildProfileMavenWAR:
		return runtimeProfile{
			readiness: true, initialDelaySeconds: 30,
			requestCPU: "250m", requestMemory: "512Mi",
			limitCPU: "1000m", limitMemory: "1Gi",
		}
	}
	return defaultRuntime
}

// readinessProbeLines 는 컨테이너 아래에 들어갈 readinessProbe 다. 없으면 빈 문자열이다.
//
// HTTP 경로를 보지 않고 포트가 열렸는지를 본다. actuator 는 앱마다 있다는 보장이 없고(eGov 부트
// 샘플 v5.0.0 에는 없어 /actuator/health 가 404 다), WAR 마다 / 를 연다는 보장도 없다. Spring
// Boot 는 컨텍스트가 다 뜬 뒤에, Tomcat 은 웹앱을 배포한 뒤에 포트를 연다.
//
// 실패 허용을 넉넉히 둔다(초기 지연 + 3분). 느린 노드에서 JVM 기동이 늦어도 준비되지 않은
// 것으로만 보고 재시작하지 않는다 — liveness 는 두지 않는다.
func readinessProbeLines(rt runtimeProfile) string {
	if !rt.readiness {
		return ""
	}
	return fmt.Sprintf(`          readinessProbe:
            tcpSocket:
              port: http
            initialDelaySeconds: %d
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 18
`, rt.initialDelaySeconds)
}

// Package scaffold 는 앱 프로젝트에 커밋할 초기 파일들을 만든다.
//
// 소스와 배포 매니페스트가 한 저장소에 함께 산다. CI 가 이미지를 만들어
// 레지스트리에 올린 뒤 deploy/ 의 태그를 갱신해 되쓰면, Argo CD 가 그 커밋을
// 감지해 배포한다.
//
// 이미지 저장 위치는 이 패키지가 정하지 않는다 — port.ImageTarget 으로 받는다.
// GitLab 프로젝트 레지스트리를 전제하면 Harbor 스택에서 엉뚱한 곳에 올린다.
package scaffold

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// InitialImageTag 는 첫 커밋 시점의 태그다. 아직 빌드된 이미지가 없으므로
	// CI 가 첫 실행에서 실제 커밋 SHA 로 치환한다.
	InitialImageTag = "bootstrap"

	// DeployTokenVar 는 GitLab CI 가 매니페스트를 되쓰기 위해 쓰는 토큰 변수다.
	// CI_JOB_TOKEN 은 저장소 push 권한이 없어 별도 토큰이 필요하다.
	//
	// GitHub 에는 대응물이 없다 — 내장 GITHUB_TOKEN 이 contents:write 로 push 한다.
	DeployTokenVar = "NULLUS_DEPLOY_TOKEN" // #nosec G101 -- CI 변수 이름

	// GitLabPipelinePath / GitHubWorkflowPath 는 각 플랫폼이 파이프라인 정의를
	// 읽는 위치다. 경로가 틀리면 파일은 커밋되지만 CI 가 영영 돌지 않는다.
	GitLabPipelinePath = ".gitlab-ci.yml"
	GitHubWorkflowPath = ".github/workflows/nullus-ci.yml"
	// JenkinsfilePath 는 Jenkins multibranch job 이 파이프라인 정의를 찾는
	// 위치다. 리포 루트가 아니면 job 이 "Jenkinsfile not found" 로 브랜치를
	// 통째로 건너뛴다.
	JenkinsfilePath = "Jenkinsfile"

	// githubActorVar / githubTokenVar 는 GitHub Actions 가 제공하는 값이다.
	// 이 이름이면 사용자 시크릿이 아니라 내장 표현식으로 바꿔 써야 한다.
	githubActorVar = "GITHUB_ACTOR"
	githubTokenVar = "GITHUB_TOKEN" // #nosec G101 -- CI 변수 이름

	// stackLabelKey / templateLabelKey 는 배포된 워크로드의 소속을 클러스터에서
	// 알아보게 하는 라벨이다. 조회하는 쪽(stack/adapter/handler/workloads.go)과
	// 문자열이 같아야 한다.
	stackLabelKey    = "nullus.io/stack-id"
	templateLabelKey = "nullus.io/cicd-template-id"

	defaultPort     = 8080
	defaultReplicas = 1

	// nginxDefaultPort 는 자리표시자 이미지가 기본으로 듣는 포트다.
	nginxDefaultPort = 80
)

// Input 은 스캐폴딩 렌더 요청이다.
type Input struct {
	AppName string
	// AppType 은 어떤 앱을 만들지다. web 이면 바로 도는 React 앱을 스캐폴딩한다.
	// 비면 예전처럼 자리만 잡는 nginx Dockerfile 을 만든다.
	AppType   domain.AppType
	Namespace string
	Port      int32
	Replicas  int32
	// Platform 은 소스 저장소 플랫폼이다. 비면 GitLab 으로 본다.
	Platform port.SCMPlatform
	// CIPlatform 은 파이프라인 정의를 어느 형식으로 쓸지 정한다.
	//
	// SCM 과 별개의 축이다 — Gitea 는 소스만 담당하고 빌드는 Jenkins 가 한다.
	// 비면 Platform 이 함의하는 기본 CI 를 쓴다.
	CIPlatform port.CIPlatform
	// ImageTarget 은 resolver 가 정한 이미지 저장 위치다.
	ImageTarget *port.ImageTarget
	// AccessDomain / GatewayName / GatewayNamespace 가 모두 있으면
	// 외부 접근용 HTTPRoute 를 함께 만든다. 없으면 클러스터 내부에서만
	// 접근 가능한 Service 까지만 만든다.
	AccessDomain     string
	GatewayName      string
	GatewayNamespace string
	// StackID 는 배포된 워크로드가 어느 스택 소속인지 클러스터에서 알아보게 한다.
	// 네임스페이스로는 판별할 수 없다 — 파이프라인이 default 에 깔 수도 있고
	// 여러 스택이 한 네임스페이스를 공유할 수도 있다. 비면 라벨을 붙이지 않는다.
	StackID string
	// TemplateID 는 어느 CI/CD 템플릿에서 나온 앱인지 알아보게 한다.
	// 템플릿별 자원 사용 비교나 템플릿 단위 조회에 쓴다. 비면 라벨을 붙이지 않는다.
	TemplateID string
}

// Render 는 앱 프로젝트에 커밋할 파일들을 만든다.
//
// 같은 입력이면 같은 결과다 — 재스캐폴딩이 의미 없는 diff 를 만들지 않도록
// 파일 경로를 정렬해 돌려준다.
func Render(in Input) ([]port.CommitFile, error) {
	app := strings.TrimSpace(in.AppName)
	if app == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	if in.ImageTarget == nil || strings.TrimSpace(in.ImageTarget.Repository) == "" {
		return nil, fmt.Errorf("image target is required (app=%q)", app)
	}

	namespace := strings.TrimSpace(in.Namespace)
	if namespace == "" {
		namespace = "default"
	}
	appPort := in.Port
	if appPort <= 0 {
		appPort = defaultPort
	}
	replicas := in.Replicas
	if replicas <= 0 {
		replicas = defaultReplicas
	}

	pipelinePath, pipelineContent := renderPipelineFor(in.Platform, in.CIPlatform, app, in.ImageTarget)

	files := []port.CommitFile{
		{Path: pipelinePath, Content: pipelineContent},
		{Path: "Dockerfile", Content: renderDockerfile(in.AppType, appPort)},
		{Path: "README.md", Content: renderReadme(in.Platform, app, namespace, in.ImageTarget)},
		{Path: "deploy/deployment.yaml", Content: renderDeployment(app, namespace, in.ImageTarget.Repository, appPort, replicas, in.StackID, in.TemplateID)},
		{Path: "deploy/service.yaml", Content: renderService(app, namespace, appPort, in.StackID, in.TemplateID)},
	}

	// 웹 앱이면 실제로 도는 소스를 함께 넣는다. 자리만 잡아 두면 배포는
	// 성공하는데 열어 보면 nginx 기본 페이지가 뜬다.
	if in.AppType == domain.AppTypeWeb {
		files = append(files, reactAppFiles(app, appPort)...)
	}

	// 게이트웨이 정보가 있으면 외부 접근 경로를 함께 만든다.
	// Service 만으로는 클러스터 밖에서 앱에 닿을 수 없다.
	if route, ok := renderHTTPRoute(app, namespace, appPort, in); ok {
		files = append(files, port.CommitFile{Path: "deploy/httproute.yaml", Content: route})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// renderPipelineFor 는 CI 플랫폼에 맞는 파이프라인 정의 파일을 만든다.
//
// 분기 기준은 SCM 이 아니라 CI 다. GitLab·GitHub 은 SCM 이 CI 를 겸해 둘이
// 같아 보이지만, Gitea 는 소스만 담당하고 빌드는 Jenkins 가 한다 — SCM 으로
// 분기하면 Gitea 스택에 .gitlab-ci.yml 이 깔리고 Jenkins 는 읽을 파일이 없다.
func renderPipelineFor(
	platform port.SCMPlatform,
	ci port.CIPlatform,
	app string,
	target *port.ImageTarget,
) (path, content string) {
	if strings.TrimSpace(string(ci)) == "" {
		ci = port.DefaultCIPlatformFor(platform)
	}
	switch ci {
	case port.CIPlatformJenkins:
		return JenkinsfilePath, renderJenkinsfile(app, target)
	case port.CIPlatformGitHubActions:
		return GitHubWorkflowPath, renderGitHubWorkflow(app, target)
	default:
		return GitLabPipelinePath, renderPipeline(app, target)
	}
}

// renderGitHubWorkflow 는 build → deploy 2단계 GitHub Actions 워크플로를 만든다.
//
// GitLab 판과 세 가지가 다르다.
//
// 하나, dind 가 필요 없다. GitHub 호스티드 러너에는 Docker 데몬이 이미 떠 있다.
//
// 둘, 되쓰기 토큰이 필요 없다. contents:write 권한의 내장 GITHUB_TOKEN 을
// actions/checkout 이 remote 자격증명으로 심어두므로 git push 가 그대로 된다.
//
// 셋, [skip ci] 가 필요 없다. GITHUB_TOKEN 으로 만든 push 는 워크플로를 다시
// 트리거하지 않는다 — GitHub 이 무한 루프를 막기 위해 그렇게 정의했다.
func renderGitHubWorkflow(app string, target *port.ImageTarget) string {
	var b strings.Builder

	b.WriteString("# Nullus 가 생성한 파이프라인입니다.\n")
	b.WriteString("# build: 이미지를 만들어 레지스트리에 올립니다.\n")
	b.WriteString("# deploy: deploy/ 의 이미지 태그를 갱신해 커밋합니다. 배포는 Argo CD 가 합니다.\n\n")

	fmt.Fprintf(&b, "name: nullus-ci-%s\n\n", app)

	b.WriteString("on:\n  push:\n    branches:\n      - main\n      - master\n\n")

	// 최소 권한만 준다. contents 는 매니페스트 되쓰기, packages 는 GHCR push 용이다.
	b.WriteString("permissions:\n  contents: write\n  packages: write\n\n")

	// 같은 브랜치에 연속 push 가 들어오면 뒤 실행이 앞 실행의 커밋을 덮어쓸 수
	// 있다. 취소하지 않고 직렬화한다 — 빌드는 끝까지 가야 이미지가 남는다.
	b.WriteString("concurrency:\n")
	b.WriteString("  group: nullus-ci-${{ github.ref }}\n")
	b.WriteString("  cancel-in-progress: false\n\n")

	b.WriteString("env:\n")
	fmt.Fprintf(&b, "  IMAGE_REPOSITORY: %q\n", target.Repository)
	fmt.Fprintf(&b, "  REGISTRY_HOST: %q\n", target.Host)
	b.WriteString("  IMAGE_TAG: ${{ github.sha }}\n\n")

	b.WriteString("jobs:\n")

	b.WriteString("  build:\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n")
	b.WriteString("      - name: Log in to the container registry\n")
	b.WriteString("        env:\n")
	fmt.Fprintf(&b, "          REGISTRY_USERNAME: %s\n", githubExpressionFor(target.UsernameVar))
	fmt.Fprintf(&b, "          REGISTRY_PASSWORD: %s\n", githubExpressionFor(target.PasswordVar))
	b.WriteString("        run: |\n")
	b.WriteString("          echo \"$REGISTRY_PASSWORD\" | docker login \"$REGISTRY_HOST\" -u \"$REGISTRY_USERNAME\" --password-stdin\n")
	b.WriteString("      - name: Build and push image\n")
	b.WriteString("        run: |\n")
	b.WriteString("          docker build -t \"$IMAGE_REPOSITORY:$IMAGE_TAG\" .\n")
	b.WriteString("          docker push \"$IMAGE_REPOSITORY:$IMAGE_TAG\"\n\n")

	b.WriteString("  deploy:\n")
	b.WriteString("    needs: build\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n")
	b.WriteString("      - name: Update manifest image tag\n")
	b.WriteString("        run: |\n")
	// 매니페스트의 태그만 바꾼다. 저장소 경로는 고정이라 안전하게 매칭된다.
	b.WriteString("          sed -i \"s#image: $IMAGE_REPOSITORY:.*#image: $IMAGE_REPOSITORY:$IMAGE_TAG#\" deploy/deployment.yaml\n")
	b.WriteString("          git config user.email \"ci@nullus.local\"\n")
	b.WriteString("          git config user.name \"Nullus CI\"\n")
	b.WriteString("          git add deploy/deployment.yaml\n")
	// 변경이 없으면 커밋이 실패하므로 무변경을 정상 종료로 처리한다.
	b.WriteString("          git diff --cached --quiet && echo \"변경 없음\" && exit 0\n")
	b.WriteString("          git commit -m \"chore(deploy): $IMAGE_TAG\"\n")
	b.WriteString("          git push\n")

	return b.String()
}

// githubExpressionFor 는 변수 이름을 워크플로에서 읽을 표현식으로 바꾼다.
//
// 내장 값은 secrets 에 없다. GITHUB_ACTOR 를 ${{ secrets.GITHUB_ACTOR }} 로
// 쓰면 빈 문자열이 들어가 docker login 이 사용자명 없이 실행된다.
func githubExpressionFor(varName string) string {
	switch strings.ToUpper(strings.TrimSpace(varName)) {
	case githubActorVar:
		return "${{ github.actor }}"
	case githubTokenVar:
		return "${{ secrets.GITHUB_TOKEN }}"
	default:
		return "${{ secrets." + strings.ToUpper(strings.TrimSpace(varName)) + " }}"
	}
}

// renderPipeline 은 build → deploy 2단계 파이프라인을 만든다.
//
// deploy 단계는 배포하지 않는다. 매니페스트의 이미지 태그만 갱신해 커밋하고,
// 실제 배포는 Argo CD 가 그 커밋을 보고 수행한다(GitOps).
func renderPipeline(app string, target *port.ImageTarget) string {
	var b strings.Builder

	b.WriteString("# Nullus 가 생성한 파이프라인입니다.\n")
	b.WriteString("# build: 이미지를 만들어 레지스트리에 올립니다.\n")
	b.WriteString("# deploy: deploy/ 의 이미지 태그를 갱신해 커밋합니다. 배포는 Argo CD 가 합니다.\n\n")

	b.WriteString("stages:\n  - build\n  - deploy\n\n")

	b.WriteString("variables:\n")
	fmt.Fprintf(&b, "  IMAGE_REPOSITORY: %q\n", target.Repository)
	fmt.Fprintf(&b, "  REGISTRY_HOST: %q\n", target.Host)
	b.WriteString("  IMAGE_TAG: $CI_COMMIT_SHORT_SHA\n")
	// docker CLI 는 기본적으로 유닉스 소켓을 찾는데 job 컨테이너에는 데몬이 없다.
	// Kubernetes executor 는 서비스와 네트워크를 공유하므로 dind 를 TCP 로 가리킨다.
	// DOCKER_TLS_CERTDIR 를 비우지 않으면 dind 가 TLS(2376)로만 열려 2375 연결이 거부된다.
	b.WriteString("  DOCKER_HOST: tcp://docker:2375\n")
	b.WriteString("  DOCKER_TLS_CERTDIR: \"\"\n\n")

	b.WriteString("build:\n")
	b.WriteString("  stage: build\n")
	b.WriteString("  image: docker:27\n")
	// 클러스터 내부 레지스트리는 보통 HTTP 로 노출된다. docker 는 기본적으로
	// HTTPS 를 요구하므로 대상 호스트를 명시하지 않으면 push 가 TLS 오류로 죽는다.
	b.WriteString("  services:\n")
	b.WriteString("    - name: docker:27-dind\n")
	fmt.Fprintf(&b, "      command: [\"--insecure-registry=%s\"]\n", target.Host)
	b.WriteString("  script:\n")
	writeScriptLines(&b, []string{
		// dind 는 즉시 뜨지 않는다. 기다리지 않으면 첫 docker 명령이 데몬 없이
		// 실행돼 엉뚱한 오류로 죽고, 원인이 레지스트리 문제처럼 보인다.
		`for i in $(seq 1 60); do docker info >/dev/null 2>&1 && break; sleep 2; done`,
		`docker info >/dev/null 2>&1 || { echo "docker 데몬(dind)에 연결하지 못했습니다"; exit 1; }`,
		fmt.Sprintf(`echo "$%s" | docker login "$REGISTRY_HOST" -u "$%s" --password-stdin`,
			target.PasswordVar, target.UsernameVar),
		`docker build -t "$IMAGE_REPOSITORY:$IMAGE_TAG" .`,
		`docker push "$IMAGE_REPOSITORY:$IMAGE_TAG"`,
	})
	b.WriteString("\n")

	b.WriteString("deploy:\n")
	b.WriteString("  stage: deploy\n")
	b.WriteString("  image: alpine:3.20\n")
	b.WriteString("  needs:\n    - build\n")
	b.WriteString("  script:\n")
	writeScriptLines(&b, []string{
		`apk add --no-cache git`,
		`git config user.email "ci@nullus.local"`,
		`git config user.name "Nullus CI"`,
		// 매니페스트의 태그만 바꾼다. 저장소 경로는 고정이라 안전하게 매칭된다.
		`sed -i "s#image: $IMAGE_REPOSITORY:.*#image: $IMAGE_REPOSITORY:$IMAGE_TAG#" deploy/deployment.yaml`,
		`git add deploy/deployment.yaml`,
		// 변경이 없으면 커밋이 실패하므로 무변경을 정상 종료로 처리한다.
		`git diff --cached --quiet && echo "변경 없음" && exit 0`,
		// [skip ci] 가 없으면 되쓰기 커밋이 파이프라인을 다시 돌려 무한 루프가 된다.
		`git commit -m "chore(deploy): $IMAGE_TAG [skip ci]"`,
		// 프로토콜·포트를 하드코딩하지 않는다. 게이트웨이가 HTTP 로만 노출된
		// 구성에서 https 로 밀면 연결 자체가 되지 않는다.
		fmt.Sprintf(`git push "${CI_SERVER_PROTOCOL}://oauth2:$%s@${CI_SERVER_HOST}:${CI_SERVER_PORT}/${CI_PROJECT_PATH}.git" HEAD:$CI_COMMIT_REF_NAME`,
			DeployTokenVar),
	})
	b.WriteString("  rules:\n")
	b.WriteString("    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH\n")

	return b.String()
}

// writeScriptLines 는 script 항목을 YAML 안전하게 적는다.
//
// 셸 명령에는 `chore(deploy): ` 처럼 콜론+공백이 들어갈 수 있는데, 인용 없이
// 적으면 YAML 이 그 지점을 매핑으로 읽어 파일 전체가 깨진다.
// 명령들은 내부에 큰따옴표를 쓰므로 작은따옴표로 감싼다.
func writeScriptLines(b *strings.Builder, lines []string) {
	for _, line := range lines {
		fmt.Fprintf(b, "    - '%s'\n", strings.ReplaceAll(line, "'", "''"))
	}
}

// renderDockerfile 은 자리표시자 Dockerfile 을 만든다.
//
// nginx 기본 설정은 80 에서 듣는데 Service·Deployment·HTTPRoute 는 사용자가 고른
// 포트를 가리킨다. EXPOSE 는 문서일 뿐 바인딩을 바꾸지 않으므로, 설정을 함께
// 고치지 않으면 첫 배포가 "파드는 Running 인데 아무도 응답하지 않는" 상태로 끝난다
// — 자리표시자에는 readinessProbe 도 없어 Argo CD 까지 Healthy 로 보고한다.
func renderDockerfile(appType domain.AppType, appPort int32) string {
	if appType == domain.AppTypeWeb {
		return reactDockerfile(appPort)
	}
	header := `# Nullus 가 생성한 기본 Dockerfile 입니다. 애플리케이션에 맞게 수정하세요.
# 조직 공용 베이스 이미지를 쓰려면 common 프로젝트의 이미지를 FROM 에 지정하면 됩니다.
FROM nginx:alpine
`
	// 80 이면 기본 설정 그대로다. 불필요한 치환을 넣으면 사용자가 이미지를 바꿀 때
	// 지워야 할 것만 늘어난다.
	if appPort == nginxDefaultPort {
		return fmt.Sprintf("%s\nEXPOSE %d\n", header, appPort)
	}

	// 기본 설정을 통째로 바꿔 쓴다. sed 치환은 listen 지시자의 공백 폭과
	// IPv6 줄이 이미지 버전마다 달라 조용히 어긋날 수 있고, 사용자가 읽고
	// 고치기도 어렵다.
	return fmt.Sprintf(`%s
# nginx 기본 설정은 80 을 듣는다. 배포 매니페스트가 가리키는 포트로 맞춘다 —
# EXPOSE 는 문서일 뿐 바인딩을 바꾸지 않는다.
RUN printf 'server {\n    listen %d;\n    location / {\n        root  /usr/share/nginx/html;\n        index index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf

EXPOSE %d
`, header, appPort, appPort)
}

// ownerLabelLines 는 들여쓰기에 맞춘 소유 라벨들이다.
//
// 값이 빈 라벨은 아예 쓰지 않는다 — 빈 값 라벨("")은 쿠버네티스에서 유효하지만
// "소속 없음" 과 구분되지 않아, 라벨 셀렉터가 의도치 않게 걸린다.
//
// 라벨 값에 쓰는 것은 항상 id 다. 이름은 바뀌면 이미 떠 있는 파드와 어긋나고,
// 템플릿 이름처럼 공백·em dash 가 든 값은 라벨 값으로 유효하지도 않다.
func ownerLabelLines(stackID, templateID, indent string) string {
	var b strings.Builder
	for _, label := range []struct{ key, value string }{
		{stackLabelKey, stackID},
		{templateLabelKey, templateID},
	} {
		value := strings.TrimSpace(label.value)
		if value == "" {
			continue
		}
		fmt.Fprintf(&b, "\n%s%s: %s", indent, label.key, value)
	}
	return b.String()
}

func renderDeployment(app, namespace, repository string, appPort, replicas int32, stackID, templateID string) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app.kubernetes.io/name: %[1]s
    app.kubernetes.io/managed-by: nullus-cicd%[8]s
spec:
  replicas: %[5]d
  selector:
    matchLabels:
      app.kubernetes.io/name: %[1]s
  template:
    metadata:
      labels:
        app.kubernetes.io/name: %[1]s
        app.kubernetes.io/managed-by: nullus-cicd%[9]s
    spec:
      # private 레지스트리는 익명 pull 을 거부한다. Nullus 가 이 이름으로
      # 자격증명 Secret 을 네임스페이스에 만들어 둔다.
      imagePullSecrets:
        - name: %[7]s
      containers:
        - name: %[1]s
          # 이 태그는 CI 의 deploy 단계가 갱신합니다.
          image: %[3]s:%[6]s
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: %[4]d
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
`, app, namespace, repository, appPort, replicas, InitialImageTag, kube.ImagePullSecretName,
		ownerLabelLines(stackID, templateID, "    "), ownerLabelLines(stackID, templateID, "        "))
}

func renderService(app, namespace string, appPort int32, stackID, templateID string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app.kubernetes.io/name: %[1]s
    app.kubernetes.io/managed-by: nullus-cicd%[4]s
spec:
  selector:
    app.kubernetes.io/name: %[1]s
  ports:
    - name: http
      port: %[3]d
      targetPort: http
`, app, namespace, appPort, ownerLabelLines(stackID, templateID, "    "))
}

func renderReadme(platform port.SCMPlatform, app, namespace string, target *port.ImageTarget) string {
	var b strings.Builder
	isGitHub := platform == port.SCMPlatformGitHub

	b.WriteString("# " + app + "\n\n")
	b.WriteString("Nullus 가 생성한 애플리케이션 프로젝트입니다.\n")
	b.WriteString("소스와 배포 매니페스트가 이 저장소에 함께 있습니다.\n\n")

	pipelinePath := GitLabPipelinePath
	if isGitHub {
		pipelinePath = GitHubWorkflowPath
	}

	b.WriteString("## 흐름\n\n")
	b.WriteString("1. 기본 브랜치에 push\n")
	b.WriteString("2. CI `build` — 이미지를 만들어 `" + target.Repository + "` 에 올림\n")
	b.WriteString("3. CI `deploy` — `deploy/deployment.yaml` 의 태그를 갱신해 커밋\n")
	b.WriteString("4. Argo CD 가 그 커밋을 감지해 `" + namespace + "` 네임스페이스에 배포\n\n")
	b.WriteString("파이프라인 정의는 `" + pipelinePath + "` 에 있습니다.\n\n")

	b.WriteString("## 이미지 레지스트리\n\n")
	fmt.Fprintf(&b, "- 종류: `%s`\n", target.Kind)
	fmt.Fprintf(&b, "- 저장소: `%s`\n\n", target.Repository)

	if len(target.RequiredVariables) > 0 {
		b.WriteString("### 필요한 CI/CD 변수\n\n")
		if isGitHub {
			b.WriteString("아래 값을 저장소의 **Settings → Secrets and variables → Actions** 에\n")
			b.WriteString("등록해야 빌드가 레지스트리에 로그인할 수 있습니다.\n\n")
		} else {
			b.WriteString("아래 변수를 프로젝트의 **Settings → CI/CD → Variables 에 등록**해야\n")
			b.WriteString("빌드가 레지스트리에 로그인할 수 있습니다.\n\n")
		}
		for _, v := range target.RequiredVariables {
			b.WriteString("- `" + v + "`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("### 매니페스트 되쓰기 권한\n\n")
	if isGitHub {
		// 여기에 PAT 를 등록하라고 적으면 사용자가 불필요한 장기 토큰을
		// 워크플로에 노출한다. 내장 토큰으로 충분하다는 점을 분명히 한다.
		b.WriteString("추가 토큰이 필요 없습니다. 워크플로의 `permissions: contents: write` 로\n")
		b.WriteString("내장 `GITHUB_TOKEN` 이 커밋을 push 합니다.\n")
		b.WriteString("이 push 는 워크플로를 다시 트리거하지 않으므로 무한 루프가 생기지 않습니다.\n\n")
	} else {
		b.WriteString("`deploy` 단계가 저장소에 커밋하려면 쓰기 권한 토큰이 필요합니다.\n")
		b.WriteString("`" + DeployTokenVar + "` 변수에 `write_repository` 스코프 토큰을 등록하세요.\n\n")
	}

	b.WriteString("---\n\n")
	b.WriteString("이 파일은 Nullus 가 생성했습니다. 프로비저닝이 다시 실행되면 덮어써집니다.\n")
	return b.String()
}

// renderHTTPRoute 는 게이트웨이에 앱을 붙이는 라우트를 만든다.
//
// 게이트웨이는 스택 네임스페이스에 있고 앱은 자기 네임스페이스에 있으므로
// parentRef 가 네임스페이스를 넘는다. 게이트웨이의 allowedRoutes 가
// from: All 이어야 붙는다.
func renderHTTPRoute(app, namespace string, appPort int32, in Input) (string, bool) {
	domain := strings.TrimSpace(in.AccessDomain)
	gwName := strings.TrimSpace(in.GatewayName)
	gwNS := strings.TrimSpace(in.GatewayNamespace)
	if domain == "" || gwName == "" || gwNS == "" {
		return "", false
	}

	return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app.kubernetes.io/name: %[1]s
    app.kubernetes.io/managed-by: nullus-cicd
spec:
  parentRefs:
    - name: %[3]s
      namespace: %[4]s
  hostnames:
    - %[1]s.%[5]s
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: %[1]s
          port: %[6]d
`, app, namespace, gwName, gwNS, domain, appPort), true
}

// PipelineStageNames 는 스캐폴딩이 만드는 파이프라인의 단계 이름이다.
//
// 템플릿이 선언하는 stages 의 출처다. 손으로 적으면 실제와 어긋나고, 어긋나면
// 화면이 돌지도 않은 단계를 보여준다 — 실제로 템플릿은 Build/Test/ImageBuild/
// Deploy 4단계를 선언했지만 스캐폴딩은 2단계만 만들었다.
//
// GitLab 판과 Jenkins 판이 같은 단계를 만든다. 이름 표기만 다르므로(소문자 대
// 대문자) 여기서는 표시용 표기를 쓴다.
func PipelineStageNames() []string {
	return []string{"Build", "Deploy"}
}

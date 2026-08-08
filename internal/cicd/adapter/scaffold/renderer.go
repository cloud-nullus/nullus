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
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// InitialImageTag 는 첫 커밋 시점의 태그다. 아직 빌드된 이미지가 없으므로
	// CI 가 첫 실행에서 실제 커밋 SHA 로 치환한다.
	InitialImageTag = "bootstrap"

	// DeployTokenVar 는 CI 가 매니페스트를 되쓰기 위해 쓰는 토큰 변수다.
	// CI_JOB_TOKEN 은 저장소 push 권한이 없어 별도 토큰이 필요하다.
	DeployTokenVar = "NULLUS_DEPLOY_TOKEN" // #nosec G101 -- CI 변수 이름

	defaultPort     = 8080
	defaultReplicas = 1
)

// Input 은 스캐폴딩 렌더 요청이다.
type Input struct {
	AppName   string
	Namespace string
	Port      int32
	Replicas  int32
	// ImageTarget 은 resolver 가 정한 이미지 저장 위치다.
	ImageTarget *port.ImageTarget
	// AccessDomain / GatewayName / GatewayNamespace 가 모두 있으면
	// 외부 접근용 HTTPRoute 를 함께 만든다. 없으면 클러스터 내부에서만
	// 접근 가능한 Service 까지만 만든다.
	AccessDomain     string
	GatewayName      string
	GatewayNamespace string
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

	files := []port.CommitFile{
		{Path: ".gitlab-ci.yml", Content: renderPipeline(app, in.ImageTarget)},
		{Path: "Dockerfile", Content: renderDockerfile(appPort)},
		{Path: "README.md", Content: renderReadme(app, namespace, in.ImageTarget)},
		{Path: "deploy/deployment.yaml", Content: renderDeployment(app, namespace, in.ImageTarget.Repository, appPort, replicas)},
		{Path: "deploy/service.yaml", Content: renderService(app, namespace, appPort)},
	}

	// 게이트웨이 정보가 있으면 외부 접근 경로를 함께 만든다.
	// Service 만으로는 클러스터 밖에서 앱에 닿을 수 없다.
	if route, ok := renderHTTPRoute(app, namespace, appPort, in); ok {
		files = append(files, port.CommitFile{Path: "deploy/httproute.yaml", Content: route})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
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

func renderDockerfile(appPort int32) string {
	return fmt.Sprintf(`# Nullus 가 생성한 기본 Dockerfile 입니다. 애플리케이션에 맞게 수정하세요.
# 조직 공용 베이스 이미지를 쓰려면 common 프로젝트의 이미지를 FROM 에 지정하면 됩니다.
FROM nginx:alpine

EXPOSE %d
`, appPort)
}

func renderDeployment(app, namespace, repository string, appPort, replicas int32) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app.kubernetes.io/name: %[1]s
    app.kubernetes.io/managed-by: nullus-cicd
spec:
  replicas: %[5]d
  selector:
    matchLabels:
      app.kubernetes.io/name: %[1]s
  template:
    metadata:
      labels:
        app.kubernetes.io/name: %[1]s
        app.kubernetes.io/managed-by: nullus-cicd
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
`, app, namespace, repository, appPort, replicas, InitialImageTag, kube.ImagePullSecretName)
}

func renderService(app, namespace string, appPort int32) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app.kubernetes.io/name: %[1]s
    app.kubernetes.io/managed-by: nullus-cicd
spec:
  selector:
    app.kubernetes.io/name: %[1]s
  ports:
    - name: http
      port: %[3]d
      targetPort: http
`, app, namespace, appPort)
}

func renderReadme(app, namespace string, target *port.ImageTarget) string {
	var b strings.Builder

	b.WriteString("# " + app + "\n\n")
	b.WriteString("Nullus 가 생성한 애플리케이션 프로젝트입니다.\n")
	b.WriteString("소스와 배포 매니페스트가 이 저장소에 함께 있습니다.\n\n")

	b.WriteString("## 흐름\n\n")
	b.WriteString("1. 기본 브랜치에 push\n")
	b.WriteString("2. CI `build` — 이미지를 만들어 `" + target.Repository + "` 에 올림\n")
	b.WriteString("3. CI `deploy` — `deploy/deployment.yaml` 의 태그를 갱신해 커밋\n")
	b.WriteString("4. Argo CD 가 그 커밋을 감지해 `" + namespace + "` 네임스페이스에 배포\n\n")

	b.WriteString("## 이미지 레지스트리\n\n")
	fmt.Fprintf(&b, "- 종류: `%s`\n", target.Kind)
	fmt.Fprintf(&b, "- 저장소: `%s`\n\n", target.Repository)

	if len(target.RequiredVariables) > 0 {
		b.WriteString("### 필요한 CI/CD 변수\n\n")
		b.WriteString("아래 변수를 프로젝트의 **Settings → CI/CD → Variables 에 등록**해야\n")
		b.WriteString("빌드가 레지스트리에 로그인할 수 있습니다.\n\n")
		for _, v := range target.RequiredVariables {
			b.WriteString("- `" + v + "`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("### 매니페스트 되쓰기 토큰\n\n")
	b.WriteString("`deploy` 단계가 저장소에 커밋하려면 쓰기 권한 토큰이 필요합니다.\n")
	b.WriteString("`" + DeployTokenVar + "` 변수에 `write_repository` 스코프 토큰을 등록하세요.\n\n")

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

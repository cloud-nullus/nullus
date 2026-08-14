package scaffold

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// jenkinsAgentImage 는 빌드가 도는 컨테이너다. dind 사이드카에 붙어
// docker build/push 를 한다 — Jenkins 는 GitLab CI 와 달리 실행기를 별도
// 차트로 세우지 않고 kubernetes 플러그인이 이 파드를 빌드마다 띄운다.
const (
	jenkinsAgentImage = "docker:27-cli"
	jenkinsDindImage  = "docker:27-dind"
)

// renderJenkinsfile 은 build → deploy 2단계 선언적 파이프라인을 만든다.
//
// GitLab 판과 세 가지가 다르다.
//
// 하나, 실행기를 우리가 정의한다. GitLab 은 러너가 이미 떠 있지만 Jenkins 는
// kubernetes 플러그인이 이 podTemplate 대로 agent 파드를 매번 띄운다.
//
// 둘, 자격증명이 Jenkins Credentials 가 아니라 환경변수로 들어온다. OpenBao →
// ESO → K8s Secret 평면이 나르고 agent 파드가 secretKeyRef 로 받는다.
// Jenkins Credentials 를 1차 저장소로 쓰면 자격증명 사본이 하나 더 생기고
// 회전 경로가 둘로 갈린다.
//
// 셋, 되커밋 루프를 두 겹으로 막는다. GitLab 은 [skip ci] 하나로 끊지만
// Jenkins multibranch 는 그 관례를 자동 인식하지 않는다 — 커밋 메시지 규약에
// 더해 changeset 조건으로 "매니페스트만 바뀐 커밋" 을 걸러 낸다.
//
// 배포는 하지 않는다. 이미지 태그를 매니페스트에 되쓰고 Argo CD 가 그 커밋을
// 동기화한다 (cicd-golden-path.md 가 고른 Git + Argo CD 방식).
func renderJenkinsfile(app string, target *port.ImageTarget) string {
	var b strings.Builder

	b.WriteString("// Nullus 가 생성한 파이프라인입니다.\n")
	b.WriteString("// 빌드한 이미지를 레지스트리에 올린 뒤 deploy/ 의 태그를 갱신해 되커밋합니다.\n")
	b.WriteString("// 배포는 Argo CD 가 그 커밋을 동기화하며 수행합니다.\n\n")

	b.WriteString("pipeline {\n")
	b.WriteString("  agent {\n")
	b.WriteString("    kubernetes {\n")
	b.WriteString("      yaml '''\n")
	b.WriteString("apiVersion: v1\n")
	b.WriteString("kind: Pod\n")
	b.WriteString("spec:\n")
	b.WriteString("  containers:\n")
	b.WriteString("    - name: builder\n")
	fmt.Fprintf(&b, "      image: %s\n", jenkinsAgentImage)
	b.WriteString("      command: [\"cat\"]\n")
	b.WriteString("      tty: true\n")
	b.WriteString("      env:\n")
	b.WriteString("        - name: DOCKER_HOST\n")
	b.WriteString("          value: tcp://localhost:2375\n")
	b.WriteString("      envFrom:\n")
	b.WriteString("        # ESO 가 OpenBao 에서 동기화한 파이프라인 자격증명이다.\n")
	fmt.Fprintf(&b, "        - secretRef: {name: %s}\n", ciSecretName(app))
	b.WriteString("    - name: dind\n")
	fmt.Fprintf(&b, "      image: %s\n", jenkinsDindImage)
	b.WriteString("      securityContext: {privileged: true}\n")
	b.WriteString("      env:\n")
	b.WriteString("        - name: DOCKER_TLS_CERTDIR\n")
	b.WriteString("          value: \"\"\n")
	if host := strings.TrimSpace(target.Host); host != "" {
		b.WriteString("      args:\n")
		// 사설 레지스트리가 자체 서명 인증서를 쓰면 push 가 x509 로 막힌다.
		fmt.Fprintf(&b, "        - --insecure-registry=%s\n", host)
	}
	b.WriteString("'''\n")
	b.WriteString("    }\n")
	b.WriteString("  }\n\n")

	b.WriteString("  options {\n")
	// 같은 브랜치의 빌드가 겹치면 태그 되커밋이 서로를 덮어쓴다.
	b.WriteString("    disableConcurrentBuilds()\n")
	b.WriteString("  }\n\n")

	b.WriteString("  environment {\n")
	fmt.Fprintf(&b, "    IMAGE_REPOSITORY = '%s'\n", target.Repository)
	fmt.Fprintf(&b, "    REGISTRY_HOST = '%s'\n", target.Host)
	// GIT_COMMIT 은 multibranch 가 주입한다. 짧은 SHA 로 태그를 만든다.
	b.WriteString("    IMAGE_TAG = \"${env.GIT_COMMIT.take(8)}\"\n")
	b.WriteString("  }\n\n")

	b.WriteString("  stages {\n")

	// build
	b.WriteString("    stage('Build') {\n")
	b.WriteString("      when {\n")
	b.WriteString("        beforeAgent true\n")
	b.WriteString("        allOf {\n")
	b.WriteString("          branch 'main'\n")
	b.WriteString("          // 되커밋은 deploy/ 만 바꾼다. 그 변경만으로 다시 빌드하면\n")
	b.WriteString("          // 무한 루프가 된다 — [skip ci] 를 인식하지 않는 multibranch 를\n")
	b.WriteString("          // 위한 두 번째 방어선이다.\n")
	b.WriteString("          not { changeset pattern: 'deploy/**' , comparator: 'ANT' }\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("      steps {\n")
	b.WriteString("        container('builder') {\n")
	b.WriteString("          sh '''\n")
	b.WriteString("            set -eu\n")
	b.WriteString("            for i in $(seq 1 60); do docker info >/dev/null 2>&1 && break; sleep 2; done\n")
	b.WriteString("            docker info >/dev/null 2>&1 || { echo \"docker 데몬(dind)에 연결하지 못했습니다\"; exit 1; }\n")
	fmt.Fprintf(&b, "            echo \"$%s\" | docker login \"$REGISTRY_HOST\" -u \"$%s\" --password-stdin\n",
		target.PasswordVar, target.UsernameVar)
	b.WriteString("            docker build -t \"$IMAGE_REPOSITORY:$IMAGE_TAG\" .\n")
	b.WriteString("            docker push \"$IMAGE_REPOSITORY:$IMAGE_TAG\"\n")
	b.WriteString("          '''\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n\n")

	// deploy — 매니페스트 태그 갱신 후 되커밋
	b.WriteString("    stage('Deploy') {\n")
	b.WriteString("      when {\n")
	b.WriteString("        beforeAgent true\n")
	b.WriteString("        allOf {\n")
	b.WriteString("          branch 'main'\n")
	b.WriteString("          not { changeset pattern: 'deploy/**' , comparator: 'ANT' }\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("      steps {\n")
	b.WriteString("        container('builder') {\n")
	b.WriteString("          sh '''\n")
	b.WriteString("            set -eu\n")
	b.WriteString("            apk add --no-cache git >/dev/null 2>&1 || true\n")
	// 워크스페이스는 체크아웃을 한 jnlp 컨테이너(uid 1000)의 소유인데 이
	// 컨테이너는 root 로 돈다. git 2.35.2+ 는 다른 사용자 소유의 저장소를
	// 거부하므로, 풀어 주지 않으면 이어지는 git config 가
	// "fatal: not in a git directory" 로 죽는다 — 이미지는 올라갔는데
	// 매니페스트 되커밋만 실패해 Argo CD 가 배포할 새 커밋이 영영 없다.
	b.WriteString("            git config --global --add safe.directory \"$PWD\"\n")
	b.WriteString("            sed -i \"s#image: $IMAGE_REPOSITORY:.*#image: $IMAGE_REPOSITORY:$IMAGE_TAG#\" deploy/deployment.yaml\n")
	b.WriteString("            git config user.email \"nullus-ci@local\"\n")
	b.WriteString("            git config user.name \"Nullus CI\"\n")
	b.WriteString("            git add deploy/deployment.yaml\n")
	b.WriteString("            git diff --cached --quiet && echo \"변경 없음\" && exit 0\n")
	// [skip ci] 는 사람이 로그에서 원인을 알아보게 하는 표식이자, webhook 을
	// 거르는 쪽에서 쓰는 관례다.
	b.WriteString("            git commit -m \"chore(deploy): $IMAGE_TAG [skip ci]\"\n")
	fmt.Fprintf(&b,
		"            git push \"$(echo \"$GIT_URL\" | sed \"s#://#://$%s:$%s@#\")\" HEAD:main\n",
		GitUsernameVar, GitPasswordVar)
	b.WriteString("          '''\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n")

	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

const (
	// GitUsernameVar / GitPasswordVar 는 매니페스트 되커밋에 쓰는 자격증명이다.
	// ESO 가 만든 파이프라인 Secret 에서 환경변수로 들어온다.
	//
	// 자격증명을 채우는 쪽(cicd/usecase 의 configureGiteaPipeline)이 같은
	// 이름을 써야 한다 — 갈라지면 push 가 인증 실패로 죽는다.
	GitUsernameVar = "GIT_USERNAME"
	GitPasswordVar = "GIT_PASSWORD" // #nosec G101 -- CI 변수 이름
)

// ciSecretName 은 이 파이프라인의 자격증명 Secret 이름이다.
//
// Argo CD 리포 Secret 의 기존 규약(nullus-repo-<app>)과 짝을 맞춘다.
// 파이프라인 단위라 하나의 자격증명 유출이 다른 파이프라인으로 번지지 않고,
// 파이프라인을 지울 때 함께 지울 수 있다.
func ciSecretName(app string) string {
	return "nullus-ci-" + strings.TrimSpace(app)
}

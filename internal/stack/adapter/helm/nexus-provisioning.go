package helm

// Nexus 는 설치만으로는 쓸 수 없다.
//
// 차트는 웹 UI(8081)만 띄운다. Docker 레지스트리는 별도 HTTP 커넥터를 열어야
// 생기고, 저장소도 하나도 없으며, 관리자 비밀번호는 첫 기동 때 컨테이너가
// 무작위로 만들어 PVC 안(/nexus-data/admin.password)에 둔다.
//
// 이 단계가 없으면 CI 가 이미지를 올릴 곳이 존재하지 않는다 — 설치는 성공했는데
// 파이프라인만 실패하는, 원인이 멀리 떨어진 실패가 된다.

import (
	"context"
	"fmt"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const (
	nexusProvisionJobName = "nullus-nexus-bootstrap"
	// nexusBootstrapPasswordPath 는 컨테이너가 첫 기동 때 무작위 비밀번호를
	// 적어 두는 곳이다. 비밀번호를 바꾸고 나면 컨테이너가 이 파일을 지운다.
	nexusBootstrapPasswordPath = "/nexus-data/admin.password" // #nosec G101 -- 컨테이너 내부 경로
)

// ensureNexusProvisioned 는 Nexus 를 실제로 쓸 수 있는 상태로 만든다.
func (o *Orchestrator) ensureNexusProvisioned(ctx context.Context, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultStackNamespace
	}

	// 저장소 API 는 기동이 끝나야 응답한다. 여기서 기다리지 않으면 아래 Job 이
	// connection refused 로 죽고, 원인이 "Nexus 프로비저닝 실패" 로만 남는다.
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=available", "--timeout=600s",
		"deployment/"+domain.NexusServiceName); err != nil {
		return fmt.Errorf("nexus 기동을 기다리지 못했습니다: %w", err)
	}

	targetPassword, err := o.readSecretValue(ctx, namespace, domain.NexusAdminSecret, domain.NexusAdminPassKey)
	if err != nil {
		return fmt.Errorf("nexus 관리자 비밀번호를 읽지 못했습니다 (%s): %w", domain.NexusAdminSecret, err)
	}

	bootstrapPassword, err := o.readNexusBootstrapPassword(ctx, namespace)
	if err != nil {
		return err
	}

	if err := o.runNexusBootstrapJob(ctx, namespace, bootstrapPassword, targetPassword); err != nil {
		return err
	}

	// Docker 커넥터는 차트 Service 에 없는 포트라 직접 노출한다.
	if err := o.applyManifest(ctx, namespace, nexusDockerServiceManifest(namespace)); err != nil {
		return fmt.Errorf("nexus docker service 생성 실패: %w", err)
	}
	return nil
}

// readNexusBootstrapPassword 는 첫 기동 비밀번호를 읽는다.
//
// 이미 비밀번호를 바꾼 뒤라면 파일이 없다. 그 경우는 재실행이므로 빈 값을
// 돌려주고, Job 이 프로비저닝된 비밀번호로 로그인한다.
func (o *Orchestrator) readNexusBootstrapPassword(ctx context.Context, namespace string) (string, error) {
	out, err := o.runKubectl(ctx, "exec", "-n", namespace,
		"deployment/"+domain.NexusServiceName, "--",
		"sh", "-c", "cat "+nexusBootstrapPasswordPath+" 2>/dev/null || true")
	if err != nil {
		return "", fmt.Errorf("nexus 초기 비밀번호를 읽지 못했습니다: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runNexusBootstrapJob 은 Nexus REST API 호출을 클러스터 안에서 실행한다.
//
// 컨트롤러가 직접 호출하지 않는 이유는 Nexus 가 ClusterIP 로만 열려 있기
// 때문이다. 오브젝트 스토리지 버킷 부트스트랩과 같은 방식이다.
func (o *Orchestrator) runNexusBootstrapJob(ctx context.Context, namespace, bootstrapPassword, targetPassword string) error {
	script := nexusBootstrapScript()

	manifest := fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: curl
        image: %s
        env:
        - name: NEXUS_URL
          value: %q
        - name: NEXUS_USER
          value: %q
        - name: BOOTSTRAP_PASSWORD
          value: %q
        - name: TARGET_PASSWORD
          value: %q
        - name: DOCKER_PORT
          value: "%d"
        - name: DOCKER_REPO
          value: %q
        - name: MAVEN_REPO
          value: %q
        - name: NPM_REPO
          value: %q
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`,
		nexusProvisionJobName, namespace, shareddomain.CurlImage,
		fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", domain.NexusServiceName, namespace, domain.NexusServicePort),
		domain.NexusAdminUser,
		bootstrapPassword,
		targetPassword,
		domain.NexusDockerServicePort,
		domain.NexusDockerRepository,
		domain.NexusMavenRepository,
		domain.NexusNpmRepository,
		indentYAML(script, 12))

	_, _ = o.runKubectl(ctx, "delete", "job", nexusProvisionJobName, "-n", namespace, "--ignore-not-found=true")
	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return fmt.Errorf("nexus bootstrap job 생성 실패: %w", err)
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=complete", "--timeout=300s", "job/"+nexusProvisionJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+nexusProvisionJobName)
		return fmt.Errorf("nexus bootstrap job 실패: %w (%s)", err, strings.TrimSpace(string(logs)))
	}
	if _, err := o.runKubectl(ctx, "delete", "job", nexusProvisionJobName, "-n", namespace, "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("nexus bootstrap job 정리 실패: %w", err)
	}
	return nil
}

// nexusBootstrapScript 는 비밀번호 교체와 저장소 생성을 수행한다.
//
// 모든 단계가 재실행 가능해야 한다 — 설치는 중간에 실패해 재개되는 일이 잦고,
// 그때마다 이미 만든 저장소 때문에 전체가 실패하면 복구할 방법이 없다.
func nexusBootstrapScript() string {
	return `set -eu

api() {
  # $1=method $2=path $3=content-type $4=body
  curl -sS -o /tmp/body -w '%{http_code}' -u "$NEXUS_USER:$PASSWORD" \
    -X "$1" "$NEXUS_URL$2" -H "Content-Type: $3" --data-binary "$4"
}

# 비밀번호가 이미 바뀐 뒤의 재실행을 먼저 다룬다. 초기 비밀번호로만 시도하면
# 두 번째 실행이 401 로 죽는다.
PASSWORD="$TARGET_PASSWORD"
if ! curl -sSf -u "$NEXUS_USER:$PASSWORD" "$NEXUS_URL/service/rest/v1/status" >/dev/null 2>&1; then
  if [ -z "$BOOTSTRAP_PASSWORD" ]; then
    echo "초기 비밀번호 파일이 없고 프로비저닝된 비밀번호로도 로그인할 수 없습니다" >&2
    exit 1
  fi
  PASSWORD="$BOOTSTRAP_PASSWORD"
  echo "초기 비밀번호로 로그인해 관리자 비밀번호를 교체합니다"
  code=$(curl -sS -o /tmp/body -w '%{http_code}' -u "$NEXUS_USER:$PASSWORD" \
    -X PUT "$NEXUS_URL/service/rest/v1/security/users/$NEXUS_USER/change-password" \
    -H 'Content-Type: text/plain' --data-binary "$TARGET_PASSWORD")
  if [ "$code" != "204" ]; then
    echo "비밀번호 교체 실패 (HTTP $code): $(cat /tmp/body)" >&2
    exit 1
  fi
  PASSWORD="$TARGET_PASSWORD"
fi

# docker login 은 Bearer 토큰 realm 이 켜져 있어야 동작한다.
# 기본 활성 목록은 ["NexusAuthenticatingRealm"] 뿐이라, 켜지 않으면 CI 의
# docker login 이 항상 실패한다.
#
# 목록은 전체 교체이므로 기존 realm 을 함께 보내야 한다 — DockerToken 만
# 보내면 일반 로그인이 끊긴다. 3.64 기준 유효한 ID 는
# /service/rest/v1/security/realms/available 로 확인했다.
code=$(api PUT /service/rest/v1/security/realms/active application/json \
  '["NexusAuthenticatingRealm","DockerToken"]')
case "$code" in
  20*) echo "realm 설정 완료" ;;
  *) echo "realm 설정 실패 (HTTP $code): $(cat /tmp/body)" >&2; exit 1 ;;
esac

create_repo() {
  # $1=사람이 읽는 이름 $2=API 경로 $3=본문
  code=$(api POST "$2" application/json "$3")
  case "$code" in
    20*) echo "$1 생성 완료" ;;
    400|409) echo "$1 이미 있음 — 건너뜀" ;;
    *) echo "$1 생성 실패 (HTTP $code): $(cat /tmp/body)" >&2; exit 1 ;;
  esac
}

create_repo "docker 저장소" /service/rest/v1/repositories/docker/hosted "{
  \"name\": \"$DOCKER_REPO\",
  \"online\": true,
  \"storage\": {\"blobStoreName\": \"default\", \"strictContentTypeValidation\": true, \"writePolicy\": \"ALLOW\"},
  \"docker\": {\"v1Enabled\": false, \"forceBasicAuth\": false, \"httpPort\": $DOCKER_PORT}
}"

create_repo "maven 저장소" /service/rest/v1/repositories/maven/hosted "{
  \"name\": \"$MAVEN_REPO\",
  \"online\": true,
  \"storage\": {\"blobStoreName\": \"default\", \"strictContentTypeValidation\": true, \"writePolicy\": \"ALLOW\"},
  \"maven\": {\"versionPolicy\": \"MIXED\", \"layoutPolicy\": \"STRICT\"}
}"

create_repo "npm 저장소" /service/rest/v1/repositories/npm/hosted "{
  \"name\": \"$NPM_REPO\",
  \"online\": true,
  \"storage\": {\"blobStoreName\": \"default\", \"strictContentTypeValidation\": true, \"writePolicy\": \"ALLOW\"}
}"

echo "nexus 프로비저닝 완료"
`
}

// nexusDockerServiceManifest 는 Docker 커넥터 포트를 노출하는 Service 다.
//
// 차트 Service 는 8081 만 연다. 커넥터를 열어도 클러스터 안에서 닿을 수 없으면
// CI 가 이미지를 올리지 못하므로 함께 만든다.
func nexusDockerServiceManifest(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s-docker
  namespace: %s
  labels:
    app.kubernetes.io/name: nexus-repository-manager
    app.kubernetes.io/managed-by: nullus
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: nexus-repository-manager
    app.kubernetes.io/instance: %s
  ports:
    - name: docker
      port: %d
      targetPort: %d
`, domain.NexusServiceName, namespace, domain.NexusReleaseName,
		domain.NexusDockerServicePort, domain.NexusDockerServicePort)
}

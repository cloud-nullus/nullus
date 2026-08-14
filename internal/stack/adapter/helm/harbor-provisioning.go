package helm

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const (
	// defaultImageProjectName 은 이미지 저장소 접두사의 기본값이다.
	//
	// CI/CD 모듈의 그룹 경로(NULLUS_SCM_GROUP, 기본 "nullus")와 같아야 한다 —
	// 레지스트리 리졸버가 harbor.<domain>/<group>/<app> 를 만들기 때문이다.
	// 모듈 간 직접 import 가 금지돼 값을 조립 지점에서 주입받고, 여기 기본값은
	// 주입이 없을 때의 폴백이다.
	defaultImageProjectName = "nullus"

	harborProvisionJobName = "harbor-provision"
)

// harborProjectNamePattern 은 Harbor 가 받는 프로젝트 이름 형식이다.
// 소문자·숫자와 . _ - 만 허용한다.
var harborProjectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,254}$`)

// harborProjectName 은 이 스택이 만들 Harbor 프로젝트 이름이다.
func (o *Orchestrator) harborProjectName() string {
	if o != nil {
		if name := strings.TrimSpace(o.imageProjectName); name != "" {
			return name
		}
	}
	return defaultImageProjectName
}

// ensureHarborProvisioned 는 이미지를 올릴 Harbor 프로젝트를 만든다.
//
// Harbor 는 push 전에 프로젝트가 존재해야 한다. 없으면 첫 push 가
// "unauthorized: project <name> not found" 로 죽는데, 스택은 정상 설치됐고
// 빌드도 성공한 뒤라 원인이 멀리 떨어진 실패가 된다.
//
// Nexus 가 provisioning_nexus 로 커넥터·저장소를 맞추는 것과 같은 자리다.
func (o *Orchestrator) ensureHarborProvisioned(ctx context.Context, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultStackNamespace
	}

	// Harbor API 는 기동이 끝나야 응답한다. 기다리지 않으면 아래 Job 이
	// connection refused 로 죽고 원인이 "Harbor 프로비저닝 실패" 로만 남는다.
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=available", "--timeout=600s",
		"deployment/"+domain.HarborServiceName+"-core"); err != nil {
		return fmt.Errorf("harbor 기동을 기다리지 못했습니다: %w", err)
	}

	// 비밀번호는 읽어 두지 않는다. Job 이 Secret 을 직접 참조하므로 매니페스트에
	// 평문이 실리지 않고, 회전돼도 Job 이 최신 값을 본다.
	return o.runHarborProvisionJob(ctx, namespace)
}

// harborProjectScript 는 프로젝트를 만드는 셸 스크립트다.
//
// 멱등해야 한다 — 스택을 다시 배포하면 이 단계가 또 돈다. 이미 있으면 Harbor 가
// 409 를 주므로 그것을 성공으로 다룬다.
func harborProjectScript(project string) string {
	name := strings.TrimSpace(project)
	if !harborProjectNamePattern.MatchString(name) {
		// 형식에 맞지 않는 이름은 JSON 을 깨뜨리거나 Harbor 가 거부한다.
		// 조용히 다른 프로젝트를 만드는 것보다 기본값으로 떨어뜨리는 편이 낫다.
		name = defaultImageProjectName
	}

	return fmt.Sprintf(`set -eu
code=$(curl -s -o /tmp/out -w '%%{http_code}' -u "admin:$HARBOR_PASSWORD" \
  -H 'Content-Type: application/json' -X POST \
  "$HARBOR_URL/api/v2.0/projects" \
  -d "{\"project_name\":\"%s\",\"public\":true,\"metadata\":{\"public\":\"true\"}}")
case "$code" in
  201) echo "harbor project %s 생성" ;;
  409) echo "harbor project %s 이미 존재" ;;
  *) echo "harbor project 생성 실패 (HTTP $code)"; cat /tmp/out; exit 1 ;;
esac
`, name, name, name)
}

func (o *Orchestrator) runHarborProvisionJob(ctx context.Context, namespace string) error {
	// 클러스터 안에서 Service 로 부른다 — Harbor 를 외부에 노출하지 않아도
	// 동작해야 하고, 게이트웨이 라우트보다 앞서 도는 단계이기 때문이다.
	harborURL := fmt.Sprintf("http://%s.%s.svc:%d",
		domain.HarborServiceName, namespace, domain.HarborServicePort)

	manifest := fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 300
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: curl
        image: curlimages/curl:8.11.1
        env:
        - name: HARBOR_URL
          value: %q
        - name: HARBOR_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
        command: ["sh", "-c"]
        args:
        - |
%s
`, harborProvisionJobName, namespace, harborURL,
		domain.HarborAdminSecret, domain.HarborAdminPassKey,
		indentYAML(harborProjectScript(o.harborProjectName()), 10))

	// 이전 실행이 남아 있으면 Job 은 불변 필드 때문에 갱신되지 않는다.
	_, _ = o.runKubectl(ctx, "delete", "job", harborProvisionJobName, "-n", namespace, "--ignore-not-found")

	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return fmt.Errorf("harbor 프로비저닝 Job 생성 실패: %w", err)
	}

	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=complete", "--timeout=300s",
		"job/"+harborProvisionJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+harborProvisionJobName)
		return fmt.Errorf("harbor 프로비저닝 Job 실패: %w (로그: %s)", err, strings.TrimSpace(string(logs)))
	}

	return nil
}

package helm

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
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
  401|403)
    echo "harbor project 생성 실패 (HTTP $code): 관리자 자격증명이 맞지 않습니다."
    echo "Harbor 는 관리자 비밀번호를 자기 데이터베이스에 굽습니다. 이전 설치의"
    echo "볼륨(database-data-harbor-database-0)이 남은 채 다시 설치하면, 새로 만든"
    echo "Secret(%s)의 값과 DB 안의 값이 어긋납니다."
    echo "네임스페이스를 통째로 지우고 다시 설치하거나, Harbor 볼륨을 지우세요."
    cat /tmp/out
    exit 1 ;;
  *) echo "harbor project 생성 실패 (HTTP $code)"; cat /tmp/out; exit 1 ;;
esac
`, name, name, name, domain.HarborAdminSecret)
}

// harborOIDCScript 는 Harbor 의 인증을 Keycloak 으로 바꾸는 셸 스크립트다.
//
// Harbor 는 OIDC 설정을 Helm values 가 아니라 자기 API 로 받는다. 그래서 다른
// 도구처럼 oidc-values 의 switch 에 넣을 수 없고 이 Job 에 붙는다 — 그동안
// Keycloak 에 클라이언트만 만들어지고 Harbor 는 db_auth 로 남아 있었다.
//
// 멱등하다. 같은 값을 다시 PUT 하면 Harbor 가 200 을 준다.
// (주의: Harbor 는 admin 외 사용자가 생긴 뒤에는 auth_mode 변경을 거부한다.
//
//	그 경우 응답 본문과 함께 실패한다 — 조용히 넘기면 SSO 가 안 되는 이유가
//	어디에도 남지 않는다.)
func harborOIDCScript(clientID, issuer string) string {
	// http endpoint 는 검증할 인증서가 없다. https 면 Harbor 가 체인을 검증한다.
	verifyCert := "false"
	if strings.HasPrefix(issuer, "https://") {
		verifyCert = "true"
	}

	// JSON 은 셸 작은따옴표로 감싸고 비밀값만 따옴표를 교차해 끼운다.
	//
	//   -d '{... "x":"'"$VAR"'" ...}'
	//
	// 큰따옴표로 감싸고 \" 로 이스케이프하는 방식은 한 겹만 어긋나도 셸이
	// 바깥 따옴표를 첫 안쪽 따옴표에서 닫아 버린다. 실제로 그렇게 나가서
	// Harbor 가 422 로 거부했다("invalid character 'a' looking for beginning
	// of object key string"). heredoc 은 매니페스트 들여쓰기와 충돌해 못 쓴다
	// (종료 표식이 열 맞춤으로 밀리면 셸이 인식하지 못한다).
	//
	// 비밀값에 따옴표가 들어가면 여전히 깨지지만, 생성 비밀번호는 영숫자만
	// 쓴다(secretAlphabet).
	return fmt.Sprintf(`
payload='{"auth_mode":"oidc_auth","oidc_name":"Keycloak","oidc_endpoint":"%s","oidc_client_id":"%s","oidc_client_secret":"'"$HARBOR_OIDC_SECRET"'","oidc_scope":"openid,profile,email","oidc_verify_cert":%s,"oidc_auto_onboard":true,"oidc_user_claim":"preferred_username"}'
code=$(curl -s -o /tmp/oidc -w '%%{http_code}' -u "admin:$HARBOR_PASSWORD" \
  -H 'Content-Type: application/json' -X PUT \
  "$HARBOR_URL/api/v2.0/configurations" -d "$payload")
case "$code" in
  200) echo "harbor OIDC 설정 완료 (client=%s)" ;;
  *) echo "harbor OIDC 설정 실패 (HTTP $code)"; cat /tmp/oidc; exit 1 ;;
esac
`, issuer, clientID, verifyCert, clientID)
}

// harborOIDCSettings 는 이 스택에서 Harbor SSO 를 켤 수 있는지와 그 값을 준다.
//
// 둘 중 하나라도 없으면 켜지 않는다. endpoint 없는 oidc_auth 는 아무도 로그인할
// 수 없는 Harbor 를 만든다 — db_auth 로 두는 편이 낫다.
func (o *Orchestrator) harborOIDCSettings() (clientID, issuer string, ok bool) {
	provisioner := o.ssoProvisioner()
	if provisioner == nil {
		return "", "", false
	}
	clientID, found := provisioner.ClientIDFor("installing_harbor")
	if !found || strings.TrimSpace(clientID) == "" {
		return "", "", false
	}

	o.mu.Lock()
	issuer = o.toolOIDCIssuer
	o.mu.Unlock()
	if strings.TrimSpace(issuer) == "" {
		return "", "", false
	}
	return clientID, issuer, true
}

// harborProvisionManifest 는 프로젝트를 만드는 Job 매니페스트다.
func (o *Orchestrator) harborProvisionManifest(namespace string) string {
	// 클러스터 안에서 Service 로 부른다 — Harbor 를 외부에 노출하지 않아도
	// 동작해야 하고, 게이트웨이 라우트보다 앞서 도는 단계이기 때문이다.
	harborURL := fmt.Sprintf("http://%s.%s.svc:%d",
		domain.HarborServiceName, namespace, domain.HarborServicePort)

	// SSO 를 켠 설치에서만 OIDC 를 설정한다. client secret 은 ESO 가 만든
	// Secret 을 참조한다 — 관리자 비밀번호와 같은 이유로 매니페스트에 평문이
	// 실리면 안 되고, 회전돼도 Job 이 최신 값을 봐야 한다.
	script := harborProjectScript(o.harborProjectName())
	oidcEnv := ""
	if clientID, issuer, ok := o.harborOIDCSettings(); ok {
		script += harborOIDCScript(clientID, issuer)
		oidcEnv = fmt.Sprintf(`
        - name: HARBOR_OIDC_SECRET
          valueFrom:
            secretKeyRef:
              name: %s
              key: client-secret`, SSOSecretName(clientID))
	}

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
        image: %s
        env:
        - name: HARBOR_URL
          value: %q
        - name: HARBOR_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s%s
        command: ["sh", "-c"]
        args:
        - |
%s
`, harborProvisionJobName, namespace, shareddomain.CurlImage, harborURL,
		domain.HarborAdminSecret, domain.HarborAdminPassKey, oidcEnv,
		indentYAML(script, 10))

	return manifest
}

func (o *Orchestrator) runHarborProvisionJob(ctx context.Context, namespace string) error {
	manifest := o.harborProvisionManifest(namespace)

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

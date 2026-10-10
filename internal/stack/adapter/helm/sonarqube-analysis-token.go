package helm

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// SonarQube 분석 토큰.
//
// CI 의 소스 정적 분석 단계는 이 토큰으로 SonarQube 에 결과를 올린다. 스택 설치가 발급해
// 스택의 OpenBao 에 두고, 파이프라인을 만드는 쪽(cicd)이 그 값을 읽어 시크릿 CI 변수로 건다
// (shareddomain.SonarQubeAnalysisTokenPath). 그래서 cicd 는 SonarQube 에 직접 닿을 필요가 없다.
//
// 발급은 클러스터 안 Job 이 한다. 플랫폼 API 는 클러스터 밖에서 돌 수 있어 SonarQube 에
// 닿는다는 보장이 없다. 토큰은 표준 출력에 싣지 않는다 — 스택에 로그 수집(OTel agent →
// Loki)이 있으면 파드 로그가 그대로 보관돼 Job 을 지워도 사본이 남는다. Job 은 결과를 파드 안
// 파일에 두고 기다리며, 오케스트레이터가 exec 로 읽은 뒤 Job 을 지운다.
const (
	sonarqubeTokenJobName = "nullus-sonarqube-ci-token"
	// sonarqubeAnalysisTokenName 은 SonarQube 안에서 토큰을 가리키는 이름(의 접두사)이다.
	sonarqubeAnalysisTokenName = "nullus-ci"
	// sonarTokenResultFile 은 Job 이 결과를 두는 파드 안 경로다.
	sonarTokenResultFile = "/tmp/nullus-sonar-result"

	sonarTokenResultToken  = "TOKEN="
	sonarTokenResultExists = "EXISTS"
	sonarTokenResultError  = "ERROR="

	// sonarTokenJobWait 은 오케스트레이터가 결과를 기다리는 한도다. Job 은 SonarQube 가 뜨기를
	// 최대 10분 기다린다.
	sonarTokenJobWait = 15 * time.Minute
)

// sonarTokenJobResult 는 발급 Job 의 결과다. 이미 있었으면 Exists, 새로 발급했으면 Token 이다.
type sonarTokenJobResult struct {
	Token  string
	Exists bool
}

// sonarqubeTokenJobManifest 는 분석 토큰을 발급하는 Job 이다.
//
// 이미 Nullus 가 발급한 토큰(이름이 nullus-ci 로 시작)이 있으면 발급하지 않는다. SonarQube 는
// 토큰 값을 다시 보여 주지 않으므로, OpenBao 에서 값을 잃었을 때(rotate)만 새 이름으로 하나
// 더 발급한다. 이전 토큰은 폐기하지 않는다 — 기존 파이프라인 변수에 사본이 남아 있을 수 있어,
// 폐기하면 그 파이프라인이 모두 인증에서 죽는다.
//
// 권한은 분석만이다(GLOBAL_ANALYSIS_TOKEN). 관리자 사용자 토큰을 주면 CI 가 관리자 권한을 갖는다.
func sonarqubeTokenJobManifest(namespace, stackName string, rotate bool) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = defaultStackNamespace
	}
	admin := domain.SonarQubeAdminUser
	result := sonarTokenResultFile
	// fail 은 이유를 결과 파일에 남기고 읽힐 때까지 기다린다 — 비밀이 아닌 이유는 로그에도 남긴다.
	fail := func(msg string) string {
		return fmt.Sprintf("echo '%s%s' > %s; echo '%s'; hold; exit 1", sonarTokenResultError, msg, result, msg)
	}
	lines := []string{
		"set -u",
		"umask 077",
		// PID 1 인 sh 는 처리기가 없는 신호를 무시한다. 그대로 두면 Job 을 지워도 파드가 유예
		// 시간 내내 남아, 같은 이름으로 다시 만든 Job 의 결과 대신 읽힐 수 있다.
		"trap 'exit 0' TERM INT",
		// hold 는 결과를 읽어 갈 때까지 기다린다. 백그라운드 sleep 을 wait 해야 신호에 바로 깬다.
		"hold() { sleep 600 & wait $!; }",
		fmt.Sprintf("BASE=%s", shareddomain.SonarQubeServerEndpoint(ns)),
		fmt.Sprintf("NAME=%s", sonarqubeAnalysisTokenName),
		"i=0",
		`until curl -fsS "$BASE/api/system/status" 2>/dev/null | grep -q '"status":"UP"'; do`,
		`  i=$((i+1))`,
		`  if [ "$i" -gt 120 ]; then ` + fail("sonarqube did not become UP") + `; fi`,
		"  sleep 5",
		"done",
	}
	if rotate {
		lines = append(lines, `NAME="$NAME-$(date +%Y%m%d%H%M%S)"`)
	} else {
		lines = append(lines,
			fmt.Sprintf(`if curl -fsS -u "%s:$ADMIN_PASSWORD" "$BASE/api/user_tokens/search?login=%s" | grep -q "\"name\":\"$NAME"; then`, admin, admin),
			fmt.Sprintf("  echo '%s' > %s; echo 'analysis token already issued'; hold; exit 0", sonarTokenResultExists, result),
			"fi",
		)
	}
	lines = append(lines,
		fmt.Sprintf(`RESP=$(curl -fsS -u "%s:$ADMIN_PASSWORD" -X POST "$BASE/api/user_tokens/generate" \`, admin),
		fmt.Sprintf(`  --data-urlencode "login=%s" --data-urlencode "name=$NAME" --data-urlencode "type=GLOBAL_ANALYSIS_TOKEN") || RESP=""`, admin),
		`TOKEN=$(printf '%s' "$RESP" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')`,
		`if [ -z "$TOKEN" ]; then `+fail("token response did not carry a token")+`; fi`,
		fmt.Sprintf(`echo "%s$TOKEN" > %s`, sonarTokenResultToken, result),
		"echo 'analysis token issued'",
		"hold",
	)

	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  backoffLimit: 0
  # 오케스트레이터가 결과를 읽기 전에 죽어도 Job 이 남지 않는다.
  activeDeadlineSeconds: 1500
  ttlSecondsAfterFinished: 60
  template:
    metadata:
      labels:
        nullus.io/stack-name: %s
    spec:
      restartPolicy: Never
      containers:
      - name: token
        image: %s
        env:
        - name: ADMIN_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, sonarqubeTokenJobName, ns, stackName, stackName, shareddomain.CurlImage,
		domain.SonarQubeSecret, domain.SonarQubeAdminPasswordKey, indentYAML(strings.Join(lines, "\n"), 12))
}

// parseSonarQubeTokenResult 는 Job 이 결과 파일에 남긴 줄을 읽는다.
func parseSonarQubeTokenResult(content string) (sonarTokenJobResult, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == sonarTokenResultExists:
			return sonarTokenJobResult{Exists: true}, nil
		case strings.HasPrefix(line, sonarTokenResultToken):
			if token := strings.TrimSpace(strings.TrimPrefix(line, sonarTokenResultToken)); token != "" {
				return sonarTokenJobResult{Token: token}, nil
			}
		case strings.HasPrefix(line, sonarTokenResultError):
			return sonarTokenJobResult{}, fmt.Errorf("SonarQube 분석 토큰을 발급하지 못했습니다: %s",
				strings.TrimPrefix(line, sonarTokenResultError))
		}
	}
	return sonarTokenJobResult{}, fmt.Errorf("토큰 발급 Job 의 결과를 찾지 못했습니다")
}

// isSecretNotFound 는 OpenBao 에 그 경로가 없다는 뜻인지 본다. 클러스터 밖에서 직접 붙으면
// secrets.StatusError, API 서버 프록시를 거치면 쿠버네티스 오류로 온다.
func isSecretNotFound(err error) bool {
	return secrets.IsNotFound(err) || apierrors.IsNotFound(err)
}

// syncSonarQubeAnalysisToken 은 SonarQube 의 분석 토큰과 OpenBao 를 맞춘다.
//
// 다시 돌려도 같은 결과다. 이미 발급했고 OpenBao 에 값이 있으면 그대로 둔다. OpenBao 에 값이
// 없을 때(경로가 없거나 비었을 때)만 새로 발급한다 — 읽지 못한 것(봉인·장애·권한)은 값을
// 잃은 것과 다르다. 그때 발급하면 기록도 실패해 새 토큰을 잃으므로 멈추고 다음 시도에 맡긴다.
func syncSonarQubeAnalysisToken(
	ctx context.Context,
	writer SecretWriter,
	path string,
	run func(ctx context.Context, rotate bool) (sonarTokenJobResult, error),
) error {
	result, err := run(ctx, false)
	if err != nil {
		return err
	}
	if result.Exists {
		current, getErr := writer.GetToken(ctx, path)
		switch {
		case getErr == nil && strings.TrimSpace(current) != "":
			return nil
		case getErr != nil && !isSecretNotFound(getErr):
			return fmt.Errorf("OpenBao 에서 SonarQube 분석 토큰을 읽지 못해 다시 발급하지 않습니다 (%s): %w", path, getErr)
		}
		if result, err = run(ctx, true); err != nil {
			return err
		}
	}
	if strings.TrimSpace(result.Token) == "" {
		return fmt.Errorf("SonarQube 분석 토큰을 발급하지 못했습니다")
	}
	if err := writer.PutToken(ctx, path, result.Token); err != nil {
		return fmt.Errorf("SonarQube 분석 토큰을 OpenBao 에 기록하지 못했습니다 (%s): %w", path, err)
	}
	return nil
}

// runSonarQubeTokenJob 은 발급 Job 을 돌리고, 결과 파일을 exec 로 읽은 뒤 Job 을 지운다.
func (o *Orchestrator) runSonarQubeTokenJob(ctx context.Context, namespace, stackName string, rotate bool) (sonarTokenJobResult, error) {
	// 앞선 실행의 파드까지 사라진 뒤에 만든다.
	_, _ = o.runKubectl(ctx, "delete", "job", sonarqubeTokenJobName, "-n", namespace,
		"--ignore-not-found=true", "--cascade=foreground", "--timeout=120s")
	// 토큰이 파드 안에 있으므로 읽은 뒤에는 남기지 않는다.
	defer func() {
		_, _ = o.runKubectl(context.WithoutCancel(ctx), "delete", "job", sonarqubeTokenJobName, "-n", namespace,
			"--ignore-not-found=true", "--wait=false")
	}()

	if err := o.applyManifest(ctx, namespace, sonarqubeTokenJobManifest(namespace, stackName, rotate)); err != nil {
		return sonarTokenJobResult{}, fmt.Errorf("SonarQube 분석 토큰 Job 생성 실패: %w", err)
	}
	// 같은 이름의 앞선 Job 파드가 아직 남아 있을 수 있다(종료 유예). 이름이 아니라 이 Job 의
	// uid 로 파드를 고른다 — 앞선 파드의 결과(EXISTS)를 읽으면 방금 발급한 토큰을 잃는다.
	uidOut, err := o.runKubectl(ctx, "get", "job", sonarqubeTokenJobName, "-n", namespace, "-o", "jsonpath={.metadata.uid}")
	uid := strings.TrimSpace(string(uidOut))
	if err != nil || uid == "" {
		return sonarTokenJobResult{}, fmt.Errorf("SonarQube 분석 토큰 Job 을 찾지 못했습니다: %v", err)
	}

	deadline := time.Now().Add(sonarTokenJobWait)
	for {
		if content, ok := o.readSonarQubeTokenResult(ctx, namespace, uid); ok {
			return parseSonarQubeTokenResult(content)
		}
		if time.Now().After(deadline) {
			// 로그에는 토큰이 실리지 않는다 — 결과는 파일로만 넘긴다.
			logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+sonarqubeTokenJobName, "--tail=20")
			return sonarTokenJobResult{}, fmt.Errorf("SonarQube 분석 토큰 Job 이 %s 안에 결과를 내지 않았습니다 (로그: %s)",
				sonarTokenJobWait, strings.TrimSpace(string(logs)))
		}
		select {
		case <-ctx.Done():
			return sonarTokenJobResult{}, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// readSonarQubeTokenResult 는 이 Job 인스턴스(uid) 파드의 결과 파일을 읽는다. 아직 없으면 ok=false 다.
func (o *Orchestrator) readSonarQubeTokenResult(ctx context.Context, namespace, jobUID string) (string, bool) {
	pod, err := o.runKubectl(ctx, "get", "pods", "-n", namespace, "-l", "batch.kubernetes.io/controller-uid="+jobUID,
		"-o", "jsonpath={.items[0].metadata.name}")
	name := strings.TrimSpace(string(pod))
	if err != nil || name == "" {
		return "", false
	}
	out, err := o.runKubectl(ctx, "exec", "-n", namespace, name, "--", "cat", sonarTokenResultFile)
	content := strings.TrimSpace(string(out))
	if err != nil || content == "" {
		return "", false
	}
	return content, true
}

// ensureSonarQubeAnalysisToken 은 CI 가 쓸 분석 토큰을 발급해 스택의 OpenBao 에 둔다.
func (o *Orchestrator) ensureSonarQubeAnalysisToken(ctx context.Context, namespace, stackName string) error {
	o.mu.Lock()
	env, orgID := o.secretEnv, o.secretOrgID
	o.mu.Unlock()

	store, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     o.kubeconfig,
		Namespace:      namespace,
		Role:           secrets.ControllerRole,
		ServiceAccount: secrets.ControllerServiceAccount,
	})
	if err != nil {
		return fmt.Errorf("OpenBao 컨트롤러 자격 생성 실패: %w", err)
	}
	path := secretPathPrefix(env, orgID) + shareddomain.SonarQubeAnalysisTokenPath
	return syncSonarQubeAnalysisToken(ctx, store, path, func(ctx context.Context, rotate bool) (sonarTokenJobResult, error) {
		return o.runSonarQubeTokenJob(ctx, namespace, stackName, rotate)
	})
}

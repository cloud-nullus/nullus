package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// installOCIChartWithHelmCLI runs `helm upgrade --install` for an OCI chart reference.
// wait controls whether --wait is appended (nil defaults to true).
// valuesFile, if non-empty, is passed via --values.
// plainHTTP adds --plain-http for insecure HTTP OCI registries.
func installOCIChartWithHelmCLI(ctx context.Context, kubeconfig []byte, releaseName, chartName, namespace, version string, wait *bool, valuesFile string, plainHTTP bool) error {
	if strings.TrimSpace(releaseName) == "" || strings.TrimSpace(chartName) == "" || strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("invalid helm cli install arguments")
	}
	if len(kubeconfig) == 0 {
		return fmt.Errorf("kubeconfig is empty")
	}
	tmpFile, err := os.CreateTemp("", "nullus-helm-kubeconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err := tmpFile.Write(kubeconfig); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close kubeconfig temp file: %w", err)
	}

	args := []string{"upgrade", "--install", releaseName, chartName, "--namespace", namespace, "--create-namespace", "--skip-crds"}
	if strings.TrimSpace(version) != "" {
		args = append(args, "--version", version)
	}
	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}
	if plainHTTP {
		args = append(args, "--plain-http")
	}
	doWait := true
	if wait != nil {
		doWait = *wait
	}
	if doWait {
		args = append(args, "--wait")
	}
	args = append(args, "--kubeconfig", tmpFile.Name())
	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm %s failed: %w (output=%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

// ErrRunnerTokenDiscovery 는 Runner 등록 토큰을 얻지 못했음을 알리는 sentinel 이다.
//
// 과거에는 이 실패를 무시하고 스텝을 completed 로 마킹했다. 그러면 CI 실행기가
// 없는데도 스택이 completed 로 끝나 파이프라인이 한 건도 돌지 않는 상태가
// 조용히 만들어진다. 설치 실패로 드러내야 재시도 경로를 탈 수 있다.
var ErrRunnerTokenDiscovery = errors.New("gitlab runner 등록 토큰을 얻지 못했습니다")

// wrapRunnerTokenDiscoveryError 는 토큰 발견 실패를 sentinel 로 감싼다.
// nil 은 nil 로 통과시켜 호출부 분기를 단순하게 유지한다.
func wrapRunnerTokenDiscoveryError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: GitLab 이 완전히 기동했는지 확인하세요 "+
		"(kubectl -n <ns> get deploy gitlab-toolbox, gitlab-webservice-default): %v",
		ErrRunnerTokenDiscovery, err)
}

func (o *Orchestrator) discoverGitLabRunnerRegistrationToken(ctx context.Context, namespace string) (string, error) {
	const (
		maxAttempts = 24
		retryDelay  = 10 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := o.discoverGitLabRunnerRegistrationTokenOnce(ctx, namespace)
		if err == nil {
			return token, nil
		}
		lastErr = err

		retryable := isRetryableRunnerTokenDiscoveryError(err)
		if !retryable || attempt == maxAttempts {
			return "", err
		}

		slog.Warn("gitlab runner token not ready yet; retrying",
			"namespace", namespace,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("runner token discovery failed")
	}
	return "", lastErr
}

func (o *Orchestrator) discoverGitLabRunnerRegistrationTokenOnce(ctx context.Context, namespace string) (string, error) {
	authTokenScript := `runner = Ci::Runner.where(description: "nullus-shared-runner", runner_type: :instance_type).order(id: :desc).first; runner ||= Ci::Runner.create!(description: "nullus-shared-runner", runner_type: :instance_type, run_untagged: true, locked: false); puts runner.token.to_s`
	token, authErr := o.discoverGitLabRunnerTokenFromRailsRunner(ctx, namespace, authTokenScript)
	if authErr == nil {
		return token, nil
	}

	legacyRegistrationTokenScript := `puts ApplicationSetting.current.runners_registration_token`
	token, legacyErr := o.discoverGitLabRunnerTokenFromRailsRunner(ctx, namespace, legacyRegistrationTokenScript)
	if legacyErr == nil {
		return token, nil
	}

	return "", runnerTokenNotFoundError(authErr, legacyErr)
}

// runnerTokenNotFoundError 는 두 조회 경로의 원인을 모두 보존한다.
//
// 원인을 버리고 일반 메시지만 돌려주면 isRetryableRunnerTokenDiscoveryError 가
// 힌트를 찾지 못해 재시도 루프가 한 번도 돌지 않는다. GitLab 이 아직 마이그레이션
// 중인 정상 상황에서도 즉시 실패하게 되므로, 원인 문자열을 반드시 남긴다.
func runnerTokenNotFoundError(authErr, legacyErr error) error {
	return fmt.Errorf("runner token not found in rails output (auth: %v; legacy: %w)", authErr, legacyErr)
}

func (o *Orchestrator) discoverGitLabRunnerTokenFromRailsRunner(ctx context.Context, namespace, script string) (string, error) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return "", fmt.Errorf("kubeconfig unavailable")
	}
	kubeconfigPath, err := o.writeKubeconfigTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	args := []string{
		"--kubeconfig", kubeconfigPath,
		"-n", namespace,
		"exec", "deploy/gitlab-toolbox",
		"-c", "toolbox",
		"--", "bash", "-lc",
		fmt.Sprintf("gitlab-rails runner '%s'", script),
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl exec failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	token := parseGitLabRunnerRegistrationTokenOutput(string(output))
	if token == "" {
		return "", fmt.Errorf("runner token not found in output")
	}

	return token, nil
}

func isRetryableRunnerTokenDiscoveryError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	retryHints := []string{
		"container not found",
		"unable to upgrade connection",
		"does not have a host assigned",
		"pods \"gitlab-toolbox\" not found",
		"deployments.apps \"gitlab-toolbox\" not found",
		"no such host",
		"i/o timeout",
		"connection refused",
		"context deadline exceeded",
		"application_settings",
		"pg::undefinedtable",
	}

	for _, hint := range retryHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}

	return false
}

func parseGitLabRunnerRegistrationTokenOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	token := ""
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		if candidate == "" || strings.HasPrefix(candidate, "Defaulted container") || strings.Contains(candidate, " ") {
			continue
		}
		token = candidate
	}
	return token
}

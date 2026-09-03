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

func (o *Orchestrator) discoverGitLabRunnerRegistrationToken(ctx context.Context, namespace string) (runnerToken, error) {
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
			return runnerToken{}, err
		}

		slog.Warn("gitlab runner token not ready yet; retrying",
			"namespace", namespace,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return runnerToken{}, ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("runner token discovery failed")
	}
	return runnerToken{}, lastErr
}

func (o *Orchestrator) discoverGitLabRunnerRegistrationTokenOnce(ctx context.Context, namespace string) (runnerToken, error) {
	output, err := o.runGitLabRails(ctx, namespace, runnerTokenProbeScript)
	if err != nil {
		return runnerToken{}, err
	}
	token := parseRunnerTokenProbe(output)
	if token.Kind == runnerTokenNone {
		return runnerToken{}, fmt.Errorf("runner token not found in rails output")
	}
	return token, nil
}


func (o *Orchestrator) runGitLabRails(ctx context.Context, namespace, script string) (string, error) {
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

	return string(output), nil
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


// runnerTokenKind 는 토큰의 종류다. 차트에서 쓰는 자리가 다르므로 값만
// 들고 다니면 안 된다 — 자리를 잘못 잡으면 러너가 조용히 죽는다.
type runnerTokenKind int

const (
	runnerTokenNone runnerTokenKind = iota
	// runnerTokenRegistration 은 등록 토큰이다. 러너가 이것으로 **스스로
	// 등록**하므로, 등록된 러너가 사라져도 다시 등록해 회복한다.
	runnerTokenRegistration
	// runnerTokenAuthentication 은 이미 등록된 러너의 자격증명이다.
	// 가리키는 러너가 사라지면 **복구 경로가 없다** — 러너는 등록이 아니라
	// 검증을 시도하고 "Verifying runner... is removed" 로 영구 실패한다.
	runnerTokenAuthentication
)

type runnerToken struct {
	Value string
	Kind  runnerTokenKind
}

// runnerTokenValues 는 토큰을 차트가 읽는 자리에 넣는다.
//
// 값이 비어 있으면 아무것도 넣지 않는다. 빈 문자열을 넘기면 차트가 빈 값으로
// Secret 을 만들고 러너는 그것으로 등록을 시도하다 죽는다 — 호출부가 실패로
// 다루도록 여기서는 조용히 비워 둔다.
func runnerTokenValues(t runnerToken) map[string]any {
	if strings.TrimSpace(t.Value) == "" {
		return map[string]any{}
	}
	switch t.Kind {
	case runnerTokenRegistration:
		return map[string]any{"runnerRegistrationToken": t.Value}
	case runnerTokenAuthentication:
		return map[string]any{"runnerToken": t.Value}
	default:
		return map[string]any{}
	}
}

// runnerTokenProbeScript 는 등록 토큰 허용 여부와 두 토큰을 한 번에 읽는다.
//
// 출력은 key=value 로 고정한다. 예전 파서는 "마지막 공백 없는 줄" 을 토큰으로
// 집었는데, rails 가 뒤에 경고 한 줄만 찍어도 그것이 토큰이 되는 구조였다.
//
// 인증 토큰 쪽은 find-or-create 다. 이미 있으면 그 토큰을, 없으면 새로 만들어
// 돌려준다 — 등록 토큰을 쓸 수 없는 인스턴스를 위한 대비책이다.
const runnerTokenProbeScript = `
s = ApplicationSetting.current
allowed = s.respond_to?(:allow_runner_registration_token) ? s.allow_runner_registration_token : true
reg = allowed ? s.runners_registration_token.to_s : ""
r = Ci::Runner.where(description: "nullus-shared-runner", runner_type: :instance_type).order(id: :desc).first
r ||= Ci::Runner.create!(description: "nullus-shared-runner", runner_type: :instance_type, run_untagged: true, locked: false)
puts "registration_allowed=#{allowed}"
puts "registration_token=#{reg}"
puts "auth_token=#{r.token}"
`

// parseRunnerTokenProbe 는 탐침 출력에서 쓸 토큰 하나를 고른다.
//
// **등록 토큰을 먼저 고른다.** 러너가 그것으로 스스로 등록하므로, 등록된
// 러너가 사라져도 다시 등록해 회복한다. 인증 토큰은 가리키는 러너가 사라지면
// 복구 경로가 없어 CI 가 영구히 멈춘다 — 실환경에서 정확히 그렇게 됐다.
func parseRunnerTokenProbe(output string) runnerToken {
	field := func(key string) string {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(line, key+"="); ok {
				return strings.TrimSpace(after)
			}
		}
		return ""
	}

	if strings.EqualFold(field("registration_allowed"), "true") {
		if reg := field("registration_token"); reg != "" {
			return runnerToken{Value: reg, Kind: runnerTokenRegistration}
		}
	}
	if auth := field("auth_token"); auth != "" {
		return runnerToken{Value: auth, Kind: runnerTokenAuthentication}
	}
	return runnerToken{}
}

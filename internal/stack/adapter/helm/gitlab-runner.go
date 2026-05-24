package helm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

func installOCIChartWithHelmCLI(ctx context.Context, kubeconfig []byte, releaseName, chartName, namespace, version string) error {
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
	args = append(args, "--kubeconfig", tmpFile.Name())
	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm %s failed: %w (output=%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
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
	if token, err := o.discoverGitLabRunnerTokenFromRailsRunner(ctx, namespace, authTokenScript); err == nil {
		return token, nil
	}

	legacyRegistrationTokenScript := `puts ApplicationSetting.current.runners_registration_token`
	if token, err := o.discoverGitLabRunnerTokenFromRailsRunner(ctx, namespace, legacyRegistrationTokenScript); err == nil {
		return token, nil
	}

	return "", fmt.Errorf("runner token not found in rails output")
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

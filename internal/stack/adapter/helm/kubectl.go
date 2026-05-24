package helm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func isKubectlNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "notfound") || strings.Contains(msg, "not found")
}

func (o *Orchestrator) waitForKubectlNonEmptyOutput(ctx context.Context, args ...string) error {
	const (
		maxAttempts = 30
		retryDelay  = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err := o.runKubectl(ctx, args...)
		if err == nil && strings.TrimSpace(string(output)) != "" {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("empty output")
		}

		if attempt == maxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("resource output not ready")
	}
	return lastErr
}

func (o *Orchestrator) applyManifest(ctx context.Context, namespace, manifest string) error {
	if strings.TrimSpace(manifest) == "" {
		return nil
	}

	kubeconfigPath, err := o.writeKubeconfigTempFile()
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-n", namespace, "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (o *Orchestrator) writeKubeconfigTempFile() (string, error) {
	if len(o.kubeconfig) == 0 {
		return "", fmt.Errorf("kubeconfig is empty")
	}
	tmpFile, err := os.CreateTemp("", "nullus-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
	}()
	if _, err := tmpFile.Write(o.kubeconfig); err != nil {
		return "", fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	return tmpFile.Name(), nil
}

func (o *Orchestrator) runKubectl(ctx context.Context, args ...string) ([]byte, error) {
	kubeconfigPath, err := o.writeKubeconfigTempFile()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	cmdArgs := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("kubectl %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (o *Orchestrator) waitForKubectlGet(ctx context.Context, args ...string) error {
	const (
		maxAttempts = 60
		retryDelay  = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if _, err := o.runKubectl(ctx, append([]string{"get"}, args...)...); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("kubectl get %s failed", strings.Join(args, " "))
	}
	return lastErr
}

func (o *Orchestrator) ensureGatewayAPICRDs(ctx context.Context) error {
	if _, err := o.runKubectl(ctx, "get", "crd", "gatewayclasses.gateway.networking.k8s.io"); err == nil {
		return nil
	}

	if _, err := o.runKubectl(ctx, "apply", "-f", gatewayAPIStandardInstallURL); err != nil {
		return err
	}
	if _, err := o.runKubectl(ctx, "get", "crd", "gatewayclasses.gateway.networking.k8s.io"); err != nil {
		return err
	}
	return nil
}

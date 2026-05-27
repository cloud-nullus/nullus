package helm

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const dbConnectivityJobName = "nullus-gitlab-db-connectivity-check"

func (o *Orchestrator) ensureGitLabDatabaseConnectivity(ctx context.Context, namespace string) error {
	cfg := o.currentStackConfig()
	if cfg == nil || cfg.Storage == nil {
		return nil
	}
	target := cfg.Storage.Database
	if strings.TrimSpace(target.Mode) != "existing-connect" {
		return nil
	}

	if strings.TrimSpace(target.AccessSecretRef) != "" {
		if _, err := o.runKubectl(ctx, "get", "secret", strings.TrimSpace(target.AccessSecretRef), "-n", namespace); err != nil {
			return fmt.Errorf("database access secret %q not found in namespace %q", strings.TrimSpace(target.AccessSecretRef), namespace)
		}
	}

	host, port, err := splitEndpointHostPort(strings.TrimSpace(target.Endpoint))
	if err != nil {
		return err
	}

	script := fmt.Sprintf("set -e\nnslookup %q >/dev/null\nnc -z -w 3 %q %s\n", host, host, port)
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
      - name: check
        image: busybox:1.36
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, dbConnectivityJobName, namespace, indentYAML(script, 12))

	_, _ = o.runKubectl(ctx, "delete", "job", dbConnectivityJobName, "-n", namespace, "--ignore-not-found=true")
	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return err
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace, "--for=condition=complete", "--timeout=120s", "job/"+dbConnectivityJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+dbConnectivityJobName)
		return fmt.Errorf("database connectivity check failed: %w (%s)", err, strings.TrimSpace(string(logs)))
	}
	_, _ = o.runKubectl(ctx, "delete", "job", dbConnectivityJobName, "-n", namespace, "--ignore-not-found=true")
	return nil
}

func splitEndpointHostPort(endpoint string) (string, string, error) {
	if endpoint == "" {
		return "", "", fmt.Errorf("storage.database.endpoint is required for existing-connect")
	}
	value := endpoint
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", "", fmt.Errorf("invalid storage.database.endpoint: %w", err)
		}
		value = u.Host
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", "", fmt.Errorf("storage.database.endpoint must include host:port")
	}
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return "", "", fmt.Errorf("storage.database.endpoint must include host:port")
	}
	return host, port, nil
}

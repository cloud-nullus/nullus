package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const objectStorageBucketJobName = "nullus-gitlab-bucket-bootstrap"

var gitLabRequiredBuckets = []string{
	"gitlab-artifacts",
	"git-lfs",
	"gitlab-uploads",
	"gitlab-packages",
	"gitlab-pages",
}

func (o *Orchestrator) currentStackConfig() *domain.StackConfig {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stackConfig == nil {
		return nil
	}
	cfg := *o.stackConfig
	return &cfg
}

func (o *Orchestrator) ensureGitLabObjectStorageBuckets(ctx context.Context, namespace string) error {
	target, err := o.resolveObjectStorageTarget(ctx, namespace)
	if err != nil {
		return err
	}

	script := buildBucketBootstrapScript(target.Endpoint, target.AccessKey, target.SecretKey, gitLabRequiredBuckets)
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
      - name: mc
        image: minio/mc:RELEASE.2025-05-21T01-59-54Z
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, objectStorageBucketJobName, namespace, indentYAML(script, 12))

	_, _ = o.runKubectl(ctx, "delete", "job", objectStorageBucketJobName, "-n", namespace, "--ignore-not-found=true")
	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return err
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace, "--for=condition=complete", "--timeout=180s", "job/"+objectStorageBucketJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+objectStorageBucketJobName)
		return fmt.Errorf("bucket bootstrap job failed: %w (%s)", err, strings.TrimSpace(string(logs)))
	}
	if _, err := o.runKubectl(ctx, "delete", "job", objectStorageBucketJobName, "-n", namespace, "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("cleanup bucket bootstrap job: %w", err)
	}

	return nil
}

type objectStorageTarget struct {
	Endpoint  string
	AccessKey string
	SecretKey string
}

func (o *Orchestrator) resolveObjectStorageTarget(ctx context.Context, namespace string) (objectStorageTarget, error) {
	cfg := o.currentStackConfig()
	if cfg == nil || cfg.Storage == nil {
		return objectStorageTarget{
			Endpoint:  fmt.Sprintf("http://nullus-minio.%s.svc.cluster.local:9000", namespace),
			AccessKey: "nullus-admin",
			SecretKey: "nullus-minio-secret",
		}, nil
	}

	target := cfg.Storage.ObjectStorage
	if strings.TrimSpace(target.Mode) != "existing-connect" {
		return objectStorageTarget{
			Endpoint:  fmt.Sprintf("http://nullus-minio.%s.svc.cluster.local:9000", namespace),
			AccessKey: "nullus-admin",
			SecretKey: "nullus-minio-secret",
		}, nil
	}

	endpoint := strings.TrimSpace(target.Endpoint)
	if endpoint == "" {
		return objectStorageTarget{}, fmt.Errorf("storage.object_storage.endpoint is required for existing-connect")
	}

	accessKey := strings.TrimSpace(target.AuthID)
	secretKey := strings.TrimSpace(target.AuthPasswordKey)
	if strings.TrimSpace(target.AccessSecretRef) != "" {
		resolvedAccessKey, resolvedSecretKey, err := o.resolveObjectStorageCredentialsFromSecret(ctx, namespace, target)
		if err != nil {
			return objectStorageTarget{}, err
		}
		if strings.TrimSpace(resolvedAccessKey) != "" {
			accessKey = resolvedAccessKey
		}
		if strings.TrimSpace(resolvedSecretKey) != "" {
			secretKey = resolvedSecretKey
		}
	}

	if accessKey == "" || secretKey == "" {
		return objectStorageTarget{}, fmt.Errorf("storage.object_storage credentials could not be resolved for existing-connect")
	}

	return objectStorageTarget{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey}, nil
}

func (o *Orchestrator) resolveObjectStorageCredentialsFromSecret(ctx context.Context, namespace string, target domain.StorageTarget) (string, string, error) {
	secretRef := strings.TrimSpace(target.AccessSecretRef)
	if secretRef == "" {
		return "", "", fmt.Errorf("storage.object_storage.access_secret_ref is empty")
	}
	passwordKeyRef := strings.TrimSpace(target.AuthPasswordKey)
	if passwordKeyRef == "" {
		return "", "", fmt.Errorf("storage.object_storage.auth_password_key is empty")
	}

	accessKeyRef := strings.TrimSpace(target.AuthID)
	if accessKeyRef == "" {
		accessKeyRef = "accessKey"
	}

	accessKey, accessErr := o.getSecretValue(ctx, namespace, secretRef, accessKeyRef)
	if accessErr != nil {
		accessKey = strings.TrimSpace(target.AuthID)
	}
	secretKey, secretErr := o.getSecretValue(ctx, namespace, secretRef, passwordKeyRef)
	if secretErr != nil {
		secretKey = strings.TrimSpace(target.AuthPasswordKey)
	}
	if strings.TrimSpace(accessKey) == "" || strings.TrimSpace(secretKey) == "" {
		return "", "", fmt.Errorf("failed to resolve object storage credentials from secret %q (access=%q secret=%q)", secretRef, accessKeyRef, passwordKeyRef)
	}

	return strings.TrimSpace(accessKey), strings.TrimSpace(secretKey), nil
}

func (o *Orchestrator) getSecretValue(ctx context.Context, namespace, name, key string) (string, error) {
	jsonpath := fmt.Sprintf("jsonpath={.data.%s}", escapeJSONPathKey(key))
	out, err := o.runKubectl(ctx, "get", "secret", name, "-n", namespace, "-o", jsonpath)
	if err != nil {
		return "", err
	}
	encoded := strings.TrimSpace(string(out))
	if encoded == "" {
		return "", fmt.Errorf("key %q missing in secret %q", key, name)
	}
	decoded, err := decodeBase64String(encoded)
	if err != nil {
		return "", fmt.Errorf("decode key %q from secret %q: %w", key, name, err)
	}
	return decoded, nil
}

func escapeJSONPathKey(key string) string {
	replacer := strings.NewReplacer(".", "\\.", "-", "\\-")
	return replacer.Replace(key)
}

func buildBucketBootstrapScript(endpoint, accessKey, secretKey string, buckets []string) string {
	var sb strings.Builder
	sb.WriteString("set -e\n")
	sb.WriteString(fmt.Sprintf("mc alias set target %q %q %q >/dev/null\n", endpoint, accessKey, secretKey))
	for _, bucket := range buckets {
		sb.WriteString(fmt.Sprintf("mc mb --ignore-existing target/%s >/dev/null\n", bucket))
		sb.WriteString(fmt.Sprintf("mc ls target/%s >/dev/null\n", bucket))
	}
	return sb.String()
}

func decodeBase64String(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

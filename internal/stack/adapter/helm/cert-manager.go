package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func (o *Orchestrator) certManagerNamespaceCandidates() []string {
	candidates := []string{"cert-manager", "nullus", "default"}
	if releaseNamespace, err := o.detectCertManagerReleaseNamespaceFromCRD(context.Background()); err == nil && strings.TrimSpace(releaseNamespace) != "" {
		trimmed := strings.TrimSpace(releaseNamespace)
		for _, candidate := range candidates {
			if candidate == trimmed {
				goto includeOrchestratorNamespace
			}
		}
		candidates = append([]string{trimmed}, candidates...)
	}

includeOrchestratorNamespace:
	if ns := strings.TrimSpace(o.namespace); ns != "" {
		for _, candidate := range candidates {
			if candidate == ns {
				return candidates
			}
		}
		candidates = append([]string{ns}, candidates...)
	}
	return candidates
}

func (o *Orchestrator) detectCertManagerNamespace(ctx context.Context) (string, error) {
	deployments := []string{
		"deployment/cert-manager",
		"deployment/cert-manager-webhook",
		"deployment/cert-manager-cainjector",
	}

	for _, namespace := range o.certManagerNamespaceCandidates() {
		allFound := true
		for _, deployment := range deployments {
			if _, err := o.runKubectl(ctx, "get", "-n", namespace, deployment); err != nil {
				allFound = false
				break
			}
		}
		if allFound {
			return namespace, nil
		}
	}

	return "", fmt.Errorf("cert-manager deployments not found in candidate namespaces")
}

func (o *Orchestrator) detectCertManagerReleaseNamespaceFromCRD(ctx context.Context) (string, error) {
	output, err := o.runKubectl(ctx, "get", "crd", "certificaterequests.cert-manager.io", "-o", "jsonpath={.metadata.annotations.meta\\.helm\\.sh/release-namespace}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (o *Orchestrator) bootstrapInternalCA(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	manifest := o.internalCAManifest(namespace)
	if strings.TrimSpace(manifest) == "" {
		return nil
	}
	return o.applyManifest(ctx, namespace, manifest)
}

func (o *Orchestrator) waitForCertManagerInstallation(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	requiredCRDs := []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"issuers.cert-manager.io",
	}
	for _, crd := range requiredCRDs {
		if err := o.waitForKubectlGet(ctx, "crd", crd); err != nil {
			return fmt.Errorf("cert-manager crd %s not ready: %w", crd, err)
		}
	}

	certManagerNamespace, err := o.detectCertManagerNamespace(ctx)
	if err != nil {
		return err
	}

	deployments := []string{
		"deployment/cert-manager",
		"deployment/cert-manager-webhook",
		"deployment/cert-manager-cainjector",
	}
	for _, deployment := range deployments {
		if err := o.waitForKubectlGet(ctx, "-n", certManagerNamespace, deployment); err != nil {
			return fmt.Errorf("cert-manager deployment %s not found: %w", deployment, err)
		}
		if _, err := o.runKubectl(ctx, "rollout", "status", "-n", certManagerNamespace, deployment, "--timeout=180s"); err != nil {
			return fmt.Errorf("cert-manager deployment %s not ready: %w", deployment, err)
		}
	}

	if err := o.waitForCertManagerWebhookTrust(ctx); err != nil {
		return fmt.Errorf("cert-manager webhook trust not stabilized: %w", err)
	}

	if err := o.waitForCertManagerStartupAPICheck(ctx, certManagerNamespace); err != nil {
		return fmt.Errorf("cert-manager startup API check not complete: %w", err)
	}

	return nil
}

func (o *Orchestrator) waitForCertManagerWebhookTrust(ctx context.Context) error {
	jsonpaths := []string{
		"{.webhooks[0].clientConfig.caBundle}",
		"{.webhooks[1].clientConfig.caBundle}",
	}
	resources := []string{
		"mutatingwebhookconfiguration/cert-manager-webhook",
		"validatingwebhookconfiguration/cert-manager-webhook",
	}

	for _, resource := range resources {
		ready := false
		for _, jsonpath := range jsonpaths {
			if err := o.waitForKubectlNonEmptyOutput(ctx, "get", resource, "-o", "jsonpath="+jsonpath); err == nil {
				ready = true
				break
			}
		}
		if !ready {
			return fmt.Errorf("cabundle not injected for %s", resource)
		}
	}

	return nil
}

func (o *Orchestrator) waitForCertManagerStartupAPICheck(ctx context.Context, namespace string) error {
	const resource = "job/cert-manager-startupapicheck"

	if err := o.waitForKubectlGet(ctx, "-n", namespace, resource); err != nil {
		if isKubectlNotFoundError(err) {
			slog.Info("cert-manager startup API check job not found; skipping wait", "namespace", namespace)
			return nil
		}
		return err
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace, "--for=condition=complete", "--timeout=180s", resource); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) internalCAManifest(namespace string) string {
	issuerName := defaultInternalCAIssuer
	secretName := defaultInternalCASecretName
	certName := defaultInternalCACertName

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	if cfg != nil && cfg.AccessDomainTLS != nil {
		if strings.TrimSpace(cfg.AccessDomainTLS.IssuerName) != "" {
			slog.Info("ignoring access-domain TLS issuer override for internal CA bootstrap", "issuer", cfg.AccessDomainTLS.IssuerName)
		}
	}

	return fmt.Sprintf(`apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: %s
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %s
  namespace: %s
spec:
  isCA: true
  commonName: nullus-internal-root
  secretName: %s
  duration: 87600h
  renewBefore: 720h
  issuerRef:
    name: %s
    kind: ClusterIssuer
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: %s
spec:
  ca:
    secretName: %s
`, defaultSelfSignedBootstrapIssuer, certName, namespace, secretName, defaultSelfSignedBootstrapIssuer, issuerName, secretName)
}

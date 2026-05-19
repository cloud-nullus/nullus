package helm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

var installGatewayOCIRelease = installOCIChartWithHelmCLI
var bootstrapInternalCAInstallation = func(ctx context.Context, o *Orchestrator, namespace string) error {
	return o.bootstrapInternalCA(ctx, namespace)
}
var waitForCertManagerInstallation = func(ctx context.Context, o *Orchestrator) error {
	return o.waitForCertManagerInstallation(ctx)
}
var verifyReleaseRuntimeReadiness = func(ctx context.Context, o *Orchestrator, step, releaseName, namespace string) error {
	return o.verifyReleaseRuntimeReadiness(ctx, step, releaseName, namespace)
}
var checkExistingCertManagerInstallation = func(ctx context.Context, o *Orchestrator) (bool, error) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return false, nil
	}

	requiredCRDs := []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"issuers.cert-manager.io",
	}
	for _, crd := range requiredCRDs {
		if _, err := o.runKubectl(ctx, "get", "crd", crd); err != nil {
			return false, nil
		}
	}

	if _, err := o.detectCertManagerNamespace(ctx); err != nil {
		return false, nil
	}

	return true, nil
}

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

func (o *Orchestrator) filterOptionalGatewayPolicies(ctx context.Context, manifest string) (string, bool, error) {
	if strings.TrimSpace(manifest) == "" {
		return manifest, false, nil
	}
	if _, err := o.runKubectl(ctx, "get", "crd", "backendtlspolicies.gateway.networking.k8s.io"); err == nil {
		return manifest, false, nil
	}
	return filterGatewayManifestDocuments(manifest, func(apiVersion, kind string) bool {
		return strings.HasPrefix(apiVersion, "gateway.networking.k8s.io/") && kind == "BackendTLSPolicy"
	})
}

func (o *Orchestrator) reconcileGatewayDataPlaneTLSSecret(ctx context.Context, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	if err := o.waitForKubectlGet(ctx, "-n", namespace, "secret/"+defaultEnvoyControlPlaneSecret); err != nil {
		return err
	}

	caCRT, err := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "ca.crt")
	if err != nil {
		fallbackCA, fallbackErr := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "tls.crt")
		if fallbackErr != nil {
			return err
		}
		caCRT = fallbackCA
	}
	tlsCRT, err := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "tls.crt")
	if err != nil {
		return err
	}
	tlsKey, err := o.secretDataField(ctx, namespace, defaultEnvoyControlPlaneSecret, "tls.key")
	if err != nil {
		return err
	}

	matches, err := o.secretDataMatches(ctx, namespace, defaultEnvoyDataPlaneTLSSecret, map[string]string{
		"ca.crt":  caCRT,
		"tls.crt": tlsCRT,
		"tls.key": tlsKey,
	})
	if err != nil {
		matches = false
	}

	secretManifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
  labels:
    control-plane: envoy-gateway
type: kubernetes.io/tls
data:
  ca.crt: %s
  tls.crt: %s
  tls.key: %s
`, defaultEnvoyDataPlaneTLSSecret, namespace, caCRT, tlsCRT, tlsKey)

	if err := o.applyManifest(ctx, namespace, secretManifest); err != nil {
		return err
	}

	if !matches {
		_, _ = o.runKubectl(ctx, "delete", "pod", "-n", namespace, "-l", "app.kubernetes.io/name=envoy", "--ignore-not-found=true")
	}

	return nil
}
func (o *Orchestrator) tryReconcileGatewayDataPlaneTLSSecret(ctx context.Context, namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	if _, err := o.runKubectl(ctx, "get", "secret", defaultEnvoyControlPlaneSecret, "-n", namespace, "-o", "name"); err != nil {
		return nil
	}
	return o.reconcileGatewayDataPlaneTLSSecret(ctx, namespace)
}
func (o *Orchestrator) secretDataField(ctx context.Context, namespace, secretName, key string) (string, error) {
	goTemplate := fmt.Sprintf("go-template={{ index .data %q }}", key)
	output, err := o.runKubectl(ctx, "get", "secret", secretName, "-n", namespace, "-o", goTemplate)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("secret %s/%s missing data key %s", namespace, secretName, key)
	}
	return value, nil
}

func (o *Orchestrator) secretDataMatches(ctx context.Context, namespace, secretName string, expected map[string]string) (bool, error) {
	if len(expected) == 0 {
		return true, nil
	}
	for key, expectedValue := range expected {
		actual, err := o.secretDataField(ctx, namespace, secretName, key)
		if err != nil {
			return false, err
		}
		if actual != strings.TrimSpace(expectedValue) {
			return false, nil
		}
	}
	return true, nil
}

func (o *Orchestrator) releasePodSnapshot(ctx context.Context, releaseName, namespace string) (*podListSnapshot, error) {
	selectors := releaseLabelSelectors(releaseName)
	for _, selector := range selectors {
		output, err := o.runKubectl(ctx,
			"get", "pods",
			"-n", namespace,
			"-l", selector,
			"-o", "json",
		)
		if err != nil {
			return nil, err
		}

		var snapshot podListSnapshot
		if err := json.Unmarshal(output, &snapshot); err != nil {
			return nil, err
		}
		if len(snapshot.Items) > 0 {
			return &snapshot, nil
		}
	}

	return &podListSnapshot{}, nil
}

func (o *Orchestrator) waitForReleaseRollouts(ctx context.Context, releaseName, namespace string) error {
	resources := []string{"deployments", "statefulsets", "daemonsets"}
	selectors := releaseLabelSelectors(releaseName)
	rolloutTimeout := "180s"
	if strings.TrimSpace(releaseName) == "gitlab" {
		rolloutTimeout = "600s"
	}
	for _, resourceType := range resources {
		for _, selector := range selectors {
			output, err := o.runKubectl(ctx,
				"get", resourceType,
				"-n", namespace,
				"-l", selector,
				"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`,
			)
			if err != nil {
				return err
			}
			for _, rawName := range strings.Split(string(output), "\n") {
				name := strings.TrimSpace(rawName)
				if name == "" {
					continue
				}
				resource := strings.TrimSuffix(resourceType, "s") + "/" + name
				if _, err := o.runKubectl(ctx, "rollout", "status", "-n", namespace, resource, "--timeout="+rolloutTimeout); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (o *Orchestrator) verifyReleaseRuntimeReadiness(ctx context.Context, step, releaseName, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	if err := o.waitForReleaseRollouts(ctx, releaseName, namespace); err != nil {
		return err
	}

	snapshot, err := o.releasePodSnapshot(ctx, releaseName, namespace)
	if err != nil {
		return err
	}
	if len(snapshot.Items) == 0 {
		return fmt.Errorf("no pods found for release %s in namespace %s", releaseName, namespace)
	}

	for _, pod := range snapshot.Items {
		phase := strings.TrimSpace(strings.ToLower(pod.Status.Phase))
		if phase == "succeeded" {
			continue
		}
		if phase != "running" {
			return fmt.Errorf("pod %s phase=%s", pod.Metadata.Name, strings.TrimSpace(pod.Status.Phase))
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			return fmt.Errorf("pod %s has no container status yet", pod.Metadata.Name)
		}
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready {
				return fmt.Errorf("pod %s container %s not ready", pod.Metadata.Name, container.Name)
			}
		}
	}

	_ = step
	return nil
}

func (o *Orchestrator) cleanupResidualReleaseResources(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	resourceKinds := []string{"deploy", "sts", "ds", "job", "cronjob", "svc", "cm", "secret", "pvc"}
	seen := map[string]struct{}{}
	var errs []error

	for _, step := range o.orderedStep {
		spec, ok := o.chartConfig[step]
		if !ok {
			continue
		}
		spec = o.resolveChartSpecForStep(step, spec)
		releaseName := strings.TrimSpace(o.releaseNameForSpec(spec))
		if releaseName == "" {
			continue
		}
		namespace := strings.TrimSpace(o.namespace)
		if strings.TrimSpace(spec.Namespace) != "" {
			namespace = strings.TrimSpace(spec.Namespace)
		}
		if step == stepInstallingCertManager {
			if detectedNS, err := o.detectCertManagerReleaseNamespaceFromCRD(ctx); err == nil && strings.TrimSpace(detectedNS) != "" {
				namespace = strings.TrimSpace(detectedNS)
			}
		}
		if namespace == "" {
			continue
		}

		key := namespace + "::" + releaseName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		for _, kind := range resourceKinds {
			selector := "app.kubernetes.io/instance=" + releaseName
			if _, err := o.runKubectl(ctx, "delete", kind, "-n", namespace, "-l", selector, "--ignore-not-found"); err != nil {
				errs = append(errs, fmt.Errorf("delete %s for release %s in namespace %s: %w", kind, releaseName, namespace, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
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

func looksLikeKubeconfig(kubeconfig []byte) bool {
	if len(kubeconfig) == 0 {
		return false
	}
	text := string(kubeconfig)
	return strings.Contains(text, "apiVersion:") && strings.Contains(text, "clusters:")
}

func isReleaseNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "release: not found") || strings.Contains(msg, "release not loaded")
}

func isKubectlNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "notfound") || strings.Contains(msg, "not found")
}

func releaseLabelSelectors(releaseName string) []string {
	name := strings.TrimSpace(releaseName)
	if name == "" {
		return []string{""}
	}
	return []string{
		fmt.Sprintf("app.kubernetes.io/instance=%s", name),
		fmt.Sprintf("release=%s", name),
	}
}

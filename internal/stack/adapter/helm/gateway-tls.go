package helm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

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

func normalizeGatewayBackendServiceAliases(manifest string) (string, bool, error) {
	aliasByService := map[string]string{
		"grafana-svc":    "grafana",
		"prometheus-svc": "kube-prometheus-stack-prometheus",
	}

	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	docs := make([]string, 0)
	normalizedAny := false

	for {
		var doc any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return "", false, err
		}
		if doc == nil {
			continue
		}

		apiVersion := yamlDocumentStringField(doc, "apiVersion")
		kind := yamlDocumentStringField(doc, "kind")
		if strings.HasPrefix(apiVersion, "gateway.networking.k8s.io/") && kind == "HTTPRoute" {
			if normalizeHTTPRouteBackendRefs(doc, aliasByService) {
				normalizedAny = true
			}
		}

		encoded, err := yaml.Marshal(doc)
		if err != nil {
			return "", false, err
		}
		trimmed := strings.TrimSpace(string(encoded))
		if trimmed != "" {
			docs = append(docs, trimmed)
		}
	}

	return strings.Join(docs, "\n---\n"), normalizedAny, nil
}

func normalizeHTTPRouteBackendRefs(doc any, aliasByService map[string]string) bool {
	root, ok := doc.(map[string]any)
	if !ok {
		return false
	}
	spec, ok := root["spec"].(map[string]any)
	if !ok {
		return false
	}
	rules, ok := spec["rules"].([]any)
	if !ok {
		return false
	}

	normalized := false
	for _, rawRule := range rules {
		rule, ok := rawRule.(map[string]any)
		if !ok {
			continue
		}
		backendRefs, ok := rule["backendRefs"].([]any)
		if !ok {
			continue
		}
		for _, rawBackendRef := range backendRefs {
			backendRef, ok := rawBackendRef.(map[string]any)
			if !ok {
				continue
			}
			name, ok := backendRef["name"].(string)
			if !ok {
				continue
			}
			replacement, ok := aliasByService[strings.TrimSpace(name)]
			if !ok {
				continue
			}
			backendRef["name"] = replacement
			normalized = true
		}
	}

	return normalized
}

func filterGatewayManifestDocuments(manifest string, skip func(apiVersion, kind string) bool) (string, bool, error) {
	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	kept := make([]string, 0)
	skippedAny := false

	for {
		var doc any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return "", false, err
		}
		if doc == nil {
			continue
		}

		apiVersion := yamlDocumentStringField(doc, "apiVersion")
		kind := yamlDocumentStringField(doc, "kind")
		if skip != nil && skip(apiVersion, kind) {
			skippedAny = true
			continue
		}

		encoded, err := yaml.Marshal(doc)
		if err != nil {
			return "", false, err
		}
		trimmed := strings.TrimSpace(string(encoded))
		if trimmed != "" {
			kept = append(kept, trimmed)
		}
	}

	var buffer bytes.Buffer
	for index, doc := range kept {
		if index > 0 {
			buffer.WriteString("\n---\n")
		}
		buffer.WriteString(doc)
	}
	return buffer.String(), skippedAny, nil
}

func yamlDocumentStringField(doc any, key string) string {
	switch typed := doc.(type) {
	case map[string]any:
		if value, ok := typed[key].(string); ok {
			return strings.TrimSpace(value)
		}
	case map[any]any:
		for rawKey, rawValue := range typed {
			keyString, ok := rawKey.(string)
			if !ok || keyString != key {
				continue
			}
			if value, ok := rawValue.(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

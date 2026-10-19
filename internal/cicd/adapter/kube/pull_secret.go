package kube

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// ImagePullSecretNameFor 는 앱 네임스페이스에 만드는 그 앱의 pull 자격증명 이름이다.
// 스캐폴딩된 Deployment 가 이 이름을 참조한다. 백업이 같은 자격증명을 담아야 하므로 공용 도메인이 갖는다.
func ImagePullSecretNameFor(app string) string {
	return shareddomain.AppImagePullSecretNameFor(app)
}

// ImagePullSecretSpec 은 pull 자격증명 요청이다.
type ImagePullSecretSpec struct {
	// AppName 은 자격증명을 쓸 앱이다. Secret 이름이 앱마다 따로다(ImagePullSecretNameFor).
	AppName      string
	Namespace    string
	RegistryHost string
	Username     string
	Password     string
}

// RenderImagePullSecret 은 kubelet 이 private 레지스트리에서 이미지를 받아올
// dockerconfigjson Secret 을 만든다.
//
// 없으면 pull 이 "failed to fetch anonymous token: 403" 으로 실패한다.
func RenderImagePullSecret(spec ImagePullSecretSpec) (string, error) {
	app := strings.TrimSpace(spec.AppName)
	if app == "" {
		return "", fmt.Errorf("app name is required")
	}
	ns := strings.TrimSpace(spec.Namespace)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}
	host := strings.TrimSpace(spec.RegistryHost)
	if host == "" {
		return "", fmt.Errorf("registry host is required")
	}
	password := strings.TrimSpace(spec.Password)
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	username := strings.TrimSpace(spec.Username)
	if username == "" {
		username = "nullus-puller"
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	cfg := map[string]any{
		"auths": map[string]any{
			host: map[string]any{
				"username": username,
				"password": password,
				"auth":     auth,
			},
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal dockerconfigjson: %w", err)
	}

	doc := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"type":       shareddomain.AppImagePullSecretType,
		"metadata": map[string]any{
			"name":      ImagePullSecretNameFor(app),
			"namespace": ns,
			"labels": map[string]any{
				shareddomain.AppManagedByLabel: shareddomain.AppManagedByCICD,
			},
		},
		// stringData 를 쓴다 — base64 는 쿠버네티스가 한다.
		"stringData": map[string]any{
			".dockerconfigjson": string(raw),
		},
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal image pull secret: %w", err)
	}
	return string(out), nil
}

// RenderNamespace 는 네임스페이스 매니페스트를 만든다.
//
// pull 자격증명을 넣으려면 네임스페이스가 먼저 있어야 하는데, Argo CD 의
// CreateNamespace 는 첫 동기화 시점에야 만든다.
func RenderNamespace(name string) (string, error) {
	ns := strings.TrimSpace(name)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}
	doc := map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name": ns,
			"labels": map[string]any{
				shareddomain.AppManagedByLabel: shareddomain.AppManagedByCICD,
			},
		},
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal namespace: %w", err)
	}
	return string(out), nil
}

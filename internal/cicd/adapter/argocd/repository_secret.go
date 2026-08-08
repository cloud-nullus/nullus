package argocd

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// repositorySecretTypeLabel 은 Argo CD 가 저장소 자격증명으로 인식하는 라벨이다.
// 이 라벨이 없으면 Secret 이 그냥 무시된다.
const repositorySecretTypeLabel = "argocd.argoproj.io/secret-type"

// defaultRepositoryUsername 은 토큰 인증 시 쓰는 사용자명이다.
// GitLab 은 토큰을 비밀번호로 받고 사용자명은 임의값을 허용한다.
const defaultRepositoryUsername = "nullus-argocd"

// RepositorySecretSpec 은 저장소 자격증명 Secret 요청이다.
type RepositorySecretSpec struct {
	AppName       string
	ArgoNamespace string
	RepoURL       string
	Username      string
	Password      string
}

// RenderRepositorySecret 은 Argo CD 저장소 자격증명 Secret 을 만든다.
//
// Application 만 만들고 자격증명을 주지 않으면 private 저장소에서
// "authentication required" 로 동기화가 실패한다.
func RenderRepositorySecret(spec RepositorySecretSpec) (string, error) {
	app := strings.TrimSpace(spec.AppName)
	if app == "" {
		return "", fmt.Errorf("app_name is required")
	}
	ns := strings.TrimSpace(spec.ArgoNamespace)
	if ns == "" {
		return "", fmt.Errorf("argo namespace is required")
	}
	repoURL := strings.TrimSpace(spec.RepoURL)
	if repoURL == "" {
		return "", fmt.Errorf("repo_url is required")
	}
	password := strings.TrimSpace(spec.Password)
	if password == "" {
		return "", fmt.Errorf("password is required — 자격증명 없이는 private 저장소를 동기화할 수 없습니다")
	}

	doc := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			// 앱마다 이름이 달라야 서로의 자격증명을 덮어쓰지 않는다.
			"name":      "nullus-repo-" + app,
			"namespace": ns,
			"labels": map[string]any{
				repositorySecretTypeLabel:      "repository",
				"app.kubernetes.io/name":       app,
				"app.kubernetes.io/managed-by": "nullus-cicd",
			},
		},
		// stringData 를 쓴다 — base64 는 쿠버네티스가 한다.
		// 직접 인코딩하면 이중 인코딩이 되어 인증이 실패한다.
		"stringData": map[string]any{
			"type":     "git",
			"url":      repoURL,
			"username": firstNonEmpty(spec.Username, defaultRepositoryUsername),
			"password": password,
		},
	}

	raw, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal argocd repository secret: %w", err)
	}
	return string(raw), nil
}

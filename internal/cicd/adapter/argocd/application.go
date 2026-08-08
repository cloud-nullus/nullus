// Package argocd 는 Argo CD Application 리소스를 만든다.
//
// 스택이 Argo CD 를 설치해도 Application 이 없으면 아무것도 동기화하지 않는다.
// 앱 저장소의 deploy/ 경로를 가리키는 Application 을 만들어야, CI 가 갱신한
// 이미지 태그가 실제 배포로 이어진다.
package argocd

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	// DefaultManifestPath 는 앱 저장소에서 매니페스트가 있는 경로다.
	// 스캐폴딩이 deploy/ 에 파일을 넣으므로 기본값을 맞춘다.
	DefaultManifestPath = "deploy"

	// DefaultTargetRevision 은 추적할 기본 브랜치다.
	DefaultTargetRevision = "main"

	// DefaultArgoProject 는 Argo CD 의 기본 프로젝트다.
	DefaultArgoProject = "default"

	// inClusterServer 는 Argo CD 가 자기 클러스터를 가리킬 때 쓰는 주소다.
	inClusterServer = "https://kubernetes.default.svc"
)

// ApplicationSpec 은 Application 렌더 요청이다.
type ApplicationSpec struct {
	AppName string
	// ArgoNamespace 는 Argo CD 가 설치된 네임스페이스다.
	// Application 이 이 네임스페이스에 있어야 컨트롤러가 인식한다.
	ArgoNamespace string
	// ArgoProject 는 Argo CD 프로젝트다. 비면 default.
	ArgoProject string
	// RepoURL 은 앱 저장소 주소다.
	RepoURL string
	// Path 는 저장소 안 매니페스트 경로다. 비면 deploy.
	Path string
	// TargetRevision 은 추적할 브랜치/태그다. 비면 main.
	TargetRevision string
	// DestinationNamespace 는 배포 대상 네임스페이스다.
	DestinationNamespace string
}

// RenderApplication 은 Application 매니페스트 YAML 을 만든다.
//
// 구조체를 map 으로 조립한 뒤 직렬화한다 — 문자열 템플릿은 들여쓰기 실수가
// 조용히 통과할 수 있고, 그 결과는 클러스터에 적용할 때야 드러난다.
func RenderApplication(spec ApplicationSpec) (string, error) {
	app := strings.TrimSpace(spec.AppName)
	if app == "" {
		return "", fmt.Errorf("app_name is required")
	}
	argoNS := strings.TrimSpace(spec.ArgoNamespace)
	if argoNS == "" {
		return "", fmt.Errorf("argo namespace is required")
	}
	repoURL := strings.TrimSpace(spec.RepoURL)
	if repoURL == "" {
		return "", fmt.Errorf("repo_url is required")
	}

	destNS := strings.TrimSpace(spec.DestinationNamespace)
	if destNS == "" {
		destNS = "default"
	}

	doc := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      app,
			"namespace": argoNS,
			"labels": map[string]any{
				"app.kubernetes.io/name":       app,
				"app.kubernetes.io/managed-by": "nullus-cicd",
			},
		},
		"spec": map[string]any{
			"project": firstNonEmpty(spec.ArgoProject, DefaultArgoProject),
			"source": map[string]any{
				"repoURL":        repoURL,
				"path":           firstNonEmpty(spec.Path, DefaultManifestPath),
				"targetRevision": firstNonEmpty(spec.TargetRevision, DefaultTargetRevision),
			},
			"destination": map[string]any{
				"server":    inClusterServer,
				"namespace": destNS,
			},
			"syncPolicy": map[string]any{
				// 자동 동기화가 없으면 CI 가 태그를 갱신해도 배포가 일어나지 않는다.
				"automated": map[string]any{
					"prune":    true,
					"selfHeal": true,
				},
				// 대상 네임스페이스가 없으면 첫 동기화가 실패한다.
				"syncOptions": []any{"CreateNamespace=true"},
			},
		},
	}

	raw, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal argocd application: %w", err)
	}
	return string(raw), nil
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

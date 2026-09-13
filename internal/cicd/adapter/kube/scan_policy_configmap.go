package kube

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// RenderScanPolicyConfigMap 은 CI 변수 저장소가 없는 CI(Jenkins)가 읽는 스캔 정책
// ConfigMap 을 만든다.
//
// 스택 네임스페이스에 하나다. Jenkinsfile 의 스캐너 컨테이너가 envFrom 으로
// 읽으므로, 정책을 바꾸면 그 스택의 Jenkins 파이프라인 전체가 다음 실행부터 따른다.
// 비밀이 아니라 Secret 이 아니다.
func RenderScanPolicyConfigMap(namespace string, vars []port.ProjectVariable) (string, error) {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return "", fmt.Errorf("스캔 정책 ConfigMap 의 네임스페이스가 필요합니다")
	}
	data := make(map[string]string, len(vars))
	for _, v := range vars {
		data[v.Key] = v.Value
	}
	doc := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      port.ScanPolicyConfigMapName,
			"namespace": ns,
			"labels": map[string]any{
				"app.kubernetes.io/managed-by": "nullus-cicd",
			},
		},
		"data": data,
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal scan policy configmap: %w", err)
	}
	return string(out), nil
}

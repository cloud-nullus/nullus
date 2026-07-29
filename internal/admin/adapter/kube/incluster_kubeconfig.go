package kube

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"k8s.io/client-go/rest"
)

// InClusterKubeconfig 는 Nullus 가 떠 있는 클러스터를 가리키는 kubeconfig 를 만든다.
//
// 자기 클러스터 등록(self-registration)에 쓰인다. 에어갭 무인 설치와 단일
// 클러스터 배포에서는 운영자가 kubeconfig 를 업로드할 대상이 자기 자신이라
// 업로드 단계 자체가 불필요하다.
//
// 파드에 마운트된 ServiceAccount 토큰과 CA 로 구성하므로 새로운 자격증명이
// 생기지 않는다. 토큰은 kubelet 이 자동 로테이션하는 값이다.
func InClusterKubeconfig() ([]byte, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster 구성을 읽지 못했습니다 (클러스터 밖에서 실행 중일 수 있음): %w", err)
	}

	token := strings.TrimSpace(cfg.BearerToken)
	if token == "" && cfg.BearerTokenFile != "" {
		raw, readErr := os.ReadFile(cfg.BearerTokenFile) // #nosec G304 -- kubelet 이 마운트한 고정 경로
		if readErr != nil {
			return nil, fmt.Errorf("ServiceAccount 토큰을 읽지 못했습니다: %w", readErr)
		}
		token = strings.TrimSpace(string(raw))
	}
	if token == "" {
		return nil, fmt.Errorf("ServiceAccount 토큰이 비어 있습니다")
	}

	caData := cfg.CAData
	if len(caData) == 0 && cfg.CAFile != "" {
		raw, readErr := os.ReadFile(cfg.CAFile) // #nosec G304 -- kubelet 이 마운트한 고정 경로
		if readErr != nil {
			return nil, fmt.Errorf("클러스터 CA 를 읽지 못했습니다: %w", readErr)
		}
		caData = raw
	}
	if len(caData) == 0 {
		return nil, fmt.Errorf("클러스터 CA 가 비어 있습니다")
	}

	return []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
  - name: nullus-self
    cluster:
      server: %s
      certificate-authority-data: %s
contexts:
  - name: nullus-self
    context:
      cluster: nullus-self
      user: nullus-self
current-context: nullus-self
users:
  - name: nullus-self
    user:
      token: %s
`, cfg.Host, base64.StdEncoding.EncodeToString(caData), token)), nil
}

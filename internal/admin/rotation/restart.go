package rotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 회전 후 반영(restart_required) 전략.
//
// 시크릿이 회전되고 ESO 가 Kubernetes Secret 을 갱신해도, 이미 떠 있는 파드는
// 소비 방식에 따라 이전 값을 계속 붙들고 있을 수 있다. 이 차이를 흡수하지 않으면
// "회전은 성공했는데 인증은 깨진" 상태가 되고, 증상이 회전과 멀리 떨어져
// 나타나 원인 추적이 어렵다.
//
//   - GitLab Runner: config.toml 을 기동 시 1회만 렌더링 → 재시작 필요
//   - ArgoCD repository Secret: 매 요청 시점에 읽음 → 재시작 불필요

// restartRequiredProviders 는 회전 후 소비자 재시작이 필요한 provider 다.
// 모르는 provider 는 재시작하지 않는다 — 불필요한 재기동이 더 위험하다.
var restartRequiredProviders = map[string]bool{
	"gitlab":          true,
	"gitlab-ce":       true,
	"gitlab-ci":       true,
	"gitlab-runner":   true,
	"gitlab-registry": true,
	"postgresql":      true,
	"minio":           true,
	"harbor":          true,
	"grafana":         true,

	// ArgoCD 는 repository Secret 을 실시간으로 읽으므로 재시작이 필요 없다.
	"argocd":  false,
	"argo-cd": false,
}

// RequiresRestart 는 provider 소비자가 회전 후 재시작을 필요로 하는지 알려준다.
func RequiresRestart(provider string) bool {
	return restartRequiredProviders[strings.ToLower(strings.TrimSpace(provider))]
}

// WorkloadRef 는 재시작 대상 워크로드다.
type WorkloadRef struct {
	Kind string
	Name string
}

// WorkloadRestarter 는 회전 후 소비자를 rolling restart 한다.
//
// lookup/restart 를 필드로 두어 테스트에서 교체할 수 있게 한다.
type WorkloadRestarter struct {
	lookup  func(ctx context.Context, namespace, provider string) ([]WorkloadRef, error)
	restart func(ctx context.Context, namespace, kind, name string) error
}

// NewWorkloadRestarter 는 kubeconfig 로 동작하는 restarter 를 만든다.
func NewWorkloadRestarter(kubeconfig []byte, runKubectl KubectlRunner) *WorkloadRestarter {
	return &WorkloadRestarter{
		lookup: func(ctx context.Context, namespace, provider string) ([]WorkloadRef, error) {
			return lookupWorkloads(ctx, kubeconfig, runKubectl, namespace, provider)
		},
		restart: func(ctx context.Context, namespace, kind, name string) error {
			_, err := runKubectl(ctx, kubeconfig, "rollout", "restart",
				fmt.Sprintf("%s/%s", kind, name), "-n", namespace)
			return err
		},
	}
}

// KubectlRunner 는 kubectl 실행을 추상화한다.
type KubectlRunner func(ctx context.Context, kubeconfig []byte, args ...string) ([]byte, error)

// RestartForProvider 는 해당 provider 의 소비자를 rolling restart 한다.
//
// 재시작이 불필요한 provider 는 조회조차 하지 않는다.
func (r *WorkloadRestarter) RestartForProvider(ctx context.Context, namespace, provider string) error {
	if r == nil || !RequiresRestart(provider) {
		return nil
	}
	if r.lookup == nil {
		return nil
	}

	workloads, err := r.lookup(ctx, namespace, provider)
	if err != nil {
		return fmt.Errorf("재시작 대상 조회 실패 (%s): %w", provider, err)
	}
	if len(workloads) == 0 {
		return nil
	}

	// 하나가 실패해도 나머지를 계속 시도한다. 일부만 옛 자격증명에 머무는
	// 상태가 전부 실패하는 것보다 진단하기 어렵다.
	var errs []error
	for _, w := range workloads {
		if r.restart == nil {
			continue
		}
		if err := r.restart(ctx, namespace, w.Kind, w.Name); err != nil {
			errs = append(errs, fmt.Errorf("%s/%s 재시작 실패: %w", w.Kind, w.Name, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// lookupWorkloads 는 provider 라벨이 붙은 Deployment/StatefulSet 을 찾는다.
func lookupWorkloads(ctx context.Context, kubeconfig []byte, runKubectl KubectlRunner, namespace, provider string) ([]WorkloadRef, error) {
	if runKubectl == nil {
		return nil, nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	selector := fmt.Sprintf("app.kubernetes.io/instance=%s", normalizeReleaseName(provider))
	out, err := runKubectl(lookupCtx, kubeconfig, "get", "deployment,statefulset",
		"-n", namespace, "-l", selector, "-o", "json")
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("워크로드 목록 파싱 실패: %w", err)
	}

	refs := make([]WorkloadRef, 0, len(list.Items))
	for _, item := range list.Items {
		kind := strings.ToLower(strings.TrimSpace(item.Kind))
		if kind == "" {
			continue
		}
		refs = append(refs, WorkloadRef{Kind: kind, Name: item.Metadata.Name})
	}
	return refs, nil
}

// normalizeReleaseName 은 provider 이름을 Helm 릴리스 이름으로 맞춘다.
func normalizeReleaseName(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "argocd":
		return "argo-cd"
	case "gitlab-ce", "gitlab-ci", "gitlab-registry":
		return "gitlab"
	case "postgresql":
		return shareddomain.PostgresReleaseName
	case "minio":
		return shareddomain.MinIOReleaseName
	default:
		return p
	}
}

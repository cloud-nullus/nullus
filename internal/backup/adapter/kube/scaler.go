// Package kube 는 정지/재개, 볼륨 아카이브, 리소스 덤프의 Kubernetes 어댑터다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §3.4 (nullus-plan#75)
package kube

import (
	"context"
	"fmt"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// WorkloadScaler 는 Deployment/StatefulSet 을 0 으로 내렸다 되돌린다.
type WorkloadScaler struct {
	client kubernetes.Interface
	// waitTimeout 은 파드 종료를 기다리는 총 시간이다. 넘으면 정지 백업을
	// 포기한다 — 파드가 남아 있는 채로 복사하면 정지 백업의 의미가 없다.
	waitTimeout time.Duration
}

func NewWorkloadScaler(client kubernetes.Interface) *WorkloadScaler {
	return &WorkloadScaler{client: client, waitTimeout: 5 * time.Minute}
}

func (s *WorkloadScaler) List(ctx context.Context, namespace string) ([]domain.Workload, error) {
	out := make([]domain.Workload, 0)

	deps, err := s.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Deployment 목록 조회(%s): %w", namespace, err)
	}
	for _, d := range deps.Items {
		var replicas int32
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		out = append(out, domain.Workload{
			Kind: "Deployment", Namespace: d.Namespace, Name: d.Name, Replicas: replicas,
		})
	}

	sts, err := s.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("StatefulSet 목록 조회(%s): %w", namespace, err)
	}
	for _, st := range sts.Items {
		var replicas int32
		if st.Spec.Replicas != nil {
			replicas = *st.Spec.Replicas
		}
		out = append(out, domain.Workload{
			Kind: "StatefulSet", Namespace: st.Namespace, Name: st.Name, Replicas: replicas,
		})
	}
	return out, nil
}

func (s *WorkloadScaler) Scale(ctx context.Context, t domain.QuiesceTarget, replicas int32) error {
	scale, err := s.getScale(ctx, t)
	if err != nil {
		return err
	}
	scale.Spec.Replicas = replicas

	switch t.Kind {
	case "Deployment":
		_, err = s.client.AppsV1().Deployments(t.Namespace).UpdateScale(ctx, t.Name, scale, metav1.UpdateOptions{})
	case "StatefulSet":
		_, err = s.client.AppsV1().StatefulSets(t.Namespace).UpdateScale(ctx, t.Name, scale, metav1.UpdateOptions{})
	default:
		return fmt.Errorf("지원하지 않는 워크로드 종류: %s", t.Kind)
	}
	if err != nil {
		return fmt.Errorf("%s/%s 를 %d 로 조정: %w", t.Kind, t.Name, replicas, err)
	}
	return nil
}

func (s *WorkloadScaler) getScale(ctx context.Context, t domain.QuiesceTarget) (*autoscalingv1.Scale, error) {
	switch t.Kind {
	case "Deployment":
		return s.client.AppsV1().Deployments(t.Namespace).GetScale(ctx, t.Name, metav1.GetOptions{})
	case "StatefulSet":
		return s.client.AppsV1().StatefulSets(t.Namespace).GetScale(ctx, t.Name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("지원하지 않는 워크로드 종류: %s", t.Kind)
	}
}

// WaitStopped 는 대상 워크로드의 파드가 모두 사라질 때까지 기다린다.
//
// replica 를 0 으로 바꾸는 것과 파드가 실제로 종료되는 것은 다르다. 종료
// 전에 복사를 시작하면 쓰기가 진행 중인 파일을 뜨게 되고, 그것은 정지
// 백업이 아니다.
func (s *WorkloadScaler) WaitStopped(ctx context.Context, namespace string, targets []domain.QuiesceTarget) error {
	if len(targets) == 0 {
		return nil
	}
	deadline := time.Now().Add(s.waitTimeout)
	for {
		remaining, err := s.runningReplicas(ctx, targets)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("파드가 %s 안에 종료되지 않았습니다 (남은 replica %d). 쓰기가 멈추지 않은 채로 복사할 수 없습니다",
				s.waitTimeout, remaining)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *WorkloadScaler) runningReplicas(ctx context.Context, targets []domain.QuiesceTarget) (int32, error) {
	var total int32
	for _, t := range targets {
		switch t.Kind {
		case "Deployment":
			d, err := s.client.AppsV1().Deployments(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
			if err != nil {
				return 0, fmt.Errorf("Deployment %s 조회: %w", t.Name, err)
			}
			total += d.Status.Replicas
		case "StatefulSet":
			st, err := s.client.AppsV1().StatefulSets(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
			if err != nil {
				return 0, fmt.Errorf("StatefulSet %s 조회: %w", t.Name, err)
			}
			total += st.Status.Replicas
		}
	}
	return total, nil
}

var _ port.WorkloadScaler = (*WorkloadScaler)(nil)

package helm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const argoCDPartOfSelector = "app.kubernetes.io/part-of=argocd"

// ArgoCDUpgradeHealthVerifier 는 Helm 이 성공이라 한 뒤 Argo CD 가 실제로 그 버전으로 살아 있는지
// 본다 — 모든 Deployment · StatefulSet 이 새 세대로 준비됐고, API 서버가 대상 이미지로 떠 있어야
// 한다. Secret 은 읽지 않는다.
type ArgoCDUpgradeHealthVerifier struct {
	newClient func(kubeconfig []byte) (kubernetes.Interface, error)
	interval  time.Duration
	timeout   time.Duration
}

func NewArgoCDUpgradeHealthVerifier() *ArgoCDUpgradeHealthVerifier {
	return &ArgoCDUpgradeHealthVerifier{
		newClient: func(kubeconfig []byte) (kubernetes.Interface, error) {
			cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("parse kubeconfig: %w", err)
			}
			return kubernetes.NewForConfig(cfg)
		},
		interval: 5 * time.Second,
		// Helm 이 이미 기다린 뒤라 오래 걸리지 않는다. 롤아웃 꼬리만 받아 준다.
		timeout: 3 * time.Minute,
	}
}

func (v *ArgoCDUpgradeHealthVerifier) Verify(ctx context.Context, kubeconfig []byte, namespace string, bundle domain.UpgradeBundle) error {
	if bundle.Tool != domain.UpgradeToolArgoCD {
		return fmt.Errorf("상태 검사를 지원하지 않는 도구다: %s", bundle.Tool)
	}
	client, err := v.newClient(kubeconfig)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(v.timeout)
	for {
		err := argoCDHealth(ctx, client, namespace, bundle.TargetAppVersion)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return errors.Join(err, ctx.Err())
		case <-time.After(v.interval):
		}
	}
}

func argoCDHealth(ctx context.Context, client kubernetes.Interface, namespace, appVersion string) error {
	opts := metav1.ListOptions{LabelSelector: argoCDPartOfSelector}
	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, opts)
	if err != nil {
		return fmt.Errorf("list Argo CD deployments: %w", err)
	}
	statefulSets, err := client.AppsV1().StatefulSets(namespace).List(ctx, opts)
	if err != nil {
		return fmt.Errorf("list Argo CD statefulsets: %w", err)
	}
	if len(deployments.Items) == 0 && len(statefulSets.Items) == 0 {
		return fmt.Errorf("Argo CD 워크로드가 %s 에 없다", namespace)
	}

	serverImage := ""
	for _, d := range deployments.Items {
		desired := replicasOrOne(d.Spec.Replicas)
		if d.Status.ObservedGeneration < d.Generation || d.Status.ReadyReplicas < desired || d.Status.UpdatedReplicas < desired {
			return fmt.Errorf("Argo CD %s 가 준비되지 않았다 (%d/%d)", d.Name, d.Status.ReadyReplicas, desired)
		}
		if d.Labels["app.kubernetes.io/component"] == "server" && len(d.Spec.Template.Spec.Containers) > 0 {
			serverImage = d.Spec.Template.Spec.Containers[0].Image
		}
	}
	for _, s := range statefulSets.Items {
		desired := replicasOrOne(s.Spec.Replicas)
		if s.Status.ObservedGeneration < s.Generation || s.Status.ReadyReplicas < desired || s.Status.UpdatedReplicas < desired {
			return fmt.Errorf("Argo CD %s 가 준비되지 않았다 (%d/%d)", s.Name, s.Status.ReadyReplicas, desired)
		}
	}

	if appVersion != "" {
		if serverImage == "" {
			return fmt.Errorf("Argo CD API 서버(component=server)를 찾지 못했다")
		}
		if !strings.HasSuffix(serverImage, ":"+appVersion) {
			return fmt.Errorf("Argo CD API 서버 이미지가 %s 다 — 기대한 버전은 %s", serverImage, appVersion)
		}
	}
	return nil
}

func replicasOrOne(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}

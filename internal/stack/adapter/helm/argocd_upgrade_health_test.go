package helm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func argoLabels(component string) map[string]string {
	return map[string]string{"app.kubernetes.io/part-of": "argocd", "app.kubernetes.io/component": component}
}

func argoDeployment(name, component, image string, ready int32) *appsv1.Deployment {
	one := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "nullus-prod", Labels: argoLabels(component), Generation: 2},
		Spec: appsv1.DeploymentSpec{Replicas: &one, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: component, Image: image}}}}},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 2, ReadyReplicas: ready, UpdatedReplicas: ready},
	}
}

func argoController(image string, ready int32) *appsv1.StatefulSet {
	one := int32(1)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "argo-cd-argocd-application-controller", Namespace: "nullus-prod",
			Labels: argoLabels("application-controller"), Generation: 1},
		Spec: appsv1.StatefulSetSpec{Replicas: &one, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "application-controller", Image: image}}}}},
		Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, ReadyReplicas: ready, UpdatedReplicas: ready},
	}
}

func verifierWith(objects ...runtime.Object) *ArgoCDUpgradeHealthVerifier {
	client := fake.NewSimpleClientset(objects...)
	v := NewArgoCDUpgradeHealthVerifier()
	v.newClient = func([]byte) (kubernetes.Interface, error) { return client, nil }
	v.interval, v.timeout = time.Millisecond, 20*time.Millisecond
	return v
}

var argoTarget = domain.UpgradeBundle{Tool: domain.UpgradeToolArgoCD, TargetChartVersion: "7.7.20", TargetAppVersion: "v2.13.5"}

func TestArgoCDUpgradeHealth_ReadyAtTargetVersion(t *testing.T) {
	v := verifierWith(
		argoDeployment("argo-cd-argocd-server", "server", "quay.io/argoproj/argocd:v2.13.5", 1),
		argoDeployment("argo-cd-argocd-repo-server", "repo-server", "quay.io/argoproj/argocd:v2.13.5", 1),
		argoController("quay.io/argoproj/argocd:v2.13.5", 1),
	)
	require.NoError(t, v.Verify(context.Background(), nil, "nullus-prod", argoTarget))
}

// Helm 이 성공이라 해도 서버가 옛 이미지로 떠 있으면 업그레이드가 아니다 — values 가 태그를
// 고정해 두었으면 그렇게 된다.
func TestArgoCDUpgradeHealth_RejectsOldServerImage(t *testing.T) {
	v := verifierWith(
		argoDeployment("argo-cd-argocd-server", "server", "quay.io/argoproj/argocd:v2.13.3", 1),
		argoController("quay.io/argoproj/argocd:v2.13.3", 1),
	)
	err := v.Verify(context.Background(), nil, "nullus-prod", argoTarget)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2.13.5")
}

func TestArgoCDUpgradeHealth_RejectsNotReadyWorkload(t *testing.T) {
	v := verifierWith(
		argoDeployment("argo-cd-argocd-server", "server", "quay.io/argoproj/argocd:v2.13.5", 1),
		argoController("quay.io/argoproj/argocd:v2.13.5", 0),
	)
	err := v.Verify(context.Background(), nil, "nullus-prod", argoTarget)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "application-controller")
}

func TestArgoCDUpgradeHealth_RejectsMissingWorkloads(t *testing.T) {
	err := verifierWith().Verify(context.Background(), nil, "nullus-prod", argoTarget)
	require.Error(t, err)
}

func TestArgoCDUpgradeHealth_RejectsOtherTools(t *testing.T) {
	err := verifierWith().Verify(context.Background(), nil, "nullus-prod", domain.UpgradeBundle{Tool: "grafana"})
	require.Error(t, err)
}

// 처음엔 준비 중이어도 시간 안에 준비되면 통과한다(Helm 대기 직후의 롤아웃 꼬리).
func TestArgoCDUpgradeHealth_WaitsUntilReady(t *testing.T) {
	ctrl := argoController("quay.io/argoproj/argocd:v2.13.5", 0)
	client := fake.NewSimpleClientset(argoDeployment("argo-cd-argocd-server", "server", "quay.io/argoproj/argocd:v2.13.5", 1), ctrl)
	v := NewArgoCDUpgradeHealthVerifier()
	v.newClient = func([]byte) (kubernetes.Interface, error) { return client, nil }
	v.interval, v.timeout = time.Millisecond, time.Second
	go func() {
		time.Sleep(10 * time.Millisecond)
		ready := ctrl.DeepCopy()
		ready.Status.ReadyReplicas, ready.Status.UpdatedReplicas = 1, 1
		_, _ = client.AppsV1().StatefulSets("nullus-prod").UpdateStatus(context.Background(), ready, metav1.UpdateOptions{})
	}()
	require.NoError(t, v.Verify(context.Background(), nil, "nullus-prod", argoTarget))
}

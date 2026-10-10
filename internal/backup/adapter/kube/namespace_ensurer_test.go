package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 새 클러스터나 지운 네임스페이스로 복구하면 네임스페이스부터 만든다. 출처를 남긴다 — 설치
// preflight 가 남은 자원을 볼 때, 복구가 놓아둔 것과 실패한 설치의 잔여를 가를 근거다.
func TestNamespaceEnsurer_CreatesMissingNamespace(t *testing.T) {
	c := fake.NewSimpleClientset()

	require.NoError(t, NewNamespaceEnsurer(c).EnsureNamespace(context.Background(), "nullus-gitlab", "bk_1"))

	ns, err := c.CoreV1().Namespaces().Get(context.Background(), "nullus-gitlab", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "bk_1", ns.Annotations[shareddomain.RestoredFromBackupAnnotation])
}

// 이미 있으면 그대로 둔다 — 같은 클러스터로 되돌리는 흔한 경우다. 라벨·주석을 덮어쓰지 않는다.
func TestNamespaceEnsurer_KeepsExistingNamespace(t *testing.T) {
	c := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "nullus-gitlab", Labels: map[string]string{"team": "a"},
	}})

	require.NoError(t, NewNamespaceEnsurer(c).EnsureNamespace(context.Background(), "nullus-gitlab", "bk_1"))

	ns, err := c.CoreV1().Namespaces().Get(context.Background(), "nullus-gitlab", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "a", ns.Labels["team"])
	assert.Empty(t, ns.Annotations[shareddomain.RestoredFromBackupAnnotation])
}

func TestNamespaceEnsurer_RejectsEmptyName(t *testing.T) {
	assert.Error(t, NewNamespaceEnsurer(fake.NewSimpleClientset()).EnsureNamespace(context.Background(), " ", "bk_1"))
}

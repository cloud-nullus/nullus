package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

func secretObject(ns, name string, labels map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("Secret")
	u.SetNamespace(ns)
	u.SetName(name)
	u.SetLabels(labels)
	return u
}

func secretDeleterWith(t *testing.T, objs ...*unstructured.Unstructured) (*ManagedSecretDeleter, dynamic.Interface) {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	for _, o := range objs {
		_, err := client.Resource(secretGVR).Namespace(o.GetNamespace()).Create(context.Background(), o, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	d := &ManagedSecretDeleter{newClient: func([]byte) (dynamic.Interface, error) { return client, nil }}
	return d, client
}

func secretExists(t *testing.T, client dynamic.Interface, ns, name string) bool {
	t.Helper()
	_, err := client.Resource(secretGVR).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

var managedByCICD = map[string]string{shareddomain.AppManagedByLabel: shareddomain.AppManagedByCICD}

func TestManagedSecretDeleter_DeletesPlatformSecret(t *testing.T) {
	d, client := secretDeleterWith(t,
		secretObject("apps", "nullus-registry-myapp", managedByCICD),
		secretObject("apps", "nullus-registry-other", managedByCICD),
	)

	require.NoError(t, d.DeleteManagedSecret(context.Background(), nil, "apps", "nullus-registry-myapp"))

	assert.False(t, secretExists(t, client, "apps", "nullus-registry-myapp"))
	assert.True(t, secretExists(t, client, "apps", "nullus-registry-other"), "다른 앱의 Secret 은 남아야 한다")
}

// 이름이 같아도 플랫폼 표시가 없으면 사용자가 만든 것일 수 있다 — 지우지 않는다.
func TestManagedSecretDeleter_LeavesUnmanagedSecret(t *testing.T) {
	d, client := secretDeleterWith(t,
		secretObject("apps", "nullus-registry-myapp", map[string]string{"owner": "customer"}),
	)

	require.NoError(t, d.DeleteManagedSecret(context.Background(), nil, "apps", "nullus-registry-myapp"))

	assert.True(t, secretExists(t, client, "apps", "nullus-registry-myapp"))
}

// 이미 없는 것은 성공이다 — 다시 시도한 삭제가 같은 자리에서 막히면 안 된다.
func TestManagedSecretDeleter_MissingSecretIsSuccess(t *testing.T) {
	d, _ := secretDeleterWith(t)

	assert.NoError(t, d.DeleteManagedSecret(context.Background(), nil, "apps", "nullus-registry-myapp"))
}

func TestManagedSecretDeleter_RequiresNamespaceAndName(t *testing.T) {
	d, _ := secretDeleterWith(t)

	assert.Error(t, d.DeleteManagedSecret(context.Background(), nil, "", "nullus-registry-myapp"))
	assert.Error(t, d.DeleteManagedSecret(context.Background(), nil, "apps", " "))
}

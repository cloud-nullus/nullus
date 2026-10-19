package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const credPrefix = "kv/nullus/dev/org-1/cicd/pipelines/api/"

func credObject() port.PipelineCredentialObject {
	return port.PipelineCredentialObject{Namespace: "stack-ns", Name: "nullus-ci-api", RemoteKeyPrefix: credPrefix}
}

func externalSecret(labels map[string]string, remoteKeys ...string) *unstructured.Unstructured {
	data := make([]any, 0, len(remoteKeys))
	for _, k := range remoteKeys {
		data = append(data, map[string]any{"secretKey": "K", "remoteRef": map[string]any{"key": k, "property": "token"}})
	}
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "external-secrets.io/v1", "kind": "ExternalSecret",
		"spec": map[string]any{"secretStoreRef": map[string]any{"name": "nullus-openbao"}, "data": data},
	}}
	u.SetNamespace("stack-ns")
	u.SetName("nullus-ci-api")
	u.SetLabels(labels)
	return u
}

func externalSecretDeleterWith(t *testing.T, objs ...*unstructured.Unstructured) (*ExternalSecretDeleter, dynamic.Interface) {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{externalSecretGVR: "ExternalSecretList"})
	for _, o := range objs {
		_, err := client.Resource(externalSecretGVR).Namespace(o.GetNamespace()).Create(context.Background(), o, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	return &ExternalSecretDeleter{newClient: func([]byte) (dynamic.Interface, error) { return client, nil }}, client
}

func externalSecretExists(t *testing.T, client dynamic.Interface) bool {
	t.Helper()
	_, err := client.Resource(externalSecretGVR).Namespace("stack-ns").Get(context.Background(), "nullus-ci-api", metav1.GetOptions{})
	return err == nil
}

func TestExternalSecretDeleter_DeletesLabeledObject(t *testing.T) {
	d, client := externalSecretDeleterWith(t, externalSecret(managedByCICD, credPrefix+"k"))

	require.NoError(t, d.DeleteCredentialObject(context.Background(), nil, credObject()))
	assert.False(t, externalSecretExists(t, client))
}

// 라벨이 생기기 전에 만든 것도 지운다 — 원격 키가 모두 이 파이프라인의 금고 경로 아래면
// 이 파이프라인이 만든 것이다.
func TestExternalSecretDeleter_DeletesLegacyObjectPointingOnlyAtPipelinePath(t *testing.T) {
	d, client := externalSecretDeleterWith(t, externalSecret(nil, credPrefix+"harbor_password", credPrefix+"harbor_username"))

	require.NoError(t, d.DeleteCredentialObject(context.Background(), nil, credObject()))
	assert.False(t, externalSecretExists(t, client))
}

// 이름이 같아도 다른 경로를 가리키는 표시 없는 객체는 남의 것일 수 있다.
func TestExternalSecretDeleter_LeavesForeignObject(t *testing.T) {
	d, client := externalSecretDeleterWith(t,
		externalSecret(nil, credPrefix+"harbor_password", "kv/nullus/dev/org-1/stack/gitea/admin"))

	require.NoError(t, d.DeleteCredentialObject(context.Background(), nil, credObject()))
	assert.True(t, externalSecretExists(t, client))
}

func TestExternalSecretDeleter_LeavesUnlabeledObjectWithoutRemoteKeys(t *testing.T) {
	d, client := externalSecretDeleterWith(t, externalSecret(nil))

	require.NoError(t, d.DeleteCredentialObject(context.Background(), nil, credObject()))
	assert.True(t, externalSecretExists(t, client))
}

func TestExternalSecretDeleter_MissingIsSuccess(t *testing.T) {
	d, _ := externalSecretDeleterWith(t)
	assert.NoError(t, d.DeleteCredentialObject(context.Background(), nil, credObject()))
}

func TestExternalSecretDeleter_RequiresNamespaceAndName(t *testing.T) {
	d, _ := externalSecretDeleterWith(t)
	assert.Error(t, d.DeleteCredentialObject(context.Background(), nil, port.PipelineCredentialObject{Name: "x"}))
	assert.Error(t, d.DeleteCredentialObject(context.Background(), nil, port.PipelineCredentialObject{Namespace: "x"}))
}

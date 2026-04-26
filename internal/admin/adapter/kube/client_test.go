package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const fakeKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://kube.test:6443
    insecure-skip-tls-verify: true
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    token: test-token
`

func newNode(name, arch string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{Architecture: arch},
		},
	}
}

// withFakeClient swaps the package-level clientset builder for the duration
// of the test so DiscoverCluster hits an in-memory fixture instead of a real
// API server. Returns nothing; the t.Cleanup handler restores the default.
func withFakeClient(t *testing.T, nodes []corev1.Node, gitVersion string) {
	t.Helper()
	fakeCS := fake.NewClientset()
	for i := range nodes {
		_, err := fakeCS.CoreV1().Nodes().Create(context.Background(), &nodes[i], metav1.CreateOptions{})
		require.NoError(t, err)
	}
	if fd, ok := fakeCS.Discovery().(*fakediscovery.FakeDiscovery); ok {
		fd.FakedServerVersion = &version.Info{GitVersion: gitVersion}
	}
	previous := clientsetBuilder
	clientsetBuilder = func(_ *rest.Config) (kubernetes.Interface, error) {
		return fakeCS, nil
	}
	t.Cleanup(func() { clientsetBuilder = previous })
}

func TestDiscoverCluster_SingleArchCluster(t *testing.T) {
	withFakeClient(t, []corev1.Node{
		newNode("node-1", "amd64"),
	}, "v1.30.1")

	info, err := DiscoverCluster(context.Background(), []byte(fakeKubeconfig))
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "v1.30.1", info.ServerVersion)
	assert.Equal(t, []string{"amd64"}, info.NodeArchitectures)
	assert.Equal(t, 1, info.NodeCount)
	assert.False(t, info.DiscoveredAt.IsZero())
}

func TestDiscoverCluster_MixedArchCluster(t *testing.T) {
	withFakeClient(t, []corev1.Node{
		newNode("node-a", "arm64"),
		newNode("node-b", "amd64"),
		newNode("node-c", "amd64"),
	}, "v1.31.0")

	info, err := DiscoverCluster(context.Background(), []byte(fakeKubeconfig))
	require.NoError(t, err)
	require.NotNil(t, info)
	// Sorted + deduped: amd64 < arm64, duplicate amd64 collapsed.
	assert.Equal(t, []string{"amd64", "arm64"}, info.NodeArchitectures)
	assert.Equal(t, 3, info.NodeCount)
}

func TestDiscoverCluster_NoNodes(t *testing.T) {
	withFakeClient(t, nil, "v1.29.4")

	info, err := DiscoverCluster(context.Background(), []byte(fakeKubeconfig))
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Nil(t, info.NodeArchitectures)
	assert.Equal(t, 0, info.NodeCount)
	assert.Equal(t, "v1.29.4", info.ServerVersion)
}

func TestVerifyCluster_DelegatesToDiscover(t *testing.T) {
	withFakeClient(t, []corev1.Node{
		newNode("node-1", "amd64"),
	}, "v1.30.1")

	res, err := VerifyCluster([]byte(fakeKubeconfig))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "connected", res.Status)
	assert.Equal(t, "v1.30.1", res.Version)
}

package kube

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helmReleaseSecret 은 Helm 이 실제로 저장하는 모양을 흉내낸다:
// JSON → gzip → base64 가 Secret 의 release 값이다.
func helmReleaseSecret(name string, revision int, payload string) *corev1.Secret {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(payload))
	_ = w.Close()
	encoded := base64.StdEncoding.EncodeToString(gz.Bytes())
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1." + name + ".v" + string(rune('0'+revision)),
			Namespace: "nullus-app",
			Labels:    map[string]string{"owner": "helm", "name": name},
		},
		Type: "helm.sh/release.v1",
		Data: map[string][]byte{"release": []byte(encoded)},
	}
}

func TestDecodeHelmRelease_차트_이름과_버전을_읽는다(t *testing.T) {
	got, err := decodeHelmRelease([]byte(func() string {
		var gz bytes.Buffer
		w := gzip.NewWriter(&gz)
		_, _ = w.Write([]byte(`{"name":"gitea","version":2,
			"chart":{"metadata":{"name":"gitea","version":"10.4.1","appVersion":"1.22.3"}}}`))
		_ = w.Close()
		return base64.StdEncoding.EncodeToString(gz.Bytes())
	}()))
	require.NoError(t, err)
	assert.Equal(t, "gitea", got.Name)
	assert.Equal(t, "gitea", got.Chart)
	assert.Equal(t, "10.4.1", got.Version)
	assert.Equal(t, "1.22.3", got.AppVersion)
	assert.Equal(t, 2, got.Revision)
}

func TestDecodeHelmRelease_읽을_수_없으면_오류(t *testing.T) {
	// 조용히 빈 값을 돌려주면 매니페스트에 "차트를 모른다" 가 "차트가 없다" 로
	// 기록된다 — 복구가 임의 버전을 골라도 아무도 못 막는다.
	_, err := decodeHelmRelease([]byte("not-base64-gzip"))
	require.Error(t, err)
}

func TestListHelmReleases_최신_리비전만_남긴다(t *testing.T) {
	// Helm 은 리비전마다 Secret 을 남긴다. 복구가 재현해야 하는 것은 백업
	// 시점에 **실제로 돌고 있던** 버전이므로 가장 높은 리비전만 쓴다.
	c := fake.NewSimpleClientset(
		helmReleaseSecret("gitea", 1, `{"name":"gitea","version":1,"chart":{"metadata":{"name":"gitea","version":"10.3.0"}}}`),
		helmReleaseSecret("gitea", 3, `{"name":"gitea","version":3,"chart":{"metadata":{"name":"gitea","version":"10.4.1"}}}`),
		helmReleaseSecret("harbor", 1, `{"name":"harbor","version":1,"chart":{"metadata":{"name":"harbor","version":"1.15.1"}}}`),
	)
	got, err := NewHelmReleaseReader(c).ListHelmReleases(context.Background(), "nullus-app")
	require.NoError(t, err)
	require.Len(t, got, 2)

	byName := map[string]string{}
	for _, r := range got {
		byName[r.Name] = r.Version
	}
	assert.Equal(t, "10.4.1", byName["gitea"], "낡은 리비전을 고르면 복구가 다른 버전으로 재설치된다")
	assert.Equal(t, "1.15.1", byName["harbor"])
}

func TestListHelmReleases_릴리스가_없으면_빈_결과(t *testing.T) {
	got, err := NewHelmReleaseReader(fake.NewSimpleClientset()).
		ListHelmReleases(context.Background(), "nullus-app")
	require.NoError(t, err)
	assert.Empty(t, got)
}

package externalsecret

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestRender_KeepEntriesMergesTemplateWithEntries(t *testing.T) {
	spec := Spec{
		Name:            "creds",
		Namespace:       "nullus",
		SecretStoreName: "store",
		Entries:         []Entry{{SecretKey: "secretKey", RemotePath: "kv/x"}},
		TemplateData:    map[string]string{"config": `{"key":"{{ .secretKey }}"}`},
		KeepEntries:     true,
	}
	manifest, err := Render(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(manifest, "      engineVersion: v2\n      mergePolicy: Merge\n      data:\n") {
		t.Fatalf("mergePolicy: Merge 가 template 아래에 없다:\n%s", manifest)
	}
}

// 기본은 ESO 와 같다(Replace) — 이미 설치된 Secret 의 모양이 바뀌면 안 된다.
func TestRender_TemplateWithoutKeepEntriesHasNoMergePolicy(t *testing.T) {
	manifest, err := Render(Spec{
		Name:            "creds",
		Namespace:       "nullus",
		SecretStoreName: "store",
		Entries:         []Entry{{SecretKey: "secretKey", RemotePath: "kv/x"}},
		TemplateData:    map[string]string{"config": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(manifest, "mergePolicy") {
		t.Fatalf("KeepEntries 없이 mergePolicy 가 붙었다:\n%s", manifest)
	}
}

func TestRender_AddsLabelsWhenGiven(t *testing.T) {
	out, err := Render(Spec{
		Name: "nullus-ci-api", Namespace: "ns", SecretStoreName: "store",
		Labels:  map[string]string{"app.kubernetes.io/name": "api", "app.kubernetes.io/managed-by": "nullus-cicd"},
		Entries: []Entry{{SecretKey: "K", RemotePath: "kv/x/k"}},
	})
	require.NoError(t, err)

	var doc struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(out), &doc))
	assert.Equal(t, map[string]string{"app.kubernetes.io/name": "api", "app.kubernetes.io/managed-by": "nullus-cicd"}, doc.Metadata.Labels)
}

// 라벨을 주지 않는 호출자(스택 모듈)의 출력은 그대로다.
func TestRender_WithoutLabelsIsUnchanged(t *testing.T) {
	out, err := Render(Spec{
		Name: "argocd-secret", Namespace: "ns", SecretStoreName: "store",
		Entries: []Entry{{SecretKey: "K", RemotePath: "kv/x/k"}},
	})
	require.NoError(t, err)
	assert.NotContains(t, out, "labels:")
	assert.Contains(t, out, "metadata:\n  name: argocd-secret\n  namespace: ns\nspec:")
}

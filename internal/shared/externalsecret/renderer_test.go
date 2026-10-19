package externalsecret

import (
	"strings"
	"testing"
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

package secrets

import (
	"testing"
)

func TestSplitKVPath(t *testing.T) {
	cases := []struct {
		path      string
		wantMount string
		wantSub   string
		wantErr   bool
	}{
		{"kv/nullus/dev/org1/pipeline/argocd/token", "kv", "nullus/dev/org1/pipeline/argocd/token", false},
		{"kv/nullus/prod/org2/artifacts/gitlab/token", "kv", "nullus/prod/org2/artifacts/gitlab/token", false},
		{"secret/foo/bar", "secret", "foo/bar", false},
		{"/kv/a/b/", "kv", "a/b", false},
		{"single", "", "", true},
	}
	for _, c := range cases {
		mount, sub, err := splitKVPath(c.path)
		if c.wantErr {
			if err == nil {
				t.Errorf("splitKVPath(%q): expected error, got nil", c.path)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitKVPath(%q): unexpected error: %v", c.path, err)
			continue
		}
		if mount != c.wantMount {
			t.Errorf("splitKVPath(%q) mount = %q, want %q", c.path, mount, c.wantMount)
		}
		if sub != c.wantSub {
			t.Errorf("splitKVPath(%q) sub = %q, want %q", c.path, sub, c.wantSub)
		}
	}
}

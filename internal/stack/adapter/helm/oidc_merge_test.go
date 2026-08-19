package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OIDC values 는 기존 values 에 "더하는" 성격이다. 그런데 mergeMaps 는 맵만 깊게
// 합치고 슬라이스는 통째로 바꾼다. 그래서 Jenkins 의 additionalExistingSecrets 에
// OIDC 시크릿을 넣자 기존 Gitea 자격 항목 두 개가 사라졌고, JCasC 가
// ${nullus-gitea-credentials-username} 을 풀지 못해 Jenkins 가 기동에 실패했다:
//
//	SEVERE hudson.util.BootFailure: Failed to initialize Jenkins
//
// 실측한 적용값에는 OIDC 항목 하나만 남아 있었다.
func TestMergeOIDCValues_AppendsListsInsteadOfReplacing(t *testing.T) {
	base := map[string]any{
		"controller": map[string]any{
			"additionalExistingSecrets": []any{
				map[string]any{"name": "nullus-gitea-credentials", "keyName": "username"},
				map[string]any{"name": "nullus-gitea-credentials", "keyName": "password"},
			},
			"installPlugins": []any{"kubernetes", "workflow-aggregator"},
		},
	}
	oidc := map[string]any{
		"controller": map[string]any{
			"additionalExistingSecrets": []any{
				map[string]any{"name": "stack-jenkins-oidc", "keyName": "client-secret"},
			},
			"additionalPlugins": []any{"oic-auth"},
		},
	}

	merged := mergeOIDCValues(base, oidc)
	controller := merged["controller"].(map[string]any)

	secrets := controller["additionalExistingSecrets"].([]any)
	require.Len(t, secrets, 3, "기존 항목을 밀어내면 JCasC 가 자격을 풀지 못한다: %v", secrets)

	plugins := controller["installPlugins"].([]any)
	assert.Len(t, plugins, 2, "손대지 않은 목록은 그대로여야 한다")
	assert.Equal(t, []any{"oic-auth"}, controller["additionalPlugins"])
}

// 맵은 기존대로 깊게 합쳐야 한다 — JCasC 조각이 서로를 덮으면 안 된다.
func TestMergeOIDCValues_KeepsDeepMapMerge(t *testing.T) {
	base := map[string]any{
		"controller": map[string]any{
			"JCasC": map[string]any{"configScripts": map[string]any{"gitea": "a"}},
		},
	}
	oidc := map[string]any{
		"controller": map[string]any{
			"JCasC": map[string]any{"configScripts": map[string]any{"oidc": "b"}},
		},
	}

	scripts := mergeOIDCValues(base, oidc)["controller"].(map[string]any)["JCasC"].(map[string]any)["configScripts"].(map[string]any)
	assert.Equal(t, "a", scripts["gitea"], "기존 JCasC 조각이 사라지면 multibranch job 이 브랜치를 못 찾는다")
	assert.Equal(t, "b", scripts["oidc"])
}

// 스칼라는 덮어쓴다 — OIDC 가 정하는 값(예: url)이 있어야 한다.
func TestMergeOIDCValues_ScalarsStillOverride(t *testing.T) {
	merged := mergeOIDCValues(
		map[string]any{"url": "http://old"},
		map[string]any{"url": "https://new"},
	)
	assert.Equal(t, "https://new", merged["url"])
}

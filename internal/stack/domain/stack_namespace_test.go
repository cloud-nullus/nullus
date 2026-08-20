package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 스택 기본 네임스페이스는 플랫폼이 사는 곳과 달라야 한다.
//
// 예전 기본값은 "nullus" 로 플랫폼 네임스페이스와 같았다. 그래서 스택 설치는
// 플랫폼의 nullus-postgresql 과 이름이 충돌해 실패했고, 스택 삭제는 플랫폼
// 리소스를 지웠다(2026-08-20 운영 사고).
func TestDefaultStackNamespaceFor_NeverReturnsPlatformNamespace(t *testing.T) {
	for _, name := range []string{"nullus", "NULLUS", "  nullus  ", ""} {
		assert.NotEqual(t, DefaultStackNamespace, DefaultStackNamespaceFor(name), "stack name %q", name)
	}
}

func TestDefaultStackNamespaceFor_DerivesFromStackName(t *testing.T) {
	assert.Equal(t, "nullus-gitea-jenkins-v1", DefaultStackNamespaceFor("gitea-jenkins-v1"))
	assert.Equal(t, "nullus-my-stack", DefaultStackNamespaceFor("My Stack"))
	assert.Equal(t, "nullus-team-a", DefaultStackNamespaceFor("team_a"))
}

func TestDefaultStackNamespaceFor_FallsBackWhenNameIsUnusable(t *testing.T) {
	assert.Equal(t, "nullus-stack", DefaultStackNamespaceFor(""))
	assert.Equal(t, "nullus-stack", DefaultStackNamespaceFor("!!!"))
}

// 네임스페이스는 RFC1123 라벨이라 63자를 넘을 수 없다.
func TestDefaultStackNamespaceFor_FitsKubernetesLabelLimit(t *testing.T) {
	long := "this-is-a-very-long-stack-name-that-goes-well-past-the-kubernetes-limit-for-labels"
	ns := DefaultStackNamespaceFor(long)

	assert.LessOrEqual(t, len(ns), 63)
	assert.NotEqual(t, "-", string(ns[len(ns)-1]), "끝이 하이픈이면 유효한 이름이 아니다")
}

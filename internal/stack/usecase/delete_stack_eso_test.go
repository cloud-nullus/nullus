package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ESO CRD·검증 웹훅은 클러스터에 하나뿐이고 모든 네임스페이스의 ESO 가 함께 쓴다. 스택 하나를
// 지우며 그것을 걷어내면 다른 네임스페이스의 ExternalSecret 이 CRD 와 함께 사라지고, 그 Secret
// 은 가비지 컬렉터가 거둔다 — 그쪽 스택은 다음 파드 재시작에서 "secret not found" 로 죽는다.
// 2026-10-12 리허설에서 DB 에만 남은 유령 스택을 지우자 살아 있던 스택의 자격증명 Secret 이
// 전부 사라졐고, 그 뒤에 뜬 백업에는 그것들이 없었다.
func newESODeleteTest(t *testing.T, usageOut string) (*DeleteStack, func() []string) {
	t.Helper()
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-eso",
		Name:      "eso stack",
		ClusterID: "cluster-eso",
		Namespace: "nullus-eso",
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller { return &fakeHelmInstaller{} })

	var mu sync.Mutex
	var commands []string
	uc.runKubectlFunc = func(_ context.Context, _ []byte, args ...string) (string, error) {
		cmd := strings.Join(args, " ")
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
		switch {
		case strings.HasPrefix(cmd, "get externalsecrets.external-secrets.io -A"),
			strings.HasPrefix(cmd, "get secretstores.external-secrets.io -A"),
			strings.HasPrefix(cmd, "get clustersecretstores.external-secrets.io -A"):
			return usageOut, nil
		case cmd == "get crd -o name":
			return "customresourcedefinition.apiextensions.k8s.io/externalsecrets.external-secrets.io\n" +
				"customresourcedefinition.apiextensions.k8s.io/secretstores.external-secrets.io\n", nil
		}
		return "", nil
	}
	return uc, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), commands...)
	}
}

func TestDeleteStack_다른_네임스페이스가_ESO_를_쓰면_CRD_와_웹훅을_남긴다(t *testing.T) {
	uc, commands := newESODeleteTest(t, "nullus-eso/mine\nnullus-live/db-credentials\n")
	require.NoError(t, uc.Execute(context.Background(), "stk-eso"))

	var deletedCRDs, deletedWebhooks, deletedOwnCRs []string
	for _, cmd := range commands() {
		switch {
		case strings.HasPrefix(cmd, "delete crd ") && strings.Contains(cmd, ".external-secrets.io"):
			deletedCRDs = append(deletedCRDs, cmd)
		case strings.HasPrefix(cmd, "delete validatingwebhookconfiguration "):
			deletedWebhooks = append(deletedWebhooks, cmd)
		case strings.HasPrefix(cmd, "delete externalsecrets") && strings.Contains(cmd, "-n nullus-eso"):
			deletedOwnCRs = append(deletedOwnCRs, cmd)
		}
	}
	assert.Empty(t, deletedCRDs, "다른 네임스페이스의 ExternalSecret 이 CRD 와 함께 사라진다")
	assert.Empty(t, deletedWebhooks, "웹훅은 클러스터에 하나뿐이다 — 남의 ESO 검증이 깨진다")
	assert.NotEmpty(t, deletedOwnCRs, "자기 네임스페이스의 CR 은 지운다")
}

func TestDeleteStack_ESO_를_쓰는_곳이_자기_네임스페이스뿐이면_CRD_와_웹훅을_지운다(t *testing.T) {
	uc, commands := newESODeleteTest(t, "nullus-eso/mine\n")
	require.NoError(t, uc.Execute(context.Background(), "stk-eso"))

	joined := strings.Join(commands(), "\n")
	assert.Contains(t, joined, "delete crd externalsecrets.external-secrets.io")
	assert.Contains(t, joined, "delete validatingwebhookconfiguration externalsecret-validate")
}

func TestDeleteStack_ClusterSecretStore_는_네임스페이스가_없어도_쓰는_곳이다(t *testing.T) {
	uc, commands := newESODeleteTest(t, "/shared-vault\n")
	require.NoError(t, uc.Execute(context.Background(), "stk-eso"))

	joined := strings.Join(commands(), "\n")
	assert.NotContains(t, joined, "delete crd externalsecrets.external-secrets.io",
		"ClusterSecretStore 는 Nullus 가 만들지 않는다 — 사용자 것이다")
}

func TestESOResourcesOutside(t *testing.T) {
	assert.False(t, esoResourcesOutside("", "nullus-eso"))
	assert.False(t, esoResourcesOutside("nullus-eso/a\nnullus-eso/b\n", "nullus-eso"))
	assert.True(t, esoResourcesOutside("nullus-eso/a\nother/b\n", "nullus-eso"))
	assert.True(t, esoResourcesOutside("/cluster-scoped\n", "nullus-eso"), "클러스터 범위(네임스페이스 없음)도 남의 것이다")
}

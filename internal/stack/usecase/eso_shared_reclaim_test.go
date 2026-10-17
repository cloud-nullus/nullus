package usecase

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ESO 는 이제 클러스터 공용 네임스페이스에 하나만 깔린다 — 마지막 스택을 지울 때
// 함께 회수해야 한다.
//
// 예전에는 스택 네임스페이스에 깔려 스택을 지우면 같이 사라졌다. 공용으로 옮기면
// 아무도 지우지 않아 클러스터에 남는다. 그러면 나중에 사용자가 같은 것을 설치할 때
// Helm ownership 충돌로 막힌다 — cert-manager·metrics-server 가 #284 에서 같은
// 이유로 회수 대상이 됐다.
func TestSharedReclaimIncludesExternalSecrets(t *testing.T) {
	src := readSourceFile(t, "delete_shared_resources.go")

	// reclaimSharedClusterResources 가 돌리는 단계 목록에 들어 있어야 한다.
	steps := regexp.MustCompile(`(?s)for _, step := range \[\]func\(context\.Context\)\{(.*?)\}`).
		FindStringSubmatch(src)
	require.NotNil(t, steps, "공용 회수 단계 목록을 찾지 못했다")

	assert.Contains(t, steps[1], "r.reclaimExternalSecrets",
		"ESO 회수 단계가 목록에 없다 — 공용 ESO 가 클러스터에 남는다")

	// 구현도 있어야 한다(목록에만 적고 함수가 없으면 컴파일이 깨지지만, 의도를 남긴다).
	assert.Contains(t, src, "func (r *sharedReclaim) reclaimExternalSecrets(")
}

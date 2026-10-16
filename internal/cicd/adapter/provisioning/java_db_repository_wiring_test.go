package provisioning

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 에어갭 Java DB 미러는 팩토리 옵션에서 번들로 간다. 클러스터 안 CI(GitLab 러너,
// Jenkins 에이전트)를 쓰는 번들에만 싣는다 — GitHub 호스티드 러너는 내부 레지스트리에
// 닿지 않는다. 번들 조립은 SCM·레지스트리를 모두 세워야 돌아 배선 자체를 고정한다.
func TestJavaDBRepository_IsWiredIntoInClusterCIBundles(t *testing.T) {
	raw, err := os.ReadFile("bundle_factory.go")
	require.NoError(t, err)
	src := strings.Join(strings.Fields(string(raw)), " ")
	assert.Equal(t, 2, strings.Count(src, "ImageScannerJavaDBRepository: f.opts.TrivyJavaDBRepository"),
		"GitLab 번들과 Gitea+Jenkins 번들에 한 번씩")
}

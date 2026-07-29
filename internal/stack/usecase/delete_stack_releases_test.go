package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치되는 모든 릴리스는 삭제 대상 목록에 있어야 한다.
//
// 목록에서 빠진 릴리스는 helm uninstall 이 아예 호출되지 않아 Deployment 는
// 물론 ClusterRole/ClusterRoleBinding/webhook/CRD 같은 cluster-scoped 리소스가
// 통째로 남는다. 그 잔재는 다음 설치에서 Helm ownership 충돌
// ("exists and cannot be imported into the current release")로 되돌아온다.
//
// external-secrets 가 실제로 이 상태였다 — 시크릿 평면이 상시 설치로 바뀌면서
// 스택을 지울 때마다 CRD 24개와 ClusterRole 5개가 고아로 남았다.
func TestStackHelmReleaseNames_CoversEveryInstalledRelease(t *testing.T) {
	for _, name := range domain.InstalledHelmReleaseNames {
		assert.Containsf(t, stackHelmReleaseNames, name,
			"릴리스 %q 가 삭제 대상에서 빠졌다 — cluster-scoped 리소스가 고아로 남는다", name)
	}
}

// 예전 이름으로 설치된 스택도 지울 수 있어야 한다.
func TestStackHelmReleaseNames_KeepsLegacyAliases(t *testing.T) {
	for _, name := range domain.LegacyHelmReleaseNames {
		assert.Containsf(t, stackHelmReleaseNames, name, "legacy 별칭 %q 가 빠졌다", name)
	}
}

func TestStackHelmReleaseNames_HasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range stackHelmReleaseNames {
		assert.Falsef(t, seen[name], "중복된 릴리스 이름: %s", name)
		seen[name] = true
	}
}

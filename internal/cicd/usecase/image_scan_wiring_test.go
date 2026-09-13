package usecase

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 스캐너 주소는 스택 → StackSummary → 번들 → 스캐폴딩 입력까지 이어져야 한다.
//
// 한 칸이라도 끊기면 스캐너를 고른 스택에서도 스캔 단계가 만들어지지 않는다.
// 설치는 정상이고 파이프라인만 조용히 비어 있어, 아무도 그 사실을 모른다.
//
// 실행 없이 배선 자체를 고정한다 — 이 경로는 SCM·레지스트리·CI 를 모두
// 세워야 돌아서 단위 테스트로 끝까지 태울 수 없다.
func TestImageScannerEndpoint_IsWiredToScaffold(t *testing.T) {
	hops := []struct {
		file string
		want string
	}{
		{"provision_pipeline_repository.go", "ImageScannerEndpoint: bundle.ImageScannerEndpoint"},
		{"provision_app_project.go", "ImageScannerEndpoint: input.ImageScannerEndpoint"},
	}
	for _, h := range hops {
		raw, err := os.ReadFile(h.file)
		require.NoError(t, err)
		assert.Containsf(t, strings.Join(strings.Fields(string(raw)), " "),
			strings.Join(strings.Fields(h.want), " "),
			"%s 에서 스캐너 주소가 끊긴다", h.file)
	}
}

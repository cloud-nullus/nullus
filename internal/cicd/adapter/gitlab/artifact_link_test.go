package gitlab

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 리포트 링크는 브라우저가 연다. API 클라이언트의 클러스터 내부 주소가 아니라
// 스택의 접속 도메인 주소여야 한다.
func TestBuildReader_ArtifactWebURL(t *testing.T) {
	ref := port.CIArtifactRef{JobName: "app", Stage: port.CIStage{ID: "11"}, Path: "trivy-report.json"}

	r := NewBuildReader(nil, "nullus").WithWebBaseURL("https://gitlab.nullus.local/")
	assert.Equal(t, "https://gitlab.nullus.local/nullus/app/-/jobs/11/artifacts/file/trivy-report.json", r.ArtifactWebURL(ref))

	// 외부 주소를 모르면 링크를 만들지 않는다 — 죽은 링크보다 없는 편이 낫다.
	assert.Empty(t, NewBuildReader(nil, "nullus").ArtifactWebURL(ref))
	// 잡 id 가 없으면 어느 잡의 산출물인지 알 수 없다.
	assert.Empty(t, r.ArtifactWebURL(port.CIArtifactRef{JobName: "app", Path: "trivy-report.json"}))
}

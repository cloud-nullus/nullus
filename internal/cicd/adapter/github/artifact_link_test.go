package github

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// GitHub 은 산출물 파일 주소가 없다. 실행 페이지가 산출물 목록을 보여 준다.
func TestBuildReader_ArtifactWebURL(t *testing.T) {
	ref := port.CIArtifactRef{JobName: "app", Build: port.CIBuild{ID: "123"}, Name: "trivy-report"}

	assert.Equal(t, "https://github.com/acme/app/actions/runs/123",
		NewBuildReader(NewClient("", "t"), "acme").ArtifactWebURL(ref))
	// GitHub Enterprise Server 는 API 주소에서 /api/v3 을 떼면 웹 주소다.
	assert.Equal(t, "https://ghe.example.com/acme/app/actions/runs/123",
		NewBuildReader(NewClient("https://ghe.example.com/api/v3", "t"), "acme").ArtifactWebURL(ref))
	assert.Empty(t, NewBuildReader(NewClient("", "t"), "acme").ArtifactWebURL(port.CIArtifactRef{JobName: "app"}))
}

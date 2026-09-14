package jenkins

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestClient_ArtifactWebURL(t *testing.T) {
	ref := port.CIArtifactRef{JobName: "app", Branch: "main", Build: port.CIBuild{Number: 7}, Path: "trivy-report.json"}

	c := NewClient("http://jenkins.devsecops.svc:8080", "u", "p").WithWebBaseURL("https://jenkins.nullus.local")
	assert.Equal(t, "https://jenkins.nullus.local/job/app/job/main/7/artifact/trivy-report.json", c.ArtifactWebURL(ref))
	// 클러스터 내부 주소로 링크를 만들지 않는다.
	assert.Empty(t, NewClient("http://jenkins.devsecops.svc:8080", "u", "p").ArtifactWebURL(ref))
}

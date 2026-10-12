package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 파이프라인 정의 파일만 다시 쓰는 경로(기존 파이프라인의 CI 정의 갱신)가 이 판별에
// 기댄다. 앱 코드·Dockerfile·배포 매니페스트는 사용자 것이라 건드리면 안 된다.
func TestIsPipelineDefinitionPath(t *testing.T) {
	for _, p := range []string{GitLabPipelinePath, GitHubWorkflowPath, JenkinsfilePath} {
		assert.True(t, IsPipelineDefinitionPath(p), p)
	}
	for _, p := range []string{"Dockerfile", "README.md", "deploy/deployment.yaml", "deploy/service.yaml", "deploy/httproute.yaml", "src/index.ts"} {
		assert.False(t, IsPipelineDefinitionPath(p), p)
	}
}

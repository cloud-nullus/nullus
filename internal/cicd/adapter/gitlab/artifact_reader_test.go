package gitlab

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestBuildReader_ImplementsCIArtifactReader(t *testing.T) {
	var _ port.CIArtifactReader = (*BuildReader)(nil)
}

// .gitlab-ci.yml 의 image-scan 잡은 artifacts.paths 로 리포트를 남긴다.
// GitLab 은 산출물을 잡 단위로 주므로 스캔 단계의 잡 id 로 읽는다.
func TestBuildReader_ReadArtifact_ReadsJobArtifactFile(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.URL.EscapedPath() == "/api/v4/projects/nullus%2Fshop/jobs/5002/artifacts/trivy-report.json" {
			_, _ = w.Write([]byte(`{"SchemaVersion":2}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	data, found, err := NewBuildReader(NewClient(srv.URL, "tok"), "nullus").ReadArtifact(context.Background(),
		port.CIArtifactRef{
			JobName: "shop", Branch: "main",
			Build: port.CIBuild{Number: 12, ID: "901"},
			Stage: port.CIStage{ID: "5002", Name: "image-scan"},
			Name:  port.ImageScanReportArtifact,
			Path:  port.ImageScanReportFile,
		})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{"SchemaVersion":2}`, string(data))
	require.Len(t, *recorded, 1)
}

// 잡 id 를 모르면 물을 곳이 없다. 추측해서 다른 잡의 산출물을 읽지 않는다.
func TestBuildReader_ReadArtifact_WithoutJobIDIsNotFound(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, found, err := NewBuildReader(NewClient(srv.URL, "tok"), "nullus").ReadArtifact(context.Background(),
		port.CIArtifactRef{JobName: "shop", Stage: port.CIStage{Name: "image-scan"}, Path: port.ImageScanReportFile})
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, *recorded)
}

func TestBuildReader_ReadArtifact_MissingIsNotFound(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, found, err := NewBuildReader(NewClient(srv.URL, "tok"), "nullus").ReadArtifact(context.Background(),
		port.CIArtifactRef{JobName: "shop", Stage: port.CIStage{ID: "1"}, Path: port.ImageScanReportFile})
	require.NoError(t, err)
	assert.False(t, found)
}

package jenkins

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestClient_ImplementsCIArtifactReader(t *testing.T) {
	var _ port.CIArtifactReader = (*Client)(nil)
}

func scanReportRef() port.CIArtifactRef {
	return port.CIArtifactRef{
		JobName: "shop", Branch: "main",
		Build: port.CIBuild{Number: 12},
		Stage: port.CIStage{Name: "ImageScan"},
		Name:  port.ImageScanReportArtifact,
		Path:  port.ImageScanReportFile,
	}
}

// Jenkinsfile 의 ImageScan 단계는 archiveArtifacts 로 리포트를 남긴다.
func TestClient_ReadArtifact_ReadsArchivedFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/job/shop/job/main/12/artifact/trivy-report.json" {
			_, _ = w.Write([]byte(`{"SchemaVersion":2}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	data, found, err := NewClient(srv.URL, "admin", "pw").ReadArtifact(context.Background(), scanReportRef())
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{"SchemaVersion":2}`, string(data))
}

// 리포트가 없는 실행(스캐너에 닿지 못한 경우, 스캔 단계 이전 실행)은 정상 경로다.
func TestClient_ReadArtifact_MissingIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	data, found, err := NewClient(srv.URL, "admin", "pw").ReadArtifact(context.Background(), scanReportRef())
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, data)
}

// 산출물은 사용자 파이프라인이 만든다. 크기를 믿고 통째로 메모리에 올리지 않는다.
func TestClient_ReadArtifact_RejectsOversizedFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), int(port.MaxCIArtifactBytes)+1))
	}))
	defer srv.Close()

	_, _, err := NewClient(srv.URL, "admin", "pw").ReadArtifact(context.Background(), scanReportRef())
	assert.Error(t, err)
}

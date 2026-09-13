package github

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestBuildReader_ImplementsCIArtifactReader(t *testing.T) {
	var _ port.CIArtifactReader = (*BuildReader)(nil)
}

func zipWith(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func reportRef() port.CIArtifactRef {
	return port.CIArtifactRef{
		JobName: "shop", Branch: "main",
		Build: port.CIBuild{Number: 4, ID: "7001"},
		Stage: port.CIStage{ID: "82", Name: "image-scan"},
		Name:  port.ImageScanReportArtifact,
		Path:  port.ImageScanReportFile,
	}
}

// upload-artifact 는 파일을 zip 으로 묶어 올린다. 다운로드 주소는 저장소로
// 리다이렉트된다 — 따라가서 zip 안의 리포트를 꺼낸다.
func TestBuildReader_ReadArtifact_ExtractsReportFromZip(t *testing.T) {
	archive := zipWith(t, "trivy-report.json", []byte(`{"SchemaVersion":2}`))
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/shop/actions/runs/7001/artifacts":
			assert.Equal(t, "trivy-report", r.URL.Query().Get("name"))
			_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": []map[string]any{
				{"id": 55, "name": "trivy-report", "expired": false,
					"archive_download_url": srv.URL + "/repos/acme/shop/actions/artifacts/55/zip"},
			}})
		case "/repos/acme/shop/actions/artifacts/55/zip":
			http.Redirect(w, r, srv.URL+"/blob/55", http.StatusFound)
		case "/blob/55":
			_, _ = w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	data, found, err := NewBuildReader(NewClient(srv.URL, "tok"), "acme").ReadArtifact(context.Background(), reportRef())
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{"SchemaVersion":2}`, string(data))
}

// 산출물 보존 기간이 지나면 expired 다. 없는 것과 같다.
func TestBuildReader_ReadArtifact_ExpiredIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/shop/actions/runs/7001/artifacts" {
			_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": []map[string]any{
				{"id": 55, "name": "trivy-report", "expired": true, "archive_download_url": "http://unused"},
			}})
			return
		}
		t.Errorf("만료된 산출물을 내려받으려 했다: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, found, err := NewBuildReader(NewClient(srv.URL, "tok"), "acme").ReadArtifact(context.Background(), reportRef())
	require.NoError(t, err)
	assert.False(t, found)
}

// 다운로드 주소는 API 응답에 들어 있다. API 와 다른 호스트면 토큰을 붙여 보내지 않는다.
func TestBuildReader_ReadArtifact_RefusesForeignDownloadHost(t *testing.T) {
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API 가 아닌 호스트로 요청했다 (Authorization=%q)", r.Header.Get("Authorization"))
	}))
	t.Cleanup(foreign.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": []map[string]any{
			{"id": 55, "name": "trivy-report", "expired": false, "archive_download_url": foreign.URL + "/steal"},
		}})
	}))
	t.Cleanup(srv.Close)

	_, _, err := NewBuildReader(NewClient(srv.URL, "tok"), "acme").ReadArtifact(context.Background(), reportRef())
	assert.Error(t, err)
}

// 압축을 풀면 커지는 산출물을 통째로 메모리에 올리지 않는다.
func TestBuildReader_ReadArtifact_RejectsOversizedEntry(t *testing.T) {
	archive := zipWith(t, "trivy-report.json", make([]byte, port.MaxCIArtifactBytes+1))
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/shop/actions/runs/7001/artifacts":
			_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": []map[string]any{
				{"id": 55, "name": "trivy-report", "expired": false, "archive_download_url": srv.URL + "/zip"},
			}})
		case "/zip":
			_, _ = w.Write(archive)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, err := NewBuildReader(NewClient(srv.URL, "tok"), "acme").ReadArtifact(context.Background(), reportRef())
	assert.Error(t, err)
}

package sourcearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const testCommit = "158daebd2bfdab951e90ad526c3d2a6b6b42513b"

type tarEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
}

// githubTarball 은 codeload 가 주는 모양의 아카이브다. 루트 디렉터리 하나 아래에 파일이 있고,
// 맨 앞에 커밋 SHA 를 담은 pax 전역 헤더가 붙는다.
func githubTarball(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeXGlobalHeader,
		Name:       "pax_global_header",
		PAXRecords: map[string]string{"comment": testCommit},
	}))
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: e.typeflag, Linkname: e.linkname}
		if e.typeflag == 0 {
			hdr.Typeflag = tar.TypeReg
		}
		if hdr.Typeflag != tar.TypeReg {
			hdr.Size = 0
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if hdr.Typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func serveTarball(t *testing.T, archive []byte) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/eGovFramework/egovframe-web-sample/tar.gz/"+testCommit {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-gzip")
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

func filesByPath(files []port.CommitFile) map[string]string {
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[f.Path] = f.Content
	}
	return out
}

func TestGitHub_Fetch_ReturnsFilesAtRepositoryRoot(t *testing.T) {
	font := "wOFF\x00\x01\xff\xfe"
	root := "egovframe-web-sample-" + testCommit + "/"
	srv, paths := serveTarball(t, githubTarball(t,
		tarEntry{name: root, typeflag: tar.TypeDir},
		tarEntry{name: root + "pom.xml", body: "<project/>"},
		tarEntry{name: root + "src/main/webapp/", typeflag: tar.TypeDir},
		tarEntry{name: root + "src/main/webapp/fonts/a.woff", body: font},
	))

	files, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	require.NoError(t, err)

	assert.Equal(t, []string{"/eGovFramework/egovframe-web-sample/tar.gz/" + testCommit}, *paths)
	got := filesByPath(files)
	assert.Equal(t, map[string]string{
		"pom.xml":                      "<project/>",
		"src/main/webapp/fonts/a.woff": font,
	}, got, "루트 디렉터리를 벗기고 디렉터리·pax 헤더는 빼며 바이너리는 바이트 그대로다")
}

func TestGitHub_Fetch_AcceptsGitSuffixAndTrailingSlash(t *testing.T) {
	srv, _ := serveTarball(t, githubTarball(t, tarEntry{name: "r/README.md", body: "# r"}))
	for _, repoURL := range []string{
		"https://github.com/eGovFramework/egovframe-web-sample.git",
		"https://github.com/eGovFramework/egovframe-web-sample/",
	} {
		files, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).Fetch(context.Background(), repoURL, testCommit)
		require.NoError(t, err, repoURL)
		assert.Len(t, files, 1, repoURL)
	}
}

func TestGitHub_Fetch_UnsupportedHost(t *testing.T) {
	_, err := NewGitHub(nil).Fetch(context.Background(), "https://gitlab.com/group/project", testCommit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github.com")
}

func TestGitHub_Fetch_NotFound(t *testing.T) {
	srv, _ := serveTarball(t, nil)
	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/missing", testCommit)
	assert.ErrorIs(t, err, port.ErrSourceNotFound)
}

func TestGitHub_Fetch_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close()

	_, err := NewGitHub(nil).WithBaseURL(base).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	assert.ErrorIs(t, err, port.ErrSourceUnreachable, "오프라인 환경에서는 인터넷이 필요하다고 알려야 한다")
}

func TestGitHub_Fetch_RejectsSymlinks(t *testing.T) {
	// 심볼릭 링크를 조용히 빼면 빌드가 엉뚱한 곳에서 깨진다. 무엇이 문제인지 그 자리에서 알린다.
	srv, _ := serveTarball(t, githubTarball(t,
		tarEntry{name: "r/pom.xml", body: "<project/>"},
		tarEntry{name: "r/link", typeflag: tar.TypeSymlink, linkname: "pom.xml"},
	))
	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link")
}

func TestGitHub_Fetch_RejectsPathTraversal(t *testing.T) {
	srv, _ := serveTarball(t, githubTarball(t, tarEntry{name: "r/../../etc/passwd", body: "x"}))
	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	require.Error(t, err)
}

func TestGitHub_Fetch_TooManyFiles(t *testing.T) {
	entries := make([]tarEntry, 0, 4)
	for _, name := range []string{"a", "b", "c", "d"} {
		entries = append(entries, tarEntry{name: "r/" + name, body: name})
	}
	srv, _ := serveTarball(t, githubTarball(t, entries...))
	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).WithLimits(Limits{MaxFiles: 3, MaxTotalBytes: 1 << 20, MaxFileBytes: 1 << 20}).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	assert.ErrorIs(t, err, port.ErrSourceTooLarge)
}

func TestGitHub_Fetch_TooLargeFile(t *testing.T) {
	srv, _ := serveTarball(t, githubTarball(t, tarEntry{name: "r/big.bin", body: strings.Repeat("x", 64)}))
	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).WithLimits(Limits{MaxFiles: 10, MaxTotalBytes: 1 << 20, MaxFileBytes: 32}).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	assert.True(t, errors.Is(err, port.ErrSourceTooLarge), "got %v", err)
}

// 원본 호스트 쪽 문제(한도 초과 · 장애 · 중간에 끊김)는 모두 "닿지 못했다" 로 분류한다. 요청을 고쳐도
// 낫지 않으므로 API 가 400 이 아니라 502 로 돌려준다.
func TestGitHub_Fetch_UpstreamFailuresAreUnreachable(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadGateway} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
			Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
		srv.Close()
		assert.ErrorIs(t, err, port.ErrSourceUnreachable, "HTTP %d", status)
	}
}

func TestGitHub_Fetch_TruncatedArchiveIsUnreachable(t *testing.T) {
	archive := githubTarball(t, tarEntry{name: "r/pom.xml", body: strings.Repeat("x", 4096)})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive[:len(archive)/2])
	}))
	defer srv.Close()

	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	assert.ErrorIs(t, err, port.ErrSourceUnreachable, "중간에 끊긴 응답도 원본 호스트 쪽 문제다")
}

// 원본에 담을 수 없는 항목(심볼릭 링크 · 저장소 밖 경로)과 지원하지 않는 호스트는 템플릿이 쓸 수 없는
// 원본이라는 뜻이다.
func TestGitHub_Fetch_UnusableSourceIsUnsupported(t *testing.T) {
	srv, _ := serveTarball(t, githubTarball(t, tarEntry{name: "r/link", typeflag: tar.TypeSymlink, linkname: "x"}))
	_, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	assert.ErrorIs(t, err, port.ErrSourceUnsupported)

	_, err = NewGitHub(nil).Fetch(context.Background(), "https://gitlab.com/group/project", testCommit)
	assert.ErrorIs(t, err, port.ErrSourceUnsupported)
}

func TestGitHub_Supports(t *testing.T) {
	g := NewGitHub(nil)
	assert.NoError(t, g.Supports("https://github.com/eGovFramework/egovframe-web-sample"))
	assert.NoError(t, g.Supports("https://github.com/eGovFramework/egovframe-web-sample.git"))
	for _, url := range []string{
		"https://gitlab.com/group/project",
		"https://github.com/eGovFramework/egovframe-web-sample/tree/main",
		"https://github.com/eGovFramework",
	} {
		assert.ErrorIs(t, g.Supports(url), port.ErrSourceUnsupported, url)
	}
}

// 원본의 실행 권한(mvnw · entrypoint.sh)을 잃으면 원본 Dockerfile 이 permission denied 로 죽는다.
func TestGitHub_Fetch_KeepsExecutableBit(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range []struct {
		name string
		mode int64
	}{{"r/mvnw", 0o755}, {"r/pom.xml", 0o644}} {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: e.name, Mode: e.mode, Size: 1, Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte("x"))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	srv, _ := serveTarball(t, buf.Bytes())

	files, err := NewGitHub(srv.Client()).WithBaseURL(srv.URL).
		Fetch(context.Background(), "https://github.com/eGovFramework/egovframe-web-sample", testCommit)
	require.NoError(t, err)
	exec := map[string]bool{}
	for _, f := range files {
		exec[f.Path] = f.Executable
	}
	assert.Equal(t, map[string]bool{"mvnw": true, "pom.xml": false}, exec)
}

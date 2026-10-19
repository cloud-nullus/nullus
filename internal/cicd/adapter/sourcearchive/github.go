// Package sourcearchive 는 외부 원본 저장소의 특정 커밋 파일을 가져온다.
//
// 템플릿이 원본을 새 앱 저장소로 복사할 때 쓴다(port.SourceArchive). git 을 실행하지 않고
// 호스트가 내주는 커밋 아카이브를 받아 푼다 — API 파드에는 git 이 없고, 이력은 필요 없다.
package sourcearchive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// defaultCodeloadURL 은 GitHub 이 커밋 아카이브를 내주는 곳이다. github.com/<o>/<r>/archive/<sha>.tar.gz
// 도 결국 여기로 보낸다. REST API(api.github.com)를 거치지 않으므로 비인증 호출 한도(시간당 60회)에
// 걸리지 않는다.
const defaultCodeloadURL = "https://codeload.github.com"

// defaultTimeout 은 아카이브 하나를 받는 데 기다리는 시간이다. 파이프라인 생성 요청 안에서
// 돌기 때문에 무한정 기다리지 않는다.
const defaultTimeout = 2 * time.Minute

// Limits 는 가져올 수 있는 원본의 크기다.
//
// 가져온 파일은 SCM 커밋 API 요청 하나에 실린다. 너무 크면 SCM 이 요청을 거부하거나
// API 메모리를 다 쓴다 — 템플릿은 출발점이라 큰 저장소를 옮기는 용도가 아니다.
type Limits struct {
	MaxFiles      int
	MaxTotalBytes int64
	MaxFileBytes  int64
}

// DefaultLimits 는 eGov 샘플(파일 약 150개, 2.8MB)에 여유를 둔 값이다. 가져온 파일은 커밋 요청
// 하나에 실리고(GitLab 커밋 API 는 요청 하나, 바이너리는 base64 로 3분의 1 이 늘어난다) 그 요청은
// 타임아웃 안에 끝나야 한다.
var DefaultLimits = Limits{
	MaxFiles:      1000,
	MaxTotalBytes: 32 << 20,
	MaxFileBytes:  8 << 20,
}

// GitHub 은 github.com 저장소의 커밋 아카이브를 받는다.
type GitHub struct {
	client  *http.Client
	baseURL string
	limits  Limits
}

var _ port.SourceArchive = (*GitHub)(nil)

// NewGitHub 은 어댑터를 만든다. client 가 nil 이면 제한 시간을 둔 기본 클라이언트를 쓴다.
func NewGitHub(client *http.Client) *GitHub {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &GitHub{client: client, baseURL: defaultCodeloadURL, limits: DefaultLimits}
}

// WithBaseURL 은 아카이브를 받을 주소를 바꾼다(테스트·사설 미러).
func (g *GitHub) WithBaseURL(baseURL string) *GitHub {
	g.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return g
}

// WithLimits 는 크기 한도를 바꾼다.
func (g *GitHub) WithLimits(limits Limits) *GitHub {
	g.limits = limits
	return g
}

// Fetch 는 저장소의 commit 시점 파일을 돌려준다.
func (g *GitHub) Fetch(ctx context.Context, repoURL, commit string) ([]port.CommitFile, error) {
	owner, repo, err := parseGitHubRepo(repoURL)
	if err != nil {
		return nil, err
	}
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return nil, fmt.Errorf("원본 커밋이 비어 있습니다")
	}

	endpoint := fmt.Sprintf("%s/%s/%s/tar.gz/%s",
		g.baseURL, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(commit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build source archive request: %w", err)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w (%s/%s@%s): %v", port.ErrSourceUnreachable, owner, repo, shortSHA(commit), err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 404 가 아닌 실패(한도 초과 403 · 429, 장애 5xx)는 원본 호스트 쪽 문제다 — 요청을 고쳐도 낫지 않는다.
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s/%s@%s", port.ErrSourceNotFound, owner, repo, shortSHA(commit))
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w (%s/%s@%s): HTTP %d",
			port.ErrSourceUnreachable, owner, repo, shortSHA(commit), resp.StatusCode)
	}

	files, err := g.extract(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("원본 아카이브를 풀지 못했습니다 (%s/%s@%s): %w", owner, repo, shortSHA(commit), err)
	}
	return files, nil
}

// Supports 는 그 주소를 가져올 수 있는 모양인지 본다. 네트워크를 쓰지 않는다.
func (g *GitHub) Supports(repoURL string) error {
	_, _, err := parseGitHubRepo(repoURL)
	return err
}

// extract 는 tar.gz 를 풀어 저장소 루트 기준 파일로 돌려준다.
//
// codeload 아카이브는 "<repo>-<sha>/" 디렉터리 하나 아래에 파일을 둔다. 첫 경로 마디를 벗긴다.
func (g *GitHub) extract(body io.Reader) ([]port.CommitFile, error) {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", port.ErrSourceUnreachable, err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var (
		files []port.CommitFile
		total int64
	)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			// 응답이 중간에 끊겼거나 깨졌다 — 원본 호스트·네트워크 쪽 문제다.
			return nil, fmt.Errorf("%w: %v", port.ErrSourceUnreachable, err)
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			// 심볼릭 링크를 조용히 빼면 빌드가 엉뚱한 곳에서 깨진다. CommitFile 로는 링크를 담을 수 없다.
			return nil, fmt.Errorf("%w: 일반 파일이 아닌 항목은 가져올 수 없습니다: %s", port.ErrSourceUnsupported, hdr.Name)
		}

		rel, err := repositoryPath(hdr.Name)
		if err != nil {
			return nil, err
		}
		if len(files) >= g.limits.MaxFiles {
			return nil, fmt.Errorf("%w: 파일이 %d개를 넘습니다", port.ErrSourceTooLarge, g.limits.MaxFiles)
		}
		if hdr.Size > g.limits.MaxFileBytes {
			return nil, fmt.Errorf("%w: %s 가 %d 바이트를 넘습니다", port.ErrSourceTooLarge, rel, g.limits.MaxFileBytes)
		}
		total += hdr.Size
		if total > g.limits.MaxTotalBytes {
			return nil, fmt.Errorf("%w: 전체 크기가 %d 바이트를 넘습니다", port.ErrSourceTooLarge, g.limits.MaxTotalBytes)
		}

		content, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
		if err != nil {
			return nil, fmt.Errorf("%w: read %s: %v", port.ErrSourceUnreachable, rel, err)
		}
		files = append(files, port.CommitFile{
			Path:       rel,
			Content:    string(content),
			Executable: hdr.Mode&0o111 != 0,
		})
	}
}

// repositoryPath 는 아카이브 항목 이름에서 루트 디렉터리를 벗긴 저장소 경로다.
func repositoryPath(name string) (string, error) {
	_, rest, ok := strings.Cut(name, "/")
	if !ok || rest == "" {
		return "", fmt.Errorf("%w: 아카이브 루트 밖의 항목입니다: %s", port.ErrSourceUnsupported, name)
	}
	cleaned := path.Clean(rest)
	if cleaned == "." || path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: 저장소 밖을 가리키는 경로입니다: %s", port.ErrSourceUnsupported, name)
	}
	return cleaned, nil
}

// parseGitHubRepo 는 https://github.com/<owner>/<repo>(.git) 에서 소유자와 이름을 꺼낸다.
func parseGitHubRepo(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil {
		return "", "", fmt.Errorf("%w: 원본 주소를 읽지 못했습니다: %v", port.ErrSourceUnsupported, err)
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "https" || (host != "github.com" && host != "www.github.com") {
		return "", "", fmt.Errorf("%w: 지원하지 않는 원본 호스트입니다 (%s) — 지금은 https://github.com 저장소만 가져올 수 있습니다", port.ErrSourceUnsupported, u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: 원본 주소는 https://github.com/<소유자>/<저장소> 모양이어야 합니다: %s", port.ErrSourceUnsupported, repoURL)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func shortSHA(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

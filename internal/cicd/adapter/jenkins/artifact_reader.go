package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ReadArtifact 는 빌드가 archiveArtifacts 로 남긴 파일을 읽는다.
//
// Jenkins 는 산출물을 빌드 단위로 보관한다. 단계·묶음 이름은 쓰지 않는다.
func (c *Client) ReadArtifact(ctx context.Context, ref port.CIArtifactRef) ([]byte, bool, error) {
	job := strings.TrimSpace(ref.JobName)
	file := strings.Trim(strings.TrimSpace(ref.Path), "/")
	if job == "" || file == "" || ref.Build.Number <= 0 {
		return nil, false, nil
	}

	// multibranch job 은 브랜치가 하위 job 이다(ListBuilds 와 같은 경로 규칙).
	path := "/job/" + url.PathEscape(job)
	if b := strings.TrimSpace(ref.Branch); b != "" {
		path += "/job/" + url.PathEscape(b)
	}
	path += fmt.Sprintf("/%d/artifact/%s", ref.Build.Number, escapeSegments(file))
	return c.getRaw(ctx, path)
}

// ArtifactWebURL 은 빌드 산출물 파일을 Jenkins 화면에서 여는 주소다.
func (c *Client) ArtifactWebURL(ref port.CIArtifactRef) string {
	job := strings.TrimSpace(ref.JobName)
	file := strings.Trim(strings.TrimSpace(ref.Path), "/")
	if c.webBaseURL == "" || job == "" || file == "" || ref.Build.Number <= 0 {
		return ""
	}
	path := "/job/" + url.PathEscape(job)
	if b := strings.TrimSpace(ref.Branch); b != "" {
		path += "/job/" + url.PathEscape(b)
	}
	return c.webBaseURL + path + fmt.Sprintf("/%d/artifact/%s", ref.Build.Number, escapeSegments(file))
}

// getRaw 는 JSON 이 아닌 응답 본문을 상한까지만 읽는다. 404 는 found=false 다.
func (c *Client) getRaw(ctx context.Context, path string) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, false, err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("jenkins GET %s: %s", path, describeError(resp))
	}
	data, err := readLimited(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("jenkins GET %s: %w", path, err)
	}
	return data, true, nil
}

func escapeSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, port.MaxCIArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > port.MaxCIArtifactBytes {
		return nil, fmt.Errorf("산출물이 상한(%d바이트)을 넘습니다", port.MaxCIArtifactBytes)
	}
	return data, nil
}

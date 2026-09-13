package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ReadArtifact 는 잡이 artifacts.paths 로 남긴 파일 하나를 읽는다.
//
// GitLab 은 산출물을 잡 단위로 보관한다. 단계의 잡 id 가 없으면 물을 곳이 없다 —
// 추측해서 다른 잡의 산출물을 읽지 않는다.
func (r *BuildReader) ReadArtifact(ctx context.Context, ref port.CIArtifactRef) ([]byte, bool, error) {
	app := strings.TrimSpace(ref.JobName)
	jobID := strings.TrimSpace(ref.Stage.ID)
	file := strings.Trim(strings.TrimSpace(ref.Path), "/")
	if app == "" || jobID == "" || file == "" {
		return nil, false, nil
	}

	path := r.projectBase(app) + "/jobs/" + url.PathEscape(jobID) + "/artifacts/" + escapeSegments(file)
	return r.client.getRaw(ctx, path)
}

// getRaw 는 JSON 이 아닌 응답 본문을 상한까지만 읽는다. 404 는 found=false 다.
func (c *Client) getRaw(ctx context.Context, path string) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, false, err
	}
	c.applyAuth(req)
	// 파일 본문이다. JSON 만 받겠다고 하면 406 이 날 수 있다.
	req.Header.Set("Accept", "*/*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode >= 300 {
		return nil, false, statusError(resp)
	}
	data, err := readLimited(resp.Body)
	if err != nil {
		return nil, false, err
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

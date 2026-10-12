package github

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ReadArtifact 는 실행이 upload-artifact 로 남긴 묶음에서 파일 하나를 꺼낸다.
//
// GitHub 은 산출물을 실행 안의 이름 붙은 zip 으로 보관한다. 보존 기간이 지나면
// expired 가 되고, 그것은 없는 것과 같다.
func (r *BuildReader) ReadArtifact(ctx context.Context, ref port.CIArtifactRef) ([]byte, bool, error) {
	repo, owner := strings.TrimSpace(ref.JobName), strings.TrimSpace(r.owner)
	runID := strings.TrimSpace(ref.Build.ID)
	name := strings.TrimSpace(ref.Name)
	file := strings.Trim(strings.TrimSpace(ref.Path), "/")
	if repo == "" || owner == "" || runID == "" || name == "" || file == "" {
		return nil, false, nil
	}

	q := url.Values{}
	q.Set("name", name)
	q.Set("per_page", "100")
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) +
		"/actions/runs/" + url.PathEscape(runID) + "/artifacts?" + q.Encode()

	var payload struct {
		Artifacts []struct {
			Name               string `json:"name"`
			Expired            bool   `json:"expired"`
			ArchiveDownloadURL string `json:"archive_download_url"`
		} `json:"artifacts"`
	}
	found, err := r.client.get(ctx, path, &payload)
	if err != nil {
		return nil, false, fmt.Errorf("github 산출물 목록 조회 (%s/%s run %s): %w", owner, repo, runID, err)
	}
	if !found {
		return nil, false, nil
	}

	for _, a := range payload.Artifacts {
		if a.Name != name {
			continue
		}
		if a.Expired {
			return nil, false, nil
		}
		archive, ok, err := r.client.download(ctx, a.ArchiveDownloadURL)
		if err != nil || !ok {
			return nil, false, err
		}
		return extractFromZip(archive, file)
	}
	return nil, false, nil
}

// ArtifactWebURL 은 실행 페이지 주소다. GitHub 은 산출물 파일 주소를 주지 않고,
// 실행 페이지가 산출물 목록을 보여 준다.
func (r *BuildReader) ArtifactWebURL(ref port.CIArtifactRef) string {
	repo, owner := strings.TrimSpace(ref.JobName), strings.TrimSpace(r.owner)
	runID := strings.TrimSpace(ref.Build.ID)
	if repo == "" || owner == "" || runID == "" || r.client == nil {
		return ""
	}
	return WebBaseURLFor(r.client.baseURL) + "/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) +
		"/actions/runs/" + url.PathEscape(runID)
}

// download 는 API 가 알려준 절대 주소에서 내려받는다.
//
// 주소는 응답 본문에서 왔다. API 와 다른 호스트면 토큰을 붙여 보내지 않는다.
// 저장소로 가는 리다이렉트는 따라가되, Go 의 http 클라이언트는 다른 호스트로
// 넘어갈 때 Authorization 을 떼어 낸다.
func (c *Client) download(ctx context.Context, rawURL string) ([]byte, bool, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, false, fmt.Errorf("github: 산출물 다운로드 주소를 해석할 수 없습니다: %w", err)
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, false, err
	}
	if !strings.EqualFold(target.Scheme, base.Scheme) || !strings.EqualFold(target.Host, base.Host) {
		return nil, false, fmt.Errorf("github: 산출물 다운로드 주소의 호스트(%s)가 API 호스트(%s)와 다릅니다",
			target.Host, base.Host)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, false, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
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

// extractFromZip 은 zip 안에서 파일 하나를 상한까지만 꺼낸다.
//
// 헤더의 크기는 믿지 않고 먼저 거른 뒤, 실제로 풀면서도 상한을 건다.
func extractFromZip(archive []byte, file string) ([]byte, bool, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, false, fmt.Errorf("github: 산출물 zip 을 열 수 없습니다: %w", err)
	}
	for _, f := range zr.File {
		if strings.Trim(f.Name, "/") != file {
			continue
		}
		if f.UncompressedSize64 > uint64(port.MaxCIArtifactBytes) {
			return nil, false, fmt.Errorf("github: 산출물 %s 이 상한(%d바이트)을 넘습니다", file, port.MaxCIArtifactBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, false, err
		}
		data, err := readLimited(rc)
		_ = rc.Close()
		if err != nil {
			return nil, false, err
		}
		return data, true, nil
	}
	return nil, false, nil
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

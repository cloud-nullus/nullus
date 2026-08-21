// Package harbor 는 Harbor 레지스트리에서 이미지 저장소를 지운다.
//
// 플랫폼은 Harbor 프로젝트를 만들고 이미지를 밀어 넣으면서 정리할 수단은 갖고
// 있지 않았다. 파이프라인을 지워도 이미지가 남아 디스크는 아무도 안 보는 사이에
// 찬다.
//
// github.Client 가 GHCR 패키지에 대해 하는 일과 같은 자리다 — port.ImageRepositoryDeleter
// 의 Harbor 구현체이고, 다른 레지스트리는 각자 같은 계약을 구현하면 된다.
package harbor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	defaultTimeout       = 30 * time.Second
	maxErrorBodyReadSize = 4 << 10
	apiPathPrefix        = "/api/v2.0"
)

// Client 는 Harbor API 클라이언트다.
type Client struct {
	baseURL    string
	user       string
	password   string
	httpClient *http.Client
}

// NewClient 는 Harbor 클라이언트를 만든다.
//
// baseURL 은 Harbor 의 외부 주소다 (예: https://harbor.nullus.io).
// user/password 는 관리자 자격증명이며, 스택 설치가 OpenBao 에 만들어 둔 것을
// registrycreds 가 풀어 준다.
func NewClient(baseURL, user, password string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		user:       strings.TrimSpace(user),
		password:   password,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// WithHTTPClient 는 타임아웃·전송 계층을 교체한다.
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	if h != nil {
		c.httpClient = h
	}
	return c
}

// DeleteImageRepository 는 프로젝트 안의 저장소를 통째로 지운다.
//
// 이미 없으면 성공으로 본다 — 삭제의 목표는 "없는 상태" 이고, 404 를 오류로
// 올리면 재시도가 영영 끝나지 않는다.
func (c *Client) DeleteImageRepository(ctx context.Context, target *port.ImageTarget) error {
	if target == nil || target.Kind != port.RegistryKindHarbor {
		// 다른 레지스트리의 대상을 조용히 성공으로 넘기면 사용자는 이미지가
		// 사라진 줄 안다.
		return port.ErrImageDeletionUnsupported
	}

	project, repository, err := splitRepository(target.Repository)
	if err != nil {
		return err
	}

	// 저장소 이름에는 슬래시가 들어갈 수 있다(team/app). Harbor 는 그것을 경로가
	// 아니라 **하나의 이름**으로 받으므로 인코딩해야 한다 — 그러지 않으면 404 다.
	endpoint := fmt.Sprintf("%s/projects/%s/repositories/%s",
		apiPathPrefix, url.PathEscape(project), url.PathEscape(repository))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	if c.user != "" || c.password != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete harbor repository %s/%s: %w", project, repository, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete harbor repository %s/%s: %s",
			project, repository, describeError(resp))
	}
	return nil
}

// splitRepository 는 완전한 이미지 경로에서 프로젝트와 저장소를 가른다.
//
//	harbor.nullus.io/nullus/sample-frontend → nullus, sample-frontend
//	harbor.nullus.io/nullus/team/app        → nullus, team/app
//
// 가려내지 못하면 지우지 않는다. 잘못 가려내면 남의 저장소를 지운다.
func splitRepository(repository string) (project, repo string, err error) {
	trimmed := strings.Trim(strings.TrimSpace(repository), "/")
	parts := strings.Split(trimmed, "/")
	// 호스트 / 프로젝트 / 저장소… 최소 세 조각이 있어야 한다.
	if len(parts) < 3 {
		return "", "", fmt.Errorf(
			"harbor 이미지 경로에서 프로젝트와 저장소를 가려내지 못했습니다: %q", repository)
	}
	project = parts[1]
	repo = strings.Join(parts[2:], "/")
	if project == "" || repo == "" {
		return "", "", fmt.Errorf(
			"harbor 이미지 경로에서 프로젝트와 저장소를 가려내지 못했습니다: %q", repository)
	}
	return project, repo, nil
}

// describeError 는 응답 본문을 잘라 오류 메시지에 담는다.
// 상태 코드만으로는 무엇이 잘못됐는지 알 수 없다.
func describeError(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyReadSize))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return resp.Status
	}
	return resp.Status + ": " + msg
}

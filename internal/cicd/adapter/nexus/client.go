// Package nexus 는 Nexus 레지스트리에서 이미지를 지운다.
//
// Harbor 와 달리 "이미지 저장소를 통째로 지운다" 는 단일 API 가 없다. 이름으로
// 컴포넌트를 찾아 하나씩 지운다 — 태그마다 컴포넌트가 하나씩 생기므로 여럿이다.
//
// harbor.Client 와 같은 자리다 — port.ImageRepositoryDeleter 의 Nexus 구현체이고,
// 어느 레지스트리를 쓰는지는 번들을 조립하는 쪽이 정한다.
package nexus

import (
	"context"
	"encoding/json"
	"errors"
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
	apiPathPrefix        = "/service/rest/v1"

	// dockerRepository 는 스택 설치가 만드는 docker 저장소 이름이다.
	//
	// 스택 모듈의 domain.NexusDockerRepository 와 같아야 한다. 모듈 경계 때문에
	// 직접 참조하지 않고 값을 맞춰 둔다 — 갈라지면 검색이 아무것도 찾지 못하고,
	// 삭제는 "지울 것이 없었다" 로 조용히 성공한다.
	dockerRepository = "docker-hosted"

	// searchPageLimit 은 한 번에 도는 쪽의 상한이다. 무한 루프를 막는 안전장치이지
	// 페이지 크기가 아니다 — 페이지 크기는 Nexus 가 정한다.
	searchPageLimit = 100
)

// Client 는 Nexus API 클라이언트다.
type Client struct {
	baseURL    string
	user       string
	password   string
	httpClient *http.Client
}

// NewClient 는 Nexus 클라이언트를 만든다.
//
// baseURL 은 Nexus 의 API 주소다 (예: https://nexus.nullus.io). docker 커넥터
// 주소(registry.<도메인>)와 다르다 — 그쪽은 이미지 push/pull 전용 포트다.
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

// DeleteImageRepository 는 이 이미지의 컴포넌트를 전부 지운다.
//
// 하나가 실패해도 나머지를 시도한 뒤 실패 사실을 올린다. 첫 실패에서 멈추면
// 절반만 지워진 채 성공도 실패도 아닌 상태가 된다.
func (c *Client) DeleteImageRepository(ctx context.Context, target *port.ImageTarget) error {
	if target == nil || target.Kind != port.RegistryKindNexus {
		return port.ErrImageDeletionUnsupported
	}

	image, err := imageName(target.Repository)
	if err != nil {
		return err
	}

	ids, err := c.findComponents(ctx, image)
	if err != nil {
		return err
	}

	var failures []error
	for _, id := range ids {
		if delErr := c.deleteComponent(ctx, id); delErr != nil {
			failures = append(failures, delErr)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("nexus 이미지 %s 삭제 중 %d건 실패: %w",
			image, len(failures), errors.Join(failures...))
	}
	return nil
}

// findComponents 는 이름이 같은 컴포넌트의 id 를 모은다.
//
// 검색 결과는 쪽으로 나뉜다. 첫 쪽만 지우면 나머지 태그가 조용히 남는다.
func (c *Client) findComponents(ctx context.Context, image string) ([]string, error) {
	ids := make([]string, 0, 8)
	token := ""

	for page := 0; page < searchPageLimit; page++ {
		query := url.Values{}
		query.Set("repository", dockerRepository)
		query.Set("name", image)
		if token != "" {
			query.Set("continuationToken", token)
		}

		var payload struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
			ContinuationToken string `json:"continuationToken"`
		}
		if err := c.get(ctx, apiPathPrefix+"/search?"+query.Encode(), &payload); err != nil {
			return nil, fmt.Errorf("search nexus components for %s: %w", image, err)
		}

		for _, item := range payload.Items {
			if strings.TrimSpace(item.ID) != "" {
				ids = append(ids, item.ID)
			}
		}
		token = strings.TrimSpace(payload.ContinuationToken)
		if token == "" {
			return ids, nil
		}
	}
	return ids, nil
}

func (c *Client) deleteComponent(ctx context.Context, id string) error {
	endpoint := apiPathPrefix + "/components/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete nexus component %s: %w", id, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 이미 없으면 성공으로 본다 — 삭제의 목표는 "없는 상태" 다.
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete nexus component %s: %s", id, describeError(resp))
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s", describeError(resp))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) applyAuth(req *http.Request) {
	if c.user != "" || c.password != "" {
		req.SetBasicAuth(c.user, c.password)
	}
}

// imageName 은 완전한 이미지 경로에서 이름을 가려낸다.
//
//	registry.nullus.io/sample-frontend → sample-frontend
//
// 가려내지 못하면 지우지 않는다. 빈 이름으로 검색하면 저장소의 모든 컴포넌트가
// 걸려 남의 이미지까지 지운다.
func imageName(repository string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(repository), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("nexus 이미지 경로에서 이름을 가려내지 못했습니다: %q", repository)
	}
	name := strings.Join(parts[1:], "/")
	if name == "" {
		return "", fmt.Errorf("nexus 이미지 경로에서 이름을 가려내지 못했습니다: %q", repository)
	}
	return name, nil
}

func describeError(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyReadSize))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return resp.Status
	}
	return resp.Status + ": " + msg
}

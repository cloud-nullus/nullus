// Package gitlab 은 GitLab API v4 를 통해 그룹·프로젝트·파일을 프로비저닝한다.
//
// GitLab 의 Container Registry 와 Package Registry 는 프로젝트 단위로 존재한다.
// 따라서 공용 베이스 이미지나 npm/maven 패키지를 두려면 그것을 소유할 프로젝트가
// 반드시 필요하다 — 이 어댑터가 그 프로젝트를 만드는 경로다.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
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
	defaultBranchName    = "main"
	defaultVisibility    = "private"
	maxErrorBodyReadSize = 4 << 10
)

// Client 는 port.SCMProvisioner 의 GitLab 구현체다.
type Client struct {
	baseURL      string
	token        string
	registryHost string
	httpClient   *http.Client
}

// NewClient 는 GitLab API v4 클라이언트를 만든다.
//
// baseURL 은 API 접두사를 뺀 인스턴스 주소다 (예: http://gitlab-webservice-default.ns.svc:8181).
// token 은 api 스코프를 가진 Personal/Project Access Token 이다.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// WithRegistryHost 는 응답에 container_registry_url 이 없을 때 쓸 폴백 호스트를 설정한다.
func (c *Client) WithRegistryHost(host string) *Client {
	c.registryHost = strings.TrimRight(strings.TrimSpace(host), "/")
	return c
}

// BaseURL 은 이 클라이언트가 바라보는 인스턴스 주소다.
func (c *Client) BaseURL() string { return c.baseURL }

// Ping 은 현재 토큰이 실제로 인증되는지 확인한다.
//
// 보관된 토큰은 폐기·만료될 수 있어, 쓰기 전에 한 번 확인하지 않으면 이후
// 모든 호출이 401 로 죽는다.
func (c *Client) Ping(ctx context.Context) error {
	var out struct {
		Username string `json:"username"`
	}
	found, err := c.get(ctx, "/api/v4/user", &out)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("gitlab user endpoint not found")
	}
	return nil
}

// WithHTTPClient 는 타임아웃·전송 계층을 교체한다.
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	if h != nil {
		c.httpClient = h
	}
	return c
}

// EnsureGroup 은 그룹을 조회하고 없으면 만든다.
func (c *Client) EnsureGroup(ctx context.Context, spec port.GroupSpec) (*port.SCMGroup, error) {
	path := strings.TrimSpace(spec.Path)
	if path == "" {
		return nil, fmt.Errorf("group path is required")
	}

	var existing groupResponse
	found, err := c.get(ctx, "/api/v4/groups/"+url.PathEscape(path), &existing)
	if err != nil {
		return nil, fmt.Errorf("lookup group %q: %w", path, err)
	}
	if found {
		return existing.toDomain(), nil
	}

	body := map[string]any{
		"name": firstNonEmpty(spec.Name, path),
		"path": path,
	}
	if spec.Description != "" {
		body["description"] = spec.Description
	}

	var created groupResponse
	if err := c.post(ctx, "/api/v4/groups", body, &created); err != nil {
		return nil, fmt.Errorf("create group %q: %w", path, err)
	}
	return created.toDomain(), nil
}

// EnsureProject 는 프로젝트를 조회하고 없으면 만든다.
func (c *Client) EnsureProject(ctx context.Context, spec port.ProjectSpec) (*port.SCMProject, error) {
	path := strings.TrimSpace(spec.Path)
	if path == "" {
		return nil, fmt.Errorf("project path is required")
	}

	lookup := path
	if group := strings.TrimSpace(spec.GroupPath); group != "" {
		lookup = group + "/" + path
	}

	var existing projectResponse
	found, err := c.get(ctx, "/api/v4/projects/"+url.PathEscape(lookup), &existing)
	if err != nil {
		return nil, fmt.Errorf("lookup project %q: %w", lookup, err)
	}
	if found {
		return c.projectToDomain(existing), nil
	}

	body := map[string]any{
		"name":       firstNonEmpty(spec.Name, path),
		"path":       path,
		"visibility": firstNonEmpty(spec.Visibility, defaultVisibility),
		// 브랜치가 없으면 파일 커밋이 실패하므로 첫 커밋 전에 기본 브랜치를 만든다.
		"initialize_with_readme": spec.InitReadme,
	}
	if spec.Description != "" {
		body["description"] = spec.Description
	}
	if gid := strings.TrimSpace(spec.GroupID); gid != "" {
		id, convErr := parseID(gid)
		if convErr != nil {
			return nil, fmt.Errorf("invalid group id %q: %w", gid, convErr)
		}
		body["namespace_id"] = id
	}

	var created projectResponse
	if err := c.post(ctx, "/api/v4/projects", body, &created); err != nil {
		return nil, fmt.Errorf("create project %q: %w", path, err)
	}
	project := c.projectToDomain(created)
	project.Created = true
	return project, nil
}

// CommitFiles 는 여러 파일을 한 커밋으로 올린다.
//
// GitLab 커밋 API 에는 upsert 액션이 없고, action 은 파일마다 지정한다.
// 새 프로젝트는 initialize_with_readme 로 README.md 만 있는 상태라 스캐폴딩은
// "이미 있는 파일 + 없는 파일" 이 섞인다. 커밋 전체에 하나의 action 을 쓰면
// 어느 쪽으로 보내도 실패하므로, 트리를 한 번 읽어 파일별로 정한다.
func (c *Client) CommitFiles(ctx context.Context, projectID string, spec port.CommitSpec) error {
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("project id is required")
	}
	if len(spec.Files) == 0 {
		return nil
	}

	branch := firstNonEmpty(spec.Branch, defaultBranchName)
	existing := c.existingFilePaths(ctx, projectID, branch)

	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/commits", url.PathEscape(projectID))
	if err := c.post(ctx, endpoint, commitBody(branch, spec, existing), nil); err != nil {
		return fmt.Errorf("commit files to project %s: %w", projectID, err)
	}
	return nil
}

// existingFilePaths 는 브랜치에 이미 있는 파일 경로 집합이다.
//
// 조회에 실패하면 빈 집합을 돌려준다 — 빈 저장소이거나 브랜치가 아직 없는
// 경우이므로 전부 create 로 보내는 것이 맞다.
func (c *Client) existingFilePaths(ctx context.Context, projectID, branch string) map[string]struct{} {
	paths := map[string]struct{}{}

	endpoint := fmt.Sprintf("/api/v4/projects/%s/repository/tree?ref=%s&recursive=true&per_page=100",
		url.PathEscape(projectID), url.QueryEscape(branch))

	var entries []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	found, err := c.get(ctx, endpoint, &entries)
	if err != nil || !found {
		return paths
	}
	for _, e := range entries {
		if e.Type == "blob" {
			paths[e.Path] = struct{}{}
		}
	}
	return paths
}

func commitBody(branch string, spec port.CommitSpec, existing map[string]struct{}) map[string]any {
	actions := make([]map[string]any, 0, len(spec.Files))
	for _, f := range spec.Files {
		action := "create"
		if _, ok := existing[f.Path]; ok {
			action = "update"
		}
		actions = append(actions, map[string]any{
			"action":    action,
			"file_path": f.Path,
			"content":   f.Content,
		})
	}
	return map[string]any{
		"branch":         branch,
		"commit_message": spec.Message,
		"actions":        actions,
	}
}

// --- HTTP 하부 ---

// get 은 200 이면 true, 404 면 false 를 돌려준다. 그 외는 오류다.
func (c *Client) get(ctx context.Context, path string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		return false, statusError(resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return false, fmt.Errorf("decode response: %w", err)
		}
	}
	return true, nil
}

func (c *Client) post(ctx context.Context, path string, body map[string]any, out any) error {
	return c.send(ctx, http.MethodPost, path, body, out)
}

func (c *Client) put(ctx context.Context, path string, body map[string]any, out any) error {
	return c.send(ctx, http.MethodPut, path, body, out)
}

func (c *Client) send(ctx context.Context, method, path string, body map[string]any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return statusError(resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) applyAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	req.Header.Set("Accept", "application/json")
}

type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("gitlab api status=%d body=%s", e.StatusCode, e.Body)
}

func statusError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyReadSize)) // #nosec G104 -- best-effort
	return &apiError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
}

// isAlreadyExistsError 는 커밋 create 액션이 기존 파일과 충돌했는지 본다.
func isAlreadyExistsError(err error) bool {
	var apiErr *apiError
	if !asAPIError(err, &apiErr) {
		return false
	}
	msg := strings.ToLower(apiErr.Body)
	return strings.Contains(msg, "already exists")
}

func asAPIError(err error, target **apiError) bool {
	for err != nil {
		if e, ok := err.(*apiError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// --- 응답 매핑 ---

type groupResponse struct {
	ID       json.Number `json:"id"`
	Name     string      `json:"name"`
	FullPath string      `json:"full_path"`
	WebURL   string      `json:"web_url"`
}

func (g groupResponse) toDomain() *port.SCMGroup {
	return &port.SCMGroup{
		ID:       g.ID.String(),
		Name:     g.Name,
		FullPath: g.FullPath,
		WebURL:   g.WebURL,
	}
}

type projectResponse struct {
	ID                   json.Number `json:"id"`
	Name                 string      `json:"name"`
	PathWithNamespace    string      `json:"path_with_namespace"`
	WebURL               string      `json:"web_url"`
	HTTPURLToRepo        string      `json:"http_url_to_repo"`
	ContainerRegistryURL string      `json:"container_registry_url"`
	DefaultBranch        string      `json:"default_branch"`
}

func (c *Client) projectToDomain(p projectResponse) *port.SCMProject {
	registry := strings.TrimSpace(p.ContainerRegistryURL)
	if registry == "" && c.registryHost != "" && p.PathWithNamespace != "" {
		// 응답에 레지스트리 경로가 없는 버전이 있다. 경로를 모르면 CI 가 이미지를
		// 어디에 올릴지 정할 수 없으므로 호스트 + 프로젝트 경로로 만든다.
		registry = c.registryHost + "/" + p.PathWithNamespace
	}
	return &port.SCMProject{
		ID:            p.ID.String(),
		Name:          p.Name,
		FullPath:      p.PathWithNamespace,
		WebURL:        p.WebURL,
		HTTPCloneURL:  p.HTTPURLToRepo,
		RegistryURL:   registry,
		DefaultBranch: firstNonEmpty(p.DefaultBranch, defaultBranchName),
	}
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func parseID(v string) (int64, error) {
	n, err := json.Number(v).Int64()
	if err != nil {
		return 0, err
	}
	return n, nil
}

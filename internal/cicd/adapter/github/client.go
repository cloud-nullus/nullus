// Package github 은 GitHub REST API 로 리포지토리를 프로비저닝한다.
//
// GitLab 어댑터와 두 가지가 근본적으로 다르다.
//
// 하나, 네임스페이스를 만들 수 없다. GitHub Organization 은 API 로 생성되지
// 않으므로 EnsureGroup 은 "확인"만 한다 — 없으면 사람이 먼저 만들어야 한다.
//
// 둘, 토큰을 발급할 수 없다. GitLab 은 프로젝트 범위 토큰 API 가 있지만 GitHub
// 에는 리포 단위 토큰이 없다. 대신 워크플로가 내장 GITHUB_TOKEN 을 쓰고,
// 클러스터 쪽(Argo CD·kubelet)은 스택에 등록된 PAT 를 재사용한다.
package github

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
	// DefaultAPIBaseURL 는 github.com 의 API 주소다.
	// GitHub Enterprise Server 는 https://{host}/api/v3 형태를 쓴다.
	DefaultAPIBaseURL = "https://api.github.com"

	// apiVersion 은 REST API 버전 헤더 값이다.
	// 명시하지 않으면 GitHub 이 기본값을 고르므로 응답 형태가 예고 없이 바뀔 수 있다.
	apiVersion = "2022-11-28"

	// GHCRHost 는 GitHub Container Registry 의 호스트다.
	GHCRHost = "ghcr.io"

	defaultTimeout       = 30 * time.Second
	defaultBranchName    = "main"
	blobFileMode         = "100644"
	maxErrorBodyReadSize = 4 << 10
)

// Client 는 port.SCMProvisioner 와 port.PipelineConfigurator 의 GitHub 구현체다.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient 는 GitHub REST 클라이언트를 만든다.
//
// baseURL 이 비면 github.com 을 쓴다. token 은 repo·workflow 스코프를 가진 PAT 다.
func NewClient(baseURL, token string) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultAPIBaseURL
	}
	return &Client{
		baseURL:    base,
		token:      strings.TrimSpace(token),
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

// BaseURL 은 이 클라이언트가 바라보는 API 주소다.
func (c *Client) BaseURL() string { return c.baseURL }

// Ping 은 현재 토큰이 실제로 인증되는지 확인한다.
//
// 보관된 PAT 는 사용자가 폐기하거나 만료될 수 있다. 쓰기 전에 확인하지 않으면
// 이후 모든 호출이 401 로 죽고 원인이 프로비저닝 실패처럼 보인다.
func (c *Client) Ping(ctx context.Context) error {
	var out struct {
		Login string `json:"login"`
	}
	found, err := c.get(ctx, "/user", &out)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("github /user 엔드포인트를 찾을 수 없습니다")
	}
	return nil
}

// EnsureGroup 은 소유자(Organization 또는 사용자 계정)가 존재하는지 확인한다.
//
// 만들지 않는다 — GitHub Organization 은 API 로 생성할 수 없다. 없는 소유자를
// 조용히 넘기면 리포 생성이 엉뚱한 네임스페이스로 흘러가므로 여기서 끊는다.
func (c *Client) EnsureGroup(ctx context.Context, spec port.GroupSpec) (*port.SCMGroup, error) {
	owner := strings.TrimSpace(spec.Path)
	if owner == "" {
		return nil, fmt.Errorf("github owner(organization 또는 사용자)가 필요합니다")
	}

	var org ownerResponse
	found, err := c.get(ctx, "/orgs/"+url.PathEscape(owner), &org)
	if err != nil {
		return nil, fmt.Errorf("lookup github org %q: %w", owner, err)
	}
	if found {
		return org.toDomain(owner), nil
	}

	// org 가 아니면 개인 계정일 수 있다. 개인 계정도 리포를 담을 수 있으므로
	// 유효한 소유자로 취급한다.
	var user ownerResponse
	found, err = c.get(ctx, "/users/"+url.PathEscape(owner), &user)
	if err != nil {
		return nil, fmt.Errorf("lookup github user %q: %w", owner, err)
	}
	if found {
		return user.toDomain(owner), nil
	}

	return nil, fmt.Errorf(
		"github organization/사용자 %q 를 찾을 수 없습니다 — GitHub Organization 은 API 로 만들 수 없으니 "+
			"먼저 생성한 뒤 PAT 에 접근 권한이 있는지 확인하세요", owner)
}

// EnsureProject 는 리포지토리를 조회하고 없으면 만든다.
func (c *Client) EnsureProject(ctx context.Context, spec port.ProjectSpec) (*port.SCMProject, error) {
	repo := strings.TrimSpace(spec.Path)
	if repo == "" {
		return nil, fmt.Errorf("repository name is required")
	}
	owner := firstNonEmpty(strings.TrimSpace(spec.GroupPath), strings.TrimSpace(spec.GroupID))
	if owner == "" {
		return nil, fmt.Errorf("github owner is required (repo=%q)", repo)
	}

	var existing repoResponse
	found, err := c.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), &existing)
	if err != nil {
		return nil, fmt.Errorf("lookup repository %s/%s: %w", owner, repo, err)
	}
	if found {
		return existing.toDomain(), nil
	}

	body := map[string]any{
		"name": firstNonEmpty(spec.Name, repo),
		// 기본값은 private 다. 스캐폴딩에 배포 매니페스트가 함께 들어가므로
		// 실수로 public 이 되면 클러스터 구성이 그대로 노출된다.
		"private": !strings.EqualFold(strings.TrimSpace(spec.Visibility), "public"),
		// 브랜치가 없으면 트리 커밋의 base ref 를 찾지 못해 스캐폴딩이 실패한다.
		"auto_init": spec.InitReadme,
	}
	if spec.Description != "" {
		body["description"] = spec.Description
	}

	// 개인 계정에는 /orgs/{owner}/repos 가 없다. 소유자 종류에 따라 경로가 갈린다.
	endpoint := "/user/repos"
	if c.isOrganization(ctx, owner) {
		endpoint = "/orgs/" + url.PathEscape(owner) + "/repos"
	}

	var created repoResponse
	if err := c.send(ctx, http.MethodPost, endpoint, body, &created); err != nil {
		return nil, fmt.Errorf("create repository %s/%s: %w", owner, repo, err)
	}
	return created.toDomain(), nil
}

// isOrganization 은 소유자가 Organization 인지 본다.
//
// 조회에 실패하면 개인 계정으로 본다 — 그 경우 /user/repos 로 가는데, 토큰
// 소유자와 다르면 어차피 명확한 오류가 난다.
func (c *Client) isOrganization(ctx context.Context, owner string) bool {
	found, err := c.get(ctx, "/orgs/"+url.PathEscape(owner), nil)
	return err == nil && found
}

// CommitFiles 는 여러 파일을 한 커밋으로 올린다(upsert).
//
// Contents API 를 파일마다 부르면 커밋이 파일 수만큼 생기고, Argo CD 가 그
// 중간 상태를 하나씩 동기화해 배포가 여러 번 일어난다. Git Data API 로
// 트리 하나를 만들어 커밋 하나로 밀어야 한다.
func (c *Client) CommitFiles(ctx context.Context, projectID string, spec port.CommitSpec) error {
	repo := strings.Trim(strings.TrimSpace(projectID), "/")
	if repo == "" {
		return fmt.Errorf("repository (owner/name) is required")
	}
	if len(spec.Files) == 0 {
		return nil
	}
	branch := firstNonEmpty(spec.Branch, defaultBranchName)

	baseCommitSHA, baseTreeSHA, err := c.baseCommit(ctx, repo, branch)
	if err != nil {
		return err
	}

	entries := make([]map[string]any, 0, len(spec.Files))
	for _, f := range spec.Files {
		// content 를 그대로 넣으면 blob 을 따로 만들지 않아도 된다.
		// base_tree 위에 얹으므로 같은 경로는 자연스럽게 덮어써진다.
		entries = append(entries, map[string]any{
			"path":    f.Path,
			"mode":    blobFileMode,
			"type":    "blob",
			"content": f.Content,
		})
	}

	var tree struct {
		SHA string `json:"sha"`
	}
	if err := c.send(ctx, http.MethodPost, repoPath(repo, "/git/trees"), map[string]any{
		// base_tree 를 빼면 이 트리에 없는 기존 파일이 전부 삭제된다.
		"base_tree": baseTreeSHA,
		"tree":      entries,
	}, &tree); err != nil {
		return fmt.Errorf("create tree for %s: %w", repo, err)
	}

	var commit struct {
		SHA string `json:"sha"`
	}
	if err := c.send(ctx, http.MethodPost, repoPath(repo, "/git/commits"), map[string]any{
		"message": spec.Message,
		"tree":    tree.SHA,
		"parents": []string{baseCommitSHA},
	}, &commit); err != nil {
		return fmt.Errorf("create commit for %s: %w", repo, err)
	}

	refPath := repoPath(repo, "/git/refs/heads/"+url.PathEscape(branch))
	if err := c.send(ctx, http.MethodPatch, refPath, map[string]any{
		"sha": commit.SHA,
	}, nil); err != nil {
		return fmt.Errorf("update %s ref %s: %w", repo, branch, err)
	}
	return nil
}

// baseCommit 은 브랜치가 가리키는 커밋과 그 트리의 SHA 를 읽는다.
func (c *Client) baseCommit(ctx context.Context, repo, branch string) (commitSHA, treeSHA string, err error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	found, err := c.get(ctx, repoPath(repo, "/git/ref/heads/"+url.PathEscape(branch)), &ref)
	if err != nil {
		return "", "", fmt.Errorf("lookup %s branch %s: %w", repo, branch, err)
	}
	if !found || strings.TrimSpace(ref.Object.SHA) == "" {
		return "", "", fmt.Errorf(
			"%s 저장소에 %s 브랜치가 없습니다 — 리포를 auto_init 으로 만들어야 첫 커밋이 가능합니다",
			repo, branch)
	}

	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	found, err = c.get(ctx, repoPath(repo, "/git/commits/"+url.PathEscape(ref.Object.SHA)), &commit)
	if err != nil {
		return "", "", fmt.Errorf("lookup %s commit %s: %w", repo, ref.Object.SHA, err)
	}
	if !found {
		return "", "", fmt.Errorf("%s 의 커밋 %s 을 읽지 못했습니다", repo, ref.Object.SHA)
	}
	return ref.Object.SHA, commit.Tree.SHA, nil
}

// repoPath 는 owner/repo 를 세그먼트별로 이스케이프해 경로를 만든다.
//
// 통째로 PathEscape 하면 구분자 "/" 까지 %2F 가 되어 404 가 난다.
func repoPath(repo, suffix string) string {
	parts := strings.Split(repo, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/repos/" + strings.Join(parts, "/") + suffix
}

// --- HTTP 하부 ---

// get 은 200 이면 true, 404 면 false 를 돌려준다. 그 외는 오류다.
func (c *Client) get(ctx context.Context, path string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	c.applyHeaders(req)

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

func (c *Client) send(ctx context.Context, method, path string, body map[string]any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	c.applyHeaders(req)
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

func (c *Client) applyHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
}

type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("github api status=%d body=%s", e.StatusCode, e.Body)
}

func statusError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyReadSize)) // #nosec G104 -- best-effort
	return &apiError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
}

// --- 응답 매핑 ---

type ownerResponse struct {
	Login   string `json:"login"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

func (o ownerResponse) toDomain(fallbackLogin string) *port.SCMGroup {
	login := firstNonEmpty(o.Login, fallbackLogin)
	return &port.SCMGroup{
		// GitHub 은 숫자 ID 대신 경로로 API 를 친다. 하위 호출이 그대로 쓸 수
		// 있도록 login 을 ID 로 둔다.
		ID:       login,
		Name:     firstNonEmpty(o.Name, login),
		FullPath: login,
		WebURL:   o.HTMLURL,
	}
}

type repoResponse struct {
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
}

func (r repoResponse) toDomain() *port.SCMProject {
	return &port.SCMProject{
		// GitLab 의 숫자 ID 자리에 owner/repo 를 둔다 — GitHub API 의 리소스 키다.
		ID:           r.FullName,
		Name:         r.Name,
		FullPath:     r.FullName,
		WebURL:       r.HTMLURL,
		HTTPCloneURL: r.CloneURL,
		// GHCR 은 경로에 소문자만 받는다. 대문자가 섞인 org/리포 이름을 그대로
		// 쓰면 docker push 가 거부된다.
		RegistryURL:   GHCRHost + "/" + strings.ToLower(r.FullName),
		DefaultBranch: firstNonEmpty(r.DefaultBranch, defaultBranchName),
	}
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

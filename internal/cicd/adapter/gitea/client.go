// Package gitea 는 Gitea REST API 로 리포지토리를 프로비저닝한다.
//
// GitHub 어댑터와 형태가 비슷하지만 세 가지가 다르다.
//
// 하나, 조직을 만들 수 있다. GitHub Organization 은 API 로 생성되지 않아
// EnsureGroup 이 "확인"만 하지만, Gitea 는 실제로 만든다 — 스택 안에 설치되는
// Git 서버라 우리가 소유자다.
//
// 둘, 파일 커밋에 Git Data API 대신 배치 contents API 를 쓴다. Gitea 는
// POST /repos/{owner}/{repo}/contents 하나로 여러 파일을 한 커밋에 담을 수
// 있다. 스캐폴딩이 여러 커밋으로 쪼개지면 Argo CD 가 중간 상태를 동기화한다.
//
// 셋, 클러스터 안에 있다. 주소가 서비스 DNS 라 외부 노출이 필요 없다.
package gitea

import (
	"bytes"
	"context"
	"encoding/base64"
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
	// APIPathPrefix 는 Gitea REST API 의 경로 접두사다.
	APIPathPrefix = "/api/v1"

	defaultTimeout       = 30 * time.Second
	defaultBranchName    = "main"
	maxErrorBodyReadSize = 4 << 10
)

// Client 는 port.SCMProvisioner 의 Gitea 구현체다.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient 는 Gitea REST 클라이언트를 만든다.
//
// baseURL 은 API 접두사를 뺀 서버 주소다 (예: http://gitea-http.nullus.svc:3000).
// token 은 Gitea 액세스 토큰이다.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
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

// BaseURL 은 이 클라이언트가 바라보는 서버 주소다.
func (c *Client) BaseURL() string { return c.baseURL }

// Ping 은 현재 토큰이 실제로 인증되는지 확인한다.
//
// 보관된 토큰은 폐기·만료될 수 있다. 쓰기 전에 확인하지 않으면 이후 모든
// 호출이 401 로 죽고 원인이 프로비저닝 실패처럼 보인다.
func (c *Client) Ping(ctx context.Context) error {
	var out struct {
		Login string `json:"login"`
	}
	found, err := c.get(ctx, "/user", &out)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("gitea /user 엔드포인트를 찾을 수 없습니다 (주소: %s)", c.baseURL)
	}
	return nil
}

// EnsureGroup 은 조직을 조회하고 없으면 만든다.
//
// GitHub 과 달리 Gitea 는 조직을 API 로 만들 수 있다 — 스택 안에 설치되는
// Git 서버라 우리가 소유자다.
func (c *Client) EnsureGroup(ctx context.Context, spec port.GroupSpec) (*port.SCMGroup, error) {
	org := strings.TrimSpace(spec.Path)
	if org == "" {
		return nil, fmt.Errorf("gitea 조직 경로가 필요합니다")
	}

	var existing orgResponse
	found, err := c.get(ctx, "/orgs/"+url.PathEscape(org), &existing)
	if err != nil {
		return nil, fmt.Errorf("lookup gitea org %q: %w", org, err)
	}
	if found {
		return existing.toDomain(org, c.baseURL), nil
	}

	body := map[string]any{
		"username":  org,
		"full_name": firstNonEmpty(spec.Name, org),
		// 스캐폴딩에 배포 매니페스트가 함께 들어가므로 기본값을 private 로 둔다.
		"visibility": "private",
	}
	if spec.Description != "" {
		body["description"] = spec.Description
	}

	var created orgResponse
	if err := c.send(ctx, http.MethodPost, "/orgs", body, &created); err != nil {
		return nil, fmt.Errorf("create gitea org %q: %w", org, err)
	}
	return created.toDomain(org, c.baseURL), nil
}

// EnsureProject 는 리포지토리를 조회하고 없으면 만든다.
func (c *Client) EnsureProject(ctx context.Context, spec port.ProjectSpec) (*port.SCMProject, error) {
	repo := strings.TrimSpace(spec.Path)
	if repo == "" {
		return nil, fmt.Errorf("repository name is required")
	}
	owner := firstNonEmpty(strings.TrimSpace(spec.GroupPath), strings.TrimSpace(spec.GroupID))
	if owner == "" {
		return nil, fmt.Errorf("gitea owner is required (repo=%q)", repo)
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
		// 브랜치가 없으면 파일 커밋이 실패한다.
		"auto_init":      spec.InitReadme,
		"default_branch": defaultBranchName,
	}
	if spec.Description != "" {
		body["description"] = spec.Description
	}

	var created repoResponse
	if err := c.send(ctx, http.MethodPost, "/orgs/"+url.PathEscape(owner)+"/repos", body, &created); err != nil {
		return nil, fmt.Errorf("create repository %s/%s: %w", owner, repo, err)
	}
	project := created.toDomain()
	project.Created = true
	return project, nil
}

// CommitFiles 는 여러 파일을 한 커밋으로 올린다(upsert).
//
// 커밋이 여러 개로 쪼개지면 Argo CD 가 중간 상태를 동기화한다 — 매니페스트만
// 있고 이미지가 아직 없는 커밋을 배포하려다 실패한다. Gitea 의 배치 contents
// API 로 한 번에 밀어 넣는다.
//
// 파일마다 create / update 를 나눠야 한다. Gitea 는 기존 파일을 update 로
// 보내면서 그 파일의 sha 를 함께 요구한다 — create 로 보내면 409 가 난다.
func (c *Client) CommitFiles(ctx context.Context, projectID string, spec port.CommitSpec) error {
	repo := strings.Trim(strings.TrimSpace(projectID), "/")
	if repo == "" {
		return fmt.Errorf("repository (owner/name) is required")
	}
	if len(spec.Files) == 0 {
		return nil
	}
	branch := firstNonEmpty(spec.Branch, defaultBranchName)

	files := make([]map[string]any, 0, len(spec.Files))
	for _, f := range spec.Files {
		entry := map[string]any{
			"path": f.Path,
			// Gitea 의 contents API 는 base64 를 요구한다. 평문을 보내면
			// 파일이 깨진 채로 커밋된다.
			"content": base64.StdEncoding.EncodeToString([]byte(f.Content)),
		}
		sha, exists, err := c.fileSHA(ctx, repo, f.Path, branch)
		if err != nil {
			return err
		}
		if exists {
			entry["operation"] = "update"
			entry["sha"] = sha
		} else {
			entry["operation"] = "create"
		}
		files = append(files, entry)
	}

	body := map[string]any{
		"branch":  branch,
		"message": spec.Message,
		"files":   files,
	}
	if err := c.send(ctx, http.MethodPost, repoPath(repo, "/contents"), body, nil); err != nil {
		return fmt.Errorf("commit files to %s (%s): %w", repo, branch, err)
	}
	return nil
}

// fileSHA 는 브랜치에 있는 파일의 blob sha 를 읽는다. 없으면 exists=false 다.
func (c *Client) fileSHA(ctx context.Context, repo, path, branch string) (sha string, exists bool, err error) {
	var out struct {
		SHA string `json:"sha"`
	}
	endpoint := repoPath(repo, "/contents/"+escapePath(path)+"?ref="+url.QueryEscape(branch))
	found, err := c.get(ctx, endpoint, &out)
	if err != nil {
		return "", false, fmt.Errorf("lookup %s in %s: %w", path, repo, err)
	}
	if !found {
		return "", false, nil
	}
	return out.SHA, true, nil
}

// DeleteProject 는 리포지토리를 지운다.
//
// 이미 없으면 성공으로 본다 — 삭제의 목표는 "없는 상태" 이고, 404 를 오류로
// 올리면 앞선 시도가 절반쯤 성공한 뒤 재시도할 때 영영 끝나지 않는다.
func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	repo := strings.Trim(strings.TrimSpace(projectID), "/")
	if repo == "" {
		return fmt.Errorf("repository (owner/name) is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+APIPathPrefix+repoPath(repo, ""), nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete repository %s: %w", repo, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete repository %s: %s", repo, describeError(resp))
	}
	return nil
}

// EnsureWebhook 은 리포지토리에 webhook 을 걸어 둔다(멱등).
//
// Jenkins multibranch job 은 스스로 폴링하지 않는 한 새 커밋을 모른다.
// 폴링은 지연이 크고 리포가 늘수록 부하가 커지므로 push webhook 을 건다.
func (c *Client) EnsureWebhook(ctx context.Context, projectID, targetURL, secret string) error {
	repo := strings.Trim(strings.TrimSpace(projectID), "/")
	if repo == "" {
		return fmt.Errorf("repository (owner/name) is required")
	}
	if strings.TrimSpace(targetURL) == "" {
		return fmt.Errorf("webhook 대상 주소가 필요합니다 (repo=%s)", repo)
	}

	var existing []struct {
		ID     int64             `json:"id"`
		Config map[string]string `json:"config"`
	}
	if _, err := c.get(ctx, repoPath(repo, "/hooks"), &existing); err != nil {
		return fmt.Errorf("list webhooks for %s: %w", repo, err)
	}
	for _, hook := range existing {
		if strings.EqualFold(strings.TrimSpace(hook.Config["url"]), strings.TrimSpace(targetURL)) {
			return nil
		}
	}

	config := map[string]string{
		"url":          targetURL,
		"content_type": "json",
	}
	if strings.TrimSpace(secret) != "" {
		config["secret"] = secret
	}
	body := map[string]any{
		"type":   "gitea",
		"active": true,
		"events": []string{"push"},
		"config": config,
	}
	if err := c.send(ctx, http.MethodPost, repoPath(repo, "/hooks"), body, nil); err != nil {
		return fmt.Errorf("create webhook for %s: %w", repo, err)
	}
	return nil
}

func repoPath(repo, suffix string) string {
	parts := strings.Split(repo, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/repos/" + strings.Join(parts, "/") + suffix
}

// escapePath 는 파일 경로의 각 구간만 이스케이프한다.
// 경로 전체를 PathEscape 하면 디렉터리 구분자까지 %2F 가 되어 404 가 난다.
func escapePath(p string) string {
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}

func (c *Client) applyHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
}

// get 은 GET 요청을 보낸다. 404 는 오류가 아니라 found=false 다.
func (c *Client) get(ctx context.Context, path string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+APIPathPrefix+path, nil)
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
		return false, fmt.Errorf("gitea GET %s: %s", path, describeError(resp))
	}
	if out == nil {
		return true, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return false, fmt.Errorf("decode gitea response for %s: %w", path, err)
	}
	return true, nil
}

func (c *Client) send(ctx context.Context, method, path string, body map[string]any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+APIPathPrefix+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("gitea %s %s: %s", method, path, describeError(resp))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode gitea response for %s: %w", path, err)
	}
	return nil
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

type orgResponse struct {
	Username    string `json:"username"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
}

func (o orgResponse) toDomain(fallback, baseURL string) *port.SCMGroup {
	name := firstNonEmpty(o.Username, fallback)
	return &port.SCMGroup{
		ID:       name,
		Name:     firstNonEmpty(o.FullName, name),
		FullPath: name,
		WebURL:   strings.TrimRight(baseURL, "/") + "/" + name,
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
		// Gitea 는 owner/name 이 곧 리포 식별자다. 숫자 ID 도 있지만 경로 기반
		// 호출이 훨씬 많으므로 GitHub 어댑터와 같은 규약을 쓴다.
		ID:            r.FullName,
		Name:          r.Name,
		FullPath:      r.FullName,
		WebURL:        r.HTMLURL,
		HTTPCloneURL:  r.CloneURL,
		DefaultBranch: firstNonEmpty(r.DefaultBranch, defaultBranchName),
	}
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

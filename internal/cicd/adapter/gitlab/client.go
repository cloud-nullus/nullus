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

// Client implements port.GitLabProvisioner using the GitLab REST API v4.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// EnsureProject creates the project if it doesn't exist, or returns the existing one.
func (c *Client) EnsureProject(ctx context.Context, input port.GitLabProvisionInput) (*port.GitLabProvisionResult, error) {
	// Check if project already exists
	existing, err := c.findProject(ctx, input.Endpoint, input.Token, input.ProjectPath)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Create new project
	name := input.ProjectPath
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	body, _ := json.Marshal(map[string]any{
		"name":                name,
		"path":                name,
		"visibility":          "private",
		"initialize_with_readme": true,
	})

	resp, err := c.do(ctx, http.MethodPost, input.Endpoint+"/api/v4/projects", input.Token, body)
	if err != nil {
		return nil, fmt.Errorf("create gitlab project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create gitlab project: status %d: %s", resp.StatusCode, raw)
	}

	return c.parseProjectResponse(resp.Body)
}

// CommitCIConfig commits a .gitlab-ci.yml to the default branch.
func (c *Client) CommitCIConfig(ctx context.Context, endpoint, token string, projectID int, ciEnvVars map[string]string) error {
	ciYAML := buildGitLabCIYAML(ciEnvVars)

	action := map[string]any{
		"action":    "create",
		"file_path": ".gitlab-ci.yml",
		"content":   ciYAML,
	}

	// Try update if create fails (file may already exist)
	payload := map[string]any{
		"branch":         "main",
		"commit_message": "chore: add nullus CI pipeline",
		"actions":        []any{action},
	}
	body, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/commits", endpoint, projectID)
	resp, err := c.do(ctx, http.MethodPost, apiURL, token, body)
	if err != nil {
		return fmt.Errorf("commit ci config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}

	// File may already exist — try with "update" action
	action["action"] = "update"
	body, _ = json.Marshal(payload)
	resp2, err := c.do(ctx, http.MethodPost, apiURL, token, body)
	if err != nil {
		return fmt.Errorf("update ci config: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusCreated || resp2.StatusCode == http.StatusOK {
		return nil
	}

	raw, _ := io.ReadAll(resp2.Body)
	return fmt.Errorf("commit ci config: status %d: %s", resp2.StatusCode, raw)
}

func (c *Client) findProject(ctx context.Context, endpoint, token, projectPath string) (*port.GitLabProvisionResult, error) {
	encoded := url.PathEscape(projectPath)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s", endpoint, encoded)

	resp, err := c.do(ctx, http.MethodGet, apiURL, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return c.parseProjectResponse(resp.Body)
}

type gitlabProjectResponse struct {
	ID                int    `json:"id"`
	WebURL            string `json:"web_url"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
}

func (c *Client) parseProjectResponse(body io.Reader) (*port.GitLabProvisionResult, error) {
	var p gitlabProjectResponse
	if err := json.NewDecoder(body).Decode(&p); err != nil {
		return nil, fmt.Errorf("parse project response: %w", err)
	}
	return &port.GitLabProvisionResult{
		ProjectID:   p.ID,
		ProjectURL:  p.WebURL,
		HTTPURL:     p.HTTPURLToRepo,
		SSHCloneURL: p.SSHURLToRepo,
	}, nil
}

func (c *Client) do(ctx context.Context, method, rawURL, token string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// buildGitLabCIYAML generates a .gitlab-ci.yml that builds and pushes a Docker image.
func buildGitLabCIYAML(extraEnvVars map[string]string) string {
	envLines := ""
	for k, v := range extraEnvVars {
		envLines += fmt.Sprintf("  %s: %q\n", k, v)
	}

	return fmt.Sprintf(`stages:
  - build
  - deploy

variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA
%s
build:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  before_script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
  script:
    - docker build -t $IMAGE_TAG .
    - docker push $IMAGE_TAG
  only:
    - main

update-manifest:
  stage: deploy
  image: alpine/git:latest
  script:
    - git config --global user.email "nullus-ci@nullus.io"
    - git config --global user.name "Nullus CI"
    - |
      if [ -n "$ENV_REPO_URL" ]; then
        git clone https://oauth2:$GITLAB_TOKEN@$(echo $ENV_REPO_URL | sed 's|https://||') /tmp/env-repo
        cd /tmp/env-repo
        sed -i "s|image:.*|image: $IMAGE_TAG|g" deployment.yaml || true
        git add -A && git commit -m "chore: update image to $IMAGE_TAG" && git push || true
      fi
  only:
    - main
`, envLines)
}

package argocd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// Client implements port.ArgoCDProvisioner using the ArgoCD REST API.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// EnsureApplication creates or updates an ArgoCD Application.
func (c *Client) EnsureApplication(ctx context.Context, input port.ArgoCDProvisionInput) (*port.ArgoCDProvisionResult, error) {
	app := buildAppManifest(input)
	body, _ := json.Marshal(app)

	// Try to get existing app first
	getURL := fmt.Sprintf("%s/api/v1/applications/%s", input.Endpoint, input.AppName)
	resp, err := c.do(ctx, http.MethodGet, getURL, input.Token, nil)
	if err != nil {
		return nil, fmt.Errorf("check argocd app: %w", err)
	}
	resp.Body.Close()

	var apiURL, method string
	if resp.StatusCode == http.StatusOK {
		// Update existing
		apiURL = getURL
		method = http.MethodPut
	} else {
		// Create new
		apiURL = fmt.Sprintf("%s/api/v1/applications", input.Endpoint)
		method = http.MethodPost
	}

	resp2, err := c.do(ctx, method, apiURL, input.Token, body)
	if err != nil {
		return nil, fmt.Errorf("upsert argocd app: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp2.Body)
		return nil, fmt.Errorf("argocd app upsert: status %d: %s", resp2.StatusCode, raw)
	}

	return &port.ArgoCDProvisionResult{
		AppName:   input.AppName,
		ServerURL: input.Endpoint,
		SyncURL:   fmt.Sprintf("%s/applications/%s", input.Endpoint, input.AppName),
	}, nil
}

// TriggerSync forces an immediate sync on the named Application.
func (c *Client) TriggerSync(ctx context.Context, endpoint, token, appName string) error {
	apiURL := fmt.Sprintf("%s/api/v1/applications/%s/sync", endpoint, appName)
	body, _ := json.Marshal(map[string]any{"prune": false, "dryRun": false})

	resp, err := c.do(ctx, http.MethodPost, apiURL, token, body)
	if err != nil {
		return fmt.Errorf("trigger argocd sync: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("argocd sync: status %d: %s", resp.StatusCode, raw)
	}
	return nil
}

// GetSyncStatus returns the current sync/health status of the Application.
func (c *Client) GetSyncStatus(ctx context.Context, endpoint, token, appName string) (syncStatus, healthStatus string, err error) {
	apiURL := fmt.Sprintf("%s/api/v1/applications/%s", endpoint, appName)

	resp, err := c.do(ctx, http.MethodGet, apiURL, token, nil)
	if err != nil {
		return "", "", fmt.Errorf("get argocd app status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("argocd get app: status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Status struct {
			Sync   struct{ Status string `json:"status"` } `json:"sync"`
			Health struct{ Status string `json:"status"` } `json:"health"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("parse argocd status: %w", err)
	}

	return result.Status.Sync.Status, result.Status.Health.Status, nil
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
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func buildAppManifest(input port.ArgoCDProvisionInput) map[string]any {
	clusterURL := input.ClusterURL
	if clusterURL == "" {
		clusterURL = "https://kubernetes.default.svc"
	}
	targetRevision := input.TargetRevision
	if targetRevision == "" {
		targetRevision = "HEAD"
	}
	repoPath := input.RepoPath
	if repoPath == "" {
		repoPath = "."
	}

	return map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      input.AppName,
			"namespace": "argocd",
		},
		"spec": map[string]any{
			"project": "default",
			"source": map[string]any{
				"repoURL":        input.RepoURL,
				"path":           repoPath,
				"targetRevision": targetRevision,
			},
			"destination": map[string]any{
				"server":    clusterURL,
				"namespace": input.Namespace,
			},
			"syncPolicy": map[string]any{
				"automated": map[string]any{
					"prune":    true,
					"selfHeal": true,
				},
				"syncOptions": []string{"CreateNamespace=true"},
			},
		},
	}
}

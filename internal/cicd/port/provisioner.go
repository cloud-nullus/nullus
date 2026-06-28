package port

import "context"

// GitLabProvisionInput holds what's needed to provision a GitLab project for a pipeline.
type GitLabProvisionInput struct {
	Endpoint    string
	Token       string
	ProjectPath string // "group/app-name" or just "app-name"
	PipelineName string
	GitRepoURL  string
	// EnvVars to inject into .gitlab-ci.yml (ARGOCD_URL, ARGOCD_TOKEN, etc.)
	CIEnvVars map[string]string
}

// GitLabProvisionResult is the output of provisioning a GitLab project.
type GitLabProvisionResult struct {
	ProjectID   int
	ProjectURL  string
	HTTPURL     string
	SSHCloneURL string
	WebhookID   int
}

// GitLabProvisioner provisions GitLab projects and CI pipelines.
type GitLabProvisioner interface {
	// EnsureProject creates the project if it doesn't exist, returns existing otherwise.
	EnsureProject(ctx context.Context, input GitLabProvisionInput) (*GitLabProvisionResult, error)
	// CommitCIConfig commits a .gitlab-ci.yml that builds & pushes an image to the GitLab registry.
	CommitCIConfig(ctx context.Context, endpoint, token string, projectID int, ciEnvVars map[string]string) error
}

// ArgoCDProvisionInput holds what's needed to create an ArgoCD Application.
type ArgoCDProvisionInput struct {
	Endpoint    string
	Token       string
	AppName     string
	RepoURL     string // Git source repo for manifests
	RepoPath    string // path inside repo (e.g. "k8s/")
	TargetRevision string
	Namespace   string // destination namespace in the cluster
	ClusterURL  string // destination cluster API URL ("https://kubernetes.default.svc" for in-cluster)
}

// ArgoCDProvisionResult is the output of creating an ArgoCD Application.
type ArgoCDProvisionResult struct {
	AppName   string
	ServerURL string
	SyncURL   string
}

// ArgoCDProvisioner provisions ArgoCD Applications and triggers syncs.
type ArgoCDProvisioner interface {
	// EnsureApplication creates the ArgoCD Application if it doesn't exist.
	EnsureApplication(ctx context.Context, input ArgoCDProvisionInput) (*ArgoCDProvisionResult, error)
	// TriggerSync forces an immediate sync on the named Application.
	TriggerSync(ctx context.Context, endpoint, token, appName string) error
	// GetSyncStatus returns the current sync/health status of the Application.
	GetSyncStatus(ctx context.Context, endpoint, token, appName string) (syncStatus, healthStatus string, err error)
}

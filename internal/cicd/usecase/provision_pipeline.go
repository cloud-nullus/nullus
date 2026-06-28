package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ProvisionPipelineInput holds parameters for provisioning a stack_integrated pipeline.
type ProvisionPipelineInput struct {
	PipelineID string
	// EnvRepoURL is the GitLab repository used as the GitOps environment repo.
	// If empty, the same source repo is used.
	EnvRepoURL string
	// EnvRepoPath is the path inside the env repo where k8s manifests live.
	EnvRepoPath string
}

// ProvisionPipelineOutput describes what was provisioned.
type ProvisionPipelineOutput struct {
	GitLabProjectURL string
	ArgoCDAppName    string
	ArgoCDSyncURL    string
}

// ProvisionPipeline sets up GitLab project + CI config and creates an ArgoCD Application.
type ProvisionPipeline struct {
	pipelineRepo    port.PipelineRepository
	integrationReader port.StackIntegrationReader
	gitlabClient    port.GitLabProvisioner
	argocdClient    port.ArgoCDProvisioner
}

func NewProvisionPipeline(
	pipelineRepo port.PipelineRepository,
	integrationReader port.StackIntegrationReader,
	gitlabClient port.GitLabProvisioner,
	argocdClient port.ArgoCDProvisioner,
) *ProvisionPipeline {
	return &ProvisionPipeline{
		pipelineRepo:      pipelineRepo,
		integrationReader: integrationReader,
		gitlabClient:      gitlabClient,
		argocdClient:      argocdClient,
	}
}

// Execute provisions GitLab + ArgoCD for a stack_integrated pipeline.
func (uc *ProvisionPipeline) Execute(ctx context.Context, input ProvisionPipelineInput) (*ProvisionPipelineOutput, error) {
	pipeline, err := uc.pipelineRepo.GetByID(ctx, input.PipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline not found: %w", err)
	}

	if pipeline.ExecutionMode != "stack_integrated" {
		return nil, fmt.Errorf("provision is only available for stack_integrated pipelines (this pipeline is %q)", pipeline.ExecutionMode)
	}
	if pipeline.StackID == "" {
		return nil, fmt.Errorf("pipeline has no stack_id — cannot provision")
	}

	profile, err := uc.integrationReader.GetStackIntegrationProfile(ctx, pipeline.StackID)
	if err != nil {
		return nil, fmt.Errorf("read stack integrations: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("stack %q not found", pipeline.StackID)
	}

	gitlabIntegration := profile.ByType("code_repository")
	if gitlabIntegration == nil || gitlabIntegration.Endpoint == "" {
		return nil, fmt.Errorf("stack %q has no code_repository integration", pipeline.StackID)
	}
	if gitlabIntegration.Token == "" {
		return nil, fmt.Errorf("stack %q: gitlab_token not configured in stack credentials", pipeline.StackID)
	}

	argocdIntegration := profile.ByType("cd_tool")
	if argocdIntegration == nil || argocdIntegration.Endpoint == "" {
		return nil, fmt.Errorf("stack %q has no cd_tool integration", pipeline.StackID)
	}
	if argocdIntegration.Token == "" {
		return nil, fmt.Errorf("stack %q: argocd_token not configured in stack credentials", pipeline.StackID)
	}

	// 1. Ensure GitLab project exists
	projectPath := projectPathFor(pipeline.Name)
	gitlabResult, err := uc.gitlabClient.EnsureProject(ctx, port.GitLabProvisionInput{
		Endpoint:     gitlabIntegration.Endpoint,
		Token:        gitlabIntegration.Token,
		ProjectPath:  projectPath,
		PipelineName: pipeline.Name,
		GitRepoURL:   pipeline.GitRepoURL,
		CIEnvVars:    ciEnvVars(pipeline.EnvVars, argocdIntegration, input),
	})
	if err != nil {
		return nil, fmt.Errorf("provision gitlab project: %w", err)
	}

	// 2. Commit .gitlab-ci.yml
	ciVars := ciEnvVars(pipeline.EnvVars, argocdIntegration, input)
	if err := uc.gitlabClient.CommitCIConfig(ctx, gitlabIntegration.Endpoint, gitlabIntegration.Token, gitlabResult.ProjectID, ciVars); err != nil {
		return nil, fmt.Errorf("commit ci config: %w", err)
	}

	// 3. Create/update ArgoCD Application
	envRepoURL := input.EnvRepoURL
	if envRepoURL == "" {
		envRepoURL = gitlabResult.HTTPURL
	}
	appName := pipeline.Name

	argoResult, err := uc.argocdClient.EnsureApplication(ctx, port.ArgoCDProvisionInput{
		Endpoint:       argocdIntegration.Endpoint,
		Token:          argocdIntegration.Token,
		AppName:        appName,
		RepoURL:        envRepoURL,
		RepoPath:       input.EnvRepoPath,
		TargetRevision: "main",
		Namespace:      pipeline.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("provision argocd app: %w", err)
	}

	return &ProvisionPipelineOutput{
		GitLabProjectURL: gitlabResult.ProjectURL,
		ArgoCDAppName:    argoResult.AppName,
		ArgoCDSyncURL:    argoResult.SyncURL,
	}, nil
}

func projectPathFor(pipelineName string) string {
	return strings.ToLower(strings.ReplaceAll(pipelineName, " ", "-"))
}

func ciEnvVars(pipelineEnvVars map[string]string, argocd *port.StackIntegration, input ProvisionPipelineInput) map[string]string {
	vars := map[string]string{
		"ARGOCD_URL":   argocd.Endpoint,
		"ARGOCD_TOKEN": argocd.Token,
	}
	if input.EnvRepoURL != "" {
		vars["ENV_REPO_URL"] = input.EnvRepoURL
	}
	for k, v := range pipelineEnvVars {
		vars[k] = v
	}
	return vars
}

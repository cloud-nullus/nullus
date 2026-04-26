package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type Builder struct {
	tracker *kube.StepTracker
}

func NewBuilder(tracker *kube.StepTracker) *Builder {
	return &Builder{tracker: tracker}
}

func (b *Builder) PrepareImage(ctx context.Context, opts port.PrepareImageOpts) (string, error) {
	tmpDir, err := os.MkdirTemp("", "nullus-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	b.markRunning(opts.DeploymentID, 0)
	b.log(opts.DeploymentID, 0, "$ git clone --depth=1 %s", opts.GitRepoURL)
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", opts.GitRepoURL, tmpDir)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		b.log(opts.DeploymentID, 0, "error: %s", strings.TrimSpace(string(output)))
		b.markFailed(opts.DeploymentID, 0, "git clone failed")
		return "", fmt.Errorf("git clone: %w", err)
	}
	b.markSuccess(opts.DeploymentID, 0, "Cloned successfully")

	dockerfilePath := filepath.Join(tmpDir, opts.DockerfilePath)
	buildContext := filepath.Join(tmpDir, opts.DockerContext)

	b.markRunning(opts.DeploymentID, 1)
	b.log(opts.DeploymentID, 1, "$ docker build -t %s -f %s %s", opts.ImageName, opts.DockerfilePath, opts.DockerContext)
	buildCmd := exec.CommandContext(ctx, "docker", "build",
		"-t", opts.ImageName,
		"-f", dockerfilePath,
		buildContext,
	)
	if output, err := buildCmd.CombinedOutput(); err != nil {
		b.log(opts.DeploymentID, 1, "error: %s", strings.TrimSpace(string(output)))
		b.markFailed(opts.DeploymentID, 1, "docker build failed")
		return "", fmt.Errorf("docker build: %w", err)
	}
	b.markSuccess(opts.DeploymentID, 1, fmt.Sprintf("Built %s", opts.ImageName))

	b.markRunning(opts.DeploymentID, 2)
	b.log(opts.DeploymentID, 2, "$ kind load docker-image %s --name %s", opts.ImageName, opts.ClusterName)
	loadCmd := exec.CommandContext(ctx, "kind", "load", "docker-image", opts.ImageName, "--name", opts.ClusterName)
	if output, err := loadCmd.CombinedOutput(); err != nil {
		b.log(opts.DeploymentID, 2, "error: %s", strings.TrimSpace(string(output)))
		b.markFailed(opts.DeploymentID, 2, "kind load failed")
		return "", fmt.Errorf("kind load: %w", err)
	}
	b.markSuccess(opts.DeploymentID, 2, fmt.Sprintf("Loaded into kind-%s", opts.ClusterName))

	return opts.ImageName, nil
}

func (b *Builder) log(deploymentID string, stepIndex int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Info("build", "step", stepIndex, "msg", msg)
	if b.tracker != nil && deploymentID != "" {
		b.tracker.AppendLog(deploymentID, stepIndex, msg)
	}
}

func (b *Builder) markRunning(deploymentID string, stepIndex int) {
	if b.tracker != nil && deploymentID != "" {
		b.tracker.MarkRunning(deploymentID, stepIndex, "")
	}
}

func (b *Builder) markSuccess(deploymentID string, stepIndex int, message string) {
	if b.tracker != nil && deploymentID != "" {
		b.tracker.MarkSuccess(deploymentID, stepIndex, message)
	}
}

func (b *Builder) markFailed(deploymentID string, stepIndex int, message string) {
	if b.tracker != nil && deploymentID != "" {
		b.tracker.MarkFailed(deploymentID, stepIndex, message)
	}
}

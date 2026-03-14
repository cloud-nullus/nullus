package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipeline_InitialStatus(t *testing.T) {
	p := &Pipeline{
		ID:        "pip_abc123",
		Name:      "my-pipeline",
		OrgID:     "org_001",
		ClusterID: "cls_001",
		AppType:   AppTypeBackend,
		Status:    PipelineStatusActive,
		CreatedAt: time.Now(),
	}

	assert.Equal(t, PipelineStatusActive, p.Status)
	assert.Equal(t, AppTypeBackend, p.AppType)
}

func TestDeployment_StatusTransitions(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Second)

	d := &Deployment{
		ID:          "dep_abc123",
		PipelineID:  "pip_abc123",
		Version:     "v1.0.0",
		Status:      DeploymentStatusPending,
		StartedAt:   now,
		DeployedBy:  "user_001",
	}

	assert.Equal(t, DeploymentStatusPending, d.Status)

	d.Status = DeploymentStatusRunning
	assert.Equal(t, DeploymentStatusRunning, d.Status)

	d.Status = DeploymentStatusSuccess
	d.CompletedAt = &completed
	assert.Equal(t, DeploymentStatusSuccess, d.Status)
	assert.NotNil(t, d.CompletedAt)
}

func TestDeployment_FailedStatus(t *testing.T) {
	d := &Deployment{
		ID:         "dep_xyz999",
		PipelineID: "pip_abc123",
		Version:    "v1.0.1",
		Status:     DeploymentStatusFailed,
		StartedAt:  time.Now(),
		DeployedBy: "user_002",
	}

	assert.Equal(t, DeploymentStatusFailed, d.Status)
	assert.Nil(t, d.CompletedAt)
}

func TestPipelineTemplate_AppTypes(t *testing.T) {
	templates := []PipelineTemplate{
		{ID: "web-backend-v1", AppType: AppTypeBackend, Stages: []string{"Build", "Test", "ImageBuild", "Deploy"}},
		{ID: "web-frontend-v1", AppType: AppTypeWeb, Stages: []string{"Build", "Test", "StaticBuild", "Deploy"}},
		{ID: "batch-job-v1", AppType: AppTypeBatch, Stages: []string{"Build", "ImageBuild", "CronJobDeploy"}},
	}

	assert.Equal(t, AppTypeBackend, templates[0].AppType)
	assert.Equal(t, AppTypeWeb, templates[1].AppType)
	assert.Equal(t, AppTypeBatch, templates[2].AppType)
	assert.Len(t, templates[0].Stages, 4)
	assert.Len(t, templates[2].Stages, 3)
}

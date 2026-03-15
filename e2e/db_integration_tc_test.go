//go:build integration

package e2e

import (
	"context"
	"testing"
	"time"

	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	cicddomain "github.com/cloud-nullus/draft/internal/cicd/domain"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	obsdomain "github.com/cloud-nullus/draft/internal/observability/domain"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBIntegration_PostgresRepositories(t *testing.T) {
	t.Parallel()

	pool, cleanup := SetupPostgres(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	now := time.Now().UTC()
	suffix := now.Format("20060102150405")

	orgRepo := adminrepo.NewPostgresOrgRepository(pool)
	stackRepo := stackrepo.NewPostgresStackRepository(pool)
	pipelineRepo := cicdrepo.NewPostgresPipelineRepository(pool)
	deploymentRepo := cicdrepo.NewPostgresDeploymentRepository(pool)
	alertRuleRepo := obsrepo.NewPostgresAlertRuleRepository(pool)
	templateRepo := stackrepo.NewPostgresTemplateRepository(pool)

	t.Run("create organization and get by id", func(t *testing.T) {
		orgID := uuid.NewString()
		org := &admindomain.Organization{
			ID:        orgID,
			Name:      "Integration Org",
			Slug:      "integration-org-" + suffix,
			Domain:    "integration.test",
			Status:    admindomain.OrgStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}

		require.NoError(t, orgRepo.Create(ctx, org))

		got, err := orgRepo.GetByID(ctx, org.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, org.ID, got.ID)
		assert.Equal(t, org.Name, got.Name)
		assert.Equal(t, org.Slug, got.Slug)
	})

	t.Run("create stack and list by organization", func(t *testing.T) {
		org := &admindomain.Organization{
			ID:        uuid.NewString(),
			Name:      "Stack Org",
			Slug:      "stack-org-" + suffix,
			Domain:    "stack.integration.test",
			Status:    admindomain.OrgStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, orgRepo.Create(ctx, org))

		var clusterID string
		err := pool.QueryRow(ctx,
			`INSERT INTO clusters (name, type, endpoint, connection_status, org_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			"cluster-"+suffix,
			"pipeline",
			"https://k8s.integration.test",
			"connected",
			org.ID,
		).Scan(&clusterID)
		require.NoError(t, err)

		stack := &stackdomain.Stack{
			ID:         "stack-" + suffix,
			Name:       "Integration Stack",
			TemplateID: "gitlab-allinone-v1",
			OrgID:      org.ID,
			ClusterID:  clusterID,
			State:      stackdomain.StatePending,
			Config:     stackdomain.StackConfig{},
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		require.NoError(t, stackRepo.Create(ctx, stack))

		stacks, err := stackRepo.List(ctx, stack.OrgID)
		require.NoError(t, err)
		require.NotEmpty(t, stacks)
		assert.True(t, containsStack(stacks, stack.ID))
	})

	t.Run("create pipeline deploy and list deployments", func(t *testing.T) {
		pipeline := &cicddomain.Pipeline{
			ID:         "pipeline-" + suffix,
			Name:       "Integration Pipeline",
			TemplateID: "web-backend-v1",
			OrgID:      "org-" + suffix,
			ClusterID:  "cluster-" + suffix,
			Namespace:  "default",
			AppType:    cicddomain.AppTypeBackend,
			GitRepoURL: "https://github.com/cloud-nullus/draft",
			Status:     cicddomain.PipelineStatusActive,
			CreatedAt:  now,
		}

		require.NoError(t, pipelineRepo.Create(ctx, pipeline))

		deployment := &cicddomain.Deployment{
			ID:         "deployment-" + suffix,
			PipelineID: pipeline.ID,
			Version:    "v1.0.0",
			Status:     cicddomain.DeploymentStatusSuccess,
			StartedAt:  now,
			DeployedBy: "integration-test",
		}

		require.NoError(t, deploymentRepo.Create(ctx, deployment))

		deployments, err := deploymentRepo.ListByPipelineID(ctx, pipeline.ID)
		require.NoError(t, err)
		require.NotEmpty(t, deployments)
		assert.Equal(t, deployment.ID, deployments[0].ID)
	})

	t.Run("create alert rule and list", func(t *testing.T) {
		rule := &obsdomain.AlertRule{
			ID:        "rule-" + suffix,
			Name:      "CPU High",
			Condition: "cpu_usage > threshold",
			Threshold: 80,
			Channel:   obsdomain.AlertChannelSlack,
			Enabled:   true,
		}

		require.NoError(t, alertRuleRepo.Create(ctx, rule))

		rules, err := alertRuleRepo.List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, rules)
		assert.True(t, containsAlertRule(rules, rule.ID))
	})

	t.Run("template seed data exists after migrations", func(t *testing.T) {
		templates, err := templateRepo.List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, templates)

		ids := make([]string, 0, len(templates))
		for _, tmpl := range templates {
			ids = append(ids, tmpl.ID)
		}
		assert.Contains(t, ids, "gitlab-allinone-v1")
	})
}

func containsStack(stacks []*stackdomain.Stack, id string) bool {
	for _, stack := range stacks {
		if stack.ID == id {
			return true
		}
	}
	return false
}

func containsAlertRule(rules []*obsdomain.AlertRule, id string) bool {
	for _, rule := range rules {
		if rule.ID == id {
			return true
		}
	}
	return false
}

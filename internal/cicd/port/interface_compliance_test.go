package port_test

import (
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

var _ port.PipelineRepository = (*cicdrepo.PostgresPipelineRepository)(nil)
var _ port.PipelineRepository = (*cicdrepo.MemoryPipelineRepository)(nil)

var _ port.PipelineTemplateRepository = (*cicdrepo.PostgresCICDTemplateRepository)(nil)
var _ port.PipelineTemplateRepository = (*cicdrepo.MemoryCICDTemplateRepository)(nil)

var _ port.DeploymentRepository = (*cicdrepo.PostgresDeploymentRepository)(nil)
var _ port.DeploymentRepository = (*cicdrepo.MemoryDeploymentRepository)(nil)

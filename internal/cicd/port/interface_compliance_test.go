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

var _ port.StackReader = (*cicdrepo.PostgresStackReader)(nil)

var _ port.DependencyScanResultRepository = (*cicdrepo.PostgresDependencyScanResultRepository)(nil)
var _ port.DependencyScanResultRepository = (*cicdrepo.MemoryDependencyScanResultRepository)(nil)

var _ port.SecretScanResultRepository = (*cicdrepo.PostgresSecretScanResultRepository)(nil)
var _ port.SecretScanResultRepository = (*cicdrepo.MemorySecretScanResultRepository)(nil)

var _ port.PipelineSecurityReader = (*cicdrepo.PostgresPipelineSecurityReader)(nil)

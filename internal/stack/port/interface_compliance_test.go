package port_test

import (
	helmadapter "github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

var _ port.StackRepository = (*stackrepo.PostgresStackRepository)(nil)
var _ port.StackRepository = (*stackrepo.MemoryStackRepository)(nil)

var _ port.TemplateRepository = (*stackrepo.PostgresTemplateRepository)(nil)
var _ port.TemplateRepository = (*stackrepo.MemoryTemplateRepository)(nil)

var _ port.CompatibilityRepository = (*stackrepo.PostgresCompatibilityRepository)(nil)
var _ port.CompatibilityRepository = (*stackrepo.MemoryCompatibilityRepository)(nil)

var _ port.HistoryRepository = (*stackrepo.PostgresHistoryRepository)(nil)
var _ port.HistoryRepository = (*stackrepo.MemoryHistoryRepository)(nil)

var _ port.LogStreamer = (*logadapter.MemoryStreamer)(nil)

var _ port.ResourceDefaultRepository = (*stackrepo.PostgresResourceDefaultRepository)(nil)
var _ port.ResourceDefaultRepository = (*stackrepo.MemoryResourceDefaultRepository)(nil)

var _ port.HelmStepMetadataRepository = (*stackrepo.PostgresHelmStepMetadataRepository)(nil)
var _ port.HelmStepMetadataRepository = (*stackrepo.MemoryHelmStepMetadataRepository)(nil)

var _ port.StepExecutor = (*helmadapter.Orchestrator)(nil)
var _ port.KubeconfigProvider = (*stackrepo.PostgresKubeconfigProvider)(nil)
var _ port.HelmInstaller = (*helmadapter.HelmInstaller)(nil)
var _ port.TokenSourceRegistry = (*stackrepo.PostgresTokenSourceRegistry)(nil)

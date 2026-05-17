package port_test

import (
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/port"
)

var _ port.OrgRepository = (*adminrepo.PostgresOrgRepository)(nil)
var _ port.OrgRepository = (*adminrepo.MemoryOrgRepository)(nil)
var _ port.ResourceProfileRepository = (*adminrepo.PostgresResourceProfileRepository)(nil)

var _ port.ClusterRepository = (*adminrepo.PostgresClusterRepository)(nil)
var _ port.ClusterRepository = (*adminrepo.MemoryClusterRepository)(nil)

var _ port.UserRepository = (*adminrepo.PostgresUserRepository)(nil)
var _ port.TokenSourceRepository = (*adminrepo.PostgresTokenSourceRepository)(nil)

var _ port.KnownIssuesRepository = (*adminrepo.PostgresKnownIssuesRepository)(nil)

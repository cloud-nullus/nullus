package port_test

import (
	promadapter "github.com/cloud-nullus/draft/internal/observability/adapter/prometheus"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

var _ port.DashboardRepository = (*obsrepo.MemoryDashboardRepository)(nil)
var _ port.DashboardRepository = (*promadapter.DashboardRepository)(nil)

var _ port.AlertRuleRepository = (*obsrepo.PostgresAlertRuleRepository)(nil)
var _ port.AlertRuleRepository = (*obsrepo.MemoryAlertRuleRepository)(nil)

var _ port.AlertRepository = (*obsrepo.PostgresAlertRepository)(nil)
var _ port.AlertRepository = (*obsrepo.MemoryAlertRepository)(nil)

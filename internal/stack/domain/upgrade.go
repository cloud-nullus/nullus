package domain

import "time"

const (
	UpgradeStatusPreflighting = "preflighting"
	UpgradeStatusBlocked      = "blocked"
	UpgradeStatusUpgrading    = "upgrading"
	UpgradeStatusSucceeded    = "succeeded"
	UpgradeStatusRollingBack  = "rolling_back"
	UpgradeStatusRolledBack   = "rolled_back"
	UpgradeStatusManual       = "manual_intervention_required"
)

type UpgradeBundle struct {
	ID                     string   `json:"id"`
	Tool                   string   `json:"tool"`
	ReleaseName            string   `json:"release_name"`
	ChartName              string   `json:"chart_name"`
	RepoURL                string   `json:"repo_url"`
	SourceChartVersion     string   `json:"source_chart_version"`
	SourceAppVersion       string   `json:"source_app_version"`
	TargetChartVersion     string   `json:"target_chart_version"`
	TargetAppVersion       string   `json:"target_app_version"`
	RiskLevel              string   `json:"risk_level"`
	ExpectedDisruption     string   `json:"expected_disruption"`
	ReleaseNotes           string   `json:"release_notes,omitempty"`
	RequiresBackup         bool     `json:"requires_backup"`
	SupportedArchitectures []string `json:"supported_architectures,omitempty"`
	Status                 string   `json:"status"`
}

type UpgradeCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // pass | warning | blocked
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type UpgradeCandidate struct {
	Tool                string         `json:"tool"`
	ReleaseName         string         `json:"release_name"`
	CurrentChartVersion string         `json:"current_chart_version"`
	CurrentAppVersion   string         `json:"current_app_version"`
	CurrentRevision     int            `json:"current_revision"`
	ReleaseStatus       string         `json:"release_status"`
	Bundle              *UpgradeBundle `json:"bundle,omitempty"`
	UpToDate            bool           `json:"up_to_date"`
	BlockedReason       string         `json:"blocked_reason,omitempty"`
	CheckedAt           time.Time      `json:"checked_at"`
}

type UpgradeRun struct {
	ID                 string         `json:"id"`
	StackID            string         `json:"stack_id"`
	BundleID           string         `json:"bundle_id"`
	Tool               string         `json:"tool"`
	Status             string         `json:"status"`
	CurrentStep        string         `json:"current_step"`
	RequestedBy        string         `json:"requested_by"`
	Reason             string         `json:"reason"`
	IdempotencyKey     string         `json:"idempotency_key,omitempty"`
	SourceChartVersion string         `json:"source_chart_version"`
	TargetChartVersion string         `json:"target_chart_version"`
	PreviousRevision   int            `json:"previous_revision"`
	ResultRevision     int            `json:"result_revision,omitempty"`
	Checks             []UpgradeCheck `json:"checks"`
	Error              string         `json:"error,omitempty"`
	StartedAt          time.Time      `json:"started_at"`
	FinishedAt         *time.Time     `json:"finished_at,omitempty"`
}

func (r *UpgradeRun) Terminal() bool {
	return r.Status == UpgradeStatusBlocked || r.Status == UpgradeStatusSucceeded ||
		r.Status == UpgradeStatusRolledBack || r.Status == UpgradeStatusManual
}

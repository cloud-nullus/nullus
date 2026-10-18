package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 설치된 OSS 업그레이드(설계: docs/11_기능설계/Nullus_OSS_업그레이드_설계.md).
//
// 임의 버전으로 올리지 않는다. 실클러스터 리허설을 거쳐 verified 가 된 업그레이드 번들
// (source 차트 버전 → target 차트 버전)만 실행할 수 있다. 지금은 Argo CD 하나다.

const UpgradeToolArgoCD = "argocd"

// 번들 상태. verified 만 후보가 되고 실행할 수 있다.
const (
	UpgradeBundleDraft      = "draft"
	UpgradeBundleVerified   = "verified"
	UpgradeBundleDeprecated = "deprecated"
	UpgradeBundleRevoked    = "revoked"
)

// 실행 상태. upgrading · rolling_back 은 진행 중이고, 나머지는 끝난 상태다.
const (
	UpgradeStatusBlocked     = "blocked"
	UpgradeStatusUpgrading   = "upgrading"
	UpgradeStatusSucceeded   = "succeeded"
	UpgradeStatusRollingBack = "rolling_back"
	UpgradeStatusRolledBack  = "rolled_back"
	UpgradeStatusManual      = "manual_intervention_required"
)

// 실행 단계.
const (
	UpgradeStepPreflight   = "preflight"
	UpgradeStepHelmUpgrade = "helm_upgrade"
	UpgradeStepHealthCheck = "health_check"
	UpgradeStepRollback    = "rollback"
	UpgradeStepComplete    = "complete"
)

// 사전 점검 결과.
const (
	UpgradeCheckPass    = "pass"
	UpgradeCheckWarning = "warning"
	UpgradeCheckBlocked = "blocked"
)

// UpgradeStepMaxDuration 은 한 단계가 갱신 없이 걸릴 수 있는 최대 시간이다. Helm 의 대기
// 시간(30분)에 여유를 더했다. 이보다 오래 갱신이 없는 진행 중 실행은 API 가 작업 중에 죽은
// 것으로 본다 — 그대로 두면 그 스택의 업그레이드가 영원히 막힌다.
const UpgradeStepMaxDuration = 45 * time.Minute

var (
	ErrUpgradeReasonRequired         = errors.New("업그레이드 사유가 필요하다")
	ErrUpgradeIdempotencyKeyRequired = errors.New("Idempotency-Key 가 필요하다")
	ErrUpgradeInProgress             = errors.New("이 스택에서 다른 업그레이드가 진행 중이다")
	ErrUpgradeBundleNotFound         = errors.New("검증된 업그레이드 번들이 없다")
	ErrUpgradeRunNotFound            = errors.New("업그레이드 실행이 없다")
	ErrUpgradeReleaseNotFound        = errors.New("업그레이드할 Helm 릴리스가 없다")
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

// SelectUpgradeBundle 은 같은 출발점의 번들 중 가장 높은 대상 버전을 고른다.
// 읽을 수 없는 버전은 읽을 수 있는 버전보다 뒤로 민다.
func SelectUpgradeBundle(bundles []UpgradeBundle) *UpgradeBundle {
	var best *UpgradeBundle
	for i := range bundles {
		if best == nil || chartVersionGreater(bundles[i].TargetChartVersion, best.TargetChartVersion) {
			best = &bundles[i]
		}
	}
	return best
}

type UpgradeCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // pass | warning | blocked
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

func PassedCheck(name, message string) UpgradeCheck {
	return UpgradeCheck{Name: name, Status: UpgradeCheckPass, Message: message}
}

func BlockedCheck(name, message, remediation string) UpgradeCheck {
	return UpgradeCheck{Name: name, Status: UpgradeCheckBlocked, Message: message, Remediation: remediation}
}

// UpgradeChecksBlocked 는 하나라도 차단이면 참이다. 경고는 실행을 막지 않는다.
func UpgradeChecksBlocked(checks []UpgradeCheck) bool {
	for _, c := range checks {
		if c.Status == UpgradeCheckBlocked {
			return true
		}
	}
	return false
}

// EvaluateUpgradeBackup 은 마지막 복구 가능 백업으로 백업 점검을 만든다. 오래된 백업은
// 없는 것과 같다 — 그 사이의 변경은 되돌릴 수 없다.
func EvaluateUpgradeBackup(finishedAt time.Time, found bool, now time.Time, maxAge time.Duration) UpgradeCheck {
	const name, fix = "backup", "백업 화면에서 이 스택의 백업을 먼저 받으세요."
	if !found {
		return BlockedCheck(name, "복구 가능한 네임스페이스 리소스 백업이 없습니다.", fix)
	}
	if age := now.Sub(finishedAt); age > maxAge {
		return BlockedCheck(name, fmt.Sprintf("마지막 성공 백업이 %s 전입니다(허용 %s).", age.Round(time.Minute), maxAge), fix)
	}
	return PassedCheck(name, "마지막 성공 백업: "+finishedAt.UTC().Format(time.RFC3339))
}

// UpgradeCandidate 는 스택에 실제로 설치된 릴리스와 거기서 갈 수 있는 검증 경로다.
// Bundle 이 없다는 것은 "검증된 업그레이드 경로가 없다" 는 뜻이지 최신이라는 뜻이 아니다.
type UpgradeCandidate struct {
	Tool                string         `json:"tool"`
	ReleaseName         string         `json:"release_name"`
	CurrentChartVersion string         `json:"current_chart_version"`
	CurrentAppVersion   string         `json:"current_app_version"`
	CurrentRevision     int            `json:"current_revision"`
	ReleaseStatus       string         `json:"release_status"`
	Bundle              *UpgradeBundle `json:"bundle,omitempty"`
	BlockedReason       string         `json:"blocked_reason,omitempty"`
	LastRun             *UpgradeRun    `json:"last_run,omitempty"`
	CheckedAt           time.Time      `json:"checked_at"`
}

func NewUpgradeCandidate(tool, releaseName, chartVersion, appVersion string, revision int, releaseStatus string, bundle *UpgradeBundle, now time.Time) UpgradeCandidate {
	c := UpgradeCandidate{Tool: tool, ReleaseName: releaseName, CurrentChartVersion: chartVersion,
		CurrentAppVersion: appVersion, CurrentRevision: revision, ReleaseStatus: releaseStatus,
		Bundle: bundle, CheckedAt: now}
	if bundle != nil && releaseStatus != "deployed" {
		c.BlockedReason = "Helm 릴리스 상태가 " + releaseStatus + " 입니다 — deployed 로 돌아온 뒤 업그레이드할 수 있습니다."
	}
	return c
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
	IdempotencyKey     string         `json:"-"`
	SourceChartVersion string         `json:"source_chart_version"`
	TargetChartVersion string         `json:"target_chart_version"`
	PreviousRevision   int            `json:"previous_revision"`
	ResultRevision     int            `json:"result_revision,omitempty"`
	Checks             []UpgradeCheck `json:"checks"`
	Error              string         `json:"error,omitempty"`
	StartedAt          time.Time      `json:"started_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	FinishedAt         *time.Time     `json:"finished_at,omitempty"`
}

type NewUpgradeRunInput struct {
	ID, StackID        string
	Bundle             UpgradeBundle
	SourceChartVersion string
	PreviousRevision   int
	RequestedBy        string
	Reason             string
	IdempotencyKey     string
	Checks             []UpgradeCheck
	Now                time.Time
}

// NewUpgradeRun 은 사전 점검이 끝난 요청으로 실행을 만든다. 점검이 막혔으면 곧바로 끝난
// 실행(blocked)이 된다 — 시도와 막힌 이유도 감사 기록이다.
func NewUpgradeRun(in NewUpgradeRunInput) (*UpgradeRun, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrUpgradeReasonRequired
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return nil, ErrUpgradeIdempotencyKeyRequired
	}
	run := &UpgradeRun{ID: in.ID, StackID: in.StackID, BundleID: in.Bundle.ID, Tool: in.Bundle.Tool,
		Status: UpgradeStatusUpgrading, CurrentStep: UpgradeStepHelmUpgrade,
		RequestedBy: in.RequestedBy, Reason: strings.TrimSpace(in.Reason), IdempotencyKey: in.IdempotencyKey,
		SourceChartVersion: in.SourceChartVersion, TargetChartVersion: in.Bundle.TargetChartVersion,
		PreviousRevision: in.PreviousRevision, Checks: in.Checks, StartedAt: in.Now, UpdatedAt: in.Now}
	if run.Checks == nil {
		run.Checks = []UpgradeCheck{}
	}
	if UpgradeChecksBlocked(in.Checks) {
		run.Status, run.CurrentStep = UpgradeStatusBlocked, UpgradeStepPreflight
		run.Error = "사전 점검에서 차단됐다"
		run.finish(in.Now)
	}
	return run, nil
}

func (r *UpgradeRun) Active() bool {
	return r.Status == UpgradeStatusUpgrading || r.Status == UpgradeStatusRollingBack
}

func (r *UpgradeRun) Terminal() bool { return !r.Active() }

// Stale 은 진행 중인데 한 단계의 최대 시간보다 오래 갱신이 없는 실행이다.
func (r *UpgradeRun) Stale(now time.Time) bool {
	return r.Active() && now.Sub(r.UpdatedAt) > UpgradeStepMaxDuration
}

func (r *UpgradeRun) EnterHealthCheck(revision int, now time.Time) {
	r.ResultRevision, r.CurrentStep, r.UpdatedAt = revision, UpgradeStepHealthCheck, now
}

func (r *UpgradeRun) Succeed(now time.Time) {
	r.Status, r.CurrentStep = UpgradeStatusSucceeded, UpgradeStepComplete
	r.finish(now)
}

func (r *UpgradeRun) BeginRollback(cause error, now time.Time) {
	r.Status, r.CurrentStep, r.UpdatedAt = UpgradeStatusRollingBack, UpgradeStepRollback, now
	r.Error = cause.Error()
}

func (r *UpgradeRun) RolledBack(now time.Time) {
	r.Status = UpgradeStatusRolledBack
	r.finish(now)
}

func (r *UpgradeRun) RequireManualIntervention(cause error, now time.Time) {
	r.Status = UpgradeStatusManual
	if r.Error == "" {
		r.Error = cause.Error()
	} else {
		r.Error += "; " + cause.Error()
	}
	r.finish(now)
}

// Interrupt 는 작업 중에 API 가 사라진 실행을 닫는다. Helm 릴리스가 어디까지 갔는지 모르므로
// 자동으로 되돌리지 않고 사람에게 넘긴다.
func (r *UpgradeRun) Interrupt(now time.Time) {
	r.RequireManualIntervention(fmt.Errorf("작업 중 API 가 중단됐다(%s 단계) — Helm 릴리스 상태를 확인하라", r.CurrentStep), now)
}

func (r *UpgradeRun) finish(now time.Time) {
	r.UpdatedAt = now
	r.FinishedAt = &now
}

// chartVersionGreater 는 a 가 b 보다 높은 버전이면 참이다. 읽을 수 있는 버전이 읽을 수
// 없는 버전보다 높고, 둘 다 읽을 수 없으면 문자열로 비교한다.
func chartVersionGreater(a, b string) bool {
	va, okA := parseChartVersion(a)
	vb, okB := parseChartVersion(b)
	switch {
	case okA && okB:
		for i := 0; i < 3; i++ {
			if va.parts[i] != vb.parts[i] {
				return va.parts[i] > vb.parts[i]
			}
		}
		// 같은 번호면 정식판이 프리릴리스보다 높다.
		if (va.pre == "") != (vb.pre == "") {
			return va.pre == ""
		}
		return va.pre > vb.pre
	case okA:
		return true
	case okB:
		return false
	default:
		return a > b
	}
}

type chartVersion struct {
	parts [3]int
	pre   string
}

func parseChartVersion(v string) (chartVersion, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var out chartVersion
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, out.pre = v[:i], v[i+1:]
	}
	fields := strings.Split(v, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return chartVersion{}, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return chartVersion{}, false
		}
		out.parts[i] = n
	}
	return out, true
}

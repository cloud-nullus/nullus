package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	cicddomain "github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/observability/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakePipelineSecurity struct {
	list *cicddomain.PipelineSecurityList
	err  error
}

func (f fakePipelineSecurity) Execute(context.Context, string) (*cicddomain.PipelineSecurityList, error) {
	return f.list, f.err
}

// cicd 의 단계별 최신 결과를 관측 도메인 입력으로 옮긴다. 모르는 숫자는 nil 그대로다 — 0 으로 채우지 않는다.
func TestAppSecuritySource_MapsSnapshot(t *testing.T) {
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	two, vulns, hotspots := 2, 4, 2
	db := at.Add(-48 * time.Hour)
	snap := &cicddomain.PipelineSecuritySnapshot{
		PipelineID: "pip_1", Name: "order-api", StackID: "stk_prod", ClusterID: "cl_prod",
		Stages:        []string{"SecretScan", "DependencyScan", "Build", "SAST", "ImageScan", "Deploy"},
		HasSecretScan: true, HasDependencyScan: true, HasSAST: true, HasImageScan: true,
		LatestRun: &cicddomain.RunSummary{DeploymentID: "dep_42", Version: "#42", Status: cicddomain.DeploymentStatusFailed, StartedAt: at},
		SecretScan: &cicddomain.SecretScanResult{DeploymentID: "dep_42", GateResult: cicddomain.GateResultBlock, FindingsCount: &two,
			Rules: []cicddomain.SecretRuleCount{{RuleID: "github-pat", Count: 2}}, ReportURI: "https://ci/gitleaks", ScannedAt: at},
		DependencyScan: &cicddomain.DependencyScanResult{DeploymentID: "dep_42", GateResult: cicddomain.GateResultBlock,
			Counts: &cicddomain.SeverityCounts{Critical: 3, High: 5}, Targets: &two, DBUpdatedAt: &db, ReportURI: "https://ci/dep", ScannedAt: at},
		SAST: &cicddomain.SASTResult{DeploymentID: "dep_41", GateResult: cicddomain.GateResultWarn, DashboardURL: "https://sonar/x",
			Metrics:    &cicddomain.SASTMetrics{Vulnerabilities: &vulns, SecurityHotspots: &hotspots},
			Conditions: []cicddomain.SASTCondition{{Metric: "new_violations", Status: "ERROR"}, {Metric: "coverage", Status: "OK"}},
			AnalyzedAt: at.Add(-time.Hour)},
		ImageScan: &cicddomain.ImageScanResult{DeploymentID: "dep_41", GateResult: cicddomain.GateResultBlock,
			ImageRepository: "registry/order-api", ImageTag: "abc", ImageDigest: "sha256:ab",
			Counts: &cicddomain.SeverityCounts{Critical: 2, High: 11}, ScannedAt: at.Add(-time.Hour)},
		RunVersions:     map[string]string{"dep_41": "#41"},
		SecretDetection: cicddomain.SecretDetectionWarn,
	}
	apps, truncated, err := NewAppSecuritySource(fakePipelineSecurity{list: &cicddomain.PipelineSecurityList{
		Pipelines: []*cicddomain.PipelineSecuritySnapshot{snap, nil}, Truncated: true,
	}}).ListAppSecurity(context.Background(), "org-1")
	require.NoError(t, err)
	assert.True(t, truncated)
	require.Len(t, apps, 1)
	a := apps[0]

	assert.Equal(t, "order-api", a.Name)
	assert.True(t, a.StagesKnown)
	assert.True(t, a.HasStage[domain.StageSAST])
	assert.False(t, a.SecretPolicyOff)
	require.NotNil(t, a.LatestRun)
	assert.Equal(t, "failed", a.LatestRun.Status)

	sec := a.Records[domain.StageSecretScan]
	require.NotNil(t, sec)
	assert.Equal(t, "block", sec.Gate)
	assert.Equal(t, "#42", sec.Version, "최근 실행이면 그 번호를 쓴다")
	assert.Equal(t, 2, *sec.Findings)
	assert.Equal(t, []domain.SecretRule{{RuleID: "github-pat", Count: 2}}, sec.Rules)
	assert.Equal(t, "https://ci/gitleaks", sec.Link)

	dep := a.Records[domain.StageDependencyScan]
	assert.Equal(t, 2, *dep.Targets)
	assert.Equal(t, 3, dep.Counts.Critical)
	assert.Equal(t, &db, dep.DBUpdatedAt)

	sast := a.Records[domain.StageSAST]
	assert.Equal(t, "#41", sast.Version)
	assert.Equal(t, 4, *sast.Vulnerabilities)
	assert.Equal(t, 2, *sast.SecurityHotspots)
	assert.Equal(t, []string{"new_violations"}, sast.FailedConditions)
	assert.Equal(t, "https://sonar/x", sast.Link)

	img := a.Records[domain.StageImageScan]
	assert.Equal(t, "registry/order-api@sha256:ab", img.Image)
	assert.Equal(t, 11, img.Counts.High)
}

// 정책 off · 스택 없음 · 옛 파이프라인 · 결과 없음을 그대로 옮긴다.
func TestAppSecuritySource_PolicyAndMissing(t *testing.T) {
	list := &cicddomain.PipelineSecurityList{Pipelines: []*cicddomain.PipelineSecuritySnapshot{
		{PipelineID: "off", StackID: "stk_1", SecretDetection: cicddomain.SecretDetectionOff},
		{PipelineID: "nostack", SecretDetection: cicddomain.SecretDetectionOff},
		{PipelineID: "sast-unknown", Stages: []string{"SAST"}, HasSAST: true,
			SAST:       &cicddomain.SASTResult{DeploymentID: "dep_x", GateResult: cicddomain.GateResultError},
			ImageScan:  &cicddomain.ImageScanResult{DeploymentID: "dep_y", GateResult: cicddomain.GateResultError, ImageRepository: "repo", ImageTag: "1"},
			SecretScan: &cicddomain.SecretScanResult{DeploymentID: "dep_y", GateResult: cicddomain.GateResultError}},
	}}
	apps, truncated, err := NewAppSecuritySource(fakePipelineSecurity{list: list}).ListAppSecurity(context.Background(), "org-1")
	require.NoError(t, err)
	assert.False(t, truncated)
	require.Len(t, apps, 3)

	assert.True(t, apps[0].SecretPolicyOff)
	assert.False(t, apps[0].StagesKnown)
	assert.Empty(t, apps[0].Records)
	assert.Nil(t, apps[0].LatestRun)
	assert.False(t, apps[1].SecretPolicyOff, "스택이 없으면 정책도 없다 — 단계 유무로만 본다")

	sast := apps[2].Records[domain.StageSAST]
	require.NotNil(t, sast)
	assert.Nil(t, sast.Vulnerabilities, "분석하지 못한 실행의 지표는 모른다")
	assert.Equal(t, []string{}, sast.FailedConditions)
	assert.Empty(t, sast.Version, "실행 번호를 모르면 지어내지 않는다")
	assert.Equal(t, "repo:1", apps[2].Records[domain.StageImageScan].Image)
	assert.Nil(t, apps[2].Records[domain.StageSecretScan].Findings)
	assert.Equal(t, []domain.SecretRule{}, apps[2].Records[domain.StageSecretScan].Rules)
}

func TestAppSecuritySource_Errors(t *testing.T) {
	_, _, err := NewAppSecuritySource(fakePipelineSecurity{err: errors.New("db down")}).ListAppSecurity(context.Background(), "org-1")
	assert.Error(t, err)
	_, _, err = NewAppSecuritySource(nil).ListAppSecurity(context.Background(), "org-1")
	assert.Error(t, err)
	apps, _, err := NewAppSecuritySource(fakePipelineSecurity{}).ListAppSecurity(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Empty(t, apps)
}

func TestImageRef(t *testing.T) {
	assert.Equal(t, "repo@sha256:1", imageRef("repo", "tag", "sha256:1"))
	assert.Equal(t, "repo:tag", imageRef("repo", "tag", ""))
	assert.Equal(t, "repo", imageRef("repo", "", ""))
	assert.Equal(t, "", imageRef(" ", "tag", "sha256:1"))
}

type fakeOrgReports struct {
	reports []stackdomain.OrgStackImageScan
	err     error
}

func (f fakeOrgReports) ReportsForOrg(context.Context, string) ([]stackdomain.OrgStackImageScan, error) {
	return f.reports, f.err
}

func TestStackSecuritySource_MapsReports(t *testing.T) {
	at := time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)
	counts := shareddomain.SeverityCounts{Critical: 61, High: 1569}
	fixable := shareddomain.SeverityCounts{Critical: 1}
	report := stackdomain.BuildImageScanReport("stk_prod", "", []stackdomain.StackImageScan{
		{Image: "gitlab-toolbox", Status: stackdomain.ImageScanStatusScanned, Counts: &counts, FixableCounts: &fixable, ScannedAt: at, DBUpdatedAt: &at},
		{Image: "cert-manager", Status: stackdomain.ImageScanStatusFailed, Shared: true, Exposed: true, ScannedAt: at},
	})
	report.Airgap = true
	notScanned := stackdomain.BuildImageScanReport("stk_dev", stackdomain.ImageScanReasonScannerNotInstalled, nil)
	src := NewStackSecuritySource(fakeOrgReports{reports: []stackdomain.OrgStackImageScan{
		{Stack: &stackdomain.Stack{ID: "stk_prod", Name: "stack-prod", ClusterID: "cl_prod"}, Report: report},
		{Stack: nil, Report: report},
		{Stack: &stackdomain.Stack{ID: "stk_dev", Name: "stack-dev", ClusterID: "cl_dev"}, Report: notScanned},
	}})

	got, err := src.ListStackImageSecurity(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	prod := got[0]
	assert.Equal(t, "stack-prod", prod.StackName)
	assert.Equal(t, "scanned", prod.State)
	assert.True(t, prod.Airgap)
	require.NotNil(t, prod.ScannedAt)
	require.Len(t, prod.Images, 2)
	assert.True(t, prod.Images[0].Scanned)
	assert.Equal(t, &fixable, prod.Images[0].Fixable)
	assert.False(t, prod.Images[1].Scanned)
	assert.True(t, prod.Images[1].Shared)
	assert.True(t, prod.Images[1].Exposed)

	dev := got[1]
	assert.Equal(t, "not_scanned", dev.State)
	assert.Equal(t, "scanner_not_installed", dev.Reason)
	assert.Empty(t, dev.Images)

	_, err = NewStackSecuritySource(fakeOrgReports{err: errors.New("db down")}).ListStackImageSecurity(context.Background(), "org-1")
	assert.Error(t, err)
	_, err = NewStackSecuritySource(nil).ListStackImageSecurity(context.Background(), "org-1")
	assert.Error(t, err)
}

type fakeOrgStacks struct {
	stacks         []*stackdomain.Stack
	err            error
	includeDeleted bool
}

func (f *fakeOrgStacks) List(_ context.Context, _ string, includeDeleted bool) ([]*stackdomain.Stack, error) {
	f.includeDeleted = includeDeleted
	return f.stacks, f.err
}

type fakeOrgClusters struct {
	clusters []*admindomain.Cluster
	err      error
}

func (f fakeOrgClusters) List(context.Context, string) ([]*admindomain.Cluster, error) {
	return f.clusters, f.err
}

// 범위 이름은 stack · admin 모듈의 공개 저장소로 읽는다 — clusters 테이블을 관측 모듈이 직접 읽지 않는다.
func TestSecurityScope(t *testing.T) {
	stacks := &fakeOrgStacks{stacks: []*stackdomain.Stack{{ID: "stk_1", Name: "stack-prod", ClusterID: "cl_1"}, nil}}
	scope := NewSecurityScope(stacks, fakeOrgClusters{clusters: []*admindomain.Cluster{{ID: "cl_1", Name: "prod-a"}, nil}})

	refs, err := scope.ListStacks(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, []domain.SecurityStackRef{{ID: "stk_1", Name: "stack-prod", ClusterID: "cl_1"}}, refs)
	assert.False(t, stacks.includeDeleted, "지운 스택은 묶음이 되지 않는다")

	names, err := scope.ClusterNames(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"cl_1": "prod-a"}, names)

	failing := NewSecurityScope(&fakeOrgStacks{err: errors.New("x")}, fakeOrgClusters{err: errors.New("y")})
	_, err = failing.ListStacks(context.Background(), "org-1")
	assert.Error(t, err)
	_, err = failing.ClusterNames(context.Background(), "org-1")
	assert.Error(t, err)

	unwired := NewSecurityScope(nil, nil)
	_, err = unwired.ListStacks(context.Background(), "org-1")
	assert.Error(t, err)
	_, err = unwired.ClusterNames(context.Background(), "org-1")
	assert.Error(t, err)
}

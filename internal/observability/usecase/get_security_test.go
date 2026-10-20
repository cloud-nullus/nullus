package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

var secNow = time.Date(2026, 10, 20, 9, 12, 0, 0, time.UTC)

type fakeAppSource struct {
	apps      []domain.AppSecurityRecord
	truncated bool
	err       error
	gotOrg    string
}

func (f *fakeAppSource) ListAppSecurity(_ context.Context, orgID string) ([]domain.AppSecurityRecord, bool, error) {
	f.gotOrg = orgID
	return f.apps, f.truncated, f.err
}

type fakeStackSource struct {
	inputs []domain.StackImageInput
	err    error
}

func (f *fakeStackSource) ListStackImageSecurity(context.Context, string) ([]domain.StackImageInput, error) {
	return f.inputs, f.err
}

type fakeScope struct {
	stacks     []domain.SecurityStackRef
	stacksErr  error
	clusters   map[string]string
	clusterErr error
}

func (f *fakeScope) ListStacks(context.Context, string) ([]domain.SecurityStackRef, error) {
	return f.stacks, f.stacksErr
}

func (f *fakeScope) ClusterNames(context.Context, string) (map[string]string, error) {
	return f.clusters, f.clusterErr
}

func secRec(gate string) *domain.ScanRecord {
	return &domain.ScanRecord{Gate: gate, DeploymentID: "dep", At: secNow}
}

func allSecurityStages() map[domain.SecurityStage]bool {
	return map[domain.SecurityStage]bool{domain.StageSecretScan: true, domain.StageDependencyScan: true,
		domain.StageSAST: true, domain.StageImageScan: true}
}

func passingApp(name, stackID, clusterID string) domain.AppSecurityRecord {
	return domain.AppSecurityRecord{
		PipelineID: "pip_" + name, Name: name, StackID: stackID, ClusterID: clusterID,
		StagesKnown: true, HasStage: allSecurityStages(),
		Records: map[domain.SecurityStage]*domain.ScanRecord{
			domain.StageSecretScan: secRec("pass"), domain.StageDependencyScan: secRec("pass"),
			domain.StageSAST: secRec("pass"), domain.StageImageScan: secRec("pass"),
		},
	}
}

func securityFixture() (*fakeAppSource, *fakeScope) {
	leak := passingApp("order-api", "stk_prod", "cl_prod")
	leak.Records[domain.StageSecretScan] = secRec("block")
	blocked := passingApp("web-front", "stk_stg", "cl_stg")
	img := secRec("block")
	img.Counts = &shareddomain.SeverityCounts{Critical: 1}
	blocked.Records[domain.StageImageScan] = img
	ok := passingApp("payment-api", "stk_prod", "cl_prod")
	direct := passingApp("direct-app", "", "cl_prod")
	direct.HasStage = map[domain.SecurityStage]bool{}
	// 스택이 지워진(소프트 삭제) 앱 — stack_id 가 남아 있어도 "스택 없음" 이다.
	orphan := passingApp("orphan-app", "stk_deleted", "cl_old")
	return &fakeAppSource{apps: []domain.AppSecurityRecord{ok, direct, blocked, leak, orphan}, truncated: true},
		&fakeScope{
			stacks: []domain.SecurityStackRef{
				{ID: "stk_prod", Name: "stack-prod", ClusterID: "cl_prod"},
				{ID: "stk_stg", Name: "stack-stg", ClusterID: "cl_stg"},
			},
			clusters: map[string]string{"cl_prod": "prod-a", "cl_stg": "stg-a"},
		}
}

func newAppUC(apps *fakeAppSource, scope *fakeScope) *GetAppSecurity {
	uc := NewGetAppSecurity(apps, scope)
	uc.now = func() time.Time { return secNow }
	return uc
}

func appNames(ps []domain.AppSecurityPosture) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.App.Name)
	}
	return out
}

// 조직 전체를 위험순으로 내리고 스택별로 묶는다. 지표 · 적용률 · 묶음은 범위 안의 앱 전체로 센다.
func TestGetAppSecurity_OrgWide(t *testing.T) {
	apps, scope := securityFixture()
	view, err := newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: " org-1 "})
	require.NoError(t, err)

	assert.Equal(t, "org-1", apps.gotOrg)
	assert.Equal(t, secNow, view.GeneratedAt)
	assert.True(t, view.Truncated)
	assert.Equal(t, []string{"order-api", "web-front", "orphan-app", "payment-api", "direct-app"}, appNames(view.Apps))
	assert.Equal(t, domain.AppSecuritySummary{Apps: 5, WithSecurityStage: 4, Blocked: 2, Secrets: 1, Critical: 1}, view.Summary)
	assert.Equal(t, domain.StageCoverage{SecretScan: 4, DependencyScan: 4, SAST: 4, ImageScan: 4}, view.Coverage)

	require.Len(t, view.Groups, 3)
	assert.Equal(t, "stk_prod", view.Groups[0].StackID)
	assert.Equal(t, "prod-a", view.Groups[0].ClusterName)
	assert.Equal(t, 2, view.Groups[0].Apps)
	assert.Equal(t, "stk_stg", view.Groups[1].StackID)
	assert.Equal(t, "", view.Groups[2].StackID, "직접 배포 · 스택이 지워진 앱은 스택 없음으로 묶는다")
	assert.Equal(t, 2, view.Groups[2].Apps)

	for _, p := range view.Apps {
		if p.App.Name == "orphan-app" {
			assert.Empty(t, p.App.StackID)
			assert.Equal(t, "cl_old", p.App.ClusterID, "스택이 없으면 파이프라인의 클러스터다")
		}
	}
}

// "문제 있는 것만" 은 표만 줄인다 — 지표 · 적용률 · 묶음은 그대로다(설계 §3).
func TestGetAppSecurity_ProblemOnlyKeepsKPIs(t *testing.T) {
	apps, scope := securityFixture()
	all, err := newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: "org-1"})
	require.NoError(t, err)
	problems, err := newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: "org-1", ProblemOnly: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"order-api", "web-front"}, appNames(problems.Apps))
	assert.Equal(t, all.Summary, problems.Summary)
	assert.Equal(t, all.Coverage, problems.Coverage)
	assert.Equal(t, all.Groups, problems.Groups)
}

// 범위 필터(클러스터 · 스택 · 검색)는 지표까지 따른다. stack_id=none 은 스택 없는 앱이다.
func TestGetAppSecurity_ScopeFilters(t *testing.T) {
	apps, scope := securityFixture()
	uc := newAppUC(apps, scope)

	view, err := uc.Execute(context.Background(), AppSecurityQuery{OrgID: "org-1", ClusterID: "cl_prod"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"order-api", "payment-api", "direct-app"}, appNames(view.Apps))
	assert.Equal(t, 3, view.Summary.Apps)

	view, err = uc.Execute(context.Background(), AppSecurityQuery{OrgID: "org-1", StackID: "stk_stg"})
	require.NoError(t, err)
	assert.Equal(t, []string{"web-front"}, appNames(view.Apps))
	assert.Equal(t, 1, view.Summary.Blocked)
	require.Len(t, view.Groups, 1)

	view, err = uc.Execute(context.Background(), AppSecurityQuery{OrgID: "org-1", StackID: StackIDNone})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"direct-app", "orphan-app"}, appNames(view.Apps))

	view, err = uc.Execute(context.Background(), AppSecurityQuery{OrgID: "org-1", Query: "  API "})
	require.NoError(t, err)
	assert.Equal(t, []string{"order-api", "payment-api"}, appNames(view.Apps), "이름 검색은 대소문자를 가리지 않는다")
	assert.Equal(t, 2, view.Summary.Apps)
}

// 클러스터 이름은 머리말일 뿐이다 — 못 읽어도 화면은 뜬다. 앱 · 스택을 못 읽으면 실패다.
func TestGetAppSecurity_Errors(t *testing.T) {
	apps, scope := securityFixture()
	scope.clusterErr = errors.New("admin down")
	view, err := newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: "org-1"})
	require.NoError(t, err)
	assert.Empty(t, view.Groups[0].ClusterName)

	_, err = newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: " "})
	assert.Error(t, err)

	apps.err = errors.New("db down")
	_, err = newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: "org-1"})
	assert.Error(t, err)

	apps.err = nil
	scope.stacksErr = errors.New("stack down")
	_, err = newAppUC(apps, scope).Execute(context.Background(), AppSecurityQuery{OrgID: "org-1"})
	assert.Error(t, err)

	var nilUC *GetAppSecurity
	_, err = nilUC.Execute(context.Background(), AppSecurityQuery{OrgID: "org-1"})
	assert.Error(t, err)
}

// 앱이 없으면 빈 목록이다(null 이 아니다).
func TestGetAppSecurity_NoApps(t *testing.T) {
	view, err := newAppUC(&fakeAppSource{}, &fakeScope{}).Execute(context.Background(), AppSecurityQuery{OrgID: "org-1"})
	require.NoError(t, err)
	assert.NotNil(t, view.Apps)
	assert.NotNil(t, view.Groups)
	assert.Zero(t, view.Summary.Apps)
}

func stackFixture() (*fakeStackSource, *fakeScope) {
	crit := shareddomain.SeverityCounts{Critical: 1}
	at := secNow.Add(-time.Hour)
	db := secNow.Add(-24 * time.Hour)
	return &fakeStackSource{inputs: []domain.StackImageInput{
			{StackID: "stk_dev", StackName: "stack-dev", ClusterID: "cl_dev", State: domain.StackScanNotScanned, Reason: "scanner_not_installed"},
			{StackID: "stk_prod", StackName: "stack-prod", ClusterID: "cl_prod", State: domain.StackScanScanned, ScannedAt: &at,
				Images: []domain.StackImage{{Image: "harbor-core", Exposed: true, Scanned: true, Counts: &crit, Fixable: &crit, DBUpdatedAt: &db}}},
			{StackID: "stk_stg", StackName: "stack-stg", ClusterID: "cl_stg", State: domain.StackScanScanned, ScannedAt: &at,
				Images: []domain.StackImage{{Image: "redis", Scanned: true, Counts: &crit, Fixable: &crit, DBUpdatedAt: &db}}},
		}},
		&fakeScope{clusters: map[string]string{"cl_prod": "prod-a", "cl_stg": "stg-a"}}
}

func newStackUC(src *fakeStackSource, scope *fakeScope) *GetStackSecurity {
	uc := NewGetStackSecurity(src, scope)
	uc.now = func() time.Time { return secNow }
	return uc
}

// 설치 OSS 축 — 조치할 것이 많은 스택부터, 스캔하지 않은 스택은 숫자 없이 뒤로.
func TestGetStackSecurity(t *testing.T) {
	src, scope := stackFixture()
	view, err := newStackUC(src, scope).Execute(context.Background(), StackSecurityQuery{OrgID: "org-1"})
	require.NoError(t, err)

	assert.Equal(t, secNow, view.GeneratedAt)
	require.Len(t, view.Stacks, 3)
	assert.Equal(t, []string{"stk_prod", "stk_stg", "stk_dev"}, []string{view.Stacks[0].StackID, view.Stacks[1].StackID, view.Stacks[2].StackID})
	assert.Equal(t, "prod-a", view.Stacks[0].ClusterName)
	assert.Equal(t, 1, *view.Stacks[0].GradeA)
	assert.Equal(t, 1, *view.Stacks[1].GradeB)
	assert.Nil(t, view.Stacks[2].Images)
	assert.Equal(t, domain.StackSecuritySummary{Stacks: 3, ScannedStacks: 2, NotScannedStacks: 1,
		ActionImages: 2, GradeAImages: 1, GradeBImages: 1}, view.Summary)

	view, err = newStackUC(src, scope).Execute(context.Background(), StackSecurityQuery{OrgID: "org-1", ClusterID: "cl_stg"})
	require.NoError(t, err)
	require.Len(t, view.Stacks, 1)
	assert.Equal(t, 1, view.Summary.Stacks)

	view, err = newStackUC(src, scope).Execute(context.Background(), StackSecurityQuery{OrgID: "org-1", StackID: "stk_dev"})
	require.NoError(t, err)
	require.Len(t, view.Stacks, 1)
	assert.Equal(t, 1, view.Summary.NotScannedStacks)
}

func TestGetStackSecurity_Errors(t *testing.T) {
	src, scope := stackFixture()
	scope.clusterErr = errors.New("admin down")
	view, err := newStackUC(src, scope).Execute(context.Background(), StackSecurityQuery{OrgID: "org-1"})
	require.NoError(t, err, "클러스터 이름을 못 읽어도 화면은 뜬다")
	assert.Empty(t, view.Stacks[0].ClusterName)

	_, err = newStackUC(src, scope).Execute(context.Background(), StackSecurityQuery{OrgID: ""})
	assert.Error(t, err)

	src.err = errors.New("db down")
	_, err = newStackUC(src, scope).Execute(context.Background(), StackSecurityQuery{OrgID: "org-1"})
	assert.Error(t, err)

	var nilUC *GetStackSecurity
	_, err = nilUC.Execute(context.Background(), StackSecurityQuery{OrgID: "org-1"})
	assert.Error(t, err)
}

package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

var secNow = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

func intp(v int) *int { return &v }

func allStages() map[SecurityStage]bool {
	return map[SecurityStage]bool{StageSecretScan: true, StageDependencyScan: true, StageSAST: true, StageImageScan: true}
}

func rec(gate, deploymentID string) *ScanRecord {
	return &ScanRecord{Gate: gate, DeploymentID: deploymentID, Version: "#1", At: secNow}
}

func app(name string, records map[SecurityStage]*ScanRecord) AppSecurityRecord {
	return AppSecurityRecord{
		PipelineID: "pip_" + name, Name: name, StackID: "stk_1", ClusterID: "cl_1",
		StagesKnown: true, HasStage: allStages(),
		LatestRun: &AppRun{DeploymentID: "dep_latest", Version: "#42", Status: "success", StartedAt: secNow},
		Records:   records,
	}
}

// 셀 상태 여섯 가지 × 단계 유무 × 정책 off (설계 §2.2).
func TestCellFor_States(t *testing.T) {
	a := app("orders", map[SecurityStage]*ScanRecord{
		StageSecretScan:     rec("pass", "dep_latest"),
		StageDependencyScan: rec("warn", "dep_latest"),
		StageSAST:           rec("block", "dep_41"),
	})
	assert.Equal(t, CellPass, CellFor(a, StageSecretScan, secNow).State)
	assert.Equal(t, CellWarn, CellFor(a, StageDependencyScan, secNow).State)
	assert.Equal(t, CellBlock, CellFor(a, StageSAST, secNow).State)
	assert.Equal(t, CellUnrecorded, CellFor(a, StageImageScan, secNow).State, "단계는 있는데 결과가 없다")

	a.Records[StageImageScan] = rec("error", "dep_latest")
	assert.Equal(t, CellError, CellFor(a, StageImageScan, secNow).State)
	a.Records[StageImageScan] = rec("weird", "dep_latest")
	assert.Equal(t, CellError, CellFor(a, StageImageScan, secNow).State, "모르는 판정을 통과로 넘겨짚지 않는다")

	// 파이프라인에 단계가 없으면 결과가 남아 있어도 미적용이다.
	a.HasStage = map[SecurityStage]bool{StageSecretScan: true}
	assert.Equal(t, CellNotApplied, CellFor(a, StageSAST, secNow).State)
	assert.Nil(t, CellFor(a, StageSAST, secNow).Record)

	// 스택 정책이 off 면 시크릿은 결과가 있어도 미적용이다 — 단계는 돌되 바로 통과한다.
	a.SecretPolicyOff = true
	assert.Equal(t, CellNotApplied, CellFor(a, StageSecretScan, secNow).State)
}

// 단계 기록 전의 옛 파이프라인은 결과가 있으면 보이고, 없으면 미적용이다(기록 없음이 아니다).
func TestCellFor_LegacyPipelineWithoutStages(t *testing.T) {
	a := app("legacy", map[SecurityStage]*ScanRecord{StageImageScan: rec("block", "dep_latest")})
	a.StagesKnown, a.HasStage = false, nil

	assert.Equal(t, CellBlock, CellFor(a, StageImageScan, secNow).State)
	assert.Equal(t, CellNotApplied, CellFor(a, StageSAST, secNow).State)
	assert.Equal(t, CellNotApplied, CellFor(a, StageSecretScan, secNow).State)
}

// 셀은 단계별 최신 기록이다 — 앱의 최근 실행과 다르면 표시한다(설계 §2.1).
func TestCellFor_StaleRun(t *testing.T) {
	a := app("orders", map[SecurityStage]*ScanRecord{
		StageSecretScan: rec("block", "dep_latest"),
		StageImageScan:  rec("block", "dep_41"),
	})
	assert.False(t, CellFor(a, StageSecretScan, secNow).StaleRun)
	assert.True(t, CellFor(a, StageImageScan, secNow).StaleRun)

	a.LatestRun = nil
	assert.False(t, CellFor(a, StageImageScan, secNow).StaleRun, "실행을 모르면 낡았다고 하지 않는다")
}

// DB 신선도는 리포트를 읽은 의존성 · 이미지 셀에만 있다. DB 시각을 모르면 낡은 것으로 본다.
func TestCellFor_DBStale(t *testing.T) {
	fresh := secNow.Add(-24 * time.Hour)
	old := secNow.Add(-34 * 24 * time.Hour)
	img := rec("block", "dep_latest")
	img.Counts, img.DBUpdatedAt = &shareddomain.SeverityCounts{Critical: 2}, &fresh
	dep := rec("pass", "dep_latest")
	dep.Counts, dep.DBUpdatedAt = &shareddomain.SeverityCounts{}, &old
	unknownDB := rec("pass", "dep_latest")
	unknownDB.Counts = &shareddomain.SeverityCounts{}
	a := app("orders", map[SecurityStage]*ScanRecord{
		StageImageScan: img, StageDependencyScan: dep, StageSAST: rec("pass", "dep_latest"),
		StageSecretScan: rec("pass", "dep_latest"),
	})

	require.NotNil(t, CellFor(a, StageImageScan, secNow).DBStale)
	assert.False(t, *CellFor(a, StageImageScan, secNow).DBStale)
	assert.True(t, *CellFor(a, StageDependencyScan, secNow).DBStale)
	assert.Nil(t, CellFor(a, StageSAST, secNow).DBStale, "SAST 에는 취약점 DB 가 없다")
	assert.Nil(t, CellFor(a, StageSecretScan, secNow).DBStale)

	a.Records[StageDependencyScan] = unknownDB
	assert.True(t, *CellFor(a, StageDependencyScan, secNow).DBStale, "DB 시각을 모르면 낡은 것으로 본다")

	a.Records[StageDependencyScan] = rec("error", "dep_latest")
	assert.Nil(t, CellFor(a, StageDependencyScan, secNow).DBStale, "리포트를 못 읽은 셀은 DB 를 모른다")
}

func cells(states ...CellState) map[SecurityStage]SecurityCell {
	out := map[SecurityStage]SecurityCell{}
	for i, st := range states {
		out[SecurityStages[i]] = SecurityCell{State: st}
	}
	return out
}

// 위험 순위(설계 §2.3).
func TestRankOf(t *testing.T) {
	cases := []struct {
		name  string
		cells map[SecurityStage]SecurityCell
		want  int
	}{
		{"시크릿 차단", cells(CellBlock, CellPass, CellPass, CellPass), RankSecretLeak},
		{"시크릿 경고도 유출이다", cells(CellWarn, CellPass, CellNotApplied, CellPass), RankSecretLeak},
		{"차단", cells(CellPass, CellBlock, CellError, CellWarn), RankBlocked},
		{"판정 불가", cells(CellPass, CellError, CellWarn, CellPass), RankUndetermined},
		{"기록 없음", cells(CellPass, CellUnrecorded, CellPass, CellPass), RankUndetermined},
		{"경고", cells(CellPass, CellWarn, CellNotApplied, CellPass), RankWarning},
		{"모두 통과", cells(CellPass, CellNotApplied, CellNotApplied, CellPass), RankPassing},
		{"보안 단계 없음", cells(CellNotApplied, CellNotApplied, CellNotApplied, CellNotApplied), RankNoStage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RankOf(tc.cells))
		})
	}
	assert.True(t, IsProblemRank(RankSecretLeak))
	assert.True(t, IsProblemRank(RankWarning))
	assert.False(t, IsProblemRank(RankPassing))
	assert.False(t, IsProblemRank(RankNoStage))
	assert.False(t, IsProblemRank(0))
}

func withCounts(r *ScanRecord, critical, high int) *ScanRecord {
	r.Counts = &shareddomain.SeverityCounts{Critical: critical, High: high}
	return r
}

// 같은 순위 안에서는 CRITICAL 합(의존성 + 이미지) → 최근 실행 → 이름 순이다.
func TestSortAppPostures(t *testing.T) {
	mk := func(name string, rank int, critical int, runAt *time.Time) AppSecurityPosture {
		a := AppSecurityRecord{PipelineID: "pip_" + name, Name: name}
		if runAt != nil {
			a.LatestRun = &AppRun{StartedAt: *runAt}
		}
		return AppSecurityPosture{App: a, Rank: rank, Critical: critical}
	}
	earlier := secNow.Add(-time.Hour)
	ps := []AppSecurityPosture{
		mk("zeta", RankBlocked, 1, &secNow),
		mk("alpha", RankPassing, 0, &secNow),
		mk("beta", RankBlocked, 5, &earlier),
		mk("gamma", RankBlocked, 1, &earlier),
		mk("delta", RankBlocked, 1, nil),
		mk("eta", RankBlocked, 1, &secNow),
		mk("leak", RankSecretLeak, 0, nil),
	}
	ps = append(ps, AppSecurityPosture{App: AppSecurityRecord{PipelineID: "pip_eta_2", Name: "eta", LatestRun: &AppRun{StartedAt: secNow}}, Rank: RankBlocked, Critical: 1})
	SortAppPostures(ps)

	names := []string{}
	for _, p := range ps {
		names = append(names, p.App.PipelineID)
	}
	assert.Equal(t, []string{"pip_leak", "pip_beta", "pip_eta", "pip_eta_2", "pip_zeta", "pip_gamma", "pip_delta", "pip_alpha"}, names)
}

// EvaluateApp 는 셀 · 순위 · CRITICAL 합(모르는 건수는 0)을 함께 낸다. SAST 는 CRITICAL 합에 들지 않는다.
func TestEvaluateApp(t *testing.T) {
	a := app("orders", map[SecurityStage]*ScanRecord{
		StageSecretScan:     rec("pass", "dep_latest"),
		StageDependencyScan: withCounts(rec("block", "dep_latest"), 3, 5),
		StageSAST:           rec("warn", "dep_41"),
		StageImageScan:      withCounts(rec("block", "dep_41"), 2, 11),
	})
	p := EvaluateApp(a, secNow)
	assert.Equal(t, RankBlocked, p.Rank)
	assert.Equal(t, 5, p.Critical)
	assert.Len(t, p.Cells, 4)

	a.Records[StageImageScan] = rec("error", "dep_41")
	assert.Equal(t, 3, EvaluateApp(a, secNow).Critical, "모르는 건수는 0 으로 센다(정렬용)")
}

// 지표는 범위 안의 앱 전체로 센다 — 대상 수로 센다(설계 §3).
func TestSummarizeApps(t *testing.T) {
	leak := app("leak", map[SecurityStage]*ScanRecord{
		StageSecretScan: rec("warn", "dep_latest"), StageDependencyScan: withCounts(rec("pass", "dep_latest"), 0, 0),
		StageSAST: rec("pass", "dep_latest"), StageImageScan: withCounts(rec("block", "dep_latest"), 2, 0),
	})
	unknown := app("unknown", map[SecurityStage]*ScanRecord{
		StageSecretScan: rec("pass", "dep_latest"), StageDependencyScan: withCounts(rec("warn", "dep_latest"), 1, 0),
	})
	unknown.HasStage = map[SecurityStage]bool{StageSecretScan: true, StageDependencyScan: true, StageImageScan: true}
	none := app("none", nil)
	none.HasStage = map[SecurityStage]bool{}
	ps := []AppSecurityPosture{EvaluateApp(leak, secNow), EvaluateApp(unknown, secNow), EvaluateApp(none, secNow)}

	sum, cov := SummarizeApps(ps)
	assert.Equal(t, AppSecuritySummary{Apps: 3, WithSecurityStage: 2, Blocked: 1, Secrets: 1, Critical: 2, Undetermined: 1}, sum)
	assert.Equal(t, StageCoverage{SecretScan: 2, DependencyScan: 2, SAST: 1, ImageScan: 2}, cov)
}

// 앱 표는 스택별로 묶고 머리에 집계를 단다. 가장 위험한 앱이 있는 묶음부터, 스택 없음은 맨 뒤다.
func TestGroupAppsByStack(t *testing.T) {
	prod := app("order-api", map[SecurityStage]*ScanRecord{StageSecretScan: rec("block", "dep_latest"),
		StageImageScan: withCounts(rec("block", "dep_latest"), 1, 0), StageDependencyScan: rec("pass", "dep_latest"),
		StageSAST: rec("pass", "dep_latest")})
	prod.StackID = "stk_prod"
	prodOK := app("payment", map[SecurityStage]*ScanRecord{StageSecretScan: rec("pass", "d"), StageDependencyScan: rec("pass", "d"),
		StageSAST: rec("pass", "d"), StageImageScan: rec("pass", "d")})
	prodOK.StackID = "stk_prod"
	stg := app("web", map[SecurityStage]*ScanRecord{StageSecretScan: rec("pass", "d"), StageDependencyScan: rec("warn", "d"),
		StageSAST: rec("error", "d"), StageImageScan: rec("pass", "d")})
	stg.StackID = "stk_stg"
	dev := app("tool", map[SecurityStage]*ScanRecord{StageSecretScan: rec("pass", "d"), StageDependencyScan: rec("pass", "d"),
		StageSAST: rec("pass", "d"), StageImageScan: rec("pass", "d")})
	dev.StackID = "stk_dev"
	dev2 := dev
	dev2.PipelineID, dev2.Name, dev2.StackID = "pip_tool2", "tool2", "stk_dev2"
	orphan := app("orphan", map[SecurityStage]*ScanRecord{StageSecretScan: rec("block", "d")})
	orphan.StackID = ""

	ps := []AppSecurityPosture{EvaluateApp(orphan, secNow), EvaluateApp(dev2, secNow), EvaluateApp(dev, secNow),
		EvaluateApp(stg, secNow), EvaluateApp(prodOK, secNow), EvaluateApp(prod, secNow)}
	stacks := map[string]SecurityStackRef{
		"stk_prod": {ID: "stk_prod", Name: "stack-prod", ClusterID: "cl_prod"},
		"stk_stg":  {ID: "stk_stg", Name: "stack-stg", ClusterID: "cl_stg"},
		"stk_dev":  {ID: "stk_dev", Name: "stack-dev", ClusterID: "cl_dev"},
		"stk_dev2": {ID: "stk_dev2", Name: "stack-dev", ClusterID: "cl_dev"},
	}
	groups := GroupAppsByStack(ps, stacks, map[string]string{"cl_prod": "prod-a", "cl_stg": "stg-a"})

	require.Len(t, groups, 5)
	assert.Equal(t, []string{"stk_prod", "stk_stg", "stk_dev", "stk_dev2", ""},
		[]string{groups[0].StackID, groups[1].StackID, groups[2].StackID, groups[3].StackID, groups[4].StackID})
	g := groups[0]
	assert.Equal(t, "stack-prod", g.StackName)
	assert.Equal(t, "cl_prod", g.ClusterID)
	assert.Equal(t, "prod-a", g.ClusterName)
	assert.Equal(t, 2, g.Apps)
	assert.Equal(t, 1, g.Blocked)
	assert.Equal(t, 1, g.Secrets)
	assert.Equal(t, 1, g.Critical)
	assert.Equal(t, 0, g.Undetermined)
	assert.Equal(t, 1, g.Problems)
	assert.Equal(t, 1, groups[1].Undetermined)
	assert.Equal(t, "", groups[2].ClusterName, "모르는 클러스터 이름은 비운다")
	none := groups[4]
	assert.Empty(t, none.StackName)
	assert.Empty(t, none.ClusterID, "스택 없음 묶음에는 클러스터가 없다")
	assert.Equal(t, 1, none.Apps)
}

// 묶음 순서 — 같은 위험이면 문제 수 많은 순이다.
func TestGroupAppsByStack_OrdersByProblems(t *testing.T) {
	warn := func(name, stack string) AppSecurityPosture {
		a := app(name, map[SecurityStage]*ScanRecord{StageSecretScan: rec("pass", "d"), StageDependencyScan: rec("warn", "d"),
			StageSAST: rec("pass", "d"), StageImageScan: rec("pass", "d")})
		a.StackID = stack
		return EvaluateApp(a, secNow)
	}
	groups := GroupAppsByStack([]AppSecurityPosture{warn("a", "stk_a"), warn("b", "stk_b"), warn("c", "stk_b")},
		map[string]SecurityStackRef{"stk_a": {Name: "zzz"}, "stk_b": {Name: "aaa-not-first-by-name"}}, nil)
	assert.Equal(t, "stk_b", groups[0].StackID)

	// 위험도 문제 수도 같으면 스택 이름 순이다.
	groups = GroupAppsByStack([]AppSecurityPosture{warn("a", "stk_a"), warn("b", "stk_b")},
		map[string]SecurityStackRef{"stk_a": {Name: "zzz"}, "stk_b": {Name: "aaa"}}, nil)
	assert.Equal(t, []string{"stk_b", "stk_a"}, []string{groups[0].StackID, groups[1].StackID})
}

func scannedImage(name string, exposed bool, fixable, all shareddomain.SeverityCounts, db *time.Time) StackImage {
	return StackImage{Image: name, Exposed: exposed, Scanned: true, Counts: &all, Fixable: &fixable, DBUpdatedAt: db}
}

// 이미지 등급(설계 §3 M5) — 수정판 있는 CRITICAL 이 있으면 A(노출) · B(내부), 없고 HIGH 만 있으면 C.
func TestGradeImage(t *testing.T) {
	crit := shareddomain.SeverityCounts{Critical: 1, High: 3}
	high := shareddomain.SeverityCounts{High: 2}
	assert.Equal(t, GradeA, GradeImage(scannedImage("a", true, crit, crit, nil)))
	assert.Equal(t, GradeB, GradeImage(scannedImage("b", false, crit, crit, nil)))
	assert.Equal(t, GradeC, GradeImage(scannedImage("c", true, high, high, nil)))
	assert.Equal(t, GradeNone, GradeImage(scannedImage("d", false, shareddomain.SeverityCounts{Medium: 4}, crit, nil)),
		"수정판 없는 CRITICAL 은 조치 대상이 아니다")
	assert.Equal(t, GradeNone, GradeImage(StackImage{Image: "e", Scanned: false}))
	assert.Equal(t, GradeNone, GradeImage(StackImage{Image: "f", Scanned: true}), "수정판 건수를 모르면 매기지 않는다")
}

// 설치 OSS 행 — 스캔한 스택만 숫자가 있다. 가장 심각한 이미지는 전체 건수로 고른다.
func TestEvaluateStack_Scanned(t *testing.T) {
	fresh := secNow.Add(-2 * 24 * time.Hour)
	older := secNow.Add(-5 * 24 * time.Hour)
	scannedAt := secNow.Add(-3 * time.Hour)
	in := StackImageInput{
		StackID: "stk_prod", StackName: "stack-prod", ClusterID: "cl_prod", State: StackScanScanned, Airgap: true,
		ScannedAt: &scannedAt,
		Images: []StackImage{
			scannedImage("gitlab-toolbox", false, shareddomain.SeverityCounts{Critical: 1, High: 20},
				shareddomain.SeverityCounts{Critical: 61, High: 1569}, &fresh),
			scannedImage("harbor-core", true, shareddomain.SeverityCounts{Critical: 2}, shareddomain.SeverityCounts{Critical: 3}, &older),
			scannedImage("redis", false, shareddomain.SeverityCounts{High: 1}, shareddomain.SeverityCounts{High: 1}, &fresh),
			{Image: "cert-manager", Shared: true, Scanned: false},
		},
	}
	in.Images[2].Shared = true

	p := EvaluateStack(in, secNow)
	assert.Equal(t, "scanned", p.State)
	assert.True(t, p.Airgap)
	assert.Equal(t, 4, *p.Images)
	assert.Equal(t, 1, *p.FailedImages)
	assert.Equal(t, 2, *p.SharedImages)
	assert.Equal(t, 1, *p.GradeA)
	assert.Equal(t, 1, *p.GradeB)
	assert.Equal(t, 1, *p.GradeC)
	assert.Equal(t, shareddomain.SeverityCounts{Critical: 3, High: 21}, *p.Fixable)
	require.NotNil(t, p.WorstImage)
	assert.Equal(t, WorstImage{Image: "gitlab-toolbox", Critical: 61, High: 1569}, *p.WorstImage)
	require.NotNil(t, p.DBUpdatedAt)
	assert.True(t, older.Equal(*p.DBUpdatedAt), "가장 오래된 DB 시각을 숨기지 않는다")
	assert.False(t, *p.DBStale)
	assert.Equal(t, &scannedAt, p.ScannedAt)
}

// DB 시각을 모르는 이미지가 있거나 DB 가 30일을 넘으면 낡았다고 본다. 같은 건수면 이름 순으로 고른다.
func TestEvaluateStack_DBStaleAndTies(t *testing.T) {
	old := secNow.Add(-40 * 24 * time.Hour)
	same := shareddomain.SeverityCounts{Critical: 1}
	p := EvaluateStack(StackImageInput{State: StackScanScanned, Images: []StackImage{
		scannedImage("b-image", false, same, same, &old),
		scannedImage("a-image", false, same, same, &old),
	}}, secNow)
	assert.True(t, *p.DBStale)
	assert.Equal(t, "a-image", p.WorstImage.Image)

	p = EvaluateStack(StackImageInput{State: StackScanScanned, Images: []StackImage{
		{Image: "x", Scanned: true, Counts: &same},
	}}, secNow)
	assert.True(t, *p.DBStale, "DB 시각을 모르면 낡은 것으로 본다")
	assert.Nil(t, p.DBUpdatedAt)
	assert.Zero(t, *p.GradeB, "수정판 건수를 모르면 등급을 매기지 않는다")
	assert.Equal(t, shareddomain.SeverityCounts{}, *p.Fixable)

	lower := shareddomain.SeverityCounts{High: 9}
	p = EvaluateStack(StackImageInput{State: StackScanScanned, Images: []StackImage{
		scannedImage("low", false, lower, lower, nil), scannedImage("crit", false, same, same, nil),
	}}, secNow)
	assert.Equal(t, "crit", p.WorstImage.Image)
}

// 스캔하지 않은 스택의 숫자는 0 이 아니라 없다 — 사유를 그대로 둔다.
func TestEvaluateStack_NotScanned(t *testing.T) {
	for _, state := range []string{StackScanNotScanned, StackScanPending} {
		p := EvaluateStack(StackImageInput{StackID: "stk_dev", State: state, Reason: "scanner_not_installed"}, secNow)
		assert.Equal(t, state, p.State)
		assert.Nil(t, p.Images)
		assert.Nil(t, p.GradeA)
		assert.Nil(t, p.Fixable)
		assert.Nil(t, p.WorstImage)
		assert.Nil(t, p.DBStale)
	}
	assert.Equal(t, "scanner_not_installed", EvaluateStack(StackImageInput{State: StackScanNotScanned, Reason: "scanner_not_installed"}, secNow).Reason)
}

func TestSortStackPostures(t *testing.T) {
	mk := func(id, state string, a, b, c, fixCrit int) StackSecurityPosture {
		p := StackSecurityPosture{StackID: id, StackName: "s-" + id, State: state}
		if state == StackScanScanned {
			p.GradeA, p.GradeB, p.GradeC = intp(a), intp(b), intp(c)
			p.Fixable = &shareddomain.SeverityCounts{Critical: fixCrit}
		}
		return p
	}
	ps := []StackSecurityPosture{
		mk("dev", StackScanNotScanned, 0, 0, 0, 0),
		mk("weird", "unknown", 0, 0, 0, 0),
		mk("pending", StackScanPending, 0, 0, 0, 0),
		mk("stg", StackScanScanned, 0, 4, 6, 4),
		mk("prod", StackScanScanned, 3, 5, 9, 8),
		mk("qa", StackScanScanned, 0, 4, 6, 9),
		mk("qa2", StackScanScanned, 0, 4, 6, 9),
		mk("ok", StackScanScanned, 0, 0, 0, 0),
	}
	ps[6].StackName = ps[5].StackName
	SortStackPostures(ps)
	ids := []string{}
	for _, p := range ps {
		ids = append(ids, p.StackID)
	}
	assert.Equal(t, []string{"prod", "qa", "qa2", "stg", "ok", "pending", "dev", "weird"}, ids)

	nilFix := []StackSecurityPosture{{StackID: "b", StackName: "b", State: StackScanScanned}, {StackID: "a", StackName: "a", State: StackScanScanned}}
	SortStackPostures(nilFix)
	assert.Equal(t, "a", nilFix[0].StackID, "숫자를 모르면 0 으로 보고 이름 순")
}

func TestSummarizeStacks(t *testing.T) {
	ps := []StackSecurityPosture{
		{State: StackScanScanned, GradeA: intp(3), GradeB: intp(5)},
		{State: StackScanScanned, GradeA: intp(0), GradeB: intp(4)},
		{State: StackScanNotScanned},
		{State: StackScanPending},
	}
	assert.Equal(t, StackSecuritySummary{Stacks: 4, ScannedStacks: 2, NotScannedStacks: 1,
		ActionImages: 12, GradeAImages: 3, GradeBImages: 9}, SummarizeStacks(ps))
}

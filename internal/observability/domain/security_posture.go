package domain

import (
	"sort"
	"strings"
	"time"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 보안 대시보드(nullus-plan#65, docs/11_기능설계/Nullus_Security_Dashboard_설계.md)의 읽기 모델이다.
//
// 두 축을 섞지 않는다(결정 2) — 앱 축은 배포를 막는 게이트(파이프라인 단계별 최신 결과)이고, 설치 OSS
// 축은 막지 않는 보고(스택 설치 이미지 스캔)다. 숫자를 합치지 않는다. 결과 테이블은 cicd · stack 이
// 소유하고, 여기서는 bridge 가 옮겨 준 값으로 셀 상태 · 위험 순위 · 지표 · 등급만 계산한다.

// SecurityStage 는 앱 행의 보안 단계 열이다. 순서는 파이프라인 단계 순서(시크릿 → 의존성 → SAST → 이미지)다.
type SecurityStage string

const (
	StageSecretScan     SecurityStage = "secret_scan"
	StageDependencyScan SecurityStage = "dependency_scan"
	StageSAST           SecurityStage = "sast"
	StageImageScan      SecurityStage = "image_scan"
)

// SecurityStages 는 열 순서다.
var SecurityStages = []SecurityStage{StageSecretScan, StageDependencyScan, StageSAST, StageImageScan}

// CellState 는 셀 하나의 상태다(설계 §2.2). 뒤의 셋은 초록으로 보이지 않는다 — 돌지 않은 것을 성공으로
// 보여주지 않는다(000070 의 교훈).
type CellState string

const (
	CellPass  CellState = "pass"
	CellWarn  CellState = "warn"
	CellBlock CellState = "block"
	// CellError 는 판정 불가다(스캐너 장애 · 리포트 없음). "통과 아님".
	CellError CellState = "error"
	// CellUnrecorded 는 파이프라인에 단계는 있는데 결과가 아직 없다는 뜻이다.
	CellUnrecorded CellState = "unrecorded"
	// CellNotApplied 는 파이프라인에 단계가 없다는 뜻이다(시크릿은 스택 정책 off 도).
	CellNotApplied CellState = "not_applied"
)

// SecretRule 은 시크릿 탐지 규칙 하나의 건수다.
type SecretRule struct {
	RuleID string
	Count  int
}

// ScanRecord 는 단계 하나의 최신 결과 기록이다. 모르는 숫자는 nil 이다 — 0 으로 채우지 않는다.
type ScanRecord struct {
	// Gate 는 결과의 판정(pass · warn · block · error)이다.
	Gate         string
	DeploymentID string
	// Version 은 실행의 표시 번호(#42)다. 모르면 비어 있다.
	Version string
	At      time.Time
	// Link 는 리포트(시크릿 · 의존성 · 이미지) 또는 SonarQube 화면(SAST) 주소다.
	Link string

	// 시크릿 탐지
	Findings *int
	Rules    []SecretRule

	// 의존성 · 이미지 스캔. Counts 는 스택 정책(ignore_unfixed)을 따른 건수다.
	Counts      *shareddomain.SeverityCounts
	Targets     *int
	DBUpdatedAt *time.Time
	Image       string

	// SAST
	Vulnerabilities  *int
	SecurityHotspots *int
	FailedConditions []string
}

// AppRun 은 앱의 최근 실행이다.
type AppRun struct {
	DeploymentID string
	Version      string
	Status       string
	StartedAt    time.Time
}

// AppSecurityRecord 는 앱(파이프라인) 하나의 입력이다. 앱 테이블은 없다 — 파이프라인 한 행이 앱 하나다.
type AppSecurityRecord struct {
	PipelineID string
	Name       string
	// StackID 는 앱이 속한 (지우지 않은) 스택이다. 비면 "스택 없음" 이다.
	StackID   string
	ClusterID string
	// StagesKnown 이 거짓이면 단계 기록 전의 파이프라인이다 — 결과가 있으면 보이고 없으면 미적용이다.
	StagesKnown bool
	// HasStage 는 파이프라인 stages 에 그 단계가 있는지다. StagesKnown 일 때만 뜻이 있다.
	HasStage map[SecurityStage]bool
	// SecretPolicyOff 는 스택 정책의 시크릿 탐지가 off 인지다. 스택이 없으면 거짓이다(단계 유무로만 본다).
	SecretPolicyOff bool
	LatestRun       *AppRun
	Records         map[SecurityStage]*ScanRecord
}

// SecurityCell 은 도출한 셀 하나다. Record 는 상태가 결과에서 왔을 때만 있다.
type SecurityCell struct {
	State  CellState
	Record *ScanRecord
	// StaleRun 은 셀의 실행이 앱의 최근 실행이 아니라는 뜻이다 — 앞 단계가 막으면 뒤 단계는 돌지 않는다.
	StaleRun bool
	// DBStale 은 의존성 · 이미지 셀에서 리포트를 읽었을 때만 있다. DB 시각을 모르면 낡은 것으로 본다.
	DBStale *bool
}

// AppSecurityPosture 는 앱 하나의 도출 결과다.
type AppSecurityPosture struct {
	App   AppSecurityRecord
	Rank  int
	Cells map[SecurityStage]SecurityCell
	// Critical 은 의존성 + 이미지 셀의 CRITICAL 합이다(정렬용). 모르는 건수는 0 으로 센다.
	Critical int
}

// 위험 순위(설계 §2.3). 작을수록 위험하다.
const (
	RankSecretLeak   = 1 // 시크릿 셀이 block · warn — 커밋을 지워도 이미 유출이다
	RankBlocked      = 2 // 셀 하나라도 block — 배포가 멈춰 있다
	RankUndetermined = 3 // 셀 하나라도 error · unrecorded — 상태를 모른다
	RankWarning      = 4 // 셀 하나라도 warn
	RankPassing      = 5 // 적용된 셀이 모두 pass
	RankNoStage      = 6 // 모든 셀이 not_applied — 보안 단계가 없다
)

// IsProblemRank 는 "문제 있는 것만" 에 드는 순위(1~4)인지 본다.
func IsProblemRank(rank int) bool {
	return rank >= RankSecretLeak && rank <= RankWarning
}

// CellFor 는 앱의 단계 하나의 셀을 도출한다(설계 §2.2).
//
//   - 시크릿은 스택 정책이 off 면 미적용이다 — 단계는 돌되 바로 통과하므로 결과가 있어도 탐지한 것이 아니다.
//   - 단계 목록을 아는 파이프라인: 단계가 없으면 미적용, 있는데 결과가 없으면 기록 없음.
//   - 단계 목록을 모르는 옛 파이프라인: 결과가 있으면 보이고, 없으면 미적용이다.
func CellFor(app AppSecurityRecord, stage SecurityStage, now time.Time) SecurityCell {
	if stage == StageSecretScan && app.SecretPolicyOff {
		return SecurityCell{State: CellNotApplied}
	}
	rec := app.Records[stage]
	if app.StagesKnown && !app.HasStage[stage] {
		return SecurityCell{State: CellNotApplied}
	}
	if rec == nil {
		if app.StagesKnown {
			return SecurityCell{State: CellUnrecorded}
		}
		return SecurityCell{State: CellNotApplied}
	}

	cell := SecurityCell{State: cellStateFromGate(rec.Gate), Record: rec}
	if app.LatestRun != nil && rec.DeploymentID != app.LatestRun.DeploymentID {
		cell.StaleRun = true
	}
	if (stage == StageDependencyScan || stage == StageImageScan) && rec.Counts != nil {
		stale := shareddomain.IsDBStale(rec.DBUpdatedAt, now)
		cell.DBStale = &stale
	}
	return cell
}

// cellStateFromGate 는 결과의 판정을 셀 상태로 옮긴다. 모르는 값은 판정 불가다 — 통과로 넘겨짚지 않는다.
func cellStateFromGate(gate string) CellState {
	switch CellState(strings.ToLower(strings.TrimSpace(gate))) {
	case CellPass:
		return CellPass
	case CellWarn:
		return CellWarn
	case CellBlock:
		return CellBlock
	default:
		return CellError
	}
}

// RankOf 는 셀들로 위험 순위를 매긴다(설계 §2.3).
func RankOf(cells map[SecurityStage]SecurityCell) int {
	if s := cells[StageSecretScan].State; s == CellBlock || s == CellWarn {
		return RankSecretLeak
	}
	var block, unknown, warn, applied bool
	for _, stage := range SecurityStages {
		switch cells[stage].State {
		case CellBlock:
			block = true
		case CellError, CellUnrecorded:
			unknown = true
		case CellWarn:
			warn = true
		}
		if st := cells[stage].State; st != CellNotApplied && st != "" {
			applied = true
		}
	}
	switch {
	case block:
		return RankBlocked
	case unknown:
		return RankUndetermined
	case warn:
		return RankWarning
	case applied:
		return RankPassing
	default:
		return RankNoStage
	}
}

// EvaluateApp 는 앱 하나의 셀 · 순위 · CRITICAL 합을 도출한다.
func EvaluateApp(app AppSecurityRecord, now time.Time) AppSecurityPosture {
	cells := make(map[SecurityStage]SecurityCell, len(SecurityStages))
	for _, stage := range SecurityStages {
		cells[stage] = CellFor(app, stage, now)
	}
	return AppSecurityPosture{App: app, Rank: RankOf(cells), Cells: cells, Critical: criticalOf(cells)}
}

// criticalOf 는 의존성 + 이미지 셀의 CRITICAL 합이다. SAST 에는 심각도 건수가 없다.
func criticalOf(cells map[SecurityStage]SecurityCell) int {
	total := 0
	for _, stage := range []SecurityStage{StageDependencyScan, StageImageScan} {
		if c := cells[stage]; c.Record != nil && c.Record.Counts != nil {
			total += c.Record.Counts.Critical
		}
	}
	return total
}

// SortAppPostures 는 위험순으로 정렬한다(설계 §2.3) — 순위 → CRITICAL 합 많은 순 → 최근 실행 최근 순 →
// 이름 순. 화면은 받은 순서를 그대로 쓴다.
func SortAppPostures(ps []AppSecurityPosture) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		if a.Critical != b.Critical {
			return a.Critical > b.Critical
		}
		at, bt := lastRunAt(a.App), lastRunAt(b.App)
		if !at.Equal(bt) {
			return at.After(bt)
		}
		if a.App.Name != b.App.Name {
			return a.App.Name < b.App.Name
		}
		return a.App.PipelineID < b.App.PipelineID
	})
}

// lastRunAt 은 최근 실행 시각이다. 실행이 없으면 0 값이라 뒤로 간다.
func lastRunAt(app AppSecurityRecord) time.Time {
	if app.LatestRun == nil {
		return time.Time{}
	}
	return app.LatestRun.StartedAt
}

// AppSecuritySummary 는 앱 축의 지표다(설계 §3, M1~M4). 건수가 아니라 조치할 앱 수로 센다.
type AppSecuritySummary struct {
	Apps int
	// WithSecurityStage 는 보안 단계가 하나라도 적용된 앱 수다(M1 의 분모).
	WithSecurityStage int
	// Blocked 는 셀 하나라도 block 인 앱 수다(M1).
	Blocked int
	// Secrets 는 시크릿 셀이 block 또는 warn 인 앱 수다 — 경고 정책이어도 센다(M2).
	Secrets int
	// Critical 은 이미지 또는 의존성 셀의 CRITICAL 이 1 이상인 앱 수다(M3, 정책 기준).
	Critical int
	// Undetermined 는 셀 하나라도 error 또는 unrecorded 인 앱 수다(M4).
	Undetermined int
}

// StageCoverage 는 단계마다 적용된(미적용이 아닌) 앱 수다. KPI 가 아니라 열 머리에 둔다(설계 §3).
type StageCoverage struct {
	SecretScan     int
	DependencyScan int
	SAST           int
	ImageScan      int
}

// appFlags 는 앱 하나가 지표의 어디에 드는지다.
type appFlags struct {
	applied, blocked, secret, critical, undetermined bool
}

func flagsOf(p AppSecurityPosture) appFlags {
	var f appFlags
	f.applied = p.Rank != RankNoStage
	if s := p.Cells[StageSecretScan].State; s == CellBlock || s == CellWarn {
		f.secret = true
	}
	for _, stage := range SecurityStages {
		c := p.Cells[stage]
		switch c.State {
		case CellBlock:
			f.blocked = true
		case CellError, CellUnrecorded:
			f.undetermined = true
		}
		if (stage == StageDependencyScan || stage == StageImageScan) && c.Record != nil &&
			c.Record.Counts != nil && c.Record.Counts.Critical > 0 {
			f.critical = true
		}
	}
	return f
}

// SummarizeApps 는 지표와 적용률을 센다. 범위 필터를 거친 앱 전체로 센다 — "문제 있는 것만" 으로 표를
// 줄여도 KPI 는 그대로다(설계 §3).
func SummarizeApps(ps []AppSecurityPosture) (AppSecuritySummary, StageCoverage) {
	var sum AppSecuritySummary
	var cov StageCoverage
	for _, p := range ps {
		sum.Apps++
		f := flagsOf(p)
		if f.applied {
			sum.WithSecurityStage++
		}
		if f.blocked {
			sum.Blocked++
		}
		if f.secret {
			sum.Secrets++
		}
		if f.critical {
			sum.Critical++
		}
		if f.undetermined {
			sum.Undetermined++
		}
		if p.Cells[StageSecretScan].State != CellNotApplied {
			cov.SecretScan++
		}
		if p.Cells[StageDependencyScan].State != CellNotApplied {
			cov.DependencyScan++
		}
		if p.Cells[StageSAST].State != CellNotApplied {
			cov.SAST++
		}
		if p.Cells[StageImageScan].State != CellNotApplied {
			cov.ImageScan++
		}
	}
	return sum, cov
}

// SecurityStackRef 는 스택 하나의 이름과 클러스터다.
type SecurityStackRef struct {
	ID        string
	Name      string
	ClusterID string
}

// AppSecurityGroup 은 스택 하나의 묶음 머리다. 스택 없음 묶음은 StackID 가 비어 있다.
type AppSecurityGroup struct {
	StackID      string
	StackName    string
	ClusterID    string
	ClusterName  string
	Apps         int
	Blocked      int
	Secrets      int
	Critical     int
	Undetermined int
	// Problems 는 문제 있는 앱(순위 1~4) 수다.
	Problems int
	// bestRank 는 묶음에서 가장 위험한 앱의 순위다(정렬용).
	bestRank int
}

// GroupAppsByStack 는 앱을 스택별로 묶고 머리에 집계를 단다. 묶음 순서는 가장 위험한 앱이 있는 묶음부터 →
// 문제 수 많은 순 → 스택 이름 순이고, 스택 없음 묶음은 맨 뒤다. 클러스터는 스택의 것이다(스택 없음은 비어 있다).
func GroupAppsByStack(ps []AppSecurityPosture, stacks map[string]SecurityStackRef, clusterNames map[string]string) []AppSecurityGroup {
	byID := map[string]*AppSecurityGroup{}
	order := make([]string, 0)
	for _, p := range ps {
		id := p.App.StackID
		g, ok := byID[id]
		if !ok {
			g = &AppSecurityGroup{StackID: id, bestRank: RankNoStage + 1}
			if ref, known := stacks[id]; known && id != "" {
				g.StackName = ref.Name
				g.ClusterID = ref.ClusterID
				g.ClusterName = clusterNames[ref.ClusterID]
			}
			byID[id] = g
			order = append(order, id)
		}
		f := flagsOf(p)
		g.Apps++
		if f.blocked {
			g.Blocked++
		}
		if f.secret {
			g.Secrets++
		}
		if f.critical {
			g.Critical++
		}
		if f.undetermined {
			g.Undetermined++
		}
		if IsProblemRank(p.Rank) {
			g.Problems++
		}
		if p.Rank < g.bestRank {
			g.bestRank = p.Rank
		}
	}
	out := make([]AppSecurityGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.StackID == "") != (b.StackID == "") {
			return b.StackID == ""
		}
		if a.bestRank != b.bestRank {
			return a.bestRank < b.bestRank
		}
		if a.Problems != b.Problems {
			return a.Problems > b.Problems
		}
		if a.StackName != b.StackName {
			return a.StackName < b.StackName
		}
		return a.StackID < b.StackID
	})
	return out
}

// ── 설치 OSS 축 ────────────────────────────────────────────────────────────

// 스택 설치 이미지 스캔 보고서 상태(스택 모듈의 어휘와 같다).
const (
	StackScanScanned    = "scanned"
	StackScanPending    = "pending"
	StackScanNotScanned = "not_scanned"
)

// StackImage 는 설치 이미지 하나의 스캔 결과다.
type StackImage struct {
	Image   string
	Shared  bool
	Exposed bool
	// Scanned 가 거짓이면 그 이미지를 스캔하지 못했다(건수를 모른다).
	Scanned     bool
	Counts      *shareddomain.SeverityCounts
	Fixable     *shareddomain.SeverityCounts
	DBUpdatedAt *time.Time
}

// StackImageInput 은 스택 하나의 설치 이미지 스캔 보고서다.
type StackImageInput struct {
	StackID   string
	StackName string
	ClusterID string
	// State 는 보고서 상태(scanned · pending · not_scanned)다.
	State  string
	Reason string
	Airgap bool
	// ScannedAt 은 마지막 스캔 시각이다. 스캔한 적 없으면 nil 이다.
	ScannedAt *time.Time
	Images    []StackImage
}

// ImageGrade 는 이미지 단위의 CVE 대응 등급이다(설치 OSS CVE 대응 정책 §3.1 을 이미지 단위로 매긴다, 설계 §3 M5).
type ImageGrade string

const (
	// GradeA 는 수정판 있는 CRITICAL + 외부 노출이다.
	GradeA ImageGrade = "A"
	// GradeB 는 수정판 있는 CRITICAL + 내부다.
	GradeB ImageGrade = "B"
	// GradeC 는 수정판 있는 CRITICAL 은 없고 수정판 있는 HIGH 만 있다.
	GradeC ImageGrade = "C"
	// GradeNone 은 조치 대상이 아니거나(수정판 있는 것이 없다) 모른다(스캔 실패).
	GradeNone ImageGrade = ""
)

// GradeImage 는 이미지의 등급이다. 정책의 등급은 CVE 단위지만 KPI 는 대상 수이므로, 수정판 있는 CRITICAL 이
// 하나라도 있으면 그 이미지는 A(외부 노출) 또는 B(내부)다. 수정판 건수를 모르면 등급을 매기지 않는다.
func GradeImage(img StackImage) ImageGrade {
	if !img.Scanned || img.Fixable == nil {
		return GradeNone
	}
	if img.Fixable.Critical > 0 {
		if img.Exposed {
			return GradeA
		}
		return GradeB
	}
	if img.Fixable.High > 0 {
		return GradeC
	}
	return GradeNone
}

// WorstImage 는 스택에서 가장 심각한 이미지다(전체 건수 기준 — 업스트림 이미지의 규모를 숨기지 않는다).
type WorstImage struct {
	Image    string
	Critical int
	High     int
	Exposed  bool
}

// StackSecurityPosture 는 스택 하나의 설치 OSS 행이다. 스캔하지 않은 스택의 숫자는 nil 이다 — "스캔 안 함"
// 을 0 으로 보이지 않는다(설계 §4.4).
type StackSecurityPosture struct {
	StackID     string
	StackName   string
	ClusterID   string
	ClusterName string
	State       string
	Reason      string
	Airgap      bool

	Images       *int
	FailedImages *int
	SharedImages *int
	GradeA       *int
	GradeB       *int
	GradeC       *int
	// Fixable 은 수정판 있는 취약점 건수의 합이다(스캔한 이미지).
	Fixable    *shareddomain.SeverityCounts
	WorstImage *WorstImage

	ScannedAt *time.Time
	// DBUpdatedAt 은 스캔한 이미지 가운데 가장 오래된 취약점 DB 시각이다 — 낡은 쪽을 숨기지 않는다.
	DBUpdatedAt *time.Time
	DBStale     *bool
}

// EvaluateStack 은 스택 하나의 설치 OSS 행을 만든다.
func EvaluateStack(in StackImageInput, now time.Time) StackSecurityPosture {
	p := StackSecurityPosture{
		StackID: in.StackID, StackName: in.StackName, ClusterID: in.ClusterID,
		State: in.State, Reason: in.Reason, Airgap: in.Airgap, ScannedAt: in.ScannedAt,
	}
	if in.State != StackScanScanned {
		return p
	}
	var images, failed, shared, a, b, c int
	var fixable shareddomain.SeverityCounts
	var worst *WorstImage
	var worstCounts shareddomain.SeverityCounts
	var oldestDB *time.Time
	dbKnown := true
	for _, img := range in.Images {
		images++
		if img.Shared {
			shared++
		}
		if !img.Scanned {
			failed++
			continue
		}
		switch GradeImage(img) {
		case GradeA:
			a++
		case GradeB:
			b++
		case GradeC:
			c++
		}
		if img.Fixable != nil {
			fixable = fixable.Plus(*img.Fixable)
		}
		if img.Counts != nil && (worst == nil || moreSevereCounts(*img.Counts, worstCounts, img.Image, worst.Image)) {
			worstCounts = *img.Counts
			worst = &WorstImage{Image: img.Image, Critical: img.Counts.Critical, High: img.Counts.High, Exposed: img.Exposed}
		}
		if img.DBUpdatedAt == nil || img.DBUpdatedAt.IsZero() {
			dbKnown = false
		} else if oldestDB == nil || img.DBUpdatedAt.Before(*oldestDB) {
			t := *img.DBUpdatedAt
			oldestDB = &t
		}
	}
	p.Images, p.FailedImages, p.SharedImages = &images, &failed, &shared
	p.GradeA, p.GradeB, p.GradeC = &a, &b, &c
	p.Fixable = &fixable
	p.WorstImage = worst
	p.DBUpdatedAt = oldestDB
	stale := !dbKnown || shareddomain.IsDBStale(oldestDB, now)
	p.DBStale = &stale
	return p
}

// moreSevereCounts 는 a 가 b 보다 심각한지다 — CRITICAL → HIGH → MEDIUM → LOW, 같으면 이름 순.
func moreSevereCounts(a, b shareddomain.SeverityCounts, aName, bName string) bool {
	ka := []int{a.Critical, a.High, a.Medium, a.Low}
	kb := []int{b.Critical, b.High, b.Medium, b.Low}
	for i := range ka {
		if ka[i] != kb[i] {
			return ka[i] > kb[i]
		}
	}
	return aName < bName
}

// SortStackPostures 는 조치할 것이 많은 스택부터 둔다 — 스캔한 스택(A → B → C → 수정판 CRITICAL 많은 순) →
// 스캔 대기 → 스캔 안 함, 같으면 이름 순.
func SortStackPostures(ps []StackSecurityPosture) {
	stateOrder := func(s string) int {
		switch s {
		case StackScanScanned:
			return 0
		case StackScanPending:
			return 1
		case StackScanNotScanned:
			return 2
		default:
			return 3
		}
	}
	val := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	fixCrit := func(p StackSecurityPosture) int {
		if p.Fixable == nil {
			return 0
		}
		return p.Fixable.Critical
	}
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if sa, sb := stateOrder(a.State), stateOrder(b.State); sa != sb {
			return sa < sb
		}
		for _, pair := range [][2]int{{val(a.GradeA), val(b.GradeA)}, {val(a.GradeB), val(b.GradeB)},
			{val(a.GradeC), val(b.GradeC)}, {fixCrit(a), fixCrit(b)}} {
			if pair[0] != pair[1] {
				return pair[0] > pair[1]
			}
		}
		if a.StackName != b.StackName {
			return a.StackName < b.StackName
		}
		return a.StackID < b.StackID
	})
}

// StackSecuritySummary 는 설치 OSS 축의 지표다(설계 §3 M5).
type StackSecuritySummary struct {
	Stacks           int
	ScannedStacks    int
	NotScannedStacks int
	// ActionImages 는 수정판 있는 CRITICAL 을 가진 이미지 수 = A + B 다.
	ActionImages int
	GradeAImages int
	GradeBImages int
}

// SummarizeStacks 는 설치 OSS 지표를 센다.
func SummarizeStacks(ps []StackSecurityPosture) StackSecuritySummary {
	var s StackSecuritySummary
	for _, p := range ps {
		s.Stacks++
		switch p.State {
		case StackScanScanned:
			s.ScannedStacks++
		case StackScanNotScanned:
			s.NotScannedStacks++
		}
		if p.GradeA != nil {
			s.GradeAImages += *p.GradeA
		}
		if p.GradeB != nil {
			s.GradeBImages += *p.GradeB
		}
	}
	s.ActionImages = s.GradeAImages + s.GradeBImages
	return s
}

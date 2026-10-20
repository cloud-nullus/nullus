package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

type appSecurityGetter interface {
	Execute(ctx context.Context, q usecase.AppSecurityQuery) (*usecase.AppSecurityView, error)
}

type stackSecurityGetter interface {
	Execute(ctx context.Context, q usecase.StackSecurityQuery) (*usecase.StackSecurityView, error)
}

// SecurityHandler 는 보안 대시보드 API 다(nullus-plan#65, 설계 §5.2).
//
// 권한별로 두 경로로 나눈다(결정 11) — 앱 축은 모든 역할, 설치 OSS 축은 admin · devops 만. /observability
// 그룹에는 역할 검사가 없어, 한 응답에 섞으면 핸들러가 필드를 지워야 하고 빠뜨리면 그대로 샌다.
type SecurityHandler struct {
	apps   appSecurityGetter
	stacks stackSecurityGetter
}

// NewSecurityHandler 는 핸들러를 만든다.
func NewSecurityHandler(apps appSecurityGetter, stacks stackSecurityGetter) *SecurityHandler {
	return &SecurityHandler{apps: apps, stacks: stacks}
}

// RegisterRoutes 는 /observability 그룹에 붙는다. stackGuards 는 설치 OSS 경로에만 붙는 역할 검사다(admin ·
// devops). 관측 모듈은 인증 모듈을 알지 못하므로 배선하는 쪽이 넘긴다 — 지금 /stacks 를 쓸 수 없는 역할이
// 이 경로로 같은 숫자를 보면 안 된다.
func (h *SecurityHandler) RegisterRoutes(g *echo.Group, stackGuards ...echo.MiddlewareFunc) {
	g.GET("/security/apps", h.GetApps)
	g.GET("/security/stacks", h.GetStacks, stackGuards...)
}

// GetApps handles GET /observability/security/apps?cluster_id=&stack_id=&q=&problem_only=.
//
// 조직은 로그인한 사용자(principal)의 것이다. summary · coverage · groups 는 범위 필터만 따르고
// problem_only 는 따르지 않는다. apps 는 위험순 그대로다.
func (h *SecurityHandler) GetApps(c echo.Context) error {
	problemOnly, _ := strconv.ParseBool(strings.TrimSpace(c.QueryParam("problem_only")))
	view, err := h.apps.Execute(c.Request().Context(), usecase.AppSecurityQuery{
		OrgID:       principalOrgID(c),
		ClusterID:   c.QueryParam("cluster_id"),
		StackID:     c.QueryParam("stack_id"),
		Query:       c.QueryParam("q"),
		ProblemOnly: problemOnly,
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "SECURITY_APPS_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, appSecurityResponse(view))
}

// GetStacks handles GET /observability/security/stacks?cluster_id=&stack_id=. admin · devops 만 닿는다.
func (h *SecurityHandler) GetStacks(c echo.Context) error {
	view, err := h.stacks.Execute(c.Request().Context(), usecase.StackSecurityQuery{
		OrgID:     principalOrgID(c),
		ClusterID: c.QueryParam("cluster_id"),
		StackID:   c.QueryParam("stack_id"),
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "SECURITY_STACKS_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, stackSecurityResponse(view))
}

// ── 앱 축 응답 ─────────────────────────────────────────────────────────────

type appSecuritySummaryJSON struct {
	Apps              int `json:"apps"`
	WithSecurityStage int `json:"with_security_stage"`
	Blocked           int `json:"blocked"`
	Secrets           int `json:"secrets"`
	Critical          int `json:"critical"`
	Undetermined      int `json:"undetermined"`
}

type stageCoverageJSON struct {
	SecretScan     int `json:"secret_scan"`
	DependencyScan int `json:"dependency_scan"`
	SAST           int `json:"sast"`
	ImageScan      int `json:"image_scan"`
}

type appSecurityGroupJSON struct {
	StackID      string `json:"stack_id"`
	StackName    string `json:"stack_name"`
	ClusterID    string `json:"cluster_id"`
	ClusterName  string `json:"cluster_name"`
	Apps         int    `json:"apps"`
	Blocked      int    `json:"blocked"`
	Secrets      int    `json:"secrets"`
	Critical     int    `json:"critical"`
	Undetermined int    `json:"undetermined"`
	Problems     int    `json:"problems"`
}

type appRunJSON struct {
	DeploymentID string    `json:"deployment_id"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
}

type appSecurityJSON struct {
	PipelineID string                    `json:"pipeline_id"`
	Name       string                    `json:"name"`
	StackID    string                    `json:"stack_id"`
	Rank       int                       `json:"rank"`
	LatestRun  *appRunJSON               `json:"latest_run"`
	Cells      map[string]map[string]any `json:"cells"`
}

type appSecurityResponseJSON struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Truncated   bool                   `json:"truncated"`
	Summary     appSecuritySummaryJSON `json:"summary"`
	Coverage    stageCoverageJSON      `json:"coverage"`
	Groups      []appSecurityGroupJSON `json:"groups"`
	Apps        []appSecurityJSON      `json:"apps"`
}

func appSecurityResponse(v *usecase.AppSecurityView) appSecurityResponseJSON {
	out := appSecurityResponseJSON{
		GeneratedAt: v.GeneratedAt.UTC(),
		Truncated:   v.Truncated,
		Summary: appSecuritySummaryJSON{
			Apps: v.Summary.Apps, WithSecurityStage: v.Summary.WithSecurityStage, Blocked: v.Summary.Blocked,
			Secrets: v.Summary.Secrets, Critical: v.Summary.Critical, Undetermined: v.Summary.Undetermined,
		},
		Coverage: stageCoverageJSON{
			SecretScan: v.Coverage.SecretScan, DependencyScan: v.Coverage.DependencyScan,
			SAST: v.Coverage.SAST, ImageScan: v.Coverage.ImageScan,
		},
		Groups: make([]appSecurityGroupJSON, 0, len(v.Groups)),
		Apps:   make([]appSecurityJSON, 0, len(v.Apps)),
	}
	for _, g := range v.Groups {
		out.Groups = append(out.Groups, appSecurityGroupJSON{
			StackID: g.StackID, StackName: g.StackName, ClusterID: g.ClusterID, ClusterName: g.ClusterName,
			Apps: g.Apps, Blocked: g.Blocked, Secrets: g.Secrets, Critical: g.Critical,
			Undetermined: g.Undetermined, Problems: g.Problems,
		})
	}
	for _, p := range v.Apps {
		app := appSecurityJSON{
			PipelineID: p.App.PipelineID, Name: p.App.Name, StackID: p.App.StackID, Rank: p.Rank,
			Cells: make(map[string]map[string]any, len(domain.SecurityStages)),
		}
		if run := p.App.LatestRun; run != nil {
			app.LatestRun = &appRunJSON{DeploymentID: run.DeploymentID, Version: run.Version, Status: run.Status, StartedAt: run.StartedAt.UTC()}
		}
		for _, stage := range domain.SecurityStages {
			app.Cells[string(stage)] = cellJSON(stage, p.Cells[stage])
		}
		out.Apps = append(out.Apps, app)
	}
	return out
}

// cellJSON 은 셀 하나다. 결과가 없는 셀(unrecorded · not_applied)은 state 만 있다. 숫자 칸은 모르면 빠진다 —
// 0 으로 채우지 않는다. 목록 칸(rules · failed_conditions)은 결과가 있으면 늘 배열이다.
func cellJSON(stage domain.SecurityStage, cell domain.SecurityCell) map[string]any {
	out := map[string]any{"state": string(cell.State)}
	r := cell.Record
	if r == nil {
		return out
	}
	if r.DeploymentID != "" {
		out["deployment_id"] = r.DeploymentID
	}
	if r.Version != "" {
		out["version"] = r.Version
	}
	if !r.At.IsZero() {
		out["at"] = r.At.UTC()
	}
	out["stale_run"] = cell.StaleRun
	if r.Link != "" {
		out["link"] = r.Link
	}
	switch stage {
	case domain.StageSecretScan:
		if r.Findings != nil {
			out["findings"] = *r.Findings
		}
		rules := make([]map[string]any, 0, len(r.Rules))
		for _, rule := range r.Rules {
			rules = append(rules, map[string]any{"rule_id": rule.RuleID, "count": rule.Count})
		}
		out["rules"] = rules
	case domain.StageDependencyScan:
		if r.Targets != nil {
			out["targets"] = *r.Targets
		}
		addScanCounts(out, r, cell)
	case domain.StageSAST:
		if r.Vulnerabilities != nil {
			out["vulnerabilities"] = *r.Vulnerabilities
		}
		if r.SecurityHotspots != nil {
			out["security_hotspots"] = *r.SecurityHotspots
		}
		failed := r.FailedConditions
		if failed == nil {
			failed = []string{}
		}
		out["failed_conditions"] = failed
	case domain.StageImageScan:
		if r.Image != "" {
			out["image"] = r.Image
		}
		addScanCounts(out, r, cell)
	}
	return out
}

// addScanCounts 는 의존성 · 이미지 셀의 건수와 DB 신선도다. 리포트를 읽지 못했으면 셋 다 빠진다.
func addScanCounts(out map[string]any, r *domain.ScanRecord, cell domain.SecurityCell) {
	if r.Counts != nil {
		out["counts"] = *r.Counts
	}
	if cell.DBStale != nil {
		out["db_stale"] = *cell.DBStale
	}
	if r.DBUpdatedAt != nil && !r.DBUpdatedAt.IsZero() {
		out["db_updated_at"] = r.DBUpdatedAt.UTC()
	}
}

// ── 설치 OSS 축 응답 ───────────────────────────────────────────────────────

type stackSecuritySummaryJSON struct {
	Stacks           int `json:"stacks"`
	ScannedStacks    int `json:"scanned_stacks"`
	NotScannedStacks int `json:"not_scanned_stacks"`
	ActionImages     int `json:"action_images"`
	GradeAImages     int `json:"grade_a_images"`
	GradeBImages     int `json:"grade_b_images"`
}

type worstImageJSON struct {
	Image    string `json:"image"`
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	Exposed  bool   `json:"exposed"`
}

// stackSecurityJSON 은 스택 한 행이다. 스캔하지 않은 스택의 숫자 칸은 null 이다 — "스캔 안 함" 을 0 으로
// 보이지 않는다(설계 §4.4).
type stackSecurityJSON struct {
	StackID      string                       `json:"stack_id"`
	StackName    string                       `json:"stack_name"`
	ClusterID    string                       `json:"cluster_id"`
	ClusterName  string                       `json:"cluster_name"`
	State        string                       `json:"state"`
	Reason       string                       `json:"reason"`
	Airgap       bool                         `json:"airgap"`
	Images       *int                         `json:"images"`
	FailedImages *int                         `json:"failed_images"`
	SharedImages *int                         `json:"shared_images"`
	GradeAImages *int                         `json:"grade_a_images"`
	GradeBImages *int                         `json:"grade_b_images"`
	GradeCImages *int                         `json:"grade_c_images"`
	Fixable      *shareddomain.SeverityCounts `json:"fixable"`
	WorstImage   *worstImageJSON              `json:"worst_image"`
	ScannedAt    *time.Time                   `json:"scanned_at"`
	DBUpdatedAt  *time.Time                   `json:"db_updated_at"`
	DBStale      *bool                        `json:"db_stale"`
}

type stackSecurityResponseJSON struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Summary     stackSecuritySummaryJSON `json:"summary"`
	Stacks      []stackSecurityJSON      `json:"stacks"`
}

func stackSecurityResponse(v *usecase.StackSecurityView) stackSecurityResponseJSON {
	out := stackSecurityResponseJSON{
		GeneratedAt: v.GeneratedAt.UTC(),
		Summary: stackSecuritySummaryJSON{
			Stacks: v.Summary.Stacks, ScannedStacks: v.Summary.ScannedStacks, NotScannedStacks: v.Summary.NotScannedStacks,
			ActionImages: v.Summary.ActionImages, GradeAImages: v.Summary.GradeAImages, GradeBImages: v.Summary.GradeBImages,
		},
		Stacks: make([]stackSecurityJSON, 0, len(v.Stacks)),
	}
	for _, p := range v.Stacks {
		row := stackSecurityJSON{
			StackID: p.StackID, StackName: p.StackName, ClusterID: p.ClusterID, ClusterName: p.ClusterName,
			State: p.State, Reason: p.Reason, Airgap: p.Airgap,
			Images: p.Images, FailedImages: p.FailedImages, SharedImages: p.SharedImages,
			GradeAImages: p.GradeA, GradeBImages: p.GradeB, GradeCImages: p.GradeC,
			Fixable: p.Fixable, ScannedAt: utcPtr(p.ScannedAt), DBUpdatedAt: utcPtr(p.DBUpdatedAt), DBStale: p.DBStale,
		}
		if w := p.WorstImage; w != nil {
			row.WorstImage = &worstImageJSON{Image: w.Image, Critical: w.Critical, High: w.High, Exposed: w.Exposed}
		}
		out.Stacks = append(out.Stacks, row)
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

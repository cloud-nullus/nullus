package bridge

import (
	"context"
	"fmt"
	"strings"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	cicddomain "github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

// 보안 대시보드(#65)의 입력을 cicd · stack · admin 의 공개 경로에서 읽어 관측 도메인 타입으로 옮긴다. 결과
// 테이블은 그 모듈들이 소유한다 — 여기서 테이블을 읽지 않는다(설계 결정 10).

// PipelineSecurityLister 는 cicd 모듈의 공개 경로다(usecase.ListPipelineSecurity).
type PipelineSecurityLister interface {
	Execute(ctx context.Context, orgID string) (*cicddomain.PipelineSecurityList, error)
}

// AppSecuritySource 는 port.AppSecuritySource 다.
type AppSecuritySource struct {
	pipelines PipelineSecurityLister
}

// NewAppSecuritySource 는 앱 축 입력을 만든다.
func NewAppSecuritySource(pipelines PipelineSecurityLister) *AppSecuritySource {
	return &AppSecuritySource{pipelines: pipelines}
}

var _ port.AppSecuritySource = (*AppSecuritySource)(nil)

// ListAppSecurity 는 조직 파이프라인의 단계별 최신 보안 결과를 앱 입력으로 옮긴다.
func (s *AppSecuritySource) ListAppSecurity(ctx context.Context, orgID string) ([]domain.AppSecurityRecord, bool, error) {
	if s == nil || s.pipelines == nil {
		return nil, false, fmt.Errorf("파이프라인 보안 결과 경로가 배선되지 않았습니다")
	}
	list, err := s.pipelines.Execute(ctx, orgID)
	if err != nil {
		return nil, false, err
	}
	if list == nil {
		return []domain.AppSecurityRecord{}, false, nil
	}
	out := make([]domain.AppSecurityRecord, 0, len(list.Pipelines))
	for _, snap := range list.Pipelines {
		if snap != nil {
			out = append(out, appRecordFrom(snap))
		}
	}
	return out, list.Truncated, nil
}

// appRecordFrom 은 cicd 의 스냅숏 하나를 앱 입력으로 옮긴다.
func appRecordFrom(snap *cicddomain.PipelineSecuritySnapshot) domain.AppSecurityRecord {
	rec := domain.AppSecurityRecord{
		PipelineID:  snap.PipelineID,
		Name:        snap.Name,
		StackID:     snap.StackID,
		ClusterID:   snap.ClusterID,
		StagesKnown: len(snap.Stages) > 0,
		HasStage: map[domain.SecurityStage]bool{
			domain.StageSecretScan:     snap.HasSecretScan,
			domain.StageDependencyScan: snap.HasDependencyScan,
			domain.StageSAST:           snap.HasSAST,
			domain.StageImageScan:      snap.HasImageScan,
		},
		// 스택이 없으면 정책도 없다 — 단계 유무로만 본다.
		SecretPolicyOff: snap.StackID != "" && snap.SecretDetection == cicddomain.SecretDetectionOff,
		Records:         map[domain.SecurityStage]*domain.ScanRecord{},
	}
	if run := snap.LatestRun; run != nil {
		rec.LatestRun = &domain.AppRun{
			DeploymentID: run.DeploymentID, Version: run.Version, Status: string(run.Status), StartedAt: run.StartedAt,
		}
	}
	version := func(deploymentID string) string {
		if v := snap.RunVersions[deploymentID]; v != "" {
			return v
		}
		if snap.LatestRun != nil && snap.LatestRun.DeploymentID == deploymentID {
			return snap.LatestRun.Version
		}
		return ""
	}

	if r := snap.SecretScan; r != nil {
		rules := make([]domain.SecretRule, 0, len(r.Rules))
		for _, rule := range r.Rules {
			rules = append(rules, domain.SecretRule{RuleID: rule.RuleID, Count: rule.Count})
		}
		rec.Records[domain.StageSecretScan] = &domain.ScanRecord{
			Gate: string(r.GateResult), DeploymentID: r.DeploymentID, Version: version(r.DeploymentID),
			At: r.ScannedAt, Link: r.ReportURI, Findings: r.FindingsCount, Rules: rules,
		}
	}
	if r := snap.DependencyScan; r != nil {
		rec.Records[domain.StageDependencyScan] = &domain.ScanRecord{
			Gate: string(r.GateResult), DeploymentID: r.DeploymentID, Version: version(r.DeploymentID),
			At: r.ScannedAt, Link: r.ReportURI, Counts: r.Counts, Targets: r.Targets, DBUpdatedAt: r.DBUpdatedAt,
		}
	}
	if r := snap.SAST; r != nil {
		sast := &domain.ScanRecord{
			Gate: string(r.GateResult), DeploymentID: r.DeploymentID, Version: version(r.DeploymentID),
			At: r.AnalyzedAt, Link: r.DashboardURL, FailedConditions: []string{},
		}
		if m := r.Metrics; m != nil {
			sast.Vulnerabilities, sast.SecurityHotspots = m.Vulnerabilities, m.SecurityHotspots
		}
		for _, c := range r.Conditions {
			if strings.EqualFold(c.Status, "ERROR") {
				sast.FailedConditions = append(sast.FailedConditions, c.Metric)
			}
		}
		rec.Records[domain.StageSAST] = sast
	}
	if r := snap.ImageScan; r != nil {
		rec.Records[domain.StageImageScan] = &domain.ScanRecord{
			Gate: string(r.GateResult), DeploymentID: r.DeploymentID, Version: version(r.DeploymentID),
			At: r.ScannedAt, Link: r.ReportURI, Counts: r.Counts, DBUpdatedAt: r.DBUpdatedAt,
			Image: imageRef(r.ImageRepository, r.ImageTag, r.ImageDigest),
		}
	}
	return rec
}

// imageRef 는 사람이 읽는 이미지 표기다. 결과는 다이제스트에 붙으므로 있으면 다이제스트로 적는다.
func imageRef(repo, tag, digest string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	if d := strings.TrimSpace(digest); d != "" {
		return repo + "@" + d
	}
	if t := strings.TrimSpace(tag); t != "" {
		return repo + ":" + t
	}
	return repo
}

// OrgStackImageReporter 는 stack 모듈의 공개 경로다(usecase.ScanStackImages.ReportsForOrg).
type OrgStackImageReporter interface {
	ReportsForOrg(ctx context.Context, orgID string) ([]stackdomain.OrgStackImageScan, error)
}

// StackSecuritySource 는 port.StackSecuritySource 다.
type StackSecuritySource struct {
	reports OrgStackImageReporter
}

// NewStackSecuritySource 는 설치 OSS 축 입력을 만든다.
func NewStackSecuritySource(reports OrgStackImageReporter) *StackSecuritySource {
	return &StackSecuritySource{reports: reports}
}

var _ port.StackSecuritySource = (*StackSecuritySource)(nil)

// ListStackImageSecurity 는 조직 스택의 설치 이미지 보고서를 옮긴다.
func (s *StackSecuritySource) ListStackImageSecurity(ctx context.Context, orgID string) ([]domain.StackImageInput, error) {
	if s == nil || s.reports == nil {
		return nil, fmt.Errorf("스택 설치 이미지 보고서 경로가 배선되지 않았습니다")
	}
	reports, err := s.reports.ReportsForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StackImageInput, 0, len(reports))
	for _, r := range reports {
		if r.Stack == nil {
			continue
		}
		in := domain.StackImageInput{
			StackID: r.Stack.ID, StackName: r.Stack.Name, ClusterID: r.Stack.ClusterID,
			State: string(r.Report.Status), Reason: string(r.Report.Reason), Airgap: r.Report.Airgap,
			ScannedAt: r.Report.LastScannedAt,
			Images:    make([]domain.StackImage, 0, len(r.Report.Items)),
		}
		for _, img := range r.Report.Items {
			in.Images = append(in.Images, domain.StackImage{
				Image: img.Image, Shared: img.Shared, Exposed: img.Exposed,
				Scanned: img.Status == stackdomain.ImageScanStatusScanned,
				Counts:  img.Counts, Fixable: img.FixableCounts, DBUpdatedAt: img.DBUpdatedAt,
			})
		}
		out = append(out, in)
	}
	return out, nil
}

// OrgStackLister 는 stack 모듈의 스택 목록 창구다(port.StackRepository 의 List).
type OrgStackLister interface {
	List(ctx context.Context, orgID string, includeDeleted bool) ([]*stackdomain.Stack, error)
}

// OrgClusterLister 는 admin 모듈의 클러스터 목록 창구다(port.ClusterRepository 의 List).
type OrgClusterLister interface {
	List(ctx context.Context, orgID string) ([]*admindomain.Cluster, error)
}

// SecurityScope 는 port.ScopeResolver 다.
type SecurityScope struct {
	stacks   OrgStackLister
	clusters OrgClusterLister
}

// NewSecurityScope 는 범위 필터 · 묶음 머리의 이름 조회를 만든다.
func NewSecurityScope(stacks OrgStackLister, clusters OrgClusterLister) *SecurityScope {
	return &SecurityScope{stacks: stacks, clusters: clusters}
}

var _ port.ScopeResolver = (*SecurityScope)(nil)

// ListStacks 는 조직의 지우지 않은 스택이다.
func (s *SecurityScope) ListStacks(ctx context.Context, orgID string) ([]domain.SecurityStackRef, error) {
	if s == nil || s.stacks == nil {
		return nil, fmt.Errorf("스택 저장소가 배선되지 않았습니다")
	}
	stacks, err := s.stacks.List(ctx, orgID, false)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SecurityStackRef, 0, len(stacks))
	for _, st := range stacks {
		if st != nil {
			out = append(out, domain.SecurityStackRef{ID: st.ID, Name: st.Name, ClusterID: st.ClusterID})
		}
	}
	return out, nil
}

// ClusterNames 는 조직 클러스터의 ID → 이름이다.
func (s *SecurityScope) ClusterNames(ctx context.Context, orgID string) (map[string]string, error) {
	if s == nil || s.clusters == nil {
		return nil, fmt.Errorf("클러스터 저장소가 배선되지 않았습니다")
	}
	clusters, err := s.clusters.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(clusters))
	for _, c := range clusters {
		if c != nil {
			out[c.ID] = c.Name
		}
	}
	return out, nil
}

package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// SyncPipelineRuns 는 CI 서버의 빌드 이력을 배포 기록으로 들인다.
//
// GitOps 경로에서는 플랫폼이 배포를 실행하지 않는다 — CI 가 빌드하고 Argo CD 가
// 동기화한다. 그래서 실행 기록이 CI 서버에만 있고, 들이지 않으면 빌드가
// 성공해도 화면의 실행 통계가 영원히 0 으로 남는다.
//
// 멱등하다. 배포 ID 를 job 과 빌드 번호에서 만들어, 같은 빌드를 여러 번 동기화해도
// 기록이 늘지 않고 상태만 갱신된다(실행 중 → 성공).
type SyncPipelineRuns struct {
	syncable    port.SyncablePipelineLister
	builds      port.CIBuildReader
	deployments port.DeploymentRepository
	// factory / pipelines 는 파이프라인마다 CI 서버를 찾을 때 쓴다.
	factory   port.SCMBundleFactory
	pipelines port.PipelineRepository
	// imageScans 는 스캔 단계의 게이트 판정을 남긴다. nil 이면 남기지 않는다 —
	// 스캔 결과 저장이 배선되지 않은 구성에서도 실행 기록 동기화는 돌아야 한다.
	imageScans port.ImageScanResultRepository
	// artifacts 는 스캔 리포트를 읽는다. nil 이면 단계 결과만으로 판정한다.
	artifacts port.CIArtifactReader
	// policies 는 파이프라인이 속한 스택의 스캔 정책을 찾는다. nil 이면 기본 정책이다.
	policies port.ScanPolicyRepository
}

const (
	// defaultRunBranch 는 실행 기록을 읽을 브랜치다.
	defaultRunBranch = "main"
	// runSyncLimit 는 한 번에 들이는 최근 빌드 수다.
	runSyncLimit = 30
)

// NewSyncPipelineRuns 는 SyncPipelineRuns 를 만든다.
func NewSyncPipelineRuns(builds port.CIBuildReader, deployments port.DeploymentRepository) *SyncPipelineRuns {
	return &SyncPipelineRuns{builds: builds, deployments: deployments}
}

// SyncPipelineRunsInput 은 어느 job 의 어느 브랜치를 들일지다.
type SyncPipelineRunsInput struct {
	PipelineID string
	JobName    string
	Branch     string
	Limit      int
	// Policy 는 판정에 쓸 스택 정책이다. nil 이면 기본 정책이다 — CI 는 푸시된 정책으로
	// 막으므로, 동기화도 같은 정책으로 읽어야 차단과 스캔 오류를 바르게 가른다.
	Policy *domain.ScanPolicy
}

// runDeploymentID 는 빌드 하나에 대응하는 배포 기록 ID 다.
//
// 빌드 번호에서 만들어 재동기화가 기록을 늘리지 않게 한다.
func runDeploymentID(pipelineID string, buildNumber int) string {
	return fmt.Sprintf("dep_ci_%s_%d", strings.TrimSpace(pipelineID), buildNumber)
}

// Execute 는 최근 빌드를 읽어 배포 기록으로 반영한다.
func (uc *SyncPipelineRuns) Execute(ctx context.Context, input SyncPipelineRunsInput) (int, error) {
	if uc == nil || uc.builds == nil || uc.deployments == nil {
		return 0, nil
	}
	pipelineID := strings.TrimSpace(input.PipelineID)
	if pipelineID == "" {
		return 0, fmt.Errorf("pipeline_id 가 필요합니다")
	}

	builds, err := uc.builds.ListBuilds(ctx, input.JobName, input.Branch, input.Limit)
	if err != nil {
		return 0, fmt.Errorf("CI 빌드 이력 조회 실패 (%s): %w", input.JobName, err)
	}

	synced := 0
	for _, b := range builds {
		deployment := deploymentFromBuild(pipelineID, b)

		// 이미 있으면 갱신한다 — 실행 중이던 빌드가 끝나면 상태가 바뀐다.
		existing, getErr := uc.deployments.GetByID(ctx, deployment.ID)
		if getErr == nil && existing != nil {
			// 상태가 같아도 단계 정보가 새로 생겼으면 갱신한다 — CI 에 단계
			// 플러그인을 나중에 깔면 이미 끝난 실행에 단계가 붙는다. 상태만
			// 보고 건너뛰면 그 실행은 영원히 "실행 정보 없음" 으로 남는다.
			if existing.Status == deployment.Status &&
				len(existing.Steps) == len(deployment.Steps) {
				continue
			}
			if err := uc.deployments.Update(ctx, deployment); err != nil {
				return synced, fmt.Errorf("배포 기록 갱신 실패 (%s): %w", deployment.ID, err)
			}
			synced++
			continue
		}

		if err := uc.deployments.Create(ctx, deployment); err != nil {
			return synced, fmt.Errorf("배포 기록 생성 실패 (%s): %w", deployment.ID, err)
		}
		synced++
	}

	uc.recordImageScans(ctx, input, pipelineID, builds)
	return synced, nil
}

// imageScanStageKey 는 스캐폴딩이 만드는 스캔 단계의 비교 키다.
//
// CI 마다 표기가 다르다 — Jenkins 는 ImageScan, GitLab·GitHub 은 잡 키 image-scan.
// port.StageKey 로 맞춰 비교한다.
var imageScanStageKey = port.StageKey("ImageScan")

// recordImageScans 는 스캔 단계가 끝난 실행의 게이트 판정을 남긴다.
//
// 산출물 조회가 배선돼 있으면 CI 가 남긴 리포트로 건수·다이제스트·DB 시각을
// 채우고 판정을 가다듬는다. 리포트를 못 읽으면 건수는 비워 둔다 — 0 으로 채우면
// "취약점 0건" 으로 읽힌다.
//
// 기록 실패로 실행 기록 동기화를 실패시키지 않는다. 대시보드 입력이 한 번 빠지는
// 것보다, 배포 이력 화면이 통째로 멈추는 것이 더 나쁘다.
func (uc *SyncPipelineRuns) recordImageScans(
	ctx context.Context,
	input SyncPipelineRunsInput,
	pipelineID string,
	builds []port.CIBuild,
) {
	if uc.imageScans == nil {
		return
	}
	withReport := uc.scansWithReport(ctx, pipelineID)
	policy := domain.DefaultScanPolicy()
	if input.Policy != nil {
		policy = *input.Policy
	}

	for _, b := range builds {
		for _, st := range b.Stages {
			if port.StageKey(st.Name) != imageScanStageKey {
				continue
			}
			deploymentID := runDeploymentID(pipelineID, b.Number)
			// 실행 하나에 스캔 하나다. 같은 실행을 다시 동기화해도 기록이 늘지 않는다.
			id := "scan_" + deploymentID

			if st.Status == port.CIStageCanceled {
				// 취소는 스캔 실패가 아니다 — 새 커밋이 앞선 실행을 밀어낸 것이다.
				// 판정을 남기지 않고, 이 규칙이 생기기 전에 error 로 남긴 기록은
				// 걷어낸다. 남겨 두면 대시보드가 스캐너 장애로 센다.
				if err := uc.imageScans.Delete(ctx, id); err != nil {
					slog.Warn("취소된 실행의 스캔 기록 정리 실패",
						"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
				}
				continue
			}
			gate, ok := domain.GateResultFromStageStatus(string(st.Status))
			if !ok {
				continue // 도는 중인 것을 통과로 적지 않는다
			}
			ref := port.CIArtifactRef{
				JobName: input.JobName,
				Branch:  input.Branch,
				Build:   b,
				Stage:   st,
				Name:    port.ImageScanReportArtifact,
				Path:    port.ImageScanReportFile,
			}
			if existing := withReport[id]; existing != nil {
				// 이미 리포트까지 읽었다. 화면을 열 때마다 동기화가 도는데,
				// 끝난 실행의 리포트를 매번 다시 내려받을 이유가 없다. 링크·리포트 위치
				// 기능 전에 기록한 실행이면 그것만 채운다.
				changed := false
				if existing.ReportURI == "" {
					if link := uc.reportLink(ref); link != "" {
						existing.ReportURI = link
						changed = true
					}
				}
				if existing.ReportRef == nil {
					existing.ReportRef = reportRefFrom(ref)
					changed = true
				}
				if changed {
					if err := uc.imageScans.Upsert(ctx, existing); err != nil {
						slog.Warn("이미지 스캔 리포트 위치 기록 실패",
							"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
					}
				}
				continue
			}

			scannedAt := st.StartedAt
			if scannedAt.IsZero() {
				scannedAt = b.StartedAt
			}
			result := &domain.ImageScanResult{
				ID:           id,
				PipelineID:   pipelineID,
				DeploymentID: deploymentID,
				ScanSource:   string(port.ScanSourceCentral),
				Scanner:      "trivy",
				GateResult:   gate,
				ScannedAt:    scannedAt,
			}
			uc.applyScanReport(ctx, result, ref, policy)
			// 리포트를 읽은 실행에만 링크를 건다 — 리포트가 없으면 열어도 없는 파일이다.
			if result.Counts != nil {
				result.ReportURI = uc.reportLink(ref)
				result.ReportRef = reportRefFrom(ref)
			}

			if err := uc.imageScans.Upsert(ctx, result); err != nil {
				slog.Warn("이미지 스캔 결과 기록 실패",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
			}
		}
	}
}

// scansWithReport 는 이미 리포트를 읽어 건수가 채워진 스캔 기록의 ID 다.
//
// 조회에 실패하면 비어 있다 — 리포트를 한 번 더 내려받는 편이 기록을 빠뜨리는
// 것보다 낫다. 리포트가 없던 실행은 여기 들지 않아 다음 동기화 때 다시 확인한다.
func (uc *SyncPipelineRuns) scansWithReport(ctx context.Context, pipelineID string) map[string]*domain.ImageScanResult {
	if uc.artifacts == nil {
		return nil
	}
	existing, err := uc.imageScans.ListByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil
	}
	out := make(map[string]*domain.ImageScanResult, len(existing))
	for _, r := range existing {
		if r != nil && r.Counts != nil {
			out[r.ID] = r
		}
	}
	return out
}

// reportLink 는 리포트를 브라우저로 여는 주소다. 링크를 만들 수 없는 조회기면 비어 있다.
func (uc *SyncPipelineRuns) reportLink(ref port.CIArtifactRef) string {
	linker, ok := uc.artifacts.(port.CIArtifactLinker)
	if !ok {
		return ""
	}
	return strings.TrimSpace(linker.ArtifactWebURL(ref))
}

// applyScanReport 는 리포트를 읽어 판정과 건수를 채운다.
func (uc *SyncPipelineRuns) applyScanReport(
	ctx context.Context,
	result *domain.ImageScanResult,
	ref port.CIArtifactRef,
	policy domain.ScanPolicy,
) {
	if uc.artifacts == nil {
		return
	}
	status := string(ref.Stage.Status)

	raw, found, err := uc.artifacts.ReadArtifact(ctx, ref)
	if err != nil {
		// 모르는 것이다. 단계 결과로 남기고 다음 동기화 때 다시 읽는다.
		slog.Warn("이미지 스캔 리포트를 읽지 못했습니다",
			"pipeline_id", result.PipelineID, "deployment_id", result.DeploymentID, "error", err)
		return
	}
	if !found {
		if gate, ok := domain.GateWhenReportMissing(status); ok {
			result.GateResult = gate
		}
		return
	}

	summary, err := domain.ParseTrivyReport(raw)
	if err != nil {
		slog.Warn("이미지 스캔 리포트 형식이 맞지 않습니다",
			"pipeline_id", result.PipelineID, "deployment_id", result.DeploymentID, "error", err)
		return
	}
	if gate, ok := domain.GateFromStageAndReport(status, summary, policy); ok {
		result.GateResult = gate
	}
	// 건수는 판정이 본 것과 같아야 한다 — 수정본 없는 것을 뺐다면 건수에서도 뺀다.
	counts := summary.All
	if policy.IgnoreUnfixed {
		counts = summary.Fixable
	}
	result.Counts = &counts
	result.ImageRepository = summary.ImageRepository
	result.ImageTag = summary.ImageTag
	result.ImageDigest = summary.ImageDigest
	result.ScannerVersion = summary.ScannerVersion
	result.DBUpdatedAt = summary.DBUpdatedAt
}

// WithImageScans 는 스캔 게이트 판정을 남기도록 배선한다.
func (uc *SyncPipelineRuns) WithImageScans(repo port.ImageScanResultRepository) *SyncPipelineRuns {
	uc.imageScans = repo
	return uc
}

// WithArtifacts 는 스캔 리포트를 읽어 건수를 채우도록 배선한다.
func (uc *SyncPipelineRuns) WithArtifacts(reader port.CIArtifactReader) *SyncPipelineRuns {
	uc.artifacts = reader
	return uc
}

// WithScanPolicies 는 파이프라인의 스택 정책으로 판정하도록 배선한다.
func (uc *SyncPipelineRuns) WithScanPolicies(repo port.ScanPolicyRepository) *SyncPipelineRuns {
	uc.policies = repo
	return uc
}

// deploymentFromBuild 는 CI 빌드를 배포 기록으로 옮긴다.
func deploymentFromBuild(pipelineID string, b port.CIBuild) *domain.Deployment {
	deployment := &domain.Deployment{
		ID:         runDeploymentID(pipelineID, b.Number),
		PipelineID: pipelineID,
		Version:    fmt.Sprintf("#%d", b.Number),
		Status:     deploymentStatusFromBuild(b),
		StartedAt:  b.StartedAt,
		// 사람이 아니라 CI 가 실행했다는 사실을 남긴다.
		DeployedBy: "ci",
	}

	// 실행 중인 빌드는 완료 시각이 없다. 0 값을 넣으면 화면이 1970 년을 보여준다.
	if !b.Building && b.Duration > 0 {
		completed := b.StartedAt.Add(b.Duration)
		deployment.CompletedAt = &completed
	}

	deployment.Steps = stepsFromStages(b.Stages)
	return deployment
}

// stepsFromStages 는 정규화된 단계를 도메인 스텝으로 옮긴다.
//
// CI 별 어휘는 이미 어댑터가 정규화했다. 여기서 CI 종류를 알 필요가 없다 —
// OSS 를 하나 늘려도 이 함수는 그대로다.
//
// 단계가 없으면 nil 이다. 빈 목록과 "모두 성공" 은 다르고, 화면은 그 차이를
// "실행 정보 없음" 으로 표시한다.
func stepsFromStages(stages []port.CIStage) []domain.DeployStep {
	if len(stages) == 0 {
		return nil
	}

	steps := make([]domain.DeployStep, 0, len(stages))
	for _, st := range stages {
		step := domain.DeployStep{
			Name:   st.Name,
			Status: string(st.Status),
			// 플랫폼이 직접 적용한 리소스가 아니라 CI 가 실행한 단계다.
			Kind: "ci_stage",
		}
		if !st.StartedAt.IsZero() {
			step.AppliedAt = st.StartedAt.UTC().Format(time.RFC3339)
		}
		steps = append(steps, step)
	}
	return steps
}

func deploymentStatusFromBuild(b port.CIBuild) domain.DeploymentStatus {
	if b.Building {
		return domain.DeploymentStatusRunning
	}
	switch strings.ToUpper(strings.TrimSpace(b.Result)) {
	case "SUCCESS":
		return domain.DeploymentStatusSuccess
	case "":
		// 결과가 없고 실행 중도 아니면 아직 시작 전이다.
		return domain.DeploymentStatusRunning
	default:
		// FAILURE / ABORTED / UNSTABLE 은 모두 실패로 다룬다 — 화면은 성공 여부만
		// 구분하고, 세부 사유는 CI 로그가 갖고 있다.
		return domain.DeploymentStatusFailed
	}
}

// WithBundleFactory 는 파이프라인마다 CI 서버를 찾아 쓰도록 배선한다.
//
// CI 서버는 스택마다 따로 서므로 기동 시점에 하나로 고정할 수 없다.
func (uc *SyncPipelineRuns) WithBundleFactory(
	factory port.SCMBundleFactory,
	pipelines port.PipelineRepository,
) *SyncPipelineRuns {
	uc.factory = factory
	uc.pipelines = pipelines
	return uc
}

// ForPipeline 은 파이프라인 하나의 실행 기록을 들인다.
//
// CI 가 배선되지 않은 스택(Jenkins 자격증명을 못 읽은 경우 등)은 경고만 남기고
// 건너뛴다 — 실행 기록 조회 때문에 화면 조회 자체를 실패시키지 않는다.
func (uc *SyncPipelineRuns) ForPipeline(ctx context.Context, pipelineID string) (int, error) {
	if uc == nil || uc.factory == nil || uc.pipelines == nil {
		return 0, nil
	}
	pipeline, err := uc.pipelines.GetByID(ctx, strings.TrimSpace(pipelineID))
	if err != nil || pipeline == nil {
		return 0, err
	}
	if strings.TrimSpace(pipeline.StackID) == "" {
		return 0, nil
	}

	bundle, err := uc.factory.For(ctx, pipeline.StackID)
	if err != nil {
		// 스택이 아직 준비되지 않았을 수 있다. 조회 자체를 실패시키지는 않되,
		// 조용히 삼키면 통계가 왜 비는지 알 수 없으므로 남긴다.
		slog.Warn("CI 실행 기록: 스택 번들을 만들지 못했습니다",
			"pipeline_id", pipeline.ID, "stack_id", pipeline.StackID, "error", err)
		return 0, nil
	}
	return uc.syncWithBundle(ctx, pipeline, bundle)
}

// WithSyncablePipelines 는 주기 동기화가 돌 파이프라인 목록을 배선한다.
func (uc *SyncPipelineRuns) WithSyncablePipelines(lister port.SyncablePipelineLister) *SyncPipelineRuns {
	uc.syncable = lister
	return uc
}

// SyncAll 은 스택에 묶인 모든 파이프라인의 실행 기록과 스캔 결과를 들이고, 동기화한
// 파이프라인 수를 돌려준다.
//
// 화면을 열 때만 들이면 아무도 보지 않는 파이프라인의 스캔 결과가 쌓이지 않는다.
// 번들은 스택마다 한 번만 만든다 — 조립마다 SCM 인증 확인이 따른다. 한 스택의
// 실패로 나머지를 멈추지 않는다.
func (uc *SyncPipelineRuns) SyncAll(ctx context.Context) (int, error) {
	if uc == nil || uc.factory == nil || uc.syncable == nil {
		return 0, nil
	}
	pipelines, err := uc.syncable.ListWithStack(ctx)
	if err != nil {
		return 0, fmt.Errorf("동기화할 파이프라인 목록 조회 실패: %w", err)
	}

	byStack := map[string][]*domain.Pipeline{}
	var stacks []string
	for _, p := range pipelines {
		if p == nil || strings.TrimSpace(p.StackID) == "" {
			continue
		}
		if _, seen := byStack[p.StackID]; !seen {
			stacks = append(stacks, p.StackID)
		}
		byStack[p.StackID] = append(byStack[p.StackID], p)
	}

	synced := 0
	for _, stackID := range stacks {
		if ctx.Err() != nil {
			break
		}
		bundle, err := uc.factory.For(ctx, stackID)
		if err != nil {
			if errors.Is(err, port.ErrStackToolsUnavailable) {
				// 설치 중이거나 사라진 스택이다. 주기마다 경고로 쌓지 않는다.
				slog.Debug("CI 실행 기록 주기 동기화: 스택 도구를 쓸 수 없어 건너뜁니다", "stack_id", stackID, "error", err)
			} else {
				slog.Warn("CI 실행 기록 주기 동기화: 스택 번들을 만들지 못했습니다", "stack_id", stackID, "error", err)
			}
			continue
		}
		for _, p := range byStack[stackID] {
			if _, err := uc.syncWithBundle(ctx, p, bundle); err != nil {
				slog.Warn("CI 실행 기록 주기 동기화 실패", "pipeline_id", p.ID, "stack_id", stackID, "error", err)
				continue
			}
			synced++
		}
	}
	return synced, nil
}

// syncWithBundle 은 이미 조립한 번들로 파이프라인 하나의 실행 기록을 들인다.
func (uc *SyncPipelineRuns) syncWithBundle(ctx context.Context, pipeline *domain.Pipeline, bundle *port.SCMBundle) (int, error) {
	if bundle == nil || bundle.CIBuilds == nil {
		slog.Warn("CI 실행 기록: 이 스택에는 빌드 이력을 읽을 CI 가 배선되지 않았습니다",
			"pipeline_id", pipeline.ID, "stack_id", pipeline.StackID)
		return 0, nil
	}

	reader := uc.builds
	if reader == nil {
		reader = bundle.CIBuilds
	}
	artifacts := uc.artifacts
	if artifacts == nil {
		artifacts = bundle.CIArtifacts
	}
	sync := NewSyncPipelineRuns(reader, uc.deployments).
		WithImageScans(uc.imageScans).
		WithArtifacts(artifacts)
	return sync.Execute(ctx, SyncPipelineRunsInput{
		PipelineID: pipeline.ID,
		JobName:    pipeline.Name,
		// 스캐폴딩한 파이프라인은 기본 브랜치에서만 돈다
		// (Jenkinsfile 의 when { branch 'main' }, 워크플로의 on.push.branches).
		Branch: defaultRunBranch,
		Limit:  runSyncLimit,
		Policy: uc.stackPolicy(ctx, pipeline.StackID),
	})
}

// stackPolicy 는 스택에 저장된 정책이다. 저장한 적 없거나 읽지 못하면 nil(기본 정책)이다.
func (uc *SyncPipelineRuns) stackPolicy(ctx context.Context, stackID string) *domain.ScanPolicy {
	if uc.policies == nil {
		return nil
	}
	policy, found, err := uc.policies.Get(ctx, stackID)
	if err != nil {
		slog.Warn("CI 실행 기록: 스택 스캔 정책을 읽지 못해 기본 정책으로 판정합니다",
			"stack_id", stackID, "error", err)
		return nil
	}
	if !found {
		return nil
	}
	return &policy
}

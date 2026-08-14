package usecase

import (
	"context"
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
	builds      port.CIBuildReader
	deployments port.DeploymentRepository
	// factory / pipelines 는 파이프라인마다 CI 서버를 찾을 때 쓴다.
	factory   port.SCMBundleFactory
	pipelines port.PipelineRepository
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
			if existing.Status == deployment.Status {
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
	return synced, nil
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
// CI 가 배선되지 않은 파이프라인(GitLab CI·GitHub Actions)은 조용히 건너뛴다 —
// 그쪽 실행 기록을 들이는 경로는 아직 없다.
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
	if bundle == nil || bundle.CIBuilds == nil {
		slog.Warn("CI 실행 기록: 이 스택에는 빌드 이력을 읽을 CI 가 배선되지 않았습니다",
			"pipeline_id", pipeline.ID, "stack_id", pipeline.StackID)
		return 0, nil
	}

	reader := uc.builds
	if reader == nil {
		reader = bundle.CIBuilds
	}
	sync := NewSyncPipelineRuns(reader, uc.deployments)
	return sync.Execute(ctx, SyncPipelineRunsInput{
		PipelineID: pipeline.ID,
		JobName:    pipeline.Name,
		// 파이프라인은 기본 브랜치에서만 돈다(Jenkinsfile 의 when { branch 'main' }).
		Branch: defaultRunBranch,
		Limit:  runSyncLimit,
	})
}

package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// DeletePipelineInput 은 무엇까지 지울지 고른 결과다.
//
// 셋 다 기본값 false 다. 파이프라인 레코드만 지우는 것이 종전 동작이고, 그보다
// 파괴적인 일은 사용자가 명시적으로 골라야 한다.
type DeletePipelineInput struct {
	PipelineID string
	// DeleteClusterResources 는 Argo CD Application 과 그것이 배포한 워크로드를 지운다.
	DeleteClusterResources bool
	// DeleteRepository 는 소스 저장소를 지운다. 되돌릴 수 없다.
	DeleteRepository bool
	// DeleteImages 는 레지스트리의 이미지 저장소를 지운다.
	DeleteImages bool
}

// DeletePipelineOutput 은 실제로 무엇이 지워졌는지 알린다.
//
// 요청과 결과를 따로 두는 이유는, 지원하지 않는 레지스트리처럼 요청은 했지만
// 수행하지 못한 경우가 있기 때문이다. 요청만 보고 "지워졌다" 고 표시하면
// 사용자는 남아 있는 리소스를 영영 모른다.
type DeletePipelineOutput struct {
	ClusterResourcesDeleted bool
	RepositoryDeleted       bool
	ImagesDeleted           bool
	Warnings                []string
}

// DeletePipeline 은 파이프라인과, 사용자가 고른 부수 리소스를 지운다.
type DeletePipeline struct {
	pipelines  port.PipelineRepository
	factory    port.SCMBundleFactory
	kubeconfig port.KubeconfigProvider
	// workloads 는 매니페스트를 직접 적용한 경로가 남긴 워크로드를 지운다.
	// 배선되지 않으면 Argo CD 경로만 정리된다 — 종전 동작이다.
	workloads port.WorkloadDeleter
}

// WithWorkloadDeleter 는 직접 배포 워크로드 정리 경로를 배선한다.
func (uc *DeletePipeline) WithWorkloadDeleter(d port.WorkloadDeleter) *DeletePipeline {
	uc.workloads = d
	return uc
}

func NewDeletePipeline(
	pipelines port.PipelineRepository,
	factory port.SCMBundleFactory,
	kubeconfig port.KubeconfigProvider,
) *DeletePipeline {
	return &DeletePipeline{
		pipelines:  pipelines,
		factory:    factory,
		kubeconfig: kubeconfig,
	}
}

// Execute 는 고른 리소스를 지운 뒤 파이프라인 레코드를 지운다.
//
// 순서가 중요하다. 레코드를 먼저 지우면 저장소 경로·클러스터·네임스페이스를
// 잃어 남은 리소스를 찾을 방법이 없어진다.
//
// 요청한 삭제가 하나라도 실패하면 레코드를 남기고 오류를 돌려준다. 레코드가
// 사라지면 사용자는 목록에서 파이프라인을 못 보는데 리소스는 남아 있어, 다시
// 시도할 방법조차 없어지기 때문이다.
func (uc *DeletePipeline) Execute(
	ctx context.Context,
	input DeletePipelineInput,
) (*DeletePipelineOutput, error) {
	id := strings.TrimSpace(input.PipelineID)
	if id == "" {
		return nil, fmt.Errorf("pipeline id is required")
	}

	pipeline, err := uc.pipelines.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}

	out := &DeletePipelineOutput{}
	// 클러스터 리소스 삭제도 번들이 필요하다. CD 도구가 어느 네임스페이스에
	// 사는지는 스택이 알고, 그것 없이는 애플리케이션을 없는 곳에서 찾게 된다.
	needsExternal := input.DeleteRepository || input.DeleteImages || input.DeleteClusterResources

	// 번들은 언제나 시도한다. CI job 은 플래그 없이도 지워야 하기 때문이다 —
	// 파이프라인이 사라진 뒤에도 job 이 남으면, 없어진 리포를 계속 스캔하며
	// 실패한다. 다만 조립에 실패했을 때 삭제 전체를 세우는 것은 사용자가
	// 요청한 항목(저장소·이미지·클러스터 리소스)이 있을 때뿐이다.
	var bundle *port.SCMBundle
	if uc.factory != nil {
		bundle, err = uc.factory.For(ctx, pipeline.StackID)
		switch {
		case err == nil:
		case errors.Is(err, port.ErrStackToolsUnavailable):
			// 스택이 사라졌거나 아직 준비되지 않았다. 그 도구들에 지금 아무것도
			// 할 수 없다는 뜻이지 삭제의 실패가 아니다.
			//
			// 여기서 막으면 스택을 먼저 지운 파이프라인은 영영 지워지지 않는다 —
			// 목록에는 보이는데 손댈 수 없는 좀비가 남는다. 할 수 있는 것(레코드
			// 삭제)은 하고, 하지 못한 것을 정확히 말한다.
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"스택의 도구에 닿을 수 없어 저장소·이미지·CI job 을 지우지 못했습니다: %v. "+
					"각 도구에서 직접 지워야 합니다", err))
			bundle = nil
		case needsExternal:
			return nil, fmt.Errorf("resolve scm bundle for stack %s: %w", pipeline.StackID, err)
		default:
			slog.Warn("스택 연동을 읽지 못해 CI job 정리를 건너뜁니다",
				"stack_id", pipeline.StackID, "error", err)
			bundle = nil
		}
	} else if needsExternal {
		return nil, fmt.Errorf("SCM 연동이 배선되지 않아 저장소·이미지를 지울 수 없습니다")
	}

	// CI job 을 먼저 지운다. 저장소나 이미지를 지우는 동안 job 이 살아 있으면
	// 그 사이 트리거된 빌드가 방금 지운 것을 다시 만들 수 있다.
	if warning := uc.deleteCIJob(ctx, bundle, pipeline.Name); warning != "" {
		out.Warnings = append(out.Warnings, warning)
	}

	if input.DeleteClusterResources {
		if err := uc.deleteClusterResources(ctx, bundle, pipeline); err != nil {
			return nil, err
		}
		out.ClusterResourcesDeleted = true
	}

	// 이미지를 저장소보다 먼저 지운다. GHCR 패키지는 리포와 별개 리소스지만
	// 어느 패키지인지 알아내는 경로가 리포 소유자에 기대므로, 리포가 사라진 뒤에는
	// 정리가 더 번거로워진다.
	if input.DeleteImages {
		err := uc.deleteImages(ctx, bundle, pipeline.Name)
		switch {
		case err == nil:
			out.ImagesDeleted = true
		case errors.Is(err, port.ErrImageDeletionUnsupported):
			// 지원하지 않는 것은 삭제의 실패가 아니라 이 플랫폼이 할 수 없는
			// 일이다. 그것으로 삭제 전체를 막으면 파이프라인을 영영 못 지운다 —
			// 클러스터 리소스는 이미 지워진 뒤인데 레코드는 남고, 다시 눌러도
			// 같은 자리에서 막힌다.
			//
			// 그렇다고 조용히 넘기지도 않는다. 지우지 못했다는 사실을 경고로
			// 남긴다 — 성공으로 넘기면 사용자는 레지스트리에 남은 것을 영영 모른다.
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"이미지 저장소는 지우지 못했습니다: %v. 레지스트리에서 직접 지워야 합니다", err))
		default:
			// 진짜 실패는 레코드를 남겨 다시 시도할 수 있게 한다.
			return nil, err
		}
	}

	if input.DeleteRepository && bundle != nil {
		repoPath := repositoryPathFor(bundle.GroupPath, pipeline.Name)
		if err := bundle.Provisioner.DeleteProject(ctx, repoPath); err != nil {
			return nil, fmt.Errorf("저장소 %s 삭제 실패: %w", repoPath, err)
		}
		out.RepositoryDeleted = true
	}

	if err := uc.pipelines.Delete(ctx, id); err != nil {
		return nil, err
	}
	return out, nil
}

// deleteCIJob 은 CI 서버에 만들어 둔 job 을 지운다.
//
// 플래그로 고르게 하지 않는다. job 은 이 파이프라인 몫으로 플랫폼이 만든 것이고,
// 파이프라인이 사라진 뒤에는 없어진 리포를 계속 스캔하며 실패할 뿐이다.
//
// 지우지 못해도 삭제를 멈추지 않는다 — CI 서버가 잠깐 응답하지 않는다고 해서
// 파이프라인을 못 지우게 되면, 클러스터 리소스는 이미 지워진 뒤라 되돌리기
// 어려운 상태에 갇힌다. 대신 지우지 못했다는 사실을 경고로 남긴다.
//
// 지원하지 않는 플랫폼(GitLab CI·GitHub Actions)은 CIJobs 가 nil 이다. 그쪽은
// 파이프라인 정의를 리포에서 읽으므로 지울 job 자체가 없다.
func (uc *DeletePipeline) deleteCIJob(ctx context.Context, bundle *port.SCMBundle, appName string) string {
	if bundle == nil || bundle.CIJobs == nil {
		return ""
	}
	if err := bundle.CIJobs.DeleteJob(ctx, appName); err != nil {
		slog.Warn("CI job 삭제 실패", "app", appName, "error", err)
		return fmt.Sprintf("CI job %s 를 지우지 못했습니다: %v. CI 서버에서 직접 지워야 합니다", appName, err)
	}
	return ""
}

func (uc *DeletePipeline) deleteClusterResources(
	ctx context.Context,
	bundle *port.SCMBundle,
	pipeline *domain.Pipeline,
) error {
	if uc.kubeconfig == nil {
		return fmt.Errorf("클러스터 접근이 배선되지 않아 배포 리소스를 지울 수 없습니다")
	}
	kubeconfig, err := uc.kubeconfig.GetKubeconfig(ctx, pipeline.ClusterID)
	if err != nil {
		return fmt.Errorf("클러스터 %s kubeconfig 로드 실패: %w", pipeline.ClusterID, err)
	}

	if bundle != nil && bundle.CDApplications != nil {
		// 애플리케이션은 **CD 도구의 네임스페이스**에 산다. 배포 대상
		// 네임스페이스(앱이 서는 곳)를 넘기면 없는 곳을 뒤지고, 구현체는
		// "이미 없음" 을 성공으로 보므로 조용히 남는다 — 실제로 그렇게
		// 파이프라인을 지워도 Argo CD 에 애플리케이션이 계속 남았다.
		cdNamespace := strings.TrimSpace(bundle.CDNamespace)
		if cdNamespace == "" {
			return fmt.Errorf("CD 도구의 네임스페이스를 알 수 없어 배포된 애플리케이션을 지울 수 없습니다")
		}
		if err := bundle.CDApplications.DeleteApplication(ctx, kubeconfig, cdNamespace, pipeline.Name); err != nil {
			return fmt.Errorf("배포된 애플리케이션 삭제 실패: %w", err)
		}
	}

	// Argo CD Application 이 없어도 위 호출은 성공으로 본다 — 매니페스트를
	// 직접 적용한 파이프라인이 그렇다. 그 경우 워크로드를 지워 줄 주체가
	// 없으므로 라벨로 찾아 함께 지운다. 이것이 빠져 있어서 파이프라인을
	// "클러스터 리소스까지" 지워도 앱이 계속 도는 상태가 남았다.
	if uc.workloads == nil {
		return nil
	}
	// 매니페스트 생성기가 모든 리소스에 붙이는 라벨이다.
	//
	// 워크로드는 **배포 대상 네임스페이스**에 있다 — CD 도구가 사는 곳이 아니다.
	selector := "app=" + pipeline.Name
	if err := uc.workloads.DeleteByLabel(ctx, kubeconfig, pipeline.Namespace, selector); err != nil {
		return fmt.Errorf("배포된 워크로드 삭제 실패 (%s): %w", selector, err)
	}
	return nil
}

func (uc *DeletePipeline) deleteImages(
	ctx context.Context,
	bundle *port.SCMBundle,
	appName string,
) error {
	// 번들이 없으면 레지스트리에 닿을 수단 자체가 없다 — 스택이 사라진 경우다.
	if bundle == nil || bundle.Images == nil {
		return port.ErrImageDeletionUnsupported
	}
	target, err := bundle.Registry.Resolve(ctx, port.ImageTargetSpec{
		AppName: appName,
		OrgPath: bundle.GroupPath,
	})
	if err != nil {
		return fmt.Errorf("이미지 저장소 위치를 알아내지 못했습니다: %w", err)
	}
	if err := bundle.Images.DeleteImageRepository(ctx, target); err != nil {
		if errors.Is(err, port.ErrImageDeletionUnsupported) {
			return err
		}
		return fmt.Errorf("이미지 저장소 %s 삭제 실패: %w", target.Repository, err)
	}
	return nil
}

// repositoryPathFor 는 저장소를 가리키는 식별자를 고른다.
//
// GitHub 은 "owner/repo", GitLab 은 "group/project" 형태를 모두 받는다.
// 파이프라인에 저장된 git_repo_url 대신 조합하는 이유는, URL 에는 .git 접미사와
// 스킴이 섞여 있어 API 경로로 바로 쓸 수 없기 때문이다.
func repositoryPathFor(groupPath, appName string) string {
	return strings.Trim(strings.TrimSpace(groupPath), "/") + "/" + strings.TrimSpace(appName)
}

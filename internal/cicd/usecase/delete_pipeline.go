package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	argoApps   port.ArgoApplicationDeleter
	kubeconfig port.KubeconfigProvider
}

func NewDeletePipeline(
	pipelines port.PipelineRepository,
	factory port.SCMBundleFactory,
	argoApps port.ArgoApplicationDeleter,
	kubeconfig port.KubeconfigProvider,
) *DeletePipeline {
	return &DeletePipeline{
		pipelines:  pipelines,
		factory:    factory,
		argoApps:   argoApps,
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
	needsExternal := input.DeleteRepository || input.DeleteImages

	var bundle *port.SCMBundle
	if needsExternal {
		if uc.factory == nil {
			return nil, fmt.Errorf("SCM 연동이 배선되지 않아 저장소·이미지를 지울 수 없습니다")
		}
		bundle, err = uc.factory.For(ctx, pipeline.StackID)
		if err != nil {
			return nil, fmt.Errorf("resolve scm bundle for stack %s: %w", pipeline.StackID, err)
		}
	}

	if input.DeleteClusterResources {
		if err := uc.deleteClusterResources(ctx, pipeline.ClusterID, pipeline.Namespace, pipeline.Name); err != nil {
			return nil, err
		}
		out.ClusterResourcesDeleted = true
	}

	// 이미지를 저장소보다 먼저 지운다. GHCR 패키지는 리포와 별개 리소스지만
	// 어느 패키지인지 알아내는 경로가 리포 소유자에 기대므로, 리포가 사라진 뒤에는
	// 정리가 더 번거로워진다.
	if input.DeleteImages {
		if err := uc.deleteImages(ctx, bundle, pipeline.Name); err != nil {
			return nil, err
		}
		out.ImagesDeleted = true
	}

	if input.DeleteRepository {
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

func (uc *DeletePipeline) deleteClusterResources(
	ctx context.Context,
	clusterID, namespace, appName string,
) error {
	if uc.argoApps == nil || uc.kubeconfig == nil {
		return fmt.Errorf("클러스터 접근이 배선되지 않아 배포 리소스를 지울 수 없습니다")
	}
	kubeconfig, err := uc.kubeconfig.GetKubeconfig(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("클러스터 %s kubeconfig 로드 실패: %w", clusterID, err)
	}
	if err := uc.argoApps.Delete(ctx, kubeconfig, namespace, appName); err != nil {
		return fmt.Errorf("Argo CD Application 삭제 실패: %w", err)
	}
	return nil
}

func (uc *DeletePipeline) deleteImages(
	ctx context.Context,
	bundle *port.SCMBundle,
	appName string,
) error {
	if bundle.Images == nil {
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

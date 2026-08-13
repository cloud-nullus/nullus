package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
)

// 직접 배포(POST /cicd/deploy-app)가 실제로 클러스터에 적용하도록 하는 경로다.
//
// 이 엔드포인트는 매니페스트를 만들어 응답에 담고 Deployment 레코드를
// status=success 로 저장하면서 **클러스터에는 아무것도 적용하지 않았다**.
// 화면에는 성공한 배포로 보이는데 실체가 없다 — 배포 목록과 클러스터가
// 어긋나고, 사용자는 없는 앱의 로그를 찾게 된다.

// deployAppManifestDocs 는 적용할 매니페스트를 의존 순서로 늘어놓는다.
//
// 네임스페이스가 먼저다. 없는 네임스페이스에 Deployment 를 넣으면 그 자리에서
// 실패한다. 빈 문서는 뺀다 — kubectl apply 가 빈 입력에서 오류를 낸다.
func deployAppManifestDocs(generated *manifests.GeneratedManifests) []string {
	if generated == nil {
		return nil
	}
	ordered := []string{
		generated.Namespace,
		generated.Deployment,
		generated.Service,
		generated.Ingress,
	}
	docs := make([]string, 0, len(ordered))
	for _, doc := range ordered {
		if strings.TrimSpace(doc) != "" {
			docs = append(docs, doc)
		}
	}
	return docs
}

// applyDeployAppManifests 는 생성된 매니페스트를 대상 클러스터에 적용한다.
//
// 배선이 없으면 오류를 돌려준다. 조용히 건너뛰면 예전처럼 "적용하지 않고
// 성공으로 기록" 하는 상태로 되돌아간다.
func (h *PipelineHandler) applyDeployAppManifests(
	ctx context.Context,
	clusterID string,
	generated *manifests.GeneratedManifests,
	deploymentID string,
) error {
	if h.applier == nil {
		return fmt.Errorf("매니페스트 적용기가 배선되지 않아 배포할 수 없습니다")
	}
	if h.kubeconfig == nil {
		return fmt.Errorf("클러스터 접근이 배선되지 않아 배포할 수 없습니다")
	}

	kubeconfig, err := h.kubeconfig.GetKubeconfig(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("클러스터 %s kubeconfig 로드 실패: %w", clusterID, err)
	}

	docs := deployAppManifestDocs(generated)
	if len(docs) == 0 {
		return fmt.Errorf("적용할 매니페스트가 없습니다")
	}
	return h.applier.ApplyWithTracking(ctx, kubeconfig, docs, deploymentID)
}

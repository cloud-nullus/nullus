package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// MaxSecurityPipelines 는 보안 대시보드가 한 번에 읽는 파이프라인 수의 상한이다. 주기 동기화 대상
// (ListWithStack)과 같은 값이다 — 상한에 닿는 조직이 생기면 그때 페이지를 나눈다(설계 §5.2).
const MaxSecurityPipelines = 1000

// ListPipelineSecurity 는 보안 대시보드(#65)가 읽는 cicd 의 공개 경로다.
//
// 결과 테이블(image_scan_results · sast_results · dependency_scan_results · secret_scan_results)과 스캔
// 정책은 cicd 가 소유한다. 관측 모듈은 테이블을 직접 읽지 않고 이 유스케이스로 읽는다.
type ListPipelineSecurity struct {
	reader port.PipelineSecurityReader
}

// NewListPipelineSecurity 는 유스케이스를 만든다.
func NewListPipelineSecurity(reader port.PipelineSecurityReader) *ListPipelineSecurity {
	return &ListPipelineSecurity{reader: reader}
}

// Execute 는 조직의 파이프라인을 최근 만든 것부터 상한까지 돌려준다.
func (uc *ListPipelineSecurity) Execute(ctx context.Context, orgID string) (*domain.PipelineSecurityList, error) {
	if uc == nil || uc.reader == nil {
		return nil, errors.New("파이프라인 보안 결과 조회가 배선되지 않았습니다")
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, errors.New("org_id 가 필요합니다")
	}
	// 하나 더 읽어 상한을 넘었는지 안다.
	snapshots, err := uc.reader.ListPipelineSecurity(ctx, orgID, MaxSecurityPipelines+1)
	if err != nil {
		return nil, fmt.Errorf("파이프라인 보안 결과 조회 실패: %w", err)
	}
	out := &domain.PipelineSecurityList{Pipelines: snapshots}
	if len(snapshots) > MaxSecurityPipelines {
		out.Pipelines = snapshots[:MaxSecurityPipelines]
		out.Truncated = true
	}
	for _, s := range out.Pipelines {
		if s == nil {
			continue
		}
		s.RunVersions = runVersions(s)
		markSecurityStages(s)
	}
	return out, nil
}

// markSecurityStages 는 파이프라인이 가진 보안 단계를 표시한다. 단계 이름은 CI 마다 표기가 달라
// (Jenkins 는 ImageScan, GitLab·GitHub 은 image-scan) 동기화와 같은 단계 키로 맞춘다 — 다른 모듈이
// 이 규칙을 다시 만들면 한쪽만 바뀐다.
func markSecurityStages(s *domain.PipelineSecuritySnapshot) {
	for _, name := range s.Stages {
		switch port.StageKey(name) {
		case secretScanStageKey:
			s.HasSecretScan = true
		case dependencyScanStageKey:
			s.HasDependencyScan = true
		case sastStageKey:
			s.HasSAST = true
		case imageScanStageKey:
			s.HasImageScan = true
		}
	}
}

// runVersions 는 결과 행이 가리키는 실행의 표시 번호다. 결과 행은 실행 ID 만 갖는다.
func runVersions(s *domain.PipelineSecuritySnapshot) map[string]string {
	out := map[string]string{}
	add := func(deploymentID string) {
		if v := runVersionOf(s.PipelineID, deploymentID); v != "" {
			out[deploymentID] = v
		}
	}
	if s.SecretScan != nil {
		add(s.SecretScan.DeploymentID)
	}
	if s.DependencyScan != nil {
		add(s.DependencyScan.DeploymentID)
	}
	if s.SAST != nil {
		add(s.SAST.DeploymentID)
	}
	if s.ImageScan != nil {
		add(s.ImageScan.DeploymentID)
	}
	return out
}

// runVersionOf 는 runDeploymentID 를 거꾸로 읽어 실행 번호(#42)를 돌려준다. 규칙을 따르지 않는 ID
// (플랫폼이 직접 실행한 배포)면 빈 값이다 — 번호를 지어내지 않는다.
func runVersionOf(pipelineID, deploymentID string) string {
	prefix := "dep_ci_" + strings.TrimSpace(pipelineID) + "_"
	rest, ok := strings.CutPrefix(strings.TrimSpace(deploymentID), prefix)
	if !ok {
		return ""
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("#%d", n)
}

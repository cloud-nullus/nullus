package usecase

import (
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// reportRefFrom 은 동기화가 리포트를 읽은 위치를 기록용으로 옮긴다.
func reportRefFrom(ref port.CIArtifactRef) *domain.ScanReportRef {
	return &domain.ScanReportRef{
		JobName:     ref.JobName,
		Branch:      ref.Branch,
		BuildID:     ref.Build.ID,
		BuildNumber: ref.Build.Number,
		StageID:     ref.Stage.ID,
		StageName:   ref.Stage.Name,
		Artifact:    ref.Name,
		Path:        ref.Path,
	}
}

// artifactRefFrom 은 기록한 위치로 산출물 조회기에 넘길 참조를 만든다.
func artifactRefFrom(ref *domain.ScanReportRef) port.CIArtifactRef {
	if ref == nil {
		return port.CIArtifactRef{}
	}
	return port.CIArtifactRef{
		JobName: ref.JobName,
		Branch:  ref.Branch,
		Build:   port.CIBuild{ID: ref.BuildID, Number: ref.BuildNumber},
		Stage:   port.CIStage{ID: ref.StageID, Name: ref.StageName},
		Name:    ref.Artifact,
		Path:    ref.Path,
	}
}

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 프로비저닝이 알려준 실효 단계를 파이프라인에 기록한다 (설계 §5.3 B안).
//
// 스캔 on/off 는 파이프라인 단위 선택이다. 템플릿 단위 컬럼에만 두면 스캔을 끈
// 파이프라인이 돌지도 않은 단계를 보여주게 된다(마이그레이션 000070).
func TestCreatePipeline_RecordsEffectiveStages(t *testing.T) {
	result := provisionedResult()
	result.Stages = []string{"Build", "ImageScan", "Deploy"}
	p := &fakeRepoProvisioner{out: result}
	uc, repo := newCreateWithProvisioner(p)

	out, err := uc.Execute(context.Background(), provisionInput())
	require.NoError(t, err)

	assert.Equal(t, []string{"Build", "ImageScan", "Deploy"}, out.Pipeline.Stages)
	require.Len(t, repo.created, 1)
	assert.Equal(t, []string{"Build", "ImageScan", "Deploy"}, repo.created[0].Stages)
}

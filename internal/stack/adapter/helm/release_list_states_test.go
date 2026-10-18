package helm

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// action.List 의 All 은 SetStateMask() 를 불러야 효과가 있다. 부르지 않으면 기본 마스크
// (deployed | failed)가 남아 업그레이드 중(pending-upgrade)인 릴리스가 목록에서 사라진다 —
// 업그레이드 화면은 진행 중인 Argo CD 를 "없음" 으로 보이고, 같은 때 들어온 업그레이드 요청은
// 진행 중이라 막히는 대신 404 가 됐다(실클러스터 리허설에서 발견).
func TestHelmInstaller_ListReleases_IncludesPendingReleases(t *testing.T) {
	store := storage.Init(driver.NewMemory())
	for _, rel := range []*release.Release{
		{Name: "argo-cd", Namespace: "nullus-prod", Version: 2, Info: &release.Info{Status: release.StatusSuperseded},
			Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.16", AppVersion: "v2.13.3"}}},
		{Name: "argo-cd", Namespace: "nullus-prod", Version: 3, Info: &release.Info{Status: release.StatusPendingUpgrade},
			Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.22", AppVersion: "v2.13.4"}}},
		{Name: "gitlab", Namespace: "nullus-prod", Version: 1, Info: &release.Info{Status: release.StatusDeployed},
			Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "gitlab", Version: "8.0.0"}}},
	} {
		require.NoError(t, store.Create(rel))
	}
	h := &HelmInstaller{newActionConfig: func([]byte, string) (*action.Configuration, error) {
		return &action.Configuration{Releases: store, KubeClient: &kubefake.PrintingKubeClient{Out: io.Discard}}, nil
	}}

	got, err := h.ListReleases(context.Background(), "nullus-prod")
	require.NoError(t, err)
	status := map[string]string{}
	revision := map[string]int{}
	for _, r := range got {
		status[r.ReleaseName], revision[r.ReleaseName] = r.Status, r.Revision
	}
	assert.Equal(t, "pending-upgrade", status["argo-cd"], "업그레이드 중인 릴리스도 보여야 한다")
	assert.Equal(t, 3, revision["argo-cd"], "릴리스마다 최신 리비전 하나만")
	assert.Equal(t, "deployed", status["gitlab"])
	assert.Len(t, got, 2)
}

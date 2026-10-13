package alertmanager

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

type fakeKubeconfigs struct{ cfg []byte }

func (f fakeKubeconfigs) GetKubeconfig(context.Context, string) ([]byte, error) { return f.cfg, nil }

func kubeconfigFor(server string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: c
  cluster: {server: %q}
contexts:
- name: c
  context: {cluster: c, user: u}
current-context: c
users:
- name: u
  user: {token: t}
`, server))
}

const alertsJSON = `[
 {"annotations":{"summary":"disk","description":"2% left","runbook_url":"https://rb"},
  "fingerprint":"0a65f66ce09bc90d","startsAt":"2026-10-13T00:26:16.358Z","status":{"state":"active"},
  "labels":{"alertname":"NodeFilesystemAlmostOutOfSpace","severity":"critical","instance":"10.0.0.1:9100"}}
]`

var stack = domain.MonitoredStack{ID: "stk_1", OrgID: "org-1", Name: "gitlab", Namespace: "nullus-gitlab", ClusterID: "c1"}

// API 서버 프록시로 스택 Alertmanager 의 활성 경보를 읽는다.
func TestSource_ReadsActiveAlertsThroughAPIServerProxy(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(alertsJSON))
	}))
	defer srv.Close()

	alerts, err := NewSource(fakeKubeconfigs{cfg: kubeconfigFor(srv.URL)}).ActiveAlerts(context.Background(), stack)
	require.NoError(t, err)

	assert.Equal(t, "/api/v1/namespaces/nullus-gitlab/services/http:kube-prometheus-stack-alertmanager:9093/proxy/api/v2/alerts", gotPath)
	assert.Contains(t, gotQuery, "active=true")
	require.Len(t, alerts, 1)
	a := alerts[0]
	assert.Equal(t, "0a65f66ce09bc90d", a.Fingerprint)
	assert.Equal(t, "NodeFilesystemAlmostOutOfSpace", a.Labels["alertname"])
	assert.Equal(t, "https://rb", a.Annotations["runbook_url"])
	assert.Equal(t, time.Date(2026, 10, 13, 0, 26, 16, 358000000, time.UTC), a.StartsAt.UTC())
}

// 닿지 못하면 오류다 — 빈 목록이면 수집기가 열린 경보를 모두 해제한다.
func TestSource_UnreachableIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `services "kube-prometheus-stack-alertmanager" not found`, http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := NewSource(fakeKubeconfigs{cfg: kubeconfigFor(srv.URL)}).ActiveAlerts(context.Background(), stack)
	assert.Error(t, err)

	_, err = NewSource(fakeKubeconfigs{}).ActiveAlerts(context.Background(), stack)
	assert.Error(t, err, "kubeconfig 가 없으면 닿을 수 없다")
}

// 침묵 · 억제된 경보도 가져온다 — 알리지는 않지만 "보인 것" 이라 해제하지 않는다.
func TestSource_IncludesSuppressedAlertsMarked(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
 {"fingerprint":"a","startsAt":"2026-10-13T00:00:00Z","status":{"state":"active"},"labels":{"alertname":"A","severity":"critical"}},
 {"fingerprint":"b","startsAt":"2026-10-13T00:00:00Z","status":{"state":"suppressed","silencedBy":["s1"]},"labels":{"alertname":"B","severity":"critical"}}
]`))
	}))
	defer srv.Close()

	alerts, err := NewSource(fakeKubeconfigs{cfg: kubeconfigFor(srv.URL)}).ActiveAlerts(context.Background(), stack)
	require.NoError(t, err)
	assert.NotContains(t, gotQuery, "silenced=false")
	assert.NotContains(t, gotQuery, "inhibited=false")
	require.Len(t, alerts, 2)
	assert.False(t, alerts[0].Suppressed)
	assert.True(t, alerts[1].Suppressed)
}

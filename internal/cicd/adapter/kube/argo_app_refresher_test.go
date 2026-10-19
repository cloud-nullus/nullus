package kube

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// argoController 는 Argo CD 컨트롤러 흉내다. 폴링 사이(sleep)마다 한 번 돈다.
type argoController func(app *unstructured.Unstructured)

func argoApp(reconciledAt, comparisonError string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("argoproj.io/v1alpha1")
	u.SetKind("Application")
	u.SetNamespace("devsecops")
	u.SetName("myapp")
	status := map[string]any{}
	if reconciledAt != "" {
		status["reconciledAt"] = reconciledAt
	}
	if comparisonError != "" {
		status["conditions"] = []any{map[string]any{"type": "ComparisonError", "message": comparisonError}}
	}
	u.Object["status"] = status
	return u
}

// refresherWith 는 가짜 클러스터와 가짜 시계로 refresher 를 만든다. 반환하는 patches 는
// Application 에 보낸 patch(refresh 요청) 횟수다.
func refresherWith(t *testing.T, app *unstructured.Unstructured, controller argoController) (*ArgoApplicationRefresher, *int) {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"})
	apps := client.Resource(argoApplicationGVR).Namespace("devsecops")
	if app != nil {
		_, err := apps.Create(context.Background(), app, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	patches := 0
	client.PrependReactor("patch", "applications", func(k8stesting.Action) (bool, runtime.Object, error) {
		patches++
		return false, nil, nil
	})
	clock := time.Unix(0, 0)
	r := &ArgoApplicationRefresher{
		newClient: func([]byte) (dynamic.Interface, error) { return client, nil },
		now:       func() time.Time { return clock },
		timeout:   10 * time.Second,
		poll:      time.Second,
	}
	r.sleep = func(context.Context, time.Duration) error {
		clock = clock.Add(time.Second)
		cur, err := apps.Get(context.Background(), "myapp", metav1.GetOptions{})
		if err != nil {
			return nil
		}
		if controller != nil {
			controller(cur)
			_, err = apps.Update(context.Background(), cur, metav1.UpdateOptions{})
			require.NoError(t, err)
		}
		return nil
	}
	return r, &patches
}

func TestArgoApplicationRefresher_ReturnsWhenFirstComparisonSucceeded(t *testing.T) {
	r, patches := refresherWith(t, argoApp("2026-10-19T01:52:35Z", ""), nil)

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Zero(t, *patches, "읽었으면 다시 읽게 하지 않는다")
}

// 첫 비교가 자격증명을 못 보고 실패하면 다시 읽게 한다. 컨트롤러는 refresh 주석을
// 받으면 다시 비교하고 주석을 지운다.
func TestArgoApplicationRefresher_RefreshesAfterComparisonError(t *testing.T) {
	controller := func(app *unstructured.Unstructured) {
		if app.GetAnnotations()[argoRefreshAnnotation] == "" {
			return
		}
		app.SetAnnotations(nil)
		app.Object["status"] = map[string]any{"reconciledAt": "2026-10-19T01:52:37Z"}
	}
	r, patches := refresherWith(t,
		argoApp("2026-10-19T01:52:35Z", "rpc error: code = Unknown desc = authentication required"), controller)

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Equal(t, 1, *patches)
}

// 다시 읽게 해도 같은 오류면 진짜 문제다(자격증명 · 경로). 몇 번만 시도하고 이유를 돌려준다.
func TestArgoApplicationRefresher_ReportsPersistentComparisonError(t *testing.T) {
	n := 0
	controller := func(app *unstructured.Unstructured) {
		if app.GetAnnotations()[argoRefreshAnnotation] == "" {
			return
		}
		n++
		app.SetAnnotations(nil)
		app.Object["status"] = map[string]any{
			"reconciledAt": time.Unix(int64(100+n), 0).UTC().Format(time.RFC3339),
			"conditions":   []any{map[string]any{"type": "ComparisonError", "message": "app path does not exist"}},
		}
	}
	r, patches := refresherWith(t, argoApp("2026-10-19T01:52:35Z", "app path does not exist"), controller)

	err := r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app path does not exist")
	assert.Equal(t, maxFirstComparisonRefreshes, *patches, "같은 오류를 계속 다시 읽게 하지 않는다")
}

// 컨트롤러가 아직 한 번도 비교하지 않았으면 기다린다. 끝내 안 오면 Argo CD 에 맡긴다 —
// 플랫폼이 알릴 오류가 아니다.
func TestArgoApplicationRefresher_GivesUpQuietlyWhenNeverReconciled(t *testing.T) {
	r, patches := refresherWith(t, argoApp("", ""), nil)

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Zero(t, *patches)
}

func TestArgoApplicationRefresher_WaitsForFirstReconcile(t *testing.T) {
	controller := func(app *unstructured.Unstructured) {
		app.Object["status"] = map[string]any{"reconciledAt": "2026-10-19T01:52:36Z"}
	}
	r, patches := refresherWith(t, argoApp("", ""), controller)

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Zero(t, *patches)
}

func TestArgoApplicationRefresher_RequiresNamespaceAndName(t *testing.T) {
	r, _ := refresherWith(t, nil, nil)

	assert.Error(t, r.EnsureFirstComparison(context.Background(), nil, "", "myapp"))
	assert.Error(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", ""))
}

// Argo CD 가 refresh 를 실패한 첫 비교와 같은 초 안에 처리하면 reconciledAt(초 단위)은 그대로다.
// 처리 여부는 refresh 주석으로만 본다 — 컨트롤러는 주석 삭제와 status 기록을 한 패치로 한다.
func TestArgoApplicationRefresher_RefreshProcessedWithinSameSecond(t *testing.T) {
	controller := func(app *unstructured.Unstructured) {
		if app.GetAnnotations()[argoRefreshAnnotation] == "" {
			return
		}
		app.SetAnnotations(nil)
		app.Object["status"] = map[string]any{"reconciledAt": "2026-10-19T01:52:35Z"}
	}
	r, patches := refresherWith(t, argoApp("2026-10-19T01:52:35Z", "authentication required"), controller)

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Equal(t, 1, *patches)
}

// 컨트롤러가 느려 마지막 refresh 를 아직 처리하지 못한 채 시간이 다 되면 옛 오류로 경고하지
// 않는다 — 곧 처리될 것이고, 그 결과는 아직 모른다.
func TestArgoApplicationRefresher_DoesNotReportStaleErrorWhileRefreshPending(t *testing.T) {
	r, patches := refresherWith(t, argoApp("2026-10-19T01:52:35Z", "authentication required"), nil)

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Equal(t, 1, *patches, "처리되지 않은 refresh 위에 또 요청하지 않는다")
}

// 방금 적용한 Application 이 아직 안 보이다가 나타나도 이어서 본다.
func TestArgoApplicationRefresher_WaitsForApplicationToAppear(t *testing.T) {
	r, patches := refresherWith(t, nil, nil)
	created := false
	inner := r.sleep
	r.sleep = func(ctx context.Context, d time.Duration) error {
		if !created {
			created = true
			client, _ := r.newClient(nil)
			_, err := client.Resource(argoApplicationGVR).Namespace("devsecops").Create(ctx, argoApp("2026-10-19T01:52:36Z", ""), metav1.CreateOptions{})
			require.NoError(t, err)
		}
		return inner(ctx, d)
	}

	require.NoError(t, r.EnsureFirstComparison(context.Background(), nil, "devsecops", "myapp"))
	assert.Zero(t, *patches)
}

func TestArgoApplicationRefresher_StopsWhenContextCancelled(t *testing.T) {
	r, _ := refresherWith(t, argoApp("", ""), nil)
	r.sleep = sleepContext
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.ErrorIs(t, r.EnsureFirstComparison(ctx, nil, "devsecops", "myapp"), context.Canceled)
}

package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// 새 Application 의 첫 비교가 저장소 자격증명을 못 보고 실패할 수 있다.
//
// 플랫폼은 자격증명 Secret 과 Application 을 한 번에 적용하는데, Argo CD 컨트롤러는
// 둘을 서로 다른 감시(informer)로 받는다. Secret 이 아직 안 보이는 순간에 Application
// 을 비교하면 "authentication required" 로 끝나고, 다음 정기 재조정(기본 3분)까지 그
// 상태로 남는다 — 2026-10-19 실측에서 첫 배포가 그만큼 늦었다.

const (
	// argoRefreshAnnotation 은 Argo CD 에 지금 다시 비교하라고 요청하는 주석이다.
	// 컨트롤러가 받아 처리하면 스스로 지운다.
	argoRefreshAnnotation = "argocd.argoproj.io/refresh"

	firstComparisonTimeout = 20 * time.Second
	firstComparisonPoll    = time.Second
	// maxFirstComparisonRefreshes 는 다시 읽게 하는 횟수다. 실패가 쌓이면 Argo CD 의
	// repo-server 가 그 앱의 매니페스트 생성을 한동안 멈추므로 많이 시도하지 않는다.
	maxFirstComparisonRefreshes = 2
)

// ArgoApplicationRefresher 는 Application 의 첫 비교를 확인하고 필요하면 다시 읽게 한다.
type ArgoApplicationRefresher struct {
	newClient func(kubeconfig []byte) (dynamic.Interface, error)
	now       func() time.Time
	sleep     func(ctx context.Context, d time.Duration) error
	timeout   time.Duration
	poll      time.Duration
}

func NewArgoApplicationRefresher() *ArgoApplicationRefresher {
	return &ArgoApplicationRefresher{
		newClient: func(kubeconfig []byte) (dynamic.Interface, error) {
			config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("parse kubeconfig: %w", err)
			}
			config.Timeout = 30 * time.Second
			return dynamic.NewForConfig(config)
		},
		now:     time.Now,
		sleep:   sleepContext,
		timeout: firstComparisonTimeout,
		poll:    firstComparisonPoll,
	}
}

// EnsureFirstComparison 은 첫 비교가 저장소를 읽을 때까지 잠깐 기다린다.
//
// 비교 오류가 보이면 refresh 를 요청하고, 컨트롤러가 그것을 처리한 뒤(주석이 지워진 뒤)
// 에도 오류가 남으면 한 번 더 요청한다. 횟수를 다 쓰고도 남으면 그 이유를 돌려준다.
// 컨트롤러가 아직 비교하지 않았거나 요청한 refresh 를 처리하지 못한 채 시간이 다 되면
// 조용히 넘긴다 — Argo CD 가 곧 처리할 것이고, 결과를 모르는 채 경고하지 않는다.
func (r *ArgoApplicationRefresher) EnsureFirstComparison(
	ctx context.Context,
	kubeconfig []byte,
	namespace, name string,
) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("argo application 의 namespace 와 name 이 모두 필요합니다")
	}
	client, err := r.newClient(kubeconfig)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	apps := client.Resource(argoApplicationGVR).Namespace(namespace)

	deadline := r.now().Add(r.timeout)
	refreshes := 0
	// lastError 는 컨트롤러가 처리를 마친 비교의 오류다. refresh 를 요청한 뒤 처리되기
	// 전까지는 비운다 — 그 사이의 오류는 지난 비교의 것이다.
	lastError := ""
	for {
		app, err := apps.Get(ctx, name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			// 방금 적용했다 — 조금 기다린다.
		case err != nil:
			return fmt.Errorf("get application %s/%s: %w", namespace, name, err)
		default:
			state := readComparisonState(app)
			switch {
			case state.refreshPending:
				// 요청한 refresh 를 아직 처리하지 않았다. 처리됐는지는 주석으로만 본다 —
				// 컨트롤러는 주석 삭제와 status 기록을 한 패치로 하고, reconciledAt 은 초
				// 단위라 같은 초 안에 다시 비교하면 바뀌지 않는다.
				lastError = ""
			case state.reconciledAt != "" && state.comparisonError == "":
				return nil
			case state.comparisonError != "":
				lastError = state.comparisonError
				if refreshes >= maxFirstComparisonRefreshes {
					return fmt.Errorf("다시 읽게 해도 저장소를 읽지 못했습니다: %s", lastError)
				}
				if err := requestRefresh(ctx, apps, name); err != nil {
					return err
				}
				slog.Info("Argo CD 첫 비교가 실패해 다시 읽게 했습니다",
					"application", namespace+"/"+name, "attempt", refreshes+1, "error", lastError)
				refreshes++
				lastError = ""
			}
		}
		// 시간이 다 됐다는 것은 아직 처리되지 않은 비교나 refresh 가 남았다는 뜻이다 — 처리를
		// 마친 오류는 위에서 refresh 하거나 돌려준다. 결과를 모르는 채 경고하지 않는다.
		if !r.now().Before(deadline) {
			return nil
		}
		if err := r.sleep(ctx, r.poll); err != nil {
			return err
		}
	}
}

type comparisonState struct {
	reconciledAt    string
	comparisonError string
	refreshPending  bool
}

func readComparisonState(app *unstructured.Unstructured) comparisonState {
	state := comparisonState{
		refreshPending: app.GetAnnotations()[argoRefreshAnnotation] != "",
	}
	state.reconciledAt, _, _ = unstructured.NestedString(app.Object, "status", "reconciledAt")
	conditions, _, _ := unstructured.NestedSlice(app.Object, "status", "conditions")
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok || cond["type"] != "ComparisonError" {
			continue
		}
		msg, _ := cond["message"].(string)
		state.comparisonError = strings.TrimSpace(msg)
		if state.comparisonError == "" {
			state.comparisonError = "ComparisonError"
		}
	}
	return state
}

func requestRefresh(ctx context.Context, apps dynamic.ResourceInterface, name string) error {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{argoRefreshAnnotation: "normal"},
		},
	})
	if err != nil {
		return fmt.Errorf("build refresh patch: %w", err)
	}
	if _, err := apps.Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("request refresh of application %s: %w", name, err)
	}
	return nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

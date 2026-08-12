package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// /workloads 는 클러스터의 실제 상태를 보여야 한다.
//
// 개편 전 이 엔드포인트는 쿠버네티스를 한 번도 보지 않았다. DB 의 파이프라인 행을
// 읽어 객체를 지어냈다 — replicas 는 항상 2, 포트는 항상 8080, 파드 이름은 배포
// 시각 해시, 노드 이름은 "<cluster>-node-1", Ingress 호스트는 "<앱>.<ns>.nullus.local".
// 파드 상태도 클러스터가 아니라 DB 배포 행에서 파생했다.
//
// 값이 비어 있는 것보다 나쁘다. 비면 "아직 없구나" 로 읽히지만, 지어낸 값은
// 사실로 읽힌다 — 운영자가 그걸 보고 판단한다.

func TestBuildWorkloadObjects_UsesLiveClusterState(t *testing.T) {
	live := clusterWorkloads{
		Deployments: []liveDeployment{
			{Name: "demo-app", Namespace: "apps", Replicas: 3, ReadyReplicas: 2},
		},
		Pods: []livePod{
			{Name: "demo-app-7d4f9c-abcde", Namespace: "apps", Phase: "Running", Ready: true, Node: "worker-1"},
			{Name: "demo-app-7d4f9c-fghij", Namespace: "apps", Phase: "Running", Ready: true, Node: "worker-2"},
			{Name: "demo-app-7d4f9c-klmno", Namespace: "apps", Phase: "Pending", Ready: false, Node: ""},
		},
		Services: []liveService{
			{Name: "demo-app", Namespace: "apps", Port: 9090},
		},
	}

	objects, counts := buildWorkloadObjects(live)

	// 지어낸 값이 아니라 조회한 값이어야 한다.
	var deployment *workloadK8sObj
	for i := range objects {
		if objects[i].Kind == "Deployment" {
			deployment = &objects[i]
		}
	}
	require.NotNil(t, deployment)
	assert.Equal(t, int32(3), deployment.Replicas, "하드코딩 2 가 아니라 실제 replicas")

	var service *workloadK8sObj
	for i := range objects {
		if objects[i].Kind == "Service" {
			service = &objects[i]
		}
	}
	require.NotNil(t, service)
	assert.Equal(t, int32(9090), service.Port, "하드코딩 8080 이 아니라 실제 포트")

	podNames := []string{}
	for _, o := range objects {
		if o.Kind == "Pod" {
			podNames = append(podNames, o.Name)
		}
	}
	assert.Equal(t, []string{
		"demo-app-7d4f9c-abcde",
		"demo-app-7d4f9c-fghij",
		"demo-app-7d4f9c-klmno",
	}, podNames, "타임스탬프 해시가 아니라 실제 파드 이름")

	assert.Equal(t, 2, counts.Running)
	assert.Equal(t, 1, counts.Pending)
	assert.Equal(t, 0, counts.Failed)
}

// 클러스터를 못 읽으면 지어내지 않는다. 빈 목록을 준다.
func TestCollectWorkloads_ReturnsEmptyWhenClusterUnreachable(t *testing.T) {
	h := &StackHandler{
		collectWorkloadsFn: func(context.Context, []byte, string) (clusterWorkloads, error) {
			return clusterWorkloads{}, errors.New("dial tcp: connection refused")
		},
	}

	live, err := h.workloadsFor(context.Background(), []byte("kubeconfig"), "apps")

	require.NoError(t, err, "조회 실패는 이 엔드포인트의 실패가 아니다 — 파이프라인 목록은 여전히 보여야 한다")
	assert.Empty(t, live.Pods)
	assert.Empty(t, live.Deployments)
	assert.Empty(t, live.Services)
}

// kubeconfig 를 못 얻는 환경(개발용 DB만 있는 경우)에서도 500 이 아니라 빈 값이다.
func TestCollectWorkloads_ReturnsEmptyWithoutKubeconfig(t *testing.T) {
	h := &StackHandler{}

	live, err := h.workloadsFor(context.Background(), nil, "apps")

	require.NoError(t, err)
	assert.Empty(t, live.Pods)
}

// CrashLoopBackOff 처럼 Running 이 아닌 파드는 failed 로 센다.
func TestBuildWorkloadObjects_CountsFailedPods(t *testing.T) {
	live := clusterWorkloads{
		Pods: []livePod{
			{Name: "a", Phase: "Running", Ready: true},
			{Name: "b", Phase: "CrashLoopBackOff", Ready: false},
			{Name: "c", Phase: "Failed", Ready: false},
		},
	}

	_, counts := buildWorkloadObjects(live)

	assert.Equal(t, 1, counts.Running)
	assert.Equal(t, 0, counts.Pending)
	assert.Equal(t, 2, counts.Failed)
}

// 배포된 앱이 없으면 빈 목록이다 — 파드 2개를 만들어 내지 않는다.
func TestBuildWorkloadObjects_EmptyClusterYieldsNoObjects(t *testing.T) {
	objects, counts := buildWorkloadObjects(clusterWorkloads{})

	assert.Empty(t, objects)
	assert.Equal(t, 0, counts.Running+counts.Pending+counts.Failed)
}

// 배포한 앱이 자원을 얼마나 쓰는지도 보여야 한다.
//
// 스택 도구 파드는 이미 CPU/메모리를 보여주는데(monitoring_handler.go) CI/CD 로
// 배포한 앱만 상태 문자열뿐이었다. "Running" 만으로는 파드가 메모리 한계에
// 붙어 있는지, 놀고 있는지 알 수 없다.
func TestBuildWorkloadObjects_CarriesPodUsage(t *testing.T) {
	live := clusterWorkloads{
		Pods: []livePod{
			{Name: "demo-app-a", Phase: "Running", Ready: true,
				CPUMillicores: ptrInt64(37), MemoryMiB: ptrInt64(128)},
		},
	}

	objects, _ := buildWorkloadObjects(live)

	require.Len(t, objects, 1)
	require.NotNil(t, objects[0].CPUMillicores)
	assert.Equal(t, int64(37), *objects[0].CPUMillicores)
	require.NotNil(t, objects[0].MemoryMiB)
	assert.Equal(t, int64(128), *objects[0].MemoryMiB)
}

// metrics-server 가 없는 클러스터에서는 0 이 아니라 null 이다.
//
// 0 은 "안 쓰고 있다" 로 읽힌다. 못 읽은 것과 0 을 같은 값으로 두면 운영자가
// 놀고 있는 파드로 오해한다 — 이 파일이 replicas 2, 포트 8080 을 지어내던 것과
// 같은 종류의 거짓말이다.
func TestBuildWorkloadObjects_LeavesUsageNilWhenMetricsUnavailable(t *testing.T) {
	live := clusterWorkloads{
		Pods: []livePod{{Name: "demo-app-a", Phase: "Running", Ready: true}},
	}

	objects, _ := buildWorkloadObjects(live)

	require.Len(t, objects, 1)
	assert.Nil(t, objects[0].CPUMillicores, "못 읽었으면 0 이 아니라 없음이다")
	assert.Nil(t, objects[0].MemoryMiB)
}

// 사용량을 못 읽어도 워크로드 목록은 나와야 한다. metrics-server 는 선택 설치다.
func TestApplyPodUsage_KeepsPodsWhenMetricsMissing(t *testing.T) {
	live := clusterWorkloads{
		Pods: []livePod{
			{Name: "demo-app-a", Namespace: "apps", Phase: "Running", Ready: true},
			{Name: "demo-app-b", Namespace: "apps", Phase: "Running", Ready: true},
		},
	}

	applyPodUsage(&live, map[string]podResourceUsage{
		"apps/demo-app-a": {CPUMillicores: 12, MemoryMiB: 64},
	})

	require.Len(t, live.Pods, 2)
	require.NotNil(t, live.Pods[0].CPUMillicores)
	assert.Equal(t, int64(12), *live.Pods[0].CPUMillicores)
	assert.Nil(t, live.Pods[1].CPUMillicores, "그 파드만 못 읽은 것이지 목록이 비는 게 아니다")
}

// 같은 이름의 파드가 다른 네임스페이스에 있을 수 있다. 이름만으로 맞추면 섞인다.
func TestApplyPodUsage_MatchesOnNamespaceAndName(t *testing.T) {
	live := clusterWorkloads{
		Pods: []livePod{
			{Name: "demo-app-a", Namespace: "apps", Phase: "Running", Ready: true},
			{Name: "demo-app-a", Namespace: "staging", Phase: "Running", Ready: true},
		},
	}

	applyPodUsage(&live, map[string]podResourceUsage{
		"staging/demo-app-a": {CPUMillicores: 99, MemoryMiB: 512},
	})

	assert.Nil(t, live.Pods[0].CPUMillicores, "apps 네임스페이스 파드에 staging 값이 붙으면 안 된다")
	require.NotNil(t, live.Pods[1].CPUMillicores)
	assert.Equal(t, int64(99), *live.Pods[1].CPUMillicores)
}

func ptrInt64(v int64) *int64 { return &v }

// 파이프라인이 여럿이면 파드가 자기 앱에만 붙어야 한다.
func TestGroupWorkloadsByApp_SplitsPodsByLongestPrefix(t *testing.T) {
	live := clusterWorkloads{
		Deployments: []liveDeployment{
			{Name: "demo", Replicas: 1},
			{Name: "demo-api", Replicas: 1},
		},
		Pods: []livePod{
			{Name: "demo-7d4f9c-aaaaa", Phase: "Running", Ready: true},
			{Name: "demo-api-5b8c7d-bbbbb", Phase: "Running", Ready: true},
		},
	}

	grouped := groupWorkloadsByApp(live)

	require.Len(t, grouped["demo"].Pods, 1)
	require.Len(t, grouped["demo-api"].Pods, 1)
	assert.Equal(t, "demo-7d4f9c-aaaaa", grouped["demo"].Pods[0].Name,
		"demo-api 의 파드가 demo 로 새면 안 된다")
	assert.Equal(t, "demo-api-5b8c7d-bbbbb", grouped["demo-api"].Pods[0].Name)
}

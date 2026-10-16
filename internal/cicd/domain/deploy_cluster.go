package domain

import (
	"errors"
	"fmt"
	"strings"
)

// ErrStackClusterMismatch 는 스택에 묶인 앱에 스택과 다른 클러스터를 준 경우다.
// 스택의 CD 도구(Argo CD)는 자기 클러스터에만 배포하므로, 다른 클러스터는 배포 대상이 될 수 없다.
var ErrStackClusterMismatch = errors.New("pipeline cluster must be the stack's cluster")

// DeployClusterID 는 파이프라인의 앱이 실제로 서 있는 클러스터다. stackClusterID 는
// 묶인 스택의 클러스터이고, 모르면 빈 값이다.
//
// 스택에 묶인 파이프라인의 앱은 스택의 CD 도구가 올린다. Argo CD 의 Application 은
// 자기 클러스터(kubernetes.default.svc)를 향하고 Application 도 스택 클러스터에
// 적용되므로, 앱은 스택 클러스터에 선다 — 파이프라인에 어느 클러스터가 적혀 있든.
// 생성이 스택 클러스터만 받기 전에 만든 파이프라인에는 화면에서 고른 클러스터가
// 남아 있다.
//
// 플랫폼이 직접 적용한 경로(스택 없음·긴급 직접 배포)는 파이프라인의 클러스터에
// 올렸으니 그 값을 그대로 쓴다. 스택 클러스터를 모를 때도 저장된 값이 가장 나은 추정이다.
func (p *Pipeline) DeployClusterID(stackClusterID string) string {
	if p == nil {
		return ""
	}
	stackCluster := strings.TrimSpace(stackClusterID)
	if !p.DelegatesBuildToRunner() || stackCluster == "" {
		return p.ClusterID
	}
	return stackCluster
}

// MatchStackCluster 는 스택에 묶인 앱의 클러스터를 스택 클러스터와 맞춘다.
//
// 요청이 비어 있으면 스택 클러스터로 채우고, 다르면 ErrStackClusterMismatch 로
// 거절한다 — 조용히 바꾸면 요청한 쪽이 고른 클러스터가 거짓이 된다. 스택 클러스터를
// 모르면 요청을 그대로 둔다.
func MatchStackCluster(stackClusterID, requested string) (string, error) {
	stackCluster := strings.TrimSpace(stackClusterID)
	want := strings.TrimSpace(requested)
	switch {
	case stackCluster == "":
		return want, nil
	case want == "", want == stackCluster:
		return stackCluster, nil
	default:
		return "", fmt.Errorf("%w: stack is on cluster %s, got %s", ErrStackClusterMismatch, stackCluster, want)
	}
}

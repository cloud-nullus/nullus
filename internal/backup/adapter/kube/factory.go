package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Adapters 는 한 클러스터에 대한 백업 어댑터 묶음이다.
//
// 백업 대상 스택은 등록된 워크로드 클러스터에 있으므로, 어댑터는 기동 시점이
// 아니라 요청 시점에 그 클러스터의 kubeconfig 로 만들어야 한다.
type Adapters struct {
	Scaler    *WorkloadScaler
	Archiver  *VolumeArchiver
	Resources *ResourceDumper
}

// NewAdapters 는 kubeconfig 로 어댑터 묶음을 만든다.
func NewAdapters(kubeconfig []byte, helperImage string) (*Adapters, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("대상 클러스터의 kubeconfig 가 없습니다")
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig 해석: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("클러스터 클라이언트 생성: %w", err)
	}
	return &Adapters{
		Scaler:    NewWorkloadScaler(client),
		Archiver:  NewVolumeArchiver(client, cfg, helperImage),
		Resources: NewResourceDumper(kubeconfig),
	}, nil
}

// NewInCluster 는 컨트롤 플레인 자신의 클러스터용 어댑터를 만든다.
func NewInCluster(helperImage string) (*Adapters, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Adapters{
		Scaler:    NewWorkloadScaler(client),
		Archiver:  NewVolumeArchiver(client, cfg, helperImage),
		Resources: NewResourceDumper(nil),
	}, nil
}

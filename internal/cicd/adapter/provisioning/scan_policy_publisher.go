package provisioning

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// variablePolicyPublisher 는 앱 프로젝트마다 정책 변수를 싣는다(GitLab·GitHub).
//
// 프로젝트 변수는 파이프라인 파일보다 앞서므로, 이미 스캐폴딩한 파이프라인도
// 다음 실행부터 새 정책을 따른다.
type variablePolicyPublisher struct {
	writer port.PipelineVariableWriter
	// owner 는 앱 프로젝트가 속한 곳이다(GitLab 그룹, GitHub organization).
	owner string
}

func newVariablePolicyPublisher(writer port.PipelineVariableWriter, owner string) *variablePolicyPublisher {
	return &variablePolicyPublisher{writer: writer, owner: owner}
}

// PublishScanPolicy 는 앱마다 변수를 모두 싣는다. 하나라도 실패하면 그 앱은 실패다 —
// 심각도만 바뀌고 unfixed 는 옛 값인 파이프라인을 "적용됨" 으로 보고하면 안 된다.
func (p *variablePolicyPublisher) PublishScanPolicy(
	ctx context.Context,
	apps []string,
	vars []port.ProjectVariable,
) map[string]error {
	out := make(map[string]error, len(apps))
	for _, app := range apps {
		projectID := strings.TrimSpace(app)
		if owner := strings.Trim(strings.TrimSpace(p.owner), "/"); owner != "" {
			projectID = owner + "/" + projectID
		}
		var failed error
		for _, v := range vars {
			if err := p.writer.SetPipelineVariable(ctx, projectID, v.Key, v.Value); err != nil {
				failed = fmt.Errorf("%s 에 %s 를 싣지 못했습니다: %w", projectID, v.Key, err)
				break
			}
		}
		out[app] = failed
	}
	return out
}

// configMapPolicyPublisher 는 스택 네임스페이스에 정책 ConfigMap 하나를 적용한다(Jenkins).
//
// Jenkins 에는 CI 변수 저장소가 없다. 스캐너 컨테이너가 이 ConfigMap 을 envFrom 으로
// 읽으므로, 앱 수와 상관없이 스택마다 한 번만 적용하면 된다.
type configMapPolicyPublisher struct {
	applier     port.ManifestApplier
	kubeconfigs port.KubeconfigProvider
	clusterID   string
	namespace   string
}

func newConfigMapPolicyPublisher(
	applier port.ManifestApplier,
	kubeconfigs port.KubeconfigProvider,
	clusterID, namespace string,
) *configMapPolicyPublisher {
	return &configMapPolicyPublisher{applier: applier, kubeconfigs: kubeconfigs, clusterID: clusterID, namespace: namespace}
}

// PublishScanPolicy 는 ConfigMap 을 한 번 적용하고 그 결과를 모든 앱에 돌려준다 —
// 실패하면 그 스택의 Jenkins 파이프라인 모두가 옛 정책으로 돈다.
func (p *configMapPolicyPublisher) PublishScanPolicy(
	ctx context.Context,
	apps []string,
	vars []port.ProjectVariable,
) map[string]error {
	err := p.apply(ctx, vars)
	if err != nil {
		err = fmt.Errorf("스택 네임스페이스 %s 의 스캔 정책 ConfigMap 을 적용하지 못했습니다: %w", p.namespace, err)
	}
	out := make(map[string]error, len(apps))
	for _, app := range apps {
		out[app] = err
	}
	return out
}

func (p *configMapPolicyPublisher) apply(ctx context.Context, vars []port.ProjectVariable) error {
	manifest, err := kube.RenderScanPolicyConfigMap(p.namespace, vars)
	if err != nil {
		return err
	}
	var kubeconfig []byte
	if p.kubeconfigs != nil {
		kubeconfig, err = p.kubeconfigs.GetKubeconfig(ctx, p.clusterID)
		if err != nil {
			return fmt.Errorf("kubeconfig 조회 실패 (cluster %s): %w", p.clusterID, err)
		}
	}
	return p.applier.Apply(ctx, kubeconfig, []string{manifest})
}

// WithManifestApplier 는 CI 변수 저장소가 없는 스택(Gitea + Jenkins)에 스캔 정책
// ConfigMap 을 적용할 수단을 배선한다. 배선하지 않으면 그 스택은 정책을 실을 경로가
// 없고, 정책 저장 결과가 파이프라인마다 실패로 보고된다.
func (f *BundleFactory) WithManifestApplier(applier port.ManifestApplier, kubeconfigs port.KubeconfigProvider) *BundleFactory {
	f.applier = applier
	f.kubeconfigs = kubeconfigs
	return f
}

// scanPolicyPublisherFor 는 번들의 CI 에 맞는 정책 게시자를 고른다.
//
// GitLab·GitHub 은 파이프라인 설정을 쥔 클라이언트가 곧 변수 저장소다. 변수 저장소가
// 없고 Jenkins 가 배선된 스택은 ConfigMap 으로 간다. 둘 다 아니면 nil 이다 —
// 실을 곳이 없는데 게시자를 지어내면 "적용됨" 이 거짓이 된다.
func (f *BundleFactory) scanPolicyPublisherFor(bundle *port.SCMBundle) port.ScanPolicyPublisher {
	if bundle == nil {
		return nil
	}
	if writer, ok := bundle.Pipeline.(port.PipelineVariableWriter); ok {
		return newVariablePolicyPublisher(writer, bundle.GroupPath)
	}
	if bundle.CIJobs != nil && f.applier != nil && strings.TrimSpace(bundle.CDNamespace) != "" {
		return newConfigMapPolicyPublisher(f.applier, f.kubeconfigs, bundle.ClusterID, bundle.CDNamespace)
	}
	return nil
}

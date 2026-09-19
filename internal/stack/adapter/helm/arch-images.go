package helm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// imageComponent 는 차트가 컴포넌트 이미지를 받는 values 자리와 그 이미지 이름이다.
type imageComponent struct {
	path  []string
	image string
}

// archImageComponents 는 대체 출처로 이미지를 바꿀 때 채우는 차트 values 자리다.
//
// 어느 아키텍처가 어느 출처를 쓰는지는 도메인(domain.ToolImageProfile)이 정하고,
// 그 출처를 차트의 어느 키에 넣는지는 차트를 아는 여기서 정한다.
var archImageComponents = map[string][]imageComponent{
	// Harbor 차트 1.15.0 의 이미지 키다. trivy 어댑터는 설치 values 가 끄지만 사용자가
	// 켤 수 있어 함께 바꾼다. exporter 는 뺀다 — fork 의 v2.15.x 태그 빌드는 upstream
	// Dockerfile 이 GOARCH=amd64 로 고정돼 arm64 슬롯에도 amd64 바이너리가 들어 있다
	// (dasomel/harbor README Known Limitations). 설치는 Harbor metrics 를 켜지 않아
	// exporter 파드가 뜨지 않는다.
	"installing_harbor": {
		{path: []string{"nginx", "image"}, image: "nginx-photon"},
		{path: []string{"portal", "image"}, image: "harbor-portal"},
		{path: []string{"core", "image"}, image: "harbor-core"},
		{path: []string{"jobservice", "image"}, image: "harbor-jobservice"},
		{path: []string{"registry", "registry", "image"}, image: "registry-photon"},
		{path: []string{"registry", "controller", "image"}, image: "harbor-registryctl"},
		{path: []string{"database", "internal", "image"}, image: "harbor-db"},
		{path: []string{"redis", "internal", "image"}, image: "redis-photon"},
		{path: []string{"trivy", "image"}, image: "trivy-adapter-photon"},
	},
}

// archImageValuesForStep 은 노드 아키텍처에 맞는 이미지 values 를 돌려준다.
// 공식 이미지를 그대로 쓰면 nil 이다.
func archImageValuesForStep(step string, nodeArchs []string) map[string]any {
	profile, ok := domain.ToolImageProfileForStep(step)
	if !ok {
		return nil
	}
	source := profile.Resolve(nodeArchs).Source
	if source == nil {
		return nil
	}
	return archImageValues(archImageComponents[step], *source)
}

func archImageValues(components []imageComponent, source domain.ArchImageSource) map[string]any {
	if len(components) == 0 {
		return nil
	}
	registry := strings.TrimRight(source.Registry, "/")
	values := map[string]any{}
	for _, c := range components {
		node := values
		for _, key := range c.path[:len(c.path)-1] {
			child, ok := node[key].(map[string]any)
			if !ok {
				child = map[string]any{}
				node[key] = child
			}
			node = child
		}
		node[c.path[len(c.path)-1]] = map[string]any{
			"repository": registry + "/" + c.image,
			"tag":        source.Tag,
		}
	}
	return values
}

// PreflightArchitecture 는 설치를 시작하기 전에 노드 아키텍처를 읽고, 이번에 설치할
// 단계의 도구가 그 노드에서 뜰 이미지가 있는지 본다.
//
// 공식 이미지가 amd64 뿐인 Harbor 를 arm64 노드에 깔면 헬름은 성공하고 파드만
// 크래시한다. 그 실패는 설치가 한참 진행된 뒤에야 드러나므로 여기서 멈춘다.
// 대체 출처를 쓰게 되면 그 사실을 알림으로 돌려준다 — 공식이 아닌 이미지를 쓰는
// 것은 설치 로그에 남아야 한다.
//
// 이어서 진행하면 재개 지점 앞의 단계는 보지 않는다. 다시 설치하지 않는 도구 때문에
// 뒤 단계를 이어갈 수 없게 되면 안 된다.
//
// 노드를 읽지 못하면, 이미지가 노드 아키텍처에 달린 도구를 설치할 때만 멈춘다.
// 조용히 공식 이미지로 가면 arm64 에서 같은 실패가 재발한다.
func (o *Orchestrator) PreflightArchitecture(ctx context.Context, stackID string) ([]string, error) {
	if !o.canReadNodes() {
		return nil, nil
	}
	steps := o.pendingEnabledSteps(stackID)

	archs, err := o.loadNodeArchitectures(ctx)
	if err == nil && len(archs) == 0 {
		err = errors.New("노드에 아키텍처 정보가 없습니다")
	}
	if err != nil {
		if dependent := archDependentTools(steps); len(dependent) > 0 {
			return nil, fmt.Errorf(
				"설치 전 아키텍처 검사에서 멈췄습니다 — 노드 아키텍처를 읽지 못해 %s 이(가) 쓸 이미지를 고를 수 없습니다: %v. "+
					"공식 이미지로 설치하면 arm64 노드에서 뜨지 않을 수 있습니다. kubectl get nodes 가 되는지 확인한 뒤 다시 시도하세요",
				strings.Join(dependent, ", "), err)
		}
		return []string{fmt.Sprintf("노드 아키텍처를 읽지 못해 아키텍처 검사를 건너뜁니다: %v", err)}, nil
	}
	return checkStepArchitectures(steps, archs)
}

// pendingEnabledSteps 는 이번 실행에서 설치할 켜진 단계다. 재개 지점 앞은 이미 끝났다.
func (o *Orchestrator) pendingEnabledSteps(stackID string) []string {
	o.mu.Lock()
	completed, resumed := o.progress[stackID]
	o.mu.Unlock()
	if !resumed {
		completed = -1
	}

	var steps []string
	for _, step := range o.orderedStep {
		if o.stepOrder[step] <= completed {
			continue
		}
		if o.isStepEnabled(step) {
			steps = append(steps, step)
		}
	}
	return steps
}

// archDependentTools 는 단계 중 이미지가 노드 아키텍처에 달린 도구다("Harbor(installing_harbor)").
func archDependentTools(steps []string) []string {
	var tools []string
	for _, step := range steps {
		if p, ok := domain.ToolImageProfileForStep(step); ok {
			tools = append(tools, fmt.Sprintf("%s(%s)", p.Tool, step))
		}
	}
	return tools
}

func checkStepArchitectures(steps, nodeArchs []string) ([]string, error) {
	return checkStepArchitecturesWith(steps, nodeArchs, domain.ToolImageProfileForStep)
}

func checkStepArchitecturesWith(steps, nodeArchs []string, profileFor func(string) (domain.ToolImageProfile, bool)) ([]string, error) {
	var notices, problems []string
	for _, step := range steps {
		profile, ok := profileFor(step)
		if !ok {
			continue
		}
		resolved := profile.Resolve(nodeArchs)
		switch {
		case len(resolved.Unsupported) > 0:
			problems = append(problems, fmt.Sprintf(
				"%s(%s) 는 노드 아키텍처 %s 에서 뜰 이미지가 없습니다(지원: %s)",
				profile.Tool, step, strings.Join(resolved.Unsupported, ", "), strings.Join(profile.SupportedArchs(), ", ")))
		case resolved.Source == nil:
			// 공식 이미지로 충분하다.
		case len(archImageComponents[step]) == 0:
			problems = append(problems, fmt.Sprintf(
				"%s(%s) 는 노드 아키텍처 %s 에 대체 이미지 출처가 선언돼 있지만 설치가 차트의 이미지 자리를 모릅니다",
				profile.Tool, step, strings.Join(nodeArchs, ", ")))
		default:
			src := resolved.Source
			notices = append(notices, fmt.Sprintf(
				"%s: 공식 이미지가 노드 아키텍처 %s 를 내지 않아 %s/*:%s 멀티아키 이미지로 설치합니다 (출처 %s)",
				profile.Tool, strings.Join(nodeArchs, ", "), src.Registry, src.Tag, src.Reference))
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf(
			"설치 전 아키텍처 검사에서 멈췄습니다 — %s. "+
				"이대로 설치하면 헬름은 성공하고 파드가 이미지 아키텍처 불일치로 뜨지 않습니다. "+
				"지원 아키텍처의 노드로 클러스터를 구성하거나 다른 도구를 고르세요",
			strings.Join(problems, "; "))
	}
	return notices, nil
}

// loadNodeArchitectures 는 노드 아키텍처를 한 번 읽어 둔다. 실패는 캐시하지 않는다.
func (o *Orchestrator) loadNodeArchitectures(ctx context.Context) ([]string, error) {
	o.mu.Lock()
	if o.nodeArchsLoaded {
		archs := slices.Clone(o.nodeArchs)
		o.mu.Unlock()
		return archs, nil
	}
	o.mu.Unlock()

	out, err := o.readNodes(ctx)
	if err != nil {
		return nil, err
	}
	archs, err := parseNodeArchitectures(out)
	if err != nil {
		return nil, err
	}
	o.setNodeArchitectures(archs)
	return archs, nil
}

// canReadNodes 는 노드를 읽을 수단이 있는지 본다. kubeconfig 가 없으면(로컬 단위
// 테스트 경로) 노드를 모르는 채로 지금처럼 공식 이미지를 쓴다.
func (o *Orchestrator) canReadNodes() bool {
	return o.nodeReader != nil || looksLikeKubeconfig(o.kubeconfig)
}

// readNodes 는 `kubectl get nodes -o json` 의 표준출력이다.
//
// 표준에러는 섞지 않는다. 죽은 aggregated APIService(예: 스택 삭제 뒤 남은
// v1beta1.metrics.k8s.io)가 있으면 kubectl 이 discovery 경고를 stderr 에 찍는데,
// 섞으면 JSON 이 깨져 노드를 못 읽은 것이 된다.
func (o *Orchestrator) readNodes(ctx context.Context) ([]byte, error) {
	if o.nodeReader != nil {
		return o.nodeReader(ctx)
	}
	return o.runKubectlStdout(ctx, "get", "nodes", "-o", "json")
}

func (o *Orchestrator) setNodeArchitectures(archs []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.nodeArchs = slices.Clone(archs)
	o.nodeArchsLoaded = true
}

func (o *Orchestrator) nodeArchitectures() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.nodeArchs)
}

// parseNodeArchitectures 는 `kubectl get nodes -o json` 에서 아키텍처를 모은다(정렬·중복 제거).
func parseNodeArchitectures(raw []byte) ([]string, error) {
	var list struct {
		Items []struct {
			Status struct {
				NodeInfo struct {
					Architecture string `json:"architecture"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}
	var archs []string
	for _, item := range list.Items {
		a := strings.TrimSpace(item.Status.NodeInfo.Architecture)
		if a != "" && !slices.Contains(archs, a) {
			archs = append(archs, a)
		}
	}
	slices.Sort(archs)
	return archs, nil
}

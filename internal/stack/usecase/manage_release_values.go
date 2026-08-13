package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 배포가 끝난 스택의 OSS 설정을 values.yaml 수준에서 고쳐 다시 적용한다.
// (기능분해도 NULLUS_DSS_040_040 — 스택 설정 수정 및 재배포)
//
// 편집 대상은 두 가지 중 하나다.
//
//   - live     : 릴리스에 실제로 배포된 values 전체. 보이는 그대로가 배포값이라
//     직관적이지만 플랫폼이 계산해 넣은 값까지 함께 노출된다.
//   - override : 사용자가 얹은 커스텀만. 안전하지만 지금 무엇이 적용돼 있는지는
//     보이지 않는다.
//
// 어느 쪽으로 편집하든 결과는 두 곳에 남는다. 클러스터에는 helm upgrade 로,
// 스택 설정에는 YAMLOverrides 로. 후자가 빠지면 다음 재배포·재시도에서 설치
// 경로가 값을 처음부터 다시 계산하면서 사용자의 편집이 조용히 사라진다.

// ReleaseValuesMode 는 편집 단위를 고른다.
type ReleaseValuesMode string

const (
	// ReleaseValuesModeLive 는 배포된 values 전체를 편집한다.
	ReleaseValuesModeLive ReleaseValuesMode = "live"
	// ReleaseValuesModeOverride 는 사용자 오버라이드만 편집한다.
	ReleaseValuesModeOverride ReleaseValuesMode = "override"
)

var (
	// ErrReleaseNotFound 는 클러스터에 그 릴리스가 없을 때다.
	ErrReleaseNotFound = errors.New("release not found")
	// ErrReleaseValuesInvalidYAML 은 편집본이 YAML 매핑으로 읽히지 않을 때다.
	ErrReleaseValuesInvalidYAML = errors.New("invalid yaml values")
	// ErrReleaseValuesInvalidMode 는 모드 값이 live/override 가 아닐 때다.
	ErrReleaseValuesInvalidMode = errors.New("invalid release values mode")
)

// ReleaseManagerFactory 는 대상 클러스터의 kubeconfig 로 릴리스 관리자를 만든다.
type ReleaseManagerFactory func(kubeconfig []byte) port.HelmReleaseManager

// ManageReleaseValues 는 배포된 릴리스의 values 를 읽고 다시 적용한다.
type ManageReleaseValues struct {
	stackRepo          port.StackRepository
	kubeconfigProvider port.KubeconfigProvider
	managerFactory     ReleaseManagerFactory
	manageHistory      *ManageHistory
}

// ManageReleaseValuesOption 은 선택 의존성을 주입한다.
type ManageReleaseValuesOption func(*ManageReleaseValues)

// WithReleaseValuesHistory 를 붙이면 적용 직전 설정이 이력으로 남는다.
func WithReleaseValuesHistory(history *ManageHistory) ManageReleaseValuesOption {
	return func(uc *ManageReleaseValues) { uc.manageHistory = history }
}

// NewManageReleaseValues 는 유스케이스를 조립한다.
func NewManageReleaseValues(
	stackRepo port.StackRepository,
	kubeconfigProvider port.KubeconfigProvider,
	managerFactory ReleaseManagerFactory,
	opts ...ManageReleaseValuesOption,
) *ManageReleaseValues {
	uc := &ManageReleaseValues{
		stackRepo:          stackRepo,
		kubeconfigProvider: kubeconfigProvider,
		managerFactory:     managerFactory,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// GetReleaseValuesInput 은 values 조회 파라미터다.
type GetReleaseValuesInput struct {
	StackID     string
	ReleaseName string
	Mode        ReleaseValuesMode
}

// ReleaseValuesOutput 은 에디터에 실어 줄 YAML 과 그 출처다.
type ReleaseValuesOutput struct {
	ReleaseName string            `json:"release_name"`
	StepName    string            `json:"step_name,omitempty"`
	Namespace   string            `json:"namespace"`
	Revision    int               `json:"revision"`
	Mode        ReleaseValuesMode `json:"mode"`
	YAML        string            `json:"yaml"`
	// ProtectedPaths 는 이 릴리스에서 플랫폼이 소유한 경로다. 에디터가 미리
	// 표시해 두면 사용자가 건드리기 전에 알 수 있다.
	ProtectedPaths []string `json:"protected_paths,omitempty"`
}

// ApplyReleaseValuesInput 은 적용(또는 드라이런) 파라미터다.
type ApplyReleaseValuesInput struct {
	StackID     string
	ReleaseName string
	Mode        ReleaseValuesMode
	YAML        string
	DryRun      bool
	ChangedBy   string
}

// ApplyReleaseValuesOutput 은 적용 결과다.
type ApplyReleaseValuesOutput struct {
	ReleaseName string                           `json:"release_name"`
	StepName    string                           `json:"step_name,omitempty"`
	Namespace   string                           `json:"namespace"`
	Mode        ReleaseValuesMode                `json:"mode"`
	Revision    int                              `json:"revision"`
	Status      string                           `json:"status,omitempty"`
	DryRun      bool                             `json:"dry_run"`
	Warnings    []domain.ProtectedValueViolation `json:"warnings,omitempty"`
	// EffectiveYAML 은 실제로 helm 에 넘어간 values 다. 오버라이드 모드에서
	// 병합 결과를 확인하는 용도이므로 드라이런에서 특히 중요하다.
	EffectiveYAML string `json:"effective_yaml,omitempty"`
	Manifest      string `json:"manifest,omitempty"`
	// RenderError 는 미리보기에서만 채워진다. 차트가 렌더되지 않는 것은
	// 편집 결과이지 서버 오류가 아니므로, 경고와 함께 돌려준다.
	RenderError string `json:"render_error,omitempty"`
}

// ListReleases 는 스택 네임스페이스에 올라가 있는 Helm 릴리스를 돌려준다.
func (uc *ManageReleaseValues) ListReleases(ctx context.Context, stackID string) ([]port.ReleaseInfo, error) {
	_, manager, namespace, err := uc.resolve(ctx, stackID)
	if err != nil {
		return nil, err
	}
	return manager.ListReleases(ctx, namespace)
}

// GetValues 는 선택한 모드에 맞는 편집 원본을 돌려준다.
func (uc *ManageReleaseValues) GetValues(ctx context.Context, input GetReleaseValuesInput) (*ReleaseValuesOutput, error) {
	mode, err := normalizeReleaseValuesMode(input.Mode)
	if err != nil {
		return nil, err
	}

	stack, manager, namespace, err := uc.resolve(ctx, input.StackID)
	if err != nil {
		return nil, err
	}

	release, err := uc.findRelease(ctx, manager, namespace, input.ReleaseName)
	if err != nil {
		return nil, err
	}

	out := &ReleaseValuesOutput{
		ReleaseName:    release.ReleaseName,
		StepName:       release.StepName,
		Namespace:      release.Namespace,
		Revision:       release.Revision,
		Mode:           mode,
		ProtectedPaths: domain.ProtectedValuePaths(release.StepName),
	}
	if out.Namespace == "" {
		out.Namespace = namespace
	}

	if mode == ReleaseValuesModeOverride {
		cfg, _ := stackConfigFromInterface(stack.Config)
		out.YAML = cfg.YAMLOverrides[overrideKeyForRelease(release)]
		return out, nil
	}

	values, err := manager.GetValues(ctx, release.ReleaseName, out.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get release values: %w", err)
	}
	encoded, err := marshalValues(values)
	if err != nil {
		return nil, err
	}
	out.YAML = encoded
	return out, nil
}

// Apply 는 편집본을 클러스터에 적용하고 스택 설정에 반영한다.
//
// 순서가 중요하다. helm upgrade 가 먼저다 — 적용에 실패한 설정이 DB 에 남으면
// 다음 재배포가 검증된 적 없는 값을 들고 나간다.
func (uc *ManageReleaseValues) Apply(ctx context.Context, input ApplyReleaseValuesInput) (*ApplyReleaseValuesOutput, error) {
	mode, err := normalizeReleaseValuesMode(input.Mode)
	if err != nil {
		return nil, err
	}

	stack, manager, namespace, err := uc.resolve(ctx, input.StackID)
	if err != nil {
		return nil, err
	}

	release, err := uc.findRelease(ctx, manager, namespace, input.ReleaseName)
	if err != nil {
		return nil, err
	}
	releaseNamespace := release.Namespace
	if releaseNamespace == "" {
		releaseNamespace = namespace
	}

	edited, err := parseValues(input.YAML)
	if err != nil {
		return nil, err
	}
	if mode == ReleaseValuesModeLive && len(edited) == 0 {
		return nil, fmt.Errorf("%w: values 전체를 비울 수는 없다", ErrReleaseValuesInvalidYAML)
	}

	live, err := manager.GetValues(ctx, release.ReleaseName, releaseNamespace)
	if err != nil {
		return nil, fmt.Errorf("get release values: %w", err)
	}

	effective := edited
	if mode == ReleaseValuesModeOverride {
		// 오버라이드는 현재 배포값 위에 얹는다. 누적이라는 뜻이다 — 오버라이드에서
		// 키를 지워도 이미 적용된 값은 되돌아가지 않는다. 되돌리려면 live 모드에서
		// 그 키를 직접 지워야 한다. 대신 플랫폼이 계산해 넣은 값을 이 경로가
		// 실수로 날려 버리는 일은 절대 없다.
		effective = mergeValueMaps(deepCopyValues(live), edited)
	}

	effectiveYAML, err := marshalValues(effective)
	if err != nil {
		return nil, err
	}

	out := &ApplyReleaseValuesOutput{
		ReleaseName:   release.ReleaseName,
		StepName:      release.StepName,
		Namespace:     releaseNamespace,
		Mode:          mode,
		DryRun:        input.DryRun,
		Warnings:      domain.ProtectedValueViolations(release.StepName, live, effective),
		EffectiveYAML: effectiveYAML,
	}

	if !input.DryRun {
		// 직전 설정을 이력에 남긴다. 실패해도 적용을 막지는 않는다 —
		// 이력은 편의이고, 사용자가 요청한 것은 설정 변경이다.
		uc.snapshot(ctx, stack, release.ReleaseName, input.ChangedBy)
	}

	result, err := manager.Upgrade(ctx, port.HelmUpgradeRequest{
		ReleaseName: release.ReleaseName,
		Namespace:   releaseNamespace,
		Values:      effective,
		DryRun:      input.DryRun,
	})
	if err != nil {
		// 미리보기는 "이렇게 바꾸면 어떻게 되나" 를 묻는 것이다. 렌더가 깨지는
		// 것도 답의 하나이므로, 함께 계산해 둔 보호 경로 경고와 같이 돌려준다.
		// 여기서 에러로 던지면 무엇을 잘못 건드렸는지가 함께 사라진다.
		if input.DryRun {
			out.RenderError = err.Error()
			return out, nil
		}
		return nil, fmt.Errorf("apply release values: %w", err)
	}
	if result != nil {
		out.Revision = result.Revision
		out.Status = result.Status
		out.Manifest = result.Manifest
	}

	if input.DryRun {
		return out, nil
	}

	// 재배포 때 같은 값이 다시 나오도록 스택 설정에 남긴다. live 모드는 편집한
	// values 전체가, override 모드는 사용자가 적은 것만 오버라이드가 된다.
	//
	// 편집본을 그대로 저장한다 — 다시 직렬화하면 주석과 줄 순서가 사라져,
	// 다음에 열었을 때 사용자가 쓴 문서가 아닌 것이 나온다.
	if err := uc.persistOverride(ctx, stack, overrideKeyForRelease(release), input.YAML); err != nil {
		return nil, err
	}

	return out, nil
}

// resolve 는 스택 → kubeconfig → 릴리스 관리자까지 한 번에 푼다.
func (uc *ManageReleaseValues) resolve(ctx context.Context, stackID string) (*domain.Stack, port.HelmReleaseManager, string, error) {
	if strings.TrimSpace(stackID) == "" {
		return nil, nil, "", fmt.Errorf("stack_id is required")
	}
	if uc.kubeconfigProvider == nil || uc.managerFactory == nil {
		return nil, nil, "", fmt.Errorf("release values 기능이 구성되지 않았다")
	}

	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("get stack: %w", err)
	}
	if stack == nil {
		return nil, nil, "", ErrStackNotFound
	}

	kubeconfig, err := uc.kubeconfigProvider.GetKubeconfig(ctx, stack.ClusterID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load kubeconfig: %w", err)
	}
	if len(kubeconfig) == 0 {
		return nil, nil, "", fmt.Errorf("클러스터 %s 에 kubeconfig 가 등록되어 있지 않다", stack.ClusterID)
	}

	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		namespace = "nullus"
	}

	return stack, uc.managerFactory(kubeconfig), namespace, nil
}

func (uc *ManageReleaseValues) findRelease(ctx context.Context, manager port.HelmReleaseManager, namespace, releaseName string) (port.ReleaseInfo, error) {
	name := strings.TrimSpace(releaseName)
	if name == "" {
		return port.ReleaseInfo{}, fmt.Errorf("release_name is required")
	}

	releases, err := manager.ListReleases(ctx, namespace)
	if err != nil {
		return port.ReleaseInfo{}, fmt.Errorf("list releases: %w", err)
	}
	for _, release := range releases {
		if release.ReleaseName == name {
			return release, nil
		}
	}
	return port.ReleaseInfo{}, fmt.Errorf("%w: %s", ErrReleaseNotFound, name)
}

func (uc *ManageReleaseValues) snapshot(ctx context.Context, stack *domain.Stack, releaseName, changedBy string) {
	if uc.manageHistory == nil {
		return
	}
	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok {
		return
	}
	if strings.TrimSpace(changedBy) == "" {
		changedBy = "system"
	}
	_, _ = uc.manageHistory.SaveVersion(ctx, SaveVersionInput{
		StackID:      stack.ID,
		Config:       cfg,
		ChangedBy:    changedBy,
		ChangeReason: "pre-update snapshot: " + releaseName + " values",
	})
}

func (uc *ManageReleaseValues) persistOverride(ctx context.Context, stack *domain.Stack, key, yamlText string) error {
	cfg, _ := stackConfigFromInterface(stack.Config)

	if strings.TrimSpace(yamlText) == "" {
		// 빈 편집본은 "커스텀을 걷어낸다" 는 뜻이다. 빈 문자열을 남겨 두면
		// 설치 경로가 오버라이드가 있는 줄 알고 헛돈다.
		delete(cfg.YAMLOverrides, key)
	} else {
		if cfg.YAMLOverrides == nil {
			cfg.YAMLOverrides = map[string]string{}
		}
		cfg.YAMLOverrides[key] = yamlText
	}

	stack.Config = cfg
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		// 클러스터에는 이미 적용된 뒤다. 사용자가 상황을 알 수 있도록 그대로 말한다.
		return fmt.Errorf("클러스터에는 적용됐으나 스택 설정 저장에 실패했다 — 재배포 시 편집이 유실된다: %w", err)
	}
	return nil
}

// overrideKeyForRelease 는 편집본을 저장할 YAMLOverrides 키를 고른다.
//
// 설치 경로(valuesForStep)는 step → releaseName → chartName 순으로 키를 찾으므로
// 단계 이름이 가장 정확하다. 매핑을 모르는 릴리스는 릴리스 이름으로 떨어진다.
func overrideKeyForRelease(release port.ReleaseInfo) string {
	if step := strings.TrimSpace(release.StepName); step != "" {
		return step
	}
	return release.ReleaseName
}

func normalizeReleaseValuesMode(mode ReleaseValuesMode) (ReleaseValuesMode, error) {
	switch ReleaseValuesMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case "", ReleaseValuesModeLive:
		return ReleaseValuesModeLive, nil
	case ReleaseValuesModeOverride:
		return ReleaseValuesModeOverride, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrReleaseValuesInvalidMode, mode)
	}
}

func parseValues(text string) (map[string]any, error) {
	if strings.TrimSpace(text) == "" {
		return map[string]any{}, nil
	}

	var parsed any
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReleaseValuesInvalidYAML, err)
	}
	if parsed == nil {
		return map[string]any{}, nil
	}

	values, ok := normalizeYAMLNode(parsed).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: helm values 는 최상위가 매핑이어야 한다", ErrReleaseValuesInvalidYAML)
	}
	return values, nil
}

// TopLevelValuePaths 는 편집본이 건드린 최상위 키를 정렬해 돌려준다.
//
// 감사 로그에 "무엇을 바꿨나" 를 남기되 값은 남기지 않기 위한 것이다. values
// 에는 사용자가 직접 적어 넣은 자격증명이 들어갈 수 있고, 감사 로그는 그보다
// 넓게 읽힌다. 값 자체는 스택 설정 이력(stack_config_versions)에 남으므로
// 되짚을 수 있다.
//
// 파싱되지 않는 편집본이면 빈 슬라이스다 — 감사 기록이 파싱 실패로 통째로
// 사라지면 안 되므로 여기서 에러를 올리지 않는다.
func TopLevelValuePaths(yamlText string) []string {
	values, err := parseValues(yamlText)
	if err != nil {
		return []string{}
	}
	paths := make([]string, 0, len(values))
	for key := range values {
		paths = append(paths, key)
	}
	sort.Strings(paths)
	return paths
}

func marshalValues(values map[string]any) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	encoded, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode values: %w", err)
	}
	return string(encoded), nil
}

// normalizeYAMLNode 는 yaml.v3 가 만들어 내는 map[any]any 를 map[string]any 로
// 맞춘다. 그대로 두면 helm 이 values 를 JSON 으로 다룰 때 깨진다.
func normalizeYAMLNode(node any) any {
	switch typed := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeYAMLNode(value)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprintf("%v", key)] = normalizeYAMLNode(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeYAMLNode(value)
		}
		return out
	default:
		return node
	}
}

// mergeValueMaps 는 override 를 base 위에 깊게 얹는다. 설치 경로의 병합 규칙과
// 같아야 한다 — 여기서만 다르게 합치면 편집 결과와 재배포 결과가 갈린다.
func mergeValueMaps(base, override map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range override {
		subOverride, ok := value.(map[string]any)
		if !ok {
			base[key] = value
			continue
		}
		subBase, _ := base[key].(map[string]any)
		base[key] = mergeValueMaps(subBase, subOverride)
	}
	return base
}

func deepCopyValues(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		if nested, ok := value.(map[string]any); ok {
			out[key] = deepCopyValues(nested)
			continue
		}
		out[key] = value
	}
	return out
}

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

type DiffVersionsInput struct {
	StackID  string
	VersionA int
	VersionB int
}

type DiffResult struct {
	Added   map[string]any    `json:"added"`
	Removed map[string]any    `json:"removed"`
	Changed map[string][2]any `json:"changed"`
}

type DiffVersions struct {
	historyRepo port.HistoryRepository
}

func NewDiffVersions(historyRepo port.HistoryRepository) *DiffVersions {
	return &DiffVersions{historyRepo: historyRepo}
}

func (uc *DiffVersions) Execute(ctx context.Context, input DiffVersionsInput) (*DiffResult, error) {
	if input.StackID == "" {
		return nil, fmt.Errorf("stack_id is required")
	}
	if input.VersionA <= 0 || input.VersionB <= 0 {
		return nil, fmt.Errorf("versionA and versionB must be positive")
	}

	versions, err := uc.historyRepo.ListVersions(ctx, input.StackID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	byVersion := make(map[int]map[string]any, len(versions))
	for _, version := range versions {
		cfgMap, convErr := configToMap(version.Config)
		if convErr != nil {
			return nil, fmt.Errorf("convert config version %d: %w", version.Version, convErr)
		}
		byVersion[version.Version] = cfgMap
	}

	a, ok := byVersion[input.VersionA]
	if !ok {
		return nil, fmt.Errorf("version %d not found", input.VersionA)
	}
	b, ok := byVersion[input.VersionB]
	if !ok {
		return nil, fmt.Errorf("version %d not found", input.VersionB)
	}

	flatA := make(map[string]any)
	flatB := make(map[string]any)
	flattenMeaningful("", a, flatA)
	flattenMeaningful("", b, flatB)

	result := &DiffResult{
		Added:   make(map[string]any),
		Removed: make(map[string]any),
		Changed: make(map[string][2]any),
	}

	keys := make(map[string]struct{}, len(flatA)+len(flatB))
	for k := range flatA {
		keys[k] = struct{}{}
	}
	for k := range flatB {
		keys[k] = struct{}{}
	}

	for k := range keys {
		av, aok := flatA[k]
		bv, bok := flatB[k]
		switch {
		case !aok && bok:
			result.Added[k] = bv
		case aok && !bok:
			result.Removed[k] = av
		case !reflect.DeepEqual(av, bv):
			result.Changed[k] = [2]any{av, bv}
		}
	}

	return result, nil
}

func configToMap(config any) (map[string]any, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func flattenMeaningful(prefix string, input map[string]any, out map[string]any) {
	for k, value := range input {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}

		nested, ok := value.(map[string]any)
		if ok {
			flattenMeaningful(path, nested, out)
			continue
		}

		if shouldKeepValue(value) {
			out[path] = value
		}
	}
}

func shouldKeepValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case float64:
		return math.Abs(v) > 0
	case float32:
		return math.Abs(float64(v)) > 0
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case []any:
		return len(v) > 0
	default:
		return true
	}
}

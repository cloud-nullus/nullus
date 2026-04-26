package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// MemoryHistoryRepository is an in-memory implementation of port.HistoryRepository.
type MemoryHistoryRepository struct {
	mu       sync.RWMutex
	versions map[string][]*domain.StackVersion // keyed by stackID
	byID     map[string]*domain.StackVersion   // keyed by version.ID
}

// NewMemoryHistoryRepository constructs an empty MemoryHistoryRepository.
func NewMemoryHistoryRepository() *MemoryHistoryRepository {
	return &MemoryHistoryRepository{
		versions: make(map[string][]*domain.StackVersion),
		byID:     make(map[string]*domain.StackVersion),
	}
}

// SaveVersion persists a new stack version snapshot.
func (r *MemoryHistoryRepository) SaveVersion(_ context.Context, version *domain.StackVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[version.ID]; ok {
		return fmt.Errorf("version %q already exists", version.ID)
	}
	cp := *version
	r.versions[version.StackID] = append(r.versions[version.StackID], &cp)
	r.byID[version.ID] = &cp
	return nil
}

// ListVersions returns all versions for a stack, sorted by version number ascending.
func (r *MemoryHistoryRepository) ListVersions(_ context.Context, stackID string) ([]*domain.StackVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	raw := r.versions[stackID]
	result := make([]*domain.StackVersion, len(raw))
	for i, v := range raw {
		cp := *v
		result[i] = &cp
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

// GetVersion returns a specific version by its ID, scoped to a stack.
func (r *MemoryHistoryRepository) GetVersion(_ context.Context, stackID, versionID string) (*domain.StackVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.byID[versionID]
	if !ok || v.StackID != stackID {
		return nil, fmt.Errorf("version %q not found for stack %q", versionID, stackID)
	}
	cp := *v
	return &cp, nil
}

// GetDiff computes the diff between the given version and the one immediately preceding it.
// If there is no previous version, all fields are reported as new (OldValue = "").
func (r *MemoryHistoryRepository) GetDiff(_ context.Context, stackID, versionID string) ([]domain.ConfigDiff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target, ok := r.byID[versionID]
	if !ok || target.StackID != stackID {
		return nil, fmt.Errorf("version %q not found for stack %q", versionID, stackID)
	}

	// Find previous version
	versions := r.versions[stackID]
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version < versions[j].Version
	})

	var prev *domain.StackVersion
	for _, v := range versions {
		if v.Version < target.Version {
			prev = v
		}
	}

	return computeDiff(prev, target), nil
}

// computeDiff returns field-level diffs between two StackConfig snapshots.
// If prev is nil, all fields in target are treated as new.
func computeDiff(prev, target *domain.StackVersion) []domain.ConfigDiff {
	var diffs []domain.ConfigDiff

	newFields := flattenConfig(target.Config)
	var oldFields map[string]string
	if prev != nil {
		oldFields = flattenConfig(prev.Config)
	} else {
		oldFields = make(map[string]string)
	}

	// Collect all keys
	keys := make(map[string]struct{})
	for k := range newFields {
		keys[k] = struct{}{}
	}
	for k := range oldFields {
		keys[k] = struct{}{}
	}

	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		oldVal := oldFields[k]
		newVal := newFields[k]
		if oldVal != newVal {
			diffs = append(diffs, domain.ConfigDiff{
				Field:    k,
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
	}
	return diffs
}

// flattenConfig serializes a StackConfig to a flat string map for comparison.
func flattenConfig(cfg domain.StackConfig) map[string]string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return map[string]string{}
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string)
	flattenMap("", raw, result)
	return result
}

func flattenMap(prefix string, m map[string]any, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenMap(key, val, out)
		default:
			out[key] = fmt.Sprintf("%v", val)
		}
	}
}

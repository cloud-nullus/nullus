package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"
)

// VerdictCache is the interface ValidateCompatibility consults before
// running the actual matrix match + arch check. Phase 6 of the F8 follow-up:
// the Install Wizard re-submits after every edit, and identical
// (stack tools, cluster, matrix set) tuples produce identical verdicts, so
// a short-TTL cache cuts duplicate work.
//
// Implementations MUST be safe for concurrent use — Get and Put can be
// called from multiple goroutines on shared keys.
type VerdictCache interface {
	Get(key string) (*ValidateCompatibilityOutput, bool)
	Put(key string, out *ValidateCompatibilityOutput)
	// Invalidate evicts all entries whose key starts with the given prefix.
	// Used when stack/cluster/matrix state changes; callers can also pass
	// an empty string to blow the whole cache (the initial implementation
	// uses a simple full-clear to avoid prefix-matching complexity).
	Invalidate(prefix string)
}

// MemoryVerdictCache is an in-process TTL cache. sync.Map keeps the
// lookup path lock-free; expired entries are removed lazily on Get.
type MemoryVerdictCache struct {
	ttl time.Duration
	m   sync.Map // key(string) -> *verdictEntry
}

type verdictEntry struct {
	out       *ValidateCompatibilityOutput
	expiresAt time.Time
}

// NewMemoryVerdictCache builds a cache with the given TTL. A zero or
// negative TTL disables the cache (Get always misses).
func NewMemoryVerdictCache(ttl time.Duration) *MemoryVerdictCache {
	return &MemoryVerdictCache{ttl: ttl}
}

// Get returns a cached verdict when present and not yet expired.
func (c *MemoryVerdictCache) Get(key string) (*ValidateCompatibilityOutput, bool) {
	if c.ttl <= 0 {
		return nil, false
	}
	raw, ok := c.m.Load(key)
	if !ok {
		return nil, false
	}
	entry := raw.(*verdictEntry)
	if time.Now().After(entry.expiresAt) {
		c.m.Delete(key)
		return nil, false
	}
	return entry.out, true
}

// Put stores a verdict under key with the configured TTL.
func (c *MemoryVerdictCache) Put(key string, out *ValidateCompatibilityOutput) {
	if c.ttl <= 0 || out == nil {
		return
	}
	c.m.Store(key, &verdictEntry{out: out, expiresAt: time.Now().Add(c.ttl)})
}

// Invalidate drops entries whose key starts with prefix. Passing an empty
// prefix clears the entire cache — the initial implementation's simple
// invalidation strategy.
func (c *MemoryVerdictCache) Invalidate(prefix string) {
	c.m.Range(func(k, _ any) bool {
		if prefix == "" || strings.HasPrefix(k.(string), prefix) {
			c.m.Delete(k)
		}
		return true
	})
}

// Clear drops every cached entry. F8-Phase5 matrix CRUD calls this after
// any Create/Update/Delete succeeds, since changing a matrix can affect
// every cached verdict regardless of stack or cluster.
func (c *MemoryVerdictCache) Clear() {
	c.Invalidate("")
}

// VerdictCacheKey derives a stable key for a validate input. Any
// field that influences the verdict is folded into a sha256 so matrix
// drift / tool edits / cluster arch changes each produce a new key.
func VerdictCacheKey(input ValidateCompatibilityInput) string {
	h := sha256.New()
	if input.StackID != "" {
		h.Write([]byte("stack:" + input.StackID + "|"))
	}
	if input.ClusterID != "" {
		h.Write([]byte("cluster:" + input.ClusterID + "|"))
	}
	if len(input.NodeArchitectures) > 0 {
		archs := append([]string(nil), input.NodeArchitectures...)
		sort.Strings(archs)
		h.Write([]byte("archs:" + strings.Join(archs, ",") + "|"))
	}
	if len(input.Tools) > 0 {
		keys := make([]string, 0, len(input.Tools))
		for k := range input.Tools {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k + "=" + input.Tools[k] + ";"))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

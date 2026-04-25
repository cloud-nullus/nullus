package usecase

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryVerdictCache_HitAndMiss(t *testing.T) {
	c := NewMemoryVerdictCache(30 * time.Second)
	out := &ValidateCompatibilityOutput{Compatible: true, Message: "pass"}

	_, ok := c.Get("k1")
	assert.False(t, ok, "miss on empty cache")

	c.Put("k1", out)
	got, ok := c.Get("k1")
	assert.True(t, ok)
	assert.Equal(t, out, got)
}

func TestMemoryVerdictCache_TTLExpiry(t *testing.T) {
	c := NewMemoryVerdictCache(10 * time.Millisecond)
	c.Put("k1", &ValidateCompatibilityOutput{})
	time.Sleep(25 * time.Millisecond)
	_, ok := c.Get("k1")
	assert.False(t, ok, "entry should have expired")
}

func TestMemoryVerdictCache_InvalidatePrefix(t *testing.T) {
	c := NewMemoryVerdictCache(time.Minute)
	c.Put("stack:1:hashA", &ValidateCompatibilityOutput{Message: "A"})
	c.Put("stack:2:hashB", &ValidateCompatibilityOutput{Message: "B"})

	c.Invalidate("stack:1")
	_, ok1 := c.Get("stack:1:hashA")
	_, ok2 := c.Get("stack:2:hashB")
	assert.False(t, ok1, "stack:1 entries purged")
	assert.True(t, ok2, "unrelated entries kept")
}

func TestVerdictCacheKey_DeterministicAcrossOrderings(t *testing.T) {
	// Map iteration order doesn't leak into the key.
	a := VerdictCacheKey(ValidateCompatibilityInput{
		Tools: map[string]string{"ci_platform": "GitLab CI", "source_repository": "GitLab CE"},
	})
	b := VerdictCacheKey(ValidateCompatibilityInput{
		Tools: map[string]string{"source_repository": "GitLab CE", "ci_platform": "GitLab CI"},
	})
	assert.Equal(t, a, b)
}

func TestVerdictCacheKey_DiffersByCluster(t *testing.T) {
	a := VerdictCacheKey(ValidateCompatibilityInput{ClusterID: "c1"})
	b := VerdictCacheKey(ValidateCompatibilityInput{ClusterID: "c2"})
	assert.NotEqual(t, a, b)
}

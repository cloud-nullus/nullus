package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceDefault_CreationWithAllFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	resource := ResourceDefault{
		ToolKey:          "argocd",
		DisplayName:      "Argo CD",
		CPURequest:       0.5,
		CPULimit:         1.0,
		MemoryRequestGi:  1.0,
		MemoryLimitGi:    2.0,
		StorageRequestGi: 5.0,
		StorageLimitGi:   10.0,
		IsDefault:        true,
		UpdatedAt:        now,
	}

	assert.Equal(t, "argocd", resource.ToolKey)
	assert.Equal(t, "Argo CD", resource.DisplayName)
	assert.Equal(t, 0.5, resource.CPURequest)
	assert.Equal(t, 1.0, resource.CPULimit)
	assert.Equal(t, 1.0, resource.MemoryRequestGi)
	assert.Equal(t, 2.0, resource.MemoryLimitGi)
	assert.Equal(t, 5.0, resource.StorageRequestGi)
	assert.Equal(t, 10.0, resource.StorageLimitGi)
	assert.True(t, resource.IsDefault)
	assert.True(t, resource.UpdatedAt.Equal(now))
}

func TestResourceDefault_ZeroValue(t *testing.T) {
	var resource ResourceDefault

	assert.Equal(t, "", resource.ToolKey)
	assert.Equal(t, "", resource.DisplayName)
	assert.Equal(t, 0.0, resource.CPURequest)
	assert.Equal(t, 0.0, resource.CPULimit)
	assert.Equal(t, 0.0, resource.MemoryRequestGi)
	assert.Equal(t, 0.0, resource.MemoryLimitGi)
	assert.Equal(t, 0.0, resource.StorageRequestGi)
	assert.Equal(t, 0.0, resource.StorageLimitGi)
	assert.False(t, resource.IsDefault)
	assert.True(t, resource.UpdatedAt.IsZero())
}

func TestResourceDefault_JSONTagsMatchExpectedFieldNames(t *testing.T) {
	resource := ResourceDefault{
		ToolKey:          "prometheus",
		DisplayName:      "Prometheus",
		CPURequest:       0.25,
		CPULimit:         0.5,
		MemoryRequestGi:  0.5,
		MemoryLimitGi:    1.0,
		StorageRequestGi: 2.0,
		StorageLimitGi:   4.0,
		IsDefault:        false,
		UpdatedAt:        time.Unix(1710000000, 0).UTC(),
	}

	bytes, err := json.Marshal(resource)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(bytes, &payload)
	require.NoError(t, err)

	assert.Contains(t, payload, "tool_key")
	assert.Contains(t, payload, "display_name")
	assert.Contains(t, payload, "cpu_request")
	assert.Contains(t, payload, "cpu_limit")
	assert.Contains(t, payload, "memory_request_gi")
	assert.Contains(t, payload, "memory_limit_gi")
	assert.Contains(t, payload, "storage_request_gi")
	assert.Contains(t, payload, "storage_limit_gi")
	assert.Contains(t, payload, "is_default")
	assert.Contains(t, payload, "updated_at")
}

func TestResourceDefault_IsDefaultFlagWorks(t *testing.T) {
	defaultResource := ResourceDefault{ToolKey: "grafana", IsDefault: true}
	customResource := ResourceDefault{ToolKey: "grafana", IsDefault: false}

	assert.Equal(t, "grafana", defaultResource.ToolKey)
	assert.Equal(t, "grafana", customResource.ToolKey)
	assert.True(t, defaultResource.IsDefault)
	assert.False(t, customResource.IsDefault)
}

func TestResourceDefault_LimitsGreaterThanOrEqualToRequests(t *testing.T) {
	resource := ResourceDefault{
		CPURequest:       0.5,
		CPULimit:         1.0,
		MemoryRequestGi:  1.0,
		MemoryLimitGi:    2.0,
		StorageRequestGi: 10.0,
		StorageLimitGi:   20.0,
	}

	assert.GreaterOrEqual(t, resource.CPULimit, resource.CPURequest)
	assert.GreaterOrEqual(t, resource.MemoryLimitGi, resource.MemoryRequestGi)
	assert.GreaterOrEqual(t, resource.StorageLimitGi, resource.StorageRequestGi)
}

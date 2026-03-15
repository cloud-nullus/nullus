package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultValues_CertManager(t *testing.T) {
	values := DefaultValues("installing_cert_manager")
	require.NotNil(t, values)
	assert.Equal(t, true, values["installCRDs"])
}

func TestDefaultValues_GitLab(t *testing.T) {
	values := DefaultValues("installing_gitlab")
	require.NotNil(t, values)

	global, ok := values["global"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ce", global["edition"])
}

func TestDefaultValues_UnknownStepReturnsEmptyMap(t *testing.T) {
	values := DefaultValues("unknown_step")
	require.NotNil(t, values)
	assert.Empty(t, values)
}

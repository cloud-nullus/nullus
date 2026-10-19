package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBundleFixesFound(t *testing.T) {
	bundle := &UpgradeBundle{FixesCVEs: []string{"CVE-2", "CVE-1", "CVE-9"}}

	assert.Equal(t, []string{"CVE-1", "CVE-2"}, BundleFixesFound(bundle, map[string]bool{"CVE-1": true, "CVE-2": true, "CVE-3": true}))
	assert.Empty(t, BundleFixesFound(bundle, nil))
	assert.Empty(t, BundleFixesFound(nil, map[string]bool{"CVE-1": true}))
}

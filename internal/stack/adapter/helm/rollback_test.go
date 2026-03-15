package helm

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollbackManager_RollbackAll_ReverseOrder(t *testing.T) {
	installer := &mockInstaller{}
	rm := &RollbackManager{}
	rm.Push("cert-manager")
	rm.Push("minio")
	rm.Push("gitlab")

	err := rm.RollbackAll(context.Background(), installer, "nullus")
	require.NoError(t, err)
	assert.Equal(t, []string{"gitlab", "minio", "cert-manager"}, installer.uninstalled)
}

func TestRollbackManager_RollbackAll_ContinuesOnError(t *testing.T) {
	installer := &mockInstaller{
		failDelete: map[string]error{
			"minio": fmt.Errorf("cannot uninstall minio"),
		},
	}
	rm := &RollbackManager{}
	rm.Push("cert-manager")
	rm.Push("minio")
	rm.Push("gitlab")

	err := rm.RollbackAll(context.Background(), installer, "nullus")
	require.Error(t, err)
	assert.Equal(t, []string{"gitlab", "minio", "cert-manager"}, installer.uninstalled)
	assert.Contains(t, err.Error(), "minio")
}

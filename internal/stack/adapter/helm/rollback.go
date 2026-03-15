package helm

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

type RollbackManager struct {
	mu       sync.Mutex
	releases []string
}

func (rm *RollbackManager) Push(releaseName string) {
	if releaseName == "" {
		return
	}
	rm.mu.Lock()
	rm.releases = append(rm.releases, releaseName)
	rm.mu.Unlock()
}

func (rm *RollbackManager) RollbackAll(ctx context.Context, installer port.HelmInstaller, namespace string) error {
	rm.mu.Lock()
	releases := append([]string(nil), rm.releases...)
	rm.releases = nil
	rm.mu.Unlock()

	var errs []error
	for i := len(releases) - 1; i >= 0; i-- {
		releaseName := releases[i]
		if err := installer.Uninstall(ctx, releaseName, namespace); err != nil {
			errs = append(errs, fmt.Errorf("uninstall %s: %w", releaseName, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

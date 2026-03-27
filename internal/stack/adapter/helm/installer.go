package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type HelmInstaller struct {
	newActionConfig func(kubeconfig []byte, namespace string) (*action.Configuration, error)
}

const helmOperationTimeout = 30 * time.Minute

func NewHelmInstaller(kubeconfig []byte) *HelmInstaller {
	return &HelmInstaller{
		newActionConfig: func(_ []byte, namespace string) (*action.Configuration, error) {
			return newActionConfig(kubeconfig, namespace)
		},
	}
}

func (h *HelmInstaller) Install(ctx context.Context, req port.HelmInstallRequest) (*port.HelmInstallResult, error) {
	cfg, err := h.newActionConfig(nil, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("init action config: %w", err)
	}

	client := action.NewInstall(cfg)
	client.ReleaseName = req.ReleaseName
	client.Namespace = req.Namespace
	client.Timeout = helmOperationTimeout
	client.CreateNamespace = true
	client.Wait = resolveWait(req.Wait)
	client.Version = req.Version
	client.ChartPathOptions.RepoURL = req.RepoURL
	client.ChartPathOptions.Version = req.Version

	settings := cli.New()
	chartPath, err := client.ChartPathOptions.LocateChart(req.ChartName, settings)
	if err != nil {
		return nil, fmt.Errorf("locate chart %s: %w", req.ChartName, err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", chartPath, err)
	}

	values := req.Values
	if values == nil {
		values = map[string]any{}
	}

	release, runErr := client.RunWithContext(ctx, chart, values)
	if runErr != nil {
		if shouldUpgradeOnInstallError(runErr) {
			existingStatus, statusErr := h.releaseStatus(ctx, cfg, req.ReleaseName)
			if statusErr == nil && shouldReinstallOnExistingStatus(existingStatus) {
				return h.reinstallRelease(ctx, cfg, req, values)
			}

			upgraded, upgradeErr := h.upgradeExistingRelease(ctx, cfg, req, values)
			if upgradeErr == nil {
				return upgraded, nil
			}
			if shouldReinstallOnUpgradeError(upgradeErr) {
				return h.reinstallRelease(ctx, cfg, req, values)
			}
			return upgraded, upgradeErr
		}
		if release != nil {
			return &port.HelmInstallResult{
				ReleaseName: release.Name,
				Namespace:   release.Namespace,
				Status:      release.Info.Status.String(),
				Revision:    release.Version,
			}, fmt.Errorf("install release %s: %w", req.ReleaseName, runErr)
		}
		return nil, fmt.Errorf("install release %s: %w", req.ReleaseName, runErr)
	}

	return &port.HelmInstallResult{
		ReleaseName: release.Name,
		Namespace:   release.Namespace,
		Status:      release.Info.Status.String(),
		Revision:    release.Version,
	}, nil
}

func (h *HelmInstaller) releaseStatus(ctx context.Context, cfg *action.Configuration, releaseName string) (string, error) {
	statusClient := action.NewStatus(cfg)
	release, err := statusClient.Run(releaseName)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if release == nil || release.Info == nil {
		return "", nil
	}
	return release.Info.Status.String(), nil
}

func shouldUpgradeOnInstallError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cannot re-use a name that is still in use")
}

func resolveWait(wait *bool) bool {
	if wait == nil {
		return true
	}
	return *wait
}

func shouldReinstallOnExistingStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "pending-") || s == "failed"
}

func shouldReinstallOnUpgradeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "another operation")
}

func shouldIgnoreUninstallError(err error) bool {
	if err == nil {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "release: not found") || strings.Contains(msg, "not found")
}

func shouldRetryReinstallError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found")
}

func (h *HelmInstaller) reinstallRelease(
	ctx context.Context,
	cfg *action.Configuration,
	req port.HelmInstallRequest,
	values map[string]any,
) (*port.HelmInstallResult, error) {
	uninstall := action.NewUninstall(cfg)
	_, uninstallErr := uninstall.Run(req.ReleaseName)
	if uninstallErr != nil && !shouldIgnoreUninstallError(uninstallErr) {
		return nil, fmt.Errorf("uninstall existing release %s before reinstall: %w", req.ReleaseName, uninstallErr)
	}

	client := action.NewInstall(cfg)
	client.ReleaseName = req.ReleaseName
	client.Namespace = req.Namespace
	client.Timeout = helmOperationTimeout
	client.CreateNamespace = true
	client.Wait = resolveWait(req.Wait)
	client.Version = req.Version
	client.ChartPathOptions.RepoURL = req.RepoURL
	client.ChartPathOptions.Version = req.Version

	settings := cli.New()
	chartPath, err := client.ChartPathOptions.LocateChart(req.ChartName, settings)
	if err != nil {
		return nil, fmt.Errorf("locate chart %s: %w", req.ChartName, err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", chartPath, err)
	}

	release, err := client.RunWithContext(ctx, chart, values)
	if err != nil && shouldRetryReinstallError(err) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
		release, err = client.RunWithContext(ctx, chart, values)
	}
	if err != nil {
		if release != nil {
			return &port.HelmInstallResult{
				ReleaseName: release.Name,
				Namespace:   release.Namespace,
				Status:      release.Info.Status.String(),
				Revision:    release.Version,
			}, fmt.Errorf("reinstall release %s: %w", req.ReleaseName, err)
		}
		return nil, fmt.Errorf("reinstall release %s: %w", req.ReleaseName, err)
	}

	return &port.HelmInstallResult{
		ReleaseName: release.Name,
		Namespace:   release.Namespace,
		Status:      release.Info.Status.String(),
		Revision:    release.Version,
	}, nil
}

func (h *HelmInstaller) upgradeExistingRelease(
	ctx context.Context,
	cfg *action.Configuration,
	req port.HelmInstallRequest,
	values map[string]any,
) (*port.HelmInstallResult, error) {
	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = req.Namespace
	upgrade.Timeout = helmOperationTimeout
	upgrade.Wait = resolveWait(req.Wait)
	upgrade.Install = true
	upgrade.Version = req.Version
	upgrade.ChartPathOptions.RepoURL = req.RepoURL
	upgrade.ChartPathOptions.Version = req.Version

	settings := cli.New()
	chartPath, err := upgrade.ChartPathOptions.LocateChart(req.ChartName, settings)
	if err != nil {
		return nil, fmt.Errorf("locate chart %s: %w", req.ChartName, err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", chartPath, err)
	}

	release, err := upgrade.RunWithContext(ctx, req.ReleaseName, chart, values)
	if err != nil {
		if release != nil {
			return &port.HelmInstallResult{
				ReleaseName: release.Name,
				Namespace:   release.Namespace,
				Status:      release.Info.Status.String(),
				Revision:    release.Version,
			}, fmt.Errorf("upgrade release %s: %w", req.ReleaseName, err)
		}
		return nil, fmt.Errorf("upgrade release %s: %w", req.ReleaseName, err)
	}

	return &port.HelmInstallResult{
		ReleaseName: release.Name,
		Namespace:   release.Namespace,
		Status:      release.Info.Status.String(),
		Revision:    release.Version,
	}, nil
}

func (h *HelmInstaller) Uninstall(ctx context.Context, releaseName, namespace string) error {
	cfg, err := h.newActionConfig(nil, namespace)
	if err != nil {
		return fmt.Errorf("init action config: %w", err)
	}

	client := action.NewUninstall(cfg)
	_, err = client.Run(releaseName)
	if err != nil {
		return fmt.Errorf("uninstall release %s: %w", releaseName, err)
	}
	return nil
}

func (h *HelmInstaller) Status(ctx context.Context, releaseName, namespace string) (*port.HelmInstallResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg, err := h.newActionConfig(nil, namespace)
	if err != nil {
		return nil, fmt.Errorf("init action config: %w", err)
	}

	client := action.NewStatus(cfg)
	release, err := client.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("get status for release %s: %w", releaseName, err)
	}

	return &port.HelmInstallResult{
		ReleaseName: release.Name,
		Namespace:   release.Namespace,
		Status:      release.Info.Status.String(),
		Revision:    release.Version,
	}, nil
}

func newActionConfig(kubeconfig []byte, namespace string) (*action.Configuration, error) {
	getter, err := newKubeRESTClientGetter(kubeconfig, namespace)
	if err != nil {
		return nil, err
	}

	cfg := new(action.Configuration)
	if err := cfg.Init(getter, namespace, "secret", noopHelmDebug); err != nil {
		return nil, fmt.Errorf("initialize helm action config: %w", err)
	}
	return cfg, nil
}

func noopHelmDebug(_ string, _ ...interface{}) {}

type kubeRESTClientGetter struct {
	restConfig *rest.Config
	rawConfig  clientcmd.ClientConfig
}

func newKubeRESTClientGetter(kubeconfig []byte, namespace string) (*kubeRESTClientGetter, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	overrides := &clientcmd.ConfigOverrides{}
	if strings.TrimSpace(namespace) != "" {
		overrides.Context.Namespace = namespace
	}

	raw := clientcmd.NewDefaultClientConfig(*clientcmdapi.NewConfig(), overrides)
	if config, err := clientcmd.Load(kubeconfig); err == nil {
		raw = clientcmd.NewDefaultClientConfig(*config, overrides)
	}

	return &kubeRESTClientGetter{restConfig: restConfig, rawConfig: raw}, nil
}

func (k *kubeRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return rest.CopyConfig(k.restConfig), nil
}

func (k *kubeRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	cfg, err := k.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	client, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(client), nil
}

func (k *kubeRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := k.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	return mapper, nil
}

func (k *kubeRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return k.rawConfig
}

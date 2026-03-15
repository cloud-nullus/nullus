package helm

import (
	"context"
	"fmt"
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
	client.Timeout = 10 * time.Minute
	client.CreateNamespace = true
	client.Wait = true
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
	getter, err := newKubeRESTClientGetter(kubeconfig)
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

func newKubeRESTClientGetter(kubeconfig []byte) (*kubeRESTClientGetter, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	raw := clientcmd.NewDefaultClientConfig(*clientcmdapi.NewConfig(), &clientcmd.ConfigOverrides{})
	if config, err := clientcmd.Load(kubeconfig); err == nil {
		raw = clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{})
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

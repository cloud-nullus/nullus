package kube

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// ManifestApplier applies K8s manifests using the dynamic client.
type ManifestApplier struct {
	Tracker *StepTracker
}

func NewManifestApplier() *ManifestApplier {
	return &ManifestApplier{Tracker: NewStepTracker()}
}

func (a *ManifestApplier) Apply(ctx context.Context, kubeconfig []byte, manifests []string) error {
	return a.ApplyWithTracking(ctx, kubeconfig, manifests, "")
}

// ApplyWithTracking applies manifests and reports step progress to the tracker.
func (a *ManifestApplier) ApplyWithTracking(ctx context.Context, kubeconfig []byte, manifests []string, deploymentID string) error {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}
	config.Timeout = 10 * time.Second

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	for i, manifest := range manifests {
		tracking := deploymentID != "" && a.Tracker != nil
		if tracking {
			a.Tracker.MarkRunning(deploymentID, i, "")
			a.Tracker.AppendLog(deploymentID, i, "$ kubectl apply -f -")
		}
		results, err := applyManifestDocuments(ctx, dynClient, mapper, decoder, manifest)

		for _, r := range results {
			if tracking {
				a.Tracker.AppendLog(deploymentID, i, fmt.Sprintf("%s/%s %s", strings.ToLower(r.Kind), r.Name, r.Action))
			}
		}

		if err != nil {
			msg := err.Error()
			if len(results) > 0 {
				last := results[len(results)-1]
				msg = fmt.Sprintf("%s/%s %s", strings.ToLower(last.Kind), last.Name, last.Action)
			}
			if tracking {
				a.Tracker.AppendLog(deploymentID, i, fmt.Sprintf("error: %s", err.Error()))
				a.Tracker.MarkFailed(deploymentID, i, msg)
			}
			return err
		}
		var msg string
		for _, r := range results {
			msg = fmt.Sprintf("%s/%s %s", strings.ToLower(r.Kind), r.Name, r.Action)
		}
		if tracking {
			a.Tracker.MarkSuccess(deploymentID, i, msg)
		}
	}

	return nil
}

type applyResult struct {
	Kind   string
	Name   string
	Action string
}

func applyManifestDocuments(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	decoder runtime.Decoder,
	manifest string,
) ([]applyResult, error) {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewBufferString(manifest)))
	var results []applyResult

	for {
		doc, err := reader.Read()
		if err == io.EOF {
			return results, nil
		}
		if err != nil {
			return results, fmt.Errorf("read yaml document: %w", err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		r, err := applyManifest(ctx, dynClient, mapper, decoder, doc)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
}

func applyManifest(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	decoder runtime.Decoder,
	doc []byte,
) (applyResult, error) {
	obj := &unstructured.Unstructured{}

	_, gvk, err := decoder.Decode(doc, nil, obj)
	if err != nil {
		return applyResult{}, fmt.Errorf("decode manifest: %w", err)
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return applyResult{}, fmt.Errorf("resolve rest mapping for %s: %w", gvk.String(), err)
	}

	var resource dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}
		resource = dynClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		resource = dynClient.Resource(mapping.Resource)
	}

	name := obj.GetName()
	kind := gvk.Kind
	if name == "" {
		return applyResult{}, fmt.Errorf("manifest missing metadata.name for %s", gvk.String())
	}

	_, err = resource.Create(ctx, obj, metav1.CreateOptions{})
	if err == nil {
		return applyResult{Kind: kind, Name: name, Action: "created"}, nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return applyResult{Kind: kind, Name: name, Action: "failed"}, fmt.Errorf("create %s/%s: %w", kind, name, err)
	}

	existing, getErr := resource.Get(ctx, name, metav1.GetOptions{})
	if getErr != nil {
		return applyResult{Kind: kind, Name: name, Action: "failed"}, fmt.Errorf("get existing %s/%s: %w", kind, name, getErr)
	}

	obj.SetResourceVersion(existing.GetResourceVersion())
	if _, updateErr := resource.Update(ctx, obj, metav1.UpdateOptions{}); updateErr != nil {
		return applyResult{Kind: kind, Name: name, Action: "failed"}, fmt.Errorf("update %s/%s: %w", kind, name, updateErr)
	}

	return applyResult{Kind: kind, Name: name, Action: "configured"}, nil
}

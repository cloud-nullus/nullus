package kube

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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

type ManifestApplier struct{}

func NewManifestApplier() *ManifestApplier {
	return &ManifestApplier{}
}

func (a *ManifestApplier) Apply(ctx context.Context, kubeconfig []byte, manifests []string) error {
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

	for _, manifest := range manifests {
		if err := applyManifestDocuments(ctx, dynClient, mapper, decoder, manifest); err != nil {
			return err
		}
	}

	return nil
}

func applyManifestDocuments(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	decoder runtime.Decoder,
	manifest string,
) error {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewBufferString(manifest)))

	for {
		doc, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read yaml document: %w", err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		if err := applyManifest(ctx, dynClient, mapper, decoder, doc); err != nil {
			return err
		}
	}
}

func applyManifest(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	decoder runtime.Decoder,
	doc []byte,
) error {
	obj := &unstructured.Unstructured{}

	_, gvk, err := decoder.Decode(doc, nil, obj)
	if err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("resolve rest mapping for %s: %w", gvk.String(), err)
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
	if name == "" {
		return fmt.Errorf("manifest missing metadata.name for %s", gvk.String())
	}

	_, err = resource.Create(ctx, obj, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create %s/%s: %w", gvk.Kind, name, err)
	}

	existing, getErr := resource.Get(ctx, name, metav1.GetOptions{})
	if getErr != nil {
		return fmt.Errorf("get existing %s/%s: %w", gvk.Kind, name, getErr)
	}

	obj.SetResourceVersion(existing.GetResourceVersion())
	if _, updateErr := resource.Update(ctx, obj, metav1.UpdateOptions{}); updateErr != nil {
		return fmt.Errorf("update %s/%s: %w", gvk.Kind, name, updateErr)
	}

	return nil
}

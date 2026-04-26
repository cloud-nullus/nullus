package manifests

import (
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

type DeployAppRequest struct {
	AppName   string
	GitURL    string
	Namespace string
	Template  string
	ImageRef  string
	Replicas  int32
	Port      int32
	Resources ResourceSpec
	EnvVars   map[string]string
	Labels    map[string]string
}

type ResourceSpec struct {
	CPULimit   string
	MemLimit   string
	CPURequest string
	MemRequest string
}

type GeneratedManifests struct {
	Namespace  string
	Deployment string
	Service    string
	Ingress    string
}

type templateConfig struct {
	image string
	port  int32
}

var templateConfigs = map[string]templateConfig{
	"react-spa":      {image: "nginx:alpine", port: 80},
	"next-app":       {image: "node:20-alpine", port: 3000},
	"express-api":    {image: "node:20-alpine", port: 3000},
	"spring-boot":    {image: "eclipse-temurin:21-jre", port: 8080},
	"python-fastapi": {image: "python:3.12-slim", port: 8000},
	"go-web-api":     {image: "nginx:alpine", port: 80},
}

func Generate(req DeployAppRequest) (*GeneratedManifests, error) {
	if req.AppName == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	tpl, ok := templateConfigs[req.Template]
	if !ok {
		return nil, fmt.Errorf("unsupported template: %s", req.Template)
	}

	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	port := req.Port
	if port <= 0 {
		port = tpl.port
	}

	resources, err := buildResources(req.Resources)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"app": req.AppName,
	}
	for k, v := range req.Labels {
		if k == "" || v == "" {
			continue
		}
		labels[k] = v
	}

	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Namespace,
		},
	}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.AppName,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            req.AppName,
							Image:           imageForContainer(req.ImageRef, tpl.image),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{ContainerPort: port, Name: "http"},
							},
							Resources: resources,
							Env:       buildEnvVars(req.EnvVars, req.GitURL),
						},
					},
				},
			},
		},
	}

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.AppName,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
				},
			},
		},
	}

	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.AppName,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: fmt.Sprintf("%s.%s.nullus.local", req.AppName, req.Namespace),
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: req.AppName,
											Port: networkingv1.ServiceBackendPort{Number: port},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	namespaceYAML, err := marshalYAML(ns)
	if err != nil {
		return nil, err
	}
	deploymentYAML, err := marshalYAML(dep)
	if err != nil {
		return nil, err
	}
	serviceYAML, err := marshalYAML(svc)
	if err != nil {
		return nil, err
	}
	ingressYAML, err := marshalYAML(ing)
	if err != nil {
		return nil, err
	}

	return &GeneratedManifests{
		Namespace:  namespaceYAML,
		Deployment: deploymentYAML,
		Service:    serviceYAML,
		Ingress:    ingressYAML,
	}, nil
}

func buildResources(spec ResourceSpec) (corev1.ResourceRequirements, error) {
	requestsCPU := firstNonEmpty(spec.CPURequest, "100m")
	requestsMem := firstNonEmpty(spec.MemRequest, "128Mi")
	limitsCPU := firstNonEmpty(spec.CPULimit, "500m")
	limitsMem := firstNonEmpty(spec.MemLimit, "512Mi")

	reqCPUQty, err := resource.ParseQuantity(requestsCPU)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid cpu request: %w", err)
	}
	reqMemQty, err := resource.ParseQuantity(requestsMem)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid memory request: %w", err)
	}
	limitCPUQty, err := resource.ParseQuantity(limitsCPU)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid cpu limit: %w", err)
	}
	limitMemQty, err := resource.ParseQuantity(limitsMem)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid memory limit: %w", err)
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    reqCPUQty,
			corev1.ResourceMemory: reqMemQty,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    limitCPUQty,
			corev1.ResourceMemory: limitMemQty,
		},
	}, nil
}

func buildEnvVars(env map[string]string, gitURL string) []corev1.EnvVar {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]corev1.EnvVar, 0, len(keys)+1)
	if gitURL != "" {
		result = append(result, corev1.EnvVar{Name: "GIT_URL", Value: gitURL})
	}
	for _, key := range keys {
		result = append(result, corev1.EnvVar{Name: key, Value: env[key]})
	}
	return result
}

func marshalYAML(obj any) (string, error) {
	raw, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal manifest yaml: %w", err)
	}
	return string(raw), nil
}

func firstNonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func imageForContainer(imageRef, templateImage string) string {
	if imageRef != "" {
		return imageRef
	}
	return templateImage
}

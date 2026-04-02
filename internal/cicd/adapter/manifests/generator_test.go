package manifests

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestGenerate_ReactSPATemplate(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "web-app",
		Namespace: "team-a",
		Template:  "react-spa",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	if dep.Spec.Template.Spec.Containers[0].Image != "nginx:alpine" {
		t.Fatalf("image = %q, want %q", dep.Spec.Template.Spec.Containers[0].Image, "nginx:alpine")
	}
	if dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != 80 {
		t.Fatalf("container port = %d, want 80", dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	}
}

func TestGenerate_SpringBootTemplateUsesTemurinImage(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "orders",
		Namespace: "apps",
		Template:  "spring-boot",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	if dep.Spec.Template.Spec.Containers[0].Image != "eclipse-temurin:21-jre" {
		t.Fatalf("image = %q, want %q", dep.Spec.Template.Spec.Containers[0].Image, "eclipse-temurin:21-jre")
	}
}

func TestGenerate_ImageRefOverridesTemplateImage(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "orders",
		Namespace: "apps",
		Template:  "go-web-api",
		ImageRef:  "orders:abc12345",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	if dep.Spec.Template.Spec.Containers[0].Image != "orders:abc12345" {
		t.Fatalf("image = %q, want %q", dep.Spec.Template.Spec.Containers[0].Image, "orders:abc12345")
	}
}

func TestGenerate_DefaultResourcesApplied(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "api",
		Namespace: "core",
		Template:  "go-web-api",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	res := dep.Spec.Template.Spec.Containers[0].Resources
	if gotReqCPU := quantityString(res.Requests[corev1.ResourceCPU]); gotReqCPU != "100m" {
		t.Fatalf("cpu request = %q, want %q", gotReqCPU, "100m")
	}
	if gotReqMem := quantityString(res.Requests[corev1.ResourceMemory]); gotReqMem != "128Mi" {
		t.Fatalf("memory request = %q, want %q", gotReqMem, "128Mi")
	}
	if gotLimitCPU := quantityString(res.Limits[corev1.ResourceCPU]); gotLimitCPU != "500m" {
		t.Fatalf("cpu limit = %q, want %q", gotLimitCPU, "500m")
	}
	if gotLimitMem := quantityString(res.Limits[corev1.ResourceMemory]); gotLimitMem != "512Mi" {
		t.Fatalf("memory limit = %q, want %q", gotLimitMem, "512Mi")
	}
}

func TestGenerate_CustomResourcesOverrideDefaults(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "api",
		Namespace: "core",
		Template:  "go-web-api",
		Resources: ResourceSpec{
			CPURequest: "250m",
			MemRequest: "256Mi",
			CPULimit:   "750m",
			MemLimit:   "1Gi",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	res := dep.Spec.Template.Spec.Containers[0].Resources
	if gotReqCPU := quantityString(res.Requests[corev1.ResourceCPU]); gotReqCPU != "250m" {
		t.Fatalf("cpu request = %q, want %q", gotReqCPU, "250m")
	}
	if gotReqMem := quantityString(res.Requests[corev1.ResourceMemory]); gotReqMem != "256Mi" {
		t.Fatalf("memory request = %q, want %q", gotReqMem, "256Mi")
	}
	if gotLimitCPU := quantityString(res.Limits[corev1.ResourceCPU]); gotLimitCPU != "750m" {
		t.Fatalf("cpu limit = %q, want %q", gotLimitCPU, "750m")
	}
	if gotLimitMem := quantityString(res.Limits[corev1.ResourceMemory]); gotLimitMem != "1Gi" {
		t.Fatalf("memory limit = %q, want %q", gotLimitMem, "1Gi")
	}
}

func TestGenerate_EnvironmentVariablesInjected(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "api",
		Namespace: "core",
		Template:  "express-api",
		EnvVars: map[string]string{
			"LOG_LEVEL": "debug",
			"TZ":        "UTC",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	env := dep.Spec.Template.Spec.Containers[0].Env
	seen := map[string]string{}
	for _, item := range env {
		seen[item.Name] = item.Value
	}

	if seen["LOG_LEVEL"] != "debug" {
		t.Fatalf("LOG_LEVEL = %q, want %q", seen["LOG_LEVEL"], "debug")
	}
	if seen["TZ"] != "UTC" {
		t.Fatalf("TZ = %q, want %q", seen["TZ"], "UTC")
	}
}

func TestGenerate_UnknownTemplateReturnsError(t *testing.T) {
	_, err := Generate(DeployAppRequest{
		AppName:   "api",
		Namespace: "core",
		Template:  "unknown-template",
	})
	if err == nil {
		t.Fatal("Generate() error = nil, want error")
	}
}

func TestGenerate_ManifestsYAMLCanBeUnmarshaled(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "billing",
		Namespace: "apps",
		Template:  "next-app",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var ns corev1.Namespace
	if err := yaml.Unmarshal([]byte(got.Namespace), &ns); err != nil {
		t.Fatalf("unmarshal namespace: %v", err)
	}
	if ns.TypeMeta != (metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"}) {
		t.Fatalf("namespace typemeta = %#v", ns.TypeMeta)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	var svc corev1.Service
	if err := yaml.Unmarshal([]byte(got.Service), &svc); err != nil {
		t.Fatalf("unmarshal service: %v", err)
	}

	var ing networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(got.Ingress), &ing); err != nil {
		t.Fatalf("unmarshal ingress: %v", err)
	}
}

func quantityString(q resource.Quantity) string {
	return q.String()
}

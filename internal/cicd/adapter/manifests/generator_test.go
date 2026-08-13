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

// 배포된 앱이 추적을 보내려면 "어디로 보낼지"를 알아야 한다. 수집기가 떠 있어도
// 이 주소가 없으면 앱은 아무것도 내보내지 않는다.
func TestGenerate_InjectsOTelEndpointWhenCollectorPresent(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:      "api",
		Namespace:    "core",
		Template:     "express-api",
		OTLPEndpoint: "otel-collector-opentelemetry-collector.core.svc.cluster.local:4317",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	seen := map[string]string{}
	for _, item := range dep.Spec.Template.Spec.Containers[0].Env {
		seen[item.Name] = item.Value
	}

	// SDK 규약상 gRPC 엔드포인트는 스킴이 있어야 한다.
	if want := "http://otel-collector-opentelemetry-collector.core.svc.cluster.local:4317"; seen["OTEL_EXPORTER_OTLP_ENDPOINT"] != want {
		t.Fatalf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want %q", seen["OTEL_EXPORTER_OTLP_ENDPOINT"], want)
	}
	// 서비스 이름이 없으면 모든 앱이 unknown_service 로 뭉쳐 구분되지 않는다.
	if seen["OTEL_SERVICE_NAME"] != "api" {
		t.Fatalf("OTEL_SERVICE_NAME = %q, want %q", seen["OTEL_SERVICE_NAME"], "api")
	}
	if seen["OTEL_EXPORTER_OTLP_PROTOCOL"] != "grpc" {
		t.Fatalf("OTEL_EXPORTER_OTLP_PROTOCOL = %q, want grpc", seen["OTEL_EXPORTER_OTLP_PROTOCOL"])
	}
}

// 수집기가 없는 스택에 주소를 박으면 앱이 닿지 않는 곳으로 계속 재시도하며
// 오류 로그만 쌓는다. 없으면 아예 넣지 않는다.
func TestGenerate_OmitsOTelEnvWithoutCollector(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:   "api",
		Namespace: "core",
		Template:  "express-api",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	for _, item := range dep.Spec.Template.Spec.Containers[0].Env {
		if item.Name == "OTEL_EXPORTER_OTLP_ENDPOINT" {
			t.Fatalf("수집기가 없는데 OTLP 주소가 주입되었다: %q", item.Value)
		}
	}
}

// 사용자가 직접 지정한 값이 우선이다 — 외부 수집기로 보내려는 선택을
// 플랫폼이 덮어쓰면 안 된다.
func TestGenerate_UserEnvOverridesInjectedOTelEndpoint(t *testing.T) {
	got, err := Generate(DeployAppRequest{
		AppName:      "api",
		Namespace:    "core",
		Template:     "express-api",
		OTLPEndpoint: "otel-collector-opentelemetry-collector.core.svc.cluster.local:4317",
		EnvVars:      map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://vendor.example.com:4317"},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(got.Deployment), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}

	count := 0
	value := ""
	for _, item := range dep.Spec.Template.Spec.Containers[0].Env {
		if item.Name == "OTEL_EXPORTER_OTLP_ENDPOINT" {
			count++
			value = item.Value
		}
	}
	if count != 1 {
		t.Fatalf("OTEL_EXPORTER_OTLP_ENDPOINT 가 %d 번 선언되었다 — 중복이면 어느 값이 적용될지 알 수 없다", count)
	}
	if value != "http://vendor.example.com:4317" {
		t.Fatalf("사용자 값이 덮어써졌다: %q", value)
	}
}

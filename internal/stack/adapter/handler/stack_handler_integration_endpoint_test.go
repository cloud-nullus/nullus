package handler

import (
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestIntegrationEndpoint(t *testing.T) {
	const domain_ = "example.com"
	stack := &domain.Stack{Namespace: "test-ns"}
	cfg := domain.StackConfig{AccessDomain: domain_}
	cfgEmpty := domain.StackConfig{}

	tests := []struct {
		name          string
		stack         *domain.Stack
		cfg           domain.StackConfig
		componentType string
		provider      string
		want          string
	}{
		{
			name:          "cd_tool/argocd returns argocd subdomain",
			stack:         stack,
			cfg:           cfg,
			componentType: "cd_tool",
			provider:      "argocd",
			want:          "https://argocd.example.com",
		},
		{
			name:          "ci_platform/argocd returns argocd subdomain",
			stack:         stack,
			cfg:           cfg,
			componentType: "ci_platform",
			provider:      "argocd",
			want:          "https://argocd.example.com",
		},
		{
			name:          "image_registry/harbor returns harbor subdomain",
			stack:         stack,
			cfg:           cfg,
			componentType: "image_registry",
			provider:      "harbor",
			want:          "https://harbor.example.com",
		},
		{
			name:          "grafana falls through to normalizedProvider",
			stack:         stack,
			cfg:           cfg,
			componentType: "monitoring",
			provider:      "grafana",
			want:          "https://grafana.example.com",
		},
		{
			name:          "grafana with observability componentType",
			stack:         stack,
			cfg:           cfg,
			componentType: "observability",
			provider:      "grafana",
			want:          "https://grafana.example.com",
		},
		{
			name:          "empty provider returns empty",
			stack:         stack,
			cfg:           cfg,
			componentType: "cd_tool",
			provider:      "",
			want:          "",
		},
		{
			name:          "empty accessDomain returns empty",
			stack:         &domain.Stack{Namespace: ""},
			cfg:           cfgEmpty,
			componentType: "cd_tool",
			provider:      "argocd",
			want:          "",
		},
		{
			name:          "cd_tool/flux returns flux subdomain",
			stack:         stack,
			cfg:           cfg,
			componentType: "cd_tool",
			provider:      "flux",
			want:          "https://flux.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := integrationEndpoint(tc.stack, tc.cfg, tc.componentType, tc.provider)
			if got != tc.want {
				t.Errorf("integrationEndpoint(%q, %q) = %q; want %q",
					tc.componentType, tc.provider, got, tc.want)
			}
		})
	}
}

func TestIntegrationSubdomain(t *testing.T) {
	tests := []struct {
		componentType string
		provider      string
		want          string
	}{
		{"cd_tool", "argocd", "argocd"},
		{"cd_tool", "argo-cd", "argocd"},
		{"cd_tool", "flux", "flux"},
		{"ci_platform", "argocd", "argocd"},
		{"ci_platform", "gitlab-ci", "gitlab"},
		{"image_registry", "harbor", "harbor"},
		{"image_registry", "gitlab-registry", "registry"},
		{"package_registry", "nexus", "nexus"},
		// grafana: no matching case → falls through to normalizedProvider
		{"monitoring", "grafana", "grafana"},
		{"observability", "grafana", "grafana"},
		// unknown componentType returns normalizedProvider
		{"unknown", "myprovider", "myprovider"},
	}

	for _, tc := range tests {
		t.Run(tc.componentType+"/"+tc.provider, func(t *testing.T) {
			got := integrationSubdomain(tc.componentType, tc.provider)
			if got != tc.want {
				t.Errorf("integrationSubdomain(%q, %q) = %q; want %q",
					tc.componentType, tc.provider, got, tc.want)
			}
		})
	}
}

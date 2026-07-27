package keycloak

import (
	"testing"
)

// TestBuildRedirectURI verifies the pure URI-generation helper.
func TestBuildRedirectURI(t *testing.T) {
	tests := []struct {
		name         string
		subdomain    string
		accessDomain string
		callbackPath string
		want         string
	}{
		{
			name:         "grafana default domain",
			subdomain:    "grafana",
			accessDomain: "nullus.local",
			callbackPath: "/login/generic_oauth",
			want:         "https://grafana.nullus.local/login/generic_oauth",
		},
		{
			name:         "argocd default domain",
			subdomain:    "argocd",
			accessDomain: "nullus.local",
			callbackPath: "/auth/callback",
			want:         "https://argocd.nullus.local/auth/callback",
		},
		{
			name:         "harbor default domain",
			subdomain:    "harbor",
			accessDomain: "nullus.local",
			callbackPath: "/c/oidc/callback",
			want:         "https://harbor.nullus.local/c/oidc/callback",
		},
		{
			name:         "grafana custom domain",
			subdomain:    "grafana",
			accessDomain: "example.com",
			callbackPath: "/login/generic_oauth",
			want:         "https://grafana.example.com/login/generic_oauth",
		},
		{
			name:         "harbor custom domain",
			subdomain:    "harbor",
			accessDomain: "prod.nullus.io",
			callbackPath: "/c/oidc/callback",
			want:         "https://harbor.prod.nullus.io/c/oidc/callback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRedirectURI(tc.subdomain, tc.accessDomain, tc.callbackPath)
			if got != tc.want {
				t.Errorf("buildRedirectURI(%q, %q, %q) = %q; want %q",
					tc.subdomain, tc.accessDomain, tc.callbackPath, got, tc.want)
			}
		})
	}
}

// TestSSOProvisioner_SpecClientIDs verifies each step key maps to the correct ClientID.
func TestSSOProvisioner_SpecClientIDs(t *testing.T) {
	p := NewSSOProvisioner(nil) // kc unused for spec inspection
	tests := []struct {
		stepName string
		wantID   string
	}{
		{"installing_grafana", "grafana"},
		{"installing_argocd", "argocd"},
		{"installing_harbor", "harbor"},
		{"installing_gitlab", "gitlab"},
		{"installing_minio", "minio"},
	}

	for _, tc := range tests {
		t.Run(tc.stepName, func(t *testing.T) {
			spec, ok := p.toolSpecs[tc.stepName]
			if !ok {
				t.Fatalf("spec %q not found in toolSpecs", tc.stepName)
			}
			if spec.ClientID != tc.wantID {
				t.Errorf("spec[%q].ClientID = %q; want %q", tc.stepName, spec.ClientID, tc.wantID)
			}
		})
	}
}

// TestSSOProvisioner_DefaultDomainRedirectURIs verifies redirect URIs built with
// the default "nullus.local" domain (NewSSOProvisioner backward-compat path).
func TestSSOProvisioner_DefaultDomainRedirectURIs(t *testing.T) {
	p := NewSSOProvisioner(nil)

	tests := []struct {
		stepName    string
		wantSubdomain string
		wantCallback  string
	}{
		{"installing_grafana", "grafana", "/login/generic_oauth"},
		{"installing_argocd", "argocd", "/auth/callback"},
		{"installing_harbor", "harbor", "/c/oidc/callback"},
	}

	for _, tc := range tests {
		t.Run(tc.stepName, func(t *testing.T) {
			spec, ok := p.toolSpecs[tc.stepName]
			if !ok {
				t.Fatalf("spec %q not found", tc.stepName)
			}
			wantURI := buildRedirectURI(tc.wantSubdomain, defaultAccessDomain, tc.wantCallback)
			gotURI := buildRedirectURI(spec.Subdomain, p.accessDomain, spec.CallbackPath)
			if gotURI != wantURI {
				t.Errorf("redirect URI for %q = %q; want %q", tc.stepName, gotURI, wantURI)
			}
		})
	}
}

// TestNewSSOProvisionerWithDomain verifies custom accessDomain is reflected in URIs.
func TestNewSSOProvisionerWithDomain(t *testing.T) {
	customDomain := "prod.nullus.io"
	p := NewSSOProvisionerWithDomain(nil, customDomain)

	if p.accessDomain != customDomain {
		t.Errorf("accessDomain = %q; want %q", p.accessDomain, customDomain)
	}

	tests := []struct {
		stepName string
		wantURI  string
	}{
		{"installing_grafana", "https://grafana.prod.nullus.io/login/generic_oauth"},
		{"installing_argocd", "https://argocd.prod.nullus.io/auth/callback"},
		{"installing_harbor", "https://harbor.prod.nullus.io/c/oidc/callback"},
	}

	for _, tc := range tests {
		t.Run(tc.stepName, func(t *testing.T) {
			spec, ok := p.toolSpecs[tc.stepName]
			if !ok {
				t.Fatalf("spec %q not found", tc.stepName)
			}
			gotURI := buildRedirectURI(spec.Subdomain, p.accessDomain, spec.CallbackPath)
			if gotURI != tc.wantURI {
				t.Errorf("URI for %q = %q; want %q", tc.stepName, gotURI, tc.wantURI)
			}
		})
	}
}

// TestProvisionSSO_UnknownStep verifies error on unknown step name (no kc call needed).
func TestProvisionSSO_UnknownStep(t *testing.T) {
	p := NewSSOProvisioner(nil)
	// ProvisionSSO should return error before reaching kc when spec is unknown
	err := p.ProvisionSSO(t.Context(), "installing_unknown_tool")
	if err == nil {
		t.Fatal("expected error for unknown step, got nil")
	}
}

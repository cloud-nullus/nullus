package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// VerdictCacheClearer is the minimal cache invalidator the ManageCompatibility
// use case calls after CRUD mutations. Implemented by *MemoryVerdictCache
// via its Clear() method (added alongside this use case).
type VerdictCacheClearer interface {
	Clear()
}

// ManageCompatibility is the admin-facing CRUD use case for compatibility
// matrices. F8-Phase5 (재개) brings matrix Create/Update/Delete behind a
// use case so validation lives in one place and cache invalidation is
// triggered consistently.
type ManageCompatibility struct {
	repo  port.CompatibilityRepository
	cache VerdictCacheClearer
}

// ManageCompatibilityOption configures optional dependencies.
type ManageCompatibilityOption func(*ManageCompatibility)

// WithVerdictCacheClearer wires the verdict cache so any successful mutation
// invalidates cached verdicts. Nil-safe in NewManageCompatibility.
func WithVerdictCacheClearer(c VerdictCacheClearer) ManageCompatibilityOption {
	return func(u *ManageCompatibility) { u.cache = c }
}

// NewManageCompatibility constructs the use case.
func NewManageCompatibility(repo port.CompatibilityRepository, opts ...ManageCompatibilityOption) *ManageCompatibility {
	u := &ManageCompatibility{repo: repo}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// CompatibilityValidationError is a 400-shaped error the handler layer maps
// to HTTP 400 without string matching.
type CompatibilityValidationError struct {
	Field   string
	Message string
}

func (e *CompatibilityValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// HTTPStatus is the recommended mapping — handlers can type-assert and pull
// the right status code without hard-coding.
func (e *CompatibilityValidationError) HTTPStatus() int { return http.StatusBadRequest }

// Code returns the string error code for JSON payloads.
func (e *CompatibilityValidationError) Code() string { return "COMPATIBILITY_VALIDATION" }

var (
	idRegex       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	semverRegex   = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)
	validStatuses = map[string]struct{}{
		"verified":    {},
		"untested":    {},
		"unsupported": {},
	}
	validTiers = map[string]struct{}{
		"stable":     {},
		"beta":       {},
		"deprecated": {},
	}
	validArches = map[string]struct{}{
		"amd64": {},
		"arm64": {},
	}
)

// validateMatrixPayload normalises the matrix in place (trim, sort archs,
// default tier to stable, etc.) and returns a CompatibilityValidationError
// if any field is out of spec.
func validateMatrixPayload(m *domain.CompatibilityMatrix) error {
	if m == nil {
		return &CompatibilityValidationError{Message: "matrix payload is required"}
	}
	m.ID = strings.TrimSpace(m.ID)
	m.Name = strings.TrimSpace(m.Name)
	m.Status = strings.TrimSpace(m.Status)
	m.Kubernetes.Min = strings.TrimSpace(m.Kubernetes.Min)
	m.Kubernetes.Max = strings.TrimSpace(m.Kubernetes.Max)
	m.Kubernetes.Recommended = strings.TrimSpace(m.Kubernetes.Recommended)

	if !idRegex.MatchString(m.ID) {
		return &CompatibilityValidationError{Field: "id", Message: "must match ^[a-z0-9][a-z0-9-]{0,63}$"}
	}
	if m.Name == "" || len(m.Name) > 120 {
		return &CompatibilityValidationError{Field: "name", Message: "must be 1..120 chars after trim"}
	}
	if _, ok := validStatuses[m.Status]; !ok {
		return &CompatibilityValidationError{Field: "status", Message: "must be one of verified|untested|unsupported"}
	}
	for _, p := range []struct {
		name  string
		value string
	}{
		{"kubernetes.min", m.Kubernetes.Min},
		{"kubernetes.max", m.Kubernetes.Max},
		{"kubernetes.recommended", m.Kubernetes.Recommended},
	} {
		if p.value == "" || !semverRegex.MatchString(p.value) {
			return &CompatibilityValidationError{Field: p.name, Message: "must be a non-empty semver-ish string (v1.27 or 1.27.0)"}
		}
	}
	if len(m.Tools) == 0 {
		return &CompatibilityValidationError{Field: "tools", Message: "must declare at least one tool"}
	}
	if len(m.Tools) > 32 {
		return &CompatibilityValidationError{Field: "tools", Message: "declares more than 32 tools"}
	}
	for category, tool := range m.Tools {
		if strings.TrimSpace(category) == "" {
			return &CompatibilityValidationError{Field: "tools", Message: "category key must be non-empty"}
		}
		tool.Name = strings.TrimSpace(tool.Name)
		if tool.Name == "" || len(tool.Name) > 80 {
			return &CompatibilityValidationError{Field: "tools." + category + ".name", Message: "must be 1..80 chars after trim"}
		}
		tool.HelmVersion = strings.TrimSpace(tool.HelmVersion)
		if tool.HelmVersion == "" || len(tool.HelmVersion) > 60 {
			return &CompatibilityValidationError{Field: "tools." + category + ".helm_version", Message: "must be 1..60 chars"}
		}
		tool.AppVersion = strings.TrimSpace(tool.AppVersion)
		if tool.AppVersion == "" || len(tool.AppVersion) > 60 {
			return &CompatibilityValidationError{Field: "tools." + category + ".app_version", Message: "must be 1..60 chars"}
		}
		tool.Tier = strings.TrimSpace(tool.Tier)
		if tool.Tier == "" {
			tool.Tier = domain.ToolTierStable
		}
		if _, ok := validTiers[tool.Tier]; !ok {
			return &CompatibilityValidationError{Field: "tools." + category + ".tier", Message: "must be stable|beta|deprecated"}
		}
		if len(tool.ArchSupport) == 0 {
			tool.ArchSupport = []string{domain.ArchAMD64}
		}
		// Normalize: lowercase, dedup, restrict to allowed.
		seen := map[string]struct{}{}
		cleaned := make([]string, 0, len(tool.ArchSupport))
		for _, a := range tool.ArchSupport {
			a = strings.ToLower(strings.TrimSpace(a))
			if a == "" {
				continue
			}
			if _, ok := validArches[a]; !ok {
				return &CompatibilityValidationError{Field: "tools." + category + ".arch_support", Message: "only amd64|arm64 allowed"}
			}
			if _, dup := seen[a]; dup {
				continue
			}
			seen[a] = struct{}{}
			cleaned = append(cleaned, a)
		}
		sort.Strings(cleaned)
		tool.ArchSupport = cleaned
		tool.MinK8sVersion = strings.TrimSpace(tool.MinK8sVersion)
		if tool.MinK8sVersion != "" && !semverRegex.MatchString(tool.MinK8sVersion) {
			return &CompatibilityValidationError{Field: "tools." + category + ".min_k8s_version", Message: "must be empty or semver-ish"}
		}
		m.Tools[category] = tool
	}
	return nil
}

// Create validates the payload then persists via the repository. On success
// the verdict cache (if wired) is cleared.
func (u *ManageCompatibility) Create(ctx context.Context, m *domain.CompatibilityMatrix) error {
	if err := validateMatrixPayload(m); err != nil {
		return err
	}
	if err := u.repo.Create(ctx, m); err != nil {
		return err
	}
	u.clearCache()
	return nil
}

// Update validates and persists a full replacement. NotFound from the repo
// surfaces unchanged so handlers can map to 404.
func (u *ManageCompatibility) Update(ctx context.Context, m *domain.CompatibilityMatrix) error {
	if err := validateMatrixPayload(m); err != nil {
		return err
	}
	if err := u.repo.Update(ctx, m); err != nil {
		return err
	}
	u.clearCache()
	return nil
}

// Delete is idempotent at the repo level; the verdict cache is still cleared
// so cached verdicts that referenced the deleted matrix don't linger.
func (u *ManageCompatibility) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return &CompatibilityValidationError{Field: "id", Message: "must be non-empty"}
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}
	u.clearCache()
	return nil
}

func (u *ManageCompatibility) clearCache() {
	if u.cache != nil {
		u.cache.Clear()
	}
}

// IsValidationError reports whether err wraps (or is) a CompatibilityValidationError.
func IsValidationError(err error) bool {
	var target *CompatibilityValidationError
	return errors.As(err, &target)
}

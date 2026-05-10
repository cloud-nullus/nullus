package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// stackToolsToCategoryMap converts a persisted stack's []ToolConfig to the
// {category: tool_name} shape the matrix matcher expects. Entries with
// empty category or tool name are dropped (they can't match any matrix).
func stackToolsToCategoryMap(tools []domain.ToolConfig) map[string]string {
	out := make(map[string]string, len(tools))
	for _, t := range tools {
		name := t.Name
		if name == "" {
			name = t.Tool
		}
		if t.Category == "" || name == "" {
			continue
		}
		out[t.Category] = canonicalToolName(name)
	}
	return out
}

func stackConfigToCategoryMap(cfg domain.StackConfig) map[string]string {
	out := map[string]string{}
	put := func(category, name string) {
		trimmed := strings.TrimSpace(name)
		if category == "" || trimmed == "" {
			return
		}
		out[category] = canonicalToolName(trimmed)
	}

	put("source_repository", cfg.Artifacts.SourceRepository.Name)
	put("container_registry", cfg.Artifacts.ContainerRegistry.Name)
	put("storage_backend", cfg.Artifacts.StorageBackend.Name)
	put("ci_platform", cfg.Pipeline.CIPlatform.Name)
	put("cd_tool", cfg.Pipeline.CDTool.Name)
	put("monitoring_collection", cfg.Monitoring.Collection.Name)
	put("monitoring_visualization", cfg.Monitoring.Visualization.Name)
	return out
}

func canonicalToolName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	alias := map[string]string{
		"gitlab":           "GitLab CE",
		"gitlab-ce":        "GitLab CE",
		"gitlab ci":        "GitLab CI",
		"gitlab-ci":        "GitLab CI",
		"gitlab registry":  "GitLab Registry",
		"gitlab-registry":  "GitLab Registry",
		"argocd":           "Argo CD",
		"argo cd":          "Argo CD",
		"minio":            "MinIO",
		"prometheus":       "Prometheus",
		"grafana":          "Grafana",
		"harbor":           "Harbor",
		"github":           "GitHub",
		"github actions":   "GitHub Actions",
		"github-actions":   "GitHub Actions",
	}
	if v, ok := alias[key]; ok {
		return v
	}
	return strings.TrimSpace(name)
}

// ValidateCompatibilityInput holds the tool combination to validate, plus
// optional target-cluster context the Pre-Deploy Gate uses to cross-check
// per-tool `ArchSupport` against the actual worker fleet.
//
// Two call modes are supported:
//   - Explicit mode: Tools is non-empty. The caller supplies the combination
//     directly (wizard pre-deploy preview, ad-hoc API calls).
//   - Persisted mode: Tools is empty and StackID is set. The use case loads
//     the stack via StackRepository and derives tools + ClusterID fallback
//     from the persisted aggregate. Used by the Deploy handler's server-side
//     gate so a stack can't skip verification by bypassing the UI.
//
// Explicit input wins when both are provided — a caller passing tools is
// asking about a specific combination and shouldn't be overridden by the
// stack's current persisted state.
type ValidateCompatibilityInput struct {
	// Tools maps tool category to tool name, e.g. {"ci_platform": "GitLab CI"}.
	Tools map[string]string

	// StackID, when set with empty Tools, triggers persisted mode: load the
	// stack from StackRepository, derive tools from stack.Tools, and fall
	// back to stack.ClusterID when ClusterID is empty.
	StackID string

	// ClusterID, when set, instructs the use case to resolve cluster node
	// architectures from the admin bounded context via ClusterReader. Takes
	// precedence over NodeArchitectures.
	ClusterID string

	// NodeArchitectures is the explicit override. Callers who already have
	// the fleet arch list (e.g. on-the-fly validation in the wizard before
	// a cluster row exists) should populate this directly.
	NodeArchitectures []string
}

// ValidationIssue represents a detailed compatibility finding.
type ValidationIssue struct {
	Tool     string `json:"tool"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Code     string `json:"code,omitempty"`
}

// ValidationOverall represents the rolled-up compatibility state.
type ValidationOverall struct {
	State string `json:"state"`
	Score int    `json:"score"`
}

// ValidateCompatibilityOutput holds the result of a compatibility validation.
type ValidateCompatibilityOutput struct {
	Compatible        bool
	Matrix            *domain.CompatibilityMatrix
	Message           string
	Overall           ValidationOverall
	Issues            []ValidationIssue
	NodeArchitectures []string
	CheckedAt         time.Time
}

// ValidateCompatibility checks whether a given tool combination matches a known matrix.
type ValidateCompatibility struct {
	repo          port.CompatibilityRepository
	clusterReader port.ClusterReader
	stackRepo     port.StackRepository
	cache         VerdictCache
}

// NewValidateCompatibility constructs a ValidateCompatibility use case.
func NewValidateCompatibility(repo port.CompatibilityRepository, opts ...ValidateCompatibilityOption) *ValidateCompatibility {
	uc := &ValidateCompatibility{repo: repo}
	for _, o := range opts {
		o(uc)
	}
	return uc
}

// ValidateCompatibilityOption configures optional dependencies.
type ValidateCompatibilityOption func(*ValidateCompatibility)

// WithClusterReader wires a ClusterReader so the Pre-Deploy Gate can resolve
// node architectures from a cluster_id instead of requiring the caller to
// pass NodeArchitectures explicitly.
func WithClusterReader(r port.ClusterReader) ValidateCompatibilityOption {
	return func(uc *ValidateCompatibility) { uc.clusterReader = r }
}

// WithStackRepository enables persisted mode: when Execute receives an
// input with empty Tools but a non-empty StackID, the use case loads the
// stack row and derives tools + ClusterID fallback from it. The Deploy
// handler uses this to run the gate server-side after stack creation so
// the UI cannot bypass arch / tier checks.
func WithStackRepository(r port.StackRepository) ValidateCompatibilityOption {
	return func(uc *ValidateCompatibility) { uc.stackRepo = r }
}

// WithVerdictCache wires a short-TTL cache in front of Execute. Repeat calls
// with the same (stack_id, cluster_id, node_architectures, tools) tuple
// return the previously computed verdict without touching the repository
// until the entry expires or is invalidated.
func WithVerdictCache(cache VerdictCache) ValidateCompatibilityOption {
	return func(uc *ValidateCompatibility) { uc.cache = cache }
}

// Execute validates the tool combination and returns the matching matrix if found.
func (uc *ValidateCompatibility) Execute(ctx context.Context, input ValidateCompatibilityInput) (*ValidateCompatibilityOutput, error) {
	// F8 follow-up Phase 6: short-circuit on cache hit. The key is computed
	// from the request shape before persisted-mode resolution so identical
	// shape → identical key; persisted-mode stacks still get individual
	// entries because StackID is part of the key.
	var cacheKey string
	if uc.cache != nil {
		cacheKey = VerdictCacheKey(input)
		if cached, ok := uc.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Persisted mode: derive tools + cluster context from the stored stack
	// when the caller omitted Tools. Explicit tools always take precedence
	// so callers can re-check a proposed combination without writing it.
	if len(input.Tools) == 0 && input.StackID != "" {
		if uc.stackRepo == nil {
			return nil, fmt.Errorf("persisted validate mode requires a stack repository")
		}
		stack, err := uc.stackRepo.GetByID(ctx, input.StackID)
		if err != nil {
			return nil, fmt.Errorf("load stack %q: %w", input.StackID, err)
		}
		if stack == nil {
			return nil, fmt.Errorf("stack %q not found", input.StackID)
		}
		input.Tools = stackToolsToCategoryMap(stack.Tools)
		if len(input.Tools) == 0 {
			if cfg, ok := stack.Config.(domain.StackConfig); ok {
				input.Tools = stackConfigToCategoryMap(cfg)
			}
		}
		if input.ClusterID == "" {
			input.ClusterID = stack.ClusterID
		}
	}

	checkedAt := time.Now().UTC()

	if len(input.Tools) == 0 {
		if input.StackID != "" {
			out := &ValidateCompatibilityOutput{
				Compatible: true,
				Message:    "compatibility check skipped: stack tools are not configured",
				Overall: ValidationOverall{
					State: "pass",
					Score: 100,
				},
				Issues:    nil,
				CheckedAt: checkedAt,
			}
			if uc.cache != nil && cacheKey != "" {
				uc.cache.Put(cacheKey, out)
			}
			return out, nil
		}
		return nil, fmt.Errorf("tools map or stack_id is required")
	}

	// Resolve node architectures from cluster_id when no explicit override was
	// provided. "Cluster context" is considered present whenever the caller
	// supplies either ClusterID or an explicit NodeArchitectures slice;
	// legacy tool-only calls (no cluster in sight) skip the arch check.
	nodeArchs := normalizeArchs(input.NodeArchitectures)
	clusterContextProvided := input.ClusterID != "" || len(input.NodeArchitectures) > 0
	if len(nodeArchs) == 0 && input.ClusterID != "" && uc.clusterReader != nil {
		summary, lookupErr := uc.clusterReader.GetClusterSummary(ctx, input.ClusterID)
		if lookupErr == nil && summary != nil {
			nodeArchs = normalizeArchs(summary.NodeArchitectures)
		}
	}

	matrix, err := uc.repo.Validate(ctx, input.Tools)
	if err != nil {
		out := &ValidateCompatibilityOutput{
			Compatible: false,
			Message:    "no compatible matrix found for the given tool combination",
			Overall: ValidationOverall{
				State: "fail",
				Score: 0,
			},
			Issues: []ValidationIssue{{
				Tool:     "matrix",
				Message:  "No matching compatibility matrix for requested tools",
				Severity: "error",
				Code:     "MATRIX_NOT_FOUND",
			}},
			NodeArchitectures: nodeArchs,
			CheckedAt:         checkedAt,
		}
		if uc.cache != nil && cacheKey != "" {
			uc.cache.Put(cacheKey, out)
		}
		return out, nil
	}

	overall, issues := evaluateMatrixStatus(matrix.Status)
	if clusterContextProvided {
		overall, issues = applyArchCheck(matrix, nodeArchs, overall, issues)
	}

	out := &ValidateCompatibilityOutput{
		Compatible:        overall.State != "fail",
		Matrix:            matrix,
		Message:           fmt.Sprintf("tool combination matches matrix %q (status: %s)", matrix.Name, matrix.Status),
		Overall:           overall,
		Issues:            issues,
		NodeArchitectures: nodeArchs,
		CheckedAt:         checkedAt,
	}
	if uc.cache != nil && cacheKey != "" {
		uc.cache.Put(cacheKey, out)
	}
	return out, nil
}

func normalizeArchs(archs []string) []string {
	if len(archs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(archs))
	out := make([]string, 0, len(archs))
	for _, a := range archs {
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// applyArchCheck downgrades the verdict when a tool's ArchSupport does not
// cover every architecture present in the target cluster. Policy:
//
//   - unknown archs (empty slice): add a warn issue asking the user to
//     refresh discovery, do not change state unless already fail.
//   - verified matrix + any arch miss: fail (hard block). The user must
//     either swap the tool or add a matching node pool.
//   - untested/other matrix + any arch miss: downgrade pass→warn, leave
//     existing warn/fail intact. The matrix is already unverified, so a
//     second-layer arch miss is still recoverable via explicit ack.
func applyArchCheck(matrix *domain.CompatibilityMatrix, nodeArchs []string, overall ValidationOverall, issues []ValidationIssue) (ValidationOverall, []ValidationIssue) {
	if matrix == nil {
		return overall, issues
	}
	if len(nodeArchs) == 0 {
		if overall.State == "pass" {
			overall.State = "warn"
			if overall.Score > 80 {
				overall.Score = 80
			}
		}
		issues = append(issues, ValidationIssue{
			Tool:     "cluster",
			Message:  "Cluster node architectures are unknown; refresh cluster discovery before proceeding.",
			Severity: "warning",
			Code:     "CLUSTER_ARCH_UNKNOWN",
		})
		return overall, issues
	}

	archMiss := false
	for category, tv := range matrix.Tools {
		missing := make([]string, 0, len(nodeArchs))
		for _, arch := range nodeArchs {
			if !tv.SupportsArch(arch) {
				missing = append(missing, arch)
			}
		}
		if len(missing) == 0 {
			continue
		}
		archMiss = true
		issues = append(issues, ValidationIssue{
			Tool:     tv.Name,
			Message:  fmt.Sprintf("%s (%s) does not publish images for %v", tv.Name, category, missing),
			Severity: severityForArchMiss(matrix.Status),
			Code:     "TOOL_ARCH_UNSUPPORTED",
		})
	}

	if archMiss {
		switch matrix.Status {
		case "verified":
			overall.State = "fail"
			overall.Score = 0
		default:
			// untested / unsupported / unknown: cap at warn, never silently pass.
			if overall.State == "pass" {
				overall.State = "warn"
			}
			if overall.State != "fail" && overall.Score > 60 {
				overall.Score = 60
			}
		}
	}
	return overall, issues
}

func severityForArchMiss(matrixStatus string) string {
	if matrixStatus == "verified" {
		return "error"
	}
	return "warning"
}

func evaluateMatrixStatus(status string) (ValidationOverall, []ValidationIssue) {
	switch status {
	case "verified":
		return ValidationOverall{State: "pass", Score: 100}, nil
	case "untested":
		return ValidationOverall{State: "warn", Score: 70}, []ValidationIssue{{
			Tool:     "matrix",
			Message:  "Matched matrix is untested; proceed with caution",
			Severity: "warning",
			Code:     "MATRIX_UNTESTED",
		}}
	case "unsupported":
		return ValidationOverall{State: "fail", Score: 0}, []ValidationIssue{{
			Tool:     "matrix",
			Message:  "Matched matrix is marked as unsupported",
			Severity: "error",
			Code:     "MATRIX_UNSUPPORTED",
		}}
	default:
		return ValidationOverall{State: "warn", Score: 50}, []ValidationIssue{{
			Tool:     "matrix",
			Message:  fmt.Sprintf("Matched matrix has unknown status %q", status),
			Severity: "warning",
			Code:     "MATRIX_STATUS_UNKNOWN",
		}}
	}
}

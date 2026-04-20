package domain

// CompatibilityMatrix represents a verified or untested tool combination matrix.
type CompatibilityMatrix struct {
	ID         string
	Name       string
	Status     string // verified | untested | unsupported
	Kubernetes KubernetesCompat
	Tools      map[string]ToolVersion
}

// KubernetesCompat describes the Kubernetes version range supported by the matrix.
type KubernetesCompat struct {
	Min         string
	Max         string
	Recommended string
}

// Tool tier constants describe the per-tool maturity level inside a matrix.
const (
	ToolTierStable     = "stable"
	ToolTierBeta       = "beta"
	ToolTierDeprecated = "deprecated"
)

// Node architecture constants used for per-tool ArchSupport.
const (
	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

// ToolVersion describes a single tool entry in a compatibility matrix.
//
// MinK8sVersion, ArchSupport, Tier were introduced in migration
// 000041_compat_tool_fields to support the Pre-Deploy Gate (F8 v2).
//   - MinK8sVersion: the minimum Kubernetes version the tool requires.
//     Empty string means "inherit from matrix.Kubernetes.Min".
//   - ArchSupport:   the CPU architectures the tool ships images for
//     (e.g. []string{"amd64", "arm64"}). Used by the ARM64 discovery
//     check in the deploy wizard.
//   - Tier:          the maturity of this specific tool pairing inside
//     the matrix. Distinct from CompatibilityMatrix.Status which is a
//     matrix-level verdict.
type ToolVersion struct {
	Name          string
	HelmVersion   string
	AppVersion    string
	MinK8sVersion string
	ArchSupport   []string
	Tier          string
}

// SupportsArch reports whether the tool lists the given CPU architecture
// as supported. An empty ArchSupport slice is treated as "amd64 only"
// for backward compatibility with v1 matrices that predate the field.
func (t ToolVersion) SupportsArch(arch string) bool {
	if len(t.ArchSupport) == 0 {
		return arch == ArchAMD64
	}
	for _, a := range t.ArchSupport {
		if a == arch {
			return true
		}
	}
	return false
}

// EffectiveMinK8sVersion returns MinK8sVersion if set, otherwise the
// matrix-level k8s.Min. Callers should use this when comparing against
// a target cluster version so that v1 matrices without per-tool data
// still answer correctly.
func (t ToolVersion) EffectiveMinK8sVersion(matrix KubernetesCompat) string {
	if t.MinK8sVersion != "" {
		return t.MinK8sVersion
	}
	return matrix.Min
}

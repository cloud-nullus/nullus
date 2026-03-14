package domain

// CompatibilityMatrix represents a verified or untested tool combination matrix.
type CompatibilityMatrix struct {
	ID         string
	Name       string
	Status     string // verified, untested
	Kubernetes KubernetesCompat
	Tools      map[string]ToolVersion
}

// KubernetesCompat describes the Kubernetes version range supported by the matrix.
type KubernetesCompat struct {
	Min         string
	Max         string
	Recommended string
}

// ToolVersion describes a single tool entry in a compatibility matrix.
type ToolVersion struct {
	Name        string
	HelmVersion string
	AppVersion  string
}

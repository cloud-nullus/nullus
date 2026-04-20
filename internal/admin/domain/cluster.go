package domain

import (
	"sort"
	"time"
)

// ConnectionStatus represents the connection status of a cluster.
type ConnectionStatus string

const (
	ConnectionStatusConnected        ConnectionStatus = "connected"
	ConnectionStatusPending          ConnectionStatus = "pending"
	ConnectionStatusUnreachable      ConnectionStatus = "unreachable"
	ConnectionStatusAuthFailed       ConnectionStatus = "auth_failed"
	ConnectionStatusConnectionFailed ConnectionStatus = "connection_failed"
)

// ClusterType represents the type of a cluster.
type ClusterType string

const (
	ClusterTypePipeline ClusterType = "pipeline"
	ClusterTypeTarget   ClusterType = "target"
)

type CloudProvider string

const (
	CloudProviderAWS          CloudProvider = "aws"
	CloudProviderAzure        CloudProvider = "azure"
	CloudProviderGCP          CloudProvider = "gcp"
	CloudProviderOCI          CloudProvider = "oci"
	CloudProviderIBMCloud     CloudProvider = "ibm_cloud"
	CloudProviderAlibabaCloud CloudProvider = "alibaba_cloud"
	CloudProviderTencentCloud CloudProvider = "tencent_cloud"
	CloudProviderNaverCloud   CloudProvider = "naver_cloud"
	CloudProviderKTCloud      CloudProvider = "kt_cloud"
	CloudProviderNHNCloud     CloudProvider = "nhn_cloud"
	CloudProviderOnPremise    CloudProvider = "on_premise"
)

func NormalizeClusterTypes(types []ClusterType, legacyType ClusterType) []ClusterType {
	if len(types) > 0 {
		seen := make(map[ClusterType]struct{}, len(types))
		out := make([]ClusterType, 0, len(types))
		for _, clusterType := range types {
			if clusterType == "" {
				continue
			}
			if _, ok := seen[clusterType]; ok {
				continue
			}
			seen[clusterType] = struct{}{}
			out = append(out, clusterType)
		}
		if len(out) > 0 {
			return out
		}
	}
	if legacyType != "" {
		return []ClusterType{legacyType}
	}
	return nil
}

func ResolvePrimaryClusterType(types []ClusterType, legacyType ClusterType) ClusterType {
	normalized := NormalizeClusterTypes(types, legacyType)
	for _, clusterType := range normalized {
		if clusterType == ClusterTypePipeline {
			return ClusterTypePipeline
		}
	}
	if len(normalized) > 0 {
		return normalized[0]
	}
	if legacyType != "" {
		return legacyType
	}
	return ClusterTypeTarget
}

// Cluster represents a Kubernetes cluster registered in the platform.
//
// NodeArchitectures is the sorted, de-duplicated set of
// `node.status.nodeInfo.architecture` values discovered from the cluster.
// It is maintained by ClusterUseCase discovery flows and consumed by the
// Stack Pre-Deploy Gate so a tool whose `ArchSupport` does not cover every
// arch in the target cluster can be blocked/warned before install.
type Cluster struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Type              ClusterType      `json:"type"`
	Types             []ClusterType    `json:"types"`
	CloudProvider     CloudProvider    `json:"cloud_provider"`
	Endpoint          string           `json:"endpoint"`
	ConnectionStatus  ConnectionStatus `json:"connection_status"`
	OrgID             string           `json:"org_id"`
	NodeArchitectures []string         `json:"node_architectures"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// ClusterDiscoveryInfo is the value object returned by kube discovery. It
// captures the subset of cluster facts we persist back onto the Cluster
// aggregate after a Verify/Refresh call.
type ClusterDiscoveryInfo struct {
	ServerVersion     string    `json:"server_version"`
	NodeArchitectures []string  `json:"node_architectures"`
	NodeCount         int       `json:"node_count"`
	DiscoveredAt      time.Time `json:"discovered_at"`
}

// NormalizeNodeArchitectures returns a sorted, de-duplicated copy of the
// input with empty entries removed. Callers should always use this before
// writing to the Cluster aggregate so repository layers can rely on
// deterministic ordering.
func NormalizeNodeArchitectures(archs []string) []string {
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

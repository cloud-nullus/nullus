package domain

import "time"

// ConnectionStatus represents the connection status of a cluster.
type ConnectionStatus string

const (
	ConnectionStatusConnected   ConnectionStatus = "connected"
	ConnectionStatusPending     ConnectionStatus = "pending"
	ConnectionStatusUnreachable ConnectionStatus = "unreachable"
	ConnectionStatusAuthFailed  ConnectionStatus = "auth_failed"
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
type Cluster struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Type             ClusterType      `json:"type"`
	Types            []ClusterType    `json:"types"`
	CloudProvider    CloudProvider    `json:"cloud_provider"`
	Endpoint         string           `json:"endpoint"`
	ConnectionStatus ConnectionStatus `json:"connection_status"`
	OrgID            string           `json:"org_id"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

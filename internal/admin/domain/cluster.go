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

// Cluster represents a Kubernetes cluster registered in the platform.
type Cluster struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Type             ClusterType      `json:"type"`
	Endpoint         string           `json:"endpoint"`
	ConnectionStatus ConnectionStatus `json:"connection_status"`
	OrgID            string           `json:"org_id"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

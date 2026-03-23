package domain

import "time"

type ResourceDefault struct {
	ToolKey          string    `json:"tool_key"`
	DisplayName      string    `json:"display_name"`
	CPURequest       float64   `json:"cpu_request"`
	CPULimit         float64   `json:"cpu_limit"`
	MemoryRequestGi  float64   `json:"memory_request_gi"`
	MemoryLimitGi    float64   `json:"memory_limit_gi"`
	StorageRequestGi float64   `json:"storage_request_gi"`
	StorageLimitGi   float64   `json:"storage_limit_gi"`
	IsDefault        bool      `json:"is_default"`
	UpdatedAt        time.Time `json:"updated_at"`
}

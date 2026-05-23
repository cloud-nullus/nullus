package domain

import "time"

type ResourceProfileBase string

const (
	ResourceProfileLocal      ResourceProfileBase = "local"
	ResourceProfileStartup    ResourceProfileBase = "startup"
	ResourceProfileStandard   ResourceProfileBase = "standard"
	ResourceProfileEnterprise ResourceProfileBase = "enterprise"
)

type ResourceVector struct {
	CPURequest       float64 `json:"cpuRequest"`
	CPULimit         float64 `json:"cpuLimit"`
	MemoryRequestGi  float64 `json:"memoryRequestGi"`
	MemoryLimitGi    float64 `json:"memoryLimitGi"`
	StorageRequestGi float64 `json:"storageRequestGi"`
	StorageLimitGi   float64 `json:"storageLimitGi"`
}

type PlanningRowUnit struct {
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

type OrgResourceProfile struct {
	ID                       string                        `json:"id"`
	Name                     string                        `json:"name"`
	OrgID                    string                        `json:"orgId"`
	BaseProfile              ResourceProfileBase           `json:"baseProfile"`
	OptionOverrides          map[string]map[string]float64 `json:"optionOverrides"`
	AppliedResourceOverrides map[string]ResourceVector     `json:"appliedResourceOverrides"`
	RowUnits                 map[string]PlanningRowUnit    `json:"rowUnits"`
	CreatedAt                time.Time                     `json:"createdAt"`
}

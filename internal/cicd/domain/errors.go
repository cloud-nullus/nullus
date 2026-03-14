package domain

import "fmt"

// ErrPipelineNotFound is returned when a pipeline cannot be found.
var ErrPipelineNotFound = fmt.Errorf("pipeline not found")

// ErrTemplateNotFound is returned when a pipeline template cannot be found.
var ErrTemplateNotFound = fmt.Errorf("pipeline template not found")

// ErrDeploymentNotFound is returned when a deployment cannot be found.
var ErrDeploymentNotFound = fmt.Errorf("deployment not found")
